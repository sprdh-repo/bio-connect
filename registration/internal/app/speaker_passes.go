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
// category. Staff first save a speaker's contact, which issues nothing; they
// then issue the pass (not sent), and send it with the ordinary review
// actions, so delivery, resend, download and the audit trail all behave as
// for any other registration.

type speakerPassRow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Organization string `json:"organization"`
	ImageURL     string `json:"image_url"`
	Published    bool   `json:"published"`
	// Contact is the saved email and phone, null until staff add them.
	Contact *struct {
		Email           string `json:"email"`
		Phone           string `json:"phone"`
		WhatsAppConsent bool   `json:"whatsapp_consent"`
	} `json:"contact"`
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
	// Delivery and WhatsApp are the latest email and WhatsApp deliveries of
	// the active pass, if any.
	Delivery *speakerDelivery `json:"delivery"`
	WhatsApp *speakerDelivery `json:"whatsapp"`
}

type speakerDelivery struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
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
		if e := a.SaveSpeakerContact(r.Context(), parts[1], in.Email, in.Phone, in.WhatsAppConsent, staff); e != nil {
			fail(w, 400, publicError(e))
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
	case len(parts) == 3 && parts[2] == "pass" && r.Method == "POST":
		rid, e := a.IssueSpeakerPass(r.Context(), parts[1], r.Header.Get("Idempotency-Key"), staff)
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
		c.email,c.phone,c.whatsapp_consent,
		r.id,r.reference,r.status,at.id,at.email,at.phone,at.whatsapp_consent,p.id,p.number,j.status,j.updated_at,wa.status,wa.updated_at
		FROM speakers s
		LEFT JOIN speaker_contacts c ON c.speaker_id=s.id
		LEFT JOIN speaker_registrations sr ON sr.speaker_id=s.id
		LEFT JOIN registrations r ON r.id=sr.registration_id
		LEFT JOIN LATERAL (SELECT id,email,phone,whatsapp_consent FROM attendees
			WHERE registration_id=r.id AND removed_at IS NULL ORDER BY position LIMIT 1) at ON true
		LEFT JOIN passes p ON p.attendee_id=at.id AND p.revoked_at IS NULL
		LEFT JOIN LATERAL (SELECT status,updated_at FROM delivery_jobs
			WHERE pass_id=p.id AND purpose='pass' AND channel='email' ORDER BY created_at DESC LIMIT 1) j ON true
		LEFT JOIN LATERAL (SELECT status,updated_at FROM delivery_jobs
			WHERE pass_id=p.id AND purpose='pass' AND channel='whatsapp' ORDER BY created_at DESC LIMIT 1) wa ON true
		ORDER BY s.position,s.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []speakerPassRow{}
	for rows.Next() {
		var s speakerPassRow
		var cEmail, cPhone, rid, ref, status, aid, email, phone, pid, number, job, waJob *string
		var cConsent, consent *bool
		var at, waAt *time.Time
		if e = rows.Scan(&s.ID, &s.Name, &s.Role, &s.Organization, &s.ImageURL, &s.Published, &cEmail, &cPhone, &cConsent,
			&rid, &ref, &status, &aid, &email, &phone, &consent, &pid, &number, &job, &at, &waJob, &waAt); e != nil {
			return nil, e
		}
		if cEmail != nil {
			s.Contact = &struct {
				Email           string `json:"email"`
				Phone           string `json:"phone"`
				WhatsAppConsent bool   `json:"whatsapp_consent"`
			}{*cEmail, *cPhone, *cConsent}
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
			s.Delivery = &speakerDelivery{*job, *at}
		}
		if waJob != nil {
			s.WhatsApp = &speakerDelivery{*waJob, *waAt}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SaveSpeakerContact records a speaker's email and phone. It issues nothing.
// When the speaker already holds a pass, its holder is corrected too, which
// cancels any delivery still queued to the old address.
func (a *App) SaveSpeakerContact(ctx context.Context, speakerID, email, phone string, consent bool, staff string) error {
	email, phone = strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(phone)
	if !validEmail(email) {
		return errors.New("enter a valid email address")
	}
	if !validPhone(phone, true) {
		return errors.New("phone must be international, like +919876543210")
	}
	if consent && phone == "" {
		return errors.New("WhatsApp delivery needs a phone number")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "speaker-pass:"+speakerID); e != nil {
		return e
	}
	if e = speakerExists(ctx, tx, speakerID); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO speaker_contacts(speaker_id,email,phone,whatsapp_consent,updated_by) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(speaker_id) DO UPDATE SET email=EXCLUDED.email,phone=EXCLUDED.phone,whatsapp_consent=EXCLUDED.whatsapp_consent,updated_by=EXCLUDED.updated_by,updated_at=now()`,
		speakerID, email, phone, consent, staff); e != nil {
		return e
	}
	if e = audit(ctx, tx, staff, "", "speaker_contact_saved", speakerID+" "+email); e != nil {
		return e
	}
	rid, aid, name, designation, e := activeSpeakerPass(ctx, tx, speakerID)
	if e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil || rid == "" {
		return e
	}
	// Name and designation stay as issued, since staff may have corrected them on the registration.
	return a.UpdateAttendee(ctx, rid, aid, staff, Attendee{Name: name, Designation: designation, Email: email, Phone: phone, WhatsAppConsent: consent})
}

// IssueSpeakerPass issues the speaker's complimentary pass from their saved
// contact, approved and not sent. A speaker who already holds a pass keeps it;
// one whose pass registration was cancelled gets a new one.
func (a *App) IssueSpeakerPass(ctx context.Context, speakerID, key, staff string) (string, error) {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	// One speaker, one pass: concurrent issues must not create two.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "speaker-pass:"+speakerID); e != nil {
		return "", e
	}
	if e = speakerExists(ctx, tx, speakerID); e != nil {
		return "", e
	}
	rid, _, _, _, e := activeSpeakerPass(ctx, tx, speakerID)
	if e != nil || rid != "" {
		return rid, e
	}
	var name, role, org, email, phone string
	var consent bool
	e = tx.QueryRow(ctx, `SELECT s.name,s.role,s.organization,c.email,c.phone,c.whatsapp_consent FROM speakers s
		JOIN speaker_contacts c ON c.speaker_id=s.id WHERE s.id=$1`, speakerID).Scan(&name, &role, &org, &email, &phone, &consent)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", errors.New("save this speaker's email before issuing their pass")
	}
	if e != nil {
		return "", e
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

func speakerExists(ctx context.Context, tx pgx.Tx, speakerID string) error {
	var ok bool
	if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM speakers WHERE id=$1)", speakerID).Scan(&ok); e != nil {
		return e
	}
	if !ok {
		return errors.New("this speaker is no longer in the speaker list")
	}
	return nil
}

// activeSpeakerPass returns the speaker's pass registration and its holder,
// or an empty rid when they hold no pass (none issued, or cancelled).
func activeSpeakerPass(ctx context.Context, tx pgx.Tx, speakerID string) (rid, aid, name, designation string, e error) {
	var status string
	var a, n, d *string
	e = tx.QueryRow(ctx, `SELECT r.id,r.status,at.id,at.name,at.designation FROM speaker_registrations sr
		JOIN registrations r ON r.id=sr.registration_id
		LEFT JOIN LATERAL (SELECT id,name,designation FROM attendees
			WHERE registration_id=r.id AND removed_at IS NULL ORDER BY position LIMIT 1) at ON true
		WHERE sr.speaker_id=$1`, speakerID).Scan(&rid, &status, &a, &n, &d)
	if errors.Is(e, pgx.ErrNoRows) || (e == nil && (status == "cancelled" || a == nil)) {
		return "", "", "", "", nil
	}
	if e != nil {
		return "", "", "", "", e
	}
	return rid, *a, *n, *d, nil
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
