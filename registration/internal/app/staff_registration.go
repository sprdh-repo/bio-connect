package app

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// StaffRegistrationInput is a registration staff enter from the console in a
// single form. It is confirmed when saved: either against a bank payment the
// staff member verified, or as complimentary.
type StaffRegistrationInput struct {
	RegistrationInput
	// Payment is "paid" or "complimentary".
	Payment             string `json:"payment"`
	VerifiedReference   string `json:"verified_reference"`
	VerifiedDate        string `json:"verified_date"`
	VerifiedAmountPaise int64  `json:"verified_amount_paise"`
	// Send delivers the passes (and an exhibitor's pack) at once; otherwise
	// nothing is sent and staff send them from the console when ready.
	Send bool   `json:"send"`
	Note string `json:"note"`
	// AcceptAmount accepts a verified amount different from the fee due, with
	// the reason in Note, as in a review (checkPaidAmount).
	AcceptAmount bool `json:"accept_amount"`
	// Spot marks a walk-in registered at the ops desk, which needs less
	// (entrySpot). Only the ops handler sets it.
	Spot bool `json:"-"`
}

// StaffCreate saves and approves a registration entered by staff. Unlike the
// public form it ignores whether registration or the category is open, keeps
// the phones optional, lets an exhibitor start with fewer attendees than its
// passes, and treats the logo as optional. Nothing is emailed unless Send is
// set: there is no registration email, because nothing is left to pay.
func (a *App) StaffCreate(ctx context.Context, in StaffRegistrationInput, key string, logo []byte, staff string) (string, error) {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	rid, e := a.staffCreate(ctx, tx, in, key, logo, staff)
	if e != nil {
		return "", e
	}
	return rid, tx.Commit(ctx)
}

// staffCreate is StaffCreate inside the caller's transaction, so a caller can
// record what the registration is for (a speaker's pass) atomically with it.
func (a *App) staffCreate(ctx context.Context, tx pgx.Tx, in StaffRegistrationInput, key string, logo []byte, staff string) (string, error) {
	if len(key) < 32 || len(key) > 128 {
		return "", errors.New("a 32-128 character Idempotency-Key is required")
	}
	if in.Payment != "paid" && in.Payment != "complimentary" {
		return "", errors.New("choose paid or complimentary")
	}
	if len(in.Note) > 2000 {
		return "", errors.New("note is too long")
	}
	in.FreeToken = ""
	if in.Payment == "complimentary" && in.CouponCode != "" {
		return "", errors.New("a coupon cannot be combined with a complimentary registration")
	}
	var e error
	// Serialize retries, including requests that arrive before the first insert commits.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", hash(key)); e != nil {
		return "", e
	}
	c, e := category(ctx, tx, in.CategoryID)
	if e != nil {
		return "", errors.New("unknown category")
	}
	by := entryStaff
	if in.Spot {
		by = entrySpot
	}
	if e = validateInput(&in.RegistrationInput, c, by); e != nil {
		return "", e
	}
	if c.FreeOnly && in.Payment == "paid" {
		return "", errors.New("this category is complimentary only")
	}
	rh := requestHash(in)
	var old, oldHash string
	e = tx.QueryRow(ctx, "SELECT id,request_hash FROM registrations WHERE idempotency_hash=$1", hash(key)).Scan(&old, &oldHash)
	if e == nil {
		if oldHash != rh {
			return "", errors.New("this submission key was already used for different details")
		}
		return old, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return "", e
	}
	var logoMime string
	if len(logo) > 0 {
		if c.Kind != "exhibitor" {
			return "", errors.New("only exhibitors have an institution logo")
		}
		if logoMime, e = validateUpload(logo, "logo"); e != nil {
			return "", e
		}
	}
	// validateInput already rejected a code that does not apply to this category.
	coupon, _ := lookupCoupon(in.CouponCode, c.ID)
	var quoted int64
	var date time.Time
	ref := normalizeReference(in.VerifiedReference)
	if in.Payment == "paid" {
		if date, e = paymentDate(in.VerifiedDate, a.Now()); e != nil {
			return "", e
		}
		quoted = payable(c, coupon.PercentOff, date)
		if e = checkPaidAmount(in.VerifiedAmountPaise, quoted, in.AcceptAmount, in.Note); e != nil {
			return "", e
		}
		if !validText(ref, 100) {
			return "", errors.New("verified bank reference is required")
		}
	}
	rid := id()
	reference, e := nextReference(ctx, tx, c.ID)
	if e != nil {
		return "", e
	}
	// The management link is never sent, so its token is discarded; the
	// contact can still reach the registration through Recover registration.
	_, e = tx.Exec(ctx, `INSERT INTO registrations(id,reference,idempotency_hash,request_hash,category_id,institution,contact_name,email,phone,description,status,quoted_paise,management_hash,management_expires,roster_count,coupon_code,discount_percent,approved_at,approved_by,review_note,created_by,complimentary)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'approved',$11,$12,$13,$14,$15,$16,$17,$18,$19,$18,$20)`,
		rid, reference, hash(key), rh, c.ID, in.Institution, in.ContactName, in.Email, in.Phone, in.Description, quoted, hash(randomToken()), a.Now().Add(30*24*time.Hour), c.RosterCount, coupon.Code, coupon.PercentOff, a.Now(), staff, in.Note, in.Payment == "complimentary")
	if e != nil {
		return "", e
	}
	if in.Payment == "paid" {
		_, e = tx.Exec(ctx, `INSERT INTO payment_submissions(
			id,registration_id,bank_reference,payment_date,amount_paise,
			verified_at,verified_by,verified_reference,verified_date,verified_amount_paise,beneficiary_confirmed
		) VALUES($1,$2,$3,$4,$5,now(),$6,$3,$4,$5,true)`, id(), rid, ref, date, in.VerifiedAmountPaise, staff)
		if e != nil {
			return "", e
		}
		if e = noteReusedReference(ctx, tx, staff, rid, ref); e != nil {
			return "", e
		}
		if in.VerifiedAmountPaise != quoted {
			if e = audit(ctx, tx, staff, rid, "amount_accepted", amountOverride(in.VerifiedAmountPaise, quoted, in.Note)); e != nil {
				return "", e
			}
		}
	}
	if logoMime != "" {
		fid := id()
		if e = a.Storage.Put(ctx, fid, logo, logoMime); e != nil {
			return "", errors.New("upload failed; please retry")
		}
		if _, e = tx.Exec(ctx, "INSERT INTO files(id,registration_id,kind,object_key,mime,size) VALUES($1,$2,$3,$1,$4,$5)", fid, rid, "logo", logoMime, len(logo)); e != nil {
			return "", e
		}
	}
	for i, p := range in.Attendees {
		var at *time.Time
		ct := ""
		if p.WhatsAppConsent {
			now := a.Now()
			at = &now
			ct = consentText
		}
		_, e = tx.Exec(ctx, `INSERT INTO attendees(id,registration_id,name,email,phone,designation,whatsapp_consent,consent_at,consent_text,position) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id(), rid, p.Name, p.Email, p.Phone, p.Designation, p.WhatsAppConsent, at, ct, i)
		if e != nil {
			return "", e
		}
	}
	if e = a.issuePasses(ctx, tx, rid, c.RosterCount); e != nil {
		return "", e
	}
	if in.Send {
		if e = a.queuePasses(ctx, tx, rid, "", "initial"); e != nil {
			return "", e
		}
	}
	detail := c.ID + " " + in.Payment
	if in.Send {
		detail += " sent"
	}
	if e = audit(ctx, tx, staff, rid, "staff_registered", detail); e != nil {
		return "", e
	}
	return rid, nil
}
