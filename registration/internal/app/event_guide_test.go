package app

import "testing"

func TestGuideValidationAndPublication(t *testing.T) {
	g := eventGuide{Sessions: []guideSession{{ID: "draft", Title: "Draft"}, {ID: "live", Title: "Live", Published: true}}, FAQs: []guideFAQ{{ID: "q", Question: "Where?", Answer: "Here", Published: true}}, Venue: guideVenue{Address: "Private draft"}}
	if err := g.validate(); err != nil {
		t.Fatal(err)
	}
	public := g.public(true)
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

func TestProgrammeStructureValidation(t *testing.T) {
	valid := func() eventGuide {
		return eventGuide{Sessions: []guideSession{{
			ID: "inaugural", Kind: "ceremony", Title: "Inaugural Session",
			StartsAt: "2026-10-08T10:00:00+05:30", EndsAt: "2026-10-08T11:40:00+05:30",
			People: []guidePerson{{Name: "Dr. A", Role: "moderator"}},
			Segments: []guideSegment{
				{StartsAt: "2026-10-08T10:00:00+05:30", EndsAt: "2026-10-08T10:03:00+05:30", Title: "Opening Remarks", People: []guidePerson{{Name: "Master of Ceremonies"}}},
				{Title: "Media interaction, if any"},
			},
		}}}
	}
	if err := valid().validate(); err != nil {
		t.Fatal(err)
	}
	for name, breakIt := range map[string]func(*guideSession){
		"unknown kind":          func(s *guideSession) { s.Kind = "workshop" },
		"unknown role":          func(s *guideSession) { s.People[0].Role = "chair" },
		"nameless person":       func(s *guideSession) { s.People[0].Name = " " },
		"untitled segment":      func(s *guideSession) { s.Segments[1].Title = "" },
		"half-timed segment":    func(s *guideSession) { s.Segments[1].StartsAt = "2026-10-08T11:35:00+05:30" },
		"reversed segment":      func(s *guideSession) { s.Segments[0].EndsAt = "2026-10-08T09:00:00+05:30" },
		"nameless segment host": func(s *guideSession) { s.Segments[0].People[0].Name = "" },
	} {
		g := valid()
		breakIt(&g.Sessions[0])
		if g.validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestLegacySpeakersDerivedFromPeople(t *testing.T) {
	g := eventGuide{Sessions: []guideSession{
		{ID: "panel", Kind: "panel", Title: "Panel", People: []guidePerson{
			{Name: "Dr. Mod", Designation: "Chair, X", Role: "moderator", SpeakerID: "mod"},
			{Name: "Dr. P", Role: "panelist", SpeakerID: "p"},
			{Name: "Ms. H", Role: "host"},
		}},
		{ID: "opening", Kind: "ceremony", Title: "Opening", Segments: []guideSegment{
			{StartsAt: "2026-10-08T04:33:00Z", EndsAt: "2026-10-08T04:40:00Z", Title: "Welcome", People: []guidePerson{{Name: "Shri. W", Designation: "MD", SpeakerID: "p"}}},
			{Title: "Media interaction"},
		}},
		{ID: "old", Title: "Old", Speakers: "As typed", SpeakerIDs: []string{"kept"}},
	}}
	normalizeGuide(&g)
	if got := g.Sessions[0].Speakers; got != "Moderator: Dr. Mod, Chair, X\nDr. P\nHost: Ms. H" {
		t.Fatalf("panel speakers: %q", got)
	}
	if got := g.Sessions[0].SpeakerIDs; len(got) != 2 || got[0] != "mod" || got[1] != "p" {
		t.Fatalf("panel speaker IDs: %v", got)
	}
	if got := g.Sessions[1].Speakers; got != "10:03 Welcome: Shri. W, MD\nMedia interaction" {
		t.Fatalf("ceremony speakers: %q", got)
	}
	if got := g.Sessions[2]; got.Speakers != "As typed" || len(got.SpeakerIDs) != 1 {
		t.Fatalf("legacy session changed: %+v", got)
	}
}
