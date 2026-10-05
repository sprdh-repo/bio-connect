package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const mobileOTPLifetime = 10 * time.Minute
const mobileSessionLifetime = 30 * 24 * time.Hour

var admissionQRPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func mobileIdentity(channel, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if channel == "email" {
		identifier = strings.ToLower(identifier)
		if validEmail(identifier) {
			return identifier, nil
		}
	} else if channel == "whatsapp" {
		identifier = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(identifier)
		// Public registration accepts international numbers. Allow the common Indian local form too.
		if regexp.MustCompile(`^[6-9][0-9]{9}$`).MatchString(identifier) {
			identifier = "+91" + identifier
		}
		if validPhone(identifier, false) {
			return identifier, nil
		}
	}
	return "", errors.New("enter a valid registered email or mobile number with country code")
}

// Always scope access to attendee identity, never an exhibitor's registration contact.
const mobilePassScope = ` FROM passes p JOIN attendees a ON a.id=p.attendee_id JOIN registrations r ON r.id=p.registration_id JOIN categories c ON c.id=r.category_id
 WHERE p.revoked_at IS NULL AND a.removed_at IS NULL AND r.status='approved'
 AND (($1='email' AND lower(a.email)=$2) OR ($1='whatsapp' AND a.phone=$2)) AND ($3='' OR p.qr_id=$3)`

func (a *App) mobileChallenge(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Channel    string `json:"channel"`
		Identifier string `json:"identifier"`
		QRID       string `json:"qr_id"`
		Consent    bool   `json:"whatsapp_consent"`
	}
	if !decode(w, r, &in) {
		return
	}
	identifier, err := mobileIdentity(in.Channel, in.Identifier)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if in.QRID != "" && !admissionQRPattern.MatchString(in.QRID) {
		fail(w, 400, "scan a Bio Connect admission pass QR")
		return
	}
	if in.Channel == "whatsapp" && !in.Consent {
		fail(w, 400, "confirm that we may send this verification code on WhatsApp")
		return
	}
	if in.Channel == "whatsapp" && a.Config.LiveDelivery && a.Config.MetaOTPTemplate == "" {
		fail(w, 503, "WhatsApp verification is unavailable. Use email instead.")
		return
	}
	if a.limited(r.Context(), "mobile-otp-ip:"+a.clientPeer(r), 20, time.Hour) || a.limited(r.Context(), "mobile-otp-identity:"+hash(in.Channel+":"+identifier), 5, time.Hour) || a.limited(r.Context(), "mobile-otp-cooldown:"+hash(in.Channel+":"+identifier), 1, time.Minute) {
		fail(w, 429, "too many code requests; wait before trying again")
		return
	}
	challenge := randomToken()
	expires := a.Now().Add(mobileOTPLifetime)
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	// Serialize resends so only the newest challenge for an identity remains usable.
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "mobile-otp:"+in.Channel+":"+identifier); err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	if _, err = tx.Exec(r.Context(), "UPDATE mobile_pass_challenges SET used_at=$3 WHERE channel=$1 AND identifier=$2 AND used_at IS NULL", in.Channel, identifier, a.Now()); err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	var rid string
	err = tx.QueryRow(r.Context(), "SELECT r.id"+mobilePassScope+" ORDER BY p.created_at LIMIT 1", in.Channel, identifier, in.QRID).Scan(&rid)
	var delivery any
	codeHash := ""
	if err == nil {
		codeHash = hash(challenge + ":" + code)
		if err = a.queue(r.Context(), tx, rid, "", "pass_otp", in.Channel, identifier, code, "pass-otp:"+hash(challenge)); err == nil {
			var jid string
			err = tx.QueryRow(r.Context(), "SELECT id FROM delivery_jobs WHERE dedupe_key=$1", "pass-otp:"+hash(challenge)).Scan(&jid)
			delivery = jid
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO mobile_pass_challenges(token_hash,channel,identifier,qr_id,code_hash,expires_at,delivery_id) VALUES($1,$2,$3,$4,$5,$6,$7)", hash(challenge), in.Channel, identifier, in.QRID, codeHash, expires, delivery)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	respond(w, 202, map[string]any{"challenge": challenge, "expires_at": expires, "resend_after_seconds": 60, "message": "If an issued pass matches these details, a verification code will arrive shortly."})
}

func (a *App) mobileVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !admissionQRPattern.MatchString(in.Challenge) || len(in.Code) != 6 {
		fail(w, 401, "code is incorrect or expired; request a new code")
		return
	}
	if a.limited(r.Context(), "mobile-verify:"+a.clientPeer(r), 60, time.Hour) {
		fail(w, 429, "too many verification attempts; try again later")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var channel, identifier, qr, expected string
	err = tx.QueryRow(r.Context(), `UPDATE mobile_pass_challenges SET attempts=attempts+1 WHERE token_hash=$1 AND used_at IS NULL AND expires_at>$2 AND attempts<5 RETURNING channel,identifier,qr_id,code_hash`, hash(in.Challenge), a.Now()).Scan(&channel, &identifier, &qr, &expected)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 401, "code is incorrect or expired; request a new code")
		return
	}
	if err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	actual := hash(in.Challenge + ":" + in.Code)
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
		if err = tx.Commit(r.Context()); err != nil {
			fail(w, 503, "verification unavailable")
			return
		}
		fail(w, 401, "code is incorrect or expired; request a new code")
		return
	}
	token := randomToken()
	expires := a.Now().Add(mobileSessionLifetime)
	if _, err = tx.Exec(r.Context(), "UPDATE mobile_pass_challenges SET used_at=$2 WHERE token_hash=$1", hash(in.Challenge), a.Now()); err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO mobile_pass_sessions(token_hash,channel,identifier,qr_id,expires_at) VALUES($1,$2,$3,$4,$5)", hash(token), channel, identifier, qr, expires)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, 503, "verification unavailable")
		return
	}
	respond(w, 200, map[string]any{"token": token, "expires_at": expires})
}

type mobilePass struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Institution string `json:"institution"`
	Designation string `json:"designation"`
	Category    string `json:"category"`
	Number      string `json:"number"`
	QRID        string `json:"qr_id"`
	DownloadURL string `json:"download_url"`
}

func (a *App) mobilePasses(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || !admissionQRPattern.MatchString(token) {
		fail(w, 401, "verify your email or mobile again")
		return
	}
	var channel, identifier, qr string
	err := a.DB.QueryRow(r.Context(), "SELECT channel,identifier,qr_id FROM mobile_pass_sessions WHERE token_hash=$1 AND expires_at>$2", hash(token), a.Now()).Scan(&channel, &identifier, &qr)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 401, "verify your email or mobile again")
		return
	}
	if err != nil {
		fail(w, 503, "passes unavailable; retry shortly")
		return
	}
	if r.Method == http.MethodDelete {
		if _, err = a.DB.Exec(r.Context(), "DELETE FROM mobile_pass_sessions WHERE token_hash=$1", hash(token)); err != nil {
			fail(w, 503, "could not sign out; retry")
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	rows, err := a.DB.Query(r.Context(), "SELECT p.id,a.name,r.institution,a.designation,c.label,p.number,p.qr_id,p.download_cipher"+mobilePassScope+" ORDER BY p.created_at,p.id", channel, identifier, qr)
	if err != nil {
		fail(w, 503, "passes unavailable; retry shortly")
		return
	}
	defer rows.Close()
	passes := []mobilePass{}
	for rows.Next() {
		var p mobilePass
		var cipher string
		if err = rows.Scan(&p.ID, &p.Name, &p.Institution, &p.Designation, &p.Category, &p.Number, &p.QRID, &cipher); err != nil {
			fail(w, 503, "passes unavailable; retry shortly")
			return
		}
		download, err := a.unseal(cipher)
		if err != nil {
			fail(w, 503, "passes unavailable; retry shortly")
			return
		}
		p.DownloadURL = passLink(a.Config.BaseURL, download)
		passes = append(passes, p)
	}
	if rows.Err() != nil {
		fail(w, 503, "passes unavailable; retry shortly")
		return
	}
	respond(w, 200, map[string]any{"passes": passes, "checked_at": a.Now()})
}

func (a *App) sendPassOTP(ctx context.Context, j job, code string) sendResult {
	if j.Channel == "email" {
		text := "Your Bio Connect verification code is " + code + ". It expires in 10 minutes. Do not share it. If you did not request this code, ignore this email."
		payload := map[string]any{"From": a.Config.SenderName + " <" + a.Config.SenderAddress + ">", "To": j.Recipient, "Subject": "Bio Connect 4.0 - pass verification code", "TextBody": text, "HtmlBody": "<p>Your Bio Connect verification code is <strong>" + html.EscapeString(code) + "</strong>.</p><p>It expires in 10 minutes. Do not share it. If you did not request this code, ignore this email.</p>", "MessageStream": a.Config.PostmarkStream, "Metadata": map[string]string{"application": "bioconnect4", "delivery_id": j.ID}}
		return providerRequest(ctx, a.Config.PostmarkAPIBase+"/email", "X-Postmark-Server-Token", a.Config.PostmarkToken, payload, "email")
	}
	if a.Config.MetaOTPTemplate == "" {
		return sendResult{Status: "failed", Code: "otp_template_missing"}
	}
	param := []any{map[string]string{"type": "text", "text": code}}
	components := []any{map[string]any{"type": "body", "parameters": param}, map[string]any{"type": "button", "sub_type": "url", "index": "0", "parameters": param}}
	payload := map[string]any{"messaging_product": "whatsapp", "to": strings.TrimPrefix(j.Recipient, "+"), "type": "template", "biz_opaque_callback_data": j.ID, "template": map[string]any{"name": a.Config.MetaOTPTemplate, "language": map[string]string{"code": a.Config.MetaOTPLanguage}, "components": components}}
	return providerRequest(ctx, a.Config.MetaAPIBase+"/"+a.Config.MetaVersion+"/"+a.Config.MetaPhoneID+"/messages", "Authorization", "Bearer "+a.Config.MetaToken, payload, "whatsapp")
}
