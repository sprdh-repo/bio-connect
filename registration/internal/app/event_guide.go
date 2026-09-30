package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type guideSession struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	Location    string `json:"location"`
	Speakers    string `json:"speakers"`
	Published   bool   `json:"published"`
}
type guideActivity struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Schedule    string `json:"schedule"`
	Location    string `json:"location"`
	Published   bool   `json:"published"`
}
type guideFAQ struct {
	ID        string `json:"id"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Published bool   `json:"published"`
}
type guideVenue struct {
	Address       string `json:"address"`
	Arrival       string `json:"arrival"`
	Accessibility string `json:"accessibility"`
	FloorPlanURL  string `json:"floor_plan_url"`
	HelpEmail     string `json:"help_email"`
	HelpPhone     string `json:"help_phone"`
	Published     bool   `json:"published"`
}
type eventGuide struct {
	Revision   int             `json:"revision"`
	Sessions   []guideSession  `json:"sessions"`
	Activities []guideActivity `json:"activities"`
	FAQs       []guideFAQ      `json:"faqs"`
	Venue      guideVenue      `json:"venue"`
}

func (g eventGuide) public() eventGuide {
	out := eventGuide{Revision: g.Revision, Sessions: []guideSession{}, Activities: []guideActivity{}, FAQs: []guideFAQ{}}
	for _, item := range g.Sessions {
		if item.Published {
			out.Sessions = append(out.Sessions, item)
		}
	}
	for _, item := range g.Activities {
		if item.Published {
			out.Activities = append(out.Activities, item)
		}
	}
	for _, item := range g.FAQs {
		if item.Published {
			out.FAQs = append(out.FAQs, item)
		}
	}
	if g.Venue.Published {
		out.Venue = g.Venue
	}
	return out
}
func (g eventGuide) validate() error {
	if len(g.Sessions) > 300 || len(g.Activities) > 100 || len(g.FAQs) > 100 {
		return fmt.Errorf("too many guide entries")
	}
	seen := map[string]bool{}
	check := func(kind, id, title string, fields ...string) error {
		if strings.TrimSpace(id) == "" || len(id) > 80 || seen[kind+id] {
			return fmt.Errorf("%s needs a unique ID", kind)
		}
		seen[kind+id] = true
		if strings.TrimSpace(title) == "" || len(title) > 300 {
			return fmt.Errorf("%s title is required (maximum 300 characters)", kind)
		}
		for _, s := range fields {
			if len(s) > 6000 {
				return fmt.Errorf("%s text exceeds 6000 characters", kind)
			}
		}
		return nil
	}
	for _, s := range g.Sessions {
		if err := check("session", s.ID, s.Title, s.Description, s.Location, s.Speakers); err != nil {
			return err
		}
		if (s.StartsAt == "") != (s.EndsAt == "") {
			return fmt.Errorf("provide both session start and end, or leave both blank")
		}
		if s.StartsAt != "" {
			start, e1 := time.Parse(time.RFC3339, s.StartsAt)
			end, e2 := time.Parse(time.RFC3339, s.EndsAt)
			if e1 != nil || e2 != nil || !end.After(start) {
				return fmt.Errorf("session times need timezone offsets and an end after the start")
			}
		}
	}
	for _, a := range g.Activities {
		if err := check("activity", a.ID, a.Title, a.Description, a.Schedule, a.Location); err != nil {
			return err
		}
	}
	for _, f := range g.FAQs {
		if err := check("FAQ", f.ID, f.Question, f.Answer); err != nil {
			return err
		}
		if f.Published && strings.TrimSpace(f.Answer) == "" {
			return fmt.Errorf("published FAQs need an answer")
		}
	}
	v := g.Venue
	for _, s := range []string{v.Address, v.Arrival, v.Accessibility, v.HelpEmail, v.HelpPhone, v.FloorPlanURL} {
		if len(s) > 6000 {
			return fmt.Errorf("venue text exceeds 6000 characters")
		}
	}
	if v.FloorPlanURL != "" {
		u, err := url.Parse(v.FloorPlanURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("floor plan must be a public HTTPS URL")
		}
	}
	if v.HelpEmail != "" && !validEmail(v.HelpEmail) {
		return fmt.Errorf("enter a valid help email")
	}
	for _, c := range v.HelpPhone {
		if !strings.ContainsRune("+0123456789 ()-", c) {
			return fmt.Errorf("enter a valid help phone")
		}
	}
	return nil
}
func (a *App) loadEventGuide(ctx context.Context) (eventGuide, error) {
	var g eventGuide
	var raw []byte
	var revision int
	err := a.DB.QueryRow(ctx, "SELECT document,revision FROM event_guide WHERE id='mobile'").Scan(&raw, &revision)
	if err != nil {
		return g, err
	}
	err = json.Unmarshal(raw, &g)
	g.Revision = revision
	return g, err
}
func (a *App) publicEventGuide(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	g, err := a.loadEventGuide(r.Context())
	if err != nil {
		fail(w, 503, "event guide unavailable; please retry")
		return
	}
	respond(w, 200, g.public())
}
func (a *App) adminEventGuide(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method == "GET" {
		g, err := a.loadEventGuide(r.Context())
		if err != nil {
			fail(w, 503, "event guide unavailable")
			return
		}
		respond(w, 200, g)
		return
	}
	if r.Method != "PUT" {
		w.Header().Set("Allow", "GET, PUT")
		fail(w, 405, "method not allowed")
		return
	}
	var g eventGuide
	if !decode(w, r, &g) {
		return
	}
	if err := g.validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	// Always return JSON arrays, including for a newly emptied section.
	if g.Sessions == nil {
		g.Sessions = []guideSession{}
	}
	if g.Activities == nil {
		g.Activities = []guideActivity{}
	}
	if g.FAQs == nil {
		g.FAQs = []guideFAQ{}
	}
	raw, err := json.Marshal(g)
	if err != nil {
		fail(w, 400, "invalid guide")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		fail(w, 503, "could not save guide")
		return
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), "UPDATE event_guide SET document=$1,revision=revision+1,updated_at=now() WHERE id='mobile' AND revision=$2", raw, g.Revision)
	if err != nil {
		fail(w, 503, "could not save guide")
		return
	}
	if result.RowsAffected() != 1 {
		fail(w, 409, "guide changed in another window; reload before editing again")
		return
	}
	if err = audit(r.Context(), tx, p.ID, "", "event_guide.update", fmt.Sprintf("revision %d", g.Revision+1)); err != nil {
		fail(w, 503, "could not save guide")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "could not save guide")
		return
	}
	g.Revision++
	respond(w, 200, g)
}
