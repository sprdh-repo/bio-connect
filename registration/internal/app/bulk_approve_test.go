package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Bulk approve-and-send takes free registrations awaiting review straight to
// approved with their passes sent, and refuses everything else per row: a
// paid registration needs its payment verified, and an approved one is done.
func TestBulkApproveFreeRegistrations(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	request := staffRequester(t, a, "bulk@bioconnect.test", "reviewer")
	token := randomToken()
	addFreeLink(t, a, token, a.Now().Add(48*time.Hour), false, "delegate")

	free := func(n int, email string) string {
		in := delegateInput("industry")
		in.FreeToken = token
		in.Email, in.Attendees[0].Email = email, email
		rid, _, err := a.Create(ctx, in, key(n), nil)
		if err != nil {
			t.Fatal(err)
		}
		return rid
	}
	one, two := free(1, "one@example.com"), free(2, "two@example.com")
	paidIn := delegateInput("industry")
	paidIn.Email, paidIn.Attendees[0].Email = "paid@example.com", "paid@example.com"
	paid, _, err := a.Create(ctx, paidIn, key(3), nil)
	if err != nil {
		t.Fatal(err)
	}
	payDelegate(t, a, paid, dueNow(t, a, paid))

	rr := request("POST", "/api/v1/admin/bulk-approve", map[string]any{"ids": []string{one, two, paid}})
	if rr.Code != 200 {
		t.Fatalf("bulk approve: %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out[one] != "queued" || out[two] != "queued" || !strings.Contains(out[paid], "verify the payment") {
		t.Fatalf("results = %v", out)
	}
	for _, rid := range []string{one, two} {
		if status(t, a, rid) != "approved" {
			t.Fatalf("%s status = %s", rid, status(t, a, rid))
		}
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND channel='email'", rid); n != 1 {
			t.Fatalf("%s pass emails = %d, want 1", rid, n)
		}
	}
	if status(t, a, paid) != "awaiting_review" {
		t.Fatalf("paid registration approved in bulk: %s", status(t, a, paid))
	}

	// Approving again is refused, so nobody is sent a second pass.
	rr = request("POST", "/api/v1/admin/bulk-approve", map[string]any{"ids": []string{one}})
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out[one] != "not awaiting review" {
		t.Fatalf("second approve = %v", out)
	}
	if rr := request("POST", "/api/v1/admin/bulk-approve", map[string]any{"ids": []string{}}); rr.Code != 400 {
		t.Fatalf("empty selection: %d", rr.Code)
	}
}

// A speaker with no contact can be issued a pass for download only. Nothing is
// sent to them; once an email is saved, the same pass can be sent as usual.
func TestSpeakerDownloadOnlyPass(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	const speaker = "raj-shirumalla"

	if _, err := a.IssueSpeakerPass(ctx, speaker, key(1), sid, false); err == nil || !strings.Contains(err.Error(), "download only") {
		t.Fatalf("issue without contact or flag: %v", err)
	}
	rid, err := a.IssueSpeakerPass(ctx, speaker, key(2), sid, true)
	if err != nil {
		t.Fatalf("download-only issue: %v", err)
	}
	s := speakerRow(t, a, speaker)
	if s.Pass == nil || s.Attendee == nil || s.Attendee.Email != "" || s.Attendee.Phone != "" || s.Contact != nil {
		t.Fatalf("download-only row = %+v", s)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("download-only pass queued %d deliveries", n)
	}

	if err := a.SaveSpeakerContact(ctx, speaker, "raj@example.org", "", false, sid); err != nil {
		t.Fatalf("save contact later: %v", err)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "resend", RequestID: strings.Repeat("r", 40)}); err != nil {
		t.Fatalf("resend: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND channel='email' AND recipient='raj@example.org'", rid); n != 1 {
		t.Fatalf("pass email after adding contact = %d, want 1", n)
	}

	// Only speakers may skip contact details; other staff entry still needs them.
	in := staffInput(delegateInput("industry"), "complimentary")
	in.Email, in.Phone, in.Attendees[0].Email, in.Attendees[0].Phone = "", "", "", ""
	if _, err := a.StaffCreate(ctx, in, key(3), nil, sid); err == nil {
		t.Fatal("industry delegate saved without any contact")
	}
}
