package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Speaker passes. Each speaker in the directory (the mobile content editor's
// speaker list) can hold one complimentary pass in the staff-only "speaker"
// category. Saving a speaker's email issues that pass without sending it;
// staff send it with the ordinary review actions, so delivery, resend,
// download and the audit trail all behave as for any other registration.

type speakerPassRow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Organization string `json:"organization"`
	ImageURL     string `json:"image_url"`
	Published    bool   `json:"published"`
	// Registration is null until a pass is issued. A cancelled registration is
	// still returned so staff can see why the speaker has no pass.
	Registration *struct {
		ID        string `json:"id"`
		Reference string `json:"reference"`
		Status    string `json:"status"`
	} `json:"registration"`
	Attendee *struct {
		ID              string `json:"id"`
		Email           string `json:"email"`
		Phone           string `json:"phone"`
		WhatsAppConsent bool   `json:"whatsapp_consent"`
	} `json:"attendee"`
	Pass *struct {
		ID     string `json:"id"`
		Number string `json:"number"`
	} `json:"pass"`
	// Delivery is the latest email delivery of the active pass, if any.
	Delivery *struct {
		Status string    `json:"status"`
		At     time.Time `json:"at"`
	} `json:"delivery"`
}

func (a *App) speakersAPI(w http.ResponseWriter, r *http.Request, staff, path string) {
	parts := strings.Split(path, "/")
	switch {
	case len(parts) == 1 && r.Method == "GET":
		rows, e := a.speakerPasses(r.Context())
		if e != nil {
			fail(w, 503, "speakers unavailable; please retry")
			return
		}
		respond(w, 200, map[string]any{"speakers": rows})
	case len(parts) == 2 && r.Method == "POST":
		var in struct {
			Email           string `json:"email"`
			Phone           string `json:"phone"`
			WhatsAppConsent bool   `json:"whatsapp_consent"`
		}
		if !decode(w, r, &in) {
			return
		}
		rid, e := a.SaveSpeakerContact(r.Context(), parts[1], in.Email, in.Phone, in.WhatsAppConsent, r.Header.Get("Idempotency-Key"), staff)
		if e != nil {
			fail(w, 400, publicError(e))
			return
		}
		respond(w, 200, map[string]string{"registration_id": rid})
	default:
		fail(w, 404, "not found")
	}
}

func (a *App) speakerPasses(ctx context.Context) ([]speakerPassRow, error) {
	rows, e := a.DB.Query(ctx, `SELECT s.id,s.name,s.role,s.organization,s.image_url,s.published,
		r.id,r.reference,r.status,at.id,at.email,at.phone,at.whatsapp_consent,p.id,p.number,j.status,j.updated_at
		FROM speakers s
		LEFT JOIN speaker_registrations sr ON sr.speaker_id=s.id
		LEFT JOIN registrations r ON r.id=sr.registration_id
		LEFT JOIN LATERAL (SELECT id,email,phone,whatsapp_consent FROM attendees
			WHERE registration_id=r.id AND removed_at IS NULL ORDER BY position LIMIT 1) at ON true
		LEFT JOIN passes p ON p.attendee_id=at.id AND p.revoked_at IS NULL
		LEFT JOIN LATERAL (SELECT status,updated_at FROM delivery_jobs
			WHERE pass_id=p.id AND purpose='pass' AND channel='email' ORDER BY created_at DESC LIMIT 1) j ON true
		ORDER BY s.position,s.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []speakerPassRow{}
	for rows.Next() {
		var s speakerPassRow
		var rid, ref, status, aid, email, phone, pid, number, job *string
		var consent *bool
		var at *time.Time
		if e = rows.Scan(&s.ID, &s.Name, &s.Role, &s.Organization, &s.ImageURL, &s.Published,
			&rid, &ref, &status, &aid, &email, &phone, &consent, &pid, &number, &job, &at); e != nil {
			return nil, e
		}
		if rid != nil {
			s.Registration = &struct {
				ID        string `json:"id"`
				Reference string `json:"reference"`
				Status    string `json:"status"`
			}{*rid, *ref, *status}
		}
		if aid != nil {
			s.Attendee = &struct {
				ID              string `json:"id"`
				Email           string `json:"email"`
				Phone           string `json:"phone"`
				WhatsAppConsent bool   `json:"whatsapp_consent"`
			}{*aid, *email, *phone, *consent}
		}
		if pid != nil {
			s.Pass = &struct {
				ID     string `json:"id"`
				Number string `json:"number"`
			}{*pid, *number}
		}
		if job != nil {
			s.Delivery = &struct {
				Status string    `json:"status"`
				At     time.Time `json:"at"`
			}{*job, *at}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SaveSpeakerContact records a speaker's email and phone. The first save
// issues their pass (approved, complimentary, not sent); later saves correct
// the pass holder's contact details, which cancels any delivery still queued
// to the old address. A speaker whose pass registration was cancelled gets a
// new one. It returns the registration holding the pass.
func (a *App) SaveSpeakerContact(ctx context.Context, speakerID, email, phone string, consent bool, key, staff string) (string, error) {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	// One speaker, one pass: concurrent first saves must not issue two.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "speaker-pass:"+speakerID); e != nil {
		return "", e
	}
	var name, role, org string
	if e = tx.QueryRow(ctx, "SELECT name,role,organization FROM speakers WHERE id=$1", speakerID).Scan(&name, &role, &org); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return "", errors.New("this speaker is no longer in the speaker list")
		}
		return "", e
	}
	var rid, status string
	var aid, aName, aDesignation *string
	e = tx.QueryRow(ctx, `SELECT r.id,r.status,at.id,at.name,at.designation FROM speaker_registrations sr
		JOIN registrations r ON r.id=sr.registration_id
		LEFT JOIN LATERAL (SELECT id,name,designation FROM attendees
			WHERE registration_id=r.id AND removed_at IS NULL ORDER BY position LIMIT 1) at ON true
		WHERE sr.speaker_id=$1`, speakerID).Scan(&rid, &status, &aid, &aName, &aDesignation)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return "", e
	}
	if e == nil && status != "cancelled" && aid != nil {
		// The pass exists: correct its holder. Name and designation stay as
		// issued, since staff may have corrected them on the registration.
		if e = tx.Commit(ctx); e != nil {
			return "", e
		}
		return rid, a.UpdateAttendee(ctx, rid, *aid, staff, Attendee{Name: *aName, Designation: *aDesignation, Email: email, Phone: phone, WhatsAppConsent: consent})
	}
	if org = strings.TrimSpace(org); org == "" {
		org = "Speaker"
	}
	if role = strings.TrimSpace(role); role == "" {
		role = "Speaker"
	}
	in := StaffRegistrationInput{
		RegistrationInput: RegistrationInput{
			CategoryID:  "speaker",
			Institution: clip(org, 180),
			ContactName: clip(name, 120),
			Email:       email,
			Phone:       phone,
			Attendees:   []Attendee{{Name: clip(name, 120), Email: email, Phone: phone, Designation: clip(role, 180), WhatsAppConsent: consent}},
		},
		Payment: "complimentary",
		Note:    "Speaker pass",
	}
	if rid, e = a.staffCreate(ctx, tx, in, key, nil, staff); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO speaker_registrations(speaker_id,registration_id) VALUES($1,$2)
		ON CONFLICT(speaker_id) DO UPDATE SET registration_id=EXCLUDED.registration_id,created_at=now()`, speakerID, rid); e != nil {
		return "", e
	}
	return rid, tx.Commit(ctx)
}

// clip shortens s to at most n runes, so directory text longer than a
// registration field allows still fits on the pass.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n]))
	}
	return s
}
