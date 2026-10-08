package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func (c *opsTestClient) startKiosk() {
	c.t.Helper()
	rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/start", map[string]any{})
	if rr.Code != http.StatusOK {
		c.t.Fatalf("kiosk start: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		c.t.Fatal(err)
	}
	c.csrf, c.cookies = out.CSRF, rr.Result().Cookies()
}

func TestKioskSessionIsConfinedToKioskEndpoints(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Kiosk 1", "venue-passcode")
	staffCookies := c.cookies
	c.startKiosk()

	if me := decodeOpsResponse(t, c.request(http.MethodGet, "/api/v1/ops/me", nil)); me["kiosk"] != true || me["station"] != "Kiosk 1" {
		t.Fatalf("kiosk me = %v", me)
	}
	qr := seedOpsPass(t, a, "kiosk-confined@example.com")
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/ops/roster?day=" + opsDays[0].ID},
		{http.MethodGet, "/api/v1/ops/lookup?day=" + opsDays[0].ID + "&code=" + qr},
		{http.MethodGet, "/api/v1/ops/reports"},
		{http.MethodPost, "/api/v1/ops/check-in/undo"},
		{http.MethodPost, "/api/v1/ops/auth/logout"},
		{http.MethodPost, "/api/v1/ops/kiosk/start"},
	} {
		if rr := c.request(tc.method, tc.path, map[string]any{"code": qr, "day": opsDays[0].ID, "reason": "x"}); rr.Code != http.StatusForbidden {
			t.Errorf("%s %s from a kiosk = %d, want 403", tc.method, tc.path, rr.Code)
		}
	}
	// The staff session the kiosk was opened from no longer works.
	old := &opsTestClient{t: t, h: a.Handler(), cookies: staffCookies}
	if rr := old.request(http.MethodGet, "/api/v1/ops/reports", nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("replaced staff session status = %d", rr.Code)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/exit", map[string]any{"passcode": "wrong"}); rr.Code != http.StatusUnauthorized {
		t.Fatalf("exit with wrong passcode = %d", rr.Code)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/unlock", map[string]any{"passcode": "venue-passcode"}); rr.Code != http.StatusOK {
		t.Fatalf("unlock = %d %s", rr.Code, rr.Body.String())
	}
	rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/exit", map[string]any{"passcode": "venue-passcode"})
	if rr.Code != http.StatusOK {
		t.Fatalf("exit = %d %s", rr.Code, rr.Body.String())
	}
	c.csrf = decodeOpsResponse(t, rr)["csrf"].(string)
	c.cookies = rr.Result().Cookies()
	if rr := c.request(http.MethodGet, "/api/v1/ops/roster?day="+opsDays[0].ID, nil); rr.Code != http.StatusOK {
		t.Fatalf("roster after kiosk exit = %d", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind IN ('kiosk_open','kiosk_close') AND station='Kiosk 1'"); n != 2 {
		t.Fatalf("kiosk open/close audit rows = %d", n)
	}
}

func TestKioskPrintChecksInOnceAndLimitsReprints(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	a.Now = func() time.Time { return time.Date(2026, 10, 8, 9, 0, 0, 0, india) }
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Kiosk 1", "venue-passcode")
	c.startKiosk()
	qr := seedOpsPass(t, a, "kiosk-print@example.com")
	var number string
	if err := a.DB.QueryRow(t.Context(), "SELECT number FROM passes WHERE qr_id=$1", qr).Scan(&number); err != nil {
		t.Fatal(err)
	}

	// Typed pass numbers are guessable, so the kiosk takes only the QR.
	for _, path := range []string{"scan", "print", "check-in"} {
		if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/"+path, map[string]any{"code": number}); rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("kiosk %s by pass number = %d", path, rr.Code)
		}
	}
	unknown := "QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ"
	if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/scan", map[string]any{"code": unknown}); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown QR = %d", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind='check_in_denied' AND station='Kiosk 1'"); n != 4 {
		t.Fatalf("denied kiosk scans logged = %d", n)
	}

	scan := decodeOpsResponse(t, c.request(http.MethodPost, "/api/v1/ops/kiosk/scan", map[string]any{"code": a.badgeURL(qr)}))
	if scan["status"] != "print" || scan["day"] != opsDays[0].ID {
		t.Fatalf("first scan = %v", scan)
	}
	person := scan["person"].(map[string]any)
	if _, leaked := person["email"]; leaked {
		t.Fatalf("kiosk response carries contact details: %v", person)
	}
	if _, leaked := person["phone"]; leaked {
		t.Fatalf("kiosk response carries contact details: %v", person)
	}
	// A first badge has to be printed; check-in alone is refused.
	if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/check-in", map[string]any{"code": qr}); rr.Code != http.StatusConflict {
		t.Fatalf("check-in before print = %d", rr.Code)
	}

	printed := decodeOpsResponse(t, c.request(http.MethodPost, "/api/v1/ops/kiosk/print", map[string]any{"code": qr}))
	if printed["firstCheckIn"] != true {
		t.Fatalf("print = %v", printed)
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE event_day=$1 AND checked_in_by='Kiosk 1'", opsDays[0].ID); n != 1 {
		t.Fatalf("attendance after print = %d", n)
	}
	if got := decodeOpsResponse(t, c.request(http.MethodPost, "/api/v1/ops/kiosk/scan", map[string]any{"code": qr}))["status"]; got != "done" {
		t.Fatalf("scan after print = %v", got)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/print", map[string]any{"code": qr}); rr.Code != http.StatusConflict {
		t.Fatalf("second print without retry = %d", rr.Code)
	}
	for i := 0; i < kioskReprintLimit-1; i++ {
		if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/print", map[string]any{"code": qr, "retry": true}); rr.Code != http.StatusOK {
			t.Fatalf("reprint %d = %d %s", i, rr.Code, rr.Body.String())
		}
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/kiosk/print", map[string]any{"code": qr, "retry": true}); rr.Code != http.StatusConflict {
		t.Fatalf("reprint past the limit = %d", rr.Code)
	}
	// Another kiosk cannot reprint a badge it did not print.
	other := &opsTestClient{t: t, h: a.Handler()}
	other.login("Kiosk 2", "venue-passcode")
	other.startKiosk()
	if rr := other.request(http.MethodPost, "/api/v1/ops/kiosk/print", map[string]any{"code": qr, "retry": true}); rr.Code != http.StatusConflict {
		t.Fatalf("reprint from another kiosk = %d", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind='check_in'"); n != 1 {
		t.Fatalf("check_in audit rows = %d", n)
	}

	// Day two: the attendee already has a badge, so the kiosk only checks them in.
	a.Now = func() time.Time { return time.Date(2026, 10, 9, 9, 0, 0, 0, india) }
	if got := decodeOpsResponse(t, c.request(http.MethodPost, "/api/v1/ops/kiosk/scan", map[string]any{"code": qr}))["status"]; got != "check-in" {
		t.Fatalf("day-two scan = %v", got)
	}
	for i, want := range []bool{false, true} {
		out := decodeOpsResponse(t, c.request(http.MethodPost, "/api/v1/ops/kiosk/check-in", map[string]any{"code": qr}))
		if out["repeat"] != want {
			t.Fatalf("day-two check-in %d = %v", i, out)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE event_day=$1", opsDays[1].ID); n != 1 {
		t.Fatalf("day-two attendance = %d", n)
	}
}

func TestKioskTreatsDeskPrintedBadgeAsPrinted(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	a.Now = func() time.Time { return time.Date(2026, 10, 8, 9, 0, 0, 0, india) }
	desk := &opsTestClient{t: t, h: a.Handler()}
	desk.login("Desk 1", "venue-passcode")
	qr := seedOpsPass(t, a, "kiosk-desk@example.com")
	if rr := desk.request(http.MethodPost, "/api/v1/ops/badge/printed", map[string]any{"code": qr, "day": opsDays[0].ID}); rr.Code != http.StatusOK {
		t.Fatalf("desk print = %d", rr.Code)
	}
	kiosk := &opsTestClient{t: t, h: a.Handler()}
	kiosk.login("Kiosk 1", "venue-passcode")
	kiosk.startKiosk()
	if got := decodeOpsResponse(t, kiosk.request(http.MethodPost, "/api/v1/ops/kiosk/scan", map[string]any{"code": qr}))["status"]; got != "check-in" {
		t.Fatalf("scan of a desk-printed badge = %v", got)
	}
	if rr := kiosk.request(http.MethodPost, "/api/v1/ops/kiosk/print", map[string]any{"code": qr, "retry": true}); rr.Code != http.StatusConflict {
		t.Fatalf("kiosk reprint of a desk badge = %d", rr.Code)
	}
}

// The Android kiosk app talks to the page over an origin-locked web message
// channel; these are the names and fields it depends on.
func TestKioskPageKeepsTheAppBridgeContract(t *testing.T) {
	script, err := resources.ReadFile("web/kiosk.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"window.BioConnectKiosk",
		"type:info?'status':'hello'",
		"send({type:'print',jobId,png:",
		"widthDots:badge.width,heightDots:badge.height",
		"crypto.randomUUID()",
		"m.type==='info'",
		"m.type==='printed'",
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("kiosk.js no longer contains %q", want)
		}
	}
}
