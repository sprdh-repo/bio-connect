package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestPublicAppContent(t *testing.T) {
	a := mustApp(t)
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/app-content", nil))
	if rr.Code != 200 {
		t.Fatalf("app content: %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Event struct {
			Title string `json:"title"`
		} `json:"event"`
		ProductLaunch map[string]any   `json:"product_launch"`
		Partners      []map[string]any `json:"ecosystem_partners"`
		Speakers      []publicSpeaker  `json:"speakers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Event.Title != "Bio Connect 4.0" || len(out.ProductLaunch) == 0 || len(out.Partners) != 4 || len(out.Speakers) != 56 {
		t.Fatalf("incomplete app content: event=%q launch=%d partners=%d speakers=%d", out.Event.Title, len(out.ProductLaunch), len(out.Partners), len(out.Speakers))
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("public app content needs anonymous cross-origin reads")
	}
	if _, err := a.DB.Exec(t.Context(), "UPDATE app_content SET published=false WHERE id='mobile'"); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/app-content", nil))
	if rr.Code != 503 {
		t.Fatalf("unpublished app content returned %d", rr.Code)
	}
}
