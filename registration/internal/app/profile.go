package app

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// badgeURL is what a printed badge's QR encodes. A phone camera opens the
// holder's public profile instead of a web search for an opaque token, and
// ops scanners reduce it back to the token (see opsScanCode).
func (a *App) badgeURL(qrID string) string {
	return a.Config.BaseURL + "/p/" + qrID
}

var profilePage = template.Must(template.ParseFS(resources, "web/profile.html"))

// publicProfile shows only what the badge itself prints, so scanning a badge
// reveals nothing more than looking at it. Contact details stay private.
func (a *App) publicProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	var p struct{ Status, Name, Designation, Institution, Category string }
	p.Status = "missing"
	if qrID := r.PathValue("qr"); admissionQRPattern.MatchString(qrID) {
		err := a.DB.QueryRow(r.Context(), `SELECT a.name,a.designation,r.institution,c.label FROM passes p
		 JOIN attendees a ON a.id=p.attendee_id JOIN registrations r ON r.id=p.registration_id JOIN categories c ON c.id=r.category_id
		 WHERE p.qr_id=$1 AND p.revoked_at IS NULL AND r.status='approved' AND a.removed_at IS NULL`, qrID).Scan(&p.Name, &p.Designation, &p.Institution, &p.Category)
		switch {
		case err == nil:
			p.Status = "active"
		case !errors.Is(err, pgx.ErrNoRows):
			p.Status = "unavailable"
		}
	}
	switch p.Status {
	case "missing":
		w.WriteHeader(http.StatusNotFound)
	case "unavailable":
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = profilePage.Execute(w, p)
}
