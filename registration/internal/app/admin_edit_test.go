package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// dueNow is the fee a registration owes for a payment made today, so these
// tests hold on either side of the early-bird cutoff.
func dueNow(t *testing.T, a *App, rid string) int64 {
	t.Helper()
	ctx := context.Background()
	var catID string
	var discount int
	if err := a.DB.QueryRow(ctx, "SELECT category_id,discount_percent FROM registrations WHERE id=$1", rid).Scan(&catID, &discount); err != nil {
		t.Fatal(err)
	}
	cats, err := a.categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.ID == catID {
			return payable(c, discount, a.Now())
		}
	}
	t.Fatalf("category %s not found", catID)
	return 0
}

func rosterOf(t *testing.T, a *App, cat string) int {
	return count(t, a, "SELECT roster_count FROM categories WHERE id=$1", cat)
}

// approvedExhibitor is an exhibitor approved with approve_send, so its initial
// deliveries are queued.
func approvedExhibitor(t *testing.T, a *App, k int, cat, sid string) string {
	t.Helper()
	rid, _, err := a.Create(context.Background(), exhibitorInput(cat, rosterOf(t, a, cat)), key(k), tinyPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	payExhibitor(t, a, rid, dueNow(t, a, rid))
	if err := tryApprove(a, rid, sid, "approve_send", "SBIEDIT"+rid[:6], dueNow(t, a, rid)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return rid
}

func attendeeIDs(t *testing.T, a *App, rid string) []string {
	t.Helper()
	r, err := a.registration(context.Background(), rid)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range r.Attendees {
		out = append(out, p.ID)
	}
	return out
}

func activePassOf(t *testing.T, a *App, aid string) string {
	t.Helper()
	var pid string
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM passes WHERE attendee_id=$1 AND revoked_at IS NULL", aid).Scan(&pid); err != nil {
		t.Fatalf("active pass: %v", err)
	}
	return pid
}

const allJobs = "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose IN ('pass','pack')"

func TestReinstateApprovesRejectedWithoutSending(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, exhibitorInput("standard", rosterOf(t, a, "standard")), key(1), tinyPNG(t))
	payExhibitor(t, a, rid, dueNow(t, a, rid))

	if err := tryApprove(a, rid, sid, "reinstate", "SBIRE1", dueNow(t, a, rid)); err != ErrConflict {
		t.Fatalf("reinstate before rejection: %v, want ErrConflict", err)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "rejected", Note: "reference not found"}); err != nil {
		t.Fatal(err)
	}
	// The same payment checks as approval apply.
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "reinstate", PaymentID: latestPayment(a, rid), VerifiedReference: "SBIRE1", VerifiedDate: today(a), VerifiedAmountPaise: dueNow(t, a, rid)}); err == nil {
		t.Fatal("reinstate without bank confirmation succeeded")
	}
	if err := tryApprove(a, rid, sid, "reinstate", "SBIRE1", dueNow(t, a, rid)+100); err == nil {
		t.Fatal("reinstate with a mismatched amount succeeded")
	}
	for i := 0; i < 2; i++ {
		if err := tryApprove(a, rid, sid, "reinstate", "SBIRE1", dueNow(t, a, rid)); err != nil {
			t.Fatalf("reinstate %d: %v", i, err)
		}
	}
	if s := status(t, a, rid); s != "approved" {
		t.Fatalf("status %q, want approved", s)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != rosterOf(t, a, "standard") {
		t.Fatalf("%d passes, want one per attendee", n)
	}
	if n := count(t, a, allJobs, rid); n != 0 {
		t.Fatalf("reinstate queued %d deliveries, want none", n)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'", rid); n != rosterOf(t, a, "standard") {
		t.Fatalf("send queued %d pass deliveries", n)
	}
}

func TestEditAttendeeSendsNothingAndRetargetsLaterSends(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid := approvedExhibitor(t, a, 1, "standard", sid)
	ids := attendeeIDs(t, a, rid)
	pid := activePassOf(t, a, ids[0])
	if _, err := a.passPDF(ctx, pid); err != nil {
		t.Fatal(err)
	}
	var number string
	a.DB.QueryRow(ctx, "SELECT number FROM passes WHERE id=$1", pid).Scan(&number)
	before := count(t, a, allJobs, rid)

	in := Attendee{Name: "Renamed Rep", Email: "Renamed@Example.com", Phone: "+919811111111", Designation: "Director", WhatsAppConsent: true}
	if err := a.UpdateAttendee(ctx, rid, ids[0], sid, in); err != nil {
		t.Fatalf("update: %v", err)
	}
	r, _ := a.registration(ctx, rid)
	got := r.Attendees[0]
	if got.Name != "Renamed Rep" || got.Email != "renamed@example.com" || got.Phone != "+919811111111" || got.Designation != "Director" || !got.WhatsAppConsent {
		t.Fatalf("attendee not updated: %+v", got)
	}
	if n := count(t, a, "SELECT count(*) FROM attendees WHERE id=$1 AND consent_at IS NOT NULL AND consent_text<>''", ids[0]); n != 1 {
		t.Fatal("new WhatsApp consent was not recorded")
	}
	// Same pass and number, cached PDF dropped so it renders the new name.
	if p := activePassOf(t, a, ids[0]); p != pid {
		t.Fatal("editing replaced the pass")
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE id=$1 AND file_id IS NULL AND number=$2", pid, number); n != 1 {
		t.Fatal("cached pass PDF kept or number changed")
	}
	// The queued email to the old address is cancelled; nothing new is queued.
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1 AND status='queued'", pid); n != 0 {
		t.Fatalf("%d deliveries still queued to the old address", n)
	}
	if n := count(t, a, allJobs, rid); n != before {
		t.Fatalf("editing queued deliveries: %d -> %d", before, n)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='attendee_updated' AND detail LIKE '%name: Rep 0 -> Renamed Rep%'", rid); n != 1 {
		t.Fatal("edit not audited")
	}
	// Other attendees' emails stay unique.
	dup := in
	dup.Email = "rep1@example.com"
	if err := a.UpdateAttendee(ctx, rid, ids[0], sid, dup); err == nil {
		t.Fatal("duplicate email accepted")
	}

	// "send" for the one pass skips email, which had its initial send, and
	// delivers only on WhatsApp, newly permitted; repeating it changes nothing.
	for i := 0; i < 2; i++ {
		if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send", PassID: pid}); err != nil {
			t.Fatal(err)
		}
		if n := count(t, a, allJobs, rid) - before; n != 1 {
			t.Fatalf("per-pass send %d: %d new deliveries, want 1", i, n)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1 AND status='queued' AND channel='whatsapp' AND recipient='+919811111111'", pid); n != 1 {
		t.Fatal("per-pass send did not use the newly permitted WhatsApp")
	}
	// "resend" delivers again on both channels to the corrected details, with no pack.
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "resend", PassID: pid, RequestID: key(50)}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, allJobs, rid) - before; n != 3 {
		t.Fatalf("per-pass resend queued %d deliveries, want email and WhatsApp", n-1)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1 AND status='queued' AND dedupe_key LIKE 'resend:%' AND recipient IN ('renamed@example.com','+919811111111')", pid); n != 2 {
		t.Fatal("resend did not go to the corrected contact details")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack' AND dedupe_key LIKE 'resend:%'", rid); n != 0 {
		t.Fatal("per-pass resend sent the contact pack")
	}
	other := activePassOf(t, a, ids[1])
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1 AND dedupe_key LIKE 'resend:%'", other); n != 0 {
		t.Fatal("per-pass resend reached another attendee")
	}
}

func TestEditDelegateFollowsContact(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid, _, _ := a.Create(ctx, delegateInput("industry"), key(1), nil)
	ids := attendeeIDs(t, a, rid)
	if err := a.UpdateAttendee(ctx, rid, ids[0], sid, Attendee{Name: "Asha N. Pillai", Email: "asha.p@example.com", Phone: "+919800000001", Designation: "CTO"}); err != nil {
		t.Fatal(err)
	}
	r, _ := a.registration(ctx, rid)
	if r.ContactName != "Asha N. Pillai" || r.Email != "asha.p@example.com" || r.Phone != "+919800000001" {
		t.Fatalf("delegate contact not updated: %+v", r)
	}
	if err := a.RemoveAttendee(ctx, rid, ids[0], sid, ""); err == nil {
		t.Fatal("removed a delegate's only attendee")
	}
	if err := a.ChangeCategory(ctx, rid, sid, "faculty", ""); err == nil {
		t.Fatal("changed a delegate's category")
	}
}

func TestRemoveAttendeeRevokesPassAndFreesThePlace(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid := approvedExhibitor(t, a, 1, "standard", sid)
	ids := attendeeIDs(t, a, rid)
	gone := activePassOf(t, a, ids[1])
	before := count(t, a, allJobs, rid)

	if err := a.RemoveAttendee(ctx, rid, ids[1], sid, "left the company"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE id=$1 AND revoked_at IS NOT NULL", gone); n != 1 {
		t.Fatal("removed attendee's pass is still valid")
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1 AND status='queued'", gone); n != 0 {
		t.Fatal("removed attendee still has queued deliveries")
	}
	if got := len(attendeeIDs(t, a, rid)); got != len(ids)-1 {
		t.Fatalf("%d attendees listed after removal, want %d", got, len(ids)-1)
	}
	if err := a.RemoveAttendee(ctx, rid, ids[1], sid, ""); err == nil {
		t.Fatal("removed the same attendee twice")
	}
	// Their email is free again, and the place they held is open; the staff-added
	// replacement gets a pass but no delivery until staff send it.
	repl := Attendee{Name: "Replacement", Email: "rep1@example.com", Phone: "+919822222222", Designation: "Scientist"}
	if err := a.AddAttendee(ctx, rid, sid, repl); err != nil {
		t.Fatalf("fill freed place: %v", err)
	}
	ids = attendeeIDs(t, a, rid)
	newPass := activePassOf(t, a, ids[len(ids)-1])
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1", newPass); n != 0 {
		t.Fatalf("staff-added pass was sent automatically (%d jobs)", n)
	}
	if n := count(t, a, allJobs, rid); n != before {
		t.Fatalf("remove and refill queued deliveries: %d -> %d", before, n)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send", PassID: newPass}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1", newPass); n != 1 {
		t.Fatalf("per-pass send queued %d deliveries, want 1 email", n)
	}
	// Down to one attendee, and no further.
	for _, aid := range ids[1:] {
		if err := a.RemoveAttendee(ctx, rid, aid, sid, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.RemoveAttendee(ctx, rid, ids[0], sid, ""); err == nil || !strings.Contains(err.Error(), "at least one attendee") {
		t.Fatalf("removing the last attendee: %v", err)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "resend", PassID: gone, RequestID: key(51)}); err == nil {
		t.Fatal("sent a revoked pass")
	}
}

func TestChangeStallType(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	premium, standard, table := rosterOf(t, a, "premium"), rosterOf(t, a, "standard"), rosterOf(t, a, "table")

	// Before approval the fee due follows the new stall.
	rid, _, _ := a.Create(ctx, exhibitorInput("table", table), key(1), tinyPNG(t))
	var reference string
	a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&reference)
	if err := a.ChangeCategory(ctx, rid, sid, "table", ""); err == nil {
		t.Fatal("changed to the current stall")
	}
	if err := a.ChangeCategory(ctx, rid, sid, "industry", ""); err == nil {
		t.Fatal("moved an exhibitor to a delegate category")
	}
	if err := a.ChangeCategory(ctx, rid, sid, "premium", "upgraded at the desk"); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got := rosterCount(t, a, rid); got != premium {
		t.Fatalf("roster %d after upgrade, want %d", got, premium)
	}
	var ref2 string
	a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&ref2)
	if ref2 != reference {
		t.Fatalf("reference changed %s -> %s", reference, ref2)
	}
	payExhibitor(t, a, rid, dueNow(t, a, rid))
	if err := tryApprove(a, rid, sid, "approve_only", "SBISTALL1", dueNow(t, a, rid)); err != nil {
		t.Fatalf("approve at the premium fee: %v", err)
	}
	// Approval issues passes for the attendees there are; the rest stay open.
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != table {
		t.Fatalf("%d passes, want %d", n, table)
	}

	// After approval: a smaller stall keeps everyone, the places beyond its
	// allowance kept as extra passes. Nobody is removed, no pass revoked.
	big := approvedExhibitor(t, a, 2, "premium", sid)
	passesBefore := passNumbers(t, a, big)
	jobsBefore := count(t, a, allJobs, big)
	if err := a.ChangeCategory(ctx, big, sid, "standard", ""); err != nil {
		t.Fatalf("downgrade: %v", err)
	}
	if got := rosterCount(t, a, big); got != premium {
		t.Fatalf("roster %d after downgrade, want %d (everyone kept)", got, premium)
	}
	ids := attendeeIDs(t, a, big)
	if len(ids) != premium {
		t.Fatalf("%d attendees after downgrade, want %d", len(ids), premium)
	}
	after := passNumbers(t, a, big)
	if fmt.Sprint(after) != fmt.Sprint(passesBefore) {
		t.Fatalf("downgrade changed passes: %v -> %v", passesBefore, after)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='category_changed' AND detail LIKE '%5 -> 5 passes (stall includes 3)%'", big); n != 1 {
		t.Fatal("downgrade audit does not record the kept passes")
	}
	// Removing one now frees a place only down to the kept attendees; the
	// allowance stays until staff lower it.
	if err := a.RemoveAttendee(ctx, big, ids[4], sid, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SetAllowance(ctx, big, sid, standard, "back to the stall's passes"); err == nil {
		t.Fatal("allowance lowered below the attendees holding passes")
	}
	if n := count(t, a, allJobs, big); n != jobsBefore {
		t.Fatalf("stall change queued deliveries: %d -> %d", jobsBefore, n)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='category_changed'", big); n != 1 {
		t.Fatal("stall change not audited")
	}
	// Extra passes granted on top of a stall travel with the registration.
	small := approvedExhibitor(t, a, 3, "table", sid)
	if err := a.SetAllowance(ctx, small, sid, table+2, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.ChangeCategory(ctx, small, sid, "standard", ""); err != nil {
		t.Fatal(err)
	}
	if got := rosterCount(t, a, small); got != standard+2 {
		t.Fatalf("roster %d after upgrade, want %d (stall plus 2 extra)", got, standard+2)
	}
	if err := a.Review(ctx, big, sid, ReviewInput{Action: "cancelled", Note: "withdrew"}); err != nil {
		t.Fatal(err)
	}
	if err := a.ChangeCategory(ctx, big, sid, "premium", ""); err != ErrConflict {
		t.Fatalf("change on a cancelled registration: %v, want ErrConflict", err)
	}
	if err := a.UpdateAttendee(ctx, big, ids[0], sid, extra(1)); err != ErrConflict {
		t.Fatalf("edit on a cancelled registration: %v, want ErrConflict", err)
	}
}

// Reissue now leaves sending to staff, like every other console change.
func TestReissueDoesNotSend(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	rid := approvedExhibitor(t, a, 1, "table", sid)
	pid := activePassOf(t, a, attendeeIDs(t, a, rid)[0])
	before := count(t, a, allJobs, rid)
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "reissue", PassID: pid, Note: "misprint"}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, allJobs, rid); n != before {
		t.Fatalf("reissue queued deliveries: %d -> %d", before, n)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
		t.Fatal(err)
	}
	// The replacement goes out with the next send; nothing else is repeated.
	newPass := activePassOf(t, a, attendeeIDs(t, a, rid)[0])
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1", newPass); n != 1 {
		t.Fatalf("replacement pass has %d deliveries after send, want 1", n)
	}
	if n := count(t, a, allJobs, rid); n != before+1 {
		t.Fatalf("send after reissue: %d deliveries, want %d", n, before+1)
	}
}

func TestStaffCanGrantExtraPasses(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	table := rosterOf(t, a, "table")
	rid := approvedExhibitor(t, a, 1, "table", sid)
	before := count(t, a, allJobs, rid)
	if err := a.AddAttendee(ctx, rid, sid, extra(1)); err != errRosterFull {
		t.Fatalf("add on a full stall: %v, want errRosterFull", err)
	}
	if err := a.SetAllowance(ctx, rid, sid, table-1, ""); err == nil {
		t.Fatal("allowance lowered below the attendees holding passes")
	}
	if err := a.SetAllowance(ctx, rid, sid, maxAllowance+1, ""); err == nil {
		t.Fatal("allowance above the limit accepted")
	}
	if err := a.SetAllowance(ctx, rid, sid, table+3, "sponsor guests"); err != nil {
		t.Fatalf("grant extra passes: %v", err)
	}
	if got := rosterCount(t, a, rid); got != table+3 {
		t.Fatalf("roster %d, want %d", got, table+3)
	}
	for i := 1; i <= 3; i++ {
		if err := a.AddAttendee(ctx, rid, sid, extra(i)); err != nil {
			t.Fatalf("fill extra place %d: %v", i, err)
		}
	}
	if err := a.AddAttendee(ctx, rid, sid, extra(4)); err != errRosterFull {
		t.Fatalf("add beyond the extra passes: %v, want errRosterFull", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid); n != table+3 {
		t.Fatalf("%d active passes, want %d", n, table+3)
	}
	if n := count(t, a, allJobs, rid); n != before {
		t.Fatalf("extra passes were sent automatically: %d -> %d", before, n)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='allowance_changed' AND detail LIKE '%stall includes%sponsor guests'", rid); n != 1 {
		t.Fatal("allowance change not audited")
	}
	// Delegates have no extra passes.
	del, _, _ := a.Create(ctx, delegateInput("industry"), key(2), nil)
	if err := a.SetAllowance(ctx, del, sid, 2, ""); err == nil {
		t.Fatal("granted extra passes to a delegate")
	}
}
