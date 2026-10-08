package app

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func opsReliabilityDesk(t *testing.T, a *App, station string) *opsTestClient {
	t.Helper()
	a.Config.OpsKey = "reliability-key"
	c := &opsTestClient{t: t, h: a.Handler()}
	c.login(station, a.Config.OpsKey)
	return c
}

func opsReliabilityGate(t *testing.T, c *opsTestClient, name, direction string, capacity any, multiple bool) string {
	t.Helper()
	rr := c.request("POST", "/api/v1/ops/points", map[string]any{"name": name, "mode": "enforce", "direction": direction, "capacity": capacity, "allowedCategories": []string{}, "requireCheckIn": true, "allowMultipleEntries": multiple})
	if rr.Code != 201 {
		t.Fatalf("gate: %d %s", rr.Code, rr.Body.String())
	}
	return decodeOpsResponse(t, rr)["point"].(map[string]any)["id"].(string)
}

func TestOpsConcurrentGateRules(t *testing.T) {
	a := mustApp(t)
	clients := make([]*opsTestClient, 5)
	qrs := make([]string, 5)
	day := opsDays[0].ID
	for i := range clients {
		clients[i] = opsReliabilityDesk(t, a, fmt.Sprintf("Counter %d", i+1))
		qrs[i] = seedOpsPass(t, a, fmt.Sprintf("concurrent-%d@example.com", i))
		if rr := clients[i].request("POST", "/api/v1/ops/check-in", map[string]any{"code": qrs[i], "day": day}); rr.Code != 200 {
			t.Fatal(rr.Body.String())
		}
	}
	// Deterministically widen the policy-read/scan-write race, in a throwaway DB.
	if _, err := a.DB.Exec(t.Context(), `CREATE FUNCTION delay_gate() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.08); RETURN NEW; END $$; CREATE TRIGGER delay_gate BEFORE INSERT ON access_scans FOR EACH ROW EXECUTE FUNCTION delay_gate()`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		capacity       any
		multiple, same bool
	}{
		{"capacity", 1, true, false}, {"single-entry", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := opsReliabilityGate(t, clients[0], tc.name, "entry", tc.capacity, tc.multiple)
			var wg sync.WaitGroup
			start := make(chan struct{})
			results := make(chan bool, 5)
			for i := range clients {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					qr := qrs[i]
					if tc.same {
						qr = qrs[0]
					}
					rr := clients[i].request("POST", "/api/v1/ops/points/"+gate+"/scan", map[string]any{"code": qr, "day": day})
					if rr.Code != 200 {
						t.Errorf("scan: %d %s", rr.Code, rr.Body.String())
					}
					results <- decodeOpsResponse(t, rr)["allowed"] == true
				}(i)
			}
			close(start)
			wg.Wait()
			close(results)
			allowed := 0
			for yes := range results {
				if yes {
					allowed++
				}
			}
			if allowed != 1 {
				t.Errorf("allowed %d, want exactly one", allowed)
			}
			if n := count(t, a, "SELECT count(*) FROM access_scans WHERE access_point_id=$1", gate); n != 5 {
				t.Errorf("scans=%d", n)
			}
		})
	}
}

func TestOpsGateRevocationAndRequestReplay(t *testing.T) {
	a := mustApp(t)
	c := opsReliabilityDesk(t, a, "Gate")
	q1, q2 := seedOpsPass(t, a, "inside@example.com"), seedOpsPass(t, a, "outside@example.com")
	day := opsDays[0].ID
	for _, qr := range []string{q1, q2} {
		c.request("POST", "/api/v1/ops/check-in", map[string]any{"code": qr, "day": day})
	}
	gate := opsReliabilityGate(t, c, "Capacity", "auto", 1, true)
	scan := func(qr, request string) map[string]any {
		rr := c.request("POST", "/api/v1/ops/points/"+gate+"/scan", map[string]any{"code": qr, "day": day, "requestId": request})
		if rr.Code != 200 {
			t.Fatalf("scan: %d %s", rr.Code, rr.Body.String())
		}
		return decodeOpsResponse(t, rr)
	}
	first := scan(q1, "entry-request")
	replay := scan(q1, "entry-request")
	if replay["scanId"] != first["scanId"] || replay["direction"] != "entry" || replay["insideCount"] != float64(1) {
		t.Errorf("replay changed admission: %v", replay)
	}
	conflict := c.request("POST", "/api/v1/ops/points/"+gate+"/scan", map[string]any{"code": q2, "day": day, "requestId": "entry-request"})
	if conflict.Code != 409 {
		t.Errorf("request reused with another badge: %d", conflict.Code)
	}
	if _, err := a.DB.Exec(t.Context(), "UPDATE passes SET revoked_at=now() WHERE qr_id=$1", q1); err != nil {
		t.Fatal(err)
	}
	occ := decodeOpsResponse(t, c.request("GET", "/api/v1/ops/points/"+gate+"/occupancy?day="+day, nil))
	if occ["insideCount"] != float64(1) {
		t.Errorf("revocation erased occupancy: %v", occ)
	}
	if out := scan(q2, "outside-request"); out["allowed"] != false || out["reason"] != "This area is at capacity." {
		t.Errorf("capacity bypass: %v", out)
	}
	if out := scan(q1, "exit-request"); out["allowed"] != true || out["direction"] != "exit" || out["insideCount"] != float64(0) {
		t.Errorf("revoked attendee cannot exit: %v", out)
	}
	if out := scan(q1, "reentry-request"); out["allowed"] != false {
		t.Errorf("revoked badge reentered: %v", out)
	}
	if out := scan(q2, "new-entry-request"); out["allowed"] != true {
		t.Errorf("capacity not released: %v", out)
	}
	if n := count(t, a, "SELECT count(*) FROM access_scans WHERE access_point_id=$1", gate); n != 5 {
		t.Errorf("requests retained %d scans, want 5", n)
	}
}

func TestOpsAuditFailureRollsBackMutations(t *testing.T) {
	a := mustApp(t)
	c := opsReliabilityDesk(t, a, "Audit")
	qr := seedOpsPass(t, a, "audit@example.com")
	day := opsDays[0].ID
	gate := opsReliabilityGate(t, c, "Audit gate", "entry", nil, true)
	in := map[string]any{"code": qr, "day": day, "reason": "Wrong scan"}
	setFailure := func(yes bool) {
		t.Helper()
		q := "DROP TRIGGER reject_audit ON ops_activity; DROP FUNCTION reject_audit()"
		if yes {
			q = `CREATE FUNCTION reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'controlled audit failure'; END $$; CREATE TRIGGER reject_audit BEFORE INSERT ON ops_activity FOR EACH ROW EXECUTE FUNCTION reject_audit()`
		}
		if _, err := a.DB.Exec(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	setFailure(true)
	for _, path := range []string{"check-in", "badge/printed", "points/" + gate} {
		body := any(map[string]any{"code": qr, "day": day})
		if path == "points/"+gate {
			body = map[string]any{"active": false}
		}
		if rr := c.request("POST", "/api/v1/ops/"+path, body); rr.Code != 503 {
			t.Errorf("%s reported %d, want 503", path, rr.Code)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance"); n != 0 {
		t.Errorf("unaudited attendance committed: %d", n)
	}
	if n := count(t, a, "SELECT count(*) FROM access_points WHERE id=$1 AND active", gate); n != 1 {
		t.Error("unaudited gate close committed")
	}
	setFailure(false)
	if rr := c.request("POST", "/api/v1/ops/check-in", in); rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	setFailure(true)
	for _, path := range []string{"check-out", "check-in/undo", "points/" + gate + "/scan"} {
		body := any(in)
		if path == "points/"+gate+"/scan" {
			body = map[string]any{"code": qr, "day": day}
		}
		if rr := c.request("POST", "/api/v1/ops/"+path, body); rr.Code != 503 {
			t.Errorf("%s reported %d, want 503", path, rr.Code)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE checked_out_at IS NULL"); n != 1 {
		t.Error("attendance mutation survived audit failure")
	}
	if n := count(t, a, "SELECT count(*) FROM access_scans"); n != 0 {
		t.Error("unaudited gate admission committed")
	}
	setFailure(false)
	if rr := c.request("POST", "/api/v1/ops/check-out", in); rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	setFailure(true)
	if rr := c.request("POST", "/api/v1/ops/check-out/undo", in); rr.Code != 503 {
		t.Errorf("undo checkout = %d", rr.Code)
	}
	if n := count(t, a, "SELECT count(*) FROM ops_attendance WHERE checked_out_at IS NOT NULL"); n != 1 {
		t.Error("unaudited checkout undo committed")
	}
	setFailure(false)
	if rr := c.request("POST", "/api/v1/ops/check-out/undo", in); rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
}

func TestOpsScanAndGateManagementAudit(t *testing.T) {
	a := mustApp(t)
	c := opsReliabilityDesk(t, a, "Desk")
	qr := seedOpsPass(t, a, "scan-audit@example.com")
	day := opsDays[0].ID
	gate := opsReliabilityGate(t, c, "Managed gate", "entry", nil, true)
	for _, active := range []bool{false, true} {
		if rr := c.request("POST", "/api/v1/ops/points/"+gate, map[string]any{"active": active}); rr.Code != 200 {
			t.Fatal(rr.Body.String())
		}
	}
	for _, code := range []string{qr, "bad-code"} {
		c.request("GET", "/api/v1/ops/lookup?day="+day+"&code="+code, nil)
	}
	k := opsReliabilityDesk(t, a, "Kiosk")
	k.startKiosk()
	for _, code := range []string{qr, "bad-code", qr} {
		k.request("POST", "/api/v1/ops/kiosk/scan", map[string]any{"code": code})
	}
	for kind, want := range map[string]int{"gate_created": 1, "gate_opened": 1, "gate_closed": 1, "badge_lookup": 1, "badge_lookup_denied": 1, "kiosk_scan": 2, "check_in_denied": 1} {
		if got := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind=$1", kind); got != want {
			t.Errorf("%s audit=%d, want %d", kind, got, want)
		}
	}
}

func TestKioskPrintAuthorizationReplay(t *testing.T) {
	a := mustApp(t)
	k := opsReliabilityDesk(t, a, "Kiosk")
	k.startKiosk()
	qr := seedOpsPass(t, a, "authorization@example.com")
	request := map[string]any{"code": qr, "requestId": "print-authorization"}
	var first map[string]any
	for i := 0; i < 3; i++ {
		rr := k.request("POST", "/api/v1/ops/kiosk/print", request)
		if rr.Code != 200 {
			t.Fatalf("replay %d: %d %s", i, rr.Code, rr.Body.String())
		}
		out := decodeOpsResponse(t, rr)
		if i == 0 {
			first = out
		}
		if out["firstCheckIn"] != first["firstCheckIn"] || out["day"] != first["day"] {
			t.Errorf("replay changed response: %v", out)
		}
	}
	if n := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind='badge_print'"); n != 1 {
		t.Errorf("replay consumed %d print authorizations", n)
	}
	if rr := k.request("POST", "/api/v1/ops/kiosk/print", map[string]any{"code": qr, "requestId": "print-authorization", "retry": true}); rr.Code != 409 {
		t.Errorf("changed retry reused ID: %d", rr.Code)
	}
	other := opsReliabilityDesk(t, a, "Kiosk")
	other.startKiosk()
	if rr := other.request("POST", "/api/v1/ops/kiosk/print", request); rr.Code != http.StatusConflict {
		t.Errorf("another session replayed authorization: %d", rr.Code)
	}
}
