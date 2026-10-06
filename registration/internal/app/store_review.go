package app

import (
	"bytes"
	"net/http"
	"net/url"

	"github.com/go-pdf/fpdf"
)

// Reserved example-domain identity cannot overlap with a deliverable attendee email.
const storeReviewEmail = "review@bioconnect.example"
const storeReviewQR = "STORE-REVIEW-NOT-VALID-FOR-ADMISSION"
const storeReviewPassID = "store-review"

func (a *App) isStoreReviewer(channel, identifier string) bool {
	return a.Config.StoreReviewCode != "" && channel == "email" && identifier == storeReviewEmail
}

func (a *App) storeReviewPasses(w http.ResponseWriter, r *http.Request, token string) {
	p := mobilePass{ID: storeReviewPassID, Name: "Store Review - Sample Attendee", Institution: "Bio Connect App Review", Designation: "Not valid for event admission", Category: "Sample Delegate", Number: "REVIEW - NOT VALID FOR ADMISSION", QRID: storeReviewQR,
		DownloadURL: a.Config.BaseURL + "/api/v1/mobile/review-pass.pdf?token=" + url.QueryEscape(token)}
	if err := a.DB.QueryRow(r.Context(), "SELECT review_share_email,review_share_phone FROM mobile_pass_sessions WHERE token_hash=$1", hash(token)).Scan(&p.ShareEmail, &p.SharePhone); err != nil {
		fail(w, 503, "passes unavailable")
		return
	}
	respond(w, 200, map[string]any{"passes": []mobilePass{p}, "checked_at": a.Now()})
}

// Download capability is restricted to this synthetic document, never real passes.
func (a *App) storeReviewPDF(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Authorization", "Bearer "+r.URL.Query().Get("token"))
	_, _, _, qr, ok := a.mobileSession(w, r)
	if !ok {
		return
	}
	if qr != storeReviewQR {
		fail(w, 404, "sample pass not found")
		return
	}
	p := fpdf.New("P", "mm", "A4", "")
	p.AddPage()
	p.SetFont("Helvetica", "B", 20)
	p.MultiCell(0, 12, "Bio Connect 4.0\nSTORE REVIEW SAMPLE\nNOT VALID FOR EVENT ADMISSION", "", "L", false)
	p.SetFont("Helvetica", "", 14)
	p.MultiCell(0, 10, "Sample Attendee\nBio Connect App Review\nThis synthetic pass is provided to review app functionality. It does not grant entry to the event.", "", "L", false)
	var b bytes.Buffer
	if err := p.Output(&b); err != nil {
		fail(w, 503, "sample download unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `inline; filename="bioconnect-review-sample.pdf"`)
	w.Write(b.Bytes())
}
