package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrConflict = errors.New("registration changed or action is not allowed in this state")

const consentText = "I have permission to send this attendee their Bio Connect 4.0 pass and delivery updates by WhatsApp."

func (a *App) Create(ctx context.Context, in RegistrationInput, key string) (string, string, error) {
	if !a.Config.RegistrationEnabled {
		return "", "", errors.New("registration is not open yet")
	}
	if len(key) < 32 || len(key) > 128 {
		return "", "", errors.New("a 32-128 character Idempotency-Key is required")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return "", "", e
	}
	defer tx.Rollback(ctx)
	// Serialize retries, including requests that arrive before the first insert commits.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", hash(key)); e != nil {
		return "", "", e
	}
	c, e := category(ctx, tx, in.CategoryID)
	if e != nil {
		return "", "", errors.New("unknown category")
	}
	if e = validateInput(&in, c); e != nil {
		return "", "", e
	}
	rh := requestHash(in)
	var old, oldHash string
	e = tx.QueryRow(ctx, "SELECT id,request_hash FROM registrations WHERE idempotency_hash=$1", hash(key)).Scan(&old, &oldHash)
	if e == nil {
		if oldHash != rh {
			return "", "", errors.New("this submission key was already used for different details")
		}
		return old, "", tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return "", "", e
	}
	if !c.Open {
		return "", "", errors.New("this category is closed")
	}
	rid, token := id(), randomToken()
	reference, e := nextReference(ctx, tx, c.ID)
	if e != nil {
		return "", "", e
	}
	_, e = tx.Exec(ctx, `INSERT INTO registrations(id,reference,idempotency_hash,request_hash,category_id,institution,contact_name,email,phone,description,quoted_paise,management_hash,management_expires) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, rid, reference, hash(key), rh, c.ID, in.Institution, in.ContactName, in.Email, in.Phone, in.Description, fee(c, a.Now()), hash(token), a.Now().Add(30*24*time.Hour))
	if e != nil {
		return "", "", e
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
			return "", "", e
		}
	}
	if e = a.queue(ctx, tx, rid, "", "registration", "email", in.Email, a.Config.BaseURL+"/manage/"+rid+"#"+token, "registration:"+rid); e != nil {
		return "", "", e
	}
	if e = audit(ctx, tx, "", rid, "registered", c.ID); e != nil {
		return "", "", e
	}
	return rid, token, tx.Commit(ctx)
}

type PaymentInput struct {
	Reference   string `json:"reference"`
	Date        string `json:"date"`
	AmountPaise int64  `json:"amount_paise"`
	ReceiptID   string `json:"receipt_id"`
}

func normalizeReference(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(s)), ""))
}
func (a *App) SubmitPayment(ctx context.Context, rid string, p PaymentInput) error {
	p.Reference = normalizeReference(p.Reference)
	if !validText(p.Reference, 100) || p.AmountPaise <= 0 || p.AmountPaise > 1000000000 {
		return errors.New("provide a valid bank reference and amount")
	}
	date, e := paymentDate(p.Date, a.Now())
	if e != nil {
		return e
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status, cat string
	if e = tx.QueryRow(ctx, "SELECT status,category_id FROM registrations WHERE id=$1 FOR UPDATE", rid).Scan(&status, &cat); e != nil {
		return e
	}
	if status != "awaiting_payment" && status != "correction_requested" {
		return ErrConflict
	}
	// A receipt file is optional now; link it only when the client uploaded one
	// that belongs to this registration.
	var receipt any
	if p.ReceiptID != "" {
		var ok bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM files WHERE id=$1 AND registration_id=$2 AND kind='receipt')", p.ReceiptID, rid).Scan(&ok); e != nil {
			return e
		}
		if ok {
			receipt = p.ReceiptID
		}
	}
	c, e := category(ctx, tx, cat)
	if e != nil {
		return e
	}
	if c.Kind == "exhibitor" {
		var ok bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM files WHERE registration_id=$1 AND kind='logo')", rid).Scan(&ok); e != nil {
			return e
		}
		if !ok {
			return errors.New("upload the institution logo first")
		}
	}
	// Evidence can report an incorrect amount. It is retained for staff correction, never auto-approved.
	_, e = tx.Exec(ctx, "INSERT INTO payment_submissions(id,registration_id,bank_reference,payment_date,amount_paise,receipt_id) VALUES($1,$2,$3,$4,$5,$6)", id(), rid, p.Reference, date, p.AmountPaise, receipt)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE registrations SET status='awaiting_review',updated_at=now() WHERE id=$1", rid); e != nil {
		return e
	}
	if e = audit(ctx, tx, "", rid, "payment_submitted", ""); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type ReviewInput struct {
	Action               string `json:"action"`
	Note                 string `json:"note"`
	PaymentID            string `json:"payment_id"`
	VerifiedReference    string `json:"verified_reference"`
	VerifiedDate         string `json:"verified_date"`
	VerifiedAmountPaise  int64  `json:"verified_amount_paise"`
	Successful           bool   `json:"successful"`
	BeneficiaryConfirmed bool   `json:"beneficiary_confirmed"`
	Channel              string `json:"channel"`
	PassID               string `json:"pass_id"`
	RequestID            string `json:"request_id"`
}

func (a *App) Review(ctx context.Context, rid, staff string, in ReviewInput) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status, cat, email string
	if e = tx.QueryRow(ctx, "SELECT status,category_id,email FROM registrations WHERE id=$1 FOR UPDATE", rid).Scan(&status, &cat, &email); e != nil {
		return e
	}
	if len(in.Note) > 2000 {
		return errors.New("note is too long")
	}
	switch in.Action {
	case "approve_send", "approve_only":
		if status == "approved" {
			return tx.Commit(ctx)
		}
		if status != "awaiting_review" {
			return ErrConflict
		}
		if !in.Successful || !in.BeneficiaryConfirmed {
			return errors.New("confirm successful payment and beneficiary against bank records")
		}
		date, e := paymentDate(in.VerifiedDate, a.Now())
		if e != nil {
			return e
		}
		c, e := category(ctx, tx, cat)
		if e != nil {
			return e
		}
		if in.VerifiedAmountPaise != fee(c, date) {
			return fmt.Errorf("verified payment must equal %s for the verified date", money(fee(c, date)))
		}
		ref := normalizeReference(in.VerifiedReference)
		if !validText(ref, 100) {
			return errors.New("verified bank reference is required")
		}
		var latest string
		if e = tx.QueryRow(ctx, "SELECT id FROM payment_submissions WHERE registration_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1", rid).Scan(&latest); e != nil {
			return e
		}
		if latest != in.PaymentID {
			return errors.New("review the latest payment submission")
		}
		_, e = tx.Exec(ctx, `UPDATE payment_submissions SET verified_at=now(),verified_by=$2,verified_reference=$3,verified_date=$4,verified_amount_paise=$5,beneficiary_confirmed=true WHERE id=$1`, latest, staff, ref, date, in.VerifiedAmountPaise)
		if e != nil {
			return fmt.Errorf("bank reference is already approved or payment could not be verified: %w", e)
		}
		if _, e = tx.Exec(ctx, "UPDATE registrations SET status='approved',approved_at=now(),approved_by=$2,review_note=$3,updated_at=now() WHERE id=$1", rid, staff, in.Note); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, "SELECT id FROM attendees WHERE registration_id=$1 ORDER BY position", rid)
		if e != nil {
			return e
		}
		var people []string
		for rows.Next() {
			var aid string
			if e = rows.Scan(&aid); e != nil {
				rows.Close()
				return e
			}
			people = append(people, aid)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(people) != c.RosterCount {
			return errors.New("roster is incomplete")
		}
		for _, aid := range people {
			if _, e = a.newPass(ctx, tx, rid, aid); e != nil {
				return e
			}
		}
		pack := randomToken()
		if _, e = tx.Exec(ctx, "UPDATE registrations SET contact_pack_hash=$2,contact_pack_cipher=$3 WHERE id=$1", rid, hash(pack), a.seal(pack)); e != nil {
			return e
		}
		if in.Action == "approve_send" {
			if e = a.queuePasses(ctx, tx, rid, "", "initial"); e != nil {
				return e
			}
		}
	case "correction_requested", "rejected":
		if status != "awaiting_review" {
			return ErrConflict
		}
		if strings.TrimSpace(in.Note) == "" {
			return errors.New("a reason is required")
		}
		if _, e = tx.Exec(ctx, "UPDATE registrations SET status=$2,review_note=$3,updated_at=now() WHERE id=$1", rid, in.Action, in.Note); e != nil {
			return e
		}
	case "cancelled":
		if status == "cancelled" {
			return tx.Commit(ctx)
		}
		if strings.TrimSpace(in.Note) == "" {
			return errors.New("a cancellation reason is required; refunds are handled manually")
		}
		if _, e = tx.Exec(ctx, "UPDATE registrations SET status='cancelled',review_note=$2,updated_at=now() WHERE id=$1", rid, in.Note); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE passes SET revoked_at=now() WHERE registration_id=$1 AND revoked_at IS NULL", rid); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE delivery_jobs SET status='cancelled',updated_at=now() WHERE registration_id=$1 AND purpose IN ('pass','pack') AND status='queued'", rid); e != nil {
			return e
		}
	case "send", "resend":
		if status != "approved" {
			return ErrConflict
		}
		if in.Channel != "" && in.Channel != "email" && in.Channel != "whatsapp" {
			return errors.New("invalid channel")
		}
		key := "initial"
		if in.Action == "resend" {
			if len(in.RequestID) < 32 || len(in.RequestID) > 128 {
				return errors.New("resend requires a unique request_id")
			}
			key = "resend:" + hash(in.RequestID)
		}
		if e = a.queuePasses(ctx, tx, rid, in.Channel, key); e != nil {
			return e
		}
	case "reissue":
		if status != "approved" || strings.TrimSpace(in.Note) == "" {
			return ErrConflict
		}
		var aid string
		if e = tx.QueryRow(ctx, "UPDATE passes SET revoked_at=now() WHERE id=$1 AND registration_id=$2 AND revoked_at IS NULL RETURNING attendee_id", in.PassID, rid).Scan(&aid); e != nil {
			return ErrConflict
		}
		if _, e = tx.Exec(ctx, "UPDATE delivery_jobs SET status='cancelled',updated_at=now() WHERE pass_id=$1 AND status='queued'", in.PassID); e != nil {
			return e
		}
		if _, e = a.newPass(ctx, tx, rid, aid); e != nil {
			return e
		}
		if e = a.queuePasses(ctx, tx, rid, "", "reissue:"+in.PassID); e != nil {
			return e
		}
	default:
		return errors.New("unknown review action")
	}
	if e = audit(ctx, tx, staff, rid, in.Action, in.Note); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// nextReference allocates the next number in a category's series, "BC4-EX-0007".
// The advisory lock is held for the rest of the caller's transaction, so two
// registrations in the same category cannot read the same maximum; it is
// released on commit or rollback, which also means an abandoned registration
// leaves no gap in the series.
//
// A reference and the pass numbers built from it are handles, never
// credentials: they are short and predictable on purpose, while the QR
// identifier and the download token stay full-entropy and unguessable.
func nextReference(ctx context.Context, tx pgx.Tx, catID string) (string, error) {
	prefix := "BC4-" + categoryCode(catID) + "-"
	if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "reference:"+prefix); e != nil {
		return "", e
	}
	var next int
	// The pattern ignores references issued before the series existed.
	e := tx.QueryRow(ctx, `SELECT COALESCE(MAX(substring(reference from '[0-9]+$')::bigint),0)+1 FROM registrations WHERE reference ~ $1`, "^"+prefix+"[0-9]+$").Scan(&next)
	return fmt.Sprintf("%s%04d", prefix, next), e
}

func (a *App) newPass(ctx context.Context, tx pgx.Tx, rid, aid string) (string, error) {
	pid, token := id(), randomToken()
	var version int
	if e := tx.QueryRow(ctx, "SELECT COALESCE(MAX(version),0)+1 FROM passes WHERE attendee_id=$1", aid).Scan(&version); e != nil {
		return "", e
	}
	// A pass number is its registration's reference plus the pass's place in it:
	// BC4-EX-0007-3. Passes are never deleted, so one past the count is always a
	// free place, and a reissue takes the next one rather than reusing a number
	// that is already printed on a revoked pass. The caller holds the
	// registration row lock, which serializes this with any other issue.
	var number string
	if e := tx.QueryRow(ctx, "SELECT r.reference||'-'||((SELECT count(*) FROM passes p WHERE p.registration_id=r.id)+1) FROM registrations r WHERE r.id=$1", rid).Scan(&number); e != nil {
		return "", e
	}
	_, e := tx.Exec(ctx, "INSERT INTO passes(id,registration_id,attendee_id,version,number,qr_id,download_hash,download_cipher) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", pid, rid, aid, version, number, randomToken(), hash(token), a.seal(token))
	return pid, e
}
func (a *App) queue(ctx context.Context, tx pgx.Tx, rid, pid, purpose, channel, to, payload, key string) error {
	_, e := tx.Exec(ctx, `INSERT INTO delivery_jobs(id,registration_id,pass_id,purpose,channel,recipient,payload_cipher,dedupe_key) VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8) ON CONFLICT(dedupe_key) DO NOTHING`, id(), rid, pid, purpose, channel, to, a.seal(payload), key)
	return e
}
func (a *App) queuePasses(ctx context.Context, tx pgx.Tx, rid, channel, key string) error {
	rows, e := tx.Query(ctx, `SELECT p.id,a.email,a.phone,a.whatsapp_consent FROM passes p JOIN attendees a ON a.id=p.attendee_id WHERE p.registration_id=$1 AND p.revoked_at IS NULL`, rid)
	if e != nil {
		return e
	}
	type person struct {
		pid, email, phone string
		consent           bool
	}
	var all []person
	for rows.Next() {
		var p person
		if e = rows.Scan(&p.pid, &p.email, &p.phone, &p.consent); e != nil {
			rows.Close()
			return e
		}
		all = append(all, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, p := range all {
		if channel == "" || channel == "email" {
			if e = a.queue(ctx, tx, rid, p.pid, "pass", "email", p.email, "", key+":"+p.pid+":email"); e != nil {
				return e
			}
		}
		if p.consent && (channel == "" || channel == "whatsapp") {
			if e = a.queue(ctx, tx, rid, p.pid, "pass", "whatsapp", p.phone, "", key+":"+p.pid+":whatsapp"); e != nil {
				return e
			}
		}
	}
	var kind, email string
	if e = tx.QueryRow(ctx, "SELECT c.kind,r.email FROM registrations r JOIN categories c ON c.id=r.category_id WHERE r.id=$1", rid).Scan(&kind, &email); e != nil {
		return e
	}
	if kind == "exhibitor" && (channel == "" || channel == "email") {
		return a.queue(ctx, tx, rid, "", "pack", "email", email, "", key+":"+rid+":pack")
	}
	return nil
}
