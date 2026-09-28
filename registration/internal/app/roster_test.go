package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

// preUpgrade puts the exhibitor allowances back to 3 / 2 / 2 so registrations
// can be made the way they were before migration 009, and upgrade re-applies
// 009 to them as a deploy would.
func preUpgrade(t *testing.T, a *App) {
	t.Helper()
	if _, err := a.DB.Exec(context.Background(), "UPDATE categories SET roster_count=CASE id WHEN 'premium' THEN 3 ELSE 2 END WHERE kind='exhibitor'"); err != nil {
		t.Fatal(err)
	}
}

func upgrade(t *testing.T, a *App) {
	t.Helper()
	b, err := resources.ReadFile("migrations/009_exhibitor_pass_upgrade.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), string(b)); err != nil {
		t.Fatalf("apply 009: %v", err)
	}
}

func extra(n int) Attendee {
	return Attendee{Name: fmt.Sprintf("Extra %d", n), Email: fmt.Sprintf("extra%d@example.com", n), Phone: fmt.Sprintf("+9198000000%02d", n), Designation: "Engineer", WhatsAppConsent: true}
}

func rosterCount(t *testing.T, a *App, rid string) int {
	return count(t, a, "SELECT roster_count FROM registrations WHERE id=$1", rid)
}

func passNumbers(t *testing.T, a *App, rid string) map[string]string {
	t.Helper()
	rows, err := a.DB.Query(context.Background(), "SELECT id,number FROM passes WHERE registration_id=$1 AND revoked_at IS NULL", rid)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, n string
		if err := rows.Scan(&id, &n); err != nil {
			t.Fatal(err)
		}
		out[id] = n
	}
	return out
}

func TestUpgradeGrantsLiveExhibitorsTheNewAllowance(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	preUpgrade(t, a)
	approved, _, _ := a.Create(ctx, exhibitorInput("premium", 3), key(1), tinyPNG(t))
	payExhibitor(t, a, approved, earlyPaise(t, a, approved))
	if err := tryApprove(a, approved, sid, "approve_send", "SBIUP1", earlyPaise(t, a, approved)); err != nil {
		t.Fatal(err)
	}
	before := passNumbers(t, a, approved)
	jobsBefore := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", approved)
	pending, _, _ := a.Create(ctx, exhibitorInput("standard", 2), key(2), tinyPNG(t))
	cancelled, _, _ := a.Create(ctx, exhibitorInput("premium", 3), key(3), tinyPNG(t))
	if err := a.Review(ctx, cancelled, sid, ReviewInput{Action: "cancelled", Note: "withdrew"}); err != nil {
		t.Fatal(err)
	}
	table, _, _ := a.Create(ctx, exhibitorInput("table", 2), key(4), tinyPNG(t))

	upgrade(t, a)
	upgrade(t, a) // a re-run changes nothing

	for rid, want := range map[string]int{approved: 5, pending: 3, cancelled: 3, table: 2} {
		if got := rosterCount(t, a, rid); got != want {
			t.Fatalf("roster_count=%d, want %d", got, want)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE action='roster_upgraded'"); n != 2 {
		t.Fatalf("%d upgrade audit rows, want 2 (approved and pending only, once each)", n)
	}
	var cats [3]int
	a.DB.QueryRow(ctx, "SELECT (SELECT roster_count FROM categories WHERE id='premium'),(SELECT roster_count FROM categories WHERE id='standard'),(SELECT roster_count FROM categories WHERE id='table')").Scan(&cats[0], &cats[1], &cats[2])
	if cats != [3]int{5, 3, 2} {
		t.Fatalf("category allowances %v, want 5 / 3 / 2", cats)
	}
	after := passNumbers(t, a, approved)
	if fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("upgrade changed existing passes: %v -> %v", before, after)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1", approved); n != jobsBefore {
		t.Fatalf("upgrade queued deliveries: %d -> %d", jobsBefore, n)
	}
}

// After approval and delivery, an added attendee gets the next pass number and
// the only new pass delivery; nobody already holding a pass is sent anything.
func TestAddAttendeeAfterDeliveryIssuesOnlyTheNewPass(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	preUpgrade(t, a)
	rid, _, _ := a.Create(ctx, exhibitorInput("premium", 3), key(1), tinyPNG(t))
	payExhibitor(t, a, rid, earlyPaise(t, a, rid))
	if err := tryApprove(a, rid, sid, "approve_send", "SBIADD", earlyPaise(t, a, rid)); err != nil {
		t.Fatal(err)
	}
	upgrade(t, a)
	old := passNumbers(t, a, rid)
	passJobs := "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass'"
	before := count(t, a, passJobs, rid)

	if err := a.AddAttendee(ctx, rid, "", extra(1)); err != nil {
		t.Fatalf("self-serve add: %v", err)
	}
	if err := a.AddAttendee(ctx, rid, sid, extra(2)); err != nil {
		t.Fatalf("staff add: %v", err)
	}
	for i, want := range []string{"-4", "-5"} {
		var reference string
		a.DB.QueryRow(ctx, "SELECT reference FROM registrations WHERE id=$1", rid).Scan(&reference)
		if n := nth(t, a, rid, 3+i); n != reference+want {
			t.Fatalf("added pass %d numbered %q, want %q", i+1, n, reference+want)
		}
	}
	// Two new holders, each by email and (consented) WhatsApp.
	if n := count(t, a, passJobs, rid) - before; n != 4 {
		t.Fatalf("%d new pass deliveries, want 4", n)
	}
	for pid := range old {
		if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE pass_id=$1 AND dedupe_key NOT LIKE 'initial:%'", pid); n != 0 {
			t.Fatalf("existing pass %s was re-sent", old[pid])
		}
		if n := count(t, a, "SELECT count(*) FROM passes WHERE id=$1 AND revoked_at IS NULL", pid); n != 1 {
			t.Fatalf("existing pass %s was revoked", old[pid])
		}
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pack'", rid); n != 3 {
		t.Fatalf("%d pack emails, want the original plus one per added attendee", n)
	}
	// A later "send" does not deliver the added passes a second time.
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, passJobs, rid) - before; n != 4 {
		t.Fatalf("send after adding re-queued passes: %d new deliveries, want 4", n)
	}
	if err := a.AddAttendee(ctx, rid, "", extra(3)); err != errRosterFull {
		t.Fatalf("add beyond the allowance: %v, want errRosterFull", err)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='attendee_added'", rid); n != 2 {
		t.Fatalf("%d attendee_added audit rows, want 2", n)
	}
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE registration_id=$1 AND action='attendee_added' AND staff_id=$2", rid, sid); n != 1 {
		t.Fatal("staff add not attributed to the reviewer")
	}
}

// A registration still awaiting payment approves with open places, issuing
// passes only for the people on it; the place filled after that gets its pass then.
func TestApprovalWithOpenPlacesThenAdd(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	preUpgrade(t, a)
	rid, _, _ := a.Create(ctx, exhibitorInput("standard", 2), key(1), tinyPNG(t))
	upgrade(t, a)
	payExhibitor(t, a, rid, earlyPaise(t, a, rid))
	if err := tryApprove(a, rid, sid, "approve_only", "SBIOPEN", earlyPaise(t, a, rid)); err != nil {
		t.Fatalf("approve with an open place: %v", err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 2 {
		t.Fatalf("%d passes, want 2 for the 2 attendees", n)
	}
	if err := a.AddAttendee(ctx, rid, "", extra(1)); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, "SELECT count(*) FROM passes WHERE registration_id=$1", rid); n != 3 {
		t.Fatalf("%d passes after add, want 3", n)
	}
	// approve_only sent nothing, so neither does the add; the team's first send carries all three.
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose IN ('pass','pack')", rid); n != 0 {
		t.Fatalf("add before any send queued %d deliveries", n)
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "send"}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='pass' AND channel='email'", rid); n != 3 {
		t.Fatalf("%d pass emails after send, want 3", n)
	}
}

func TestAddAttendeeGuards(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	delegate, _, _ := a.Create(ctx, delegateInput("industry"), key(1), nil)
	if err := a.AddAttendee(ctx, delegate, "", extra(1)); err == nil {
		t.Fatal("added an attendee to a delegate registration")
	}
	full, _, _ := a.Create(ctx, exhibitorInput("table", 2), key(2), tinyPNG(t))
	if err := a.AddAttendee(ctx, full, "", extra(1)); err != errRosterFull {
		t.Fatalf("full roster: %v, want errRosterFull", err)
	}
	preUpgrade(t, a)
	rid, _, _ := a.Create(ctx, exhibitorInput("premium", 3), key(3), tinyPNG(t))
	upgrade(t, a)
	dup := extra(1)
	dup.Email = "REP0@example.com"
	if err := a.AddAttendee(ctx, rid, "", dup); err == nil {
		t.Fatal("added someone already on the registration")
	}
	bad := extra(1)
	bad.Phone = "98765"
	if err := a.AddAttendee(ctx, rid, "", bad); err == nil {
		t.Fatal("accepted an invalid phone")
	}
	if err := a.Review(ctx, rid, sid, ReviewInput{Action: "cancelled", Note: "withdrew"}); err != nil {
		t.Fatal(err)
	}
	if err := a.AddAttendee(ctx, rid, "", extra(1)); err != ErrConflict {
		t.Fatalf("cancelled registration: %v, want ErrConflict", err)
	}
}

func TestRosterNoticeReachesOnlyOpenExhibitorsOnce(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	sid, _ := addStaff(t, a, "reviewer@bioconnect.test", "reviewer")
	a.Create(ctx, delegateInput("industry"), key(1), nil)
	a.Create(ctx, exhibitorInput("table", 2), key(2), tinyPNG(t))
	preUpgrade(t, a)
	open, _, _ := a.Create(ctx, exhibitorInput("premium", 3), key(3), tinyPNG(t))
	gone, _, _ := a.Create(ctx, exhibitorInput("standard", 2), key(4), tinyPNG(t))
	upgrade(t, a)
	if err := a.Review(ctx, gone, sid, ReviewInput{Action: "cancelled", Note: "withdrew"}); err != nil {
		t.Fatal(err)
	}

	results, err := a.NotifyOpenPlaces(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Outcome != "queued" {
		t.Fatalf("notify results %+v, want just the open premium registration", results)
	}
	if n := count(t, a, "SELECT count(*) FROM delivery_jobs WHERE registration_id=$1 AND purpose='roster_notice'", open); n != 1 {
		t.Fatalf("%d notices queued, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM recovery_tokens WHERE registration_id=$1", open); n != 1 {
		t.Fatal("notice carries no recovery link")
	}
	if again, _ := a.NotifyOpenPlaces(ctx); len(again) != 0 {
		t.Fatalf("re-run notified %d registrations again", len(again))
	}
	if err := a.Review(ctx, open, sid, ReviewInput{Action: "roster_notice"}); err == nil {
		t.Fatal("second notice inside 24 hours was accepted")
	}

	// Filled before the worker reached it, the notice is dropped rather than sent.
	a.AddAttendee(ctx, open, "", extra(1))
	a.AddAttendee(ctx, open, "", extra(2))
	if _, err := a.DB.Exec(ctx, "UPDATE delivery_jobs SET status='cancelled' WHERE purpose<>'roster_notice'"); err != nil {
		t.Fatal(err)
	}
	if err := a.WorkOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var st string
	a.DB.QueryRow(ctx, "SELECT status FROM delivery_jobs WHERE registration_id=$1 AND purpose='roster_notice'", open).Scan(&st)
	if st != "cancelled" {
		t.Fatalf("notice for a filled roster is %q, want cancelled", st)
	}
}

// The exhibitor fills places through their management link, and only their own.
func TestSelfServeAddAttendeeRoute(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	preUpgrade(t, a)
	rid, token, _ := a.Create(ctx, exhibitorInput("premium", 3), key(1), tinyPNG(t))
	other, otherToken, _ := a.Create(ctx, exhibitorInput("premium", 3), key(2), tinyPNG(t))
	upgrade(t, a)
	h := a.Handler()
	post := func(rid, token string) int {
		body, _ := json.Marshal(extra(1))
		req := httptest.NewRequest("POST", "/api/v1/registrations/"+rid+"/attendees", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	if code := post(rid, otherToken); code != 401 {
		t.Fatalf("another registration's token returned %d, want 401", code)
	}
	if code := post(rid, token); code != 200 {
		t.Fatalf("own token returned %d, want 200", code)
	}
	if code := post(rid, token); code != 409 {
		t.Fatalf("repeated add returned %d, want 409", code)
	}
	reg, err := a.registration(ctx, rid)
	if err != nil {
		t.Fatal(err)
	}
	if reg.RosterCount != 5 || len(reg.Attendees) != 4 {
		t.Fatalf("registration shows %d of %d, want 4 of 5", len(reg.Attendees), reg.RosterCount)
	}
	if n := count(t, a, "SELECT count(*) FROM attendees WHERE registration_id=$1", other); n != 3 {
		t.Fatalf("other registration has %d attendees, want 3", n)
	}
}
