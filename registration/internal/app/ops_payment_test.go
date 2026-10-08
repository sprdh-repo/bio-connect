package app

import "testing"

func TestOpsGateRegistrationTypes(t *testing.T) {
	a := mustApp(t)
	c := opsReliabilityDesk(t, a, "Payment gate")
	day := opsDays[0].ID
	qrs := map[string]string{}
	if _, err := a.DB.Exec(t.Context(), `INSERT INTO free_registration_links(id,token_hash,expires_at,created_by) VALUES('gate-free','gate-free',now()+interval '1 day','ops-system')`); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"paid", "complimentary", "free_link"} {
		qr := seedOpsPass(t, a, kind+"@gate.example.com")
		qrs[kind] = qr
		// A free-link origin takes precedence over a complimentary flag, as in
		// the admin listing; zero fee alone does not classify a registration.
		if _, err := a.DB.Exec(t.Context(), `UPDATE registrations SET complimentary=$2,free_link_id=CASE WHEN $3 THEN 'gate-free' ELSE NULL END WHERE id=(SELECT a.registration_id FROM attendees a JOIN passes p ON p.attendee_id=a.id WHERE p.qr_id=$1)`, qr, kind != "paid", kind == "free_link"); err != nil {
			t.Fatal(err)
		}
		c.request("POST", "/api/v1/ops/check-in", map[string]any{"code": qr, "day": day})
	}
	gate := opsReliabilityGate(t, c, "Payment filter", "auto", nil, true)
	update := func(types []string) {
		t.Helper()
		rr := c.request("POST", "/api/v1/ops/points/"+gate, map[string]any{"allowedRegistrationTypes": types})
		if rr.Code != 200 {
			t.Fatalf("update: %d %s", rr.Code, rr.Body.String())
		}
		p := decodeOpsResponse(t, rr)["point"].(map[string]any)
		if p["active"] != true {
			t.Fatal("changing payment filter closed gate")
		}
	}
	scan := func(kind string, allowed bool) map[string]any {
		t.Helper()
		rr := c.request("POST", "/api/v1/ops/points/"+gate+"/scan", map[string]any{"code": qrs[kind], "day": day})
		if rr.Code != 200 {
			t.Fatal(rr.Body.String())
		}
		out := decodeOpsResponse(t, rr)
		if out["allowed"] != allowed || out["person"].(map[string]any)["registrationType"] != kind {
			t.Fatalf("%s allowed=%v: %v", kind, allowed, out)
		}
		if !allowed && out["reason"] != "This registration type is not allowed here." {
			t.Fatalf("wrong denial: %v", out)
		}
		return out
	}
	// Each type must match only its own filter, not a generic zero-fee flag.
	for _, allowed := range []string{"paid", "complimentary", "free_link"} {
		update([]string{allowed})
		for _, kind := range []string{"paid", "complimentary", "free_link"} {
			scan(kind, kind == allowed)
		}
		if rr := c.request("POST", "/api/v1/ops/points/"+gate, map[string]any{"allowedRegistrationTypes": []string{"typo"}}); rr.Code != 400 {
			t.Fatalf("invalid filter accepted: %d", rr.Code)
		}
		// Change the filter while this attendee is inside: exit must still work.
		update([]string{"paid", "complimentary", "free_link"})
		if out := scan(allowed, true); out["direction"] != "exit" {
			t.Fatalf("expected exit: %v", out)
		}
	}
	update([]string{})
	for kind := range qrs {
		scan(kind, true)
	}
	update([]string{"paid"})
	if out := scan("complimentary", true); out["direction"] != "exit" {
		t.Fatalf("excluded occupant cannot exit: %v", out)
	}
	scan("complimentary", false)
	update([]string{})
	scan("complimentary", true)
	if got := count(t, a, "SELECT count(*) FROM ops_activity WHERE kind='gate_rules_updated'"); got != 9 {
		t.Fatalf("filter changes audited %d times, want 9", got)
	}
	// Log-only mode reports the violation but still records an allowed scan.
	if _, err := a.DB.Exec(t.Context(), `UPDATE access_points SET mode='log',direction='entry' WHERE id=$1`, gate); err != nil {
		t.Fatal(err)
	}
	update([]string{"paid"})
	if out := scan("free_link", true); out["wouldDeny"] != "This registration type is not allowed here." {
		t.Fatalf("log-only filter not reported: %v", out)
	}
	// An unavailable audit must not change the saved filter.
	if _, err := a.DB.Exec(t.Context(), `CREATE FUNCTION reject_filter_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'controlled audit failure'; END $$; CREATE TRIGGER reject_filter_audit BEFORE INSERT ON ops_activity FOR EACH ROW EXECUTE FUNCTION reject_filter_audit()`); err != nil {
		t.Fatal(err)
	}
	if rr := c.request("POST", "/api/v1/ops/points/"+gate, map[string]any{"allowedRegistrationTypes": []string{"free_link"}}); rr.Code != 503 {
		t.Fatalf("unaudited filter accepted: %d", rr.Code)
	}
	if got := count(t, a, `SELECT count(*) FROM access_points WHERE id=$1 AND allowed_registration_types=ARRAY['paid']::text[]`, gate); got != 1 {
		t.Fatal("filter changed despite audit failure")
	}
}
