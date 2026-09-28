package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func addFreeLink(t *testing.T, a *App, token string, expires time.Time, autoApprove bool, kind string) string {
	t.Helper()
	staff, _ := addStaff(t, a, "free-links-"+strings.ToLower(token[:8])+"@bioconnect.test", "reviewer")
	linkID := id()
	if _, err := a.DB.Exec(context.Background(), `UPDATE free_registration_links SET revoked_at=now(),revoked_by=created_by
		WHERE registration_kind=$1 AND revoked_at IS NULL`, kind); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), `INSERT INTO free_registration_links(id,token_hash,expires_at,auto_approve,registration_kind,created_by)
		VALUES($1,$2,$3,$4,$5,$6)`, linkID, hash(token), expires, autoApprove, kind, staff); err != nil {
		t.Fatal(err)
	}
	return linkID
}

func TestFreeLinkRegistersEveryExhibitorCategory(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	a.Config.RegistrationEnabled = false
	if _, err := a.DB.Exec(context.Background(), "UPDATE categories SET open=false WHERE kind='exhibitor'"); err != nil {
		t.Fatal(err)
	}
	token := randomToken()
	linkID := addFreeLink(t, a, token, fixed.Add(24*time.Hour), true, "exhibitor")

	for i, tc := range []struct {
		category string
		roster   int
	}{{"premium", 5}, {"standard", 3}, {"table", 2}} {
		in := exhibitorInput(tc.category, tc.roster)
		in.Email = tc.category + "-free@example.com"
		in.FreeToken = token
		rid, managementToken, err := a.Create(context.Background(), in, key(820+i), tinyPNG(t))
		if err != nil {
			t.Fatalf("%s free registration: %v", tc.category, err)
		}
		if managementToken == "" || status(t, a, rid) != "approved" {
			t.Fatalf("%s was not immediately approved", tc.category)
		}
		var quoted int64
		var storedLink string
		if err = a.DB.QueryRow(context.Background(), "SELECT quoted_paise,free_link_id FROM registrations WHERE id=$1", rid).Scan(&quoted, &storedLink); err != nil {
			t.Fatal(err)
		}
		if quoted != 0 || storedLink != linkID {
			t.Fatalf("%s quoted=%d link=%q, want 0 and %q", tc.category, quoted, storedLink, linkID)
		}
		if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != tc.roster {
			t.Fatalf("%s active passes=%d, want %d", tc.category, n, tc.roster)
		}
		if n := count(t, a, "SELECT count(*) FROM files WHERE registration_id=$1 AND kind='logo'", rid); n != 1 {
			t.Fatalf("%s logos=%d, want 1", tc.category, n)
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != tc.roster {
			t.Fatalf("%s pass deliveries=%d, want %d", tc.category, n, tc.roster)
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack'", rid); n != 1 {
			t.Fatalf("%s pack deliveries=%d, want 1", tc.category, n)
		}
	}
}

func TestFreeLinkRegistersEveryDelegateCategory(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	a.Config.RegistrationEnabled = false
	if _, err := a.DB.Exec(context.Background(), "UPDATE categories SET open=false WHERE kind='delegate'"); err != nil {
		t.Fatal(err)
	}
	token := randomToken()
	linkID := addFreeLink(t, a, token, fixed.Add(24*time.Hour), true, "delegate")

	var lastID string
	var lastInput RegistrationInput
	for i, cat := range []string{"student", "startup", "faculty", "industry"} {
		in := delegateInput(cat)
		in.Email = cat + "@example.com"
		in.Attendees[0].Email = in.Email
		in.FreeToken = token
		rid, managementToken, err := a.Create(context.Background(), in, key(800+i), nil)
		if err != nil {
			t.Fatalf("%s free registration: %v", cat, err)
		}
		if managementToken == "" || status(t, a, rid) != "approved" {
			t.Fatalf("%s was not immediately approved", cat)
		}
		var quoted int64
		var storedLink string
		if err = a.DB.QueryRow(context.Background(), "SELECT quoted_paise,free_link_id FROM registrations WHERE id=$1", rid).Scan(&quoted, &storedLink); err != nil {
			t.Fatal(err)
		}
		if quoted != 0 || storedLink != linkID {
			t.Fatalf("%s quoted=%d link=%q, want 0 and %q", cat, quoted, storedLink, linkID)
		}
		if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 1 {
			t.Fatalf("%s active passes=%d, want 1", cat, n)
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != 1 {
			t.Fatalf("%s pass deliveries=%d, want 1", cat, n)
		}
		reg, err := a.registration(context.Background(), rid)
		if err != nil || !reg.Free {
			t.Fatalf("%s free flag=%v err=%v", cat, reg.Free, err)
		}
		lastID, lastInput = rid, in
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE free_registration_links SET revoked_at=$2 WHERE id=$1", linkID, fixed); err != nil {
		t.Fatal(err)
	}
	if rid, token, err := a.Create(context.Background(), lastInput, key(803), nil); err != nil || rid != lastID || token != "" {
		t.Fatalf("idempotent retry after link expiry = (%q,%q,%v), want (%q,empty,nil)", rid, token, err, lastID)
	}
}

func TestGovtOfficialCategoryRequiresDelegateFreeLink(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }

	if _, _, err := a.Create(context.Background(), delegateInput("official"), key(810), nil); err == nil || !strings.Contains(err.Error(), "only available through a free registration link") {
		t.Fatalf("ordinary official registration error=%v", err)
	}

	token := randomToken()
	linkID := addFreeLink(t, a, token, fixed.Add(24*time.Hour), true, "delegate")
	in := delegateInput("official")
	in.FreeToken = token
	rid, _, err := a.Create(context.Background(), in, key(811), nil)
	if err != nil {
		t.Fatalf("free official registration: %v", err)
	}
	var quoted int64
	var storedLink string
	if err = a.DB.QueryRow(context.Background(), "SELECT quoted_paise,free_link_id FROM registrations WHERE id=$1", rid).Scan(&quoted, &storedLink); err != nil {
		t.Fatal(err)
	}
	if quoted != 0 || storedLink != linkID || status(t, a, rid) != "approved" {
		t.Fatalf("official registration quoted=%d link=%q status=%s", quoted, storedLink, status(t, a, rid))
	}
}

func TestFreeLinkCanRequireReviewWithoutPayment(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	token := randomToken()
	addFreeLink(t, a, token, fixed.Add(24*time.Hour), false, "delegate")
	in := delegateInput("industry")
	in.FreeToken = token
	rid, managementToken, err := a.Create(context.Background(), in, key(850), nil)
	if err != nil {
		t.Fatal(err)
	}
	if managementToken == "" || status(t, a, rid) != "awaiting_review" {
		t.Fatalf("manual free registration status=%s token empty=%v", status(t, a, rid), managementToken == "")
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("payment submissions=%d, want 0", n)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("passes before review=%d, want 0", n)
	}
	staff, _ := addStaff(t, a, "manual-free-reviewer@bioconnect.test", "reviewer")
	if err := a.Review(context.Background(), rid, staff, ReviewInput{Action: "approve_send", Note: "invitation approved"}); err != nil {
		t.Fatalf("approve free registration without payment: %v", err)
	}
	if status(t, a, rid) != "approved" {
		t.Fatalf("status=%s, want approved", status(t, a, rid))
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 1 {
		t.Fatalf("active passes=%d, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != 1 {
		t.Fatalf("pass deliveries=%d, want 1", n)
	}
	rejectedInput := delegateInput("student")
	rejectedInput.Email = "rejected-free@example.com"
	rejectedInput.Attendees[0].Email = rejectedInput.Email
	rejectedInput.FreeToken = token
	rejectedID, _, err := a.Create(context.Background(), rejectedInput, key(851), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Review(context.Background(), rejectedID, staff, ReviewInput{Action: "rejected"}); err == nil {
		t.Fatal("free registration rejection without reason succeeded")
	}
	if err = a.Review(context.Background(), rejectedID, staff, ReviewInput{Action: "rejected", Note: "invitation could not be verified"}); err != nil {
		t.Fatalf("reject free registration: %v", err)
	}
	if status(t, a, rejectedID) != "rejected" {
		t.Fatalf("rejected status=%s, want rejected", status(t, a, rejectedID))
	}
}

func TestFreeExhibitorLinkCanRequireReviewWithoutPayment(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	token := randomToken()
	addFreeLink(t, a, token, fixed.Add(24*time.Hour), false, "exhibitor")
	in := exhibitorInput("standard", 3)
	in.FreeToken = token
	rid, _, err := a.Create(context.Background(), in, key(852), tinyPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	if status(t, a, rid) != "awaiting_review" {
		t.Fatalf("status=%s, want awaiting_review", status(t, a, rid))
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("payment submissions=%d, want 0", n)
	}
	staff, _ := addStaff(t, a, "manual-free-exhibitor-reviewer@bioconnect.test", "reviewer")
	if err = a.Review(context.Background(), rid, staff, ReviewInput{Action: "approve_send", Note: "complimentary stand approved"}); err != nil {
		t.Fatalf("approve free exhibitor without payment: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 3 {
		t.Fatalf("active passes=%d, want 3", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack'", rid); n != 1 {
		t.Fatalf("pack deliveries=%d, want 1", n)
	}
}

func TestFreeLinkExpiryIncludesWholeIndiaDate(t *testing.T) {
	expires, err := freeLinkExpiry("2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 1, 0, 0, 0, 0, india)
	if !expires.Equal(want) {
		t.Fatalf("expiry=%s, want %s", expires, want)
	}
}

func TestFreeLinkRejectsExpiredRevokedWrongKindAndCouponUse(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }

	expired := randomToken()
	addFreeLink(t, a, expired, fixed, true, "delegate")
	in := delegateInput("industry")
	in.FreeToken = expired
	if _, _, err := a.Create(context.Background(), in, key(900), nil); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired link error=%v", err)
	}

	revoked := randomToken()
	linkID := addFreeLink(t, a, revoked, fixed.Add(time.Hour), true, "delegate")
	if _, err := a.DB.Exec(context.Background(), "UPDATE free_registration_links SET revoked_at=$2 WHERE id=$1", linkID, fixed); err != nil {
		t.Fatal(err)
	}
	in.FreeToken = revoked
	if _, _, err := a.Create(context.Background(), in, key(901), nil); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("revoked link error=%v", err)
	}

	valid := randomToken()
	addFreeLink(t, a, valid, fixed.Add(time.Hour), true, "delegate")
	ex := exhibitorInput("table", 2)
	ex.FreeToken = valid
	if _, _, err := a.Create(context.Background(), ex, key(902), tinyPNG(t)); err == nil || !strings.Contains(err.Error(), "not valid for exhibitors") {
		t.Fatalf("delegate link used for exhibitor error=%v", err)
	}
	exhibitorToken := randomToken()
	addFreeLink(t, a, exhibitorToken, fixed.Add(time.Hour), true, "exhibitor")
	in.FreeToken = exhibitorToken
	if _, _, err := a.Create(context.Background(), in, key(904), nil); err == nil || !strings.Contains(err.Error(), "not valid for delegates") {
		t.Fatalf("exhibitor link used for delegate error=%v", err)
	}
	in.FreeToken, in.CouponCode = valid, "KSUM30"
	if _, _, err := a.Create(context.Background(), in, key(903), nil); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("coupon plus free link error=%v", err)
	}
}

func TestAdminCanReplaceAndExpireFreeLink(t *testing.T) {
	a := mustApp(t)
	fixed := time.Date(2026, 9, 28, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	staff, _ := addStaff(t, a, "reviewer-links@bioconnect.test", "reviewer")
	session, csrf := randomToken(), randomToken()
	if _, err := a.DB.Exec(context.Background(), `INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at)
		VALUES($1,$2,$3,now()+interval '1 hour')`, hash(session), staff, hash(csrf)); err != nil {
		t.Fatal(err)
	}
	h := a.Handler()

	request := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		req.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	generate := func(date string, autoApprove bool, kind string) (linkID, token string) {
		t.Helper()
		rr := request("POST", "/api/v1/admin/free-links", map[string]any{"expires_on": date, "auto_approve": autoApprove, "registration_kind": kind})
		if rr.Code != 201 {
			t.Fatalf("generate returned %d: %s", rr.Code, rr.Body.String())
		}
		var out struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		path := "/delegates#free="
		if kind == "exhibitor" {
			path = "/exhibitors#free="
		}
		if !strings.HasPrefix(out.URL, a.Config.BaseURL+path) {
			t.Fatalf("%s URL=%q", kind, out.URL)
		}
		return out.ID, strings.TrimPrefix(out.URL, a.Config.BaseURL+path)
	}
	if rr := request("POST", "/api/v1/admin/free-links", map[string]any{"expires_on": "2026-09-30", "registration_kind": "sponsor"}); rr.Code != 400 {
		t.Fatalf("invalid registration kind returned %d", rr.Code)
	}

	firstID, firstToken := generate("2026-09-30", true, "delegate")
	exhibitorID, exhibitorToken := generate("2026-10-01", false, "exhibitor")
	secondID, secondToken := generate("2026-10-02", false, "delegate")
	if firstID == secondID || firstToken == secondToken {
		t.Fatal("replacement reused link credentials")
	}
	var firstRevoked bool
	if err := a.DB.QueryRow(context.Background(), "SELECT revoked_at IS NOT NULL FROM free_registration_links WHERE id=$1", firstID).Scan(&firstRevoked); err != nil || !firstRevoked {
		t.Fatalf("first link revoked=%v err=%v", firstRevoked, err)
	}
	var firstAuto, secondAuto bool
	if err := a.DB.QueryRow(context.Background(), "SELECT auto_approve FROM free_registration_links WHERE id=$1", firstID).Scan(&firstAuto); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT auto_approve FROM free_registration_links WHERE id=$1", secondID).Scan(&secondAuto); err != nil {
		t.Fatal(err)
	}
	if !firstAuto || secondAuto {
		t.Fatalf("stored approval modes first=%v second=%v", firstAuto, secondAuto)
	}
	var exhibitorRevoked bool
	if err := a.DB.QueryRow(context.Background(), "SELECT revoked_at IS NOT NULL FROM free_registration_links WHERE id=$1", exhibitorID).Scan(&exhibitorRevoked); err != nil || exhibitorRevoked {
		t.Fatalf("exhibitor link revoked=%v err=%v", exhibitorRevoked, err)
	}

	rr := request("POST", "/api/v1/free-registration/validate", map[string]string{"token": firstToken})
	if rr.Code != 404 {
		t.Fatalf("replaced link validation returned %d", rr.Code)
	}
	rr = request("POST", "/api/v1/free-registration/validate", map[string]string{"token": secondToken})
	if rr.Code != 200 {
		t.Fatalf("new link validation returned %d: %s", rr.Code, rr.Body.String())
	}
	rr = request("POST", "/api/v1/free-registration/validate", map[string]string{"token": exhibitorToken})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"registration_kind":"exhibitor"`) {
		t.Fatalf("exhibitor link validation returned %d: %s", rr.Code, rr.Body.String())
	}
	rr = request("POST", "/api/v1/admin/free-links/"+secondID+"/expire", map[string]any{})
	if rr.Code != 200 {
		t.Fatalf("expire returned %d: %s", rr.Code, rr.Body.String())
	}
	rr = request("POST", "/api/v1/free-registration/validate", map[string]string{"token": secondToken})
	if rr.Code != 404 {
		t.Fatalf("expired link validation returned %d", rr.Code)
	}
	rr = request("POST", "/api/v1/free-registration/validate", map[string]string{"token": exhibitorToken})
	if rr.Code != 200 {
		t.Fatalf("expiring delegate link also invalidated exhibitor link: %d %s", rr.Code, rr.Body.String())
	}
}
