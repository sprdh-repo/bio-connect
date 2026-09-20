package app

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// This is deliberately separate from Registration: contact, payment and attendee
// data must never enter the public directory's response.
type publicExhibitor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
}

func (a *App) publicExhibitors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rows, err := a.DB.Query(r.Context(), `SELECT r.institution, r.description, COALESCE(f.id,'')
 FROM registrations r JOIN categories c ON c.id=r.category_id
 LEFT JOIN LATERAL (SELECT id FROM files WHERE registration_id=r.id AND kind='logo'
 ORDER BY created_at DESC,id DESC LIMIT 1) f ON true
 WHERE c.kind='exhibitor' AND r.status='approved'
 ORDER BY lower(r.institution),r.id`)
	if err != nil {
		fail(w, 503, "exhibitors unavailable; please retry")
		return
	}
	defer rows.Close()
	exhibitors := make([]publicExhibitor, 0)
	for rows.Next() {
		var item publicExhibitor
		var logo string
		if err := rows.Scan(&item.Name, &item.Description, &logo); err != nil {
			fail(w, 503, "exhibitors unavailable; please retry")
			return
		}
		if logo != "" {
			item.LogoURL = "/api/v1/public/exhibitors/logos/" + logo
		}
		exhibitors = append(exhibitors, item)
	}
	if rows.Err() != nil {
		fail(w, 503, "exhibitors unavailable; please retry")
		return
	}
	respond(w, 200, map[string]any{"exhibitors": exhibitors})
}

func (a *App) publicExhibitorLogo(w http.ResponseWriter, r *http.Request) {
	// Check current approval on every request, including previously shared URLs.
	var key, mime string
	err := a.DB.QueryRow(r.Context(), `SELECT f.object_key,f.mime FROM files f
 JOIN registrations r ON r.id=f.registration_id JOIN categories c ON c.id=r.category_id
 WHERE f.id=$1 AND f.kind='logo' AND c.kind='exhibitor' AND r.status='approved'`, r.PathValue("file")).Scan(&key, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "logo not found")
		return
	}
	if err != nil {
		fail(w, 503, "logo unavailable; please retry")
		return
	}
	if mime != "image/png" && mime != "image/jpeg" {
		fail(w, 404, "logo not found")
		return
	}
	a.serveInline(w, r, key, mime)
}
