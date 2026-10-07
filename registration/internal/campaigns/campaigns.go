// Package campaigns holds the broadcast emails staff can send. Each template is
// a directory with email.html, email.txt and any images the HTML references as
// cid:<file>. Images travel as inline attachments, so a new template needs no
// static deployment. __GREETING__ and __EMAIL__ are replaced per recipient.
package campaigns

import (
	"embed"
	"html"
	"io/fs"
	"mime"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed invitation app-launch event-reminder plain-reminder event-today
var files embed.FS

var ist = time.FixedZone("IST", 19800)

type Template struct {
	ID, Label, Description, Subject string
	// Registrants marks templates written for registered participants; only
	// these are offered in the admin console.
	Registrants bool
	// Attach each recipient's own PDF pass; only pass holders can receive it.
	AttachPass bool
	// Dated copy is sendable from From (zero: any time) until Until. Tests
	// and previews only need the copy not to have expired.
	From, Until time.Time
	html, text  string
}

type Image struct {
	Name, ContentType string
	Data              []byte
}

var registry = map[string]Template{
	"invitation": {
		ID: "invitation", Label: "Registration invitation (early bird)",
		Description: "Invites prospective delegates and exhibitors to register.",
		Subject:     "Bio Connect 4.0: register now for Kerala's life sciences summit",
		Until:       time.Date(2026, 10, 1, 0, 0, 0, 0, ist),
	},
	"app-launch": {
		ID: "app-launch", Label: "Mobile app announcement",
		Description: "Store links, how to add the pass to the app, and the app's features.",
		Subject:     "Your Bio Connect 4.0 pass is now in the app",
		Registrants: true,
		Until:       time.Date(2026, 10, 10, 0, 0, 0, 0, ist),
	},
	// The reminders say "tomorrow", so they stop at midnight before day one.
	"event-reminder": {
		ID: "event-reminder", Label: "Event reminder with pass",
		Description: "Day-before reminder with each person's PDF pass attached, arrival steps, timings and app links.",
		Subject:     "Bio Connect 4.0 starts tomorrow: your pass and arrival guide",
		Registrants: true, AttachPass: true,
		Until: time.Date(2026, 10, 8, 0, 0, 0, 0, ist),
	},
	"plain-reminder": {
		ID: "plain-reminder", Label: "Plain reminder",
		Description: "A short day-before reminder with the date, venue and start time.",
		Subject:     "Reminder: Bio Connect 4.0 starts tomorrow",
		Registrants: true,
		Until:       time.Date(2026, 10, 8, 0, 0, 0, 0, ist),
	},
	// Says "today", so it is sendable only on day one, at any hour.
	"event-today": {
		ID: "event-today", Label: "Event day with pass",
		Description: "Day-one email with each person's PDF pass attached, how to get in and the programme.",
		Subject:     "Bio Connect 4.0 is on today: your pass and how to get in",
		Registrants: true, AttachPass: true,
		From:  time.Date(2026, 10, 8, 0, 0, 0, 0, ist),
		Until: time.Date(2026, 10, 9, 0, 0, 0, 0, ist),
	},
}

func init() {
	for id, t := range registry {
		t.html = read(id + "/email.html")
		t.text = read(id + "/email.txt")
		registry[id] = t
	}
}

func read(name string) string {
	b, err := fs.ReadFile(files, name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Get returns a template by ID.
func Get(id string) (Template, bool) {
	t, ok := registry[id]
	return t, ok
}

// All returns every template, ordered by ID.
func All() []Template {
	out := make([]Template, 0, len(registry))
	for _, t := range registry {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Open reports whether the template may be sent to recipients at now.
func (t Template) Open(now time.Time) bool {
	return !now.Before(t.From) && now.Before(t.Until)
}

// Expired reports whether the copy is past its send-by time, after which not
// even a test may go out.
func (t Template) Expired(now time.Time) bool { return !now.Before(t.Until) }

// Render personalises the HTML and plain-text bodies. An empty name gives a
// generic greeting.
func (t Template) Render(name, email string) (htmlBody, textBody string) {
	greeting := "Hello,"
	if name = strings.TrimSpace(name); name != "" {
		greeting = "Hello " + name + ","
	}
	htmlBody = strings.NewReplacer("__GREETING__", html.EscapeString(greeting), "__EMAIL__", html.EscapeString(email)).Replace(t.html)
	textBody = strings.NewReplacer("__GREETING__", greeting, "__EMAIL__", email).Replace(t.text)
	return
}

var cidRef = regexp.MustCompile(`cid:([A-Za-z0-9._-]+)`)

// Images returns the files the HTML references as cid:<name>, in order of
// first use.
func (t Template) Images() ([]Image, error) {
	var out []Image
	seen := map[string]bool{}
	for _, m := range cidRef.FindAllStringSubmatch(t.html, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		b, err := fs.ReadFile(files, t.ID+"/"+name)
		if err != nil {
			return nil, err
		}
		out = append(out, Image{Name: name, ContentType: mime.TypeByExtension(path.Ext(name)), Data: b})
	}
	return out, nil
}
