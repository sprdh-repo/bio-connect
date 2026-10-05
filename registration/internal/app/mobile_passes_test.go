package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mobileRequest(a *App, method, path string, body any, token string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, req)
	return rr
}

func TestMobilePassEmailJourney(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "passes@example.com", "reviewer")
	rid, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(1), nil, sid)
	if err != nil {
		t.Fatal(err)
	}
	rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]any{"channel": "email", "identifier": "ASHA@example.com"}, "")
	if rr.Code != 202 {
		t.Fatalf("request code: %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err = json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	challenge := out["challenge"].(string)
	var cipher string
	if err = a.DB.QueryRow(t.Context(), "SELECT payload_cipher FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass_otp'", rid).Scan(&cipher); err != nil {
		t.Fatal(err)
	}
	code, err := a.unseal(cipher)
	if err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": code}, "")
	if rr.Code != 200 {
		t.Fatalf("verify: %d %s", rr.Code, rr.Body.String())
	}
	if err = json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	token := out["token"].(string)
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || !bytes.Contains(rr.Body.Bytes(), []byte("Asha Nair")) {
		t.Fatalf("passes: %d %s", rr.Code, rr.Body.String())
	}
	// A challenge can never be replayed after successful verification.
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": code}, "")
	if rr.Code != 401 {
		t.Fatalf("replay accepted: %d", rr.Code)
	}
	// Live refresh removes a revoked pass without exposing a replacement token.
	if _, err = a.DB.Exec(t.Context(), "UPDATE passes SET revoked_at=now() WHERE registration_id=$1", rid); err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || bytes.Contains(rr.Body.Bytes(), []byte("Asha Nair")) {
		t.Fatalf("revoked pass visible: %d %s", rr.Code, rr.Body.String())
	}
}

func issueMobileCode(t *testing.T, a *App, channel, identifier, qr string) (string, string) {
	t.Helper()
	rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]any{"channel": channel, "identifier": identifier, "qr_id": qr, "whatsapp_consent": true}, "")
	if rr.Code != 202 {
		t.Fatalf("challenge: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var cipher string
	if err := a.DB.QueryRow(t.Context(), "SELECT j.payload_cipher FROM mobile_pass_challenges c JOIN delivery_jobs j ON j.id=c.delivery_id WHERE c.token_hash=$1", hash(out.Challenge)).Scan(&cipher); err != nil {
		t.Fatal(err)
	}
	code, err := a.unseal(cipher)
	if err != nil {
		t.Fatal(err)
	}
	return out.Challenge, code
}

func verifyMobileCode(t *testing.T, a *App, challenge, code string) string {
	t.Helper()
	rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": code}, "")
	if rr.Code != 200 {
		t.Fatalf("verify: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Token
}

func TestMobilePassOwnershipAndScan(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "desk@example.com", "reviewer")
	rid, err := a.StaffCreate(t.Context(), staffInput(exhibitorInput("standard", 2), "complimentary"), key(1), tinyPNG(t), sid)
	if err != nil {
		t.Fatal(err)
	}
	challenge, code := issueMobileCode(t, a, "email", "rep0@example.com", "")
	token := verifyMobileCode(t, a, challenge, code)
	rr := mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || !bytes.Contains(rr.Body.Bytes(), []byte("Rep 0")) || bytes.Contains(rr.Body.Bytes(), []byte("Rep 1")) {
		t.Fatalf("group access leaked: %d %s", rr.Code, rr.Body.String())
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, "")
	if rr.Code != 401 {
		t.Fatalf("anonymous passes: %d", rr.Code)
	}
	var qr string
	if err = a.DB.QueryRow(t.Context(), "SELECT p.qr_id FROM passes p JOIN attendees a ON a.id=p.attendee_id WHERE p.registration_id=$1 AND a.email='rep1@example.com'", rid).Scan(&qr); err != nil {
		t.Fatal(err)
	}
	// The scan path succeeds only with the scanned attendee's identity.
	challenge, code = issueMobileCode(t, a, "whatsapp", "+919812345671", qr)
	token = verifyMobileCode(t, a, challenge, code)
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || !bytes.Contains(rr.Body.Bytes(), []byte("Rep 1")) || bytes.Contains(rr.Body.Bytes(), []byte("Rep 0")) {
		t.Fatalf("scan scope: %d %s", rr.Code, rr.Body.String())
	}
	// A registration contact does not own its team's attendee passes.
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]any{"channel": "email", "identifier": "ravi@example.com"}, "")
	if rr.Code != 202 || count(t, a, "SELECT count(*) FROM delivery_jobs WHERE purpose='pass_otp' AND recipient='ravi@example.com'") != 0 {
		t.Fatal("contact gained team access")
	}
	// Scanning another attendee's QR does not reveal details or deliver a code.
	if _, err = a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]any{"channel": "email", "identifier": "rep0@example.com", "qr_id": qr}, "")
	if rr.Code != 202 || bytes.Contains(rr.Body.Bytes(), []byte("Rep 1")) {
		t.Fatal("scan leaked identity")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE purpose='pass_otp' AND recipient='rep0@example.com'"); n != 1 {
		t.Fatal("wrong owner received scanned-pass OTP")
	}
	// Inject revoked authorization, then restore it, proving both rejection and recovery.
	if _, err = a.DB.Exec(t.Context(), "UPDATE registrations SET status='cancelled' WHERE id=$1", rid); err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || bytes.Contains(rr.Body.Bytes(), []byte("Rep 1")) {
		t.Fatal("cancelled pass visible")
	}
	if _, err = a.DB.Exec(t.Context(), "UPDATE registrations SET status='approved' WHERE id=$1", rid); err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 || !bytes.Contains(rr.Body.Bytes(), []byte("Rep 1")) {
		t.Fatal("restored pass missing")
	}
	rr = mobileRequest(a, "DELETE", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 200 {
		t.Fatal("signout failed")
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 401 {
		t.Fatal("signed-out session accepted")
	}
}

func TestMobilePassOTPLimitsExpiryAndResend(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "desk@example.com", "reviewer")
	if _, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(1), nil, sid); err != nil {
		t.Fatal(err)
	}
	challenge, code := issueMobileCode(t, a, "email", "asha@example.com", "")
	wrong := "000000"
	if wrong == code {
		wrong = "999999"
	}
	for range 5 {
		rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": wrong}, "")
		if rr.Code != 401 {
			t.Fatal("incorrect code accepted")
		}
	}
	rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": code}, "")
	if rr.Code != 401 {
		t.Fatal("attempt limit bypassed")
	}
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]string{"channel": "email", "identifier": "asha@example.com"}, "")
	if rr.Code != 429 {
		t.Fatal("resend cooldown bypassed")
	}
	if _, err := a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	challenge, code = issueMobileCode(t, a, "email", "asha@example.com", "")
	if _, err := a.DB.Exec(t.Context(), "UPDATE mobile_pass_challenges SET expires_at=$2 WHERE token_hash=$1", hash(challenge), a.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": code}, "")
	if rr.Code != 401 {
		t.Fatal("expired code accepted")
	}
	if _, err := a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	old, oldCode := issueMobileCode(t, a, "email", "asha@example.com", "")
	if _, err := a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	newest, newCode := issueMobileCode(t, a, "email", "asha@example.com", "")
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": old, "code": oldCode}, "")
	if rr.Code != 401 {
		t.Fatal("old resend code accepted")
	}
	token := verifyMobileCode(t, a, newest, newCode)
	if _, err := a.DB.Exec(t.Context(), "UPDATE mobile_pass_sessions SET expires_at=$2 WHERE token_hash=$1", hash(token), a.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	rr = mobileRequest(a, "GET", "/api/v1/mobile/passes", nil, token)
	if rr.Code != 401 {
		t.Fatal("expired session accepted")
	}
}

func TestMobilePassConsentAndGenericResponse(t *testing.T) {
	a := mustApp(t)
	rr := mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]string{"channel": "whatsapp", "identifier": "9876543210"}, "")
	if rr.Code != 400 {
		t.Fatal("WhatsApp OTP sent without request consent")
	}
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]string{"channel": "email", "identifier": "unknown@example.com"}, "")
	if rr.Code != 202 || bytes.Contains(rr.Body.Bytes(), []byte("unknown@example.com")) {
		t.Fatal("unmatched identity response leaks membership")
	}
	if count(t, a, "SELECT count(*) FROM delivery_jobs WHERE purpose='pass_otp'") != 0 {
		t.Fatal("unknown identity got OTP")
	}
	a.Config.LiveDelivery = true
	rr = mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]any{"channel": "whatsapp", "identifier": "9876543210", "whatsapp_consent": true}, "")
	if rr.Code != 503 {
		t.Fatal("missing WhatsApp authentication template accepted")
	}
}

func TestMobilePassOTPProviderContract(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/email" {
			fmt.Fprint(w, `{"MessageID":"email-otp","ErrorCode":0}`)
		} else {
			fmt.Fprint(w, `{"messages":[{"id":"wa-otp"}]}`)
		}
	}))
	defer server.Close()
	a := &App{Config: Config{PostmarkAPIBase: server.URL, MetaAPIBase: server.URL, MetaOTPTemplate: "bio_pass_otp", MetaOTPLanguage: "en_US", MetaVersion: "v23.0", MetaPhoneID: "phone"}}
	result := a.sendPassOTP(t.Context(), job{ID: "otp-job", Channel: "whatsapp", Recipient: "+919876543210"}, "123456")
	if result.ID != "wa-otp" || result.Status != "accepted" {
		t.Fatalf("WhatsApp: %+v", result)
	}
	template := payload["template"].(map[string]any)
	if template["name"] != "bio_pass_otp" || len(template["components"].([]any)) != 2 {
		t.Fatal("OTP must use body and copy-code button, not the document pass template")
	}
	result = a.sendPassOTP(t.Context(), job{ID: "otp-job", Channel: "email", Recipient: "asha@example.com"}, "123456")
	if result.ID != "email-otp" || !strings.Contains(payload["TextBody"].(string), "123456") {
		t.Fatalf("email OTP: %+v", result)
	}
}

func TestMobilePassWorkerCancelsOldOTP(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "desk@example.com", "reviewer")
	if _, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(1), nil, sid); err != nil {
		t.Fatal(err)
	}
	issueMobileCode(t, a, "email", "asha@example.com", "")
	if _, err := a.DB.Exec(t.Context(), "UPDATE mobile_pass_challenges SET expires_at=$1", a.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := a.WorkOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if count(t, a, "SELECT count(*) FROM delivery_jobs WHERE purpose='pass_otp' AND status='cancelled'") != 1 {
		t.Fatal("expired OTP was delivered")
	}
}

func TestMobilePassWorkerSerializesResend(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "desk@example.com", "reviewer")
	if _, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(1), nil, sid); err != nil {
		t.Fatal(err)
	}
	issueMobileCode(t, a, "email", "asha@example.com", "")
	if _, err := a.DB.Exec(t.Context(), "DELETE FROM rate_limits"); err != nil {
		t.Fatal(err)
	}
	sending := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(sending)
		<-release
		fmt.Fprint(w, `{"MessageID":"otp-message","ErrorCode":0}`)
	}))
	defer server.Close()
	a.Config.LiveDelivery = true
	a.Config.PostmarkAPIBase = server.URL
	workDone := make(chan error, 1)
	go func() { workDone <- a.WorkOnce(t.Context()) }()
	select {
	case <-sending:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("worker did not reach provider")
	}
	requested := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		requested <- mobileRequest(a, "POST", "/api/v1/mobile/pass-access/challenges", map[string]string{"channel": "email", "identifier": "asha@example.com"}, "")
	}()
	var early *httptest.ResponseRecorder
	select {
	case early = <-requested:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-workDone; err != nil {
		t.Fatal(err)
	}
	if early != nil {
		t.Fatalf("resend invalidated code during delivery: %d", early.Code)
	}
	select {
	case rr := <-requested:
		if rr.Code != 202 {
			t.Fatalf("resend: %d %s", rr.Code, rr.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resend did not complete")
	}
}

func TestMobilePassConcurrentVerifyOnce(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "desk@example.com", "reviewer")
	if _, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(1), nil, sid); err != nil {
		t.Fatal(err)
	}
	challenge, code := issueMobileCode(t, a, "email", "asha@example.com", "")
	results := make(chan int, 8)
	for range 8 {
		go func() {
			results <- mobileRequest(a, "POST", "/api/v1/mobile/pass-access/verify", map[string]string{"challenge": challenge, "code": code}, "").Code
		}()
	}
	successes := 0
	for range 8 {
		result := <-results
		if result == 200 {
			successes++
		} else if result != 401 {
			t.Fatalf("unexpected concurrent result: %d", result)
		}
	}
	if successes != 1 || count(t, a, "SELECT count(*) FROM mobile_pass_sessions") != 1 {
		t.Fatal("OTP created multiple sessions")
	}
}
