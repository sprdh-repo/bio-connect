package app

import (
	"context"
	"strings"
	"testing"
)

func speakerRow(t *testing.T, a *App, id string) speakerPassRow {
	t.Helper()
	rows, err := a.speakerPasses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range rows {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("speaker %s not listed", id)
	return speakerPassRow{}
}

// Saving a speaker's email issues one approved, complimentary SPEAKER pass
// from the directory details and sends nothing; a second save corrects the
// holder instead of issuing another pass.
func TestSpeakerPassIssuedOnceFromDirectory(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	const speaker = "beena-pillai"

	if s := speakerRow(t, a, speaker); s.Registration != nil || s.Pass != nil {
		t.Fatalf("speaker starts with a pass: %+v", s)
	}
	rid, err := a.SaveSpeakerContact(ctx, speaker, " Beena@Example.org ", "", false, key(1), sid)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || r.CategoryID != "speaker" || !r.Complimentary || r.Institution != "BRIC-Rajiv Gandhi Centre for Biotechnology" || !strings.HasPrefix(r.Reference, "BC4-SK-") {
		t.Fatalf("registration = %+v", r)
	}
	s := speakerRow(t, a, speaker)
	if s.Attendee == nil || s.Attendee.Email != "beena@example.org" || s.Pass == nil || s.Delivery != nil {
		t.Fatalf("row after save = %+v", s)
	}
	if n := count(t, a, "SELECT count(*) FROM attendees WHERE registration_id=$1 AND name='Dr. Beena Pillai' AND designation='Director'", rid); n != 1 {
		t.Fatal("pass holder not taken from the directory")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("saving queued %d deliveries", n)
	}

	again, err := a.SaveSpeakerContact(ctx, speaker, "beena.pillai@example.org", "+919876543210", true, key(2), sid)
	if err != nil || again != rid {
		t.Fatalf("second save = %s, %v; want the same registration", again, err)
	}
	if n := count(t, a, "SELECT count(*) FROM registrations WHERE category_id='speaker'"); n != 1 {
		t.Fatalf("speaker registrations = %d, want 1", n)
	}
	s = speakerRow(t, a, speaker)
	if s.Attendee.Email != "beena.pillai@example.org" || s.Attendee.Phone != "+919876543210" || !s.Attendee.WhatsAppConsent {
		t.Fatalf("contact not corrected: %+v", s.Attendee)
	}

	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send", Channel: "email"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if s = speakerRow(t, a, speaker); s.Delivery == nil || s.Delivery.Status != "queued" {
		t.Fatalf("delivery after send = %+v", s.Delivery)
	}
}

// A cancelled speaker pass is replaced by a new registration on the next save.
func TestSpeakerPassReissuedAfterCancellation(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	const speaker = "t-p-singh"
	first, err := a.SaveSpeakerContact(ctx, speaker, "tp@example.org", "", false, key(1), sid)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Review(ctx, first, sid, ReviewInput{Action: "cancelled", Note: "duplicate"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if s := speakerRow(t, a, speaker); s.Registration == nil || s.Registration.Status != "cancelled" || s.Pass != nil {
		t.Fatalf("cancelled row = %+v", s)
	}
	second, err := a.SaveSpeakerContact(ctx, speaker, "tp@example.org", "", false, key(2), sid)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("cancelled registration reused")
	}
	if s := speakerRow(t, a, speaker); s.Registration.ID != second || s.Pass == nil {
		t.Fatalf("row after reissue = %+v", s)
	}
}

func TestSpeakerPassRejectsBadInput(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	if _, err := a.SaveSpeakerContact(ctx, "no-such-speaker", "x@example.org", "", false, key(1), sid); err == nil || !strings.Contains(err.Error(), "no longer in the speaker list") {
		t.Fatalf("unknown speaker: %v", err)
	}
	if _, err := a.SaveSpeakerContact(ctx, "t-p-singh", "not-an-email", "", false, key(2), sid); err == nil {
		t.Fatal("invalid email accepted")
	}
	if _, err := a.SaveSpeakerContact(ctx, "t-p-singh", "tp@example.org", "", true, key(3), sid); err == nil {
		t.Fatal("WhatsApp consent without a phone accepted")
	}
	if n := count(t, a, "SELECT count(*) FROM registrations WHERE category_id='speaker'"); n != 0 {
		t.Fatalf("rejected saves left %d registrations", n)
	}
}

// Speakers register only through the console: no public form or free link.
func TestSpeakerCategoryIsStaffOnly(t *testing.T) {
	a := mustApp(t)
	a.Config.RegistrationEnabled = true
	in := delegateInput("speaker")
	if _, _, err := a.Create(context.Background(), in, key(1), nil); err == nil {
		t.Fatal("public speaker registration accepted")
	}
}
