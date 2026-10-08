package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func foodInput(p Attendee) StaffRegistrationInput {
	return staffInput(RegistrationInput{CategoryID: "food", Attendees: []Attendee{p}}, "complimentary")
}

// A food pass needs a name and an email or a phone. Institution and
// designation are optional, the pass is issued in the FD series, and the
// category is closed to the public form.
func TestFoodPassNeedsNameAndOneContact(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")

	phoneOnly := foodInput(Attendee{Name: " Suresh K ", Phone: "+919876543210", WhatsAppConsent: true})
	rid, err := a.StaffCreate(ctx, phoneOnly, key(1), nil, sid)
	if err != nil {
		t.Fatalf("driver with only a phone: %v", err)
	}
	r, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || !r.Complimentary || r.Reference != "BC4-FD-0001" || r.Institution != "" || r.Attendees[0].Name != "Suresh K" || r.Attendees[0].Designation != "" {
		t.Fatalf("registration = %+v", r)
	}
	var pid string
	if err = a.DB.QueryRow(ctx, "SELECT id FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid).Scan(&pid); err != nil {
		t.Fatalf("pass not issued: %v", err)
	}
	if pdf, err := a.passPDF(ctx, pid); err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("food pass did not render: %v", err)
	}

	emailOnly := foodInput(Attendee{Name: "Biju", Designation: "Driver", Email: "biju@example.com"})
	emailOnly.Institution = "Travels"
	if _, err := a.StaffCreate(ctx, emailOnly, key(2), nil, sid); err != nil {
		t.Fatalf("driver with only an email: %v", err)
	}

	for name, p := range map[string]Attendee{
		"no name":            {Phone: "+919876543210", WhatsAppConsent: true},
		"no contact":         {Name: "A"},
		"phone, no whatsapp": {Name: "A", Phone: "+919876543210"},
		"local phone":        {Name: "A", Phone: "9876543210", WhatsAppConsent: true},
	} {
		if _, err := a.StaffCreate(ctx, foodInput(p), key(10+len(name)), nil, sid); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	public := delegateInput("food")
	if _, _, err := a.Create(ctx, public, key(40), nil); err == nil {
		t.Fatal("public form accepted a food pass")
	}

	cats, err := a.categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.FoodOnly != (c.ID == "food") || c.ID == "food" && (c.Open || c.FreeOpen || !c.FreeOnly || c.DownloadOnly) {
			t.Fatalf("category %+v", c)
		}
	}
}

// Every message a food pass holder receives says FOOD ONLY: the email subject
// and body, the attachment name, the WhatsApp document name and, when one is
// configured, the food WhatsApp template. Other passes are unchanged.
func TestFoodPassMessagesSayFoodOnly(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	// WorkOnce sends synchronously, so each capture is complete when it returns.
	var email, whatsapp map[string]any
	var media string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/email"):
			json.NewDecoder(r.Body).Decode(&email)
			json.NewEncoder(w).Encode(map[string]any{"MessageID": "pm-" + id(), "ErrorCode": 0})
		case strings.HasSuffix(r.URL.Path, "/media"):
			b, _ := io.ReadAll(r.Body)
			media = string(b)
			json.NewEncoder(w).Encode(map[string]string{"id": "media-" + id()})
		default:
			json.NewDecoder(r.Body).Decode(&whatsapp)
			json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]string{{"id": "wamid-" + id()}}})
		}
	}))
	defer srv.Close()
	a.Config.LiveDelivery = true
	a.Config.PostmarkToken, a.Config.SenderAddress, a.Config.SenderName, a.Config.PostmarkAPIBase = "t", "events@zinvos.example", "Zinvos Events", srv.URL
	a.Config.MetaToken, a.Config.MetaPhoneID, a.Config.MetaVersion, a.Config.MetaAPIBase = "t", "phone", "v21.0", srv.URL
	a.Config.MetaTemplate, a.Config.MetaFoodTemplate = "pass_delivery", "food_pass_delivery"
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	n := 0
	send := func(in StaffRegistrationInput) {
		t.Helper()
		n++
		email, whatsapp, media = nil, nil, ""
		in.Send = true
		if _, err := a.StaffCreate(ctx, in, key(n), nil, sid); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := a.WorkOnce(ctx); err != nil {
				t.Fatalf("WorkOnce: %v", err)
			}
		}
		if email == nil || whatsapp == nil {
			t.Fatalf("sent email %v, whatsapp %v", email != nil, whatsapp != nil)
		}
	}
	const file = "Bio-Connect-4.0-FOOD-ONLY-pass.pdf"
	template := func() string { return whatsapp["template"].(map[string]any)["name"].(string) }

	send(foodInput(Attendee{Name: "Suresh K", Email: "suresh@example.com", Phone: "+919876543210", WhatsAppConsent: true}))
	text, _ := email["TextBody"].(string)
	atts, _ := email["Attachments"].([]any)
	if email["Subject"] != "Bio Connect 4.0 - your food pass (meals only)" || !strings.Contains(text, "It is for food only") || !strings.Contains(text, "Pass number: BC4-FD-") || len(atts) != 1 || atts[0].(map[string]any)["Name"] != file {
		t.Fatalf("food email = subject %q, body %q, attachments %v", email["Subject"], text, atts)
	}
	if template() != "food_pass_delivery" || !strings.Contains(fmtJSON(whatsapp), file) || !strings.Contains(media, file) {
		t.Fatalf("food whatsapp = %s", fmtJSON(whatsapp))
	}

	// Until a food template is approved, the standard one carries the
	// FOOD-ONLY document.
	a.Config.MetaFoodTemplate = ""
	send(foodInput(Attendee{Name: "Biju", Email: "biju@example.com", Phone: "+919876543211", WhatsAppConsent: true}))
	if template() != "pass_delivery" || !strings.Contains(fmtJSON(whatsapp), file) {
		t.Fatalf("food whatsapp without its template = %s", fmtJSON(whatsapp))
	}

	a.Config.MetaFoodTemplate = "food_pass_delivery"
	student := staffInput(delegateInput("student"), "complimentary")
	student.Attendees[0].WhatsAppConsent = true
	send(student)
	if email["Subject"] != "Bio Connect 4.0 - your pass" || template() != "pass_delivery" || strings.Contains(fmtJSON(whatsapp)+fmtJSON(email["Attachments"]), "FOOD") {
		t.Fatalf("student pass = subject %q, whatsapp %s", email["Subject"], fmtJSON(whatsapp))
	}
}

func fmtJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
