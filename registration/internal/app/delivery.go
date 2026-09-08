package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type job struct {
	ID, RegistrationID, PassID, Purpose, Channel, Recipient, Payload string
	Attempts                                                         int
}
type sendResult struct {
	ID, Status, Code string
	Retry            bool
}

func (a *App) Worker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e := a.WorkOnce(ctx); e != nil {
				slog.Error("delivery worker cycle failed", "error_type", fmt.Sprintf("%T", e))
			}
		}
	}
}
func (a *App) WorkOnce(ctx context.Context) error {
	// A crashed send may already have reached the provider. Never automatically resend it.
	if _, e := a.DB.Exec(ctx, "UPDATE delivery_jobs SET status='uncertain',error_code='worker_lease_expired',updated_at=now() WHERE status='sending' AND claimed_at<now()-interval '5 minutes'"); e != nil {
		return e
	}
	var j job
	e := a.DB.QueryRow(ctx, `UPDATE delivery_jobs SET status='sending',attempts=attempts+1,claimed_at=now(),updated_at=now() WHERE id=(SELECT id FROM delivery_jobs WHERE status='queued' AND available_at<=now() ORDER BY available_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,registration_id,COALESCE(pass_id,''),purpose,channel,recipient,payload_cipher,attempts`).Scan(&j.ID, &j.RegistrationID, &j.PassID, &j.Purpose, &j.Channel, &j.Recipient, &j.Payload, &j.Attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	// Serializes cancellation/reissue with provider calls. The lock is bounded by HTTP timeout.
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status string
	if e = tx.QueryRow(ctx, "SELECT status FROM registrations WHERE id=$1 FOR NO KEY UPDATE", j.RegistrationID).Scan(&status); e != nil {
		return e
	}
	if j.Purpose == "pass" || j.Purpose == "pack" {
		valid := status == "approved"
		if valid && j.PassID != "" {
			e = tx.QueryRow(ctx, "SELECT revoked_at IS NULL FROM passes WHERE id=$1", j.PassID).Scan(&valid)
			if e != nil {
				return e
			}
		}
		if !valid {
			_, e = tx.Exec(ctx, "UPDATE delivery_jobs SET status='cancelled',updated_at=now() WHERE id=$1", j.ID)
			if e != nil {
				return e
			}
			return tx.Commit(ctx)
		}
	}
	// PDF generation uses a separate transaction; no registration row lock is taken there.
	result := a.send(ctx, j)
	next := a.Now()
	if result.Retry && j.Attempts < 5 {
		result.Status = "queued"
		next = next.Add(time.Duration(1<<j.Attempts) * time.Minute)
	}
	_, e = tx.Exec(ctx, `UPDATE delivery_jobs SET status=$2,provider_id=NULLIF($3,''),error_code=$4,available_at=$5,updated_at=now() WHERE id=$1`, j.ID, result.Status, result.ID, result.Code, next)
	if e != nil {
		return e
	}
	if result.ID != "" {
		_, e = tx.Exec(ctx, `UPDATE delivery_jobs SET status=CASE WHEN EXISTS(SELECT 1 FROM webhook_events WHERE channel=$2 AND provider_id=$3 AND status='delivered') THEN 'delivered' WHEN EXISTS(SELECT 1 FROM webhook_events WHERE channel=$2 AND provider_id=$3 AND status='failed') THEN 'failed' ELSE status END WHERE id=$1`, j.ID, j.Channel, result.ID)
		if e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	if result.Status == "failed" || result.Status == "uncertain" {
		slog.Error("delivery needs review", "job_id", j.ID, "status", result.Status, "code", result.Code)
	}
	return nil
}
func (a *App) send(ctx context.Context, j job) sendResult {
	if !a.Config.LiveDelivery {
		return sendResult{ID: "fake-" + j.ID, Status: "delivered", Code: "fake_provider"}
	}
	link, e := a.unseal(j.Payload)
	if e != nil {
		return sendResult{Status: "failed", Code: "payload_decryption"}
	}
	var attachments []map[string]string
	var number, waDocID string
	if j.Purpose == "pass" {
		var cipher string
		if e = a.DB.QueryRow(ctx, "SELECT download_cipher,number FROM passes WHERE id=$1 AND revoked_at IS NULL", j.PassID).Scan(&cipher, &number); e != nil {
			return sendResult{Status: "failed", Code: "pass_unavailable"}
		}
		token, e := a.unseal(cipher)
		if e != nil {
			return sendResult{Status: "failed", Code: "token_decryption"}
		}
		link = passLink(a.Config.BaseURL, token)
		// Both channels carry the PDF itself: an email attachment, or a WhatsApp
		// document header uploaded to the provider's media store. The link stays
		// in the message body as the fallback.
		if j.Channel == "email" || j.Channel == "whatsapp" {
			b, e := a.passPDF(ctx, j.PassID)
			if e != nil {
				return sendResult{Status: "failed", Code: "pdf_unavailable", Retry: true}
			}
			if j.Channel == "email" {
				attachments = append(attachments, map[string]string{"Name": "Bio-Connect-4.0-pass.pdf", "Content": base64.StdEncoding.EncodeToString(b), "ContentType": "application/pdf"})
			} else {
				waDocID, e = uploadWhatsAppMedia(ctx, a.Config, "Bio-Connect-4.0-pass.pdf", b)
				if e != nil {
					return sendResult{Status: "failed", Code: "wa_media_upload", Retry: true}
				}
			}
		}
	}
	if j.Purpose == "pack" {
		var cipher string
		if e = a.DB.QueryRow(ctx, "SELECT contact_pack_cipher FROM registrations WHERE id=$1", j.RegistrationID).Scan(&cipher); e != nil {
			return sendResult{Status: "failed", Code: "pack_unavailable"}
		}
		token, e := a.unseal(cipher)
		if e != nil {
			return sendResult{Status: "failed", Code: "token_decryption"}
		}
		link = a.Config.BaseURL + "/passes/pack/" + token
		// Named by pass number so a contact holding a team's passes can match
		// each file to the person whose pass number is printed on it.
		rows, e := a.DB.Query(ctx, "SELECT id,number FROM passes WHERE registration_id=$1 AND revoked_at IS NULL ORDER BY number", j.RegistrationID)
		if e != nil {
			return sendResult{Status: "failed", Code: "pack_unavailable", Retry: true}
		}
		type pass struct{ id, number string }
		var all []pass
		for rows.Next() {
			var p pass
			if e = rows.Scan(&p.id, &p.number); e != nil {
				break
			}
			all = append(all, p)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return sendResult{Status: "failed", Code: "pack_unavailable", Retry: true}
		}
		for _, p := range all {
			b, e := a.passPDF(ctx, p.id)
			if e != nil {
				return sendResult{Status: "failed", Code: "pdf_unavailable", Retry: true}
			}
			attachments = append(attachments, map[string]string{"Name": "Bio-Connect-4.0-pass-" + p.number + ".pdf", "Content": base64.StdEncoding.EncodeToString(b), "ContentType": "application/pdf"})
		}
	}
	if j.Channel == "email" {
		subject := "Bio Connect 4.0 - your pass"
		heading := "Your pass is ready"
		cta := "View your pass"
		intro := "Your Bio Connect 4.0 pass is attached to this email. The event takes place on 8-9 October 2026 at Hyatt Regency Trivandrum. You can also open it any time from the link below."
		switch j.Purpose {
		case "pack":
			subject = "Bio Connect 4.0 - your exhibitor pass pack"
			heading = "Your exhibitor pass pack"
			cta = "Download all passes"
			intro = "The Bio Connect 4.0 passes for your team are attached. Stall allocation is confirmed separately by the organisers."
		case "registration":
			subject = "Bio Connect 4.0 - registration saved"
			heading = "Registration saved"
			cta = "Open my registration"
			intro = "Your registration is saved. Use this private link to see the payment instructions and submit your evidence.\n\nThis is not a payment approval or an admission pass - keep the link to yourself."
		case "recovery":
			subject = "Bio Connect 4.0 - recover your registration"
			heading = "Recover your registration"
			cta = "Recover my registration"
			intro = "Use the link below within 20 minutes to regain access to your registration. If you did not request this, you can ignore this email."
		}
		// The pass number is short enough to quote in a reply or over the phone,
		// so carry it in the message as well as on the printed pass.
		if j.Purpose == "pass" {
			intro += "\n\nPass number: " + number + " - quote this if you contact us about your pass."
		}
		payload := map[string]any{"From": a.Config.SenderName + " <" + a.Config.SenderAddress + ">", "To": j.Recipient, "ReplyTo": "bioconnect@bio360.in", "Subject": subject, "HtmlBody": emailHTML(a.Config.BaseURL, heading, intro, cta, link), "TextBody": intro + "\n\n" + link + "\n\nQuestions: bioconnect@bio360.in\nSent via Zinvos for Bio Connect 4.0.", "MessageStream": a.Config.PostmarkStream, "Metadata": map[string]string{"application": "bioconnect4", "delivery_id": j.ID}, "Attachments": attachments}
		return providerRequest(ctx, a.Config.PostmarkAPIBase+"/email", "X-Postmark-Server-Token", a.Config.PostmarkToken, payload, "email")
	}
	components := []any{map[string]any{"type": "body", "parameters": []any{map[string]string{"type": "text", "text": link}}}}
	if waDocID != "" {
		components = append([]any{map[string]any{"type": "header", "parameters": []any{map[string]any{"type": "document", "document": map[string]string{"id": waDocID, "filename": "Bio-Connect-4.0-pass.pdf"}}}}}, components...)
	}
	payload := map[string]any{"messaging_product": "whatsapp", "to": strings.TrimPrefix(j.Recipient, "+"), "type": "template", "biz_opaque_callback_data": j.ID, "template": map[string]any{"name": a.Config.MetaTemplate, "language": map[string]string{"code": a.Config.MetaLanguage}, "components": components}}
	return providerRequest(ctx, a.Config.MetaAPIBase+"/"+a.Config.MetaVersion+"/"+a.Config.MetaPhoneID+"/messages", "Authorization", "Bearer "+a.Config.MetaToken, payload, "whatsapp")
}

// uploadWhatsAppMedia stores a document in the provider's media store and returns
// its media id, for use as a template document header. The id is accepted for
// roughly 30 days, which comfortably covers delivery and any retries.
func uploadWhatsAppMedia(ctx context.Context, c Config, filename string, content []byte) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("messaging_product", "whatsapp")
	_ = w.WriteField("type", "application/pdf")
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	h.Set("Content-Type", "application/pdf")
	part, e := w.CreatePart(h)
	if e != nil {
		return "", e
	}
	if _, e = part.Write(content); e != nil {
		return "", e
	}
	if e = w.Close(); e != nil {
		return "", e
	}
	url := c.MetaAPIBase + "/" + c.MetaVersion + "/" + c.MetaPhoneID + "/media"
	req, e := http.NewRequestWithContext(ctx, "POST", url, &body)
	if e != nil {
		return "", e
	}
	req.Header.Set("Authorization", "Bearer "+c.MetaToken)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, e := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("whatsapp media upload: http %d", resp.StatusCode)
	}
	var v struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &v) != nil || v.ID == "" {
		return "", errors.New("whatsapp media upload: no id in response")
	}
	return v.ID, nil
}
func providerRequest(ctx context.Context, url, header, credential string, payload any, channel string) sendResult {
	b, e := json.Marshal(payload)
	if e != nil {
		return sendResult{Status: "failed", Code: "invalid_payload"}
	}
	req, e := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if e != nil {
		return sendResult{Status: "failed", Code: "invalid_endpoint"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(header, credential)
	client := http.Client{Timeout: 25 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return sendResult{Status: "uncertain", Code: "transport_error"}
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if e != nil {
		return sendResult{Status: "uncertain", Code: "response_read_error"}
	}
	if resp.StatusCode == 429 {
		return sendResult{Status: "failed", Code: "rate_limited", Retry: true}
	}
	if resp.StatusCode >= 500 {
		return sendResult{Status: "uncertain", Code: "provider_5xx"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sendResult{Status: "failed", Code: fmt.Sprintf("provider_http_%d", resp.StatusCode)}
	}
	var v struct {
		MessageID string
		ErrorCode int
		Messages  []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if json.Unmarshal(body, &v) != nil {
		return sendResult{Status: "uncertain", Code: "invalid_provider_response"}
	}
	if channel == "email" && v.ErrorCode != 0 {
		return sendResult{Status: "failed", Code: fmt.Sprintf("postmark_%d", v.ErrorCode)}
	}
	pid := v.MessageID
	if channel == "whatsapp" && len(v.Messages) > 0 {
		pid = v.Messages[0].ID
	}
	if pid == "" {
		return sendResult{Status: "uncertain", Code: "missing_provider_id"}
	}
	return sendResult{ID: pid, Status: "accepted"}
}
