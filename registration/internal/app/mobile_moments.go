package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const momentsUploadLimit = 8 << 20

var errMobileSession = errors.New("mobile session is invalid")
var errMobilePass = errors.New("pass is not available")

// momentsAttendee authenticates the existing saved-pass session and maps one
// owned pass to an opaque, stable attendee identifier. No email or phone is
// sent to the Moments provider.
func (a *App) momentsAttendee(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	token := strings.TrimPrefix(header, "Bearer ")
	if !strings.HasPrefix(header, "Bearer ") || !admissionQRPattern.MatchString(token) {
		return "", errMobileSession
	}
	var channel, identifier, qr string
	err := a.DB.QueryRow(r.Context(), "SELECT channel,identifier,qr_id FROM mobile_pass_sessions WHERE token_hash=$1 AND expires_at>$2", hash(token), a.Now()).Scan(&channel, &identifier, &qr)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errMobileSession
	}
	if err != nil {
		return "", err
	}
	passID := strings.TrimSpace(r.URL.Query().Get("pass_id"))
	if passID == "" {
		return "", errMobilePass
	}
	var attendeeID string
	err = a.DB.QueryRow(r.Context(), "SELECT a.id"+mobilePassScope+" AND p.id=$4 ORDER BY p.created_at LIMIT 1", channel, identifier, qr, passID).Scan(&attendeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errMobilePass
	}
	if err != nil {
		return "", err
	}
	return "bio_connect_" + attendeeID, nil
}

func (a *App) momentsConfig(r *http.Request) (string, error) {
	content, _, published, err := loadMobileContent(r.Context(), a.DB)
	if err != nil {
		return "", err
	}
	if !published || content.Event.MomentsAlbumID == "" || a.Config.MomentsAPIBase == "" || a.Config.MomentsAPIToken == "" {
		return "", errors.New("Moments is not configured")
	}
	return content.Event.MomentsAlbumID, nil
}

func (a *App) momentsUpstream(r *http.Request, method, path, albumID string, body io.Reader, contentType string) (*http.Response, error) {
	endpoint, err := url.Parse(strings.TrimRight(a.Config.MomentsAPIBase, "/") + path)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("album_id", albumID)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(r.Context(), method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Config.MomentsAPIToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client.Do(req)
}

func momentsJSON(w http.ResponseWriter, response *http.Response, allowed ...int) {
	defer response.Body.Close()
	ok := false
	for _, status := range allowed {
		ok = ok || response.StatusCode == status
	}
	if !ok {
		fail(w, http.StatusBadGateway, "Moments is temporarily unavailable; try again")
		return
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil || !json.Valid(body) {
		fail(w, http.StatusBadGateway, "Moments returned an invalid response; try again")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func (a *App) mobileMoments(w http.ResponseWriter, r *http.Request) {
	externalID, err := a.momentsAttendee(r)
	if errors.Is(err, errMobileSession) {
		fail(w, http.StatusUnauthorized, "verify and save your pass to use Moments")
		return
	}
	if errors.Is(err, errMobilePass) {
		fail(w, http.StatusForbidden, "this pass is not available for Moments")
		return
	}
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "Moments is unavailable; retry shortly")
		return
	}
	albumID, err := a.momentsConfig(r)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "Moments is not configured for this event yet")
		return
	}

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/status"):
		response, err := a.momentsUpstream(r, http.MethodGet, "/photos/status/"+url.PathEscape(externalID), albumID, nil, "")
		if err != nil {
			fail(w, http.StatusBadGateway, "Could not connect to Moments; try again")
			return
		}
		momentsJSON(w, response, http.StatusOK)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/photos"):
		response, err := a.momentsUpstream(r, http.MethodGet, "/photos/end-user/external/"+url.PathEscape(externalID), albumID, nil, "")
		if err != nil {
			fail(w, http.StatusBadGateway, "Could not connect to Moments; try again")
			return
		}
		momentsJSON(w, response, http.StatusOK)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/selfie"):
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if a.limited(r.Context(), "moments-upload:"+hash(token), 10, time.Hour) {
			fail(w, http.StatusTooManyRequests, "too many selfie changes; try again later")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, momentsUploadLimit)
		if err := r.ParseMultipartForm(momentsUploadLimit); err != nil {
			fail(w, http.StatusBadRequest, "could not read the selfie")
			return
		}
		photo, header, err := r.FormFile("photo")
		if err != nil || header.Size <= 0 || header.Size > momentsUploadLimit {
			fail(w, http.StatusBadRequest, "choose a JPEG selfie smaller than 8 MB")
			return
		}
		defer photo.Close()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("photo", filepath.Base(header.Filename))
		if err == nil {
			_, err = io.Copy(part, io.LimitReader(photo, momentsUploadLimit+1))
		}
		if err == nil {
			err = writer.WriteField("album_id", albumID)
		}
		if err == nil {
			err = writer.WriteField("external_user_id", externalID)
		}
		if err == nil {
			err = writer.Close()
		}
		if err != nil {
			fail(w, http.StatusBadRequest, "could not read the selfie")
			return
		}
		response, err := a.momentsUpstream(r, http.MethodPost, "/photos/compare", albumID, &body, writer.FormDataContentType())
		if err != nil {
			fail(w, http.StatusBadGateway, "Could not upload the selfie; try again")
			return
		}
		momentsJSON(w, response, http.StatusOK, http.StatusCreated)
	case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/selfie"):
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if a.limited(r.Context(), "moments-delete:"+hash(token), 10, time.Hour) {
			fail(w, http.StatusTooManyRequests, "too many selfie changes; try again later")
			return
		}
		response, err := a.momentsUpstream(r, http.MethodDelete, "/photos/end-user/external/"+url.PathEscape(externalID), albumID, nil, "")
		if err != nil {
			fail(w, http.StatusBadGateway, "Could not remove the selfie; try again")
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			fail(w, http.StatusBadGateway, "Could not remove the selfie; try again")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
