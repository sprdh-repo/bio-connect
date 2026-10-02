package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// A registration's roster_count is its pass allowance. It normally equals the
// attendees submitted with the form, but an allowance rise (migration 009:
// exhibitors went from 3 / 2 / 2 to 5 / 3 / 2) leaves open places, which the
// exhibitor fills from the management page or staff fill from the console.

var errRosterFull = errors.New("every pass on this registration is already assigned")

// AddAttendee fills one open place. Before approval the person simply joins
// the roster and approval issues their pass with the rest. After approval
// their pass is issued at once, at the next free pass number; the existing
// passes are untouched. When the exhibitor adds the person themselves (staff
// empty) and the team's passes have gone out, the new pass is delivered to
// them alone plus a refreshed pack for the contact. A pass staff add from the
// console is not sent: they send it when ready.
func (a *App) AddAttendee(ctx context.Context, rid, staff string, p Attendee) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	if !validText(p.Name, 120) || !validText(p.Designation, 180) || !validEmail(p.Email) || !phoneRE.MatchString(p.Phone) {
		return errors.New("the attendee needs name, designation, valid email and international phone")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	// The registration row lock serializes this with approval, reissue and
	// another add, so two requests can never take the same place.
	var status, kind string
	var roster, have int
	if e = tx.QueryRow(ctx, `SELECT r.status,c.kind,r.roster_count,(SELECT count(*) FROM attendees a WHERE a.registration_id=r.id AND a.removed_at IS NULL) FROM registrations r JOIN categories c ON c.id=r.category_id WHERE r.id=$1 FOR UPDATE OF r`, rid).Scan(&status, &kind, &roster, &have); e != nil {
		return e
	}
	if kind != "exhibitor" {
		return errors.New("only exhibitor registrations have additional passes")
	}
	if status == "rejected" || status == "cancelled" {
		return ErrConflict
	}
	if have >= roster {
		return errRosterFull
	}
	var dup bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM attendees WHERE registration_id=$1 AND removed_at IS NULL AND lower(email)=$2)", rid, p.Email).Scan(&dup); e != nil {
		return e
	}
	if dup {
		return errors.New("this person is already on the registration")
	}
	var at *time.Time
	ct := ""
	if p.WhatsAppConsent {
		now := a.Now()
		at = &now
		ct = consentText
	}
	aid := id()
	if _, e = tx.Exec(ctx, `INSERT INTO attendees(id,registration_id,name,email,phone,designation,whatsapp_consent,consent_at,consent_text,position) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,(SELECT COALESCE(MAX(position),-1)+1 FROM attendees WHERE registration_id=$2))`, aid, rid, p.Name, p.Email, p.Phone, p.Designation, p.WhatsAppConsent, at, ct); e != nil {
		return e
	}
	if status == "approved" {
		pid, e := a.newPass(ctx, tx, rid, aid)
		if e != nil {
			return e
		}
		// After approve_only nothing has gone out yet; the team's first "send"
		// then delivers this pass with the others.
		sent := false
		if staff == "" {
			if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass')", rid).Scan(&sent); e != nil {
				return e
			}
		}
		if sent {
			if e = a.queuePass(ctx, tx, rid, pid, p.Email, p.Phone, p.WhatsAppConsent, "", "initial"); e != nil {
				return e
			}
			var contact string
			if e = tx.QueryRow(ctx, "SELECT email FROM registrations WHERE id=$1", rid).Scan(&contact); e != nil {
				return e
			}
			if e = a.queue(ctx, tx, rid, "", "pack", "email", contact, "", "added:"+pid+":"+rid+":pack"); e != nil {
				return e
			}
		}
	}
	if e = audit(ctx, tx, staff, rid, "attendee_added", p.Name+" <"+p.Email+">"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// rosterNotice emails the contact a single-use link back to the registration
// so they can fill its open places. Like a payment reminder it carries a
// recovery link, because the management token is stored only as a hash.
func (a *App) rosterNotice(ctx context.Context, tx pgx.Tx, rid, status, cat, email string) error {
	if status == "rejected" || status == "cancelled" {
		return ErrConflict
	}
	c, e := category(ctx, tx, cat)
	if e != nil {
		return e
	}
	if c.Kind != "exhibitor" {
		return errors.New("only exhibitor registrations have additional passes")
	}
	var open bool
	if e = tx.QueryRow(ctx, "SELECT roster_count>(SELECT count(*) FROM attendees WHERE registration_id=$1 AND removed_at IS NULL) FROM registrations WHERE id=$1", rid).Scan(&open); e != nil {
		return e
	}
	if !open {
		return errRosterFull
	}
	var recent bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM delivery_jobs WHERE registration_id=$1 AND purpose='roster_notice' AND created_at>now()-interval '24 hours')", rid).Scan(&recent); e != nil {
		return e
	}
	if recent {
		return errors.New("an additional-pass notice was already sent in the last 24 hours")
	}
	token := randomToken()
	if _, e = tx.Exec(ctx, "INSERT INTO recovery_tokens(token_hash,registration_id,expires_at) VALUES($1,$2,$3)", hash(token), rid, a.Now().Add(reminderLinkLifetime)); e != nil {
		return e
	}
	return a.queue(ctx, tx, rid, "", "roster_notice", "email", email, a.Config.BaseURL+"/recover#"+token, "roster_notice:"+hash(token))
}

// NotifyResult is one registration's outcome from NotifyOpenPlaces.
type NotifyResult struct {
	Reference, Institution, Outcome string
}

// NotifyOpenPlaces sends the additional-pass notice to every live exhibitor
// registration with open places that has never had one. Re-running it only
// reaches registrations that gained open places since.
func (a *App) NotifyOpenPlaces(ctx context.Context) ([]NotifyResult, error) {
	rows, e := a.DB.Query(ctx, `SELECT r.id,r.reference,r.institution FROM registrations r JOIN categories c ON c.id=r.category_id
	 WHERE c.kind='exhibitor' AND r.status NOT IN ('rejected','cancelled')
	  AND r.roster_count>(SELECT count(*) FROM attendees a WHERE a.registration_id=r.id AND a.removed_at IS NULL)
	  AND NOT EXISTS(SELECT 1 FROM delivery_jobs j WHERE j.registration_id=r.id AND j.purpose='roster_notice')
	 ORDER BY r.reference`)
	if e != nil {
		return nil, e
	}
	type reg struct{ id, reference, institution string }
	var all []reg
	for rows.Next() {
		var r reg
		if e = rows.Scan(&r.id, &r.reference, &r.institution); e != nil {
			rows.Close()
			return nil, e
		}
		all = append(all, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := make([]NotifyResult, 0, len(all))
	for _, r := range all {
		outcome := "queued"
		if e := a.Review(ctx, r.id, "", ReviewInput{Action: "roster_notice"}); e != nil {
			outcome = publicError(e)
		}
		out = append(out, NotifyResult{r.reference, r.institution, outcome})
	}
	return out, nil
}
