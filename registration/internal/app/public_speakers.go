package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// Short-lived, so an editor save reaches the website within a minute.
	w.Header().Set("Cache-Control", "public, max-age=60")
	respond(w, http.StatusOK, map[string]any{"speakers": speakers, "version": speakersVersion(speakers)})
}

// speakersVersion fingerprints the published list. The website build stamps
// it into speakers.html, and the page only re-renders its cards when the live
// list carries a different version.
func speakersVersion(speakers []publicSpeaker) string {
	b, _ := json.Marshal(speakers)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func (a *App) loadPublicSpeakers(ctx context.Context) ([]publicSpeaker, error) {
	rows, err := a.DB.Query(ctx, `SELECT id,name,role,organization,image_url,linkedin
 FROM speakers WHERE published ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	speakers := make([]publicSpeaker, 0)
	for rows.Next() {
		var item publicSpeaker
		if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.Organization, &item.ImageURL, &item.LinkedIn); err != nil {
			return nil, err
		}
		speakers = append(speakers, item)
	}
	return speakers, rows.Err()
}
