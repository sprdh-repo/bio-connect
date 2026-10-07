package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func guestInput(name string) StaffRegistrationInput {
	return staffInput(RegistrationInput{CategoryID: "guest", Attendees: []Attendee{{Name: name}}}, "complimentary")
}

// A guest needs only a name. The pass is issued in the GU series, renders
// without an organisation or designation, and nothing is queued to send.
func TestGuestRegistrationNeedsOnlyName(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")

	rid, err := a.StaffCreate(ctx, guestInput("  Anjali Menon  "), key(1), nil, sid)
	if err != nil {
		t.Fatalf("guest with only a name: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || !r.Complimentary || r.Reference != "BC4-GU-0001" || r.Institution != "" || r.Email != "" || r.Phone != "" || len(r.Attendees) != 1 || r.Attendees[0].Name != "Anjali Menon" {
		t.Fatalf("registration = %+v", r)
	}
	var pid string
	if err = a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid).Scan(&pid); err != nil {
		t.Fatalf("pass not issued: %v", err)
	}
	pdf, err := a.passPDF(ctx, pid)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("guest pass did not render: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("guest registration queued %d deliveries", n)
	}

	// Contact details, when given, are kept on record but must be valid, and
	// WhatsApp consent is dropped because nothing is sent.
	full := guestInput("Ravi Kumar")
	full.Institution = "Kerala Startup Mission"
	full.Attendees[0] = Attendee{Name: "Ravi Kumar", Designation: "Advisor", Email: "Ravi@Example.com", Phone: "+919876543210", WhatsAppConsent: true}
	rid2, err := a.StaffCreate(ctx, full, key(2), nil, sid)
	if err != nil {
		t.Fatalf("guest with details: %v", err)
	}
	r2, _ := a.registration(ctx, rid2)
	if r2.Reference != "BC4-GU-0002" || r2.Email != "ravi@example.com" || r2.Attendees[0].WhatsAppConsent {
		t.Fatalf("registration = %+v", r2)
	}

	for name, in := range map[string]StaffRegistrationInput{
		"no name":     guestInput("  "),
		"bad email":   func() StaffRegistrationInput { g := guestInput("A"); g.Attendees[0].Email = "not-an-email"; return g }(),
		"local phone": func() StaffRegistrationInput { g := guestInput("A"); g.Attendees[0].Phone = "9876543210"; return g }(),
		"paid":        func() StaffRegistrationInput { g := guestInput("A"); g.Payment = "paid"; return g }(),
	} {
		if _, err := a.StaffCreate(ctx, in, key(10+len(name)), nil, sid); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

// No path delivers a guest's pass: not "Confirm and send", not a send or
// resend from the registration, and not a bulk send.
func TestGuestPassIsNeverSent(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")

	send := guestInput("Anjali Menon")
	send.Attendees[0].Email = "anjali@example.com"
	send.Send = true
	if _, err := a.StaffCreate(ctx, send, key(1), nil, sid); !errors.Is(err, errDownloadOnly) {
		t.Fatalf("confirm and send: %v, want %v", err, errDownloadOnly)
	}
	if n := count(t, a, "SELECT count(*) FROM registrations WHERE category_id='guest'"); n != 0 {
		t.Fatal("a refused send still saved the guest")
	}

	send.Send = false
	rid, err := a.StaffCreate(ctx, send, key(2), nil, sid)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"send", "resend"} {
		if err := a.Review(ctx, rid, sid, ReviewInput{Action: action, RequestID: strings.Repeat("r", 40)}); !errors.Is(err, errDownloadOnly) {
			t.Fatalf("%s: %v, want %v", action, err, errDownloadOnly)
		}
	}
	var pid string
	if err = a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1", rid).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send", PassID: pid, Channel: "email"}); !errors.Is(err, errDownloadOnly) {
		t.Fatalf("send one pass: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("guest registration queued %d deliveries", n)
	}

	// Staff can still correct the guest, including clearing optional details.
	var aid string
	if err = a.DB.QueryRow(ctx, "SELECT id FROM attendees WHERE registration_id=$1", rid).Scan(&aid); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateAttendee(ctx, rid, aid, sid, Attendee{Name: "Dr. Anjali Menon"}); err != nil {
		t.Fatalf("edit guest to name only: %v", err)
	}
	if err := a.UpdateAttendee(ctx, rid, aid, sid, Attendee{Name: ""}); err == nil {
		t.Fatal("guest edit without a name accepted")
	}
}

// Guests are staff-only: neither the public form nor a free link takes them.
func TestGuestIsNotPublic(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	in := delegateInput("guest")
	if _, _, err := a.Create(ctx, in, key(1), nil); err == nil {
		t.Fatal("public guest registration accepted")
	}
	cats, err := a.categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.DownloadOnly != (c.ID == "guest") {
			t.Fatalf("%s download_only = %t", c.ID, c.DownloadOnly)
		}
		if c.ID == "guest" && (c.Open || c.FreeOpen || !c.FreeOnly || c.Kind != "delegate" || c.RosterCount != 1) {
			t.Fatalf("guest category = %+v", c)
		}
	}
}

// Guests count in the console summary and in the on-site reports like every
// other category.
func TestGuestCountsInSummaryAndOpsReports(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	for i := 1; i <= 2; i++ {
		if _, err := a.StaffCreate(ctx, guestInput("Guest"), key(i), nil, sid); err != nil {
			t.Fatal(err)
		}
	}

	rr := httptest.NewRecorder()
	a.summary(rr, httptest.NewRequest("GET", "/api/v1/admin/summary", nil))
	var sum struct{ Categories []CategorySummary }
	if err := json.Unmarshal(rr.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range sum.Categories {
		if c.ID == "guest" {
			found = true
			if c.Label != "Guest" || c.Registered != 2 || c.Confirmed != 2 || c.Passes != 2 {
				t.Fatalf("guest summary = %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("summary has no guest row")
	}

	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Spot desk", "venue-passcode")
	res := c.request(http.MethodGet, "/api/v1/ops/reports", nil)
	if res.Code != 200 {
		t.Fatalf("reports: %d %s", res.Code, res.Body.String())
	}
	var rep struct {
		Summary map[string]int
		Rows    []struct {
			Day, CategoryID string
			Registered      int
		}
	}
	if err := json.Unmarshal(res.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Summary["registered"] != 2 {
		t.Fatalf("ops registered = %d, want 2", rep.Summary["registered"])
	}
	rows := 0
	for _, row := range rep.Rows {
		if row.CategoryID == "guest" {
			rows++
			if row.Registered != 2 {
				t.Fatalf("ops guest row = %+v", row)
			}
		}
	}
	if rows != len(opsDays) {
		t.Fatalf("ops reports have %d guest rows, want one per day", rows)
	}
}

// The spot desk registers a guest with only a name, checks them in, and gets
// the PDF pass to download. The download is for guest passes only.
func TestOpsSpotGuestNeedsOnlyNameAndDownloadsPass(t *testing.T) {
	a := mustApp(t)
	a.Config.OpsKey = "venue-passcode"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login("Spot desk", "venue-passcode")
	created := c.request(http.MethodPost, "/api/v1/ops/spot-register", map[string]any{
		"day": opsDays[0].ID, "categoryId": "guest", "name": "Walk-in Guest", "payment": "complimentary",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("spot guest: %d %s", created.Code, created.Body.String())
	}
	out := decodeOpsResponse(t, created)
	person := out["person"].(map[string]any)
	passURL, _ := out["passUrl"].(string)
	if person["checkedIn"] != true || person["category"] != "Guest" || passURL == "" {
		t.Fatalf("spot guest response: %s", created.Body.String())
	}
	pdf := c.request(http.MethodGet, passURL, nil)
	if pdf.Code != 200 || !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF")) || !strings.Contains(pdf.Header().Get("Content-Disposition"), "BC4-GU-0001-1") {
		t.Fatalf("guest pass download: %d %v", pdf.Code, pdf.Header())
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE action='pass_downloaded' AND detail LIKE '%Spot desk'"); n != 1 {
		t.Fatalf("download audit events = %d", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs"); n != 0 {
		t.Fatalf("spot guest queued %d deliveries", n)
	}

	// Another category's pass is not handed out here.
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	in := delegateInput("faculty")
	rid, err := a.StaffCreate(context.Background(), staffInput(in, "complimentary"), key(1), nil, sid)
	if err != nil {
		t.Fatal(err)
	}
	var qr string
	if err = a.DB.QueryRow(context.Background(), "SELECT qr_id FROM passes WHERE registration_id=$1", rid).Scan(&qr); err != nil {
		t.Fatal(err)
	}
	if res := c.request(http.MethodGet, "/api/v1/ops/passes/"+qr+".pdf", nil); res.Code != 404 {
		t.Fatalf("faculty pass download from ops: %d", res.Code)
	}
}

// A guest pass with only a name leaves the organisation out and shows the
// category in the role panel instead of an empty box.
func TestGuestPassWithOnlyNameRenders(t *testing.T) {
	data, err := renderPass("Fathima Beevi", "", "", "guest", "Guest", "BC4-GU-0001-1", "ZSBZY1g_lYLHTOTsddkvmAWhLP9SSel-YqsIGTwhmMk")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pass.pdf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, w := range readPassWords(t, path) {
		text.WriteString(w.Text + " ")
	}
	got := text.String()
	for _, want := range []string{"GUEST", "Fathima", "Beevi", "DESIGNATION Guest", "BC4-GU-0001-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("pass is missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "ORGANISATION") || strings.Contains(got, "INSTITUTION") {
		t.Errorf("pass labels an empty organisation: %s", got)
	}
}
