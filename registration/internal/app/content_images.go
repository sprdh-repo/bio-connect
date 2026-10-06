package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"io"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5"
	_ "golang.org/x/image/webp"
)

// Speaker portraits match the website's 4:5 portrait assets exactly, so the
// website build can publish an uploaded portrait without resizing it.
const (
	portraitWidth  = 480
	portraitHeight = 600
	portraitMax    = 2 << 20
)

var contentImageFile = regexp.MustCompile(`^([0-9a-f]{64})\.(webp|jpg)$`)

// validatePortrait checks an editor upload: a WebP or JPEG of exactly the
// portrait size that decodes completely.
func validatePortrait(b []byte) (string, error) {
	if len(b) == 0 || len(b) > portraitMax {
		return "", errors.New("portrait must be between 1 byte and 2 MB")
	}
	mime := http.DetectContentType(b)
	if mime != "image/webp" && mime != "image/jpeg" {
		return "", errors.New("portrait must be a WebP or JPEG image")
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil {
		return "", errors.New("this portrait is not a readable image")
	}
	if cfg.Width != portraitWidth || cfg.Height != portraitHeight {
		return "", errors.New("portrait must be 480 × 600 pixels")
	}
	if _, _, e = image.Decode(bytes.NewReader(b)); e != nil {
		return "", errors.New("this portrait is truncated or corrupt")
	}
	return mime, nil
}

func contentImageURL(base, id, mime string) string {
	ext := "webp"
	if mime == "image/jpeg" {
		ext = "jpg"
	}
	return base + "/api/v1/public/images/" + id + "." + ext
}

// uploadContentImage stores a portrait from the mobile content editor and
// returns its permanent public URL. Uploading the same bytes again returns the
// same URL.
func (a *App) uploadContentImage(w http.ResponseWriter, r *http.Request, staff string) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		fail(w, 405, "method not allowed")
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, portraitMax))
	if e != nil {
		fail(w, 413, "portrait exceeds 2 MB")
		return
	}
	mime, e := validatePortrait(b)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	sum := sha256.Sum256(b)
	id := hex.EncodeToString(sum[:])
	var exists bool
	if e = a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM content_images WHERE id=$1)", id).Scan(&exists); e != nil {
		fail(w, 503, "upload unavailable")
		return
	}
	if !exists {
		if e = a.Storage.Put(r.Context(), "image-"+id, b, mime); e != nil {
			fail(w, 503, "upload failed; please retry")
			return
		}
		if _, e = a.DB.Exec(r.Context(), "INSERT INTO content_images(id,kind,mime,size,created_by) VALUES($1,'portrait',$2,$3,$4) ON CONFLICT DO NOTHING", id, mime, len(b), staff); e != nil {
			fail(w, 503, "upload could not be saved; retry")
			return
		}
	}
	respond(w, 201, map[string]string{"url": contentImageURL(a.Config.BaseURL, id, mime)})
}

// publicContentImage serves an uploaded image. The URL names the content
// hash, so the response never changes and is cached for a year.
func (a *App) publicContentImage(w http.ResponseWriter, r *http.Request) {
	m := contentImageFile.FindStringSubmatch(r.PathValue("file"))
	if m == nil {
		fail(w, 404, "image not found")
		return
	}
	var mime string
	e := a.DB.QueryRow(r.Context(), "SELECT mime FROM content_images WHERE id=$1", m[1]).Scan(&mime)
	if errors.Is(e, pgx.ErrNoRows) || (e == nil && (mime == "image/jpeg") != (m[2] == "jpg")) {
		fail(w, 404, "image not found")
		return
	}
	if e != nil {
		fail(w, 503, "image unavailable; please retry")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	a.serveInline(w, r, "image-"+m[1], mime)
}
