package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The mobile app is driven by three stores: the app_content document
// (event, menus, copy, themes, launch, leadership, sponsors, partners), the
// event_guide document (sessions, activities, FAQs, venue) and the speakers
// table. Staff edit all three together from /admin?view=mobile, guarded by
// app_content's revision.

type contentEvent struct {
	Title            string   `json:"title"`
	Tagline          string   `json:"tagline"`
	HeroTitle        string   `json:"hero_title"`
	Description      string   `json:"description"`
	StartDate        string   `json:"start_date"`
	EndDate          string   `json:"end_date"`
	Venue            string   `json:"venue"`
	City             string   `json:"city"`
	BrochureURL      string   `json:"brochure_url"`
	SponsorshipEmail string   `json:"sponsorship_email"`
	PrivacyURL       string   `json:"privacy_url"`
	Hidden           []string `json:"hidden"`
}
type contentTheme struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Published   bool   `json:"published"`
}
type contentNamedDescription struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}
type contentProductLaunch struct {
	Eyebrow     string                    `json:"eyebrow"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Deadline    string                    `json:"deadline"`
	Eligibility []contentNamedDescription `json:"eligibility"`
	FocusAreas  []string                  `json:"focus_areas"`
	ApplyURL    string                    `json:"apply_url"`
	ApplyLabel  string                    `json:"apply_label"`
	Hidden      []string                  `json:"hidden"`
}
type contentLeader struct {
	Name           string `json:"name"`
	Role           string `json:"role"`
	Badge          string `json:"badge"`
	ImageURL       string `json:"image_url"`
	ImageCredit    string `json:"image_credit"`
	ImageCreditURL string `json:"image_credit_url"`
	Published      bool   `json:"published"`
}
type contentNote struct {
	Name string `json:"name"`
	Note string `json:"note"`
}
type contentMember struct {
	Role         string `json:"role"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Published    bool   `json:"published"`
}
type contentCommittee struct {
	Title     string          `json:"title"`
	OrderNote string          `json:"order_note"`
	Members   []contentMember `json:"members"`
}
type contentLeadership struct {
	Intro        string           `json:"intro"`
	AdvisoryNote string           `json:"advisory_note"`
	ConvenedBy   *contentNote     `json:"convened_by"`
	People       []contentLeader  `json:"people"`
	Committee    contentCommittee `json:"committee"`
	Hidden       []string         `json:"hidden"`
}
type contentSponsor struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
	WebsiteURL  string `json:"website_url"`
	Published   bool   `json:"published"`
}
type contentPartner struct {
	Name       string `json:"name"`
	LogoURL    string `json:"logo_url"`
	WebsiteURL string `json:"website_url"`
	Published  bool   `json:"published"`
}

// menuItem is one entry in an app menu. Key names an app destination, or
// "link" for an external URL. Blank titles fall back to the app's own label.
type menuItem struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	URL       string `json:"url"`
	Published bool   `json:"published"`
}

type mobileContent struct {
	Event               contentEvent          `json:"event"`
	Themes              []contentTheme        `json:"themes"`
	ProgrammeHighlights []string              `json:"programme_highlights"`
	ProductLaunch       contentProductLaunch  `json:"product_launch"`
	Leadership          contentLeadership     `json:"leadership"`
	SponsorsIntro       string                `json:"sponsors_intro"`
	Sponsors            []contentSponsor      `json:"sponsors"`
	EcosystemPartners   []contentPartner      `json:"ecosystem_partners"`
	Menus               map[string][]menuItem `json:"menus"`
	// Copy overrides the app's headings and short texts by key, such as
	// "sessions.title". Blank or missing keys keep the app's built-in wording.
	Copy map[string]string `json:"copy"`
}

// mobileEditor is the console's view: everything the app shows, in one save.
type mobileEditor struct {
	Revision int            `json:"revision"`
	Content  mobileContent  `json:"content"`
	Guide    eventGuide     `json:"guide"`
	Speakers []adminSpeaker `json:"speakers"`
}
type adminSpeaker struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Organization string `json:"organization"`
	ImageURL     string `json:"image_url"`
	LinkedIn     string `json:"linkedin"`
	Published    bool   `json:"published"`
}

var (
	menuNames        = []string{"tabs", "home_shortcuts", "home_links", "guide"}
	menuDestinations = []string{"sessions", "speakers", "venue", "activities", "faqs", "exhibitors", "my_passes", "registration", "brochure", "product_launch", "sponsors", "leadership", "explore", "privacy", "link"}
	// Old app builds parse these, so they can never be withheld.
	eventHideable      = []string{"tagline", "hero_title", "description", "venue", "city", "brochure_url", "sponsorship_email", "privacy_url"}
	launchHideable     = []string{"eyebrow", "title", "description", "deadline", "eligibility", "focus_areas", "apply_url"}
	leadershipHideable = []string{"intro", "advisory_note", "convened_by"}
	copyKey            = regexp.MustCompile(`^[a-z_]{1,30}\.[a-z_]{1,40}$`)
	speakerID          = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func checkHidden(kind string, hidden, allowed []string) error {
	seen := map[string]bool{}
	for _, name := range hidden {
		if !slices.Contains(allowed, name) || seen[name] {
			return fmt.Errorf("%s cannot hide %q", kind, name)
		}
		seen[name] = true
	}
	return nil
}

// optionalHTTPS accepts a blank value or a public HTTPS URL.
func optionalHTTPS(s string) bool {
	if s == "" {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}

// textLimit checks a group of fields against one maximum length.
func textLimit(kind string, limit int, fields ...string) error {
	for _, s := range fields {
		if len(s) > limit {
			return fmt.Errorf("%s text exceeds %d characters", kind, limit)
		}
	}
	return nil
}

func (c mobileContent) validate() error {
	e := c.Event
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("event title is required")
	}
	start, err1 := time.Parse(time.DateOnly, e.StartDate)
	end, err2 := time.Parse(time.DateOnly, e.EndDate)
	if err1 != nil || err2 != nil || end.Before(start) {
		return fmt.Errorf("event dates must be valid, with the end on or after the start")
	}
	if err := textLimit("event", 2000, e.Title, e.Tagline, e.HeroTitle, e.Description, e.Venue, e.City, e.SponsorshipEmail); err != nil {
		return err
	}
	if !optionalHTTPS(e.BrochureURL) || !optionalHTTPS(e.PrivacyURL) {
		return fmt.Errorf("brochure and privacy links must be public HTTPS URLs")
	}
	if e.SponsorshipEmail != "" && !validEmail(e.SponsorshipEmail) {
		return fmt.Errorf("enter a valid sponsorship email")
	}
	if err := checkHidden("event", e.Hidden, eventHideable); err != nil {
		return err
	}
	if len(c.Themes) > 20 || len(c.ProgrammeHighlights) > 50 || len(c.Sponsors) > 100 || len(c.EcosystemPartners) > 100 {
		return fmt.Errorf("too many themes, highlights, sponsors or partners")
	}
	for _, t := range c.Themes {
		if strings.TrimSpace(t.Title) == "" {
			return fmt.Errorf("every theme needs a title")
		}
		// Themes ship with bundled artwork; new ones may use a hosted image.
		if !strings.HasPrefix(t.Image, "assets/images/") && (t.Image == "" || !optionalHTTPS(t.Image)) {
			return fmt.Errorf("theme %q needs a bundled image path or an HTTPS image URL", t.Title)
		}
		if err := textLimit("theme", 1000, t.Title, t.Description, t.Image); err != nil {
			return err
		}
	}
	for _, h := range c.ProgrammeHighlights {
		if strings.TrimSpace(h) == "" || len(h) > 120 {
			return fmt.Errorf("programme highlights must be 1 to 120 characters")
		}
	}
	p := c.ProductLaunch
	if err := textLimit("product launch", 2000, p.Eyebrow, p.Title, p.Description, p.Deadline, p.ApplyLabel); err != nil {
		return err
	}
	if !optionalHTTPS(p.ApplyURL) {
		return fmt.Errorf("product launch link must be a public HTTPS URL")
	}
	if len(p.Eligibility) > 20 || len(p.FocusAreas) > 30 {
		return fmt.Errorf("too many product launch entries")
	}
	for _, item := range p.Eligibility {
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("every product launch eligibility entry needs a title")
		}
		if err := textLimit("product launch", 2000, item.Title, item.Description); err != nil {
			return err
		}
	}
	for _, f := range p.FocusAreas {
		if strings.TrimSpace(f) == "" || len(f) > 120 {
			return fmt.Errorf("focus areas must be 1 to 120 characters")
		}
	}
	if err := checkHidden("product launch", p.Hidden, launchHideable); err != nil {
		return err
	}
	l := c.Leadership
	if err := textLimit("leadership", 4000, l.Intro, l.AdvisoryNote, l.Committee.Title, l.Committee.OrderNote); err != nil {
		return err
	}
	if l.ConvenedBy != nil {
		if err := textLimit("leadership", 2000, l.ConvenedBy.Name, l.ConvenedBy.Note); err != nil {
			return err
		}
	}
	if len(l.People) > 50 || len(l.Committee.Members) > 100 {
		return fmt.Errorf("too many leaders or committee members")
	}
	for _, person := range l.People {
		if strings.TrimSpace(person.Name) == "" {
			return fmt.Errorf("every leader needs a name")
		}
		if err := textLimit("leader", 2000, person.Name, person.Role, person.Badge, person.ImageCredit); err != nil {
			return err
		}
		if !optionalHTTPS(person.ImageURL) || !optionalHTTPS(person.ImageCreditURL) {
			return fmt.Errorf("leader %q links must be public HTTPS URLs", person.Name)
		}
	}
	for _, m := range l.Committee.Members {
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("every committee member needs a name")
		}
		if err := textLimit("committee", 300, m.Role, m.Name, m.Organization); err != nil {
			return err
		}
	}
	if err := checkHidden("leadership", l.Hidden, leadershipHideable); err != nil {
		return err
	}
	if err := textLimit("sponsors", 4000, c.SponsorsIntro); err != nil {
		return err
	}
	for _, s := range c.Sponsors {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("every sponsor needs a name")
		}
		if err := textLimit("sponsor", 4000, s.Name, s.Category, s.Description); err != nil {
			return err
		}
		if !optionalHTTPS(s.LogoURL) || !optionalHTTPS(s.WebsiteURL) {
			return fmt.Errorf("sponsor %q links must be public HTTPS URLs", s.Name)
		}
	}
	for _, s := range c.EcosystemPartners {
		if strings.TrimSpace(s.Name) == "" || len(s.Name) > 300 {
			return fmt.Errorf("every partner needs a name (maximum 300 characters)")
		}
		if !optionalHTTPS(s.LogoURL) || !optionalHTTPS(s.WebsiteURL) {
			return fmt.Errorf("partner %q links must be public HTTPS URLs", s.Name)
		}
	}
	for name, items := range c.Menus {
		if !slices.Contains(menuNames, name) {
			return fmt.Errorf("unknown app menu %q", name)
		}
		if len(items) > 30 {
			return fmt.Errorf("the %s menu has too many entries", name)
		}
		for _, item := range items {
			if !slices.Contains(menuDestinations, item.Key) {
				return fmt.Errorf("unknown app destination %q", item.Key)
			}
			if name == "tabs" && item.Key != "sessions" && item.Key != "speakers" {
				return fmt.Errorf("only Sessions and Speakers can be bottom tabs")
			}
			if err := textLimit("menu", 120, item.Title, item.Subtitle); err != nil {
				return err
			}
			if item.Key == "link" {
				if strings.TrimSpace(item.Title) == "" {
					return fmt.Errorf("custom links need a title")
				}
				if !validMenuLink(item.URL) {
					return fmt.Errorf("custom link %q needs an HTTPS, mailto: or tel: address", item.Title)
				}
			} else if item.URL != "" {
				return fmt.Errorf("only custom links take a URL")
			}
		}
	}
	if len(c.Copy) > 200 {
		return fmt.Errorf("too many text overrides")
	}
	for key, value := range c.Copy {
		if !copyKey.MatchString(key) || len(value) > 600 {
			return fmt.Errorf("invalid text override %q", key)
		}
	}
	return nil
}

func validMenuLink(s string) bool {
	u, err := url.Parse(s)
	if err != nil || s == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return optionalHTTPS(s)
	case "mailto":
		return validEmail(u.Opaque)
	case "tel":
		return u.Opaque != "" && helpPhone(u.Opaque)
	}
	return false
}

func validateSpeakers(items []adminSpeaker) error {
	if len(items) > 500 {
		return fmt.Errorf("too many speakers")
	}
	seen := map[string]bool{}
	for _, s := range items {
		if len(s.ID) > 80 || !speakerID.MatchString(s.ID) || seen[s.ID] {
			return fmt.Errorf("speaker %q needs a unique lowercase ID", s.Name)
		}
		seen[s.ID] = true
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("every speaker needs a name")
		}
		if err := textLimit("speaker", 300, s.Name, s.Role, s.Organization); err != nil {
			return err
		}
		if !optionalHTTPS(s.ImageURL) || !optionalHTTPS(s.LinkedIn) {
			return fmt.Errorf("speaker %q links must be public HTTPS URLs", s.Name)
		}
	}
	return nil
}

func normalizeContent(c *mobileContent) {
	if c.Themes == nil {
		c.Themes = []contentTheme{}
	}
	if c.ProgrammeHighlights == nil {
		c.ProgrammeHighlights = []string{}
	}
	if c.ProductLaunch.Eligibility == nil {
		c.ProductLaunch.Eligibility = []contentNamedDescription{}
	}
	if c.ProductLaunch.FocusAreas == nil {
		c.ProductLaunch.FocusAreas = []string{}
	}
	if c.Leadership.People == nil {
		c.Leadership.People = []contentLeader{}
	}
	if c.Leadership.Committee.Members == nil {
		c.Leadership.Committee.Members = []contentMember{}
	}
	if c.Sponsors == nil {
		c.Sponsors = []contentSponsor{}
	}
	if c.EcosystemPartners == nil {
		c.EcosystemPartners = []contentPartner{}
	}
	if c.Menus == nil {
		c.Menus = map[string][]menuItem{}
	}
	if c.Copy == nil {
		c.Copy = map[string]string{}
	}
	for _, hidden := range []*[]string{&c.Event.Hidden, &c.ProductLaunch.Hidden, &c.Leadership.Hidden} {
		if *hidden == nil {
			*hidden = []string{}
		}
	}
}

func normalizeGuide(g *eventGuide) {
	// Always return JSON arrays, including for a newly emptied section.
	if g.Sessions == nil {
		g.Sessions = []guideSession{}
	}
	if g.Activities == nil {
		g.Activities = []guideActivity{}
	}
	if g.FAQs == nil {
		g.FAQs = []guideFAQ{}
	}
	if g.Venue.Hidden == nil {
		g.Venue.Hidden = []string{}
	}
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func loadMobileContent(ctx context.Context, q queryer) (c mobileContent, revision int, published bool, err error) {
	var raw []byte
	if err = q.QueryRow(ctx, `SELECT document,revision,published FROM app_content WHERE id='mobile'`).Scan(&raw, &revision, &published); err != nil {
		return
	}
	err = json.Unmarshal(raw, &c)
	normalizeContent(&c)
	return
}

func loadAdminSpeakers(ctx context.Context, q queryer) ([]adminSpeaker, error) {
	rows, err := q.Query(ctx, `SELECT id,name,role,organization,image_url,linkedin,published FROM speakers ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []adminSpeaker{}
	for rows.Next() {
		var s adminSpeaker
		if err := rows.Scan(&s.ID, &s.Name, &s.Role, &s.Organization, &s.ImageURL, &s.LinkedIn, &s.Published); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (a *App) loadMobileEditor(ctx context.Context) (mobileEditor, error) {
	var m mobileEditor
	var err error
	if m.Content, m.Revision, _, err = loadMobileContent(ctx, a.DB); err != nil {
		return m, err
	}
	if m.Guide, err = a.loadEventGuide(ctx); err != nil {
		return m, err
	}
	normalizeGuide(&m.Guide)
	m.Speakers, err = loadAdminSpeakers(ctx, a.DB)
	return m, err
}

func (a *App) adminMobileContent(w http.ResponseWriter, r *http.Request, p principal) {
	switch r.Method {
	case "GET":
		m, err := a.loadMobileEditor(r.Context())
		if err != nil {
			fail(w, 503, "mobile content unavailable")
			return
		}
		respond(w, 200, m)
		return
	case "PUT":
	default:
		w.Header().Set("Allow", "GET, PUT")
		fail(w, 405, "method not allowed")
		return
	}
	var m mobileEditor
	if !decodeLimit(w, r, &m, 2<<20) {
		return
	}
	if m.Speakers == nil {
		m.Speakers = []adminSpeaker{}
	}
	normalizeContent(&m.Content)
	normalizeGuide(&m.Guide)
	for _, err := range []error{m.Content.validate(), m.Guide.validate(), validateSpeakers(m.Speakers)} {
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	content, err := json.Marshal(m.Content)
	if err != nil {
		fail(w, 400, "invalid content")
		return
	}
	guide, err := json.Marshal(eventGuide{Sessions: m.Guide.Sessions, Activities: m.Guide.Activities, FAQs: m.Guide.FAQs, Venue: m.Guide.Venue})
	if err != nil {
		fail(w, 400, "invalid guide")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		fail(w, 503, "could not save mobile content")
		return
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE app_content SET document=$1,revision=revision+1,updated_at=now() WHERE id='mobile' AND revision=$2`, content, m.Revision)
	if err != nil {
		fail(w, 503, "could not save mobile content")
		return
	}
	if result.RowsAffected() != 1 {
		fail(w, 409, "mobile content changed in another window; reload before editing again")
		return
	}
	if _, err = tx.Exec(ctx, `UPDATE event_guide SET document=$1,revision=revision+1,updated_at=now() WHERE id='mobile'`, guide); err != nil {
		fail(w, 503, "could not save mobile content")
		return
	}
	// Positions are unique, so clear the table and write the list in its new order.
	if _, err = tx.Exec(ctx, `DELETE FROM speakers`); err != nil {
		fail(w, 503, "could not save speakers")
		return
	}
	for i, s := range m.Speakers {
		if _, err = tx.Exec(ctx, `INSERT INTO speakers(id,name,role,organization,image_url,linkedin,position,published) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			s.ID, strings.TrimSpace(s.Name), s.Role, s.Organization, s.ImageURL, s.LinkedIn, i+1, s.Published); err != nil {
			fail(w, 503, "could not save speakers")
			return
		}
	}
	if err = audit(ctx, tx, p.ID, "", "mobile_content.update", fmt.Sprintf("revision %d", m.Revision+1)); err != nil {
		fail(w, 503, "could not save mobile content")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		fail(w, 503, "could not save mobile content")
		return
	}
	saved, err := a.loadMobileEditor(ctx)
	if err != nil {
		fail(w, 503, "saved; reload to continue editing")
		return
	}
	respond(w, 200, saved)
}

// publicView prepares stored content for the app: list entries switched off
// with "published": false are dropped, fields named in "hidden" are blanked,
// and both control keys are removed. Blanking rather than deleting keeps the
// shape that released app versions require.
func publicView(v any) any {
	switch value := v.(type) {
	case map[string]any:
		hidden, _ := value["hidden"].([]any)
		delete(value, "hidden")
		delete(value, "published")
		for key, item := range value {
			value[key] = publicView(item)
		}
		for _, name := range hidden {
			key, _ := name.(string)
			switch value[key].(type) {
			case string:
				value[key] = ""
			case []any:
				value[key] = []any{}
			case map[string]any:
				value[key] = nil
			}
		}
		return value
	case []any:
		out := make([]any, 0, len(value))
		for _, item := range value {
			if m, ok := item.(map[string]any); ok && m["published"] == false {
				continue
			}
			out = append(out, publicView(item))
		}
		return out
	}
	return v
}

func toJSONMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(raw, &out)
}

func (a *App) publicAppContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	ctx := r.Context()
	content, _, published, err := loadMobileContent(ctx, a.DB)
	if err != nil || !published {
		fail(w, http.StatusServiceUnavailable, "app content unavailable; please retry")
		return
	}
	speakers, err := a.loadPublicSpeakers(ctx)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "app content unavailable; please retry")
		return
	}
	guide, err := a.loadEventGuide(ctx)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "event guide unavailable; please retry")
		return
	}
	doc, err := toJSONMap(content)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "app content unavailable; please retry")
		return
	}
	if doc["event_guide"], err = toJSONMap(guide.public()); err != nil {
		fail(w, http.StatusServiceUnavailable, "event guide unavailable; please retry")
		return
	}
	doc = publicView(doc).(map[string]any)
	doc["speakers"] = speakers
	// App versions released before the sponsors list read one sponsor object.
	doc["sponsor"] = map[string]any{"name": "", "category": "", "description": "", "logo_url": "", "website_url": ""}
	if list, _ := doc["sponsors"].([]any); len(list) > 0 {
		doc["sponsor"] = list[0]
	}
	respond(w, http.StatusOK, doc)
}
