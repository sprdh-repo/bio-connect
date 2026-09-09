package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestContactImports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.csv")
	if err := os.WriteFile(path, []byte("\ufeffEmail,Name\nPerson@example.com,Alice\nperson@example.com,Duplicate\nsecond@example.com,Bob\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"csv", "xlsx"} {
		if ext == "xlsx" {
			path = filepath.Join(dir, "contacts.xlsx")
			f := excelize.NewFile()
			for i, row := range [][]any{{"Email", "Name"}, {"Person@example.com", "Alice"}, {"person@example.com", "Duplicate"}, {"second@example.com", "Bob"}} {
				cell, _ := excelize.CoordinatesToCellName(1, i+1)
				if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
		got, err := contacts(path, "", "Email", "Name")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "Alice" || got[0].Email != "person@example.com" {
			t.Fatalf("%s: %#v", ext, got)
		}
		if _, err := contacts(path, "", "Wrong", ""); err == nil {
			t.Fatal("missing header accepted")
		}
	}
	bad := filepath.Join(dir, "bad.csv")
	os.WriteFile(bad, []byte("Email\ngood@example.com\nwrong\n"), 0600)
	if _, err := contacts(bad, "", "Email", ""); err == nil || !strings.Contains(err.Error(), "row 3") {
		t.Fatalf("invalid import: %v", err)
	}
}

func TestRenderingAndReport(t *testing.T) {
	h, p := render(contact{Name: "<script>&"})
	if strings.Contains(h, "<script>") || !strings.Contains(h, "&lt;script&gt;&amp;") || !strings.Contains(p, "<script>&") {
		t.Fatal("incorrect greeting escaping")
	}
	if !strings.Contains(h, "{{{ pm:unsubscribe }}}") || !strings.Contains(p, "{{{ pm:unsubscribe }}}") {
		t.Fatal("missing unsubscribe")
	}
	path := filepath.Join(t.TempDir(), "report.csv")
	rows := []entry{{Email: "a@example.com", Name: " =1+1", Status: "uncertain", Error: "timeout", Time: time.Now()}, {Email: "b@example.com", Status: "not_attempted"}}
	if err := writeReport(path, rows); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if got[1][2] != "' =1+1" || got[1][3] != "uncertain" || got[2][3] != "not_attempted" {
		t.Fatalf("bad report: %#v", got)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProviderRequest(t *testing.T) {
	p := provider{base: "https://api.postmarkapp.com", token: "secret", client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Postmark-Server-Token") != "secret" || r.Method != "POST" || r.URL.Path != "/email" {
			t.Fatal("wrong request")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["To"] != "a@example.com" {
			t.Fatal("wrong recipient")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"MessageID":"accepted-id"}`))}, nil
	})}}
	var result struct{ MessageID string }
	if err := p.request("POST", "/email", map[string]string{"To": "a@example.com"}, &result); err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "accepted-id" {
		t.Fatal("lost provider ID")
	}
	p.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 422, Body: io.NopCloser(strings.NewReader("private details"))}, nil
	})
	if err := p.request("POST", "/email", nil, &result); err == nil || strings.Contains(err.Error(), "private details") {
		t.Fatalf("bad failure: %v", err)
	}
}

func TestCampaignStopsReportsAndResumesWithoutDuplicates(t *testing.T) {
	if time.Now().After(time.Date(2026, 9, 30, 18, 30, 0, 0, time.UTC)) {
		t.Skip("dated invitation has expired")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.csv")
	if err := os.WriteFile(path, []byte("Email\na@example.com\nb@example.com\nc@example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POSTMARK_SERVER_TOKEN", "fake-token")
	t.Setenv("POSTMARK_FROM_ADDRESS", "sender@example.com")
	originalTransport, originalArgs, originalFlags := http.DefaultTransport, os.Args, flag.CommandLine
	defer func() {
		http.DefaultTransport = originalTransport
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	}()
	sends := map[string]int{}
	http.DefaultTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"MessageStreamType":"Broadcasts","SubscriptionManagementConfiguration":{"UnsubscribeHandlingType":"Postmark"}}`
		if r.Method == "POST" {
			var payload struct{ To, HtmlBody, TextBody, MessageStream string }
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.MessageStream != "broadcast" || !strings.Contains(payload.HtmlBody, "pm:unsubscribe") || payload.TextBody == "" {
				t.Fatal("incomplete marketing payload")
			}
			sends[payload.To]++
			if payload.To == "b@example.com" {
				return nil, errors.New("simulated connection loss after acceptance")
			}
			body = `{"ErrorCode":0,"MessageID":"message-id"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	invoke := func() error {
		os.Args = []string{"marketing", "--contacts", path, "--state", dir, "--send"}
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
		return run()
	}
	if err := invoke(); err == nil || !strings.Contains(err.Error(), "uncertain") {
		t.Fatalf("expected uncertain send: %v", err)
	}
	reports, _ := filepath.Glob(filepath.Join(dir, "report-*.csv"))
	if len(reports) != 1 {
		t.Fatal(reports)
	}
	f, err := os.Open(reports[0])
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][3] != "accepted" || rows[2][3] != "uncertain" || rows[3][3] != "not_attempted" {
		t.Fatalf("wrong outcomes: %#v", rows)
	}
	if err := invoke(); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		if sends[email] != 1 {
			t.Fatalf("%s sent %d times", email, sends[email])
		}
	}
}
