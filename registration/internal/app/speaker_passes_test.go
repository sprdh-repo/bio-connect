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

// Saving a speaker's email stores it and issues nothing. Issuing then creates
// one approved, complimentary SPEAKER pass from the directory details, sends
// nothing, and is idempotent; a later contact change corrects the holder.
func TestSpeakerContactThenPass(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	const speaker = "beena-pillai"

	if _, err := a.IssueSpeakerPass(ctx, speaker, key(1), sid, false); err == nil || !strings.Contains(err.Error(), "save this speaker's email") {
		t.Fatalf("issue without contact: %v", err)
	}
	if err := a.SaveSpeakerContact(ctx, speaker, " Beena@Example.org ", "", false, sid); err != nil {
		t.Fatalf("save: %v", err)
	}
	s := speakerRow(t, a, speaker)
	if s.Contact == nil || s.Contact.Email != "beena@example.org" || s.Registration != nil || s.Pass != nil {
		t.Fatalf("row after save = %+v", s)
	}
	if n := count(t, a, "SELECT count(*) FROM registrations WHERE category_id='speaker'"); n != 0 {
		t.Fatalf("saving a contact issued %d registrations", n)
	}

	rid, err := a.IssueSpeakerPass(ctx, speaker, key(2), sid, false)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || r.CategoryID != "speaker" || !r.Complimentary || r.Institution != "BRIC-Rajiv Gandhi Centre for Biotechnology" || !strings.HasPrefix(r.Reference, "BC4-SK-") {
		t.Fatalf("registration = %+v", r)
	}
	if n := count(t, a, "SELECT count(*) FROM attendees WHERE registration_id=$1 AND name='Dr. Beena Pillai' AND designation='Director' AND email='beena@example.org'", rid); n != 1 {
		t.Fatal("pass holder not taken from the directory and contact")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("issuing queued %d deliveries", n)
	}
	if again, err := a.IssueSpeakerPass(ctx, speaker, key(3), sid, false); err != nil || again != rid {
		t.Fatalf("second issue = %s, %v; want the same registration", again, err)
	}

	if err := a.SaveSpeakerContact(ctx, speaker, "beena.pillai@example.org", "+919876543210", true, sid); err != nil {
		t.Fatal(err)
	}
	s = speakerRow(t, a, speaker)
	if s.Attendee.Email != "beena.pillai@example.org" || s.Attendee.Phone != "+919876543210" || !s.Attendee.WhatsAppConsent || s.Contact.Email != "beena.pillai@example.org" {
		t.Fatalf("contact not corrected: %+v %+v", s.Attendee, s.Contact)
	}

	// With a phone and WhatsApp consent, one send goes by email and WhatsApp.
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if s = speakerRow(t, a, speaker); s.Delivery == nil || s.Delivery.Status != "queued" || s.WhatsApp == nil || s.WhatsApp.Status != "queued" {
		t.Fatalf("deliveries after send = %+v %+v", s.Delivery, s.WhatsApp)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND channel='whatsapp' AND recipient='+919876543210'", rid); n != 1 {
		t.Fatalf("whatsapp jobs = %d, want 1", n)
	}
}

// A cancelled speaker pass is replaced by a new registration on the next issue.
func TestSpeakerPassReissuedAfterCancellation(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	const speaker = "t-p-singh"
	if err := a.SaveSpeakerContact(ctx, speaker, "tp@example.org", "", false, sid); err != nil {
		t.Fatal(err)
	}
	first, err := a.IssueSpeakerPass(ctx, speaker, key(1), sid, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Review(ctx, first, sid, ReviewInput{Action: "cancelled", Note: "duplicate"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if s := speakerRow(t, a, speaker); s.Registration == nil || s.Registration.Status != "cancelled" || s.Pass != nil {
		t.Fatalf("cancelled row = %+v", s)
	}
	second, err := a.IssueSpeakerPass(ctx, speaker, key(2), sid, false)
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

func TestSpeakerContactRejectsBadInput(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	for name, c := range map[string][3]string{
		"unknown speaker": {"no-such-speaker", "x@example.org", ""},
		"invalid email":   {"t-p-singh", "not-an-email", ""},
		"space in email":  {"t-p-singh", "a b@example.org", ""},
		"bad phone":       {"t-p-singh", "tp@example.org", "98765"},
	} {
		if err := a.SaveSpeakerContact(ctx, c[0], c[1], c[2], false, sid); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if err := a.SaveSpeakerContact(ctx, "t-p-singh", "tp@example.org", "", true, sid); err == nil {
		t.Fatal("WhatsApp consent without a phone accepted")
	}
	if n := count(t, a, "SELECT count(*) FROM speaker_contacts"); n != 0 {
		t.Fatalf("rejected saves left %d contacts", n)
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
