package app

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "send application/json")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, "invalid request fields or JSON")
		return false
	}
	if e := d.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		fail(w, 400, "send a single JSON object")
		return false
	}
	return true
}
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	static, _ := fs.Sub(resources, "web")
	fileSrv := http.StripPrefix("/static/", http.FileServer(http.FS(static)))
	m.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CloudFront does not cache (dynamic app), so let the browser hold static assets.
		if strings.HasSuffix(r.URL.Path, ".woff2") || strings.HasSuffix(r.URL.Path, ".png") {
			w.Header().Set("Cache-Control", "public, max-age=2592000")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=300")
		}
		fileSrv.ServeHTTP(w, r)
	}))
	// "/{$}" matches only the bare root; a catch-all "GET /" would conflict with the
	// method-less "/api/v1/..." subtree patterns under Go's ServeMux precedence rules.
	for _, path := range []string{"/{$}", "/delegates", "/exhibitors", "/recover", "/manage/{id}", "/admin"} {
		m.HandleFunc("GET "+path, a.page)
	}
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if e := a.DB.Ping(r.Context()); e != nil {
			fail(w, 503, "database unavailable")
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		c, e := a.categories(r.Context())
		if e != nil {
			fail(w, 503, "fees unavailable")
			return
		}
		respond(w, 200, map[string]any{"categories": c, "registration_enabled": a.Config.RegistrationEnabled, "server_time": a.Now().In(india), "cutoff": cutoff, "sbi_url": a.Config.SBIURL})
	})
	m.HandleFunc("POST /api/v1/registrations", func(w http.ResponseWriter, r *http.Request) {
		if a.limited(r.Context(), "registration:"+a.clientPeer(r), 20, time.Hour) {
			fail(w, 429, "too many submissions; try again later")
			return
		}
		var in RegistrationInput
		if !decode(w, r, &in) {
			return
		}
		rid, t, e := a.Create(r.Context(), in, r.Header.Get("Idempotency-Key"))
		if e != nil {
			fail(w, 400, publicError(e))
			return
		}
		out := map[string]any{"id": rid, "management_token": t, "replayed": t == ""}
		if t != "" {
			out["manage_url"] = a.Config.BaseURL + "/manage/" + rid + "#" + t
		}
		respond(w, 201, out)
	})
	m.HandleFunc("POST /api/v1/recovery", a.recover)
	m.HandleFunc("POST /api/v1/recovery/exchange", a.exchange)
	m.HandleFunc("POST /api/v1/auth/login", a.login)
	m.HandleFunc("GET /passes/{token}", a.passDownload)
	m.HandleFunc("GET /passes/pack/{token}", a.packDownload)
	m.HandleFunc("GET /api/v1/webhooks/meta", a.metaVerify)
	m.HandleFunc("POST /api/v1/webhooks/meta", a.metaWebhook)
	m.HandleFunc("POST /api/v1/webhooks/postmark", a.postmarkWebhook)
	m.HandleFunc("/api/v1/registrations/{id}", a.registrationAPI)
	m.HandleFunc("POST /api/v1/registrations/{id}/payments", a.registrationAPI)
	m.HandleFunc("POST /api/v1/registrations/{id}/files", a.registrationAPI)
	m.HandleFunc("GET /api/v1/registrations/{id}/files/{file}", a.registrationAPI)
	m.HandleFunc("/api/v1/admin/", a.adminAPI)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if a.Config.Production {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if r.Method != "GET" && r.Method != "HEAD" && strings.HasPrefix(r.URL.Path, "/api/") && !strings.Contains(r.URL.Path, "/webhooks/") {
			if origin := r.Header.Get("Origin"); origin != "" && origin != a.Config.BaseURL {
				fail(w, 403, "cross-origin request rejected")
				return
			}
		}
		defer func() {
			if recover() != nil {
				slog.Error("request panicked", "method", r.Method)
				fail(w, 500, "request failed; please retry")
			}
		}()
		m.ServeHTTP(w, r)
	})
}
func publicError(e error) string {
	if errors.Is(e, ErrConflict) {
		return ErrConflict.Error()
	}
	s := e.Error()
	if strings.Contains(s, "SQLSTATE") || strings.Contains(s, "conn") || strings.Contains(s, "timeout") {
		return "could not save this action; check the latest status and retry"
	}
	return s
}
func (a *App) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/delegates", 303)
		return
	}
	b, e := resources.ReadFile("web/page.html")
	if e != nil {
		fail(w, 500, "page unavailable")
		return
	}
	t, e := template.New("page").Parse(string(b))
	if e != nil {
		fail(w, 500, "page unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, map[string]any{"Enabled": a.Config.RegistrationEnabled, "Path": r.URL.Path})
}
func (a *App) registrationAPI(w http.ResponseWriter, r *http.Request) {
	rid := r.PathValue("id")
	if !a.manage(r, rid) {
		fail(w, 401, "use your private management link or recover by email")
		return
	}
	switch {
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/payments"):
		var in PaymentInput
		if !decode(w, r, &in) {
			return
		}
		if e := a.SubmitPayment(r.Context(), rid, in); e != nil {
			fail(w, 409, publicError(e))
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/files"):
		a.upload(w, r, rid)
	case r.Method == "GET" && r.PathValue("file") != "":
		a.downloadFile(w, r, rid, r.PathValue("file"))
	case r.Method == "GET":
		a.details(w, r, rid, false)
	default:
		fail(w, 405, "method not allowed")
	}
}
func (a *App) details(w http.ResponseWriter, r *http.Request, rid string, staff bool) {
	reg, e := a.registration(r.Context(), rid)
	if e != nil {
		fail(w, 404, "registration not found")
		return
	}
	out := map[string]any{"registration": reg, "sbi_url": a.Config.SBIURL, "reply_to": "bioconnect@bio360.in"}
	queries := map[string]string{"payments": `SELECT id,bank_reference,payment_date,amount_paise,receipt_id,created_at,verified_at,verified_date,verified_amount_paise,verified_reference FROM payment_submissions WHERE registration_id=$1 ORDER BY created_at DESC,id DESC`, "files": `SELECT id,kind,mime,size,created_at FROM files WHERE registration_id=$1 AND kind<>'pass' ORDER BY created_at DESC`}
	if staff {
		queries["passes"] = `SELECT p.id,p.number,p.version,p.attendee_id,p.created_at,p.revoked_at FROM passes p WHERE p.registration_id=$1 ORDER BY p.created_at`
		queries["deliveries"] = `SELECT id,pass_id,purpose,channel,recipient,status,attempts,provider_id,error_code,created_at,updated_at FROM delivery_jobs WHERE registration_id=$1 ORDER BY created_at DESC`
		queries["audit"] = `SELECT a.id,s.email AS staff_email,a.action,a.detail,a.created_at FROM audit_events a LEFT JOIN staff s ON s.id=a.staff_id WHERE a.registration_id=$1 ORDER BY a.created_at DESC`
	}
	for key, q := range queries {
		v, e := a.queryMaps(r, q, rid)
		if e != nil {
			fail(w, 503, "details unavailable")
			return
		}
		out[key] = v
	}
	cats, e := a.categories(r.Context())
	if e != nil {
		fail(w, 503, "fees unavailable")
		return
	}
	for _, c := range cats {
		if c.ID == reg.CategoryID {
			out["category"] = c
		}
	}
	respond(w, 200, out)
}
func (a *App) queryMaps(r *http.Request, q string, args ...any) ([]map[string]any, error) {
	rows, e := a.DB.Query(r.Context(), q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	fields := rows.FieldDescriptions()
	for rows.Next() {
		vals, e := rows.Values()
		if e != nil {
			return nil, e
		}
		m := map[string]any{}
		for i, f := range fields {
			m[f.Name] = vals[i]
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func filter(r *http.Request) (string, []any, error) {
	q := r.URL.Query()
	if len(q.Get("q")) > 200 {
		return "", nil, errors.New("search is too long")
	}
	// $6 carries the search with its separators stripped, so a reference or a
	// pass number is found however it is retyped. Empty means the term held
	// nothing that could be either, and the clause is skipped rather than
	// matching every registration.
	args := []any{"%" + q.Get("q") + "%", q.Get("category"), q.Get("status"), nil, nil, normalizePassNumber(q.Get("q"))}
	for i, k := range []string{"from", "to"} {
		if q.Get(k) != "" {
			t, e := time.ParseInLocation("2006-01-02", q.Get(k), india)
			if e != nil {
				return "", nil, errors.New("dates must use YYYY-MM-DD")
			}
			if k == "to" {
				t = t.AddDate(0, 0, 1)
			}
			args[3+i] = t
		}
	}
	return ` WHERE (r.reference ILIKE $1 OR r.institution ILIKE $1 OR r.email ILIKE $1 OR r.contact_name ILIKE $1 OR EXISTS(SELECT 1 FROM attendees a WHERE a.registration_id=r.id AND (a.name ILIKE $1 OR a.email ILIKE $1)) OR ($6<>'' AND (replace(r.reference,'-','') ILIKE '%'||$6::text||'%' OR EXISTS(SELECT 1 FROM passes pn WHERE pn.registration_id=r.id AND replace(pn.number,'-','') ILIKE '%'||$6::text||'%')))) AND ($2='' OR r.category_id=$2) AND ($3='' OR r.status=$3) AND ($4::timestamptz IS NULL OR r.created_at >= $4) AND ($5::timestamptz IS NULL OR r.created_at < $5)`, args, nil
}
func (a *App) adminAPI(w http.ResponseWriter, r *http.Request) {
	p, e := a.staff(r)
	if e != nil {
		fail(w, 401, e.Error())
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/")
	if path == "me" && r.Method == "GET" {
		respond(w, 200, map[string]string{"id": p.ID, "role": p.Role})
		return
	}
	if path == "logout" && r.Method == "POST" {
		c, _ := r.Cookie("bc_session")
		_, e = a.DB.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash(c.Value))
		if e != nil {
			fail(w, 503, "sign out unavailable")
			return
		}
		for _, name := range []string{"bc_session", "bc_csrf"} {
			http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Secure: a.Config.Production, HttpOnly: name == "bc_session", SameSite: http.SameSiteStrictMode})
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if path == "staff" && (r.Method == "GET" || r.Method == "POST") {
		a.staffAccounts(w, r, p)
		return
	}
	if p.Role != "reviewer" {
		fail(w, 403, "registration reviewer permission required")
		return
	}
	switch {
	case path == "registrations" && r.Method == "GET":
		where, args, e := filter(r)
		if e != nil {
			fail(w, 400, e.Error())
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		if page > 100000 {
			fail(w, 400, "page out of range")
			return
		}
		var count int
		if e = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM registrations r"+where, args...).Scan(&count); e != nil {
			fail(w, 503, "listing unavailable")
			return
		}
		args = append(args, (page-1)*25)
		items, e := a.queryMaps(r, "SELECT r.id,r.reference,r.institution,r.contact_name,r.email,r.category_id,r.status,r.quoted_paise,r.created_at FROM registrations r"+where+" ORDER BY r.created_at DESC,r.id LIMIT 25 OFFSET $7", args...)
		if e != nil {
			fail(w, 503, "listing unavailable")
			return
		}
		respond(w, 200, map[string]any{"items": items, "page": page, "total": count, "page_size": 25})
	case path == "export" && r.Method == "GET":
		a.export(w, r, p)
	case path == "categories" && r.Method == "POST":
		var in struct {
			ID   string `json:"id"`
			Open bool   `json:"open"`
		}
		if !decode(w, r, &in) {
			return
		}
		tx, e := a.DB.Begin(r.Context())
		if e != nil {
			fail(w, 503, "category update unavailable")
			return
		}
		defer tx.Rollback(r.Context())
		tag, e := tx.Exec(r.Context(), "UPDATE categories SET open=$2 WHERE id=$1", in.ID, in.Open)
		if e == nil && tag.RowsAffected() != 1 {
			e = errors.New("unknown category")
		}
		if e == nil {
			e = audit(r.Context(), tx, p.ID, "", "category_updated", in.ID+" open="+strconv.FormatBool(in.Open))
		}
		if e == nil {
			e = tx.Commit(r.Context())
		}
		if e != nil {
			fail(w, 400, "could not update category")
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
	case path == "bulk-send" && r.Method == "POST":
		var in struct {
			IDs     []string `json:"ids"`
			Channel string   `json:"channel"`
		}
		if !decode(w, r, &in) {
			return
		}
		if len(in.IDs) < 1 || len(in.IDs) > 100 {
			fail(w, 400, "select 1-100 registrations")
			return
		}
		result := map[string]string{}
		for _, rid := range in.IDs {
			e := a.Review(r.Context(), rid, p.ID, ReviewInput{Action: "send", Channel: in.Channel})
			if e != nil {
				result[rid] = publicError(e)
			} else {
				result[rid] = "queued"
			}
		}
		respond(w, 200, result)
	case path == "retry" && r.Method == "POST":
		a.retryJob(w, r, p)
	case strings.HasPrefix(path, "registrations/"):
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			fail(w, 404, "not found")
			return
		}
		rid := parts[1]
		if len(parts) == 2 && r.Method == "GET" {
			a.details(w, r, rid, true)
			return
		}
		if len(parts) == 3 && parts[2] == "review" && r.Method == "POST" {
			var in ReviewInput
			if !decode(w, r, &in) {
				return
			}
			if e = a.Review(r.Context(), rid, p.ID, in); e != nil {
				fail(w, 409, publicError(e))
				return
			}
			respond(w, 200, map[string]bool{"ok": true})
			return
		}
		if len(parts) == 4 && parts[2] == "files" && r.Method == "GET" {
			a.downloadFile(w, r, rid, parts[3])
			return
		}
		fail(w, 404, "not found")
	default:
		fail(w, 404, "not found")
	}
}
func (a *App) retryJob(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		ID               string `json:"id"`
		ConfirmUncertain bool   `json:"confirm_uncertain"`
		Note             string `json:"note"`
	}
	if !decode(w, r, &in) {
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "retry unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var rid, status string
	e = tx.QueryRow(r.Context(), "SELECT registration_id,status FROM delivery_jobs WHERE id=$1 FOR UPDATE", in.ID).Scan(&rid, &status)
	if e != nil {
		fail(w, 404, "delivery not found")
		return
	}
	if status != "failed" && status != "uncertain" {
		fail(w, 409, "only failed or uncertain deliveries can be retried")
		return
	}
	if status == "uncertain" && (!in.ConfirmUncertain || strings.TrimSpace(in.Note) == "") {
		fail(w, 400, "check the provider first, confirm duplicate-send risk and record a note")
		return
	}
	// Preserve the previous attempt and its provider ID for late webhooks.
	var j job
	e = tx.QueryRow(r.Context(), "SELECT COALESCE(pass_id,''),purpose,channel,recipient,payload_cipher FROM delivery_jobs WHERE id=$1", in.ID).Scan(&j.PassID, &j.Purpose, &j.Channel, &j.Recipient, &j.Payload)
	if e == nil {
		var plain string
		plain, e = a.unseal(j.Payload)
		if e == nil {
			e = a.queue(r.Context(), tx, rid, j.PassID, j.Purpose, j.Channel, j.Recipient, plain, "retry:"+in.ID)
		}
	}
	if e == nil {
		e = audit(r.Context(), tx, p.ID, rid, "delivery_retry", in.ID+" "+in.Note)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "retry unavailable")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
