package app

import (
	"encoding/json"
	"net/http"
)

func (a *App) publicAppContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var raw []byte
	if err := a.DB.QueryRow(r.Context(), `SELECT document FROM app_content WHERE id='mobile' AND published`).Scan(&raw); err != nil {
		fail(w, http.StatusServiceUnavailable, "app content unavailable; please retry")
		return
	}
	var content map[string]any
	if err := json.Unmarshal(raw, &content); err != nil {
		fail(w, http.StatusServiceUnavailable, "app content unavailable; please retry")
		return
	}
	speakers, err := a.loadPublicSpeakers(r.Context())
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "app content unavailable; please retry")
		return
	}
	guide, err := a.loadEventGuide(r.Context())
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "event guide unavailable; please retry")
		return
	}
	content["event_guide"] = guide.public()
	content["speakers"] = speakers
	respond(w, http.StatusOK, content)
}
