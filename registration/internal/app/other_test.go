package app

import (
	"bytes"
	"context"
	"testing"
)

func otherInput(p Attendee) StaffRegistrationInput {
	return staffInput(RegistrationInput{CategoryID: "other", Attendees: []Attendee{p}}, "complimentary")
}

// Other is the staff-only catch-all: only a name is required, the pass is
// issued in the OT series and renders, and any email or phone given must
// still be valid.
func TestOtherPassNeedsOnlyAName(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")

	rid, err := a.StaffCreate(ctx, otherInput(Attendee{Name: " Anyone "}), key(1), nil, sid)
	if err != nil {
		t.Fatalf("name only: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || !r.Complimentary || r.Reference != "BC4-OT-0001" || r.Institution != "" || r.Attendees[0].Name != "Anyone" {
		t.Fatalf("registration = %+v", r)
	}
	var pid string
	if err = a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid).Scan(&pid); err != nil {
		t.Fatalf("pass not issued: %v", err)
	}
	if pdf, err := a.passPDF(ctx, pid); err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("other pass did not render: %v", err)
	}

	full := otherInput(Attendee{Name: "B", Designation: "Consultant", Email: "b@example.com", Phone: "+919876543210", WhatsAppConsent: true})
	full.Institution = "Somewhere"
	if _, err := a.StaffCreate(ctx, full, key(2), nil, sid); err != nil {
		t.Fatalf("full details: %v", err)
	}

	for name, p := range map[string]Attendee{
		"no name":           {Email: "c@example.com"},
		"bad email":         {Name: "C", Email: "not-an-email"},
		"local phone":       {Name: "C", Phone: "9876543210"},
		"whatsapp no phone": {Name: "C", WhatsAppConsent: true},
	} {
		if _, err := a.StaffCreate(ctx, otherInput(p), key(10+len(name)), nil, sid); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	if _, _, err := a.Create(ctx, delegateInput("other"), key(40), nil); err == nil {
		t.Fatal("public form accepted an Other pass")
	}
	cats, err := a.categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.ID == "other" && (!c.AnyDetails || !c.FreeOnly || c.Open) {
			t.Fatalf("other category = %+v", c)
		}
	}
}
