package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func freeLinkExpiry(date string) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, india)
	if err != nil {
		return time.Time{}, errors.New("expiry date must use YYYY-MM-DD")
	}
	return day.AddDate(0, 0, 1), nil
}

// validFreeLink locks the link against concurrent revocation until the caller's
// transaction commits. Create separately ensures the selected category matches
// the registration type the staff member chose for the link.
func (a *App) validFreeLink(ctx context.Context, tx pgx.Tx, token string) (string, bool, string, error) {
	if len(token) != 43 {
		return "", false, "", errors.New("this free registration link is invalid or has expired")
	}
	var linkID string
	var autoApprove bool
	var registrationKind string
	err := tx.QueryRow(ctx, `SELECT id,auto_approve,registration_kind FROM free_registration_links
		WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>$2 FOR SHARE`, hash(token), a.Now()).Scan(&linkID, &autoApprove, &registrationKind)
	if err != nil {
		return "", false, "", errors.New("this free registration link is invalid or has expired")
	}
	return linkID, autoApprove, registrationKind, nil
}

func (a *App) validateFreeLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if a.limited(r.Context(), "free-link:"+a.clientPeer(r), 120, time.Hour) {
		fail(w, 429, "too many attempts; try again later")
		return
	}
	var expires time.Time
	var autoApprove bool
	var registrationKind string
	err := a.DB.QueryRow(r.Context(), `SELECT expires_at,auto_approve,registration_kind FROM free_registration_links
		WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>$2`, hash(in.Token), a.Now()).Scan(&expires, &autoApprove, &registrationKind)
	if err != nil {
		fail(w, 404, "this free registration link is invalid or has expired")
		return
	}
	respond(w, 200, map[string]any{"valid": true, "expires_on": expires.In(india).AddDate(0, 0, -1).Format("2006-01-02"), "auto_approve": autoApprove, "registration_kind": registrationKind})
}

func (a *App) freeLinksAPI(w http.ResponseWriter, r *http.Request, p principal, path string) {
	switch {
	case path == "free-links" && r.Method == "GET":
		items, err := a.queryMaps(r, `SELECT l.id,l.registration_kind,l.expires_at,(l.expires_at AT TIME ZONE 'Asia/Kolkata')::date-1 AS expires_on,l.auto_approve,l.created_at,l.revoked_at,s.email AS created_by,
			(SELECT count(*) FROM registrations x WHERE x.free_link_id=l.id) AS use_count
			FROM free_registration_links l JOIN staff s ON s.id=l.created_by
			ORDER BY l.created_at DESC LIMIT 50`)
		if err != nil {
			fail(w, 503, "free links unavailable")
			return
		}
		respond(w, 200, map[string]any{"items": items, "server_time": a.Now().In(india)})
	case path == "free-links" && r.Method == "POST":
		var in struct {
			ExpiresOn        string `json:"expires_on"`
			RegistrationKind string `json:"registration_kind"`
			AutoApprove      bool   `json:"auto_approve"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.RegistrationKind != "delegate" && in.RegistrationKind != "exhibitor" {
			fail(w, 400, "choose delegates or exhibitors")
			return
		}
		expires, err := freeLinkExpiry(in.ExpiresOn)
		if err != nil || !expires.After(a.Now()) {
			fail(w, 400, "choose today or a future expiry date")
			return
		}
		tx, err := a.DB.Begin(r.Context())
		if err != nil {
			fail(w, 503, "could not generate free link")
			return
		}
		defer tx.Rollback(r.Context())
		// Each registration type has one current invitation link. Generating a
		// replacement leaves the other type's active link untouched.
		if _, err = tx.Exec(r.Context(), `UPDATE free_registration_links
			SET revoked_at=$1,revoked_by=$2 WHERE registration_kind=$3 AND revoked_at IS NULL`, a.Now(), p.ID, in.RegistrationKind); err == nil {
			token, linkID := randomToken(), id()
			_, err = tx.Exec(r.Context(), `INSERT INTO free_registration_links(id,token_hash,expires_at,auto_approve,registration_kind,created_by)
				VALUES($1,$2,$3,$4,$5,$6)`, linkID, hash(token), expires, in.AutoApprove, in.RegistrationKind, p.ID)
			if err == nil {
				err = audit(r.Context(), tx, p.ID, "", "free_link_generated", linkID+" kind="+in.RegistrationKind+" expires="+in.ExpiresOn+" auto_approve="+strconv.FormatBool(in.AutoApprove))
			}
			if err == nil {
				err = tx.Commit(r.Context())
			}
			if err == nil {
				path := "/delegates"
				if in.RegistrationKind == "exhibitor" {
					path = "/exhibitors"
				}
				respond(w, 201, map[string]any{"id": linkID, "url": a.Config.BaseURL + path + "#free=" + token, "expires_at": expires})
				return
			}
		}
		fail(w, 503, "could not generate free link")
	case strings.HasPrefix(path, "free-links/") && strings.HasSuffix(path, "/expire") && r.Method == "POST":
		linkID := strings.TrimSuffix(strings.TrimPrefix(path, "free-links/"), "/expire")
		tx, err := a.DB.Begin(r.Context())
		if err != nil {
			fail(w, 503, "could not expire free link")
			return
		}
		defer tx.Rollback(r.Context())
		tag, err := tx.Exec(r.Context(), `UPDATE free_registration_links SET revoked_at=$2,revoked_by=$3
			WHERE id=$1 AND revoked_at IS NULL`, linkID, a.Now(), p.ID)
		if err == nil && tag.RowsAffected() != 1 {
			fail(w, 409, "link is already expired")
			return
		}
		if err == nil {
			err = audit(r.Context(), tx, p.ID, "", "free_link_expired", linkID)
		}
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			fail(w, 503, "could not expire free link")
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 404, "not found")
	}
}
