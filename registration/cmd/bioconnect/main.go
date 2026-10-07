package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bioconnect/registration/internal/app"
)

func main() {
	if e := run(); e != nil {
		slog.Error("application stopped", "error", e.Error())
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c := app.FromEnv()
	a, e := app.Open(ctx, c)
	if e != nil {
		return e
	}
	defer a.DB.Close()
	if e = a.Migrate(ctx); e != nil {
		return e
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			return nil
		case "staff-create":
			var in struct{ Email, Password, Role string }
			if e = json.NewDecoder(os.Stdin).Decode(&in); e != nil {
				return fmt.Errorf("provide email, password and role as JSON on stdin")
			}
			uri, e := a.AddStaff(ctx, in.Email, in.Password, in.Role)
			if e != nil {
				return e
			}
			fmt.Println(uri)
			return nil
		case "poster-seed":
			// Installs the shipped poster templates. Safe to re-run: it keeps
			// any template that already exists, so a deploy never undoes a
			// layout staff changed in the builder. "poster-seed --replace"
			// overwrites them, which is how updated artwork is rolled out.
			replace := len(os.Args) > 2 && os.Args[2] == "--replace"
			results, e := a.SeedPosters(ctx, replace)
			if e != nil {
				return e
			}
			counts := map[string]int{}
			for _, r := range results {
				counts[r.Status]++
				fmt.Printf("  %-24s %-5s %s\n", r.Family, r.Size, r.Status)
			}
			fmt.Printf("\n%d templates: %d created, %d replaced, %d kept, %d kept after staff edits\n",
				len(results), counts["created"], counts["replaced"], counts["kept"], counts["kept (edited)"])
			if counts["kept"] > 0 && !replace {
				fmt.Println("re-run with --replace to roll out new artwork")
			}
			if counts["kept (edited)"] > 0 {
				fmt.Println("templates staff have edited are never overwritten; duplicate one to start from it")
			}
			return nil
		case "roster-notice":
			// Emails every exhibitor with unassigned passes a single-use link to
			// add the people who will use them. Safe to re-run: a registration is
			// only ever notified once this way.
			results, e := a.NotifyOpenPlaces(ctx)
			if e != nil {
				return e
			}
			queued := 0
			for _, r := range results {
				if r.Outcome == "queued" {
					queued++
				}
				fmt.Printf("  %-12s %-40.40s %s\n", r.Reference, r.Institution, r.Outcome)
			}
			fmt.Printf("\n%d of %d exhibitors notified\n", queued, len(results))
			return nil
		default:
			return fmt.Errorf("unknown command")
		}
	}
	server := &http.Server{Addr: c.Listen, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go a.Worker(ctx)
	go a.CampaignWorker(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("Bio Connect registration listening", "address", c.Listen, "registration_enabled", c.RegistrationEnabled, "live_delivery", c.LiveDelivery)
	e = server.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
