package app

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Attendees rate the event and individual sessions from the app while staff
// keep feedback open. Answers are anonymous: the app sends a random
// per-install device_id so a changed answer replaces the earlier one.

var feedbackDevice = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type feedbackInput struct {
	DeviceID  string `json:"device_id"`
	Kind      string `json:"kind"`
	SessionID string `json:"session_id"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
}

func (a *App) publicFeedback(w http.ResponseWriter, r *http.Request) {
	var in feedbackInput
	if !decode(w, r, &in) {
		return
	}
	in.Comment = strings.TrimSpace(in.Comment)
	switch {
	case !feedbackDevice.MatchString(in.DeviceID):
		fail(w, 400, "update the app to send feedback")
		return
	case in.Kind != "event" && in.Kind != "session", (in.Kind == "event") != (in.SessionID == ""):
		fail(w, 400, "choose the event or a session to rate")
		return
	case in.Rating < 1 || in.Rating > 5:
		fail(w, 400, "choose a rating from 1 to 5")
		return
	case utf8.RuneCountInString(in.Comment) > 2000:
		fail(w, 400, "keep comments under 2000 characters")
		return
	}
	if a.limited(r.Context(), "feedback:"+a.clientPeer(r), 600, time.Hour) {
		fail(w, 429, "too many responses; try again shortly")
		return
	}
	content, _, _, err := loadMobileContent(r.Context(), a.DB)
	if err != nil {
		fail(w, 503, "feedback unavailable; retry shortly")
		return
	}
	if !content.Feedback.Open {
		fail(w, 409, "feedback is closed")
		return
	}
	if in.Kind == "session" {
		guide, err := a.loadEventGuide(r.Context())
		if err != nil {
			fail(w, 503, "feedback unavailable; retry shortly")
			return
		}
		found := false
		for _, s := range guide.public().Sessions {
			found = found || s.ID == in.SessionID
		}
		if !found {
			fail(w, 404, "this session is no longer in the programme")
			return
		}
	}
	_, err = a.DB.Exec(r.Context(), `INSERT INTO feedback(id,device_id,kind,session_id,rating,comment) VALUES($1,$2,$3,$4,$5,$6)
	 ON CONFLICT(device_id,kind,session_id) DO UPDATE SET rating=EXCLUDED.rating,comment=EXCLUDED.comment,updated_at=now()`,
		id(), in.DeviceID, in.Kind, in.SessionID, in.Rating, in.Comment)
	if err != nil {
		fail(w, 503, "feedback unavailable; retry shortly")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

type feedbackSummary struct {
	Kind      string  `json:"kind"`
	SessionID string  `json:"session_id"`
	Title     string  `json:"title"`
	Responses int     `json:"responses"`
	Average   float64 `json:"average"`
	// Ratings counts answers by star, index 0 for one star.
	Ratings [5]int `json:"ratings"`
}
type feedbackResponse struct {
	Kind      string    `json:"kind"`
	SessionID string    `json:"session_id"`
	Title     string    `json:"title"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	UpdatedAt time.Time `json:"updated_at"`
}

// adminFeedback reports feedback for staff: a summary per target and every
// response, newest first, or the responses as CSV with ?format=csv.
func (a *App) adminFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		fail(w, 405, "method not allowed")
		return
	}
	ctx := r.Context()
	content, _, _, err := loadMobileContent(ctx, a.DB)
	if err != nil {
		fail(w, 503, "feedback unavailable")
		return
	}
	guide, err := a.loadEventGuide(ctx)
	if err != nil {
		fail(w, 503, "feedback unavailable")
		return
	}
	titles := map[string]string{}
	for _, s := range guide.Sessions {
		titles[s.ID] = s.Title
	}
	title := func(kind, session string) string {
		if kind == "event" {
			return "The event overall"
		}
		if t := titles[session]; t != "" {
			return t
		}
		return "Removed session (" + session + ")"
	}
	rows, err := a.DB.Query(ctx, `SELECT kind,session_id,rating,comment,updated_at FROM feedback ORDER BY updated_at DESC,id`)
	if err != nil {
		fail(w, 503, "feedback unavailable")
		return
	}
	defer rows.Close()
	responses := []feedbackResponse{}
	for rows.Next() {
		var f feedbackResponse
		if err = rows.Scan(&f.Kind, &f.SessionID, &f.Rating, &f.Comment, &f.UpdatedAt); err != nil {
			fail(w, 503, "feedback unavailable")
			return
		}
		f.Title = title(f.Kind, f.SessionID)
		responses = append(responses, f)
	}
	if rows.Err() != nil {
		fail(w, 503, "feedback unavailable")
		return
	}
	if r.URL.Query().Get("format") == "csv" {
		var out bytes.Buffer
		cw := csv.NewWriter(&out)
		_ = cw.Write([]string{"target", "session_id", "rating", "comment", "updated_at"})
		for _, f := range responses {
			_ = cw.Write([]string{safeCell(f.Title), safeCell(f.SessionID), fmt.Sprint(f.Rating), safeCell(f.Comment), f.UpdatedAt.In(india).Format(time.RFC3339)})
		}
		cw.Flush()
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="Bio-Connect-4.0-Feedback.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(out.Bytes())
		return
	}
	// The event first, then sessions in programme order, then removed ones.
	order := map[string]int{"event:": 0}
	for i, s := range guide.Sessions {
		order["session:"+s.ID] = i + 1
	}
	byTarget := map[string]*feedbackSummary{}
	summary := []*feedbackSummary{}
	for _, f := range responses {
		key := f.Kind + ":" + f.SessionID
		s := byTarget[key]
		if s == nil {
			s = &feedbackSummary{Kind: f.Kind, SessionID: f.SessionID, Title: f.Title}
			byTarget[key] = s
			summary = append(summary, s)
		}
		s.Responses++
		s.Ratings[f.Rating-1]++
		s.Average += float64(f.Rating)
	}
	rank := func(s *feedbackSummary) int {
		if n, ok := order[s.Kind+":"+s.SessionID]; ok {
			return n
		}
		return len(order)
	}
	for _, s := range summary {
		s.Average = float64(int(s.Average/float64(s.Responses)*10+.5)) / 10
	}
	slices.SortStableFunc(summary, func(x, y *feedbackSummary) int { return rank(x) - rank(y) })
	respond(w, 200, map[string]any{"open": content.Feedback.Open, "summary": summary, "responses": responses})
}
