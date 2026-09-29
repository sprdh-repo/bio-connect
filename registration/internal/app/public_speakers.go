package app

import (
	"context"
	"net/http"
)

type publicSpeaker struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Organization string `json:"organization"`
	ImageURL     string `json:"image_url"`
	LinkedIn     string `json:"linkedin"`
}

func (a *App) publicSpeakers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	speakers, err := a.loadPublicSpeakers(r.Context())
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "speakers unavailable; please retry")
		return
	}
	respond(w, http.StatusOK, map[string]any{"speakers": speakers})
}

func (a *App) loadPublicSpeakers(ctx context.Context) ([]publicSpeaker, error) {
	rows, err := a.DB.Query(ctx, `SELECT id,name,role,organization,image_slug,linkedin
 FROM speakers WHERE published ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	speakers := make([]publicSpeaker, 0)
	for rows.Next() {
		var item publicSpeaker
		var imageSlug string
		if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.Organization, &imageSlug, &item.LinkedIn); err != nil {
			return nil, err
		}
		if imageSlug != "" {
			item.ImageURL = "https://bioconnect.kerala.gov.in/assets/speakers/" + imageSlug + ".webp"
		}
		speakers = append(speakers, item)
	}
	return speakers, rows.Err()
}
