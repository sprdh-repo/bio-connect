package campaigns

import (
	"strings"
	"testing"
	"time"
)

func TestEveryTemplateIsComplete(t *testing.T) {
	for _, tmpl := range All() {
		h, p := tmpl.Render("Asha", "asha@example.com")
		for _, body := range []string{h, p} {
			if strings.Contains(body, "__") || !strings.Contains(body, "{{{ pm:unsubscribe }}}") {
				t.Fatalf("%s: unreplaced placeholder or missing unsubscribe", tmpl.ID)
			}
		}
		if tmpl.Subject == "" || tmpl.Label == "" || tmpl.Until.IsZero() {
			t.Fatalf("%s: incomplete metadata", tmpl.ID)
		}
		images, err := tmpl.Images()
		if err != nil {
			t.Fatalf("%s: %v", tmpl.ID, err)
		}
		for _, img := range images {
			if !strings.HasPrefix(img.ContentType, "image/") || len(img.Data) == 0 {
				t.Fatalf("%s: bad image %s", tmpl.ID, img.Name)
			}
		}
	}
}

func TestRenderEscapesRecipientDetails(t *testing.T) {
	tmpl, _ := Get("app-launch")
	h, p := tmpl.Render("<b>&", "a&b@example.com")
	if strings.Contains(h, "<b>&") || !strings.Contains(h, "Hello &lt;b&gt;&amp;,") || !strings.Contains(h, "a&amp;b@example.com") {
		t.Fatal("HTML not escaped")
	}
	if !strings.Contains(p, "Hello <b>&,") || !strings.Contains(p, "a&b@example.com") {
		t.Fatal("text not personalised")
	}
	if h, _ := tmpl.Render(" ", "x@example.com"); !strings.Contains(h, "Hello,") {
		t.Fatal("blank name should give a generic greeting")
	}
}

func TestAppLaunchContent(t *testing.T) {
	tmpl, ok := Get("app-launch")
	if !ok || !tmpl.Registrants {
		t.Fatal("app-launch must be offered to registrants")
	}
	h, _ := tmpl.Render("", "x@example.com")
	for _, want := range []string{"id6817779744", "in.gov.kerala.bioconnect", "cid:hero.jpg", "Zinvos"} {
		if !strings.Contains(h, want) {
			t.Fatalf("missing %q", want)
		}
	}
	images, _ := tmpl.Images()
	if len(images) != 11 {
		t.Fatalf("got %d inline images, want 11", len(images))
	}
	if !tmpl.Open(time.Date(2026, 10, 9, 23, 0, 0, 0, ist)) || tmpl.Open(time.Date(2026, 10, 10, 0, 0, 0, 0, ist)) {
		t.Fatal("app-launch cutoff should be the end of 9 October IST")
	}
}

func TestEventTodayWindow(t *testing.T) {
	tmpl, _ := Get("event-today")
	before, day, after := time.Date(2026, 10, 7, 23, 59, 0, 0, ist), time.Date(2026, 10, 8, 18, 0, 0, 0, ist), time.Date(2026, 10, 9, 0, 0, 0, 0, ist)
	if tmpl.Open(before) || tmpl.Expired(before) || !tmpl.Open(day) || tmpl.Open(after) || !tmpl.Expired(after) {
		t.Fatal("event-today must be sendable only on 8 October IST")
	}
	if !tmpl.AttachPass {
		t.Fatal("event-today attaches the pass")
	}
	h, p := tmpl.Render("", "x@example.com")
	if strings.Contains(strings.ToLower(h+p), "tomorrow") {
		t.Fatal("day-of copy must not say tomorrow")
	}
}
