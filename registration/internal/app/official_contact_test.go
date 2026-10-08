package app

import (
	"context"
	"testing"
	"time"
)

func officialInput(email, phone string, whatsapp bool) RegistrationInput {
	return RegistrationInput{
		CategoryID: "official", Institution: "Department of Science and Technology",
		Attendees: []Attendee{{Name: "Meera Das", Email: email, Phone: phone, Designation: "Joint Secretary", WhatsAppConsent: whatsapp}},
	}
}

func TestOfficialNeedsEmailOrPhone(t *testing.T) {
	official := Category{ID: "official", Kind: "delegate", RosterCount: 1}
	for _, tc := range []struct {
		name         string
		email, phone string
		whatsapp     bool
		ok           bool
	}{
		{"email only", "meera@kerala.gov.in", "", false, true},
		{"phone only with WhatsApp", "", "+919876543210", true, true},
		{"both", "meera@kerala.gov.in", "+919876543210", false, true},
		{"phone only without WhatsApp", "", "+919876543210", false, false},
		{"neither", "", "", false, false},
		{"bad email", "not-an-email", "+919876543210", true, false},
		{"bad phone", "meera@kerala.gov.in", "98765", false, false},
	} {
		for _, by := range []entry{entryPublic, entryStaff} {
			in := officialInput(tc.email, tc.phone, tc.whatsapp)
			err := validateInput(&in, official, by)
			if (err == nil) != tc.ok {
				t.Fatalf("%s (entry=%v): err=%v, want ok=%v", tc.name, by, err, tc.ok)
			}
			if err == nil && (in.Email != tc.email || in.Phone != tc.phone || in.ContactName != "Meera Das") {
				t.Fatalf("%s: contact not copied from the attendee: %+v", tc.name, in)
			}
		}
	}
}

func TestOtherDelegatesStillNeedEmail(t *testing.T) {
	in := delegateInput("industry")
	in.Attendees[0].Email, in.Email = "", ""
	in.Attendees[0].WhatsAppConsent = true
	if err := validateInput(&in, Category{ID: "industry", Kind: "delegate", RosterCount: 1}, entryPublic); err == nil {
		t.Fatal("an industry delegate registered without an email")
	}
	in = delegateInput("industry")
	in.Institution = ""
	if err := validateInput(&in, Category{ID: "industry", Kind: "delegate", RosterCount: 1}, entryPublic); err == nil {
		t.Fatal("a delegate registered without an institution")
	}
}

func TestPhoneOnlyOfficialGetsPassByWhatsApp(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	fixed := time.Date(2026, 10, 6, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	token := randomToken()
	addFreeLink(t, a, token, fixed.Add(24*time.Hour), true, "delegate")

	in := officialInput("", "+919876543210", true)
	in.FreeToken = token
	rid, managementToken, err := a.Create(ctx, in, key(990), nil)
	if err != nil {
		t.Fatal(err)
	}
	if managementToken == "" || status(t, a, rid) != "approved" {
		t.Fatal("phone-only official was not approved through the link")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND channel='email'", rid); n != 0 {
		t.Fatalf("email deliveries=%d for an official without an email", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND channel='whatsapp' AND recipient='+919876543210'", rid); n != 1 {
		t.Fatalf("WhatsApp pass deliveries=%d, want 1", n)
	}
	var pid, aid string
	if err = a.DB.QueryRow(ctx, "SELECT p.id,p.attendee_id FROM passes p WHERE p.registration_id=$1", rid).Scan(&pid, &aid); err != nil {
		t.Fatal(err)
	}
	sid, _ := addStaff(t, a, "official-review@bioconnect.test", "reviewer")
	if err = a.Review(ctx, rid, sid, ReviewInput{Action: "resend", PassID: pid, Channel: "email", RequestID: key(991)}); err == nil {
		t.Fatal("an email resend was accepted for an attendee with no email")
	}

	// Staff can still correct the record, and can add an email later.
	if err = a.UpdateAttendee(ctx, rid, aid, sid, Attendee{Name: "Meera Das", Designation: "Additional Secretary", Phone: "+919876543210", WhatsAppConsent: true}); err != nil {
		t.Fatalf("edit phone-only official: %v", err)
	}
	if err = a.UpdateAttendee(ctx, rid, aid, sid, Attendee{Name: "Meera Das", Designation: "Additional Secretary", Phone: "+919876543210"}); err == nil {
		t.Fatal("staff removed WhatsApp consent from an official with no email")
	}
	if err = a.UpdateAttendee(ctx, rid, aid, sid, Attendee{Name: "Meera Das", Designation: "Additional Secretary", Email: "meera@kerala.gov.in", Phone: "+919876543210"}); err != nil {
		t.Fatalf("add email to official: %v", err)
	}
	var email string
	if err = a.DB.QueryRow(ctx, "SELECT email FROM registrations WHERE id=$1", rid).Scan(&email); err != nil || email != "meera@kerala.gov.in" {
		t.Fatalf("registration email=%q err=%v", email, err)
	}
}
