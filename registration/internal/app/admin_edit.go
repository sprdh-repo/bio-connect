package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Staff corrections to a registration after it was submitted: editing or
// removing an attendee and moving an exhibitor to another stall type. None of
// them delivers anything. When a correction leaves a pass to send, staff send
// it from the console (see the per-pass "send" and "resend" review actions),
// so nobody receives a pass the team has not decided to send.

// UpdateAttendee corrects one attendee's details. Their active pass, if any,
// stays valid and keeps its number and QR; a changed name or designation is
// rendered onto it the next time it is downloaded, and queued deliveries to a
// changed email or phone are cancelled so they cannot reach the old address.
func (a *App) UpdateAttendee(ctx context.Context, rid, aid, staff string, p Attendee) error {
	p.Designation = strings.TrimSpace(p.Designation)
	// Only staff correct attendees, so the phone is optional here.
	if !attendeeOK(&p, true) {
		return attendeeError(true)
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status, kind string
	if e = tx.QueryRow(ctx, `SELECT r.status,c.kind FROM registrations r JOIN categories c ON c.id=r.category_id WHERE r.id=$1 FOR UPDATE OF r`, rid).Scan(&status, &kind); e != nil {
		return e
	}
	if status == "cancelled" {
		return ErrConflict
	}
	var old Attendee
	if e = tx.QueryRow(ctx, "SELECT name,email,phone,designation,whatsapp_consent FROM attendees WHERE id=$1 AND registration_id=$2 AND removed_at IS NULL", aid, rid).Scan(&old.Name, &old.Email, &old.Phone, &old.Designation, &old.WhatsAppConsent); e != nil {
		return errors.New("attendee not found on this registration")
	}
	var dup bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM attendees WHERE registration_id=$1 AND id<>$2 AND removed_at IS NULL AND lower(email)=$3)", rid, aid, p.Email).Scan(&dup); e != nil {
		return e
	}
	if dup {
		return errors.New("this person is already on the registration")
	}
	var changes []string
	for _, f := range []struct{ label, from, to string }{
		{"name", old.Name, p.Name}, {"designation", old.Designation, p.Designation},
		{"email", old.Email, p.Email}, {"phone", old.Phone, p.Phone},
	} {
		if f.from != f.to {
			changes = append(changes, fmt.Sprintf("%s: %s -> %s", f.label, f.from, f.to))
		}
	}
	if old.WhatsAppConsent != p.WhatsAppConsent {
		changes = append(changes, fmt.Sprintf("whatsapp consent: %t -> %t", old.WhatsAppConsent, p.WhatsAppConsent))
	}
	if len(changes) == 0 {
		return tx.Commit(ctx)
	}
	// Consent is recorded when it is given, so an unchanged consent keeps its
	// original time and wording.
	if old.WhatsAppConsent != p.WhatsAppConsent {
		var at *time.Time
		ct := ""
		if p.WhatsAppConsent {
			now := a.Now()
			at = &now
			ct = consentText
		}
		if _, e = tx.Exec(ctx, "UPDATE attendees SET whatsapp_consent=$2,consent_at=$3,consent_text=$4 WHERE id=$1", aid, p.WhatsAppConsent, at, ct); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, "UPDATE attendees SET name=$2,email=$3,phone=$4,designation=$5 WHERE id=$1", aid, p.Name, p.Email, p.Phone, p.Designation); e != nil {
		return e
	}
	// A delegate registration is its one attendee: the contact is copied from
	// them at registration (validateInput), so it follows their corrections.
	if kind == "delegate" {
		if _, e = tx.Exec(ctx, "UPDATE registrations SET contact_name=$2,email=$3,phone=$4,updated_at=now() WHERE id=$1", rid, p.Name, p.Email, p.Phone); e != nil {
			return e
		}
	} else if _, e = tx.Exec(ctx, "UPDATE registrations SET updated_at=now() WHERE id=$1", rid); e != nil {
		return e
	}
	if old.Name != p.Name || old.Designation != p.Designation {
		// Drop the cached PDF so the pass is rendered again with the new details.
		if _, e = tx.Exec(ctx, "UPDATE passes SET file_id=NULL WHERE attendee_id=$1 AND revoked_at IS NULL", aid); e != nil {
			return e
		}
	}
	if old.Email != p.Email {
		if e = cancelQueuedPassJobs(ctx, tx, aid, "email"); e != nil {
			return e
		}
	}
	if old.Phone != p.Phone || !p.WhatsAppConsent {
		if e = cancelQueuedPassJobs(ctx, tx, aid, "whatsapp"); e != nil {
			return e
		}
	}
	if e = audit(ctx, tx, staff, rid, "attendee_updated", strings.Join(changes, "; ")); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// cancelQueuedPassJobs stops pass deliveries to one attendee on one channel
// that have not been picked up yet.
func cancelQueuedPassJobs(ctx context.Context, tx pgx.Tx, aid, channel string) error {
	_, e := tx.Exec(ctx, "UPDATE delivery_jobs SET status='cancelled',updated_at=now() WHERE status='queued' AND channel=$2 AND pass_id IN (SELECT id FROM passes WHERE attendee_id=$1)", aid, channel)
	return e
}

// RemoveAttendee takes a person off a registration and revokes their pass.
// The place they held becomes unassigned again, so an exhibitor's place can be
// given to someone else. A registration always keeps at least one attendee;
// to drop the last one, cancel the registration.
func (a *App) RemoveAttendee(ctx context.Context, rid, aid, staff, note string) error {
	if len(note) > 2000 {
		return errors.New("note is too long")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status string
	var have int
	if e = tx.QueryRow(ctx, "SELECT status,(SELECT count(*) FROM attendees a WHERE a.registration_id=r.id AND a.removed_at IS NULL) FROM registrations r WHERE id=$1 FOR UPDATE", rid).Scan(&status, &have); e != nil {
		return e
	}
	if status == "cancelled" {
		return ErrConflict
	}
	var name, email string
	if e = tx.QueryRow(ctx, "SELECT name,email FROM attendees WHERE id=$1 AND registration_id=$2 AND removed_at IS NULL", aid, rid).Scan(&name, &email); e != nil {
		return errors.New("attendee not found on this registration")
	}
	if have <= 1 {
		return errors.New("a registration needs at least one attendee; edit this attendee or cancel the registration instead")
	}
	if _, e = tx.Exec(ctx, "UPDATE attendees SET removed_at=now() WHERE id=$1", aid); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE passes SET revoked_at=now() WHERE attendee_id=$1 AND revoked_at IS NULL", aid); e != nil {
		return e
	}
	for _, ch := range []string{"email", "whatsapp"} {
		if e = cancelQueuedPassJobs(ctx, tx, aid, ch); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, "UPDATE registrations SET updated_at=now() WHERE id=$1", rid); e != nil {
		return e
	}
	detail := name + " <" + email + ">"
	if n := strings.TrimSpace(note); n != "" {
		detail += ": " + n
	}
	if e = audit(ctx, tx, staff, rid, "attendee_removed", detail); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// ChangeCategory moves an exhibitor registration to another stall type. Its
// pass allowance becomes the new stall's plus any extra passes staff granted
// earlier, and never drops below the attendees it already holds: moving to a
// smaller stall keeps everyone, with the places beyond the new stall's
// allowance kept as extra passes. Nobody is removed and no pass is revoked.
// A larger stall leaves places to fill. The reference and
// every issued pass are unchanged: all stall types share the EX series and
// print the same EXHIBITOR pass. The fee due before approval follows the new
// stall; any difference on an approved registration is settled outside the
// system.
func (a *App) ChangeCategory(ctx context.Context, rid, staff, catID, note string) error {
	if len(note) > 2000 {
		return errors.New("note is too long")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status, current string
	var roster, have int
	if e = tx.QueryRow(ctx, "SELECT status,category_id,roster_count,(SELECT count(*) FROM attendees a WHERE a.registration_id=r.id AND a.removed_at IS NULL) FROM registrations r WHERE id=$1 FOR UPDATE", rid).Scan(&status, &current, &roster, &have); e != nil {
		return e
	}
	if status == "cancelled" {
		return ErrConflict
	}
	from, e := category(ctx, tx, current)
	if e != nil {
		return e
	}
	to, e := category(ctx, tx, catID)
	if e != nil {
		return errors.New("unknown category")
	}
	if from.Kind != "exhibitor" || to.Kind != "exhibitor" {
		return errors.New("only exhibitor registrations can change stall type")
	}
	if to.ID == from.ID {
		return errors.New("the registration is already on this stall type")
	}
	allowance := to.RosterCount + max(0, roster-from.RosterCount)
	allowance = min(max(allowance, have), maxAllowance)
	if _, e = tx.Exec(ctx, "UPDATE registrations SET category_id=$2,roster_count=$3,updated_at=now() WHERE id=$1", rid, to.ID, allowance); e != nil {
		return e
	}
	// Re-render cached passes so nothing printed can lag the registration.
	if _, e = tx.Exec(ctx, "UPDATE passes SET file_id=NULL WHERE registration_id=$1 AND revoked_at IS NULL", rid); e != nil {
		return e
	}
	detail := fmt.Sprintf("%s -> %s; %d -> %d passes (stall includes %d)", from.Label, to.Label, roster, allowance, to.RosterCount)
	if n := strings.TrimSpace(note); n != "" {
		detail += ": " + n
	}
	if e = audit(ctx, tx, staff, rid, "category_changed", detail); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// maxAllowance bounds a registration's pass allowance (migration 019).
const maxAllowance = 50

// SetAllowance sets how many passes an exhibitor registration may hold,
// including more than its stall type includes. It never goes below the
// attendees already on the registration. Only staff can do this; the new
// places are filled like any other open place (AddAttendee), and nothing is
// sent. A later stall change carries the extra passes over (ChangeCategory).
func (a *App) SetAllowance(ctx context.Context, rid, staff string, n int, note string) error {
	if len(note) > 2000 {
		return errors.New("note is too long")
	}
	if n < 1 || n > maxAllowance {
		return fmt.Errorf("the pass allowance must be between 1 and %d", maxAllowance)
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status, kind string
	var current, included, have int
	if e = tx.QueryRow(ctx, `SELECT r.status,c.kind,r.roster_count,c.roster_count,(SELECT count(*) FROM attendees a WHERE a.registration_id=r.id AND a.removed_at IS NULL) FROM registrations r JOIN categories c ON c.id=r.category_id WHERE r.id=$1 FOR UPDATE OF r`, rid).Scan(&status, &kind, &current, &included, &have); e != nil {
		return e
	}
	if status == "cancelled" {
		return ErrConflict
	}
	if kind != "exhibitor" {
		return errors.New("only exhibitor registrations have additional passes")
	}
	if n == current {
		return fmt.Errorf("the registration already has %d passes", n)
	}
	if n < have {
		return fmt.Errorf("%d attendees hold passes; remove %d first", have, have-n)
	}
	if _, e = tx.Exec(ctx, "UPDATE registrations SET roster_count=$2,updated_at=now() WHERE id=$1", rid, n); e != nil {
		return e
	}
	detail := fmt.Sprintf("%d -> %d passes (stall includes %d)", current, n, included)
	if t := strings.TrimSpace(note); t != "" {
		detail += ": " + t
	}
	if e = audit(ctx, tx, staff, rid, "allowance_changed", detail); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

var stallNumberRE = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 /-]{0,15}$`)

// SetStallNumber assigns, changes or clears (empty) an exhibitor's stall on
// the expo floor. Numbers are trimmed and upper-cased so "a-12" and "A-12" are
// the same stall. Only approved exhibitors appear in the public directory, but
// staff may allocate earlier. Nothing is sent and passes are unchanged.
func (a *App) SetStallNumber(ctx context.Context, rid, staff, number, note string) error {
	if len(note) > 2000 {
		return errors.New("note is too long")
	}
	number = strings.ToUpper(strings.Join(strings.Fields(number), " "))
	if number != "" && !stallNumberRE.MatchString(number) {
		return errors.New("use up to 16 letters, numbers, spaces, hyphens or slashes for the stall number")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status, kind, current string
	if e = tx.QueryRow(ctx, `SELECT r.status,c.kind,r.stall_number FROM registrations r JOIN categories c ON c.id=r.category_id WHERE r.id=$1 FOR UPDATE OF r`, rid).Scan(&status, &kind, &current); e != nil {
		return e
	}
	if status == "cancelled" {
		return ErrConflict
	}
	if kind != "exhibitor" {
		return errors.New("only exhibitor registrations have a stall")
	}
	if number == current {
		return errors.New("the stall number is unchanged")
	}
	if _, e = tx.Exec(ctx, "UPDATE registrations SET stall_number=$2,updated_at=now() WHERE id=$1", rid, number); e != nil {
		return e
	}
	label := func(s string) string {
		if s == "" {
			return "unassigned"
		}
		return s
	}
	detail := label(current) + " -> " + label(number)
	if t := strings.TrimSpace(note); t != "" {
		detail += ": " + t
	}
	if e = audit(ctx, tx, staff, rid, "stall_number_changed", detail); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
