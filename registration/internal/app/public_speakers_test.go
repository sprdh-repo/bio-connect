package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestPublicSpeakers(t *testing.T) {
	a := mustApp(t)
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/speakers", nil))
	if rr.Code != 200 {
		t.Fatalf("speakers: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Speakers []publicSpeaker `json:"speakers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Speakers) != 49 {
		t.Fatalf("got %d speakers, want 49", len(out.Speakers))
	}
	first := out.Speakers[0]
	if first.ID != "jayakrishna-ambati" || first.Name != "Dr. Jayakrishna Ambati" || first.ImageURL == "" {
		t.Fatalf("unexpected first speaker: %+v", first)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("public speakers need anonymous cross-origin reads")
	}
	if _, err := a.DB.Exec(t.Context(), "UPDATE speakers SET published=false WHERE id='jayakrishna-ambati'"); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/speakers", nil))
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Speakers) != 48 || out.Speakers[0].ID == "jayakrishna-ambati" {
		t.Fatalf("unpublished speaker remains visible: %s", rr.Body.String())
	}
}
