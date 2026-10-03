package app

import (
	"context"
	"strings"
	"testing"
)

func staffInput(in RegistrationInput, payment string) StaffRegistrationInput {
	return StaffRegistrationInput{RegistrationInput: in, Payment: payment}
}

// A complimentary registration entered by staff is confirmed at once, ignores
// closed registration and categories, needs no phone, and sends nothing until
// staff choose to.
func TestStaffComplimentaryDelegateWithoutPhone(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	a.Config.RegistrationEnabled = false
	if _, err := a.DB.Exec(ctx, "UPDATE categories SET open=false WHERE id='faculty'"); err != nil {
		t.Fatal(err)
	}
	in := delegateInput("faculty")
	in.Phone, in.Attendees[0].Phone = "", ""
	rid, err := a.StaffCreate(ctx, staffInput(in, "complimentary"), key(1), nil, sid)
	if err != nil {
		t.Fatalf("staff create: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || !r.Free || !r.Complimentary || r.QuotedPaise != 0 || r.Phone != "" {
		t.Fatalf("registration = %+v", r)
	}
	if n := count(t, a, "SELECT count(*) FROM registrations WHERE id=$1 AND created_by=$2 AND approved_by=$2", rid, sid); n != 1 {
		t.Fatal("created_by / approved_by not recorded")
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != 1 {
		t.Fatalf("passes = %d, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("confirm only queued %d deliveries", n)
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1", rid); n != 0 {
		t.Fatal("complimentary registration recorded a payment")
	}
}

// A paid exhibitor is approved against the verified payment, may start with
// fewer attendees than its passes, and "send" delivers passes and the pack but
// no registration email.
func TestStaffPaidExhibitorPartialRosterAndSend(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	roster := rosterOf(t, a, "standard")
	in := staffInput(exhibitorInput("standard", 1), "paid")
	in.Send = true
	in.VerifiedReference, in.VerifiedDate = "sbi staff 001", today(a)
	cats, err := a.categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.ID == "standard" {
			in.VerifiedAmountPaise = payable(c, 0, a.Now())
		}
	}

	wrong := in
	wrong.VerifiedAmountPaise--
	if _, err := a.StaffCreate(ctx, wrong, key(1), nil, sid); err == nil || !strings.Contains(err.Error(), "must equal") {
		t.Fatalf("mismatched amount accepted: %v", err)
	}

	rid, err := a.StaffCreate(ctx, in, key(2), tinyPNG(t), sid)
	if err != nil {
		t.Fatalf("staff create: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || r.Free || r.RosterCount != roster || len(r.Attendees) != 1 {
		t.Fatalf("registration = %+v", r)
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1 AND verified_by=$2 AND verified_reference='SBISTAFF001' AND beneficiary_confirmed", rid, sid); n != 1 {
		t.Fatal("verified payment not recorded")
	}
	if n := count(t, a, "SELECT count(*) FROM files WHERE registration_id=$1 AND kind='logo'", rid); n != 1 {
		t.Fatal("logo not stored")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != 1 {
		t.Fatalf("pass deliveries = %d, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack'", rid); n != 1 {
		t.Fatal("pack not queued")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='registration'", rid); n != 0 {
		t.Fatal("registration email queued for a staff registration")
	}
	// The open places are filled later like any other exhibitor's.
	if err := a.AddAttendee(ctx, rid, sid, Attendee{Name: "Late Rep", Email: "late@example.com", Designation: "Engineer"}); err != nil {
		t.Fatalf("add attendee without phone: %v", err)
	}

	// The same bank reference cannot confirm a second registration.
	again := in
	again.Institution = "Another Lab"
	if _, err := a.StaffCreate(ctx, again, key(3), nil, sid); err == nil {
		t.Fatal("bank reference reused")
	}
}

func TestStaffCreateIsIdempotent(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	in := staffInput(delegateInput("student"), "complimentary")
	first, err := a.StaffCreate(ctx, in, key(1), nil, sid)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.StaffCreate(ctx, in, key(1), nil, sid)
	if err != nil || second != first {
		t.Fatalf("replay = %q, %v; want %q", second, err, first)
	}
	in.Institution = "Changed"
	if _, err := a.StaffCreate(ctx, in, key(1), nil, sid); err == nil {
		t.Fatal("key reused for different details")
	}
	if n := count(t, a, "SELECT count(*) FROM registrations"); n != 1 {
		t.Fatalf("registrations = %d, want 1", n)
	}
}

func TestStaffCreateRejectsInvalidInput(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	cases := map[string]func() StaffRegistrationInput{
		"no payment choice": func() StaffRegistrationInput { return staffInput(delegateInput("student"), "") },
		"whatsapp without phone": func() StaffRegistrationInput {
			in := staffInput(delegateInput("student"), "complimentary")
			in.Attendees[0].Phone, in.Attendees[0].WhatsAppConsent = "", true
			return in
		},
		"malformed phone": func() StaffRegistrationInput {
			in := staffInput(delegateInput("student"), "complimentary")
			in.Attendees[0].Phone = "98765"
			return in
		},
		"paid complimentary-only category": func() StaffRegistrationInput {
			in := staffInput(delegateInput("official"), "paid")
			in.VerifiedReference, in.VerifiedDate, in.VerifiedAmountPaise = "SBIGOV", today(a), 1
			return in
		},
		"coupon on complimentary": func() StaffRegistrationInput {
			in := staffInput(delegateInput("industry"), "complimentary")
			in.CouponCode = "KMTC25"
			return in
		},
		"too many exhibitor attendees": func() StaffRegistrationInput {
			return staffInput(exhibitorInput("table", rosterOf(t, a, "table")+1), "complimentary")
		},
		"logo on a delegate": func() StaffRegistrationInput { return staffInput(delegateInput("student"), "complimentary") },
	}
	i := 0
	for name, build := range cases {
		i++
		var logo []byte
		if name == "logo on a delegate" {
			logo = tinyPNG(t)
		}
		if _, err := a.StaffCreate(ctx, build(), key(i), logo, sid); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM registrations"); n != 0 {
		t.Fatalf("invalid input saved %d registrations", n)
	}
}

// The public form still requires phones, including when an exhibitor fills an
// open place themselves.
func TestPublicFormsStillRequirePhone(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	in := delegateInput("student")
	in.Phone, in.Attendees[0].Phone = "", ""
	if _, _, err := a.Create(ctx, in, key(1), nil); err == nil {
		t.Fatal("public registration without phone accepted")
	}
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	staff := staffInput(exhibitorInput("standard", 1), "complimentary")
	rid, err := a.StaffCreate(ctx, staff, key(2), nil, sid)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddAttendee(ctx, rid, "", Attendee{Name: "Self Added", Email: "self@example.com", Designation: "CEO"}); err == nil {
		t.Fatal("exhibitor added an attendee without phone")
	}
}
