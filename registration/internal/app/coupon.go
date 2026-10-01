package app

import (
	"errors"
	"strings"
	"time"
)

// Coupon is a percentage discount on the published fee. A registration saved
// with a coupon pays by direct bank transfer rather than SBI Collect, and
// submits the transfer reference as payment evidence like everyone else.
type Coupon struct {
	Code       string
	PercentOff int
	// Excluded lists the category ids the coupon cannot be used with.
	Excluded map[string]bool
}

var coupons = map[string]Coupon{
	"KSUM30": {Code: "KSUM30", PercentOff: 30, Excluded: map[string]bool{"student": true}},
	"KMTC25": {Code: "KMTC25", PercentOff: 25, Excluded: map[string]bool{"student": true}},
}

// BankAccount is where coupon registrations transfer their fee. It is the same
// account already published on the marketing site for sponsorships.
type BankAccount struct {
	AccountName   string `json:"account_name"`
	Bank          string `json:"bank"`
	Branch        string `json:"branch"`
	AccountNumber string `json:"account_number"`
	IFSC          string `json:"ifsc"`
}

var bankTransfer = BankAccount{
	AccountName:   "Kerala Lifesciences Industries Parks Private Limited",
	Bank:          "State Bank of India",
	Branch:        "Mangalapuram",
	AccountNumber: "39835516868",
	IFSC:          "SBIN0005317",
}

var errCoupon = errors.New("this coupon code is not valid for the selected category")

func normalizeCoupon(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// lookupCoupon resolves an already normalized code for a category. An empty
// code is no coupon, not an error.
func lookupCoupon(code, catID string) (Coupon, error) {
	if code == "" {
		return Coupon{}, nil
	}
	c, ok := coupons[code]
	if !ok || c.Excluded[catID] {
		return Coupon{}, errCoupon
	}
	return c, nil
}

func couponEligible(catID string) bool {
	for _, c := range coupons {
		if !c.Excluded[catID] {
			return true
		}
	}
	return false
}

// discounted applies a percentage discount, rounded to the nearest rupee so the
// amount to transfer is always a whole number.
func discounted(p int64, percentOff int) int64 {
	if percentOff == 0 {
		return p
	}
	return (p*int64(100-percentOff) + 5000) / 10000 * 100
}

// payable is what a registration owes for a payment made at t: the category's
// fee for that date less the discount frozen on the registration.
func payable(c Category, percentOff int, t time.Time) int64 {
	return discounted(fee(c, t), percentOff)
}
