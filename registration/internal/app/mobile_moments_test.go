package app

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMobileMomentsJourneyUsesConfiguredAlbumAndOpaqueAttendee(t *testing.T) {
	var externalID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer moments-secret" {
			t.Errorf("upstream authorization = %q", r.Header.Get("Authorization"))
		}
		if got := r.URL.Query().Get("album_id"); got != "42" {
			t.Errorf("album_id = %q", got)
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/photos/status/"):
			externalID = parts[len(parts)-1]
			_ = json.NewEncoder(w).Encode(map[string]any{"album_ready": true, "selfie_uploaded": false, "similar_photos_count": 0, "similar_photos_ready": false})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/photos/end-user/external/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "completed", "results": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/photos/compare":
			if err := r.ParseMultipartForm(6 << 20); err != nil {
				t.Fatal(err)
			}
			if r.FormValue("album_id") != "42" || r.FormValue("external_user_id") != externalID {
				t.Errorf("upload fields: album=%q external=%q", r.FormValue("album_id"), r.FormValue("external_user_id"))
			}
			file, header, err := r.FormFile("photo")
			if err != nil {
				t.Fatal(err)
			}
			// Moments rejects any photo part not labelled image/jpeg or image/png.
			if got := header.Header.Get("Content-Type"); got != "image/jpeg" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"Invalid file type"}`))
				return
			}
			body, _ := io.ReadAll(file)
			if !bytes.Equal(body, testJPEG) {
				t.Errorf("photo = %q", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"processing"}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/photos/end-user/external/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	a := mustApp(t)
	a.Config.MomentsAPIBase = upstream.URL
	a.Config.MomentsAPIToken = "moments-secret"
	if _, err := a.DB.Exec(t.Context(), `UPDATE app_content SET document=jsonb_set(document,'{event,moments_album_id}','"42"') WHERE id='mobile'`); err != nil {
		t.Fatal(err)
	}
	sid, _ := addStaff(t, a, "moments@example.com", "reviewer")
	if _, err := a.StaffCreate(t.Context(), staffInput(delegateInput("student"), "complimentary"), key(91), nil, sid); err != nil {
		t.Fatal(err)
	}
	challenge, code := issueMobileCode(t, a, "email", "asha@example.com", "")
	token := verifyMobileCode(t, a, challenge, code)
	passes := mobileRequest(a, http.MethodGet, "/api/v1/mobile/passes", nil, token)
	var passBody struct {
		Passes []mobilePass `json:"passes"`
	}
	if err := json.Unmarshal(passes.Body.Bytes(), &passBody); err != nil {
		t.Fatal(err)
	}
	passID := passBody.Passes[0].ID

	status := mobileRequest(a, http.MethodGet, "/api/v1/mobile/moments/status?pass_id="+passID, nil, token)
	if status.Code != http.StatusOK {
		t.Fatalf("status: %d %s", status.Code, status.Body.String())
	}
	if !strings.HasPrefix(externalID, "bio_connect_") || strings.Contains(externalID, "asha") || strings.Contains(externalID, "example") {
		t.Fatalf("unsafe external id %q", externalID)
	}
	photos := mobileRequest(a, http.MethodGet, "/api/v1/mobile/moments/photos?pass_id="+passID, nil, token)
	if photos.Code != http.StatusOK {
		t.Fatalf("photos: %d %s", photos.Code, photos.Body.String())
	}

	uploadSelfie := func(photo []byte) *httptest.ResponseRecorder {
		var upload bytes.Buffer
		mw := multipart.NewWriter(&upload)
		part, err := mw.CreateFormFile("photo", "selfie.jpg")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(photo)
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mobile/moments/selfie?pass_id="+passID, &upload)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, req)
		return rr
	}
	if rr := uploadSelfie(testJPEG); rr.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rr.Code, rr.Body.String())
	}
	if rr := uploadSelfie([]byte("not an image")); rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "JPEG or PNG") {
		t.Fatalf("non-image upload: %d %s", rr.Code, rr.Body.String())
	}
	removed := mobileRequest(a, http.MethodDelete, "/api/v1/mobile/moments/selfie?pass_id="+passID, nil, token)
	if removed.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", removed.Code, removed.Body.String())
	}
}

var testJPEG = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00jpeg-data")

func TestMobileMomentsRejectsUnverifiedSession(t *testing.T) {
	a := mustApp(t)
	rr := mobileRequest(a, http.MethodGet, "/api/v1/mobile/moments/status?pass_id=not-owned", nil, strings.Repeat("a", 43))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unverified status = %d", rr.Code)
	}
}
