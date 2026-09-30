package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuideValidationAndPublication(t *testing.T) {
	g := eventGuide{Sessions: []guideSession{{ID: "draft", Title: "Draft"}, {ID: "live", Title: "Live", Published: true}}, FAQs: []guideFAQ{{ID: "q", Question: "Where?", Answer: "Here", Published: true}}, Venue: guideVenue{Address: "Private draft"}}
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	public := g.public()
	if len(public.Sessions) != 1 || public.Sessions[0].ID != "live" || public.Venue.Address != "" {
		t.Fatal("draft content leaked")
	}
	g.Sessions[1].StartsAt = "2026-10-08T10:00:00+05:30"
	g.Sessions[1].EndsAt = "2026-10-08T09:00:00+05:30"
	if g.validate() == nil {
		t.Fatal("reversed session times accepted")
	}
	g.Sessions[1].EndsAt = "2026-10-08T11:00:00+05:30"
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	g.Venue.FloorPlanURL = "javascript:alert(1)"
	if g.validate() == nil {
		t.Fatal("unsafe URL accepted")
	}
	g.Venue.FloorPlanURL = "https://example.com/plan.pdf"
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	g.FAQs[0].Answer = ""
	if g.validate() == nil {
		t.Fatal("empty published FAQ accepted")
	}
}
func TestEventGuideAPI(t *testing.T) {
	a := mustApp(t)
	h := a.Handler()
	staff, _ := addStaff(t, a, "guide@example.com", "manager")
	session, csrf := randomToken(), randomToken()
	if _, err := a.DB.Exec(t.Context(), "INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')", hash(session), staff, hash(csrf)); err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any, auth, token bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		}
		if token {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr
	}
	const path = "/api/v1/admin/event-guide"
	if rr := request("GET", path, nil, false, false); rr.Code != 401 {
		t.Fatalf("anonymous admin: %d", rr.Code)
	}
	rr := request("GET", path, nil, true, false)
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var g eventGuide
	if err := json.Unmarshal(rr.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	g.Sessions = []guideSession{{ID: "draft", Title: "Secret draft"}, {ID: "live", Title: "Published session", Published: true}}
	if rr = request("PUT", path, g, true, false); rr.Code != 401 {
		t.Fatalf("missing CSRF: %d", rr.Code)
	}
	if rr = request("PUT", path, g, true, true); rr.Code != 200 {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}
	if rr = request("PUT", path, g, true, true); rr.Code != 409 {
		t.Fatalf("stale overwrite: %d", rr.Code)
	}
	for _, url := range []string{"/api/v1/public/event-guide", "/api/v1/public/app-content"} {
		rr = request("GET", url, nil, false, false)
		if rr.Code != 200 || bytes.Contains(rr.Body.Bytes(), []byte("Secret draft")) || !bytes.Contains(rr.Body.Bytes(), []byte("Published session")) {
			t.Fatalf("publication: %d %s", rr.Code, rr.Body.String())
		}
	}
}
