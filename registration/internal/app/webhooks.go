package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (a *App) postmarkWebhook(w http.ResponseWriter, r *http.Request) {
	u, p, ok := r.BasicAuth()
	if !ok || a.Config.WebhookPassword == "" || subtle.ConstantTimeCompare([]byte(u), []byte(a.Config.WebhookUser)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(a.Config.WebhookPassword)) != 1 {
		fail(w, 401, "unauthorized")
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		fail(w, 413, "payload too large")
		return
	}
	var v struct {
		RecordType, MessageID string
		Metadata              map[string]string
	}
	if json.Unmarshal(b, &v) != nil {
		fail(w, 400, "invalid event")
		return
	}
	if v.Metadata["application"] != "bioconnect4" {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	status := ""
	switch v.RecordType {
	case "Delivery":
		status = "delivered"
	case "Bounce":
		status = "failed"
	}
	if status != "" && v.MessageID != "" {
		if !a.recordWebhook(w, r, "email", v.MessageID, status, hash("email:"+string(b))) {
			return
		}
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (a *App) metaVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if a.Config.MetaVerifyToken == "" || q.Get("hub.mode") != "subscribe" || subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(a.Config.MetaVerifyToken)) != 1 {
		fail(w, 403, "verification failed")
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(q.Get("hub.challenge")))
}
func (a *App) metaWebhook(w http.ResponseWriter, r *http.Request) {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		fail(w, 413, "payload too large")
		return
	}
	mac := hmac.New(sha256.New, []byte(a.Config.MetaAppSecret))
	mac.Write(b)
	provided, e := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256="))
	if a.Config.MetaAppSecret == "" || e != nil || !hmac.Equal(provided, mac.Sum(nil)) {
		fail(w, 401, "invalid signature")
		return
	}
	var v struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						PhoneID string `json:"phone_number_id"`
					} `json:"metadata"`
					Statuses []struct {
						ID        string `json:"id"`
						Status    string `json:"status"`
						Timestamp string `json:"timestamp"`
					} `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(b, &v) != nil {
		fail(w, 400, "invalid event")
		return
	}
	for _, entry := range v.Entry {
		for _, c := range entry.Changes {
			if c.Value.Metadata.PhoneID != a.Config.MetaPhoneID {
				continue
			}
			for _, s := range c.Value.Statuses {
				status := ""
				switch s.Status {
				case "delivered", "read":
					status = "delivered"
				case "failed":
					status = "failed"
				case "sent":
					status = "accepted"
				}
				if status != "" && s.ID != "" {
					if !a.recordWebhook(w, r, "whatsapp", s.ID, status, hash("whatsapp:"+s.ID+":"+s.Status+":"+s.Timestamp)) {
						return
					}
				}
			}
		}
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (a *App) recordWebhook(w http.ResponseWriter, r *http.Request, channel, pid, status, event string) bool {
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "event storage unavailable")
		return false
	}
	defer tx.Rollback(r.Context())
	_, e = tx.Exec(r.Context(), "INSERT INTO webhook_events(event_hash,channel,provider_id,status) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", event, channel, pid, status)
	if e == nil {
		_, e = tx.Exec(r.Context(), `UPDATE delivery_jobs SET status=CASE WHEN status='delivered' OR $3='delivered' THEN 'delivered' WHEN status='failed' OR $3='failed' THEN 'failed' ELSE $3 END,updated_at=now() WHERE channel=$1 AND provider_id=$2 AND status<>'cancelled'`, channel, pid, status)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "event storage unavailable")
		return false
	}
	return true
}
