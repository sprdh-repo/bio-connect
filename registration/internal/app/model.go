package app

import (
	"context"
	"crypto/cipher"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql web
var resources embed.FS

type App struct {
	DB      *pgxpool.Pool
	Config  Config
	aead    cipher.AEAD
	Storage Storage
	Now     func() time.Time
}

// categoryCode is the two-letter segment carried in every reference and pass
// number: BC4-EX-0007. It is the word already printed on the pass, so the code
// and the badge always agree. The three stall sizes are one EXHIBITOR series.
func categoryCode(catID string) string {
	switch catID {
	case "student":
		return "ST"
	case "startup":
		return "SP"
	case "faculty":
		return "FC"
	case "industry":
		return "IN"
	case "official":
		return "GO"
	case "organiser":
		return "OR"
	case "sponsor":
		return "SP"
	case "volunteer":
		return "VO"
	default:
		return "EX"
	}
}

type Category struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	EarlyPaise   int64  `json:"early_paise"`
	RegularPaise int64  `json:"regular_paise"`
	RosterCount  int    `json:"roster_count"`
	Open         bool   `json:"open"`
	FreeOnly     bool   `json:"free_only"`
	// FreeOpen says whether free registration links may use the category;
	// Open governs paid registration only.
	FreeOpen     bool  `json:"free_open"`
	PayablePaise int64 `json:"payable_paise"`
	// CouponEligible says whether any coupon applies, so the form can offer the
	// field without the client knowing codes or exclusions.
	CouponEligible bool `json:"coupon_eligible"`
}
type Attendee struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	Designation     string `json:"designation"`
	WhatsAppConsent bool   `json:"whatsapp_consent"`
}
type RegistrationInput struct {
	CategoryID  string     `json:"category_id"`
	Institution string     `json:"institution"`
	ContactName string     `json:"contact_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Description string     `json:"description"`
	CouponCode  string     `json:"coupon_code"`
	FreeToken   string     `json:"free_token"`
	Attendees   []Attendee `json:"attendees"`
}
type Registration struct {
	ID          string    `json:"id"`
	Reference   string    `json:"reference"`
	CategoryID  string    `json:"category_id"`
	Institution string    `json:"institution"`
	ContactName string    `json:"contact_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	QuotedPaise int64     `json:"quoted_paise"`
	CreatedAt   time.Time `json:"created_at"`
	ReviewNote  string    `json:"review_note"`
	CouponCode  string    `json:"coupon_code"`
	// Free covers both free-link and complimentary staff registrations;
	// Complimentary is the staff kind alone.
	Free          bool `json:"free_registration"`
	Complimentary bool `json:"complimentary"`
	// DiscountPercent is frozen when the registration is saved; non-zero means
	// the fee is paid by direct bank transfer.
	DiscountPercent int `json:"discount_percent"`
	// RosterCount is this registration's pass allowance; fewer attendees than
	// this means open places (see AddAttendee).
	RosterCount int        `json:"roster_count"`
	Attendees   []Attendee `json:"attendees"`
}

var phoneRE = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func validText(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len([]rune(s)) <= max && !strings.ContainsAny(s, "\x00\r\n")
}
func validEmail(s string) bool {
	m, e := mail.ParseAddress(s)
	return e == nil && m.Address == s && len(s) <= 254
}

// validPhone accepts an international number, or no number at all when staff
// enter the registration and the phone is optional.
func validPhone(s string, optional bool) bool {
	return phoneRE.MatchString(s) || (optional && s == "")
}

// attendeeOK normalises one attendee and reports whether their details are
// complete. Staff entry may leave the phone out, but WhatsApp delivery still
// needs one.
func attendeeOK(p *Attendee, phoneOptional bool) bool {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	p.Phone = strings.TrimSpace(p.Phone)
	return validText(p.Name, 120) && validText(p.Designation, 180) && validEmail(p.Email) &&
		validPhone(p.Phone, phoneOptional) && (p.Phone != "" || !p.WhatsAppConsent)
}

// attendeeError describes what attendeeOK requires.
func attendeeError(phoneOptional bool) error {
	if phoneOptional {
		return errors.New("each attendee needs name, designation and valid email; a phone is optional but must be international (+country code), and WhatsApp delivery needs one")
	}
	return errors.New("each attendee needs name, designation, valid email and international phone")
}

// validateInput checks a registration. staff marks one entered from the
// console: the phones are optional, and an exhibitor may be saved with fewer
// attendees than its passes, leaving the rest to be filled later.
func validateInput(in *RegistrationInput, c Category, staff bool) error {
	in.Institution = strings.TrimSpace(in.Institution)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	in.CouponCode = normalizeCoupon(in.CouponCode)
	if _, e := lookupCoupon(in.CouponCode, c.ID); e != nil {
		return e
	}
	if !validText(in.Institution, 180) || !validText(in.ContactName, 120) || !validEmail(in.Email) || !validPhone(in.Phone, staff) {
		if staff {
			return errors.New("provide institution, contact name and valid email; a contact phone is optional but must be international (+country code)")
		}
		return errors.New("provide institution, contact name, valid email and international phone number")
	}
	if staff && c.Kind == "exhibitor" {
		if len(in.Attendees) < 1 || len(in.Attendees) > c.RosterCount {
			return fmt.Errorf("add between 1 and %d attendees", c.RosterCount)
		}
	} else if len(in.Attendees) != c.RosterCount {
		return errors.New("attendee count must exactly match this category")
	}
	if len([]rune(in.Description)) > 3000 || (c.Kind == "exhibitor" && strings.TrimSpace(in.Description) == "") {
		return errors.New("exhibitors need a company description (maximum 3000 characters)")
	}
	seen := map[string]bool{}
	for i := range in.Attendees {
		p := &in.Attendees[i]
		if !attendeeOK(p, staff) {
			return attendeeError(staff)
		}
		if seen[p.Email] {
			return errors.New("attendee emails must be unique within a registration")
		}
		seen[p.Email] = true
	}
	if c.Kind == "delegate" {
		p := in.Attendees[0]
		in.ContactName = p.Name
		in.Email = p.Email
		in.Phone = p.Phone
	}
	return nil
}
func (a *App) Migrate(ctx context.Context) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(420062026)"); e != nil {
		return e
	}
	var haveTable bool
	if e = tx.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&haveTable); e != nil {
		return e
	}
	// 001_initial.sql creates schema_migrations itself; later files are plain
	// incremental steps, applied in filename order and recorded by their NNN prefix.
	if !haveTable {
		b, _ := resources.ReadFile("migrations/001_initial.sql")
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			return e
		}
	}
	// The table existing means the initial schema is in place, whether or not an
	// earlier build recorded it. Backfill so fresh and existing databases agree.
	if _, e = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(1) ON CONFLICT DO NOTHING"); e != nil {
		return e
	}
	entries, e := resources.ReadDir("migrations")
	if e != nil {
		return e
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") || len(name) < 4 {
			continue
		}
		v, ce := strconv.Atoi(name[:3])
		if ce != nil || v <= 1 {
			continue
		}
		var applied bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", v).Scan(&applied); e != nil {
			return e
		}
		if applied {
			continue
		}
		b, _ := resources.ReadFile("migrations/" + name)
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", v); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (a *App) categories(ctx context.Context) ([]Category, error) {
	rows, e := a.DB.Query(ctx, "SELECT id,kind,label,early_paise,regular_paise,roster_count,open,free_only,free_open FROM categories ORDER BY kind,CASE id WHEN 'industry' THEN 1 WHEN 'faculty' THEN 2 WHEN 'startup' THEN 3 WHEN 'student' THEN 4 WHEN 'official' THEN 5 WHEN 'organiser' THEN 6 WHEN 'sponsor' THEN 7 WHEN 'volunteer' THEN 8 ELSE 9 END,CASE WHEN kind='exhibitor' THEN early_paise END DESC")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if e = rows.Scan(&c.ID, &c.Kind, &c.Label, &c.EarlyPaise, &c.RegularPaise, &c.RosterCount, &c.Open, &c.FreeOnly, &c.FreeOpen); e != nil {
			return nil, e
		}
		c.PayablePaise = fee(c, a.Now())
		c.CouponEligible = couponEligible(c.ID)
		out = append(out, c)
	}
	return out, rows.Err()
}
func category(ctx context.Context, tx pgx.Tx, key string) (Category, error) {
	var c Category
	e := tx.QueryRow(ctx, "SELECT id,kind,label,early_paise,regular_paise,roster_count,open,free_only,free_open FROM categories WHERE id=$1 FOR SHARE", key).Scan(&c.ID, &c.Kind, &c.Label, &c.EarlyPaise, &c.RegularPaise, &c.RosterCount, &c.Open, &c.FreeOnly, &c.FreeOpen)
	return c, e
}
func audit(ctx context.Context, tx pgx.Tx, staff, reg, action, detail string) error {
	_, e := tx.Exec(ctx, "INSERT INTO audit_events(staff_id,registration_id,action,detail) VALUES(NULLIF($1,''),NULLIF($2,''),$3,$4)", staff, reg, action, detail)
	return e
}
func (a *App) registration(ctx context.Context, key string) (Registration, error) {
	var r Registration
	e := a.DB.QueryRow(ctx, `SELECT id,reference,category_id,institution,contact_name,email,phone,description,status,quoted_paise,created_at,review_note,coupon_code,free_link_id IS NOT NULL OR complimentary,complimentary,discount_percent,roster_count FROM registrations WHERE id=$1`, key).Scan(&r.ID, &r.Reference, &r.CategoryID, &r.Institution, &r.ContactName, &r.Email, &r.Phone, &r.Description, &r.Status, &r.QuotedPaise, &r.CreatedAt, &r.ReviewNote, &r.CouponCode, &r.Free, &r.Complimentary, &r.DiscountPercent, &r.RosterCount)
	if e != nil {
		return r, e
	}
	rows, e := a.DB.Query(ctx, "SELECT id,name,email,phone,designation,whatsapp_consent FROM attendees WHERE registration_id=$1 AND removed_at IS NULL ORDER BY position", key)
	if e != nil {
		return r, e
	}
	defer rows.Close()
	r.Attendees = []Attendee{}
	for rows.Next() {
		var p Attendee
		if e = rows.Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.Designation, &p.WhatsAppConsent); e != nil {
			return r, e
		}
		r.Attendees = append(r.Attendees, p)
	}
	return r, rows.Err()
}
func requestHash(v any) string { b, _ := json.Marshal(v); return hash(string(b)) }
