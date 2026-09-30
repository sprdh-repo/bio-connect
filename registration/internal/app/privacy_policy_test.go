package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrivacyPolicyPublic(t *testing.T) {
	a := &App{}
	for _, path := range []string{"/privacy-policy", "/static/privacy-policy.css"} {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if path == "/privacy-policy" {
			if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
				t.Fatal("policy must be HTML")
			}
			for _, text := range []string{"Bio Connect 4.0", "Retention and deletion", "bioconnect@bio360.in"} {
				if !strings.Contains(w.Body.String(), text) {
					t.Fatalf("missing %q", text)
				}
			}
		}
	}
}
