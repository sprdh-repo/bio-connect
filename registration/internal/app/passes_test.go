package app

import (
	"encoding/xml"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Exercise the printable PDF itself without requiring a database or delivery.
func TestRenderedPassVariants(t *testing.T) {
	for _, tool := range []string{"pdftotext", "pdftoppm", "zbarimg"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
	const name = "Dr. Lakshmi Venkataraman Subramanian"
	const organisation = "Rajiv Gandhi Centre for Biotechnology, Thiruvananthapuram"
	const role = "Principal Scientist, Molecular Diagnostics"
	const number = "BC4-EX-0007-3"
	const qr = "ZSBZY1g_lYLHTOTsddkvmAWhLP9SSel-YqsIGTwhmMk"
	var anchors []pdfPassWord
	for _, sample := range []struct{ category, label string }{
		{"premium", "EXHIBITOR"}, {"standard", "EXHIBITOR"}, {"table", "EXHIBITOR"},
		{"faculty", "FACULTY"}, {"industry", "INDUSTRY"},
		{"startup", "STARTUP"}, {"student", "STUDENT"},
	} {
		t.Run(sample.category, func(t *testing.T) {
			data, err := renderPass(name, organisation, role, sample.category, "", number, qr)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "pass.pdf")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			words := readPassWords(t, path)
			var text strings.Builder
			var currentAnchors []pdfPassWord
			for _, word := range words {
				text.WriteString(word.Text)
				if word.XMin < 0 || word.YMin < 0 || word.XMax > 421 || word.YMax > 596 {
					t.Errorf("text outside A5 page: %+v", word)
				}
				if word.Text == "Building" || word.Text == "ATTENDEE" || word.Text == "NUMBER" {
					currentAnchors = append(currentAnchors, word)
				}
			}
			for _, want := range []string{sample.label, name, organisation, role, number,
				"KERALA'S INTERNATIONAL LIFE SCIENCES SUMMIT",
				"Building Kerala's Global Life Sciences Hub", "ADMISSION PASS"} {
				if !strings.Contains(text.String(), strings.ReplaceAll(want, " ", "")) {
					t.Errorf("PDF is missing %q", want)
				}
			}
			if strings.Contains(text.String(), "conclave") {
				t.Error("obsolete event wording remains")
			}
			if len(currentAnchors) != 3 {
				t.Fatalf("found %d grid anchors, want 3", len(currentAnchors))
			}
			if anchors == nil {
				anchors = currentAnchors
			} else {
				for i, word := range currentAnchors {
					if math.Abs(word.XMin-anchors[i].XMin) > 0.01 || math.Abs(word.YMin-anchors[i].YMin) > 0.01 {
						t.Errorf("%s moved between categories", word.Text)
					}
				}
			}
			// Scan only the fixed QR area at 150 dpi, including the printed frame.
			// This also verifies that every variant keeps the QR in the same place.
			prefix := filepath.Join(dir, "qr")
			if out, err := exec.Command("pdftoppm", "-singlefile", "-png", "-r", "150",
				"-x", "602", "-y", "992", "-W", "220", "-H", "230", path, prefix).CombinedOutput(); err != nil {
				t.Fatalf("render PDF: %v: %s", err, out)
			}
			out, err := exec.Command("zbarimg", "--quiet", "--raw", prefix+".png").Output()
			if err != nil || strings.TrimSpace(string(out)) != qr {
				t.Fatalf("QR scan: %q, %v", out, err)
			}
		})
	}
}

type pdfPassWord struct {
	Text string  `xml:",chardata"`
	XMin float64 `xml:"xMin,attr"`
	YMin float64 `xml:"yMin,attr"`
	XMax float64 `xml:"xMax,attr"`
	YMax float64 `xml:"yMax,attr"`
}

func readPassWords(t *testing.T, path string) []pdfPassWord {
	t.Helper()
	out, err := exec.Command("pdftotext", "-bbox", path, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Pages []struct {
			Words []pdfPassWord `xml:"word"`
		} `xml:"body>doc>page"`
	}
	if err := xml.Unmarshal(out, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Pages) != 1 {
		t.Fatalf("pass has %d pages, want 1", len(document.Pages))
	}
	return document.Pages[0].Words
}

func TestPassFitsMaximumLengthFields(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	// Wide, unbroken strings exercise accepted field limits and hard wrapping.
	// The pass number is the pre-shortening 32-character form, still rendered by
	// any pass issued before short numbers.
	name, organisation, role := strings.Repeat("W", 120), strings.Repeat("M", 180), strings.Repeat("W", 180)
	data, err := renderPass(name, organisation, role, "faculty", "", "BC4-"+strings.Repeat("F", 32), "sample-qr")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pass.pdf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var foundName, foundOrg, foundRole strings.Builder
	for _, word := range readPassWords(t, path) {
		if strings.Trim(word.Text, "WM") != "" {
			continue
		}
		x, bottom := word.XMax*25.4/72, word.YMax*25.4/72
		switch {
		case word.YMin*25.4/72 < 130:
			foundName.WriteString(word.Text)
			if x > 138.1 || bottom > 131 {
				t.Errorf("name overflow: %+v", word)
			}
		case word.YMin*25.4/72 < 160:
			foundOrg.WriteString(word.Text)
			if x > 138.1 || bottom > 162 {
				t.Errorf("organisation overflow: %+v", word)
			}
		default:
			foundRole.WriteString(word.Text)
			if x > 95.1 || bottom > 185 {
				t.Errorf("role overflow: %+v", word)
			}
		}
	}
	if foundName.String() != name || foundOrg.String() != organisation || foundRole.String() != role {
		t.Error("maximum-length fields lost content")
	}
}

// BC4-EX-0007-3: the event, the category word printed on the pass, the
// registration's place in that category's series, and the pass's place in the
// registration.
var passNumberShape = regexp.MustCompile(`^BC4-(EX|FC|IN|SP|ST)-[0-9]{4}-[0-9]+$`)

func TestCategoryCodeMatchesThePrintedBadge(t *testing.T) {
	for _, c := range []struct{ catID, code, badge string }{
		{"student", "ST", "STUDENT"}, {"startup", "SP", "STARTUP"},
		{"faculty", "FC", "FACULTY"}, {"industry", "IN", "INDUSTRY"},
		{"premium", "EX", "EXHIBITOR"}, {"standard", "EX", "EXHIBITOR"}, {"table", "EX", "EXHIBITOR"},
	} {
		if got := categoryCode(c.catID); got != c.code {
			t.Errorf("categoryCode(%q) = %q, want %q", c.catID, got, c.code)
		}
		// The two letters must be the start of the word on the pass, so a code
		// read off a badge is never a different category from the badge itself.
		if word := passStyleFor(c.catID).stripWord; word != c.badge || !strings.HasPrefix(word, c.code[:1]) {
			t.Errorf("category %q prints %q, code %q", c.catID, word, c.code)
		}
	}
}

func TestNormalizePassNumberAcceptsHowPeopleRetypeIt(t *testing.T) {
	for _, typed := range []string{"BC4-5BD7-Q8KW", "bc4 5bd7 q8kw", " BC45BD7Q8KW ", "bc4/5bd7/q8kw"} {
		if got := normalizePassNumber(typed); got != "BC45BD7Q8KW" {
			t.Errorf("normalizePassNumber(%q) = %q", typed, got)
		}
	}
}
