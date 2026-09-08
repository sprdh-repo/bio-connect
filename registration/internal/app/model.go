package app

import (
	"context"
	"crypto/cipher"
	"embed"
	"encoding/json"
	"errors"
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
	PayablePaise int64  `json:"payable_paise"`
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
	Attendees   []Attendee `json:"attendees"`
}
type Registration struct {
	ID          string     `json:"id"`
	Reference   string     `json:"reference"`
	CategoryID  string     `json:"category_id"`
	Institution string     `json:"institution"`
	ContactName string     `json:"contact_name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	QuotedPaise int64      `json:"quoted_paise"`
	CreatedAt   time.Time  `json:"created_at"`
	ReviewNote  string     `json:"review_note"`
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
func validateInput(in *RegistrationInput, c Category) error {
	in.Institution = strings.TrimSpace(in.Institution)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validText(in.Institution, 180) || !validText(in.ContactName, 120) || !validEmail(in.Email) || !phoneRE.MatchString(in.Phone) {
		return errors.New("provide institution, contact name, valid email and international phone number")
	}
	if len(in.Attendees) != c.RosterCount {
		return errors.New("attendee count must exactly match this category")
	}
	if len([]rune(in.Description)) > 3000 || (c.Kind == "exhibitor" && strings.TrimSpace(in.Description) == "") {
		return errors.New("exhibitors need a company description (maximum 3000 characters)")
	}
	seen := map[string]bool{}
	for i := range in.Attendees {
		p := &in.Attendees[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Email = strings.ToLower(strings.TrimSpace(p.Email))
		if !validText(p.Name, 120) || !validText(p.Designation, 180) || !validEmail(p.Email) || !phoneRE.MatchString(p.Phone) {
			return errors.New("each attendee needs name, designation, valid email and international phone")
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
	rows, e := a.DB.Query(ctx, "SELECT id,kind,label,early_paise,regular_paise,roster_count,open FROM categories ORDER BY kind,CASE WHEN kind='exhibitor' THEN early_paise END DESC,early_paise")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if e = rows.Scan(&c.ID, &c.Kind, &c.Label, &c.EarlyPaise, &c.RegularPaise, &c.RosterCount, &c.Open); e != nil {
			return nil, e
		}
		c.PayablePaise = fee(c, a.Now())
		out = append(out, c)
	}
	return out, rows.Err()
}
func category(ctx context.Context, tx pgx.Tx, key string) (Category, error) {
	var c Category
	e := tx.QueryRow(ctx, "SELECT id,kind,label,early_paise,regular_paise,roster_count,open FROM categories WHERE id=$1 FOR SHARE", key).Scan(&c.ID, &c.Kind, &c.Label, &c.EarlyPaise, &c.RegularPaise, &c.RosterCount, &c.Open)
	return c, e
}
func audit(ctx context.Context, tx pgx.Tx, staff, reg, action, detail string) error {
	_, e := tx.Exec(ctx, "INSERT INTO audit_events(staff_id,registration_id,action,detail) VALUES(NULLIF($1,''),NULLIF($2,''),$3,$4)", staff, reg, action, detail)
	return e
}
func (a *App) registration(ctx context.Context, key string) (Registration, error) {
	var r Registration
	e := a.DB.QueryRow(ctx, `SELECT id,reference,category_id,institution,contact_name,email,phone,description,status,quoted_paise,created_at,review_note FROM registrations WHERE id=$1`, key).Scan(&r.ID, &r.Reference, &r.CategoryID, &r.Institution, &r.ContactName, &r.Email, &r.Phone, &r.Description, &r.Status, &r.QuotedPaise, &r.CreatedAt, &r.ReviewNote)
	if e != nil {
		return r, e
	}
	rows, e := a.DB.Query(ctx, "SELECT id,name,email,phone,designation,whatsapp_consent FROM attendees WHERE registration_id=$1 ORDER BY position", key)
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
