package app

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/skip2/go-qrcode"
)

const opsSessionLifetime = 12 * time.Hour

var opsDays = []struct{ ID, Label string }{{"2026-10-08", "Day 1 · 08 October"}, {"2026-10-09", "Day 2 · 09 October"}}
var opsCodeCleaner = regexp.MustCompile(`[^A-Z0-9]`)

type opsPrincipal struct {
	Station, CSRF string
	// Kiosk sessions run unattended self-service and reach only the kiosk endpoints.
	Kiosk bool
}

type opsPerson struct {
	AttendeeID, RegistrationID, Reference, QRID, Name, Email, Phone, Designation, Institution, CategoryID, Category string
	RegistrationType                                                                                                string
	CheckedIn, CheckedOut                                                                                           bool
	CheckedInAt, CheckedInBy, CheckedOutAt, CheckedOutBy                                                            any
}

func (p opsPerson) json() map[string]any {
	return map[string]any{
		"attendeeId": p.AttendeeID, "registrationId": p.RegistrationID, "reference": p.Reference,
		"qrId": p.QRID, "name": p.Name, "email": p.Email, "phone": p.Phone,
		"designation": p.Designation, "institution": p.Institution, "categoryId": p.CategoryID,
		"category": p.Category, "checkedIn": p.CheckedIn, "checkedOut": p.CheckedOut,
		"registrationType": p.RegistrationType,
		"checkedInAt":      p.CheckedInAt, "checkedInBy": p.CheckedInBy,
		"checkedOutAt": p.CheckedOutAt, "checkedOutBy": p.CheckedOutBy,
	}
}

func opsDay(value string) (string, error) {
	for _, day := range opsDays {
		if value == day.ID {
			return value, nil
		}
	}
	return "", errors.New("choose an event day")
}

func opsToday(now time.Time) string {
	today := now.In(india).Format("2006-01-02")
	if today == opsDays[1].ID {
		return today
	}
	return opsDays[0].ID
}

func opsReference(value string) string {
	value = strings.TrimSpace(value)
	if u := strings.LastIndexAny(value, "/#"); u >= 0 {
		value = value[u+1:]
	}
	return opsCodeCleaner.ReplaceAllString(strings.ToUpper(value), "")
}

func (a *App) opsPage(w http.ResponseWriter, r *http.Request) {
	b, err := resources.ReadFile("web/ops.html")
	if err != nil {
		http.Error(w, "Ops portal unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *App) opsPrincipal(r *http.Request) (opsPrincipal, error) {
	var p opsPrincipal
	c, err := r.Cookie("bc_ops")
	if err != nil {
		return p, errors.New("start a shift")
	}
	var csrfHash string
	err = a.DB.QueryRow(r.Context(), `SELECT station,csrf_hash,kiosk FROM ops_sessions WHERE token_hash=$1 AND expires_at>now()`, hash(c.Value)).Scan(&p.Station, &csrfHash, &p.Kiosk)
	if err != nil {
		return p, errors.New("start a shift")
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && subtle.ConstantTimeCompare([]byte(csrfHash), []byte(hash(r.Header.Get("X-CSRF-Token")))) != 1 {
		return p, errors.New("request verification failed")
	}
	p.CSRF = csrfHash
	return p, nil
}

func (a *App) opsStart(w http.ResponseWriter, r *http.Request) {
	var in struct{ Station, Passcode string }
	if !decode(w, r, &in) {
		return
	}
	in.Station = strings.TrimSpace(in.Station)
	if len(in.Station) < 1 || len(in.Station) > 60 {
		fail(w, 400, "name this station")
		return
	}
	if !a.opsPasscodeValid(w, r, in.Passcode) {
		return
	}
	csrf, err := a.opsOpenSession(r.Context(), w, a.DB, in.Station, false)
	if err != nil {
		fail(w, 503, "shift could not start")
		return
	}
	respond(w, 200, map[string]any{"station": in.Station, "csrf": csrf, "today": opsToday(a.Now()), "days": opsDays})
}

// opsOpenSession stores a new station session and sets its cookies. The session
// token is HttpOnly; the CSRF value is readable so the page can recover it.
func (a *App) opsOpenSession(ctx context.Context, w http.ResponseWriter, db execer, station string, kiosk bool) (string, error) {
	token, csrf := randomToken(), randomToken()
	_, err := db.Exec(ctx, `INSERT INTO ops_sessions(token_hash,csrf_hash,station,expires_at,kiosk) VALUES($1,$2,$3,$4,$5)`, hash(token), hash(csrf), station, a.Now().Add(opsSessionLifetime), kiosk)
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: "bc_ops", Value: token, Path: "/", HttpOnly: true, Secure: a.Config.Production, SameSite: http.SameSiteStrictMode, MaxAge: int(opsSessionLifetime.Seconds())})
	http.SetCookie(w, &http.Cookie{Name: "bc_ops_csrf", Value: csrf, Path: "/", Secure: a.Config.Production, SameSite: http.SameSiteStrictMode, MaxAge: int(opsSessionLifetime.Seconds())})
	return csrf, nil
}

// opsPasscodeValid checks the shared ops passcode under the same per-client
// throttle as starting a shift.
func (a *App) opsPasscodeValid(w http.ResponseWriter, r *http.Request, passcode string) bool {
	if a.Config.OpsKey == "" {
		fail(w, 503, "ops passcode is not configured")
		return false
	}
	if a.limited(r.Context(), "ops-login:"+a.clientPeer(r), 30, 10*time.Minute) {
		fail(w, 429, "too many attempts; wait ten minutes")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(hash(passcode)), []byte(hash(a.Config.OpsKey))) != 1 {
		fail(w, 401, "passcode is incorrect")
		return false
	}
	return true
}

func (a *App) opsAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ops/")
	if path == "auth/start" && r.Method == http.MethodPost {
		a.opsStart(w, r)
		return
	}
	p, err := a.opsPrincipal(r)
	if err != nil {
		fail(w, 401, err.Error())
		return
	}
	if path == "me" && r.Method == http.MethodGet {
		csrf, _ := r.Cookie("bc_ops_csrf")
		value := ""
		if csrf != nil {
			value = csrf.Value
		}
		respond(w, 200, map[string]any{"station": p.Station, "csrf": value, "today": opsToday(a.Now()), "days": opsDays, "kiosk": p.Kiosk})
		return
	}
	if p.Kiosk {
		a.kioskAPI(w, r, p, path)
		return
	}
	switch {
	case path == "kiosk/start" && r.Method == http.MethodPost:
		a.kioskStart(w, r, p)
	case path == "auth/logout" && r.Method == http.MethodPost:
		c, _ := r.Cookie("bc_ops")
		if c != nil {
			_, _ = a.DB.Exec(r.Context(), "DELETE FROM ops_sessions WHERE token_hash=$1", hash(c.Value))
		}
		http.SetCookie(w, &http.Cookie{Name: "bc_ops", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.Config.Production, SameSite: http.SameSiteStrictMode})
		http.SetCookie(w, &http.Cookie{Name: "bc_ops_csrf", Path: "/", MaxAge: -1, Secure: a.Config.Production, SameSite: http.SameSiteStrictMode})
		respond(w, 200, map[string]bool{"ok": true})
	case path == "config" && r.Method == http.MethodGet:
		a.opsConfig(w, r, p)
	case path == "roster" && r.Method == http.MethodGet:
		a.opsRoster(w, r)
	case path == "lookup" && r.Method == http.MethodGet:
		a.opsLookup(w, r, p)
	case path == "check-in" && r.Method == http.MethodPost:
		a.opsCheckIn(w, r, p)
	case path == "check-in/undo" && r.Method == http.MethodPost:
		a.opsUndoCheckIn(w, r, p)
	case path == "check-out" && r.Method == http.MethodPost:
		a.opsCheckOut(w, r, p)
	case path == "check-out/undo" && r.Method == http.MethodPost:
		a.opsUndoCheckOut(w, r, p)
	case path == "badge/printed" && r.Method == http.MethodPost:
		a.opsBadgePrinted(w, r, p)
	case strings.HasPrefix(path, "qr/") && r.Method == http.MethodGet:
		a.opsQR(w, r, strings.TrimPrefix(path, "qr/"))
	case path == "spot-register" && r.Method == http.MethodPost:
		a.opsSpotRegister(w, r, p)
	case strings.HasPrefix(path, "passes/") && r.Method == http.MethodGet:
		a.opsPassDownload(w, r, p, strings.TrimPrefix(path, "passes/"))
	case path == "points" || strings.HasPrefix(path, "points/"):
		a.opsPoints(w, r, p, strings.TrimPrefix(path, "points"))
	case path == "reports" && r.Method == http.MethodGet:
		a.opsReports(w, r)
	case path == "reports/export" && r.Method == http.MethodGet:
		a.opsReportExport(w, r)
	case path == "activity" && r.Method == http.MethodGet:
		a.opsActivity(w, r)
	case path == "activity/export" && r.Method == http.MethodGet:
		a.opsActivityExport(w, r)
	default:
		fail(w, 404, "not found")
	}
}

func (a *App) opsConfig(w http.ResponseWriter, r *http.Request, p opsPrincipal) {
	cats, err := a.categories(r.Context())
	if err != nil {
		fail(w, 503, "configuration unavailable")
		return
	}
	respond(w, 200, map[string]any{"station": p.Station, "today": opsToday(a.Now()), "days": opsDays, "categories": cats, "coupons": couponList(), "badge": map[string]float64{"widthCm": 7.62, "heightCm": 5.08}})
}

const opsPersonSQL = `SELECT a.id,r.id,p.number,p.qr_id,a.name,a.email,a.phone,a.designation,r.institution,r.category_id,c.label,
	CASE WHEN r.free_link_id IS NOT NULL THEN 'free_link' WHEN r.complimentary THEN 'complimentary' ELSE 'paid' END,
 oa.attendee_id IS NOT NULL,oa.checked_out_at IS NOT NULL,oa.checked_in_at,oa.checked_in_by,oa.checked_out_at,oa.checked_out_by
 FROM attendees a JOIN registrations r ON r.id=a.registration_id JOIN categories c ON c.id=r.category_id
 JOIN passes p ON p.attendee_id=a.id AND p.revoked_at IS NULL
 LEFT JOIN ops_attendance oa ON oa.attendee_id=a.id AND oa.event_day=$1::date
 WHERE r.status='approved' AND a.removed_at IS NULL AND `

func scanOpsPerson(row pgx.Row) (opsPerson, error) {
	var p opsPerson
	err := row.Scan(&p.AttendeeID, &p.RegistrationID, &p.Reference, &p.QRID, &p.Name, &p.Email, &p.Phone, &p.Designation, &p.Institution, &p.CategoryID, &p.Category, &p.RegistrationType, &p.CheckedIn, &p.CheckedOut, &p.CheckedInAt, &p.CheckedInBy, &p.CheckedOutAt, &p.CheckedOutBy)
	return p, err
}

// opsScanCode reduces a scanned value to the pass QR token. Printed badges
// carry the public profile URL (see badgeURL); passes and the mobile app carry
// the bare token. Taking the trailing token rather than parsing the URL keeps
// keyboard-wedge scanners working when a keyboard layout mangles ":" or "/".
func opsScanCode(value string) string {
	value = strings.TrimSpace(value)
	path := value
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(path, "/")
	if n := len(path); n > 43 && admissionQRPattern.MatchString(path[n-43:]) {
		return path[n-43:]
	}
	return value
}

func (a *App) opsFind(ctx context.Context, value, day string) (opsPerson, error) {
	clean := opsReference(value)
	return scanOpsPerson(a.DB.QueryRow(ctx, opsPersonSQL+`(p.qr_id=$2 OR replace(p.number,'-','')=$3)`, day, opsScanCode(value), clean))
}

func (a *App) opsLookup(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	day, err := opsDay(r.URL.Query().Get("day"))
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	p, err := a.opsFind(r.Context(), r.URL.Query().Get("code"), day)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			fail(w, 503, "lookup unavailable")
			return
		}
		if err := a.opsLog(r.Context(), a.DB, "", day, "badge_lookup_denied", principal.Station, opsScanCode(r.URL.Query().Get("code"))+" | No approved pass matches that code."); err != nil {
			fail(w, 503, "scan could not be recorded")
			return
		}
		fail(w, 404, "no approved pass matches that code")
		return
	}
	if err := a.opsLog(r.Context(), a.DB, p.AttendeeID, day, "badge_lookup", principal.Station, ""); err != nil {
		fail(w, 503, "scan could not be recorded")
		return
	}
	respond(w, 200, map[string]any{"person": p.json(), "qrUrl": "/api/v1/ops/qr/" + p.QRID})
}

func (a *App) opsRoster(w http.ResponseWriter, r *http.Request) {
	day, err := opsDay(r.URL.Query().Get("day"))
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	rows, err := a.DB.Query(r.Context(), opsPersonSQL+`true ORDER BY a.name`, day)
	if err != nil {
		fail(w, 503, "roster unavailable")
		return
	}
	defer rows.Close()
	people := []map[string]any{}
	for rows.Next() {
		p, e := scanOpsPerson(rows)
		if e != nil {
			fail(w, 503, "roster unavailable")
			return
		}
		people = append(people, p.json())
	}
	if rows.Err() != nil {
		fail(w, 503, "roster unavailable")
		return
	}
	respond(w, 200, map[string]any{"day": day, "people": people})
}

func (a *App) opsLog(ctx context.Context, db execer, attendee, day, kind, station, detail string) error {
	_, err := db.Exec(ctx, `INSERT INTO ops_activity(id,attendee_id,event_day,kind,station,detail) VALUES($1,NULLIF($2,''),NULLIF($3,'')::date,$4,$5,$6)`, id(), attendee, day, kind, station, detail)
	return err
}

func decodeOpsScan(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	var in struct{ Code, Day, Reason string }
	if !decode(w, r, &in) {
		return "", "", false
	}
	day, err := opsDay(in.Day)
	if err != nil {
		fail(w, 400, err.Error())
		return "", "", false
	}
	return in.Code, day, true
}

func (a *App) opsCheckIn(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	code, day, ok := decodeOpsScan(w, r)
	if !ok {
		return
	}
	p, err := a.opsFind(r.Context(), code, day)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) || a.opsLog(r.Context(), a.DB, "", day, "check_in_denied", principal.Station, opsScanCode(code)+" | No approved pass matches that code.") != nil {
			fail(w, 503, "check-in unavailable")
			return
		}
		fail(w, 404, "no approved pass matches that code")
		return
	}
	repeat := false
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `INSERT INTO ops_attendance(attendee_id,event_day,checked_in_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, p.AttendeeID, day, principal.Station)
		if err != nil {
			return err
		}
		kind := "check_in"
		repeat = tag.RowsAffected() == 0
		if repeat {
			kind = "repeat_check_in"
		}
		return a.opsLog(r.Context(), tx, p.AttendeeID, day, kind, principal.Station, "")
	})
	if err != nil {
		fail(w, 503, "check-in unavailable")
		return
	}
	p, _ = a.opsFind(r.Context(), p.QRID, day)
	respond(w, 200, map[string]any{"person": p.json(), "repeat": repeat, "qrUrl": "/api/v1/ops/qr/" + p.QRID})
}

func (a *App) opsUndoCheckIn(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	var in struct{ Code, Day, Reason string }
	if !decode(w, r, &in) {
		return
	}
	day, err := opsDay(in.Day)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(in.Reason) == "" {
		fail(w, 400, "give a reason for the undo")
		return
	}
	p, err := a.opsFind(r.Context(), in.Code, day)
	if err != nil {
		fail(w, 404, "pass not found")
		return
	}
	changed := false
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `DELETE FROM ops_attendance WHERE attendee_id=$1 AND event_day=$2`, p.AttendeeID, day)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected() == 1
		if !changed {
			return nil
		}
		return a.opsLog(r.Context(), tx, p.AttendeeID, day, "undo_check_in", principal.Station, strings.TrimSpace(in.Reason))
	})
	if err != nil {
		fail(w, 503, "undo unavailable")
		return
	}
	if !changed {
		fail(w, 409, "there is no check-in to undo")
		return
	}
	p, _ = a.opsFind(r.Context(), p.QRID, day)
	respond(w, 200, map[string]any{"person": p.json()})
}

func (a *App) opsCheckOut(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	code, day, ok := decodeOpsScan(w, r)
	if !ok {
		return
	}
	p, err := a.opsFind(r.Context(), code, day)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) || a.opsLog(r.Context(), a.DB, "", day, "check_out_denied", principal.Station, opsScanCode(code)+" | No approved pass matches that code.") != nil {
			fail(w, 503, "checkout unavailable")
			return
		}
		fail(w, 404, "no approved pass matches that code")
		return
	}
	repeat, checkedIn := false, false
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		var checkedOut bool
		err := tx.QueryRow(r.Context(), `SELECT checked_out_at IS NOT NULL FROM ops_attendance WHERE attendee_id=$1 AND event_day=$2 FOR UPDATE`, p.AttendeeID, day).Scan(&checkedOut)
		if errors.Is(err, pgx.ErrNoRows) {
			return a.opsLog(r.Context(), tx, p.AttendeeID, day, "check_out_denied", principal.Station, p.Reference+" | This attendee has not checked in today.")
		}
		if err != nil {
			return err
		}
		checkedIn, repeat = true, checkedOut
		kind := "repeat_check_out"
		if !repeat {
			if _, err := tx.Exec(r.Context(), `UPDATE ops_attendance SET checked_out_at=now(),checked_out_by=$3 WHERE attendee_id=$1 AND event_day=$2`, p.AttendeeID, day, principal.Station); err != nil {
				return err
			}
			kind = "check_out"
		}
		return a.opsLog(r.Context(), tx, p.AttendeeID, day, kind, principal.Station, "")
	})
	if err != nil {
		fail(w, 503, "checkout unavailable")
		return
	}
	if !checkedIn {
		fail(w, 409, "this attendee has not checked in today")
		return
	}
	p, _ = a.opsFind(r.Context(), p.QRID, day)
	respond(w, 200, map[string]any{"person": p.json(), "repeat": repeat})
}

func (a *App) opsUndoCheckOut(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	var in struct{ Code, Day, Reason string }
	if !decode(w, r, &in) {
		return
	}
	day, err := opsDay(in.Day)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(in.Reason) == "" {
		fail(w, 400, "give a reason for the undo")
		return
	}
	p, err := a.opsFind(r.Context(), in.Code, day)
	if err != nil {
		fail(w, 404, "pass not found")
		return
	}
	changed := false
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `UPDATE ops_attendance SET checked_out_at=NULL,checked_out_by=NULL WHERE attendee_id=$1 AND event_day=$2 AND checked_out_at IS NOT NULL`, p.AttendeeID, day)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected() == 1
		if !changed {
			return nil
		}
		return a.opsLog(r.Context(), tx, p.AttendeeID, day, "undo_check_out", principal.Station, strings.TrimSpace(in.Reason))
	})
	if err != nil {
		fail(w, 503, "undo unavailable")
		return
	}
	if !changed {
		fail(w, 409, "there is no checkout to undo")
		return
	}
	p, _ = a.opsFind(r.Context(), p.QRID, day)
	respond(w, 200, map[string]any{"person": p.json()})
}

func (a *App) opsBadgePrinted(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	var in struct{ Code, Day string }
	if !decode(w, r, &in) {
		return
	}
	day, err := opsDay(in.Day)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	p, err := a.opsFind(r.Context(), in.Code, day)
	if err != nil {
		fail(w, 404, "pass not found")
		return
	}
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT 1 FROM attendees WHERE id=$1 FOR UPDATE`, p.AttendeeID); err != nil {
			return err
		}
		return a.opsLog(r.Context(), tx, p.AttendeeID, day, "badge_print", principal.Station, "")
	})
	if err != nil {
		fail(w, 503, "print could not be recorded")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func (a *App) opsQR(w http.ResponseWriter, r *http.Request, qrID string) {
	var ok bool
	_ = a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM passes p JOIN registrations r ON r.id=p.registration_id WHERE p.qr_id=$1 AND p.revoked_at IS NULL AND r.status='approved')`, qrID).Scan(&ok)
	if !ok {
		http.NotFound(w, r)
		return
	}
	png, err := qrcode.Encode(a.badgeURL(qrID), qrcode.Medium, 512)
	if err != nil {
		http.Error(w, "QR unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(png)
}

func (a *App) opsSpotRegister(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	var in struct {
		Day, CategoryID, Name, Email, Phone, Designation, Institution, Payment, PaymentReference, PaymentDate, CouponCode string
		AmountPaise                                                                                                       int64
	}
	if !decode(w, r, &in) {
		return
	}
	day, err := opsDay(in.Day)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	// The attendee is at the desk and agrees to delivery in person, so their
	// pass is sent at once by email and WhatsApp. A guest pass is never sent
	// (downloadOnly); the desk hands over the PDF instead.
	send := !downloadOnly(in.CategoryID)
	input := StaffRegistrationInput{RegistrationInput: RegistrationInput{CategoryID: in.CategoryID, CouponCode: in.CouponCode, Institution: in.Institution, ContactName: in.Name, Email: in.Email, Phone: in.Phone, Attendees: []Attendee{{Name: in.Name, Email: in.Email, Phone: in.Phone, Designation: in.Designation, WhatsAppConsent: send && strings.TrimSpace(in.Phone) != ""}}}, Payment: in.Payment, VerifiedReference: in.PaymentReference, VerifiedDate: in.PaymentDate, VerifiedAmountPaise: in.AmountPaise, Send: send, Spot: true, Note: "Spot registration at " + principal.Station}
	rid, err := a.StaffCreate(r.Context(), input, randomToken(), nil, "ops-system")
	if err != nil {
		fail(w, 400, publicError(err))
		return
	}
	var qrID string
	if err = a.DB.QueryRow(r.Context(), `SELECT p.qr_id FROM passes p WHERE p.registration_id=$1 AND p.revoked_at IS NULL LIMIT 1`, rid).Scan(&qrID); err != nil {
		fail(w, 503, "registration saved but pass lookup failed")
		return
	}
	p, _ := a.opsFind(r.Context(), qrID, day)
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `INSERT INTO ops_attendance(attendee_id,event_day,checked_in_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, p.AttendeeID, day, principal.Station); err != nil {
			return err
		}
		return a.opsLog(r.Context(), tx, p.AttendeeID, day, "spot_registration", principal.Station, in.Payment)
	})
	if err != nil {
		fail(w, 503, "registration saved but check-in failed")
		return
	}
	p, _ = a.opsFind(r.Context(), qrID, day)
	out := map[string]any{"person": p.json(), "qrUrl": "/api/v1/ops/qr/" + qrID}
	if downloadOnly(p.CategoryID) {
		out["passUrl"] = "/api/v1/ops/passes/" + qrID + ".pdf"
	}
	respond(w, 201, out)
}

// opsPassDownload hands the spot desk a guest's PDF pass, the only way a guest
// pass reaches its holder (downloadOnly). Other passes are sent to their
// holders, so the desk prints their badge instead.
func (a *App) opsPassDownload(w http.ResponseWriter, r *http.Request, principal opsPrincipal, file string) {
	qrID, isPDF := strings.CutSuffix(file, ".pdf")
	var pid, rid, number, cat string
	err := a.DB.QueryRow(r.Context(), `SELECT p.id,r.id,p.number,r.category_id FROM passes p JOIN registrations r ON r.id=p.registration_id WHERE p.qr_id=$1 AND p.revoked_at IS NULL AND r.status='approved'`, qrID).Scan(&pid, &rid, &number, &cat)
	if !isPDF || err != nil || !downloadOnly(cat) {
		fail(w, 404, "guest pass not found or revoked")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err == nil {
		defer tx.Rollback(r.Context())
		if err = audit(r.Context(), tx, "", rid, "pass_downloaded", number+" at "+principal.Station); err == nil {
			err = tx.Commit(r.Context())
		}
	}
	var b []byte
	if err == nil {
		b, err = a.passPDF(r.Context(), pid)
	}
	if err != nil {
		fail(w, 503, "pass is being prepared; retry shortly")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="Bio-Connect-4.0-pass-`+number+`.pdf"`)
	w.Write(b)
}

type accessPoint struct {
	ID, Name, Mode, Direction                    string
	AllowedCategories, AllowedDays               []string
	AllowedRegistrationTypes                     []string
	Capacity                                     *int
	RequireCheckIn, AllowMultipleEntries, Active bool
}

func pointMap(p accessPoint) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "mode": p.Mode, "direction": p.Direction, "allowedCategories": p.AllowedCategories, "allowedDays": p.AllowedDays, "allowedRegistrationTypes": p.AllowedRegistrationTypes, "capacity": p.Capacity, "requireCheckIn": p.RequireCheckIn, "allowMultipleEntries": p.AllowMultipleEntries, "active": p.Active}
}

func scanPoint(row pgx.Row) (accessPoint, error) {
	var p accessPoint
	err := row.Scan(&p.ID, &p.Name, &p.Mode, &p.Direction, &p.AllowedCategories, &p.AllowedDays, &p.Capacity, &p.RequireCheckIn, &p.AllowMultipleEntries, &p.Active, &p.AllowedRegistrationTypes)
	return p, err
}

const pointSelect = `SELECT id,name,mode,direction,allowed_categories,allowed_days::text[],capacity,require_check_in,allow_multiple_entries,active,allowed_registration_types FROM access_points`

func (a *App) opsPoints(w http.ResponseWriter, r *http.Request, principal opsPrincipal, rest string) {
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" && r.Method == http.MethodGet {
		rows, err := a.DB.Query(r.Context(), pointSelect+` ORDER BY name`)
		if err != nil {
			fail(w, 503, "gates unavailable")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			p, e := scanPoint(rows)
			if e != nil {
				fail(w, 503, "gates unavailable")
				return
			}
			out = append(out, pointMap(p))
		}
		respond(w, 200, map[string]any{"points": out})
		return
	}
	if rest == "" && r.Method == http.MethodPost {
		a.opsCreatePoint(w, r, principal)
		return
	}
	parts := strings.Split(rest, "/")
	p, err := scanPoint(a.DB.QueryRow(r.Context(), pointSelect+` WHERE id=$1`, parts[0]))
	if err != nil {
		fail(w, 404, "gate not found")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		var in struct {
			Active                   *bool
			AllowedRegistrationTypes *[]string
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Active == nil && in.AllowedRegistrationTypes == nil {
			fail(w, 400, "provide a gate change")
			return
		}
		if in.AllowedRegistrationTypes != nil {
			for _, kind := range *in.AllowedRegistrationTypes {
				if kind != "paid" && kind != "complimentary" && kind != "free_link" {
					fail(w, 400, "invalid registration type")
					return
				}
			}
		}
		err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
			var types any
			if in.AllowedRegistrationTypes != nil {
				types = *in.AllowedRegistrationTypes
			}
			var err error
			p, err = scanPoint(tx.QueryRow(r.Context(), `UPDATE access_points SET active=COALESCE($2,active),allowed_registration_types=COALESCE($3::text[],allowed_registration_types) WHERE id=$1 RETURNING id,name,mode,direction,allowed_categories,allowed_days::text[],capacity,require_check_in,allow_multiple_entries,active,allowed_registration_types`, p.ID, in.Active, types))
			if err != nil {
				return err
			}
			if in.Active != nil {
				kind := "gate_closed"
				if *in.Active {
					kind = "gate_opened"
				}
				if err := a.opsLog(r.Context(), tx, "", opsToday(a.Now()), kind, principal.Station, p.ID+" | "+p.Name); err != nil {
					return err
				}
			}
			if in.AllowedRegistrationTypes != nil {
				return a.opsLog(r.Context(), tx, "", opsToday(a.Now()), "gate_rules_updated", principal.Station, p.ID+" | "+p.Name+" | registration types: "+strings.Join(p.AllowedRegistrationTypes, ","))
			}
			return nil
		})
		if err != nil {
			fail(w, 503, "gate update failed")
			return
		}
		respond(w, 200, map[string]any{"point": pointMap(p)})
		return
	}
	if len(parts) == 2 && parts[1] == "occupancy" && r.Method == http.MethodGet {
		day, dayErr := opsDay(r.URL.Query().Get("day"))
		if dayErr != nil {
			fail(w, 400, dayErr.Error())
			return
		}
		inside, count, err := a.opsOccupancy(r.Context(), a.DB, p.ID, day)
		if err != nil {
			fail(w, 503, "occupancy unavailable")
			return
		}
		respond(w, 200, map[string]any{"inside": inside, "insideCount": count, "capacity": p.Capacity, "point": pointMap(p)})
		return
	}
	if len(parts) == 2 && parts[1] == "scan" && r.Method == http.MethodPost {
		a.opsGateScan(w, r, principal, p)
		return
	}
	if len(parts) == 2 && parts[1] == "entries" && r.Method == http.MethodGet {
		a.opsGateEntries(w, r, p)
		return
	}
	fail(w, 404, "not found")
}

func (a *App) opsCreatePoint(w http.ResponseWriter, r *http.Request, principal opsPrincipal) {
	var in struct {
		Name, Mode, Direction                string
		AllowedCategories, AllowedDays       []string
		AllowedRegistrationTypes             []string
		Capacity                             *int
		RequireCheckIn, AllowMultipleEntries bool
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 80 || (in.Mode != "enforce" && in.Mode != "log") || (in.Direction != "entry" && in.Direction != "exit" && in.Direction != "auto") {
		fail(w, 400, "provide a gate name, mode and direction")
		return
	}
	if len(in.AllowedDays) == 0 {
		in.AllowedDays = []string{opsDays[0].ID, opsDays[1].ID}
	}
	for _, d := range in.AllowedDays {
		if _, e := opsDay(d); e != nil {
			fail(w, 400, e.Error())
			return
		}
	}
	if in.AllowedCategories == nil {
		in.AllowedCategories = []string{}
	}
	if in.AllowedRegistrationTypes == nil {
		in.AllowedRegistrationTypes = []string{}
	}
	for _, kind := range in.AllowedRegistrationTypes {
		if kind != "paid" && kind != "complimentary" && kind != "free_link" {
			fail(w, 400, "invalid registration type")
			return
		}
	}
	p := accessPoint{ID: id(), Name: in.Name, Mode: in.Mode, Direction: in.Direction, AllowedCategories: in.AllowedCategories, AllowedDays: in.AllowedDays, AllowedRegistrationTypes: in.AllowedRegistrationTypes, Capacity: in.Capacity, RequireCheckIn: in.RequireCheckIn, AllowMultipleEntries: in.AllowMultipleEntries, Active: true}
	err := pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `INSERT INTO access_points(id,name,mode,direction,allowed_categories,allowed_days,capacity,require_check_in,allow_multiple_entries,allowed_registration_types) VALUES($1,$2,$3,$4,$5,$6::date[],$7,$8,$9,$10)`, p.ID, p.Name, p.Mode, p.Direction, p.AllowedCategories, p.AllowedDays, p.Capacity, p.RequireCheckIn, p.AllowMultipleEntries, p.AllowedRegistrationTypes); err != nil {
			return err
		}
		types := strings.Join(p.AllowedRegistrationTypes, ",")
		if types == "" {
			types = "all"
		}
		return a.opsLog(r.Context(), tx, "", opsToday(a.Now()), "gate_created", principal.Station, p.ID+" | "+p.Name+" | registration types: "+types)
	})
	if err != nil {
		fail(w, 503, "gate could not be created or recorded; check for a duplicate name")
		return
	}
	respond(w, 201, map[string]any{"point": pointMap(p)})
}

type opsQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (a *App) opsOccupancy(ctx context.Context, db opsQuerier, point, day string) ([]map[string]any, int, error) {
	rows, err := db.Query(ctx, `WITH latest AS (SELECT DISTINCT ON(attendee_id) attendee_id,reference,direction,created_at FROM access_scans WHERE access_point_id=$1 AND event_day=$2 AND decision='allow' AND voided_at IS NULL AND attendee_id IS NOT NULL ORDER BY attendee_id,created_at DESC,id DESC) SELECT a.name,l.reference,c.label,l.created_at FROM latest l JOIN attendees a ON a.id=l.attendee_id JOIN registrations r ON r.id=a.registration_id JOIN categories c ON c.id=r.category_id WHERE l.direction='entry' ORDER BY l.created_at DESC`, point, day)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var n, ref, cat string
		var at time.Time
		if err := rows.Scan(&n, &ref, &cat, &at); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]any{"name": n, "reference": ref, "category": cat, "enteredAt": at})
	}
	return out, len(out), rows.Err()
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func (a *App) opsGateScan(w http.ResponseWriter, r *http.Request, principal opsPrincipal, point accessPoint) {
	var in struct{ Code, Day, RequestID string }
	if !decode(w, r, &in) {
		return
	}
	day, err := opsDay(in.Day)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if len(in.RequestID) > 100 {
		fail(w, 400, "invalid request identifier")
		return
	}
	var out map[string]any
	requestHash := hash(day + "|" + opsScanCode(in.Code))
	err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		// One gate lock covers the current policy, every attendee and capacity.
		// Re-read after taking the lock, so a concurrent close cannot be missed.
		var err error
		point, err = scanPoint(tx.QueryRow(r.Context(), pointSelect+` WHERE id=$1 FOR UPDATE`, point.ID))
		if err != nil {
			return err
		}
		if in.RequestID != "" {
			var savedHash string
			var saved []byte
			err := tx.QueryRow(r.Context(), `SELECT request_hash,response FROM access_scans WHERE access_point_id=$1 AND station=$2 AND request_id=$3`, point.ID, principal.Station, in.RequestID).Scan(&savedHash, &saved)
			if err == nil {
				if savedHash != requestHash {
					return errOpsRequestConflict
				}
				return json.Unmarshal(saved, &out)
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		p, findErr := scanOpsPerson(tx.QueryRow(r.Context(), opsPersonSQL+`(p.qr_id=$2 OR replace(p.number,'-','')=$3)`, day, opsScanCode(in.Code), opsReference(in.Code)))
		valid := findErr == nil
		if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
			return findErr
		}
		if !valid {
			// Historical badges may identify an occupant for EXIT only.
			historySQL := strings.Replace(opsPersonSQL, " AND p.revoked_at IS NULL", "", 1)
			historySQL = strings.Replace(historySQL, "r.status='approved' AND a.removed_at IS NULL AND ", "", 1)
			p, findErr = scanOpsPerson(tx.QueryRow(r.Context(), historySQL+`(p.qr_id=$2 OR replace(p.number,'-','')=$3) ORDER BY p.version DESC LIMIT 1`, day, opsScanCode(in.Code), opsReference(in.Code)))
			if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
				return findErr
			}
		}
		inside, entries := false, 0
		if findErr == nil {
			if err := tx.QueryRow(r.Context(), `SELECT COALESCE((array_agg(direction ORDER BY created_at DESC,id DESC))[1]='entry',false),count(*) FILTER(WHERE direction='entry') FROM access_scans WHERE access_point_id=$1 AND attendee_id=$2 AND event_day=$3 AND decision='allow' AND voided_at IS NULL`, point.ID, p.AttendeeID, day).Scan(&inside, &entries); err != nil {
				return err
			}
		}
		direction := point.Direction
		if direction == "auto" {
			direction = "entry"
			if inside {
				direction = "exit"
			}
		}
		reason := ""
		if !point.Active {
			reason = "Gate is closed."
		} else if !valid {
			reason = "No approved pass matches that code."
		} else if direction == "entry" && !contains(point.AllowedDays, day) {
			reason = "This gate is not open for the selected day."
		} else if direction == "entry" && len(point.AllowedCategories) > 0 && !contains(point.AllowedCategories, p.CategoryID) {
			reason = "This badge category is not allowed here."
		} else if direction == "entry" && len(point.AllowedRegistrationTypes) > 0 && !contains(point.AllowedRegistrationTypes, p.RegistrationType) {
			reason = "This registration type is not allowed here."
		} else if direction == "entry" && point.RequireCheckIn && !p.CheckedIn {
			reason = "Badge-desk check-in is required first."
		} else if direction == "entry" && !point.AllowMultipleEntries && entries > 0 {
			reason = "This gate allows one entry only."
		} else if direction == "entry" && point.Capacity != nil {
			_, n, err := a.opsOccupancy(r.Context(), tx, point.ID, day)
			if err != nil {
				return err
			}
			if n >= *point.Capacity {
				reason = "This area is at capacity."
			}
		}
		if direction == "exit" && inside {
			reason = ""
		}
		decision, would := "allow", ""
		if reason != "" {
			if point.Mode == "log" {
				would, reason = reason, ""
			} else {
				decision = "deny"
			}
		}
		scanID, attendee, reference := id(), "", opsScanCode(in.Code)
		var person any
		if findErr == nil {
			attendee, reference, person = p.AttendeeID, p.Reference, p.json()
		}
		// clock_timestamp follows lock acquisition; transaction-start now() could
		// otherwise put a waiting scan before the scan it actually followed.
		if _, err := tx.Exec(r.Context(), `INSERT INTO access_scans(id,access_point_id,attendee_id,event_day,reference,direction,decision,reason,would_deny,station,request_id,request_hash,created_at) VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),$12,clock_timestamp())`, scanID, point.ID, attendee, day, reference, direction, decision, reason, would, principal.Station, in.RequestID, requestHash); err != nil {
			return err
		}
		kind := "gate_allow"
		if decision == "deny" {
			kind = "gate_deny"
		}
		if err := a.opsLog(r.Context(), tx, attendee, day, kind, principal.Station, reference+" | "+point.Name+": "+reason+would); err != nil {
			return err
		}
		_, count, err := a.opsOccupancy(r.Context(), tx, point.ID, day)
		if err != nil {
			return err
		}
		out = map[string]any{"allowed": decision == "allow", "direction": direction, "reason": reason, "wouldDeny": would, "person": person, "insideCount": count, "capacity": point.Capacity, "scanId": scanID}
		if in.RequestID != "" {
			response, err := json.Marshal(out)
			if err != nil {
				return err
			}
			_, err = tx.Exec(r.Context(), `UPDATE access_scans SET response=$2 WHERE id=$1`, scanID, response)
			return err
		}
		return nil
	})
	if errors.Is(err, errOpsRequestConflict) {
		fail(w, 409, "request identifier already used for a different scan")
		return
	}
	if err != nil {
		fail(w, 503, "gate scan could not be recorded")
		return
	}
	respond(w, 200, out)
}

var errOpsRequestConflict = errors.New("request identifier already used")

func (a *App) opsGateEntries(w http.ResponseWriter, r *http.Request, p accessPoint) {
	rows, err := a.DB.Query(r.Context(), `SELECT s.id,s.created_at,s.reference,COALESCE(a.name,''),s.direction,s.decision,s.reason,s.would_deny,s.station FROM access_scans s LEFT JOIN attendees a ON a.id=s.attendee_id WHERE s.access_point_id=$1 ORDER BY s.created_at DESC LIMIT 500`, p.ID)
	if err != nil {
		fail(w, 503, "gate log unavailable")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, ref, name, dir, decision, reason, would, station string
		var at time.Time
		if rows.Scan(&id, &at, &ref, &name, &dir, &decision, &reason, &would, &station) == nil {
			items = append(items, map[string]any{"id": id, "at": at, "reference": ref, "name": name, "direction": dir, "decision": decision, "reason": reason, "wouldDeny": would, "station": station})
		}
	}
	respond(w, 200, map[string]any{"point": pointMap(p), "entries": items})
}

func (a *App) opsReports(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(r.Context(), `SELECT d.day::text,c.id,c.label,COALESCE(reg.n,0),COALESCE(att.ins,0),COALESCE(att.outs,0) FROM (VALUES('2026-10-08'::date),('2026-10-09'::date)) d(day) CROSS JOIN categories c LEFT JOIN (SELECT r.category_id,count(*) n FROM attendees a JOIN registrations r ON r.id=a.registration_id JOIN passes p ON p.attendee_id=a.id AND p.revoked_at IS NULL WHERE r.status='approved' AND a.removed_at IS NULL GROUP BY r.category_id) reg ON reg.category_id=c.id LEFT JOIN (SELECT oa.event_day,r.category_id,count(*) ins,count(*) FILTER(WHERE oa.checked_out_at IS NOT NULL) outs FROM ops_attendance oa JOIN attendees a ON a.id=oa.attendee_id JOIN registrations r ON r.id=a.registration_id GROUP BY oa.event_day,r.category_id) att ON att.event_day=d.day AND att.category_id=c.id ORDER BY d.day,c.kind,c.label`)
	if err != nil {
		fail(w, 503, "reports unavailable")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var day, timeID, label string
		var registered, ins, outs int
		if rows.Scan(&day, &timeID, &label, &registered, &ins, &outs) == nil {
			items = append(items, map[string]any{"day": day, "categoryId": timeID, "category": label, "registered": registered, "checkedIn": ins, "checkedOut": outs})
		}
	}
	gates, err := a.opsGateTotals(r.Context())
	if err != nil {
		fail(w, 503, "reports unavailable")
		return
	}
	var registered, unique, daily, outs int
	_ = a.DB.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM attendees a JOIN registrations r ON r.id=a.registration_id JOIN passes p ON p.attendee_id=a.id AND p.revoked_at IS NULL WHERE r.status='approved' AND a.removed_at IS NULL),(SELECT count(DISTINCT attendee_id) FROM ops_attendance),(SELECT count(*) FROM ops_attendance),(SELECT count(*) FROM ops_attendance WHERE checked_out_at IS NOT NULL)`).Scan(&registered, &unique, &daily, &outs)
	respond(w, 200, map[string]any{"summary": map[string]int{"registered": registered, "uniqueCheckedIn": unique, "dailyCheckIns": daily, "checkouts": outs, "notYetCheckedIn": registered - unique}, "rows": items, "gates": gates})
}

// opsGateTotals counts non-voided scans per gate and event day. "inside" uses
// the same rule as opsOccupancy: the attendee's latest allowed scan is an entry.
func (a *App) opsGateTotals(ctx context.Context) ([]map[string]any, error) {
	days := make([]string, len(opsDays))
	for i, d := range opsDays {
		days[i] = d.ID
	}
	rows, err := a.DB.Query(ctx, `WITH s AS (SELECT * FROM access_scans WHERE voided_at IS NULL),
	 totals AS (SELECT access_point_id,event_day,
	  count(*) FILTER(WHERE decision='allow' AND direction='entry') entries,
	  count(DISTINCT attendee_id) FILTER(WHERE decision='allow' AND direction='entry') people,
	  count(*) FILTER(WHERE decision='allow' AND direction='exit') exits,
	  count(*) FILTER(WHERE decision='deny') denied
	  FROM s GROUP BY access_point_id,event_day),
	 latest AS (SELECT DISTINCT ON(access_point_id,event_day,attendee_id) access_point_id,event_day,direction FROM s WHERE decision='allow' AND attendee_id IS NOT NULL ORDER BY access_point_id,event_day,attendee_id,created_at DESC,id DESC),
	 inside AS (SELECT access_point_id,event_day,count(*) n FROM latest WHERE direction='entry' GROUP BY access_point_id,event_day)
	 SELECT d.day::text,p.id,p.name,p.active,COALESCE(t.entries,0),COALESCE(t.people,0),COALESCE(t.exits,0),COALESCE(t.denied,0),COALESCE(i.n,0)
	 FROM unnest($1::date[]) d(day) CROSS JOIN access_points p
	 LEFT JOIN totals t ON t.access_point_id=p.id AND t.event_day=d.day
	 LEFT JOIN inside i ON i.access_point_id=p.id AND i.event_day=d.day
	 ORDER BY d.day,p.name`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var day, id, name string
		var active bool
		var entries, people, exits, denied, inside int
		if err := rows.Scan(&day, &id, &name, &active, &entries, &people, &exits, &denied, &inside); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"day": day, "pointId": id, "gate": name, "active": active, "entries": entries, "uniquePeople": people, "exits": exits, "denied": denied, "inside": inside})
	}
	return out, rows.Err()
}

type opsActivityRow struct {
	At                         time.Time
	Day, Kind, Station, Detail string
	Name, Reference, Category  string
}

func opsActivityOutcome(kind string) string {
	if strings.HasSuffix(kind, "_denied") || kind == "gate_deny" {
		return "rejected"
	}
	if strings.HasPrefix(kind, "repeat_") {
		return "repeat"
	}
	return "success"
}

func (a *App) opsActivityRows(ctx context.Context, day string, limit int) ([]opsActivityRow, error) {
	q := `SELECT o.created_at,COALESCE(o.event_day::text,''),o.kind,o.station,o.detail,
	 COALESCE(a.name,''),COALESCE(p.number,''),COALESCE(c.label,'')
	 FROM ops_activity o
	 LEFT JOIN attendees a ON a.id=o.attendee_id
	 LEFT JOIN registrations r ON r.id=a.registration_id
	 LEFT JOIN categories c ON c.id=r.category_id
	 LEFT JOIN passes p ON p.attendee_id=a.id AND p.revoked_at IS NULL`
	args := []any{}
	if day != "" {
		q += ` WHERE o.event_day=$1::date`
		args = append(args, day)
	}
	q += ` ORDER BY o.created_at DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := a.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []opsActivityRow{}
	for rows.Next() {
		var item opsActivityRow
		if err := rows.Scan(&item.At, &item.Day, &item.Kind, &item.Station, &item.Detail, &item.Name, &item.Reference, &item.Category); err != nil {
			return nil, err
		}
		if parts := strings.SplitN(item.Detail, " | ", 2); len(parts) == 2 {
			if item.Reference == "" {
				item.Reference = parts[0]
			}
			item.Detail = parts[1]
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a *App) opsActivity(w http.ResponseWriter, r *http.Request) {
	day := r.URL.Query().Get("day")
	if day != "" {
		if _, err := opsDay(day); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	rows, err := a.opsActivityRows(r.Context(), day, 500)
	if err != nil {
		fail(w, 503, "activity log unavailable")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"at": row.At, "day": row.Day, "kind": row.Kind, "outcome": opsActivityOutcome(row.Kind),
			"station": row.Station, "detail": row.Detail, "name": row.Name,
			"reference": row.Reference, "category": row.Category,
		})
	}
	respond(w, 200, map[string]any{"activity": items})
}

func (a *App) opsActivityExport(w http.ResponseWriter, r *http.Request) {
	day := r.URL.Query().Get("day")
	if day != "" {
		if _, err := opsDay(day); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	rows, err := a.opsActivityRows(r.Context(), day, 0)
	if err != nil {
		fail(w, 503, "activity export unavailable")
		return
	}
	var b bytes.Buffer
	cw := csv.NewWriter(&b)
	_ = cw.Write([]string{"Time", "Event day", "Outcome", "Action", "Name", "Pass number", "Category", "Station", "Detail"})
	for _, row := range rows {
		_ = cw.Write([]string{
			row.At.In(india).Format(time.RFC3339), row.Day, opsActivityOutcome(row.Kind), row.Kind,
			safeCSV(row.Name), safeCSV(row.Reference), safeCSV(row.Category), safeCSV(row.Station), safeCSV(row.Detail),
		})
	}
	cw.Flush()
	if cw.Error() != nil {
		fail(w, 500, "activity export failed")
		return
	}
	name := "Bio-Connect-operations-audit"
	if day != "" {
		name += "-" + day
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	_, _ = w.Write(b.Bytes())
}

func safeCSV(v string) string {
	if strings.ContainsAny(v[:min(1, len(v))], "=+-@") {
		return "'" + v
	}
	return v
}
func (a *App) opsReportExport(w http.ResponseWriter, r *http.Request) {
	day := r.URL.Query().Get("day")
	if day != "" {
		if _, e := opsDay(day); e != nil {
			fail(w, 400, e.Error())
			return
		}
	}
	q := `SELECT p.number,a.name,a.email,a.phone,a.designation,r.institution,c.label,oa.event_day,oa.checked_in_at,oa.checked_in_by,oa.checked_out_at,oa.checked_out_by FROM ops_attendance oa JOIN attendees a ON a.id=oa.attendee_id JOIN registrations r ON r.id=a.registration_id JOIN categories c ON c.id=r.category_id JOIN passes p ON p.attendee_id=a.id AND p.revoked_at IS NULL`
	args := []any{}
	if day != "" {
		q += ` WHERE oa.event_day=$1`
		args = append(args, day)
	}
	q += ` ORDER BY oa.event_day,oa.checked_in_at`
	rows, err := a.DB.Query(r.Context(), q, args...)
	if err != nil {
		fail(w, 503, "export unavailable")
		return
	}
	defer rows.Close()
	var b bytes.Buffer
	cw := csv.NewWriter(&b)
	_ = cw.Write([]string{"Pass number", "Name", "Email", "Phone", "Designation", "Institution", "Category", "Event day", "Checked in at", "Checked in by", "Checked out at", "Checked out by"})
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			fail(w, 503, "export unavailable")
			return
		}
		record := make([]string, len(vals))
		for i, v := range vals {
			if v != nil {
				if t, ok := v.(time.Time); ok {
					record[i] = t.In(india).Format(time.RFC3339)
				} else {
					record[i] = safeCSV(fmt.Sprint(v))
				}
			}
		}
		_ = cw.Write(record)
	}
	cw.Flush()
	if cw.Error() != nil {
		fail(w, 500, "export failed")
		return
	}
	name := "Bio-Connect-attendance"
	if day != "" {
		name += "-" + day
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	_, _ = w.Write(b.Bytes())
}
