package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscountedRoundsToWholeRupees(t *testing.T) {
	for _, tc := range []struct {
		paise   int64
		percent int
		want    int64
	}{
		{600000, 30, 420000},
		{350000, 30, 245000},
		{20000000, 30, 14000000},
		{123456, 30, 86400}, // 864.19 rounds to 864
		{123456, 0, 123456},
	} {
		if got := discounted(tc.paise, tc.percent); got != tc.want {
			t.Errorf("discounted(%d, %d) = %d, want %d", tc.paise, tc.percent, got, tc.want)
		}
	}
}

func TestCouponValidity(t *testing.T) {
	for _, tc := range []struct {
		code, cat string
		ok        bool
	}{
		{"KSUM30", "industry", true},
		{"KSUM30", "startup", true},
		{"KSUM30", "faculty", true},
		{"KSUM30", "premium", true},
		{"KSUM30", "table", true},
		{"KSUM30", "student", false},
		{"KMTC30", "industry", true},
		{"KMTC30", "startup", true},
		{"KMTC30", "faculty", true},
		{"KMTC30", "premium", true},
		{"KMTC30", "table", true},
		{"KMTC30", "student", false},
		{"KSUM40", "industry", false},
	} {
		_, err := lookupCoupon(tc.code, tc.cat)
		if (err == nil) != tc.ok {
			t.Errorf("lookupCoupon(%q, %q) err=%v, want ok=%v", tc.code, tc.cat, err, tc.ok)
		}
	}
	if couponEligible("student") || !couponEligible("startup") {
		t.Error("coupon eligibility must exclude students only")
	}
}

func categoryByID(t *testing.T, a *App, id string) Category {
	t.Helper()
	cats, err := a.categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no category %s", id)
	return Category{}
}

func TestCouponRegistrationOwesOfferPrice(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	in := delegateInput("industry")
	in.CouponCode = "  ksum30 "
	rid, _, err := a.Create(ctx, in, key(1), nil)
	if err != nil {
		t.Fatalf("create with coupon: %v", err)
	}
	reg, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	full := fee(categoryByID(t, a, "industry"), a.Now())
	offer := discounted(full, 30)
	if reg.CouponCode != "KSUM30" || reg.DiscountPercent != 30 || reg.QuotedPaise != offer {
		t.Fatalf("registration coupon=%q discount=%d quoted=%d, want KSUM30 30 %d", reg.CouponCode, reg.DiscountPercent, reg.QuotedPaise, offer)
	}
	payDelegate(t, a, rid, offer)
	if err := tryApprove(a, rid, sid, "approve_only", "UTRFULL", full); err == nil {
		t.Fatal("approval accepted the full fee for a coupon registration")
	}
	if err := tryApprove(a, rid, sid, "approve_only", "UTROFFER", offer); err != nil {
		t.Fatalf("offer price rejected: %v", err)
	}
	if status(t, a, rid) != "approved" {
		t.Fatalf("status %s, want approved", status(t, a, rid))
	}
}

func TestCouponRejectedForStudentsAndUnknownCodes(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	for i, tc := range []struct{ cat, code string }{{"student", "KSUM30"}, {"industry", "NOPE"}} {
		in := delegateInput(tc.cat)
		in.CouponCode = tc.code
		if _, _, err := a.Create(ctx, in, key(i+1), nil); err == nil || !strings.Contains(err.Error(), "coupon") {
			t.Fatalf("%s with %s: err=%v, want coupon error", tc.cat, tc.code, err)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM registrations"); n != 0 {
		t.Fatalf("%d registrations saved with invalid coupons", n)
	}
}

func TestCouponAPIAndBankDetails(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	h := a.Handler()

	check := func(body string) (int, map[string]any) {
		r := httptest.NewRequest("POST", "/api/v1/coupons", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	code, out := check(`{"code":"ksum30","category_id":"startup"}`)
	want := discounted(fee(categoryByID(t, a, "startup"), a.Now()), 30)
	if code != 200 || out["code"] != "KSUM30" || int64(out["payable_paise"].(float64)) != want {
		t.Fatalf("coupon check = %d %v, want 200 KSUM30 %d", code, out, want)
	}
	if code, _ := check(`{"code":"KSUM30","category_id":"student"}`); code != 400 {
		t.Fatalf("student coupon check = %d, want 400", code)
	}

	details := func(rid, token string) map[string]any {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, bearerReq(rid, token))
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
			t.Fatalf("details %d: %s", w.Code, w.Body.String())
		}
		return out
	}
	in := delegateInput("startup")
	in.CouponCode = "KSUM30"
	rid, tok, err := a.Create(ctx, in, key(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	d := details(rid, tok)
	bank, ok := d["bank_transfer"].(map[string]any)
	if !ok || bank["account_number"] != "39835516868" || bank["ifsc"] != "SBIN0005317" {
		t.Fatalf("coupon registration bank_transfer = %v", d["bank_transfer"])
	}
	if int64(d["payable_paise"].(float64)) != want {
		t.Fatalf("payable_paise = %v, want %d", d["payable_paise"], want)
	}

	plain := delegateInput("faculty")
	plain.Email, plain.Attendees[0].Email = "f@example.com", "f@example.com"
	rid2, tok2, err := a.Create(ctx, plain, key(2), nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := details(rid2, tok2); d["bank_transfer"] != nil {
		t.Fatalf("registration without coupon got bank details: %v", d["bank_transfer"])
	}
}
