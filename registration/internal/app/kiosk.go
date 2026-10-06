package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The self-service kiosk lets an attendee scan their pass QR, confirm it is
// theirs and print their own badge; printing is what checks them in.
//
// It runs unattended, so it is deliberately narrower than a staffed desk:
//   - only the full-entropy QR identifier is accepted, never a typed pass number,
//     which is short and guessable;
//   - a pass prints once at the kiosk. A badge that did not come out may be
//     reprinted from the same kiosk for a few minutes, and anything later is a
//     help-desk reprint;
//   - the event day comes from the server clock, not the device;
//   - responses carry only what the badge itself prints, never contact details.
const kioskDetail = "Self-service kiosk"

// kioskReprintWindow and kioskReprintLimit bound "my badge didn't come out".
const (
	kioskReprintWindow = "5 minutes"
	kioskReprintLimit  = 3
)

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

var errKioskAlreadyPrinted = errors.New("already printed")

func (p opsPerson) kioskJSON() map[string]any {
	return map[string]any{
		"qrId": p.QRID, "reference": p.Reference, "name": p.Name,
		"designation": p.Designation, "institution": p.Institution,
		"categoryId": p.CategoryID, "category": p.Category,
	}
}

func opsDayLabel(day string) string {
	for _, d := range opsDays {
		if d.ID == day {
			return d.Label
		}
	}
	return day
}

func (a *App) kioskPage(w http.ResponseWriter, r *http.Request) {
	b, err := resources.ReadFile("web/kiosk.html")
	if err != nil {
		http.Error(w, "Kiosk unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *App) kioskAPI(w http.ResponseWriter, r *http.Request, p opsPrincipal, path string) {
	switch {
	case path == "kiosk/scan" && r.Method == http.MethodPost:
		a.kioskScan(w, r, p)
	case path == "kiosk/print" && r.Method == http.MethodPost:
		a.kioskPrint(w, r, p)
	case path == "kiosk/check-in" && r.Method == http.MethodPost:
		a.kioskCheckIn(w, r, p)
	case path == "kiosk/unlock" && r.Method == http.MethodPost:
		a.kioskUnlock(w, r)
	case path == "kiosk/exit" && r.Method == http.MethodPost:
		a.kioskExit(w, r, p)
	case strings.HasPrefix(path, "qr/") && r.Method == http.MethodGet:
		a.opsQR(w, r, strings.TrimPrefix(path, "qr/"))
	default:
		fail(w, 403, "this device is in kiosk mode")
	}
}

// kioskStart turns the current staff station into a kiosk. The staff session
// is replaced, so the tablet no longer holds a session with desk powers.
func (a *App) kioskStart(w http.ResponseWriter, r *http.Request, p opsPrincipal) {
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	c, _ := r.Cookie("bc_ops")
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		fail(w, 503, "kiosk could not start")
		return
	}
	defer tx.Rollback(r.Context())
	if c != nil {
		_, _ = tx.Exec(r.Context(), "DELETE FROM ops_sessions WHERE token_hash=$1", hash(c.Value))
	}
	csrf, err := a.opsOpenSession(r.Context(), w, tx, p.Station, true)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, 503, "kiosk could not start")
		return
	}
	a.opsLog(r.Context(), "", opsToday(a.Now()), "kiosk_open", p.Station, "")
	respond(w, 200, map[string]any{"station": p.Station, "csrf": csrf, "kiosk": true})
}

func (a *App) kioskUnlock(w http.ResponseWriter, r *http.Request) {
	var in struct{ Passcode string }
	if !decode(w, r, &in) || !a.opsPasscodeValid(w, r, in.Passcode) {
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

// kioskExit hands the device back to staff as an ordinary station.
func (a *App) kioskExit(w http.ResponseWriter, r *http.Request, p opsPrincipal) {
	var in struct{ Passcode string }
	if !decode(w, r, &in) || !a.opsPasscodeValid(w, r, in.Passcode) {
		return
	}
	c, _ := r.Cookie("bc_ops")
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		fail(w, 503, "kiosk could not close")
		return
	}
	defer tx.Rollback(r.Context())
	if c != nil {
		_, _ = tx.Exec(r.Context(), "DELETE FROM ops_sessions WHERE token_hash=$1", hash(c.Value))
	}
	csrf, err := a.opsOpenSession(r.Context(), w, tx, p.Station, false)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, 503, "kiosk could not close")
		return
	}
	a.opsLog(r.Context(), "", opsToday(a.Now()), "kiosk_close", p.Station, "")
	respond(w, 200, map[string]any{"station": p.Station, "csrf": csrf, "kiosk": false})
}

// kioskFind resolves a scanned value to a pass by its QR identifier only.
// ok is false (with the response written) when the value is not a pass QR or
// matches no approved pass; both are kept in the audit as rejected check-ins.
func (a *App) kioskFind(w http.ResponseWriter, r *http.Request, p opsPrincipal, code string) (opsPerson, string, bool) {
	day := opsToday(a.Now())
	token := opsScanCode(code)
	if !admissionQRPattern.MatchString(token) {
		respond(w, 422, map[string]string{"error": "scan the QR code on your pass", "code": "qr_required"})
		return opsPerson{}, "", false
	}
	person, err := scanOpsPerson(a.DB.QueryRow(r.Context(), opsPersonSQL+`p.qr_id=$2`, day, token))
	if err != nil {
		a.opsLog(r.Context(), "", day, "check_in_denied", p.Station, token+" | "+kioskDetail+": no approved pass matches that code.")
		respond(w, 404, map[string]string{"error": "no approved pass matches that code", "code": "not_found"})
		return opsPerson{}, "", false
	}
	return person, day, true
}

func (a *App) kioskPrinted(ctx context.Context, attendee string) (bool, error) {
	var printed bool
	err := a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ops_activity WHERE attendee_id=$1 AND kind='badge_print')`, attendee).Scan(&printed)
	return printed, err
}

// kioskScan tells the kiosk what this attendee needs today:
//   - "print": no badge has been printed for the pass yet;
//   - "check-in": they already have a badge and are not checked in today;
//   - "done": they have a badge and are already checked in today.
func (a *App) kioskScan(w http.ResponseWriter, r *http.Request, p opsPrincipal) {
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	person, day, ok := a.kioskFind(w, r, p, in.Code)
	if !ok {
		return
	}
	printed, err := a.kioskPrinted(r.Context(), person.AttendeeID)
	if err != nil {
		fail(w, 503, "kiosk unavailable")
		return
	}
	status := "print"
	if printed {
		status = "check-in"
		if person.CheckedIn {
			status = "done"
		}
	}
	respond(w, 200, map[string]any{"status": status, "day": day, "dayLabel": opsDayLabel(day), "person": person.kioskJSON(), "qrUrl": "/api/v1/ops/qr/" + person.QRID})
}

// kioskPrint authorises a badge print and records it together with the day's
// check-in, so a printed badge always means a checked-in attendee. retry asks
// to print again because the badge did not come out; it is honoured only for
// a pass this kiosk printed moments ago.
func (a *App) kioskPrint(w http.ResponseWriter, r *http.Request, p opsPrincipal) {
	var in struct {
		Code  string
		Retry bool
	}
	if !decode(w, r, &in) {
		return
	}
	person, day, ok := a.kioskFind(w, r, p, in.Code)
	if !ok {
		return
	}
	firstCheckIn := false
	err := pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
		// Serialise prints for one attendee so two kiosks cannot both print a first badge.
		if _, err := tx.Exec(r.Context(), `SELECT 1 FROM attendees WHERE id=$1 FOR UPDATE`, person.AttendeeID); err != nil {
			return err
		}
		var total, recent int
		if err := tx.QueryRow(r.Context(), `SELECT count(*),count(*) FILTER (WHERE station=$2 AND detail LIKE $3 AND created_at>now()-$4::interval)
		 FROM ops_activity WHERE attendee_id=$1 AND kind='badge_print'`, person.AttendeeID, p.Station, kioskDetail+"%", kioskReprintWindow).Scan(&total, &recent); err != nil {
			return err
		}
		detail := kioskDetail
		if total > 0 {
			if !in.Retry || recent == 0 || recent >= kioskReprintLimit {
				return errKioskAlreadyPrinted
			}
			detail = kioskDetail + " reprint"
		}
		tag, err := tx.Exec(r.Context(), `INSERT INTO ops_attendance(attendee_id,event_day,checked_in_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, person.AttendeeID, day, p.Station)
		if err != nil {
			return err
		}
		if firstCheckIn = tag.RowsAffected() == 1; firstCheckIn {
			if _, err := tx.Exec(r.Context(), `INSERT INTO ops_activity(id,attendee_id,event_day,kind,station,detail) VALUES($1,$2,$3,'check_in',$4,$5)`, id(), person.AttendeeID, day, p.Station, kioskDetail); err != nil {
				return err
			}
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO ops_activity(id,attendee_id,event_day,kind,station,detail) VALUES($1,$2,$3,'badge_print',$4,$5)`, id(), person.AttendeeID, day, p.Station, detail)
		return err
	})
	if errors.Is(err, errKioskAlreadyPrinted) {
		respond(w, 409, map[string]string{"error": "a badge has already been printed for this pass", "code": "already_printed"})
		return
	}
	if err != nil {
		fail(w, 503, "kiosk unavailable")
		return
	}
	respond(w, 200, map[string]any{"checkedIn": true, "firstCheckIn": firstCheckIn, "day": day, "dayLabel": opsDayLabel(day), "person": person.kioskJSON()})
}

// kioskCheckIn admits an attendee who already has a badge. A first badge has
// to go through kioskPrint, so check-in and printing cannot drift apart.
func (a *App) kioskCheckIn(w http.ResponseWriter, r *http.Request, p opsPrincipal) {
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	person, day, ok := a.kioskFind(w, r, p, in.Code)
	if !ok {
		return
	}
	printed, err := a.kioskPrinted(r.Context(), person.AttendeeID)
	if err != nil {
		fail(w, 503, "kiosk unavailable")
		return
	}
	if !printed {
		respond(w, 409, map[string]string{"error": "print the badge to check in", "code": "print_required"})
		return
	}
	tag, err := a.DB.Exec(r.Context(), `INSERT INTO ops_attendance(attendee_id,event_day,checked_in_by) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, person.AttendeeID, day, p.Station)
	if err != nil {
		fail(w, 503, "kiosk unavailable")
		return
	}
	kind := "check_in"
	if tag.RowsAffected() == 0 {
		kind = "repeat_check_in"
	}
	a.opsLog(r.Context(), person.AttendeeID, day, kind, p.Station, kioskDetail)
	respond(w, 200, map[string]any{"checkedIn": true, "repeat": kind != "check_in", "day": day, "dayLabel": opsDayLabel(day), "person": person.kioskJSON()})
}
