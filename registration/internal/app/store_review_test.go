package app

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func reviewChallenge(t *testing.T, a *App, email string) string {
	t.Helper()
	rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]any{"channel": "email", "identifier": email}, "")
	if rr.Code != 202 {
		t.Fatalf("challenge: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Challenge
}

func TestStoreReviewIsolation(t *testing.T) {
	a := mustApp(t)
	a.Config.StoreReviewCode = "584219"
	sid, _ := addStaff(t, a, "review-fixture@example.com", "reviewer")
	rid, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(1), nil, sid)
	if err != nil {
		t.Fatal(err)
	}
	var realID string
	if err = a.DB.QueryRow(t.Context(), "SELECT id FROM passes WHERE registration_id=$1", rid).Scan(&realID); err != nil {
		t.Fatal(err)
	}
	ch := reviewChallenge(t, a, "REVIEW@bioconnect.example")
	wrong := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": ch, "code": "000000"}, "")
	if wrong.Code != 401 {
		t.Fatalf("wrong code accepted: %d", wrong.Code)
	}
	token := verifyMobileCode(t, a, ch, a.Config.StoreReviewCode)
	rr := mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), storeReviewQR) || strings.Contains(rr.Body.String(), "Asha") {
		t.Fatalf("review passes: %d %s", rr.Code, rr.Body.String())
	}
	rr = mobileRequest(a, "PUT", "/api/v1/mobile/passes/"+realID+"/sharing", map[string]bool{"share_email": true}, token)
	if rr.Code != 404 {
		t.Fatal("reviewer modified real attendee")
	}
	rr = mobileRequest(a, "PUT", "/api/v1/mobile/passes/store-review/sharing", map[string]bool{"share_email": true}, token)
	if rr.Code != 200 {
		t.Fatalf("review sharing: %d", rr.Code)
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if !strings.Contains(rr.Body.String(), `"share_email":true`) {
		t.Fatal("review preference not persisted")
	}
	req := httptest.NewRequest("GET", "/api/v1/mobile/moments/status?pass_id="+realID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if _, err := a.momentsAttendee(req); err != errMobilePass {
		t.Fatalf("real moments access: %v", err)
	}
	req = httptest.NewRequest("GET", "/api/v1/mobile/moments/status?pass_id=store-review", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if id, err := a.momentsAttendee(req); err != nil || id != "bio_connect_store_review" {
		t.Fatalf("review moments identity: %s %v", id, err)
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/review-pass.pdf?token="+url.QueryEscape(token), nil, "")
	if rr.Code != 200 || !strings.HasPrefix(rr.Body.String(), "%PDF") {
		t.Fatalf("pdf: %d", rr.Code)
	}
	rr = mobileRequest(a, "GET", "/api/v1/public/badges/"+storeReviewQR, nil, "")
	if rr.Code == 200 {
		t.Fatal("review QR accepted as real badge")
	}
	var n int
	if err = a.DB.QueryRow(t.Context(), "SELECT count(*) FROM delivery_jobs WHERE purpose='pass_otp'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("review caused delivery: %d %v", n, err)
	}
	// The same credential works with a fresh challenge, but never for another email.
	if _, err := a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	ch2 := reviewChallenge(t, a, storeReviewEmail)
	verifyMobileCode(t, a, ch2, a.Config.StoreReviewCode)
	if _, err := a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	realCh := reviewChallenge(t, a, "asha@example.com")
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": realCh, "code": a.Config.StoreReviewCode}, "")
	if rr.Code != 401 {
		t.Fatal("review code unlocked real attendee")
	}
	a.Config.StoreReviewCode = ""
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 401 {
		t.Fatal("disabled reviewer session still active")
	}
	a.Config.StoreReviewCode = "584219"
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 {
		t.Fatal("restored reviewer access failed")
	}
}
