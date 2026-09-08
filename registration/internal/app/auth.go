package app

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type principal struct{ ID, Role string }

func (a *App) limited(ctx context.Context, key string, limit int, window time.Duration) bool {
	var n int
	e := a.DB.QueryRow(ctx, `INSERT INTO rate_limits(key,attempts,expires_at) VALUES($1,1,$2) ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN rate_limits.expires_at<now() THEN 1 ELSE rate_limits.attempts+1 END,expires_at=CASE WHEN rate_limits.expires_at<now() THEN EXCLUDED.expires_at ELSE rate_limits.expires_at END RETURNING attempts`, hash(key), a.Now().Add(window)).Scan(&n)
	return e != nil || n > limit
}
func peer(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return r.RemoteAddr
	}
	return host
}
func (a *App) clientPeer(r *http.Request) string {
	original := peer(r)
	if a.Config.TrustedProxyCIDR != "" {
		_, network, e := net.ParseCIDR(a.Config.TrustedProxyCIDR)
		if e == nil && network.Contains(net.ParseIP(original)) {
			h := r.Header.Get("X-BioConnect-Client-IP")
			if i := strings.IndexByte(h, ','); i >= 0 {
				h = h[:i]
			}
			if ip := net.ParseIP(strings.TrimSpace(h)); ip != nil {
				return ip.String()
			}
		}
	}
	return original
}
func (a *App) staff(r *http.Request) (principal, error) {
	var p principal
	c, e := r.Cookie("bc_session")
	if e != nil {
		return p, errors.New("sign in required")
	}
	var csrf string
	e = a.DB.QueryRow(r.Context(), "SELECT s.id,s.role,t.csrf_hash FROM sessions t JOIN staff s ON s.id=t.staff_id WHERE t.token_hash=$1 AND t.expires_at>now() AND s.active", hash(c.Value)).Scan(&p.ID, &p.Role, &csrf)
	if e != nil {
		return p, errors.New("sign in required")
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(hash(r.Header.Get("X-CSRF-Token")))) != 1 {
			return p, errors.New("invalid CSRF token")
		}
	}
	return p, nil
}
func (a *App) manage(r *http.Request, rid string) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(token) != 43 {
		return false
	}
	var ok bool
	e := a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM registrations WHERE id=$1 AND management_hash=$2 AND management_expires>now())", rid, hash(token)).Scan(&ok)
	return e == nil && ok
}
func (a *App) AddStaff(ctx context.Context, email, password, role string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || len(password) < 14 || len(password) > 72 || (role != "manager" && role != "reviewer") {
		return "", errors.New("valid email, 14-72 byte password and manager/reviewer role required")
	}
	key, e := totp.Generate(totp.GenerateOpts{Issuer: "Bio Connect 4.0", AccountName: email})
	if e != nil {
		return "", e
	}
	pw, e := bcrypt.GenerateFromPassword([]byte(password), 12)
	if e != nil {
		return "", e
	}
	_, e = a.DB.Exec(ctx, "INSERT INTO staff(id,email,password_hash,totp_cipher,role) VALUES($1,$2,$3,$4,$5)", id(), email, string(pw), a.seal(key.Secret()), role)
	return key.URL(), e
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if a.limited(r.Context(), "login-ip:"+a.clientPeer(r), 50, 15*time.Minute) || a.limited(r.Context(), "login-email:"+in.Email, 8, 15*time.Minute) {
		fail(w, 429, "too many attempts; try again in 15 minutes")
		return
	}
	var sid, pw, secret string
	e := a.DB.QueryRow(r.Context(), "SELECT id,password_hash,totp_cipher FROM staff WHERE email=$1 AND active", in.Email).Scan(&sid, &pw, &secret)
	if e != nil {
		pw = "$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW"
	}
	passErr := bcrypt.CompareHashAndPassword([]byte(pw), []byte(in.Password))
	sec, de := a.unseal(secret)
	valid, te := totp.ValidateCustom(in.Code, sec, a.Now(), totp.ValidateOpts{Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if e != nil || passErr != nil || de != nil || te != nil || !valid {
		fail(w, 401, "email, password or authenticator code is incorrect")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "sign in unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	step := a.Now().Unix() / 30
	tag, e := tx.Exec(r.Context(), "UPDATE staff SET last_totp_step=$2 WHERE id=$1 AND last_totp_step<$2 AND active", sid, step)
	if e != nil || tag.RowsAffected() != 1 {
		fail(w, 401, "wait for a new authenticator code")
		return
	}
	token, csrf := randomToken(), randomToken()
	_, e = tx.Exec(r.Context(), "INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at) VALUES($1,$2,$3,$4)", hash(token), sid, hash(csrf), a.Now().Add(8*time.Hour))
	if e == nil {
		e = audit(r.Context(), tx, sid, "", "login", "")
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "sign in unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "bc_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Config.Production, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	http.SetCookie(w, &http.Cookie{Name: "bc_csrf", Value: csrf, Path: "/", Secure: a.Config.Production, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	respond(w, 200, map[string]string{"csrf": csrf})
}
func (a *App) recover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if a.limited(r.Context(), "recover-ip:"+a.clientPeer(r), 20, time.Hour) || a.limited(r.Context(), "recover-email:"+email, 3, time.Hour) {
		respond(w, 202, map[string]string{"message": "If registrations match, a recovery email will arrive shortly."})
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "recovery unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	rows, e := tx.Query(r.Context(), "SELECT id FROM registrations WHERE lower(email)=$1 ORDER BY created_at DESC LIMIT 20", email)
	if e != nil {
		fail(w, 503, "recovery unavailable")
		return
	}
	var ids []string
	for rows.Next() {
		var rid string
		if e = rows.Scan(&rid); e != nil {
			break
		}
		ids = append(ids, rid)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		fail(w, 503, "recovery unavailable")
		return
	}
	for _, rid := range ids {
		token := randomToken()
		_, e = tx.Exec(r.Context(), "INSERT INTO recovery_tokens(token_hash,registration_id,expires_at) VALUES($1,$2,$3)", hash(token), rid, a.Now().Add(20*time.Minute))
		if e == nil {
			e = a.queue(r.Context(), tx, rid, "", "recovery", "email", email, a.Config.BaseURL+"/recover#"+token, "recover:"+hash(token))
		}
		if e != nil {
			fail(w, 503, "recovery unavailable")
			return
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 503, "recovery unavailable")
		return
	}
	respond(w, 202, map[string]string{"message": "If registrations match, a recovery email will arrive shortly."})
}
func (a *App) exchange(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if a.limited(r.Context(), "exchange:"+a.clientPeer(r), 60, time.Hour) {
		fail(w, 429, "try again later")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "recovery unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var rid string
	e = tx.QueryRow(r.Context(), "UPDATE recovery_tokens SET used_at=now() WHERE token_hash=$1 AND used_at IS NULL AND expires_at>now() RETURNING registration_id", hash(in.Token)).Scan(&rid)
	if e != nil {
		fail(w, 401, "recovery link expired or already used; request a new email")
		return
	}
	token := randomToken()
	_, e = tx.Exec(r.Context(), "UPDATE registrations SET management_hash=$2,management_expires=$3 WHERE id=$1", rid, hash(token), a.Now().Add(30*24*time.Hour))
	if e == nil {
		e = audit(r.Context(), tx, "", rid, "management_access_recovered", "")
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "recovery unavailable")
		return
	}
	respond(w, 200, map[string]string{"id": rid, "management_token": token})
}
func (a *App) staffAccounts(w http.ResponseWriter, r *http.Request, p principal) {
	if p.Role != "manager" {
		fail(w, 403, "account manager permission required")
		return
	}
	if r.Method == "GET" {
		rows, e := a.DB.Query(r.Context(), "SELECT id,email,role,active FROM staff ORDER BY email")
		if e != nil {
			fail(w, 503, "accounts unavailable")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, email, role string
			var active bool
			if e = rows.Scan(&id, &email, &role, &active); e != nil {
				fail(w, 503, "accounts unavailable")
				return
			}
			out = append(out, map[string]any{"id": id, "email": email, "role": role, "active": active})
		}
		respond(w, 200, out)
		return
	}
	var in struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Active   bool   `json:"active"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		u, e := a.AddStaff(r.Context(), in.Email, in.Password, in.Role)
		if e != nil {
			fail(w, 400, "could not create staff account; check email, role and password length")
			return
		}
		_, e = a.DB.Exec(r.Context(), "INSERT INTO audit_events(staff_id,action,detail) VALUES($1,'staff_created',$2)", p.ID, in.Email)
		if e != nil {
			fail(w, 503, "account created but audit failed; contact operator")
			return
		}
		respond(w, 201, map[string]string{"totp_uri": u})
		return
	}
	if in.ID == p.ID {
		fail(w, 400, "another manager must change your account")
		return
	}
	if in.Role != "reviewer" && in.Role != "manager" {
		fail(w, 400, "invalid role")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "account update unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	tag, e := tx.Exec(r.Context(), "UPDATE staff SET role=$2,active=$3 WHERE id=$1", in.ID, in.Role, in.Active)
	if e == nil && tag.RowsAffected() != 1 {
		e = errors.New("unknown account")
	}
	if e == nil {
		_, e = tx.Exec(r.Context(), "DELETE FROM sessions WHERE staff_id=$1", in.ID)
	}
	if e == nil {
		e = audit(r.Context(), tx, p.ID, "", "staff_updated", fmt.Sprintf("%s role=%s active=%t", in.ID, in.Role, in.Active))
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 400, "could not update account")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
