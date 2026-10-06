package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

// A programme entry. Kind sets how it is shown: a break is a divider in the
// timetable that attendees cannot save, rate or be reminded of.
type guideSession struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Title       string `json:"title"`
	Track       string `json:"track"`
	Description string `json:"description"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	Location    string `json:"location"`
	// People are listed as printed in the programme, each with the
	// designation for this session and, optionally, a directory link.
	People []guidePerson `json:"people"`
	// Segments are a ceremony's running order.
	Segments []guideSegment `json:"segments"`
	// Speakers and SpeakerIDs are derived from People and Segments. Released
	// app versions show Speakers as text and use SpeakerIDs to list a
	// speaker's sessions.
	Speakers   string   `json:"speakers"`
	SpeakerIDs []string `json:"speaker_ids"`
	Published  bool     `json:"published"`
}
type guidePerson struct {
	Name        string `json:"name"`
	Designation string `json:"designation"`
	Role        string `json:"role"`
	SpeakerID   string `json:"speaker_id"`
}
type guideSegment struct {
	StartsAt    string        `json:"starts_at"`
	EndsAt      string        `json:"ends_at"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	People      []guidePerson `json:"people"`
}

// indiaTime formats programme times; the event is in India, with no DST.
var indiaTime = time.FixedZone("IST", 330*60)

var (
	sessionKinds = []string{"", "talk", "panel", "ceremony", "social", "break"}
	personRoles  = []string{"", "moderator", "panelist", "host"}
)

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

// public is what attendees see. Breaks are left out unless withBreaks:
// app versions released before breaks existed would offer to save and rate
// them, so only clients that ask for breaks get them.
func (g eventGuide) public(withBreaks bool) eventGuide {
	out := eventGuide{Revision: g.Revision, Sessions: []guideSession{}, Activities: []guideActivity{}, FAQs: []guideFAQ{}}
	for _, item := range g.Sessions {
		if item.Published && (withBreaks || item.Kind != "break") {
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
		if !slices.Contains(sessionKinds, s.Kind) {
			return fmt.Errorf("session %q has an unknown type", s.Title)
		}
		if len(s.Label) > 120 || len(s.Track) > 120 {
			return fmt.Errorf("session %q label and track are limited to 120 characters", s.Title)
		}
		if err := sessionTimes(s.Title, s.StartsAt, s.EndsAt); err != nil {
			return err
		}
		if len(s.SpeakerIDs) > 60 {
			return fmt.Errorf("session %q links too many speakers", s.Title)
		}
		if err := checkPeople(s.Title, s.People); err != nil {
			return err
		}
		if len(s.Segments) > 40 {
			return fmt.Errorf("session %q has too many running-order items", s.Title)
		}
		for _, seg := range s.Segments {
			if strings.TrimSpace(seg.Title) == "" || len(seg.Title) > 300 || len(seg.Description) > 2000 {
				return fmt.Errorf("every running-order item in %q needs a title (maximum 300 characters)", s.Title)
			}
			if err := sessionTimes(s.Title+": "+seg.Title, seg.StartsAt, seg.EndsAt); err != nil {
				return err
			}
			if err := checkPeople(s.Title, seg.People); err != nil {
				return err
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

// sessionTimes accepts both times or neither, with offsets and in order.
func sessionTimes(title, startsAt, endsAt string) error {
	if (startsAt == "") != (endsAt == "") {
		return fmt.Errorf("%q: provide both start and end, or leave both blank", title)
	}
	if startsAt == "" {
		return nil
	}
	start, e1 := time.Parse(time.RFC3339, startsAt)
	end, e2 := time.Parse(time.RFC3339, endsAt)
	if e1 != nil || e2 != nil || !end.After(start) {
		return fmt.Errorf("%q: times need timezone offsets and an end after the start", title)
	}
	return nil
}

func checkPeople(title string, people []guidePerson) error {
	if len(people) > 30 {
		return fmt.Errorf("session %q lists too many people", title)
	}
	for _, p := range people {
		if strings.TrimSpace(p.Name) == "" || len(p.Name) > 200 || len(p.Designation) > 600 {
			return fmt.Errorf("every person in %q needs a name (maximum 200 characters)", title)
		}
		if !slices.Contains(personRoles, p.Role) {
			return fmt.Errorf("%s in %q has an unknown role", p.Name, title)
		}
	}
	return nil
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
	out, err := toJSONMap(g.public(r.URL.Query().Get("include") == "breaks"))
	if err != nil {
		fail(w, 503, "event guide unavailable; please retry")
		return
	}
	respond(w, 200, publicView(out))
}
