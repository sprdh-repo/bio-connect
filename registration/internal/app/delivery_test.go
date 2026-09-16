package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// A queued 'registration' email job, ready for the worker to pick up.
func queuedEmail(t *testing.T, a *App) (rid, jobID string) {
	t.Helper()
	ctx := context.Background()
	rid, _, err := a.Create(ctx, delegateInput("student"), key(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(ctx,
		"SELECT id FROM delivery_jobs WHERE registration_id=$1 AND purpose='registration' AND channel='email'", rid).Scan(&jobID); err != nil {
		t.Fatalf("no registration email queued: %v", err)
	}
	return
}

func jobStatus(t *testing.T, a *App, jobID string) (st, code string, attempts int) {
	t.Helper()
	if err := a.DB.QueryRow(context.Background(),
		"SELECT status,error_code,attempts FROM delivery_jobs WHERE id=$1", jobID).Scan(&st, &code, &attempts); err != nil {
		t.Fatal(err)
	}
	return
}

func TestFakeProviderDeliversInNonLiveMode(t *testing.T) {
	a := mustApp(t)
	_, jobID := queuedEmail(t, a)
	if err := a.WorkOnce(context.Background()); err != nil {
		t.Fatalf("WorkOnce: %v", err)
	}
	if st, _, _ := jobStatus(t, a, jobID); st != "delivered" {
		t.Fatalf("fake provider job status = %s, want delivered", st)
	}
}

func TestProviderOutcomesMapToJobStatus(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus string
		wantRetry  bool
	}{
		{"accepted", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"MessageID": "pm-ok", "ErrorCode": 0})
		}, "accepted", false},
		{"postmark_error_code", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"ErrorCode": 406, "Message": "inactive recipient"})
		}, "failed", false},
		{"http_422", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(422) }, "failed", false},
		{"http_500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, "uncertain", false},
		{"http_429_retries", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }, "queued", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := mustApp(t)
			srv := httptest.NewServer(c.handler)
			defer srv.Close()
			a.Config.LiveDelivery = true
			a.Config.PostmarkToken = "test-token"
			a.Config.SenderAddress = "events@zinvos.example"
			a.Config.SenderName = "Zinvos Events"
			a.Config.PostmarkAPIBase = srv.URL
			_, jobID := queuedEmail(t, a)
			if err := a.WorkOnce(context.Background()); err != nil {
				t.Fatalf("WorkOnce: %v", err)
			}
			st, _, attempts := jobStatus(t, a, jobID)
			if st != c.wantStatus {
				t.Fatalf("status = %s, want %s", st, c.wantStatus)
			}
			if c.wantRetry && attempts != 1 {
				t.Fatalf("attempts = %d, want 1 before backoff", attempts)
			}
		})
	}
}

func TestTransportErrorIsUncertain(t *testing.T) {
	a := mustApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now
	a.Config.LiveDelivery = true
	a.Config.PostmarkToken = "t"
	a.Config.SenderAddress = "a@b.co"
	a.Config.SenderName = "Z"
	a.Config.PostmarkAPIBase = url
	_, jobID := queuedEmail(t, a)
	if err := a.WorkOnce(context.Background()); err != nil {
		t.Fatalf("WorkOnce: %v", err)
	}
	if st, code, _ := jobStatus(t, a, jobID); st != "uncertain" {
		t.Fatalf("transport failure status = %s (%s), want uncertain", st, code)
	}
}

func TestStuckSendingJobBecomesUncertain(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, jobID := queuedEmail(t, a)
	_, err := a.DB.Exec(ctx,
		"UPDATE delivery_jobs SET status='sending',claimed_at=now()-interval '10 minutes' WHERE id=$1", jobID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WorkOnce(ctx); err != nil {
		t.Fatalf("WorkOnce: %v", err)
	}
	if st, code, _ := jobStatus(t, a, jobID); st != "uncertain" || code != "worker_lease_expired" {
		t.Fatalf("stuck job = %s/%s, want uncertain/worker_lease_expired", st, code)
	}
	// Retrying an uncertain job requires an explicit duplicate-send acknowledgement.
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rr := httptest.NewRecorder()
	b, _ := json.Marshal(map[string]any{"id": jobID})
	req := httptest.NewRequest("POST", "/api/v1/admin/retry", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	a.retryJob(rr, req, principal{ID: sid, Role: "reviewer"})
	if rr.Code != 400 {
		t.Fatalf("retry without confirmation returned %d, want 400", rr.Code)
	}
	rr2 := httptest.NewRecorder()
	b2, _ := json.Marshal(map[string]any{"id": jobID, "confirm_uncertain": true, "note": "checked Postmark activity, not delivered"})
	req2 := httptest.NewRequest("POST", "/api/v1/admin/retry", bytes.NewReader(b2))
	req2.Header.Set("Content-Type", "application/json")
	a.retryJob(rr2, req2, principal{ID: sid, Role: "reviewer"})
	if rr2.Code != 200 {
		t.Fatalf("confirmed retry returned %d: %s", rr2.Code, rr2.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='registration'", rid); n != 2 {
		t.Fatalf("retry did not create a fresh job (have %d)", n)
	}
}

// --- webhooks ---------------------------------------------------------

func seedAcceptedJob(t *testing.T, a *App, channel, providerID string) string {
	t.Helper()
	ctx := context.Background()
	rid, _, _ := a.Create(ctx, delegateInput("student"), key(1), nil)
	jid := id()
	_, err := a.DB.Exec(ctx, `INSERT INTO delivery_jobs(id,registration_id,purpose,channel,recipient,payload_cipher,dedupe_key,status,provider_id,attempts)
		VALUES($1,$2,'registration',$3,'x@example.com','','seed-'||$1,'accepted',$4,1)`, jid, rid, channel, providerID)
	if err != nil {
		t.Fatal(err)
	}
	return jid
}

func TestPostmarkWebhookAuthAndIdempotency(t *testing.T) {
	a := mustApp(t)
	a.Config.WebhookUser = "hook"
	a.Config.WebhookPassword = "s3cret"
	jid := seedAcceptedJob(t, a, "email", "pm-123")
	payload := `{"RecordType":"Delivery","MessageID":"pm-123","Metadata":{"application":"bioconnect4"}}`

	// Wrong credentials are rejected.
	bad := httptest.NewRecorder()
	br := httptest.NewRequest("POST", "/api/v1/webhooks/postmark", strings.NewReader(payload))
	br.SetBasicAuth("hook", "wrong")
	a.postmarkWebhook(bad, br)
	if bad.Code != 401 {
		t.Fatalf("bad-auth webhook returned %d", bad.Code)
	}

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/webhooks/postmark", strings.NewReader(payload))
		req.SetBasicAuth("hook", "s3cret")
		a.postmarkWebhook(rr, req)
		if rr.Code != 200 {
			t.Fatalf("delivery webhook %d returned %d", i, rr.Code)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM webhook_events WHERE provider_id='pm-123'"); n != 1 {
		t.Fatalf("%d webhook_events rows for duplicated event, want 1", n)
	}
	if st, _, _ := jobStatus(t, a, jid); st != "delivered" {
		t.Fatalf("job status after delivery webhook = %s", st)
	}
}

func TestMetaWebhookSignatureAndVerification(t *testing.T) {
	a := mustApp(t)
	a.Config.MetaAppSecret = "meta-secret"
	a.Config.MetaPhoneID = "PHONE1"
	a.Config.MetaVerifyToken = "verify-me"
	jid := seedAcceptedJob(t, a, "whatsapp", "wamid.1")

	// GET verification handshake.
	gv := httptest.NewRecorder()
	gr := httptest.NewRequest("GET", "/api/v1/webhooks/meta?hub.mode=subscribe&hub.verify_token=verify-me&hub.challenge=42", nil)
	a.metaVerify(gv, gr)
	if gv.Code != 200 || strings.TrimSpace(gv.Body.String()) != "42" {
		t.Fatalf("verification handshake: %d %q", gv.Code, gv.Body.String())
	}

	event := `{"entry":[{"changes":[{"value":{"metadata":{"phone_number_id":"PHONE1"},"statuses":[{"id":"wamid.1","status":"delivered","timestamp":"1700000000"}]}}]}]}`
	// Bad signature rejected.
	nr := httptest.NewRecorder()
	badReq := httptest.NewRequest("POST", "/api/v1/webhooks/meta", strings.NewReader(event))
	badReq.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	a.metaWebhook(nr, badReq)
	if nr.Code != 401 {
		t.Fatalf("unsigned meta webhook returned %d", nr.Code)
	}
	// Correct signature, delivered twice.
	mac := hmac.New(sha256.New, []byte("meta-secret"))
	mac.Write([]byte(event))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/webhooks/meta", strings.NewReader(event))
		req.Header.Set("X-Hub-Signature-256", sig)
		a.metaWebhook(rr, req)
		if rr.Code != 200 {
			t.Fatalf("signed meta webhook %d returned %d", i, rr.Code)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM webhook_events WHERE channel='whatsapp' AND provider_id='wamid.1'"); n != 1 {
		t.Fatalf("%d whatsapp webhook rows, want 1", n)
	}
	if st, _, _ := jobStatus(t, a, jid); st != "delivered" {
		t.Fatalf("whatsapp job status = %s", st)
	}
}

// --- HTTP surface: headers, auth, permissions -------------------------

func TestHandlerSecurityHeadersAndCORS(t *testing.T) {
	a := mustApp(t)
	h := a.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/categories", nil))
	if rr.Code != 200 {
		t.Fatalf("categories returned %d", rr.Code)
	}
	if csp := rr.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("missing CSP: %q", csp)
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing X-Frame-Options")
	}

	// Cross-origin state-changing request is refused before handler logic.
	xo := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/registrations", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	h.ServeHTTP(xo, req)
	if xo.Code != 403 {
		t.Fatalf("cross-origin POST returned %d, want 403", xo.Code)
	}
}

// staffLogin signs a staff account in through the real HTTP login flow and
// returns the session and CSRF cookies issued, for tests that then need an
// authenticated admin request.
func staffLogin(t *testing.T, h http.Handler, email, secret string, at time.Time) (session, csrf string) {
	t.Helper()
	code, _ := totp.GenerateCode(secret, at)
	body, _ := json.Marshal(map[string]string{"email": email, "password": "correct-horse-battery-staple", "code": code})
	lr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(lr, req)
	if lr.Code != 200 {
		t.Fatalf("login for %s returned %d: %s", email, lr.Code, lr.Body.String())
	}
	for _, c := range lr.Result().Cookies() {
		switch c.Name {
		case "bc_session":
			session = c.Value
		case "bc_csrf":
			csrf = c.Value
		}
	}
	return
}

func TestAdminRequiresSessionAndRoleSeparation(t *testing.T) {
	a := mustApp(t)
	// Pinned for deterministic TOTP, but near real time so DB now()-based
	// session expiry stays valid through the test.
	fixed := time.Now().UTC().Truncate(time.Minute)
	a.Now = func() time.Time { return fixed }
	h := a.Handler()

	revID, revSecret := addStaff(t, a, "rev@bioconnect.test", "reviewer")
	_, mgrSecret := addStaff(t, a, "mgr@bioconnect.test", "manager")
	_ = revID

	// No session.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/admin/registrations", nil))
	if rr.Code != 401 {
		t.Fatalf("unauthenticated admin call returned %d", rr.Code)
	}

	revSession, revCSRF := staffLogin(t, h, "rev@bioconnect.test", revSecret, fixed)

	// TOTP replay within the same step is refused.
	code, _ := totp.GenerateCode(revSecret, fixed)
	replay, _ := json.Marshal(map[string]string{"email": "rev@bioconnect.test", "password": "correct-horse-battery-staple", "code": code})
	rp := httptest.NewRecorder()
	rpReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(replay))
	rpReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rp, rpReq)
	if rp.Code == 200 {
		t.Fatal("TOTP code was accepted twice in the same period")
	}

	authed := func(method, path, session, csrf string, body []byte) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		var rdr *bytes.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		} else {
			rdr = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, rdr)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		h.ServeHTTP(rr, req)
		return rr
	}

	// Reviewer can list registrations...
	if rr := authed("GET", "/api/v1/admin/registrations", revSession, "", nil); rr.Code != 200 {
		t.Fatalf("reviewer registrations returned %d", rr.Code)
	}
	// ...but cannot administer staff accounts.
	newAcct, _ := json.Marshal(map[string]string{"email": "x@bioconnect.test", "password": "correct-horse-battery-staple", "role": "reviewer"})
	if rr := authed("POST", "/api/v1/admin/staff", revSession, revCSRF, newAcct); rr.Code != 403 {
		t.Fatalf("reviewer creating staff returned %d, want 403", rr.Code)
	}

	mgrSession, mgrCSRF := staffLogin(t, h, "mgr@bioconnect.test", mgrSecret, fixed)
	// Manager can administer staff...
	if rr := authed("POST", "/api/v1/admin/staff", mgrSession, mgrCSRF, newAcct); rr.Code != 201 {
		t.Fatalf("manager creating staff returned %d: %s", rr.Code, rr.Body.String())
	}
	// ...but cannot review registrations.
	if rr := authed("GET", "/api/v1/admin/registrations", mgrSession, "", nil); rr.Code != 403 {
		t.Fatalf("manager listing registrations returned %d, want 403", rr.Code)
	}
	// Missing CSRF on a state change is refused.
	if rr := authed("POST", "/api/v1/admin/staff", mgrSession, "", newAcct); rr.Code != 401 && rr.Code != 403 {
		t.Fatalf("CSRF-less POST returned %d", rr.Code)
	}
}

func TestClientPeerBehindProxyChain(t *testing.T) {
	a := &App{Config: Config{TrustedProxyCIDR: "172.30.0.0/16"}}
	mk := func(remote, hdr string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if hdr != "" {
			r.Header.Set("X-BioConnect-Client-IP", hdr)
		}
		return r
	}
	// Trusted proxy: take the left-most forwarded address.
	if got := a.clientPeer(mk("172.30.44.2:5000", "203.0.113.9, 130.176.0.10")); got != "203.0.113.9" {
		t.Fatalf("proxied client ip = %q, want 203.0.113.9", got)
	}
	// Untrusted source: ignore the header entirely.
	if got := a.clientPeer(mk("198.51.100.7:40000", "203.0.113.9")); got != "198.51.100.7" {
		t.Fatalf("spoofable header honoured from untrusted peer: %q", got)
	}
	// Trusted proxy, no/garbage header: fall back to the socket peer.
	if got := a.clientPeer(mk("172.30.44.2:5000", "not-an-ip")); got != "172.30.44.2" {
		t.Fatalf("garbage header = %q, want 172.30.44.2", got)
	}
}

func TestPrivateFileAccessIsScopedToRegistration(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	ridA, tokA, _ := a.Create(ctx, delegateInput("student"), key(1), nil)
	fid := addFile(t, a, ridA, "receipt", tinyPNG(t))
	inB := delegateInput("faculty")
	inB.Email, inB.Attendees[0].Email = "b@example.com", "b@example.com"
	ridB, tokB, _ := a.Create(ctx, inB, key(2), nil)

	h := a.Handler()
	// Owner can download.
	ok := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/registrations/%s/files/%s", ridA, fid), nil)
	req.Header.Set("Authorization", "Bearer "+tokA)
	h.ServeHTTP(ok, req)
	if ok.Code != 200 {
		t.Fatalf("owner file download returned %d", ok.Code)
	}
	// Another registration's token cannot reach it.
	no := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/registrations/%s/files/%s", ridB, fid), nil)
	req2.Header.Set("Authorization", "Bearer "+tokB)
	h.ServeHTTP(no, req2)
	if no.Code == 200 {
		t.Fatal("cross-registration file access succeeded")
	}
}

func multipartRegistration(t *testing.T, in RegistrationInput, logo []byte) *http.Request {
	t.Helper()
	payload, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("payload", string(payload)); err != nil {
		t.Fatal(err)
	}
	if logo != nil {
		fw, err := w.CreateFormFile("logo", "logo.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(logo); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/registrations", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// The registration and its logo must land in the same request: this is the
// fix for the bug where a dropped connection between two separate calls left
// a registration approved-pending with no logo to show for it.
func TestExhibitorRegistrationRequiresLogoInTheSameMultipartRequest(t *testing.T) {
	a := mustApp(t)
	h := a.Handler()

	req := multipartRegistration(t, exhibitorInput("table", 2), tinyPNG(t))
	req.Header.Set("Idempotency-Key", key(1))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("multipart exhibitor registration returned %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		ID              string `json:"id"`
		ManagementToken string `json:"management_token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ID == "" || out.ManagementToken == "" {
		t.Fatalf("missing id/token in response: %s", rr.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM files WHERE registration_id=$1 AND kind='logo'", out.ID); n != 1 {
		t.Fatalf("logo files for registration = %d, want 1", n)
	}

	// The server enforces this itself; it is not just a client-side form check.
	req2 := multipartRegistration(t, exhibitorInput("table", 2), nil)
	req2.Header.Set("Idempotency-Key", key(2))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != 400 {
		t.Fatalf("exhibitor registration without a logo returned %d, want 400: %s", rr2.Code, rr2.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM registrations"); n != 1 {
		t.Fatalf("registrations after rejected submission = %d, want 1", n)
	}
}

func TestAdminCanUploadExhibitorLogoOnRegistrantsBehalf(t *testing.T) {
	a := mustApp(t)
	fixed := time.Now().UTC().Truncate(time.Minute)
	a.Now = func() time.Time { return fixed }
	h := a.Handler()
	ctx := context.Background()

	sid, secret := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, err := a.Create(ctx, exhibitorInput("table", 2), key(1), tinyPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the real case this exists for: the exhibitor's own upload never
	// completed, and staff need to attach the logo to record their payment.
	if _, err := a.DB.Exec(ctx, "DELETE FROM files WHERE registration_id=$1 AND kind='logo'", rid); err != nil {
		t.Fatal(err)
	}

	session, csrfTok := staffLogin(t, h, "reviewer@bioconnect.test", secret, fixed)
	uploadPath := "/api/v1/admin/registrations/" + rid + "/files?kind=logo"

	// Missing CSRF token is refused, same as any other admin state change.
	noCSRF := httptest.NewRecorder()
	noCSRFReq := httptest.NewRequest("POST", uploadPath, bytes.NewReader(tinyPNG(t)))
	noCSRFReq.Header.Set("Content-Type", "application/octet-stream")
	noCSRFReq.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
	h.ServeHTTP(noCSRF, noCSRFReq)
	if noCSRF.Code != 401 {
		t.Fatalf("admin upload without CSRF token returned %d, want 401", noCSRF.Code)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", uploadPath, bytes.NewReader(tinyPNG(t)))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
	req.Header.Set("X-CSRF-Token", csrfTok)
	h.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("admin logo upload returned %d: %s", rr.Code, rr.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM files WHERE registration_id=$1 AND kind='logo'", rid); n != 1 {
		t.Fatalf("logo files after admin upload = %d, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND staff_id=$2 AND action='logo_uploaded_by_staff'", rid, sid); n != 1 {
		t.Fatalf("staff logo upload audit rows = %d, want 1", n)
	}
}
