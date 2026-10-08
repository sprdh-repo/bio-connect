package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type opsTestClient struct {
	t       *testing.T
	h       http.Handler
	cookies []*http.Cookie
	csrf    string
}

func (c *opsTestClient) request(method, path string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	c.h.ServeHTTP(rr, req)
	return rr
}

func (c *opsTestClient) login(station, passcode string) {
	c.t.Helper()
	rr := c.request(http.MethodPost, "/api/v1/ops/auth/start", map[string]any{"station": station, "passcode": passcode})
	if rr.Code != http.StatusOK {
		c.t.Fatalf("login: status %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		c.t.Fatal(err)
	}
	c.csrf = out.CSRF
	c.cookies = rr.Result().Cookies()
}

func seedOpsPass(t *testing.T, a *App, email string) string {
	t.Helper()
	in := delegateInput("student")
	in.Email, in.Attendees[0].Email = email, email
	rid, err := a.StaffCreate(context.Background(), staffInput(in, "complimentary"), randomToken(), nil, "ops-system")
	if err != nil {
		t.Fatalf("seed pass: %v", err)
	}
	var qr string
	if err := a.DB.QueryRow(context.Background(), "SELECT qr_id FROM passes WHERE registration_id=$1", rid).Scan(&qr); err != nil {
		t.Fatal(err)
	}
	return qr
}

func decodeOpsResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rr.Body.String(), err)
	}
	return out
}

func TestOpsAuthenticationAndDailyAttendance(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}

	if rr := c.request(http.MethodGet, "/api/v1/ops/reports", nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous report status = %d", rr.Code)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/auth/start", map[string]any{"station": "Desk 1", "passcode": "wrong"}); rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong passcode status = %d", rr.Code)
	}
	c.login("Desk 1", "venue-passcode")

	qr := seedOpsPass(t, a, "ops-attendance@example.com")
	savedCSRF := c.csrf
	c.csrf = ""
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": qr, "day": opsDays[0].ID}); rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing csrf status = %d: %s", rr.Code, rr.Body.String())
	}
	c.csrf = savedCSRF

	for i := 0; i < 2; i++ {
		rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": qr, "day": opsDays[0].ID})
		if rr.Code != http.StatusOK {
			t.Fatalf("check-in %d: %d %s", i, rr.Code, rr.Body.String())
		}
		if got := decodeOpsResponse(t, rr)["repeat"]; got != (i == 1) {
			t.Fatalf("check-in %d repeat = %v", i, got)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE event_day=$1", opsDays[0].ID); n != 1 {
		t.Fatalf("day-one attendance rows = %d", n)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": qr, "day": opsDays[1].ID}); rr.Code != http.StatusOK {
		t.Fatalf("independent day check-in: %d %s", rr.Code, rr.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE attendee_id=(SELECT attendee_id FROM passes WHERE qr_id=$1)", qr); n != 2 {
		t.Fatalf("daily attendance rows = %d", n)
	}

	if rr := c.request(http.MethodPost, "/api/v1/ops/check-out", map[string]any{"code": qr, "day": opsDays[0].ID}); rr.Code != http.StatusOK {
		t.Fatalf("checkout: %d %s", rr.Code, rr.Body.String())
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-out/undo", map[string]any{"code": qr, "day": opsDays[0].ID, "reason": "Scanned wrong lane"}); rr.Code != http.StatusOK {
		t.Fatalf("undo checkout: %d %s", rr.Code, rr.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE checked_out_at IS NOT NULL"); n != 0 {
		t.Fatalf("checked-out rows after undo = %d", n)
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in/undo", map[string]any{"code": qr, "day": opsDays[0].ID, "reason": "Duplicate desk record"}); rr.Code != http.StatusOK {
		t.Fatalf("undo check-in: %d %s", rr.Code, rr.Body.String())
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": qr, "day": opsDays[0].ID}); rr.Code != http.StatusOK {
		t.Fatalf("check-in after undo: %d %s", rr.Code, rr.Body.String())
	}
}

func TestOpsActivityRetainsSuccessfulRepeatedAndRejectedScans(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("=Audit Desk", "venue-passcode")

	checkedInQR := seedOpsPass(t, a, "ops-audit-success@example.com")
	notCheckedInQR := seedOpsPass(t, a, "ops-audit-rejected@example.com")
	day := opsDays[0].ID
	requests := []struct {
		path string
		code string
		want int
	}{
		{"/api/v1/ops/check-in", "=UNKNOWN-BADGE", http.StatusNotFound},
		{"/api/v1/ops/check-in", checkedInQR, http.StatusOK},
		{"/api/v1/ops/check-in", checkedInQR, http.StatusOK},
		{"/api/v1/ops/check-out", notCheckedInQR, http.StatusConflict},
	}
	for _, scan := range requests {
		rr := c.request(http.MethodPost, scan.path, map[string]any{"code": scan.code, "day": day})
		if rr.Code != scan.want {
			t.Fatalf("%s %q: status %d, want %d: %s", scan.path, scan.code, rr.Code, scan.want, rr.Body.String())
		}
	}

	rr := c.request(http.MethodGet, "/api/v1/ops/activity?day="+day, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("activity: %d %s", rr.Code, rr.Body.String())
	}
	activity, ok := decodeOpsResponse(t, rr)["activity"].([]any)
	if !ok || len(activity) != 4 {
		t.Fatalf("activity entries = %#v", activity)
	}
	byKind := map[string]map[string]any{}
	for _, raw := range activity {
		item := raw.(map[string]any)
		byKind[item["kind"].(string)] = item
	}
	for kind, outcome := range map[string]string{
		"check_in":         "success",
		"repeat_check_in":  "repeat",
		"check_in_denied":  "rejected",
		"check_out_denied": "rejected",
	} {
		item := byKind[kind]
		if item == nil {
			t.Errorf("activity is missing %s", kind)
			continue
		}
		if item["outcome"] != outcome || item["station"] != "=Audit Desk" {
			t.Errorf("%s activity = %#v", kind, item)
		}
	}
	unknown := byKind["check_in_denied"]
	if unknown["reference"] != "=UNKNOWN-BADGE" || unknown["detail"] != "No approved pass matches that code." {
		t.Errorf("unknown badge activity = %#v", unknown)
	}
	notCheckedIn := byKind["check_out_denied"]
	if notCheckedIn["name"] == "" || notCheckedIn["reference"] == "" || notCheckedIn["detail"] != "This attendee has not checked in today." {
		t.Errorf("rejected checkout activity = %#v", notCheckedIn)
	}

	export := c.request(http.MethodGet, "/api/v1/ops/activity/export?day="+day, nil)
	if export.Code != http.StatusOK {
		t.Fatalf("activity export: %d %s", export.Code, export.Body.String())
	}
	if disposition := export.Header().Get("Content-Disposition"); !strings.Contains(disposition, "Bio-Connect-operations-audit-"+day+".csv") {
		t.Errorf("activity export disposition = %q", disposition)
	}
	csv := export.Body.String()
	for _, want := range []string{"rejected", "repeat_check_in", "No approved pass matches that code.", "'=UNKNOWN-BADGE", "'=Audit Desk"} {
		if !strings.Contains(csv, want) {
			t.Errorf("activity CSV does not contain %q:\n%s", want, csv)
		}
	}
}

func TestOpsGateAllowsExitAfterRulesChange(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Hall gate", "venue-passcode")
	qr := seedOpsPass(t, a, "ops-gate@example.com")
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": qr, "day": opsDays[0].ID}); rr.Code != http.StatusOK {
		t.Fatalf("check-in: %s", rr.Body.String())
	}
	if rr := c.request(http.MethodPost, "/api/v1/ops/check-in", map[string]any{"code": qr, "day": opsDays[1].ID}); rr.Code != http.StatusOK {
		t.Fatalf("day-two check-in: %s", rr.Body.String())
	}

	created := c.request(http.MethodPost, "/api/v1/ops/points", map[string]any{
		"name": "Innovation Hall", "mode": "enforce", "direction": "auto",
		"allowedCategories": []string{"student"}, "allowedDays": []string{opsDays[0].ID, opsDays[1].ID},
		"capacity": 1, "requireCheckIn": true, "allowMultipleEntries": true,
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create gate: %d %s", created.Code, created.Body.String())
	}
	point := decodeOpsResponse(t, created)["point"].(map[string]any)
	pointID := point["id"].(string)
	entry := c.request(http.MethodPost, "/api/v1/ops/points/"+pointID+"/scan", map[string]any{"code": qr, "day": opsDays[0].ID})
	if entry.Code != http.StatusOK || decodeOpsResponse(t, entry)["allowed"] != true {
		t.Fatalf("entry: %d %s", entry.Code, entry.Body.String())
	}
	dayTwoEntry := c.request(http.MethodPost, "/api/v1/ops/points/"+pointID+"/scan", map[string]any{"code": qr, "day": opsDays[1].ID})
	dayTwo := decodeOpsResponse(t, dayTwoEntry)
	if dayTwoEntry.Code != http.StatusOK || dayTwo["allowed"] != true || dayTwo["direction"] != "entry" {
		t.Fatalf("independent day entry: %d %s", dayTwoEntry.Code, dayTwoEntry.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE access_points SET active=false,allowed_categories=ARRAY['faculty'] WHERE id=$1", pointID); err != nil {
		t.Fatal(err)
	}
	exit := c.request(http.MethodPost, "/api/v1/ops/points/"+pointID+"/scan", map[string]any{"code": qr, "day": opsDays[0].ID})
	out := decodeOpsResponse(t, exit)
	if exit.Code != http.StatusOK || out["allowed"] != true || out["direction"] != "exit" {
		t.Fatalf("exit after rule change: %d %s", exit.Code, exit.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM access_scans WHERE access_point_id=$1", pointID); n != 3 {
		t.Fatalf("retained access decisions = %d", n)
	}
}

func TestOpsPaidSpotRegistrationVerifiesAndDeduplicatesReference(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Spot desk", "venue-passcode")
	categories, err := a.categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var amount int64
	for _, category := range categories {
		if category.ID == "student" {
			amount = category.PayablePaise
		}
	}
	body := map[string]any{
		"day": opsDays[0].ID, "categoryId": "student", "name": "Maya Das",
		"email": "maya.spot@example.com", "phone": "+919876543210", "designation": "Researcher",
		"institution": "Bio Lab", "payment": "paid", "paymentReference": "SBI-SPOT-0042",
		"paymentDate": today(a), "amountPaise": amount,
	}
	created := c.request(http.MethodPost, "/api/v1/ops/spot-register", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("spot registration: %d %s", created.Code, created.Body.String())
	}
	person := decodeOpsResponse(t, created)["person"].(map[string]any)
	if person["checkedIn"] != true {
		t.Fatalf("spot registration was not checked in: %s", created.Body.String())
	}
	// Consent is given in person at the desk, so the pass goes out on both channels.
	for channel, to := range map[string]string{"email": "maya.spot@example.com", "whatsapp": "+919876543210"} {
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE purpose='pass' AND channel=$1 AND recipient=$2", channel, to); n != 1 {
			t.Fatalf("spot %s deliveries = %d", channel, n)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM attendees WHERE email='maya.spot@example.com' AND whatsapp_consent AND consent_text<>''"); n != 1 {
		t.Fatalf("spot attendee WhatsApp consent not recorded")
	}
	body["email"] = "another.spot@example.com"
	body["name"] = "Another Person"
	duplicate := c.request(http.MethodPost, "/api/v1/ops/spot-register", body)
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate payment reference: %d %s", duplicate.Code, duplicate.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE verified_reference=$1", "SBI-SPOT-0042"); n != 1 {
		t.Fatalf("verified payment rows = %d", n)
	}
}

func TestOpsCSVFormulaProtectionAndReferenceNormalisation(t *testing.T) {
	if got := opsReference("https://example.test/passes/bc26-in-0042"); got != "BC26IN0042" {
		t.Fatalf("normalised reference = %q", got)
	}
	for _, input := range []string{"=cmd", "+SUM(A1)", "-2+3", "@IMPORT"} {
		if got := safeCSV(input); !strings.HasPrefix(got, "'") {
			t.Errorf("safeCSV(%q) = %q", input, got)
		}
	}
	if got := safeCSV("Asha Nair"); got != "Asha Nair" {
		t.Fatalf("safe text changed to %q", got)
	}
}

func TestOpsBadgeIsHiddenOnScreenAndPrintsLandscapeStacked(t *testing.T) {
	css, err := resources.ReadFile("web/ops.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(css)
	for _, want := range []string{
		".badge-print{display:none}",
		"size:7.62cm 5.08cm",
		"width:7.62cm;height:5.08cm",
		".badge-print img{display:block;width:7.62cm;height:5.08cm}",
	} {
		if !strings.Contains(styles, want) {
			t.Errorf("badge stylesheet does not contain %q", want)
		}
	}
	javascript, err := resources.ReadFile("web/ops.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(javascript), `<p class="category">`) {
		t.Error("printed badge still includes the attendee category")
	}
	// The desk prints the kiosk's renderer, so both stations' badges match.
	if !strings.Contains(string(javascript), "Badge.png(p,qrUrl)") {
		t.Error("desk badge is not drawn by the shared badge renderer")
	}
	for _, page := range []string{"web/ops.html", "web/kiosk.html"} {
		html, err := resources.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(html), `<script src="/static/badge.js" defer></script>`) {
			t.Errorf("%s does not load the shared badge renderer", page)
		}
	}
	renderer, err := resources.ReadFile("web/badge.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"LABEL_MM={w:76.2,h:50.8}", "toUpperCase()", "g.textAlign='center'"} {
		if !strings.Contains(string(renderer), want) {
			t.Errorf("badge renderer does not contain %q", want)
		}
	}
}

func TestOpsScannerHasDistinctFeedbackSounds(t *testing.T) {
	javascript, err := resources.ReadFile("web/ops.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(javascript)
	for _, want := range []string{
		"success:{wave:'sine'",
		"repeat:{wave:'triangle'",
		"reject:{wave:'sawtooth'",
		"const kind=x.repeat?'repeat':'success';feedbackSound(kind)",
		"gain.gain.exponentialRampToValueAtTime(.22",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("scanner feedback does not contain %q", want)
		}
	}
}

func TestOpsReportsExposeFilterableActivityAudit(t *testing.T) {
	javascript, err := resources.ReadFile("web/ops.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(javascript)
	for _, want := range []string{
		"Operations audit",
		"Every accepted, repeated and rejected scan",
		"activity?day=",
		"/api/v1/ops/activity/export?day=",
		"/api/v1/ops/activity/export\"",
		"['all','success','repeat','rejected']",
		"data-audit-filter",
		"await loadAudit();if(current(c))renderReports()",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("operations audit UI does not contain %q", want)
		}
	}
}

func TestOpsMobileHeaderHasCompactAccessibleEndShiftControl(t *testing.T) {
	html, err := resources.ReadFile("web/ops.html")
	if err != nil {
		t.Fatal(err)
	}
	markup := string(html)
	for _, want := range []string{`id="station-button"`, `aria-label="End shift"`, `class="station-exit-icon"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("mobile shift control does not contain %q", want)
		}
	}
	javascript, err := resources.ReadFile("web/ops.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(javascript), "setAttribute('aria-label',`End shift for ${state.station}`)") {
		t.Error("shift control does not expose the current station in its accessible name")
	}
	css, err := resources.ReadFile("web/ops.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(css)
	for _, want := range []string{".station-button{display:grid;width:44px;height:44px", ".station-button .station-exit-icon{display:block}", "@media(max-width:360px)"} {
		if !strings.Contains(styles, want) {
			t.Errorf("mobile header stylesheet does not contain %q", want)
		}
	}
}

func TestOpsCameraScannerUsesEmbeddedCrossBrowserFallback(t *testing.T) {
	rr := httptest.NewRecorder()
	(&App{}).Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ops", nil))
	if policy := rr.Header().Get("Permissions-Policy"); !strings.Contains(policy, "camera=(self)") {
		t.Errorf("ops page camera permissions policy = %q", policy)
	}
	rr = httptest.NewRecorder()
	(&App{}).Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/ops.js", nil))
	if cacheControl := rr.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Errorf("ops script cache control = %q", cacheControl)
	}

	html, err := resources.ReadFile("web/ops.html")
	if err != nil {
		t.Fatal(err)
	}
	markup := string(html)
	vendorScript := `/static/vendor/html5-qrcode-2.3.8.min.js`
	appScript := `/static/ops.js`
	if vendorIndex, appIndex := strings.Index(markup, vendorScript), strings.Index(markup, appScript); vendorIndex < 0 || appIndex < 0 || vendorIndex > appIndex {
		t.Errorf("camera scanner dependency must load before ops.js")
	}
	if !strings.Contains(markup, `id="camera-reader"`) {
		t.Error("camera reader mount point is missing")
	}
	for _, want := range []string{`id="camera-result"`, `id="camera-result-name"`, `id="camera-result-meta"`, `id="camera-result-ref"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("camera attendee verification card does not contain %q", want)
		}
	}

	javascript, err := resources.ReadFile("web/ops.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(javascript)
	if !strings.Contains(script, "new Html5Qrcode('camera-reader'") {
		t.Error("camera scanner does not use the cross-browser QR decoder")
	}
	if strings.Contains(script, "BarcodeDetector") {
		t.Error("camera scanner still depends on the experimental BarcodeDetector API")
	}
	for _, want := range []string{
		"Scan with camera",
		"torchFeature()",
		"body.classList.add('camera-active')",
		"Scanning… Align the complete QR code inside the frame",
		"QR not readable yet · move farther back or improve light",
		"Ready for next badge",
		"decodedText===last.value",
		"Date.now()-last.seenAt>800",
		"unlockAudio();input.blur()",
		"state.audioContext||(state.audioContext=new Audio())",
		"showCameraResult(result,decodedText)",
		"person:x.person",
		"{fps:10,aspectRatio:1}",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("mobile camera flow does not contain %q", want)
		}
	}
	if strings.Contains(script, "qrbox:") {
		t.Error("camera scanner must decode the complete frame instead of cropping to a scan box")
	}
	if strings.Contains(script, "await closeCamera();onCode") || strings.Contains(script, "form.requestSubmit()") {
		t.Error("camera scanner must remain open and process scans without focusing the form")
	}
	css, err := resources.ReadFile("web/ops.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(css)
	if strings.Contains(styles, ".scan-actions button:first-child{display:none}") {
		t.Error("mobile stylesheet still hides the camera action")
	}
	if !strings.Contains(styles, "object-fit:contain") {
		t.Error("camera preview must preserve the decoder frame without cover cropping")
	}
	for _, want := range []string{"#camera-dialog{width:100vw", "height:100dvh", "env(safe-area-inset-bottom)", ".camera-frame video{top:42%}", ".scan-reticle{top:42%", ".camera-result{", ".camera-success #camera-note", ".camera-repeat #camera-note", ".camera-reject #camera-note"} {
		if !strings.Contains(styles, want) {
			t.Errorf("mobile camera stylesheet does not contain %q", want)
		}
	}
	if _, err := resources.ReadFile("web/vendor/html5-qrcode-2.3.8.min.js"); err != nil {
		t.Errorf("embedded camera scanner dependency: %v", err)
	}
	if _, err := resources.ReadFile("web/vendor/LICENSE.html5-qrcode"); err != nil {
		t.Errorf("embedded camera scanner license: %v", err)
	}
}
