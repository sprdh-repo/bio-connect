package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	// SpeakerIDs links the session to the speaker directory, so the app can
	// list a speaker's sessions and plan a day around saved speakers.
	SpeakerIDs []string `json:"speaker_ids"`
	Published  bool     `json:"published"`
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
	HelpWhatsApp  string `json:"help_whatsapp"`
	Published     bool   `json:"published"`
	// Hidden names fields kept in the console but withheld from the app.
	Hidden []string `json:"hidden"`
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
		if len(s.SpeakerIDs) > 30 {
			return fmt.Errorf("session %q links too many speakers", s.Title)
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
	for _, s := range []string{v.Address, v.Arrival, v.Accessibility, v.HelpEmail, v.HelpPhone, v.HelpWhatsApp, v.FloorPlanURL} {
		if len(s) > 6000 {
			return fmt.Errorf("venue text exceeds 6000 characters")
		}
	}
	if !optionalHTTPS(v.FloorPlanURL) {
		return fmt.Errorf("floor plan must be a public HTTPS URL")
	}
	if v.HelpEmail != "" && !validEmail(v.HelpEmail) {
		return fmt.Errorf("enter a valid help email")
	}
	if !helpPhone(v.HelpPhone) {
		return fmt.Errorf("enter a valid help phone")
	}
	if !helpPhone(v.HelpWhatsApp) {
		return fmt.Errorf("enter a valid WhatsApp number")
	}
	return checkHidden("venue", v.Hidden, venueHideable)
}

var venueHideable = []string{"address", "arrival", "accessibility", "floor_plan_url", "help_email", "help_phone", "help_whatsapp"}

// helpPhone accepts a blank value or a loosely formatted contact number.
func helpPhone(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("+0123456789 ()-", c) {
			return false
		}
	}
	return true
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
	normalizeGuide(&g)
	return g, err
}
func (a *App) publicEventGuide(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	g, err := a.loadEventGuide(r.Context())
	if err != nil {
		fail(w, 503, "event guide unavailable; please retry")
		return
	}
	out, err := toJSONMap(g.public())
	if err != nil {
		fail(w, 503, "event guide unavailable; please retry")
		return
	}
	respond(w, 200, publicView(out))
}
