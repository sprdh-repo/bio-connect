package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, error)
	URL(context.Context, string) (string, error)
}
type localStorage struct{ root string }

func (s localStorage) path(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, "/\\") {
		return "", errors.New("invalid object key")
	}
	return filepath.Join(s.root, key), nil
}
func (s localStorage) Put(_ context.Context, key string, b []byte, _ string) error {
	p, e := s.path(key)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(s.root, 0700); e != nil {
		return e
	}
	return os.WriteFile(p, b, 0600)
}
func (s localStorage) Get(_ context.Context, key string) ([]byte, error) {
	p, e := s.path(key)
	if e != nil {
		return nil, e
	}
	return os.ReadFile(p)
}
func (s localStorage) URL(context.Context, string) (string, error) { return "", nil }

type s3Storage struct {
	client *s3.Client
	bucket string
}

func (s s3Storage) Put(ctx context.Context, key string, b []byte, mime string) error {
	_, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(b), ContentType: aws.String(mime)})
	return e
}
func (s s3Storage) Get(ctx context.Context, key string) ([]byte, error) {
	v, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if e != nil {
		return nil, e
	}
	defer v.Body.Close()
	return io.ReadAll(io.LimitReader(v.Body, 20<<20))
}
func (s s3Storage) URL(ctx context.Context, key string) (string, error) {
	v, e := s3.NewPresignClient(s.client).PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ResponseContentDisposition: aws.String("attachment"), ResponseCacheControl: aws.String("private, no-store")}, func(o *s3.PresignOptions) { o.Expires = 60 * time.Second })
	if e != nil {
		return "", e
	}
	return v.URL, nil
}
func newStorage(ctx context.Context, c Config) (Storage, error) {
	if c.S3Bucket == "" {
		return localStorage{c.StorageDir}, nil
	}
	cfg, e := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(c.AWSRegion))
	if e != nil {
		return nil, e
	}
	return s3Storage{s3.NewFromConfig(cfg), c.S3Bucket}, nil
}
func validateUpload(b []byte, kind string) (string, error) {
	if len(b) == 0 || len(b) > 5<<20 {
		return "", errors.New("file must be between 1 byte and 5 MB")
	}
	mime := http.DetectContentType(b)
	if mime == "image/png" || mime == "image/jpeg" {
		cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 6000 || cfg.Height > 6000 || int64(cfg.Width)*int64(cfg.Height) > 20000000 {
			return "", errors.New("invalid or oversized image")
		}
		if _, _, e = image.Decode(bytes.NewReader(b)); e != nil {
			return "", errors.New("invalid image")
		}
		return mime, nil
	}
	if kind == "receipt" && mime == "application/pdf" && bytes.Contains(b, []byte("%%EOF")) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pdfinfo", "-")
		cmd.Stdin = bytes.NewReader(b)
		if e := cmd.Run(); e != nil {
			return "", errors.New("invalid, encrypted or unreadable PDF receipt")
		}
		return mime, nil
	}
	return "", errors.New("use a PNG/JPEG logo, or a PDF/PNG/JPEG receipt")
}
func (a *App) upload(w http.ResponseWriter, r *http.Request, rid string) {
	kind := r.URL.Query().Get("kind")
	if kind != "logo" && kind != "receipt" {
		fail(w, 400, "invalid file kind")
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if e != nil {
		fail(w, 413, "file exceeds 5 MB")
		return
	}
	mime, e := validateUpload(b, kind)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "upload unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var status string
	if e = tx.QueryRow(r.Context(), "SELECT status FROM registrations WHERE id=$1 FOR UPDATE", rid).Scan(&status); e != nil {
		fail(w, 404, "registration not found")
		return
	}
	if status != "awaiting_payment" && status != "correction_requested" {
		fail(w, 409, "uploads are closed while review is pending or complete")
		return
	}
	var count int
	if e = tx.QueryRow(r.Context(), "SELECT count(*) FROM files WHERE registration_id=$1 AND kind<>'pass'", rid).Scan(&count); e != nil || count >= 30 {
		fail(w, 400, "upload limit reached; contact organisers")
		return
	}
	fid := id()
	if e = a.Storage.Put(r.Context(), fid, b, mime); e != nil {
		fail(w, 503, "upload failed; please retry")
		return
	}
	_, e = tx.Exec(r.Context(), "INSERT INTO files(id,registration_id,kind,object_key,mime,size) VALUES($1,$2,$3,$1,$4,$5)", fid, rid, kind, mime, len(b))
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "upload could not be saved; retry")
		return
	}
	respond(w, 201, map[string]string{"id": fid})
}
func (a *App) downloadFile(w http.ResponseWriter, r *http.Request, rid, fid string) {
	var key, mime string
	if e := a.DB.QueryRow(r.Context(), "SELECT object_key,mime FROM files WHERE id=$1 AND registration_id=$2 AND kind<>'pass'", fid, rid).Scan(&key, &mime); e != nil {
		fail(w, 404, "file not found")
		return
	}
	a.serveObject(w, r, key, mime)
}
func (a *App) serveObject(w http.ResponseWriter, r *http.Request, key, mime string) {
	u, e := a.Storage.URL(r.Context(), key)
	if e != nil {
		fail(w, 503, "download unavailable")
		return
	}
	if u != "" {
		http.Redirect(w, r, u, http.StatusSeeOther)
		return
	}
	b, e := a.Storage.Get(r.Context(), key)
	if e != nil {
		fail(w, 503, "download unavailable")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="bioconnect-file"`)
	w.Header().Set("Content-Type", mime)
	w.Write(b)
}
