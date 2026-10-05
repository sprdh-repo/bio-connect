package app

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/skip2/go-qrcode"
)

func TestOpsScanCodeExtractsBadgeTokenFromURLs(t *testing.T) {
	token := strings.Repeat("aZ0_-", 8) + "xyz"
	for input, want := range map[string]string{
		token:                                     token,
		"  " + token + "\n":                       token,
		"https://reg.example/p/" + token:          token,
		"https://reg.example/p/" + token + "/":    token,
		"https://reg.example/p/" + token + "?s=1": token,
		// A keyboard-wedge scanner on a mismatched layout can turn ":" and "/" into other characters.
		"https;--reg.example-p-" + token: token,
		"BC26-IN-0042":                   "BC26-IN-0042",
		"=UNKNOWN-BADGE":                 "=UNKNOWN-BADGE",
		"https://reg.example/p/short":    "https://reg.example/p/short",
	} {
		if got := opsScanCode(input); got != want {
			t.Errorf("opsScanCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOpsBadgeQROpensProfileAndEveryScanAcceptsIt(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Desk 1", "venue-passcode")
	qr := seedOpsPass(t, a, "ops-badge-url@example.com")
	badge := a.Config.BaseURL + "/p/" + qr
	day := opsDays[0].ID

	rr := c.request(http.MethodGet, "/api/v1/ops/qr/"+qr, nil)
	want, err := qrcode.Encode(badge, qrcode.Medium, 512)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK || !bytes.Equal(rr.Body.Bytes(), want) {
		t.Fatalf("badge QR must encode %q: status %d", badge, rr.Code)
	}
	if q, _ := qrcode.New("https://reg.bioconnect.kerala.gov.in/p/"+qr, qrcode.Medium); q.VersionNumber > 5 {
		t.Fatalf("production badge QR grew to version %d; it is printed at 1.95 cm", q.VersionNumber)
	}

	if rr := c.request(http.MethodGet, "/api/v1/ops/lookup?day="+day+"&code="+url.QueryEscape(badge), nil); rr.Code != http.StatusOK {
		t.Fatalf("lookup by badge URL: %d %s", rr.Code, rr.Body.String())
	}
	steps := []struct{ path, code string }{
		{"/api/v1/ops/check-in", badge},
		{"/api/v1/ops/badge/printed", badge},
		{"/api/v1/ops/check-out", badge},
		{"/api/v1/ops/check-out/undo", badge},
		{"/api/v1/ops/check-in/undo", badge},
		{"/api/v1/ops/check-in", "https;--localhost;8080-p-" + qr},
		{"/api/v1/ops/check-in", qr},
	}
	for _, s := range steps {
		body := map[string]any{"code": s.code, "day": day}
		if strings.HasSuffix(s.path, "/undo") {
			body["reason"] = "scanned the wrong badge"
		}
		rr := c.request(http.MethodPost, s.path, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s with %q: %d %s", s.path, s.code, rr.Code, rr.Body.String())
		}
	}
	var number string
	if err := a.DB.QueryRow(context.Background(), "SELECT number FROM passes WHERE qr_id=$1", qr).Scan(&number); err != nil {
		t.Fatal(err)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": number, "day": day}); rr.Code != http.StatusOK {
		t.Fatalf("typed pass number: %d %s", rr.Code, rr.Body.String())
	}

	created := c.request(http.MethodPost, "/api/v1/ops/points", map[string]any{"name": "Hall A", "mode": "enforce", "direction": "auto", "allowedCategories": []string{}, "requireCheckIn": true, "allowMultipleEntries": true})
	if created.Code != http.StatusCreated {
		t.Fatalf("create gate: %d %s", created.Code, created.Body.String())
	}
	pointID := decodeOpsResponse(t, created)["point"].(map[string]any)["id"].(string)
	for _, direction := range []string{"entry", "exit"} {
		out := decodeOpsResponse(t, c.request(http.MethodPost, "/api/v1/ops/points/"+pointID+"/scan", map[string]any{"code": badge, "day": day}))
		if out["allowed"] != true || out["direction"] != direction {
			t.Fatalf("gate %s by badge URL: %v", direction, out)
		}
	}
	unknown := a.Config.BaseURL + "/p/" + strings.Repeat("Q", 43)
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": unknown, "day": day}); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown badge URL check-in: %d", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind='check_in_denied' AND detail LIKE $1", strings.Repeat("Q", 43)+" | %"); n != 1 {
		t.Fatalf("rejected scans should log the token, not the URL: %d", n)
	}
}

func TestPublicProfileShowsOnlyBadgeDetails(t *testing.T) {
	a := mustApp(t)
	h := a.Handler()
	qr := seedOpsPass(t, a, "profile@example.com")
	get := func(path string) (int, string, http.Header) {
		c := &opsTestClient{t: t, h: h}
		rr := c.request(http.MethodGet, path, nil)
		return rr.Code, rr.Body.String(), rr.Header()
	}

	code, body, header := get("/p/" + qr)
	if code != http.StatusOK {
		t.Fatalf("profile status = %d", code)
	}
	for _, want := range []string{"Asha Nair", "Student", "Kerala University", "Registered attendee"} {
		if !strings.Contains(body, want) {
			t.Errorf("profile is missing %q", want)
		}
	}
	for _, private := range []string{"profile@example.com", "9876543210", qr} {
		if strings.Contains(body, private) {
			t.Errorf("profile leaks %q", private)
		}
	}
	if !strings.Contains(header.Get("X-Robots-Tag"), "noindex") || header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("profile must not be indexed or leak its URL: %v", header)
	}

	if code, body, _ := get("/p/" + strings.Repeat("Q", 43)); code != http.StatusNotFound || !strings.Contains(body, "Badge not found") {
		t.Fatalf("unknown badge: %d", code)
	}
	if code, _, _ := get("/p/not-a-token"); code != http.StatusNotFound {
		t.Fatalf("malformed badge: %d", code)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE passes SET revoked_at=now() WHERE qr_id=$1", qr); err != nil {
		t.Fatal(err)
	}
	if code, body, _ := get("/p/" + qr); code != http.StatusNotFound || strings.Contains(body, "Asha Nair") {
		t.Fatalf("revoked badge must not show its holder: %d", code)
	}
}
