package app

import (
	"fmt"
	"os"
	"testing"
)

// Renders one pass PDF for each visual category to DUMP_PASS_DIR for manual visual
// review. Skipped unless the env var is set; not part of the normal suite.
func TestDumpSamplePasses(t *testing.T) {
	dir := os.Getenv("DUMP_PASS_DIR")
	if dir == "" {
		t.Skip("set DUMP_PASS_DIR to render sample passes")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	samples := []struct{ file, name, inst, desg, catID, label string }{
		{"pass-student.pdf", "Asha Nair", "Kerala University", "PhD Scholar", "student", "Students"},
		{"pass-startup.pdf", "Rohit Menon", "Gennext Biosciences Pvt Ltd", "Co-founder", "startup", "Incubation / Startups"},
		{"pass-faculty.pdf", "Dr. Lakshmi Venkataraman Subramanian",
			"Rajiv Gandhi Centre for Biotechnology, Thiruvananthapuram",
			"Principal Scientist, Molecular Diagnostics", "faculty", "Faculty / Scientists"},
		{"pass-industry.pdf", "Ananya Krishnan", "Biocon Biologics", "Head of Business Development", "industry", "Industry"},
		{"pass-exhibitor.pdf", "Ravi Chandran",
			"Rajiv Gandhi Centre for Biotechnology", "Booth in-charge", "premium", "Premium stall (6m x 3m)"},
	}
	for i, s := range samples {
		number := fmt.Sprintf("BC4-%s-%04d-%d", categoryCode(s.catID), i+1, i+1)
		if s.catID == "industry" {
			// One sample keeps the pre-shortening 32-character form, so the
			// footer can be eyeballed against passes issued before the change.
			number = "BC4-8B62616F7D284E91A502C394F0B673DE"
		}
		b, err := renderPass(s.name, s.inst, s.desg, s.catID, s.label, number, "opaque-qr-"+s.file)
		if err != nil {
			t.Fatalf("%s: %v", s.file, err)
		}
		if err := os.WriteFile(dir+"/"+s.file, b, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s/%s (%d bytes)", dir, s.file, len(b))
	}
}

func TestDumpSampleEmail(t *testing.T) {
	dir := os.Getenv("DUMP_PASS_DIR")
	if dir == "" {
		t.Skip("set DUMP_PASS_DIR")
	}
	for _, c := range []struct{ file, heading, intro, cta string }{
		{"email-pass.html", "Your pass is ready", "Your Bio Connect 4.0 pass is attached to this email. The event takes place on 8-9 October 2026 at Hyatt Regency Trivandrum. You can also open it any time from the link below.\n\nPass number: BC4-EX-0007-3 - quote this if you contact us about your pass.", "View your pass"},
		{"email-registration.html", "Registration saved", "Your registration is saved. Use this private link to see the payment instructions and submit your evidence.\n\nThis is not a payment approval or an admission pass - keep the link to yourself.", "Open my registration"},
	} {
		html := emailHTML("https://reg.bioconnect.kerala.gov.in", c.heading, c.intro, c.cta,
			"https://reg.bioconnect.kerala.gov.in/manage/e2d0fae033d18a1da242ac118bd23f25#1gZHE1RQAcmNs4aXTuOd3VTZFMplbLOX")
		os.WriteFile(dir+"/"+c.file, []byte(html), 0o644)
		t.Logf("wrote %s (%d bytes)", c.file, len(html))
	}
}
