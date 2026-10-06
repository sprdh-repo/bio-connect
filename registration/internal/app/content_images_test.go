package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A portrait upload is stored once under its content hash and served from a
// permanent, cacheable public URL; anything but a 480 × 600 WebP or JPEG is refused.
func TestContentImagePortraitUpload(t *testing.T) {
	a := mustApp(t)
	staff, _ := addStaff(t, a, "editor@example.com", "manager")
	session, csrf := randomToken(), randomToken()
	if _, err := a.DB.Exec(t.Context(), "INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')", hash(session), staff, hash(csrf)); err != nil {
		t.Fatal(err)
	}
	upload := func(b []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/admin/content-images", bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/octet-stream")
		r.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		r.Header.Set("X-CSRF-Token", csrf)
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, r)
		return rr
	}
	webp, err := os.ReadFile("testdata/portrait.webp")
	if err != nil {
		t.Fatal(err)
	}

	rr := upload(webp)
	if rr.Code != 201 {
		t.Fatalf("webp upload: %d %s", rr.Code, rr.Body.String())
	}
	var out struct{ URL string }
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if !strings.HasPrefix(out.URL, a.Config.BaseURL+"/api/v1/public/images/") || !strings.HasSuffix(out.URL, ".webp") {
		t.Fatalf("url = %q", out.URL)
	}
	if again := upload(webp); again.Code != 201 || !strings.Contains(again.Body.String(), out.URL) {
		t.Fatalf("re-upload: %d %s", again.Code, again.Body.String())
	}
	if n := count(t, a, "SELECT count(*) FROM content_images"); n != 1 {
		t.Fatalf("stored %d images, want 1", n)
	}

	path := strings.TrimPrefix(out.URL, a.Config.BaseURL)
	get := httptest.NewRecorder()
	a.Handler().ServeHTTP(get, httptest.NewRequest("GET", path, nil))
	if get.Code != 200 || !bytes.Equal(get.Body.Bytes(), webp) || get.Header().Get("Content-Type") != "image/webp" || !strings.Contains(get.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("serve: %d %v", get.Code, get.Header())
	}
	for _, bad := range []string{strings.TrimSuffix(path, ".webp") + ".jpg", "/api/v1/public/images/" + strings.Repeat("0", 64) + ".webp", "/api/v1/public/images/..%2Fsecret.webp"} {
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", bad, nil))
		if rr.Code != 404 {
			t.Fatalf("%s: %d, want 404", bad, rr.Code)
		}
	}

	if rr := upload(jpegOf(t, 480, 600)); rr.Code != 201 || !strings.HasSuffix(strings.TrimSpace(rr.Body.String()), `.jpg"}`) {
		t.Fatalf("jpeg upload: %d %s", rr.Code, rr.Body.String())
	}
	for name, b := range map[string][]byte{
		"wrong size": jpegOf(t, 600, 600),
		"png":        tinyPNG(t),
		"truncated":  webp[:len(webp)/2],
		"empty":      nil,
	} {
		if rr := upload(b); rr.Code != 400 {
			t.Fatalf("%s accepted: %d %s", name, rr.Code, rr.Body.String())
		}
	}

	anon := httptest.NewRecorder()
	a.Handler().ServeHTTP(anon, httptest.NewRequest("POST", "/api/v1/admin/content-images", bytes.NewReader(webp)))
	if anon.Code != 401 {
		t.Fatalf("anonymous upload: %d", anon.Code)
	}
}

// The public speaker list carries a version that changes only with its content.
func TestPublicSpeakersVersion(t *testing.T) {
	a := mustApp(t)
	version := func() string {
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/public/speakers", nil))
		var out struct{ Version string }
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		if len(out.Version) != 16 || !strings.Contains(rr.Header().Get("Cache-Control"), "max-age=60") {
			t.Fatalf("version %q, cache %q", out.Version, rr.Header().Get("Cache-Control"))
		}
		return out.Version
	}
	first := version()
	if version() != first {
		t.Fatal("version changed without an edit")
	}
	if _, err := a.DB.Exec(t.Context(), "UPDATE speakers SET role='Director General' WHERE id='beena-pillai'"); err != nil {
		t.Fatal(err)
	}
	if version() == first {
		t.Fatal("version unchanged after an edit")
	}
}
