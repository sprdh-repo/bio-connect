package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests need a throwaway PostgreSQL. Point TEST_DATABASE_URL at one; the
// helper drops and recreates the public schema before every test.
func mustApp(t *testing.T) *App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c := Config{
		DatabaseURL:         dsn,
		BaseURL:             "http://localhost:8080",
		Listen:              ":0",
		StorageDir:          t.TempDir(),
		AWSRegion:           "ap-south-1",
		RegistrationEnabled: true,
		EncryptionKey:       base64.StdEncoding.EncodeToString(key),
		PostmarkStream:      "outbound",
		PostmarkAPIBase:     "https://api.postmarkapp.com",
		MetaAPIBase:         "https://graph.facebook.com",
	}
	a, err := Open(ctx, c)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := a.DB.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := a.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(a.DB.Close)
	return a
}

func key(n int) string { return fmt.Sprintf("idempotency-key-for-test-%080d", n) }

func today(a *App) string { return a.Now().In(india).Format("2006-01-02") }

func delegateInput(cat string) RegistrationInput {
	return RegistrationInput{
		CategoryID: cat, Institution: "Kerala University", ContactName: "Asha Nair",
		Email: "asha@example.com", Phone: "+919876543210",
		Attendees: []Attendee{{Name: "Asha Nair", Email: "asha@example.com", Phone: "+919876543210", Designation: "Student"}},
	}
}

func exhibitorInput(cat string, n int) RegistrationInput {
	in := RegistrationInput{
		CategoryID: cat, Institution: "Biotech Labs Pvt Ltd", ContactName: "Ravi Menon",
		Email: "ravi@example.com", Phone: "+919812345678", Description: "Molecular diagnostics",
	}
	for i := 0; i < n; i++ {
		in.Attendees = append(in.Attendees, Attendee{
			Name: fmt.Sprintf("Rep %d", i), Email: fmt.Sprintf("rep%d@example.com", i),
			Phone: "+91981234567" + fmt.Sprint(i), Designation: "Scientist",
		})
	}
	return in
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func addFile(t *testing.T, a *App, rid, kind string, body []byte) string {
	t.Helper()
	mime, err := validateUpload(body, kind)
	if err != nil {
		t.Fatalf("validateUpload(%s): %v", kind, err)
	}
	fid := id()
	if err := a.Storage.Put(context.Background(), fid, body, mime); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(),
		"INSERT INTO files(id,registration_id,kind,object_key,mime,size) VALUES($1,$2,$3,$1,$4,$5)",
		fid, rid, kind, mime, len(body)); err != nil {
		t.Fatal(err)
	}
	return fid
}

func payDelegate(t *testing.T, a *App, rid string, amountPaise int64) {
	t.Helper()
	// A receipt upload is optional; the reference and date are the evidence.
	if err := a.SubmitPayment(context.Background(), rid, PaymentInput{
		Reference: "UTR" + rid[:8], Date: today(a), AmountPaise: amountPaise,
	}); err != nil {
		t.Fatalf("submit payment: %v", err)
	}
}

func payExhibitor(t *testing.T, a *App, rid string, amountPaise int64) {
	t.Helper()
	addFile(t, a, rid, "logo", tinyPNG(t))
	payDelegate(t, a, rid, amountPaise)
}

func addStaff(t *testing.T, a *App, email, role string) (staffID, secret string) {
	t.Helper()
	u, err := a.AddStaff(context.Background(), email, "correct-horse-battery-staple", role)
	if err != nil {
		t.Fatalf("add staff: %v", err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	secret = parsed.Query().Get("secret")
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM staff WHERE email=$1", email).Scan(&staffID); err != nil {
		t.Fatal(err)
	}
	return
}

func earlyPaise(t *testing.T, a *App, rid string) int64 {
	t.Helper()
	var p int64
	if err := a.DB.QueryRow(context.Background(),
		"SELECT c.early_paise FROM registrations r JOIN categories c ON c.id=r.category_id WHERE r.id=$1", rid).Scan(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func latestPayment(a *App, rid string) string {
	var pid string
	a.DB.QueryRow(context.Background(),
		"SELECT id FROM payment_submissions WHERE registration_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1", rid).Scan(&pid)
	return pid
}

func tryApprove(a *App, rid, staffID, action, verifiedRef string, amountPaise int64) error {
	return a.Review(context.Background(), rid, staffID, ReviewInput{
		Action: action, PaymentID: latestPayment(a, rid),
		VerifiedReference: verifiedRef, VerifiedDate: a.Now().In(india).Format("2006-01-02"),
		VerifiedAmountPaise: amountPaise, Successful: true, BeneficiaryConfirmed: true, Note: "verified against bank statement",
	})
}

func count(t *testing.T, a *App, q string, args ...any) int {
	t.Helper()
	var n int
	if err := a.DB.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", q, err)
	}
	return n
}

func status(t *testing.T, a *App, rid string) string {
	t.Helper()
	var s string
	if err := a.DB.QueryRow(context.Background(), "SELECT status FROM registrations WHERE id=$1", rid).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// --- registration journeys -------------------------------------------------

func TestDelegateJourneyEveryCategory(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	for i, cat := range []string{"student", "startup", "faculty", "industry"} {
		rid, tok, err := a.Create(ctx, delegateInput(cat), key(i))
		if err != nil {
			t.Fatalf("%s create: %v", cat, err)
		}
		if len(tok) != 43 {
			t.Fatalf("%s: management token length %d", cat, len(tok))
		}
		if status(t, a, rid) != "awaiting_payment" {
			t.Fatalf("%s: unexpected initial status", cat)
		}
		payDelegate(t, a, rid, earlyPaise(t, a, rid))
		if status(t, a, rid) != "awaiting_review" {
			t.Fatalf("%s: status after payment %s", cat, status(t, a, rid))
		}
		if err := tryApprove(a, rid, sid, "approve_send", "SBIN"+cat, earlyPaise(t, a, rid)); err != nil {
			t.Fatalf("%s approve: %v", cat, err)
		}
		if status(t, a, rid) != "approved" {
			t.Fatalf("%s: not approved", cat)
		}
		if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 1 {
			t.Fatalf("%s: %d passes, want 1", cat, n)
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND channel='email'", rid); n != 1 {
			t.Fatalf("%s: %d email pass jobs, want 1", cat, n)
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack'", rid); n != 0 {
			t.Fatalf("%s: delegate should not get a pack", cat)
		}
	}
}

func TestExhibitorRosterCountsEnforced(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	cases := []struct {
		cat  string
		want int
	}{{"premium", 6}, {"standard", 4}, {"table", 2}}
	for i, c := range cases {
		if _, _, err := a.Create(ctx, exhibitorInput(c.cat, c.want-1), key(100+i)); err == nil {
			t.Fatalf("%s: accepted %d attendees, want exactly %d", c.cat, c.want-1, c.want)
		}
		if _, _, err := a.Create(ctx, exhibitorInput(c.cat, c.want+1), key(200+i)); err == nil {
			t.Fatalf("%s: accepted %d attendees, want exactly %d", c.cat, c.want+1, c.want)
		}
		rid, _, err := a.Create(ctx, exhibitorInput(c.cat, c.want), key(300+i))
		if err != nil {
			t.Fatalf("%s: exact roster rejected: %v", c.cat, err)
		}
		payExhibitor(t, a, rid, earlyPaise(t, a, rid))
		if err := tryApprove(a, rid, sid, "approve_send", "SBIX"+c.cat, earlyPaise(t, a, rid)); err != nil {
			t.Fatalf("%s approve: %v", c.cat, err)
		}
		if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != c.want {
			t.Fatalf("%s: %d passes, want %d", c.cat, n, c.want)
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack' AND channel='email'", rid); n != 1 {
			t.Fatalf("%s: %d pack jobs, want 1", c.cat, n)
		}
	}
}

func TestPaymentBeforeSbiIsAllowedButNotAutoApproved(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, _, err := a.Create(ctx, delegateInput("faculty"), key(1))
	if err != nil {
		t.Fatal(err)
	}
	payDelegate(t, a, rid, earlyPaise(t, a, rid))
	// Submitting reference + date moves the registration to review, never to approved.
	if status(t, a, rid) != "awaiting_review" {
		t.Fatalf("status %s, want awaiting_review", status(t, a, rid))
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("passes created before staff approval: %d", n)
	}
}

func TestRegistrationDisabledByDefault(t *testing.T) {
	a := mustApp(t)
	a.Config.RegistrationEnabled = false
	if _, _, err := a.Create(context.Background(), delegateInput("student"), key(1)); err == nil {
		t.Fatal("Create succeeded while registration disabled")
	}
}

func TestClosedCategoryBlocksNewButNotExisting(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, _, err := a.Create(ctx, delegateInput("industry"), key(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(ctx, "UPDATE categories SET open=false WHERE id='industry'"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Create(ctx, delegateInput("industry"), key(2)); err == nil {
		t.Fatal("Create succeeded for closed category")
	}
	// The already-saved registration can still submit payment evidence.
	payDelegate(t, a, rid, earlyPaise(t, a, rid))
}

// --- idempotency & interrupted submissions --------------------------------

func TestIdempotentCreate(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	in := delegateInput("student")
	rid1, tok1, err := a.Create(ctx, in, key(1))
	if err != nil {
		t.Fatal(err)
	}
	rid2, tok2, err := a.Create(ctx, in, key(1))
	if err != nil {
		t.Fatalf("replay errored: %v", err)
	}
	if rid1 != rid2 {
		t.Fatalf("replay produced a new registration %s vs %s", rid1, rid2)
	}
	if tok2 != "" {
		t.Fatal("replay returned a fresh management token")
	}
	_ = tok1
	changed := in
	changed.Institution = "Different Institution"
	if _, _, err := a.Create(ctx, changed, key(1)); err == nil {
		t.Fatal("reused key with different details was accepted")
	}
	if n := count(t, a, "SELECT count(*) FROM registrations"); n != 1 {
		t.Fatalf("%d registrations, want 1", n)
	}
	if _, _, err := a.Create(ctx, in, "short"); err == nil {
		t.Fatal("accepted a too-short idempotency key")
	}
}

// --- fees & cutoff --------------------------------------------------------

func TestFeeCutoffBoundary(t *testing.T) {
	student := Category{EarlyPaise: 100000, RegularPaise: 150000}
	on30Sep, _ := time.ParseInLocation("2006-01-02", "2026-09-30", india)
	on1Oct, _ := time.ParseInLocation("2006-01-02", "2026-10-01", india)
	if got := fee(student, on30Sep); got != 100000 {
		t.Fatalf("30 Sep fee = %d, want early 100000", got)
	}
	if got := fee(student, on1Oct); got != 150000 {
		t.Fatalf("1 Oct fee = %d, want regular 150000", got)
	}
}

func TestExhibitorCategoriesAreHighestFirst(t *testing.T) {
	a := mustApp(t)
	cats, err := a.categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cats {
		if c.Kind == "exhibitor" {
			got = append(got, c.ID)
		}
	}
	if strings.Join(got, ",") != "premium,standard,table" {
		t.Fatalf("exhibitor category order = %v, want premium, standard, table", got)
	}
}

func TestDelegateCategoriesAreIndustryFirst(t *testing.T) {
	a := mustApp(t)
	cats, err := a.categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cats {
		if c.Kind == "delegate" {
			got = append(got, c.ID)
		}
	}
	if strings.Join(got, ",") != "industry,faculty,startup,student" {
		t.Fatalf("delegate category order = %v, want industry, faculty, startup, student", got)
	}
}

func TestQuoteAndDisplayedFeeFollowServerClock(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	// Before the cutoff the displayed fee is the early-bird amount.
	cats, err := a.categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.ID == "student" && c.PayablePaise != 100000 {
			t.Fatalf("student payable before cutoff = %d", c.PayablePaise)
		}
	}
	// After the cutoff the displayed fee refreshes to regular.
	a.Now = func() time.Time { return time.Date(2026, 10, 5, 9, 0, 0, 0, india) }
	cats, _ = a.categories(ctx)
	for _, c := range cats {
		if c.ID == "student" && c.PayablePaise != 150000 {
			t.Fatalf("student payable after cutoff = %d, want 150000", c.PayablePaise)
		}
	}
	// Late-submitted evidence of a timely payment still gets the early-bird fee:
	// approval checks fee(verifiedDate), not "now".
	a.Now = func() time.Time { return time.Date(2026, 10, 5, 9, 0, 0, 0, india) }
	rid, _, err := a.Create(ctx, delegateInput("student"), key(1))
	if err != nil {
		t.Fatal(err)
	}
	payDelegate(t, a, rid, 100000)
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	err = a.Review(ctx, rid, sid, ReviewInput{
		Action: "approve_only", PaymentID: latestPayment(a, rid),
		VerifiedReference: "SBILATE", VerifiedDate: "2026-09-29", VerifiedAmountPaise: 100000,
		Successful: true, BeneficiaryConfirmed: true, Note: "paid before cutoff, evidence late",
	})
	if err != nil {
		t.Fatalf("late evidence for timely payment rejected: %v", err)
	}
}

// --- payment review -----------------------------------------------------

func TestReviewerCanRecordAndApproveUnsubmittedPayment(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, delegateInput("faculty"), key(1))

	err := a.Review(ctx, rid, sid, ReviewInput{
		Action: "record_approve_only", VerifiedReference: "  sbi manual 123  ",
		VerifiedDate: today(a), VerifiedAmountPaise: 400000,
		Successful: true, BeneficiaryConfirmed: true, Note: "found in SBI Collect",
	})
	if err != nil {
		t.Fatalf("record and approve: %v", err)
	}
	if status(t, a, rid) != "approved" {
		t.Fatalf("status = %s, want approved", status(t, a, rid))
	}
	var reportedRef, verifiedRef, verifiedBy string
	var reportedAmount, verifiedAmount int64
	if err := a.DB.QueryRow(ctx, `SELECT bank_reference,amount_paise,verified_reference,verified_amount_paise,verified_by
		FROM payment_submissions WHERE registration_id=$1`, rid).Scan(
		&reportedRef, &reportedAmount, &verifiedRef, &verifiedAmount, &verifiedBy); err != nil {
		t.Fatal(err)
	}
	if reportedRef != "SBIMANUAL123" || verifiedRef != reportedRef {
		t.Fatalf("reported/verified references = %q/%q", reportedRef, verifiedRef)
	}
	if reportedAmount != 400000 || verifiedAmount != reportedAmount || verifiedBy != sid {
		t.Fatalf("payment record = reported %d, verified %d by %q", reportedAmount, verifiedAmount, verifiedBy)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND staff_id=$2 AND action='record_approve_only'", rid, sid); n != 1 {
		t.Fatalf("recorded payment audit rows = %d, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != 0 {
		t.Fatalf("record_approve_only queued %d pass jobs", n)
	}
}

func TestReviewerRecordedExhibitorPaymentRequiresLogo(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, exhibitorInput("table", 2), key(1))

	in := ReviewInput{
		Action: "record_approve_send", VerifiedReference: "SBIEXHIBITOR",
		VerifiedDate: today(a), VerifiedAmountPaise: earlyPaise(t, a, rid),
		Successful: true, BeneficiaryConfirmed: true,
	}
	err := a.Review(ctx, rid, sid, in)
	if err == nil || !strings.Contains(err.Error(), "logo") {
		t.Fatalf("record payment without exhibitor logo error = %v", err)
	}
	if status(t, a, rid) != "awaiting_payment" {
		t.Fatalf("status changed after missing-logo rejection: %s", status(t, a, rid))
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("created %d payment submissions after missing-logo rejection", n)
	}

	addFile(t, a, rid, "logo", tinyPNG(t))
	if err := a.Review(ctx, rid, sid, in); err != nil {
		t.Fatalf("record exhibitor payment after logo upload: %v", err)
	}
	if status(t, a, rid) != "approved" {
		t.Fatalf("status after recording exhibitor payment = %s", status(t, a, rid))
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 2 {
		t.Fatalf("record_approve_send created %d exhibitor passes, want 2", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose IN ('pass','pack')", rid); n != 3 {
		t.Fatalf("record_approve_send queued %d pass/pack jobs, want 3", n)
	}
}

func TestApprovalRejectsMismatchedAmount(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, delegateInput("industry"), key(1))
	payDelegate(t, a, rid, 600000)
	if err := tryApprove(a, rid, sid, "approve_send", "SBIWRONG", 500000); err == nil {
		t.Fatal("approval accepted an amount below the category fee")
	}
	if status(t, a, rid) != "awaiting_review" {
		t.Fatalf("status changed after failed approval: %s", status(t, a, rid))
	}
	if err := tryApprove(a, rid, sid, "approve_send", "SBIRIGHT", 600000); err != nil {
		t.Fatalf("correct amount rejected: %v", err)
	}
}

func TestApprovalRequiresBankConfirmationFlags(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, delegateInput("faculty"), key(1))
	payDelegate(t, a, rid, 400000)
	err := a.Review(ctx, rid, sid, ReviewInput{
		Action: "approve_send", PaymentID: latestPayment(a, rid),
		VerifiedReference: "SBIX", VerifiedDate: today(a), VerifiedAmountPaise: 400000,
		Successful: false, BeneficiaryConfirmed: true,
	})
	if err == nil {
		t.Fatal("approved without confirming the payment succeeded")
	}
}

func TestCorrectionRequestedThenResubmit(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, delegateInput("startup"), key(1))
	payDelegate(t, a, rid, 350000)
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "correction_requested", Note: "reference does not match bank record"}); err != nil {
		t.Fatal(err)
	}
	if status(t, a, rid) != "correction_requested" {
		t.Fatal("not in correction_requested")
	}
	// Registrant resubmits; history is preserved.
	payDelegate(t, a, rid, 350000)
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1", rid); n != 2 {
		t.Fatalf("payment history not preserved: %d rows", n)
	}
	if err := tryApprove(a, rid, sid, "approve_send", "SBIFIXED", 350000); err != nil {
		t.Fatalf("approve after correction: %v", err)
	}
}

func TestApprovedBankReferenceCannotBeReused(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	ridA, _, _ := a.Create(ctx, delegateInput("industry"), key(1))
	payDelegate(t, a, ridA, 600000)
	if err := tryApprove(a, ridA, sid, "approve_send", "SBISHARED", 600000); err != nil {
		t.Fatal(err)
	}
	inB := delegateInput("industry")
	inB.Attendees[0].Email = "second@example.com"
	inB.Email = "second@example.com"
	ridB, _, _ := a.Create(ctx, inB, key(2))
	payDelegate(t, a, ridB, 600000)
	if err := tryApprove(a, ridB, sid, "approve_send", "SBISHARED", 600000); err == nil {
		t.Fatal("reused an already-approved bank transaction reference")
	}
	if status(t, a, ridB) != "awaiting_review" {
		t.Fatalf("B advanced despite duplicate reference: %s", status(t, a, ridB))
	}
}

// --- concurrency ------------------------------------------------------------

func TestConcurrentApprovalCreatesNoExtraPasses(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, exhibitorInput("standard", 4), key(1))
	payExhibitor(t, a, rid, earlyPaise(t, a, rid))

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = tryApprove(a, rid, sid, "approve_send", "SBICONCURRENT", earlyPaise(t, a, rid))
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("concurrent approve %d errored: %v", i, e)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 4 {
		t.Fatalf("%d passes after concurrent approval, want 4", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND channel='email'", rid); n != 4 {
		t.Fatalf("%d email pass jobs, want 4", n)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='approve_send'", rid); n != 1 {
		t.Fatalf("%d approval audit rows, want 1", n)
	}
}

// --- delivery: send / resend / reissue / cancel ---------------------------

func approvedDelegate(t *testing.T, a *App, k int, action string) (rid, sid string) {
	t.Helper()
	ctx := context.Background()
	sid, _ = addStaff(t, a, fmt.Sprintf("rev%d@bioconnect.test", k), "reviewer")
	rid, _, _ = a.Create(ctx, delegateInput("industry"), key(k))
	payDelegate(t, a, rid, 600000)
	if err := tryApprove(a, rid, sid, action, fmt.Sprintf("SBI%d", k), 600000); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return
}

func TestApproveOnlyThenSendIsIdempotent(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, sid := approvedDelegate(t, a, 1, "approve_only")
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != 0 {
		t.Fatalf("approve_only queued %d pass jobs", n)
	}
	for i := 0; i < 3; i++ {
		if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != 1 {
		t.Fatalf("repeated send created %d jobs, want 1", n)
	}
}

func TestResendKeepsPassReissueRevokesIt(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, sid := approvedDelegate(t, a, 1, "approve_send")
	var passID string
	if err := a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1", rid).Scan(&passID); err != nil {
		t.Fatal(err)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "resend", RequestID: key(9)}); err != nil {
		t.Fatalf("resend: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 1 {
		t.Fatalf("resend changed pass count to %d", n)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE id=$1 AND revoked_at IS NULL", passID); n != 1 {
		t.Fatal("resend revoked the pass")
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "reissue", PassID: passID, Note: "printed copy lost"}); err != nil {
		t.Fatalf("reissue: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE id=$1 AND revoked_at IS NOT NULL", passID); n != 1 {
		t.Fatal("reissue did not revoke the old pass")
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 1 {
		t.Fatalf("%d active passes after reissue, want exactly 1", n)
	}
	var v int
	a.DB.QueryRow(ctx, "SELECT version FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid).Scan(&v)
	if v != 2 {
		t.Fatalf("reissued pass version = %d, want 2", v)
	}
}

func TestCancellationRevokesPassesAndPendingJobs(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, sid := approvedDelegate(t, a, 1, "approve_send")
	var token string
	a.DB.QueryRow(ctx, "SELECT download_cipher FROM passes WHERE registration_id=$1", rid).Scan(&token)
	plain, err := a.unseal(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "cancelled", Note: "duplicate registration; refund handled manually"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 0 {
		t.Fatal("cancellation left an active pass")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND status='queued'", rid); n != 0 {
		t.Fatal("queued pass jobs survived cancellation")
	}
	// Pass download by its token now fails.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/passes/"+plain, nil)
	req.SetPathValue("token", plain)
	a.passDownload(rr, req)
	if rr.Code != 404 {
		t.Fatalf("revoked pass download returned %d, want 404", rr.Code)
	}
}

func TestBulkSendReportsPerRegistration(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	ctx := context.Background()
	var approved, notApproved string
	{
		rid, _, _ := a.Create(ctx, delegateInput("industry"), key(1))
		payDelegate(t, a, rid, 600000)
		tryApprove(a, rid, sid, "approve_only", "SBIBULK1", 600000)
		approved = rid
	}
	{
		rid, _, _ := a.Create(ctx, delegateInput("faculty"), key(2))
		payDelegate(t, a, rid, 400000)
		notApproved = rid
	}
	results := map[string]string{}
	for _, rid := range []string{approved, notApproved} {
		if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
			results[rid] = err.Error()
		} else {
			results[rid] = "queued"
		}
	}
	if results[approved] != "queued" {
		t.Fatalf("approved bulk send: %s", results[approved])
	}
	if results[notApproved] == "queued" {
		t.Fatal("bulk send queued a non-approved registration")
	}
}

// --- recovery & token isolation -----------------------------------------

func TestSecureRecoveryFlow(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, firstTok, _ := a.Create(ctx, delegateInput("student"), key(1))

	rr := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"email": "asha@example.com"})
	req := httptest.NewRequest("POST", "/api/v1/recovery", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	a.recover(rr, req)
	if rr.Code != 202 {
		t.Fatalf("recovery request returned %d", rr.Code)
	}
	var tokenHash string
	if err := a.DB.QueryRow(ctx, "SELECT token_hash FROM recovery_tokens WHERE registration_id=$1", rid).Scan(&tokenHash); err != nil {
		t.Fatalf("no recovery token stored: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='recovery'", rid); n != 1 {
		t.Fatalf("%d recovery emails queued", n)
	}
	// We cannot read the raw token from outside, so drive exchange() with a known token.
	raw := randomToken()
	if _, err := a.DB.Exec(ctx, "UPDATE recovery_tokens SET token_hash=$1 WHERE registration_id=$2", hash(raw), rid); err != nil {
		t.Fatal(err)
	}
	ex := httptest.NewRecorder()
	eb, _ := json.Marshal(map[string]string{"token": raw})
	er := httptest.NewRequest("POST", "/api/v1/recovery/exchange", bytes.NewReader(eb))
	er.Header.Set("Content-Type", "application/json")
	a.exchange(ex, er)
	if ex.Code != 200 {
		t.Fatalf("exchange returned %d", ex.Code)
	}
	var out struct {
		ID              string `json:"id"`
		ManagementToken string `json:"management_token"`
	}
	json.Unmarshal(ex.Body.Bytes(), &out)
	if out.ID != rid || len(out.ManagementToken) != 43 {
		t.Fatalf("exchange payload: %+v", out)
	}
	if out.ManagementToken == firstTok {
		t.Fatal("recovery returned the original management token")
	}
	// The old management token no longer works.
	if a.manage(bearerReq(rid, firstTok), rid) {
		t.Fatal("original management token still valid after recovery")
	}
	if !a.manage(bearerReq(rid, out.ManagementToken), rid) {
		t.Fatal("recovered management token rejected")
	}
	// Single use.
	ex2 := httptest.NewRecorder()
	er2 := httptest.NewRequest("POST", "/api/v1/recovery/exchange", bytes.NewReader(eb))
	er2.Header.Set("Content-Type", "application/json")
	a.exchange(ex2, er2)
	if ex2.Code != 401 {
		t.Fatalf("reused recovery token returned %d, want 401", ex2.Code)
	}
}

func bearerReq(rid, token string) *http.Request {
	r := httptest.NewRequest("GET", "/api/v1/registrations/"+rid, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

func TestManagementTokenIsolation(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	ridA, tokA, _ := a.Create(ctx, delegateInput("student"), key(1))
	inB := delegateInput("faculty")
	inB.Email = "b@example.com"
	inB.Attendees[0].Email = "b@example.com"
	ridB, tokB, _ := a.Create(ctx, inB, key(2))
	if a.manage(bearerReq(ridB, tokA), ridB) {
		t.Fatal("registration A token unlocked registration B")
	}
	if !a.manage(bearerReq(ridA, tokA), ridA) || !a.manage(bearerReq(ridB, tokB), ridB) {
		t.Fatal("own tokens rejected")
	}
}

// --- uploads ------------------------------------------------------------

func TestUploadValidation(t *testing.T) {
	if _, err := validateUpload(nil, "receipt"); err == nil {
		t.Fatal("accepted an empty upload")
	}
	if _, err := validateUpload(bytes.Repeat([]byte{0}, 6<<20), "receipt"); err == nil {
		t.Fatal("accepted a 6 MB upload")
	}
	if _, err := validateUpload([]byte("#!/bin/sh\nrm -rf /\n"), "receipt"); err == nil {
		t.Fatal("accepted a shell script as a receipt")
	}
	if _, err := validateUpload([]byte("plain text pretending to be a receipt"), "receipt"); err == nil {
		t.Fatal("accepted text/plain as a receipt")
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	if _, err := validateUpload(b.Bytes(), "logo"); err != nil {
		t.Fatalf("rejected a valid PNG logo: %v", err)
	}
	// A well-formed PDF passes; a truncated / non-PDF one does not.
	minimalPDF := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 100 100]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")
	if _, err := validateUpload(minimalPDF, "receipt"); err != nil {
		t.Fatalf("rejected a valid minimal PDF receipt: %v", err)
	}
	if _, err := validateUpload([]byte("%PDF-1.4 not really\n%%EOF"), "receipt"); err == nil {
		t.Fatal("accepted a corrupt PDF receipt")
	}
	if _, err := validateUpload(minimalPDF, "logo"); err == nil {
		t.Fatal("accepted a PDF as a logo")
	}
}

// --- exports ----------------------------------------------------------

func TestExportContentsAndFormulaInjection(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	in := delegateInput("industry")
	in.Institution = "=cmd|' /c calc'!A1" // spreadsheet formula injection attempt
	in.ContactName = "Asha Nair"
	rid, _, err := a.Create(ctx, in, key(1))
	if err != nil {
		t.Fatal(err)
	}
	payDelegate(t, a, rid, 600000)
	tryApprove(a, rid, sid, "approve_send", "SBIEXPORT", 600000)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/export?format=csv&sheet=Registrations", nil)
	a.export(rr, req, principal{ID: sid, Role: "reviewer"})
	if rr.Code != 200 {
		t.Fatalf("csv export returned %d: %s", rr.Code, rr.Body.String())
	}
	records, err := csv.NewReader(bytes.NewReader(rr.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	head := strings.Join(records[0], ",")
	for _, col := range []string{"reference", "status", "quoted_paise", "approved_by", "delivery_summary"} {
		if !strings.Contains(head, col) {
			t.Fatalf("export missing column %q; header=%s", col, head)
		}
	}
	var instCol int = -1
	for i, h := range records[0] {
		if h == "institution" {
			instCol = i
		}
	}
	if instCol < 0 || len(records) < 2 {
		t.Fatal("no institution column / data row")
	}
	if got := records[1][instCol]; !strings.HasPrefix(got, "'") {
		t.Fatalf("formula-injection cell not neutralised: %q", got)
	}

	xr := httptest.NewRecorder()
	xq := httptest.NewRequest("GET", "/api/v1/admin/export?format=xlsx", nil)
	a.export(xr, xq, principal{ID: sid, Role: "reviewer"})
	if xr.Code != 200 || xr.Body.Len() < 1000 {
		t.Fatalf("xlsx export bad: code=%d len=%d", xr.Code, xr.Body.Len())
	}
	if ct := xr.Header().Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("xlsx content type: %s", ct)
	}
}

// --- passes & QR ------------------------------------------------------

func TestPassPDFHasReadableOpaqueQR(t *testing.T) {
	if _, err := exec.LookPath("zbarimg"); err != nil {
		t.Skip("zbarimg not installed")
	}
	a := mustApp(t)
	ctx := context.Background()
	rid, _ := approvedDelegate(t, a, 1, "approve_send")
	var passID, qrID string
	a.DB.QueryRow(ctx, "SELECT id,qr_id FROM passes WHERE registration_id=$1", rid).Scan(&passID, &qrID)

	pdf, err := a.passPDF(ctx, passID)
	if err != nil {
		t.Fatalf("passPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("pass is not a PDF")
	}
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/pass.pdf", pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("pdftoppm", "-png", "-r", "300", dir+"/pass.pdf", dir+"/pass").CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v %s", err, out)
	}
	out, _ := exec.Command("zbarimg", "--quiet", "--raw", dir+"/pass-1.png").Output()
	got := strings.TrimSpace(string(out))
	if got != qrID {
		t.Fatalf("decoded QR = %q, want opaque id %q", got, qrID)
	}
	// The opaque id carries no contact information.
	for _, pii := range []string{"asha@example.com", "+919876543210", "Asha Nair"} {
		if strings.Contains(got, pii) {
			t.Fatalf("QR payload leaked %q", pii)
		}
	}
	// Cached on second render.
	if _, err := a.passPDF(ctx, passID); err != nil {
		t.Fatalf("second passPDF: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM files WHERE registration_id=$1 AND kind='pass'", rid); n != 1 {
		t.Fatalf("%d cached pass files, want 1", n)
	}
}

func TestPassNumberIsStableAcrossRenders(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, _ := approvedDelegate(t, a, 1, "approve_send")
	var number, reference string
	a.DB.QueryRow(ctx, "SELECT number FROM passes WHERE registration_id=$1", rid).Scan(&number)
	a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&reference)
	if !passNumberShape.MatchString(number) {
		t.Fatalf("pass number %q is not a short branded number", number)
	}
	// The number names its registration, so staff read one handle off the pass.
	if number != reference+"-1" {
		t.Fatalf("pass number %q does not follow reference %q", number, reference)
	}
}

// The series is per category and dense: an exhibitor roster takes consecutive
// places in its registration, and a reissue takes the next free one rather than
// a number already printed on the revoked pass.
func TestPassNumbersFollowTheRegistrationReference(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, err := a.Create(ctx, exhibitorInput("standard", 4), key(1))
	if err != nil {
		t.Fatal(err)
	}
	payExhibitor(t, a, rid, earlyPaise(t, a, rid))
	if err := tryApprove(a, rid, sid, "approve_send", "SBIROSTER", earlyPaise(t, a, rid)); err != nil {
		t.Fatal(err)
	}
	var reference string
	a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&reference)
	if reference != "BC4-EX-0001" {
		t.Fatalf("exhibitor reference %q, want BC4-EX-0001", reference)
	}
	for i, want := range []string{"-1", "-2", "-3", "-4"} {
		if n := nth(t, a, rid, i); n != reference+want {
			t.Fatalf("pass %d numbered %q, want %q", i+1, n, reference+want)
		}
	}
	var passID string
	a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1 ORDER BY created_at,number LIMIT 1", rid).Scan(&passID)
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "reissue", PassID: passID, Note: "misprint"}); err != nil {
		t.Fatalf("reissue: %v", err)
	}
	if n := nth(t, a, rid, 4); n != reference+"-5" {
		t.Fatalf("reissued pass numbered %q, want %q", n, reference+"-5")
	}
}

func nth(t *testing.T, a *App, rid string, i int) string {
	t.Helper()
	var n string
	if err := a.DB.QueryRow(context.Background(), "SELECT number FROM passes WHERE registration_id=$1 ORDER BY created_at,number OFFSET $2 LIMIT 1", rid, i).Scan(&n); err != nil {
		t.Fatalf("pass %d: %v", i, err)
	}
	return n
}

// Each category counts on its own, so a delegate never sees an exhibitor's
// position in the series and the printed code always matches the printed badge.
func TestReferenceSeriesIsPerCategory(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	want := map[string]string{}
	for i, cat := range []string{"industry", "industry", "student", "premium", "industry"} {
		in := delegateInput(cat)
		if cat == "premium" {
			in = exhibitorInput(cat, 6)
		}
		rid, _, err := a.Create(ctx, in, key(i+1))
		if err != nil {
			t.Fatal(err)
		}
		var got string
		a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&got)
		want[got] = cat
	}
	for _, ref := range []string{"BC4-IN-0001", "BC4-IN-0002", "BC4-IN-0003", "BC4-ST-0001", "BC4-EX-0001"} {
		if want[ref] == "" {
			t.Fatalf("reference %q was not issued; got %v", ref, want)
		}
	}
}

// Registrations arriving together must not read the same maximum: the series
// has to stay dense and unique without anyone retrying.
func TestConcurrentRegistrationsGetConsecutiveReferences(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	refs := make([]string, 10)
	for i := range refs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rid, _, err := a.Create(ctx, delegateInput("faculty"), key(i+1))
			if err != nil {
				t.Errorf("create %d: %v", i, err)
				return
			}
			a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&refs[i])
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, r := range refs {
		seen[r] = true
	}
	for i := 1; i <= 10; i++ {
		if ref := fmt.Sprintf("BC4-FC-%04d", i); !seen[ref] {
			t.Fatalf("reference %q missing after concurrent creates: %v", ref, refs)
		}
	}
}

// The number printed on a pass is the handle staff are given at the desk, so a
// search has to find the registration from it however it is retyped.
func TestStaffCanSearchByPassNumber(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, sid := approvedDelegate(t, a, 1, "approve_send")
	var number, reference string
	a.DB.QueryRow(ctx, "SELECT number FROM passes WHERE registration_id=$1", rid).Scan(&number)
	a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&reference)

	for _, q := range []string{number, strings.ToLower(number), strings.ReplaceAll(number, "-", ""), strings.ReplaceAll(number, "-", " "), reference, strings.ReplaceAll(reference, "-", "")} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/admin/export?format=csv&sheet=Registrations&q="+url.QueryEscape(q), nil)
		a.export(rr, req, principal{ID: sid, Role: "reviewer"})
		if rr.Code != 200 {
			t.Fatalf("search %q returned %d: %s", q, rr.Code, rr.Body.String())
		}
		records, err := csv.NewReader(bytes.NewReader(rr.Body.Bytes())).ReadAll()
		if err != nil {
			t.Fatalf("csv parse: %v", err)
		}
		if len(records) != 2 || !strings.Contains(strings.Join(records[1], ","), reference) {
			t.Fatalf("search %q did not return exactly the matching registration: %v", q, records)
		}
	}
	// An unknown number finds nothing, and a term with no pass-number symbols in
	// it must not fall through to matching every registration that has a pass.
	for _, q := range []string{"BC4-2222-2222", "!!"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/admin/export?format=csv&sheet=Registrations&q="+url.QueryEscape(q), nil)
		a.export(rr, req, principal{ID: sid, Role: "reviewer"})
		records, _ := csv.NewReader(bytes.NewReader(rr.Body.Bytes())).ReadAll()
		if len(records) != 1 {
			t.Fatalf("search %q matched %d rows, want none", q, len(records)-1)
		}
	}
}

// --- migrations -------------------------------------------------------

func TestMigrationsApplyIncrementallyAndAreIdempotent(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	var versions []int
	rows, err := a.DB.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	if len(versions) < 2 || versions[0] != 1 || versions[1] != 2 {
		t.Fatalf("schema_migrations = %v, want 1 and 2 recorded", versions)
	}
	var nullable string
	if err := a.DB.QueryRow(ctx, "SELECT is_nullable FROM information_schema.columns WHERE table_name='payment_submissions' AND column_name='receipt_id'").Scan(&nullable); err != nil {
		t.Fatal(err)
	}
	if nullable != "YES" {
		t.Fatalf("payment_submissions.receipt_id is_nullable=%s, want YES after 002", nullable)
	}
	before := count(t, a, "SELECT count(*) FROM schema_migrations")
	if err := a.Migrate(ctx); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	if after := count(t, a, "SELECT count(*) FROM schema_migrations"); after != before {
		t.Fatalf("re-migrate changed schema_migrations from %d to %d", before, after)
	}
}

func TestPaymentEvidenceReceiptIsOptional(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	rid, _, err := a.Create(ctx, delegateInput("student"), key(1))
	if err != nil {
		t.Fatal(err)
	}
	payDelegate(t, a, rid, earlyPaise(t, a, rid))
	var receipt *string
	if err := a.DB.QueryRow(ctx, "SELECT receipt_id FROM payment_submissions WHERE registration_id=$1", rid).Scan(&receipt); err != nil {
		t.Fatal(err)
	}
	if receipt != nil {
		t.Fatalf("receipt_id = %q, want NULL when none uploaded", *receipt)
	}
	if status(t, a, rid) != "awaiting_review" {
		t.Fatalf("status %s, want awaiting_review", status(t, a, rid))
	}

	rid2, _, err := a.Create(ctx, delegateInput("student"), key(2))
	if err != nil {
		t.Fatal(err)
	}
	fid := addFile(t, a, rid2, "receipt", tinyPNG(t))
	if err := a.SubmitPayment(ctx, rid2, PaymentInput{
		Reference: "UTR2", Date: today(a), AmountPaise: earlyPaise(t, a, rid2), ReceiptID: fid,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(ctx, "SELECT receipt_id FROM payment_submissions WHERE registration_id=$1", rid2).Scan(&receipt); err != nil {
		t.Fatal(err)
	}
	if receipt == nil || *receipt != fid {
		t.Fatalf("receipt_id = %v, want %s when uploaded", receipt, fid)
	}
}
