package app

import "testing"

func TestGuideValidationAndPublication(t *testing.T) {
	g := eventGuide{Sessions: []guideSession{{ID: "draft", Title: "Draft"}, {ID: "live", Title: "Live", Published: true}}, FAQs: []guideFAQ{{ID: "q", Question: "Where?", Answer: "Here", Published: true}}, Venue: guideVenue{Address: "Private draft"}}
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	public := g.public()
	if len(public.Sessions) != 1 || public.Sessions[0].ID != "live" || public.Venue.Address != "" {
		t.Fatal("draft content leaked")
	}
	g.Sessions[1].StartsAt = "2026-10-08T10:00:00+05:30"
	g.Sessions[1].EndsAt = "2026-10-08T09:00:00+05:30"
	if g.validate() == nil {
		t.Fatal("reversed session times accepted")
	}
	g.Sessions[1].EndsAt = "2026-10-08T11:00:00+05:30"
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	g.Venue.FloorPlanURL = "javascript:alert(1)"
	if g.validate() == nil {
		t.Fatal("unsafe URL accepted")
	}
	g.Venue.FloorPlanURL = "https://example.com/plan.pdf"
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	g.FAQs[0].Answer = ""
	if g.validate() == nil {
		t.Fatal("empty published FAQ accepted")
	}
}
