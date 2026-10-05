package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// badgeContact is what the app saves when an attendee scans another's badge:
// what the badge already prints, plus the email and phone the holder chose to
// share from My passes. Nothing else about the registration is exposed.
type badgeContact struct {
	QRID        string `json:"qr_id"`
	Name        string `json:"name"`
	Designation string `json:"designation"`
	Institution string `json:"institution"`
	Category    string `json:"category"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

func (a *App) publicBadgeContact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	qr := r.PathValue("qr")
	if !admissionQRPattern.MatchString(qr) {
		fail(w, 404, "this is not a Bio Connect badge")
		return
	}
	// Badge codes cannot be guessed; the limit only slows bulk harvesting of
	// codes collected elsewhere. A venue network shares one address, so it is generous.
	if a.limited(r.Context(), "badge-contact:"+a.clientPeer(r), 3000, time.Hour) {
		fail(w, 429, "too many badge scans; try again shortly")
		return
	}
	c := badgeContact{QRID: qr}
	var shareEmail, sharePhone bool
	err := a.DB.QueryRow(r.Context(), `SELECT a.name,a.designation,r.institution,c.label,a.email,a.phone,a.share_email,a.share_phone FROM passes p
	 JOIN attendees a ON a.id=p.attendee_id JOIN registrations r ON r.id=p.registration_id JOIN categories c ON c.id=r.category_id
	 WHERE p.qr_id=$1 AND p.revoked_at IS NULL AND r.status='approved' AND a.removed_at IS NULL`, qr).
		Scan(&c.Name, &c.Designation, &c.Institution, &c.Category, &c.Email, &c.Phone, &shareEmail, &sharePhone)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "this badge is not active")
		return
	}
	if err != nil {
		fail(w, 503, "badge lookup unavailable; retry shortly")
		return
	}
	if !shareEmail {
		c.Email = ""
	}
	if !sharePhone {
		c.Phone = ""
	}
	respond(w, 200, c)
}
