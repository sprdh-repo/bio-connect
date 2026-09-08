package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL, BaseURL, Listen, StorageDir, S3Bucket, AWSRegion, TrustedProxyCIDR                              string
	RegistrationEnabled, LiveDelivery, Production                                                                bool
	EncryptionKey, SBIURL, PostmarkToken, SenderAddress, SenderName, PostmarkStream, PostmarkAPIBase             string
	MetaToken, MetaPhoneID, MetaAppSecret, MetaVerifyToken, MetaTemplate, MetaLanguage, MetaVersion, MetaAPIBase string
	WebhookUser, WebhookPassword                                                                                 string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func FromEnv() Config {
	return Config{
		TrustedProxyCIDR: os.Getenv("TRUSTED_PROXY_CIDR"), DatabaseURL: os.Getenv("DATABASE_URL"), BaseURL: env("BASE_URL", "http://localhost:8080"), Listen: env("LISTEN_ADDR", ":8080"),
		StorageDir: env("STORAGE_DIR", "./var/files"), S3Bucket: os.Getenv("S3_BUCKET"), AWSRegion: env("AWS_REGION", "ap-south-1"),
		RegistrationEnabled: os.Getenv("REGISTRATION_ENABLED") == "true", LiveDelivery: os.Getenv("LIVE_DELIVERY") == "true", Production: os.Getenv("APP_ENV") == "production",
		EncryptionKey: os.Getenv("ENCRYPTION_KEY"), SBIURL: os.Getenv("SBI_COLLECT_URL"), PostmarkToken: os.Getenv("POSTMARK_SERVER_TOKEN"),
		SenderAddress: os.Getenv("POSTMARK_FROM_ADDRESS"), SenderName: os.Getenv("POSTMARK_FROM_NAME"), PostmarkStream: env("POSTMARK_STREAM", "outbound"), PostmarkAPIBase: env("POSTMARK_API_BASE", "https://api.postmarkapp.com"),
		MetaToken: os.Getenv("META_ACCESS_TOKEN"), MetaPhoneID: os.Getenv("META_PHONE_NUMBER_ID"), MetaAppSecret: os.Getenv("META_APP_SECRET"), MetaVerifyToken: os.Getenv("META_VERIFY_TOKEN"), MetaTemplate: os.Getenv("META_TEMPLATE"), MetaLanguage: env("META_TEMPLATE_LANGUAGE", "en"), MetaVersion: os.Getenv("META_API_VERSION"), MetaAPIBase: env("META_API_BASE", "https://graph.facebook.com"),
		WebhookUser: os.Getenv("POSTMARK_WEBHOOK_USER"), WebhookPassword: os.Getenv("POSTMARK_WEBHOOK_PASSWORD"),
	}
}
func (c Config) Validate() error {
	key, e := base64.StdEncoding.DecodeString(c.EncryptionKey)
	if e != nil || len(key) != 32 {
		return errors.New("ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	u, e := url.Parse(c.BaseURL)
	if e != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.User != nil {
		return errors.New("BASE_URL must be an origin")
	}
	if c.Production && (u.Scheme != "https" || c.S3Bucket == "") {
		return errors.New("production requires HTTPS and private S3")
	}
	if !c.Production && c.LiveDelivery {
		return errors.New("live delivery requires production mode")
	}
	if c.SBIURL != "" {
		s, e := url.Parse(c.SBIURL)
		if e != nil || s.Scheme != "https" || s.Hostname() == "" || s.User != nil {
			return errors.New("SBI_COLLECT_URL must be HTTPS")
		}
	}
	if c.Production && c.RegistrationEnabled && (!c.LiveDelivery || c.SBIURL == "") {
		return errors.New("live registration requires SBI and delivery readiness")
	}
	if c.LiveDelivery {
		for _, v := range []string{c.PostmarkToken, c.SenderAddress, c.SenderName, c.MetaToken, c.MetaPhoneID, c.MetaAppSecret, c.MetaVerifyToken, c.MetaTemplate, c.MetaVersion, c.WebhookUser, c.WebhookPassword} {
			if strings.TrimSpace(v) == "" {
				return errors.New("live delivery requires existing Zinvos sender, provider and webhook settings")
			}
		}
	}
	return nil
}
func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func id() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (a *App) seal(s string) string {
	n := make([]byte, a.aead.NonceSize())
	if _, e := rand.Read(n); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(a.aead.Seal(n, n, []byte(s), nil))
}
func (a *App) unseal(s string) (string, error) {
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil || len(b) < a.aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	n := a.aead.NonceSize()
	v, e := a.aead.Open(nil, b[:n], b[n:], nil)
	return string(v), e
}
func newCipher(key string) (cipher.AEAD, error) {
	k, e := base64.StdEncoding.DecodeString(key)
	if e != nil {
		return nil, e
	}
	b, e := aes.NewCipher(k)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}

var india = func() *time.Location {
	v, e := time.LoadLocation("Asia/Kolkata")
	if e != nil {
		panic(e)
	}
	return v
}()
var cutoff = time.Date(2026, 10, 1, 0, 0, 0, 0, india)

func fee(c Category, t time.Time) int64 {
	if t.In(india).Before(cutoff) {
		return c.EarlyPaise
	}
	return c.RegularPaise
}
func paymentDate(s string, now time.Time) (time.Time, error) {
	t, e := time.ParseInLocation("2006-01-02", s, india)
	if e != nil || t.Year() != 2026 || t.After(now.In(india)) {
		return t, errors.New("use a valid 2026 payment date, no later than today")
	}
	return t, nil
}
func Open(ctx context.Context, c Config) (*App, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	db, e := pgxpool.New(ctx, c.DatabaseURL)
	if e != nil {
		return nil, e
	}
	if e = db.Ping(ctx); e != nil {
		db.Close()
		return nil, e
	}
	a := &App{DB: db, Config: c, Now: time.Now}
	a.aead, e = newCipher(c.EncryptionKey)
	if e != nil {
		return nil, e
	}
	a.Storage, e = newStorage(ctx, c)
	if e != nil {
		return nil, e
	}
	return a, nil
}
func money(p int64) string { return fmt.Sprintf("INR %d.%02d", p/100, p%100) }
