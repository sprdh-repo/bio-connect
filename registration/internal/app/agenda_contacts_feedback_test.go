package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func menuKeys(items []menuItem) string {
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.Key
	}
	return strings.Join(keys, " ")
}

// staffRequester signs in a staff account of the given role and returns a
// request helper that carries its session and CSRF token.
func staffRequester(t *testing.T, a *App, email, role string) func(method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	staff, _ := addStaff(t, a, email, role)
	session, csrf := randomToken(), randomToken()
	if _, err := a.DB.Exec(t.Context(), "INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')", hash(session), staff, hash(csrf)); err != nil {
		t.Fatal(err)
	}
	return func(method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		r.Header.Set("X-CSRF-Token", csrf)
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, r)
		return rr
	}
}

func TestSessionSpeakerLinksAndAgendaTab(t *testing.T) {
	a := mustApp(t)
	request := staffRequester(t, a, "agenda@example.com", "reviewer")
	const path = "/api/v1/admin/mobile-content"
	load := func() mobileEditor {
		rr := request("GET", path, nil)
		var m mobileEditor
		if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &m) != nil {
			t.Fatalf("load: %d %s", rr.Code, rr.Body.String())
		}
		return m
	}
	m := load()
	speaker := m.Speakers[0].ID
	m.Guide.Sessions = []guideSession{{ID: "keynote", Title: "Keynote", SpeakerIDs: []string{speaker}, Published: true}}
	unknown := m
	unknown.Guide.Sessions = []guideSession{{ID: "keynote", Title: "Keynote", SpeakerIDs: []string{"nobody-here"}, Published: true}}
	if rr := request("PUT", path, unknown); rr.Code != 400 || !strings.Contains(rr.Body.String(), "not in the speaker list") {
		t.Fatalf("unknown speaker link accepted: %d %s", rr.Code, rr.Body.String())
	}
	twice := m
	twice.Guide.Sessions = []guideSession{{ID: "keynote", Title: "Keynote", SpeakerIDs: []string{speaker, speaker}, Published: true}}
	if rr := request("PUT", path, twice); rr.Code != 400 {
		t.Fatalf("duplicate speaker link accepted: %d", rr.Code)
	}
	badTab := m
	badTab.Content.Menus = map[string][]menuItem{"tabs": {{Key: "contacts", Published: true}}}
	if rr := request("PUT", path, badTab); rr.Code != 400 || !strings.Contains(rr.Body.String(), "My agenda") {
		t.Fatalf("contacts accepted as a tab: %d %s", rr.Code, rr.Body.String())
	}
	m.Content.Feedback = contentFeedback{Open: true, Intro: "Tell us how it went."}
	if rr := request("PUT", path, m); rr.Code != 200 {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}

	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/app-content", nil))
	var doc struct {
		Feedback   contentFeedback `json:"feedback"`
		EventGuide struct {
			Sessions []map[string]any `json:"sessions"`
		} `json:"event_guide"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	ids, _ := doc.EventGuide.Sessions[0]["speaker_ids"].([]any)
	if len(ids) != 1 || ids[0] != speaker || !doc.Feedback.Open || doc.Feedback.Intro != "Tell us how it went." {
		t.Fatalf("public content: %s", rr.Body.String())
	}
}

func TestSessionsSavedBeforeSpeakerLinksReadAsEmptyLists(t *testing.T) {
	a := mustApp(t)
	if _, err := a.DB.Exec(t.Context(), `UPDATE event_guide SET document='{"sessions":[{"id":"old","title":"Old","published":true}]}' WHERE id='mobile'`); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/event-guide", nil))
	if !strings.Contains(rr.Body.String(), `"speaker_ids":[]`) {
		t.Fatalf("legacy session: %s", rr.Body.String())
	}
}

func TestBadgeContactSharesOnlyWithConsent(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "badges@example.com", "reviewer")
	rid, err := a.StaffCreate(t.Context(), staffInput(exhibitorInput("standard", 2), "complimentary"), key(1), tinyPNG(t), sid)
	if err != nil {
		t.Fatal(err)
	}
	var qr0, pass0, pass1, phone0 string
	if err = a.DB.QueryRow(t.Context(), "SELECT p.qr_id,p.id,a.phone FROM passes p JOIN attendees a ON a.id=p.attendee_id WHERE p.registration_id=$1 AND a.email='rep0@example.com'", rid).Scan(&qr0, &pass0, &phone0); err != nil {
		t.Fatal(err)
	}
	if err = a.DB.QueryRow(t.Context(), "SELECT p.id FROM passes p JOIN attendees a ON a.id=p.attendee_id WHERE p.registration_id=$1 AND a.email='rep1@example.com'", rid).Scan(&pass1); err != nil {
		t.Fatal(err)
	}
	badge := func(qr string) (int, badgeContact) {
		rr := mobileRequest(a, "GET", "/api/v1/public/badges/"+qr, nil, "")
		var c badgeContact
		_ = json.Unmarshal(rr.Body.Bytes(), &c)
		return rr.Code, c
	}
	code, c := badge(qr0)
	if code != 200 || c.Name != "Rep 0" || c.Institution == "" || c.Category == "" || c.Email != "" || c.Phone != "" {
		t.Fatalf("badge without consent: %d %+v", code, c)
	}
	if code, _ = badge(strings.Repeat("x", 43)); code != 404 {
		t.Fatalf("unknown badge: %d", code)
	}
	if code, _ = badge("not-a-badge"); code != 404 {
		t.Fatalf("malformed badge: %d", code)
	}

	challenge, otp := issueMobileCode(t, a, "email", "rep0@example.com", "")
	token := verifyMobileCode(t, a, challenge, otp)
	share := func(pass, token string, body any) int {
		return mobileRequest(a, "PUT", "/api/v1/mobile/passes/"+pass+"/sharing", body, token).Code
	}
	if got := share(pass0, "", map[string]bool{"share_email": true}); got != 401 {
		t.Fatalf("anonymous consent: %d", got)
	}
	// A verified holder cannot consent on behalf of a colleague on the same stall.
	if got := share(pass1, token, map[string]bool{"share_email": true, "share_phone": true}); got != 404 {
		t.Fatalf("consent for someone else's pass: %d", got)
	}
	if got := share(pass0, token, map[string]bool{"share_email": true}); got != 200 {
		t.Fatalf("consent: %d", got)
	}
	if _, c = badge(qr0); c.Email != "rep0@example.com" || c.Phone != "" {
		t.Fatalf("email consent: %+v", c)
	}
	rr := mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if !strings.Contains(rr.Body.String(), `"share_email":true,"share_phone":false`) {
		t.Fatalf("passes do not report consent: %s", rr.Body.String())
	}
	if got := share(pass0, token, map[string]bool{"share_phone": true}); got != 200 {
		t.Fatalf("change consent: %d", got)
	}
	if _, c = badge(qr0); c.Email != "" || c.Phone != phone0 || phone0 == "" {
		t.Fatalf("phone consent: %+v", c)
	}
	if _, err = a.DB.Exec(t.Context(), "UPDATE passes SET revoked_at=now() WHERE id=$1", pass0); err != nil {
		t.Fatal(err)
	}
	if code, _ = badge(qr0); code != 404 {
		t.Fatalf("revoked badge still resolves: %d", code)
	}
}

func TestFeedback(t *testing.T) {
	a := mustApp(t)
	if _, err := a.DB.Exec(t.Context(), `UPDATE event_guide SET document='{"sessions":[{"id":"s1","title":"=HYPERLINK(\"x\")","published":true},{"id":"draft","title":"Draft","published":false}]}' WHERE id='mobile'`); err != nil {
		t.Fatal(err)
	}
	device := "device-0123456789abcdef"
	send := func(body map[string]any) *httptest.ResponseRecorder {
		return mobileRequest(a, "POST", "/api/v1/public/feedback", body, "")
	}
	if rr := send(map[string]any{"device_id": device, "kind": "event", "rating": 5}); rr.Code != 409 {
		t.Fatalf("closed feedback accepted: %d %s", rr.Code, rr.Body.String())
	}
	if _, err := a.DB.Exec(t.Context(), `UPDATE app_content SET document=jsonb_set(document,'{feedback,open}','true') WHERE id='mobile'`); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []map[string]any{
		{"device_id": "short", "kind": "event", "rating": 5},
		{"device_id": device, "kind": "event", "rating": 6},
		{"device_id": device, "kind": "event", "session_id": "s1", "rating": 4},
		{"device_id": device, "kind": "session", "rating": 4},
		{"device_id": device, "kind": "speaker", "rating": 4},
		{"device_id": device, "kind": "event", "rating": 4, "comment": strings.Repeat("a", 2001)},
	} {
		if rr := send(bad); rr.Code != 400 {
			t.Fatalf("invalid feedback %v accepted: %d", bad, rr.Code)
		}
	}
	if rr := send(map[string]any{"device_id": device, "kind": "session", "session_id": "draft", "rating": 4}); rr.Code != 404 {
		t.Fatalf("feedback for an unpublished session: %d", rr.Code)
	}
	for _, ok := range []map[string]any{
		{"device_id": device, "kind": "event", "rating": 2, "comment": "first thought"},
		{"device_id": device, "kind": "event", "rating": 4, "comment": "changed my mind"},
		{"device_id": "device-fedcba9876543210", "kind": "event", "rating": 5},
		{"device_id": device, "kind": "session", "session_id": "s1", "rating": 3, "comment": "=1+1"},
	} {
		if rr := send(ok); rr.Code != 200 {
			t.Fatalf("feedback %v: %d %s", ok, rr.Code, rr.Body.String())
		}
	}

	if rr := mobileRequest(a, "GET", "/api/v1/admin/feedback", nil, ""); rr.Code != 401 {
		t.Fatalf("anonymous feedback report: %d", rr.Code)
	}
	// Both staff roles read feedback, like the app editor.
	request := staffRequester(t, a, "feedback@example.com", "manager")
	rr := request("GET", "/api/v1/admin/feedback", nil)
	var report struct {
		Open      bool               `json:"open"`
		Summary   []feedbackSummary  `json:"summary"`
		Responses []feedbackResponse `json:"responses"`
	}
	if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &report) != nil {
		t.Fatalf("report: %d %s", rr.Code, rr.Body.String())
	}
	if !report.Open || len(report.Responses) != 3 || len(report.Summary) != 2 {
		t.Fatalf("report: %+v", report)
	}
	event, session := report.Summary[0], report.Summary[1]
	if event.Kind != "event" || event.Responses != 2 || event.Average != 4.5 || event.Ratings != [5]int{0, 0, 0, 1, 1} {
		t.Fatalf("event summary (an answer is replaced, not added): %+v", event)
	}
	if session.SessionID != "s1" || session.Responses != 1 || session.Average != 3 {
		t.Fatalf("session summary: %+v", session)
	}
	rr = request("GET", "/api/v1/admin/feedback?format=csv", nil)
	body := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(body, "changed my mind") || strings.Contains(body, "first thought") ||
		!strings.Contains(body, "'=1+1") || !strings.Contains(body, `'=HYPERLINK`) {
		t.Fatalf("csv: %d %s", rr.Code, body)
	}
}
