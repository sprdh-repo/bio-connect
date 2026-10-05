package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Reviewers can download a pass or a registration's whole pack for officials
// with no inbox of their own. Only live passes on approved registrations come
// out, never across registrations, never to managers, and every download is
// audited.
func TestAdminPassDownload(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	fixed := time.Now().UTC().Truncate(time.Minute)
	a.Now = func() time.Time { return fixed }
	h := a.Handler()

	rid, sid := approvedDelegate(t, a, 1, "approve_only")
	otherRid, _ := approvedDelegate(t, a, 2, "approve_only")
	pendingRid, _, err := a.Create(ctx, delegateInput("student"), key(3), nil)
	if err != nil {
		t.Fatal(err)
	}
	var passID, otherPassID string
	if err := a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1", rid).Scan(&passID); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1", otherRid).Scan(&otherPassID); err != nil {
		t.Fatal(err)
	}

	revID, revSecret := addStaff(t, a, "download@bioconnect.test", "reviewer")
	_, mgrSecret := addStaff(t, a, "mgr@bioconnect.test", "manager")
	revSession, _ := staffLogin(t, h, "download@bioconnect.test", revSecret, fixed)
	mgrSession, _ := staffLogin(t, h, "mgr@bioconnect.test", mgrSecret, fixed)
	get := func(path, session string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/admin/registrations/"+path, nil)
		if session != "" {
			req.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		}
		h.ServeHTTP(rr, req)
		return rr
	}

	rr := get(rid+"/passes/"+passID+".pdf", revSession)
	if rr.Code != 200 || !bytes.HasPrefix(rr.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("reviewer pass download returned %d %q", rr.Code, rr.Body.String()[:min(80, rr.Body.Len())])
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("pass content type %q", ct)
	}
	rr = get(rid+"/passes/all.zip", revSession)
	if rr.Code != 200 || !bytes.HasPrefix(rr.Body.Bytes(), []byte("PK")) {
		t.Fatalf("reviewer pack download returned %d", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE staff_id=$1 AND registration_id=$2 AND action='pass_downloaded'", revID, rid); n != 2 {
		t.Fatalf("%d pass_downloaded audit events, want 2", n)
	}

	for name, c := range map[string]struct {
		path, session string
		want          int
	}{
		"no session":              {rid + "/passes/" + passID + ".pdf", "", 401},
		"manager":                 {rid + "/passes/" + passID + ".pdf", mgrSession, 403},
		"pass of another reg":     {rid + "/passes/" + otherPassID + ".pdf", revSession, 404},
		"missing .pdf suffix":     {rid + "/passes/" + passID, revSession, 404},
		"unapproved registration": {pendingRid + "/passes/all.zip", revSession, 404},
	} {
		if rr := get(c.path, c.session); rr.Code != c.want {
			t.Errorf("%s: got %d, want %d", name, rr.Code, c.want)
		}
	}

	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "reissue", PassID: passID, Note: "lost"}); err != nil {
		t.Fatalf("reissue: %v", err)
	}
	if rr := get(rid+"/passes/"+passID+".pdf", revSession); rr.Code != 404 {
		t.Fatalf("revoked pass download returned %d, want 404", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE action='pass_downloaded'"); n != 2 {
		t.Fatalf("refused downloads were audited: %d events, want 2", n)
	}
}
