package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Once the event is over, REGISTRATION_CLOSED refuses every public write, even
// through a free link, while a registration stays readable and staff can still
// register people from the console.
func TestRegistrationClosedRefusesPublicWrites(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "desk@bioconnect.test", "reviewer")
	rid, token, err := a.Create(ctx, delegateInput("industry"), key(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	exhibitor, exhibitorToken, err := a.Create(ctx, exhibitorInput("premium", 5), key(2), tinyPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	freeToken := randomToken()
	addFreeLink(t, a, freeToken, a.Now().Add(24*time.Hour), true, "delegate")

	a.Config.RegistrationEnabled = false
	a.Config.RegistrationClosed = true
	h := a.Handler()
	call := func(method, path, bearer string, body any) *httptest.ResponseRecorder {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr
	}

	if _, _, err := a.Create(ctx, delegateInput("industry"), key(3), nil); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("paid create: %v, want ErrRegistrationClosed", err)
	}
	free := delegateInput("industry")
	free.FreeToken = freeToken
	if _, _, err := a.Create(ctx, free, key(4), nil); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("free-link create: %v, want ErrRegistrationClosed", err)
	}

	for _, tc := range []struct {
		name, path, bearer string
		body               any
	}{
		{"free link check", "/api/v1/free-registration/validate", "", map[string]string{"token": freeToken}},
		{"coupon", "/api/v1/coupons", "", map[string]string{"code": "KSUM30", "category_id": "industry"}},
		{"payment evidence", "/api/v1/registrations/" + rid + "/payments", token, PaymentInput{Reference: "UTR123456", Date: today(a), AmountPaise: dueNow(t, a, rid)}},
		{"added attendee", "/api/v1/registrations/" + exhibitor + "/attendees", exhibitorToken, map[string]string{"name": "Late Rep", "email": "late@example.com", "phone": "+919800000000", "designation": "Scientist"}},
	} {
		if rr := call("POST", tc.path, tc.bearer, tc.body); rr.Code != 410 || !strings.Contains(rr.Body.String(), "has closed") {
			t.Errorf("%s: %d %s, want 410 has closed", tc.name, rr.Code, rr.Body)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM payment_submissions WHERE registration_id=$1", rid); n != 0 {
		t.Fatalf("%d payment submissions saved while closed", n)
	}

	// Only a valid private link learns that writes are closed.
	if rr := call("POST", "/api/v1/registrations/"+rid+"/payments", exhibitorToken, map[string]string{}); rr.Code != 401 {
		t.Fatalf("payment with another registration's token: %d, want 401", rr.Code)
	}
	if rr := call("GET", "/api/v1/registrations/"+rid, token, nil); rr.Code != 200 {
		t.Fatalf("viewing a registration while closed: %d %s", rr.Code, rr.Body)
	}
	var cats struct {
		Enabled bool `json:"registration_enabled"`
		Closed  bool `json:"registration_closed"`
	}
	if rr := call("GET", "/api/v1/categories", "", nil); rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &cats) != nil || cats.Enabled || !cats.Closed {
		t.Fatalf("categories while closed: %d %s", rr.Code, rr.Body)
	}

	// Reminders would only link to the forms that no longer accept anything.
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "payment_reminder"}); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("payment reminder: %v, want ErrRegistrationClosed", err)
	}
	if err := a.Review(ctx, exhibitor, sid, ReviewInput{Action: "roster_notice"}); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("roster notice: %v, want ErrRegistrationClosed", err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE purpose IN ('payment_reminder','roster_notice')"); n != 0 {
		t.Fatalf("%d reminders queued while closed", n)
	}

	if _, err := a.StaffCreate(ctx, staffInput(delegateInput("faculty"), "complimentary"), key(5), nil, sid); err != nil {
		t.Fatalf("staff create while closed: %v", err)
	}
}

func TestRegistrationCannotBeBothEnabledAndClosed(t *testing.T) {
	c := Config{RegistrationEnabled: true, RegistrationClosed: true}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "REGISTRATION_CLOSED") {
		t.Fatalf("Validate = %v, want the enabled/closed conflict", err)
	}
}
