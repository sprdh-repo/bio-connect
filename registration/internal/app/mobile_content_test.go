package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicViewWithholdsDraftsAndHiddenFields(t *testing.T) {
	doc := map[string]any{
		"event":    map[string]any{"title": "Bio Connect", "city": "Trivandrum", "hidden": []any{"city"}},
		"sponsors": []any{map[string]any{"name": "Draft", "published": false}, map[string]any{"name": "Live", "published": true}},
		"leadership": map[string]any{
			"convened_by": map[string]any{"name": "KLIP"},
			"people":      []any{},
			"hidden":      []any{"convened_by", "people"},
		},
	}
	out := publicView(doc).(map[string]any)
	event := out["event"].(map[string]any)
	if event["city"] != "" || event["title"] != "Bio Connect" || event["hidden"] != nil {
		t.Fatalf("event: %v", event)
	}
	sponsors := out["sponsors"].([]any)
	if len(sponsors) != 1 || sponsors[0].(map[string]any)["name"] != "Live" || sponsors[0].(map[string]any)["published"] != nil {
		t.Fatalf("sponsors: %v", sponsors)
	}
	leadership := out["leadership"].(map[string]any)
	// Released apps require the key, so a hidden map becomes null and a list empty.
	if v, ok := leadership["convened_by"]; !ok || v != nil || len(leadership["people"].([]any)) != 0 {
		t.Fatalf("leadership: %v", leadership)
	}
}

func TestMobileContentValidation(t *testing.T) {
	valid := func() mobileContent {
		return mobileContent{Event: contentEvent{Title: "Bio Connect", StartDate: "2026-10-08", EndDate: "2026-10-09"}}
	}
	cases := map[string]func(*mobileContent){
		"missing title":         func(c *mobileContent) { c.Event.Title = " " },
		"reversed dates":        func(c *mobileContent) { c.Event.EndDate = "2026-10-07" },
		"invalid moments album": func(c *mobileContent) { c.Event.MomentsAlbumID = "album-one" },
		"hide required date":    func(c *mobileContent) { c.Event.Hidden = []string{"start_date"} },
		"unsafe brochure":       func(c *mobileContent) { c.Event.BrochureURL = "javascript:alert(1)" },
		"unknown menu":          func(c *mobileContent) { c.Menus = map[string][]menuItem{"drawer": {}} },
		"unknown destination":   func(c *mobileContent) { c.Menus = map[string][]menuItem{"guide": {{Key: "admin"}}} },
		"tab outside allowed":   func(c *mobileContent) { c.Menus = map[string][]menuItem{"tabs": {{Key: "faqs"}}} },
		"link without address":  func(c *mobileContent) { c.Menus = map[string][]menuItem{"guide": {{Key: "link", Title: "Help"}}} },
		"link to script": func(c *mobileContent) {
			c.Menus = map[string][]menuItem{"guide": {{Key: "link", Title: "x", URL: "javascript:x"}}}
		},
		"bad copy key":         func(c *mobileContent) { c.Copy = map[string]string{"Title": "x"} },
		"sponsor without name": func(c *mobileContent) { c.Sponsors = []contentSponsor{{LogoURL: "https://x.test/a.png"}} },
		"theme without image":  func(c *mobileContent) { c.Themes = []contentTheme{{Title: "AI"}} },
	}
	for name, mutate := range cases {
		c := valid()
		mutate(&c)
		if c.validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := valid()
	c.Menus = map[string][]menuItem{"guide": {
		{Key: "link", Title: "Call the help desk", URL: "tel:+91 471 000 0000", Published: true},
		{Key: "link", Title: "Email", URL: "mailto:help@example.com", Published: true},
		{Key: "link", Title: "Live stream", URL: "https://example.com/live", Published: true},
		{Key: "faqs", Published: true},
	}}
	c.Copy = map[string]string{"sessions.title": "Today on stage"}
	c.Event.MomentsAlbumID = "42"
	c.Event.Hidden = []string{"brochure_url"}
	c.Themes = []contentTheme{{Title: "AI", Image: "https://example.com/ai.webp"}, {Title: "Bio", Image: "assets/images/theme-biopharma.webp"}}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	c.Event.MomentsAlbumID = "0"
	if c.validate() == nil {
		t.Fatal("zero moments album accepted")
	}
	if validateSpeakers([]adminSpeaker{{ID: "a", Name: "A"}, {ID: "a", Name: "B"}}, "") == nil {
		t.Fatal("duplicate speaker IDs accepted")
	}
	if validateSpeakers([]adminSpeaker{{ID: "Bad ID", Name: "A"}}, "") == nil {
		t.Fatal("malformed speaker ID accepted")
	}
}

func TestMobileContentAPI(t *testing.T) {
	a := mustApp(t)
	h := a.Handler()
	staff, _ := addStaff(t, a, "content@example.com", "manager")
	session, csrf := randomToken(), randomToken()
	if _, err := a.DB.Exec(t.Context(), "INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')", hash(session), staff, hash(csrf)); err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any, auth, token bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: "bc_session", Value: session})
		}
		if token {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr
	}
	const path = "/api/v1/admin/mobile-content"
	if rr := request("GET", path, nil, false, false); rr.Code != 401 {
		t.Fatalf("anonymous admin: %d", rr.Code)
	}
	rr := request("GET", path, nil, true, false)
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var m mobileEditor
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	// Migration 031 keeps every existing entry visible and seeds the released
	// menus; 033 adds My agenda and Contacts, and the feedback pages switched
	// off; 035 takes My agenda off the tabs and reorders Home.
	guide := m.Content.Menus["guide"]
	if len(m.Speakers) != 56 || m.Speakers[0].ImageURL == "" || len(m.Content.Sponsors) != 9 || !m.Content.Sponsors[0].Published ||
		len(m.Content.Leadership.Committee.Members) != 14 || !m.Content.Leadership.Committee.Members[0].Published || m.Content.Event.MomentsAlbumID != "" || m.Content.Event.ShowExhibitorRegistration ||
		len(guide) != 16 || guide[0].Key != "agenda" || !guide[0].Published || guide[13].Key != "moments" ||
		guide[14].Key != "feedback" || guide[14].Published || guide[15].Key != "hub" ||
		len(m.Content.Menus["tabs"]) != 2 || menuKeys(m.Content.Menus["home_shortcuts"]) != "speakers exhibitors venue agenda contacts moments" ||
		menuKeys(m.Content.Menus["home_links"]) != "my_passes registration explore sponsors" ||
		m.Content.Feedback.Open || m.Content.Event.PrivacyURL == "" {
		t.Fatalf("migrated content incomplete: %+v", m.Content.Menus)
	}

	m.Guide.Sessions = []guideSession{{ID: "draft", Title: "Secret draft"}, {ID: "live", Title: "Published session", Published: true}}
	m.Guide.Venue = guideVenue{Address: "Kovalam Road", HelpPhone: "+91 99999 00000", HelpWhatsApp: "+91 88888 00000", Published: true, Hidden: []string{"help_phone"}}
	m.Content.Sponsors[1].Published = false
	hiddenSponsor := m.Content.Sponsors[1].Name
	m.Content.Event.Hidden = []string{"brochure_url"}
	m.Content.Event.MomentsAlbumID = "42"
	m.Content.Event.ShowExhibitorRegistration = true
	m.Content.Copy["sessions.title"] = "Today on stage"
	m.Content.Menus["tabs"][1].Published = false
	m.Speakers = append([]adminSpeaker{{ID: "new-speaker", Name: "New Speaker", Role: "Chair", Organization: "Lab", Published: true}}, m.Speakers[1:]...)
	removed := "jayakrishna-ambati"

	if rr = request("PUT", path, m, true, false); rr.Code != 401 {
		t.Fatalf("missing CSRF: %d", rr.Code)
	}
	if rr = request("PUT", path, m, true, true); rr.Code != 200 {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}
	var saved mobileEditor
	if err := json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Revision != m.Revision+1 || saved.Speakers[0].ID != "new-speaker" || len(saved.Speakers) != 56 {
		t.Fatalf("saved editor: revision=%d first=%s count=%d", saved.Revision, saved.Speakers[0].ID, len(saved.Speakers))
	}
	if rr = request("PUT", path, m, true, true); rr.Code != 409 {
		t.Fatalf("stale overwrite: %d", rr.Code)
	}
	bad := saved
	bad.Content.Event.Hidden = []string{"start_date"}
	if rr = request("PUT", path, bad, true, true); rr.Code != 400 {
		t.Fatalf("invalid content: %d", rr.Code)
	}

	rr = request("GET", "/api/v1/public/app-content", nil, false, false)
	body := rr.Body.String()
	if rr.Code != 200 {
		t.Fatal(body)
	}
	for _, leak := range []string{"Secret draft", "+91 99999 00000", `"id":"` + removed + `"`, `"hidden"`, `"published"`} {
		if bytes.Contains(rr.Body.Bytes(), []byte(leak)) {
			t.Fatalf("public content leaked %q", leak)
		}
	}
	var out struct {
		Event      map[string]any        `json:"event"`
		Copy       map[string]string     `json:"copy"`
		Menus      map[string][]menuItem `json:"menus"`
		Speakers   []publicSpeaker       `json:"speakers"`
		Sponsor    map[string]any        `json:"sponsor"`
		Sponsors   []map[string]any      `json:"sponsors"`
		EventGuide struct {
			Sessions []guideSession `json:"sessions"`
			Venue    guideVenue     `json:"venue"`
		} `json:"event_guide"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Event["brochure_url"] != "" || out.Event["start_date"] != "2026-10-08" || out.Event["moments_album_id"] != "42" || out.Event["show_exhibitor_registration"] != true || out.Copy["sessions.title"] != "Today on stage" ||
		len(out.Menus["tabs"]) != 1 || len(out.Sponsors) != 8 || out.Sponsor["name"] != out.Sponsors[0]["name"] ||
		out.Speakers[0].ID != "new-speaker" || len(out.EventGuide.Sessions) != 1 ||
		out.EventGuide.Venue.HelpWhatsApp != "+91 88888 00000" || out.EventGuide.Venue.HelpPhone != "" {
		t.Fatalf("public content: %s", body)
	}
	for _, s := range out.Sponsors {
		if s["name"] == hiddenSponsor {
			t.Fatalf("hidden sponsor %q published", hiddenSponsor)
		}
	}
	rr = request("GET", "/api/v1/public/event-guide", nil, false, false)
	if bytes.Contains(rr.Body.Bytes(), []byte("+91 99999 00000")) || bytes.Contains(rr.Body.Bytes(), []byte("Secret draft")) {
		t.Fatalf("event guide leaked: %s", rr.Body.String())
	}
}
