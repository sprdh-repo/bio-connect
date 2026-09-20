package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicExhibitors(t *testing.T) {
	a := mustApp(t)
	ctx := context.Background()
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		return rr
	}
	empty := request("/api/v1/public/exhibitors")
	if empty.Code != 200 || strings.TrimSpace(empty.Body.String()) != `{"exhibitors":[]}` {
		t.Fatalf("empty directory: %d %s", empty.Code, empty.Body.String())
	}
	var approved string
	for i, state := range []string{"approved", "awaiting_payment", "awaiting_review", "correction_requested", "rejected", "cancelled"} {
		in := exhibitorInput("table", 2)
		in.Institution = state
		rid, _, err := a.Create(ctx, in, key(i+1), tinyPNG(t))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.DB.Exec(ctx, "UPDATE registrations SET status=$2 WHERE id=$1", rid, state); err != nil {
			t.Fatal(err)
		}
		if state == "approved" {
			approved = rid
		}
	}
	rid, _, err := a.Create(ctx, delegateInput("industry"), key(20), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(ctx, "UPDATE registrations SET status='approved' WHERE id=$1", rid); err != nil {
		t.Fatal(err)
	}
	rr := request("/api/v1/public/exhibitors")
	var out struct {
		Exhibitors []map[string]any `json:"exhibitors"`
	}
	if rr.Code != 200 {
		t.Fatalf("directory: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Exhibitors) != 1 {
		t.Fatalf("exposed non-approved exhibitor or delegate: %s", rr.Body.String())
	}
	entry := out.Exhibitors[0]
	if len(entry) != 3 || entry["name"] != "approved" || entry["description"] != "Molecular diagnostics" {
		t.Fatalf("unexpected public fields: %v", entry)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("public directory needs anonymous cross-origin reads")
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("approval changes must not be cached")
	}
	logo := entry["logo_url"].(string)
	if got := request(logo); got.Code != 200 || got.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("logo: %d", got.Code)
	}
	receipt := addFile(t, a, approved, "receipt", tinyPNG(t))
	if got := request("/api/v1/public/exhibitors/logos/" + receipt); got.Code != 404 {
		t.Fatalf("receipt exposed: %d", got.Code)
	}
	if _, err = a.DB.Exec(ctx, "UPDATE registrations SET status='cancelled' WHERE id=$1", approved); err != nil {
		t.Fatal(err)
	}
	if got := request(logo); got.Code != 404 {
		t.Fatalf("cancelled exhibitor logo exposed: %d", got.Code)
	}
	if got := request("/api/v1/public/exhibitors"); strings.TrimSpace(got.Body.String()) != `{"exhibitors":[]}` {
		t.Fatalf("cancelled entry remains: %s", got.Body.String())
	}
	if _, err = a.DB.Exec(ctx, "UPDATE registrations SET status='approved' WHERE id=$1", approved); err != nil {
		t.Fatal(err)
	}
	if got := request(logo); got.Code != 200 {
		t.Fatalf("restored logo: %d", got.Code)
	}
	if got := request("/api/v1/public/exhibitors/logos/missing"); got.Code != 404 {
		t.Fatalf("unknown logo: %d", got.Code)
	}
	// Old approved registrations may lack a logo; retain the organisation.
	if _, err = a.DB.Exec(ctx, "DELETE FROM files WHERE registration_id=$1 AND kind='logo'", approved); err != nil {
		t.Fatal(err)
	}
	missing := request("/api/v1/public/exhibitors")
	if err := json.Unmarshal(missing.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Exhibitors) != 1 || out.Exhibitors[0]["logo_url"] != "" {
		t.Fatalf("missing logo dropped organisation: %s", missing.Body.String())
	}
	latest := addFile(t, a, approved, "logo", tinyPNG(t))
	if err := json.Unmarshal(request("/api/v1/public/exhibitors").Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Exhibitors[0]["logo_url"] != "/api/v1/public/exhibitors/logos/"+latest {
		t.Fatalf("replacement logo absent: %v", out.Exhibitors)
	}
}
