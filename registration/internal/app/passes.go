package app

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

//go:embed fonts/Manrope-Bold.ttf
var fontDisplay []byte

//go:embed fonts/DMSans-Regular.ttf
var fontBody []byte

//go:embed fonts/DMSans-Medium.ttf
var fontMid []byte

//go:embed fonts/NotoSans-Regular.ttf
var fontFallback []byte

// passStyle is the per-category treatment for a pass: accent colours, the word
// carried up the left strip, and the short category label. Wording is deliberately
// plain (no "/ Scientists", no admin terms) so it reads from across a room.
type passStyle struct {
	strip, word [3]int
	stripWord   string
	badge       string
	exhibitor   bool
}

func passStyleFor(catID string) passStyle {
	forest, deep, lime, gold := [3]int{11, 51, 41}, [3]int{5, 28, 23}, [3]int{185, 220, 114}, [3]int{228, 173, 84}
	teal, aqua := [3]int{15, 96, 118}, [3]int{190, 227, 236}
	switch catID {
	case "student":
		return passStyle{lime, forest, "STUDENT", "Student", false}
	case "startup":
		return passStyle{gold, forest, "STARTUP", "Startup", false}
	case "faculty": // BioConnect teal keeps Faculty distinct from Exhibitor
		return passStyle{teal, aqua, "FACULTY", "Faculty", false}
	case "industry":
		return passStyle{deep, gold, "INDUSTRY", "Industry", false}
	default: // premium / standard / table
		return passStyle{forest, lime, "EXHIBITOR", "Exhibitor", true}
	}
}

func nonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// renderPass uses one A5 grid for every category. Text fits inside reserved
// areas so long names and organisations cannot move the QR or pass number.
func renderPass(name, institution, designation, catID, catLabel, number, qr string) ([]byte, error) {
	st := passStyleFor(catID)
	const (
		width, height             = 148.0, 210.0
		stripW                    = 33.0
		left, right               = 40.0, 138.0
		contentW                  = right - left
		logoY, logoW              = 20.0, 80.0
		nameBaseline, orgBaseline = 111.0, 137.0
		// Fixed footer grid: identical coordinates on every category. The
		// designation box, QR card, pass number and "Admission Pass" never
		// move; only the field text inside them is auto-fitted.
		footerY          = 164.0
		qrX, qrY, qrSize = 103.0, footerY, 35.0
		footerW          = qrX - left - 5.0
	)
	forest := [3]int{11, 51, 41}
	paper := [3]int{247, 245, 239}

	p := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P", UnitStr: "mm",
		Size: fpdf.SizeType{Wd: width, Ht: height},
	})
	p.SetAutoPageBreak(false, 0)
	p.SetCellMargin(0)
	p.SetTitle("Bio Connect 4.0 - "+st.badge+" admission pass", false)
	p.SetAuthor("Bio Connect 4.0", false)
	p.AddUTF8FontFromBytes("D", "", fontDisplay)
	p.AddUTF8FontFromBytes("B", "", fontBody)
	p.AddUTF8FontFromBytes("M", "", fontMid)
	p.AddUTF8FontFromBytes("F", "", fontFallback)
	p.AddPage()
	textColor := func(c [3]int) { p.SetTextColor(c[0], c[1], c[2]) }
	fillColor := func(c [3]int) { p.SetFillColor(c[0], c[1], c[2]) }

	fillColor(paper)
	p.Rect(0, 0, width, height, "F")
	fillColor(st.strip)
	p.Rect(0, 0, stripW, height, "F")
	drawSprig(p, stripW, height, st.word)

	// A shared cap height and top edge give every category equal recognition.
	textColor(st.word)
	p.SetFont("D", "", 55)
	tw := p.GetStringWidth(st.stripWord)
	p.TransformBegin()
	p.TransformRotate(90, stripW/2, 78)
	p.Text(stripW/2+78-17-tw, 78+6.7, st.stripWord)
	p.TransformEnd()

	// Slot is centred on the physical badge, clear of the brand lockup.
	p.SetDrawColor(190, 191, 182)
	p.SetLineWidth(0.25)
	p.RoundedRect(width/2-12.5, 6, 25, 5.5, 2.75, "1234", "D")

	logo, err := resources.ReadFile("web/bio-connect-logo-mark.png")
	if err != nil {
		return nil, fmt.Errorf("pass logo: %w", err)
	}
	p.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(logo))
	// The source lockup includes an obsolete tagline below the mark. Clip only
	// that tagline in the PDF; retain the original brand asset for other uses.
	p.ClipRect(left, logoY, logoW, logoW*140/718, false)
	p.ImageOptions("logo", left, logoY, logoW, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	p.ClipEnd()
	textColor(forest)
	p.SetFont("D", "", 24)
	p.Text(left+logoW+2, logoY+12.5, "4.0")

	subtitle := "KERALA'S INTERNATIONAL LIFE SCIENCES SUMMIT"
	subSize := 10.0
	p.SetFont("M", "", subSize)
	for p.GetStringWidth(subtitle) > contentW && subSize > 6 {
		subSize -= 0.25
		p.SetFont("M", "", subSize)
	}
	p.Text(left, 46, subtitle)
	p.SetFont("D", "", 22)
	p.Text(left, 62, "Building Kerala's")
	p.Text(left, 72, "Global Life Sciences Hub")
	p.SetDrawColor(168, 201, 91)
	p.SetLineWidth(0.3)
	p.Line(left, 80, right, 80)
	p.SetFont("M", "", 10)
	p.Text(left, 87, "08-09 October 2026")
	p.SetFont("B", "", 10)
	p.Text(left, 93, "Hyatt Regency Trivandrum")

	label := func(t string, x, y float64) {
		p.SetFont("M", "", 8.5)
		textColor(forest)
		p.Text(x, y, t)
	}
	label("ATTENDEE NAME", left, 102)
	textColor(forest)
	drawPassText(p, name, "D", left, nameBaseline, contentW, 123, 25)
	orgLabel, roleLabel := "INSTITUTION", "DESIGNATION"
	if st.exhibitor || catID == "industry" || catID == "startup" {
		orgLabel = "ORGANISATION"
	}
	if st.exhibitor {
		roleLabel = "REPRESENTATIVE"
	}
	label(orgLabel, left, 128)
	drawPassText(p, institution, "D", left, orgBaseline, contentW, footerY-4, 21)

	// The role panel and number share the left footer column. No invented
	// booth allocation: the application currently stores the representative role.
	fillColor(forest)
	p.RoundedRect(left, footerY, footerW, 21, 1.2, "1234", "F")
	p.SetFont("M", "", 6.5)
	p.SetTextColor(185, 220, 114)
	p.Text(left+3, footerY+4.5, roleLabel)
	p.SetTextColor(255, 255, 255)
	drawPassText(p, designation, "M", left+3, footerY+9.2, footerW-6, footerY+19, 12)

	// A short pass number is meant to be read off the print at arm's length, so
	// it is set as large as the fixed footer column allows; drawPassText still
	// steps down for any longer legacy number.
	label("PASS NUMBER", left, footerY+26)
	drawPassText(p, number, "D", left, footerY+33, footerW, footerY+36.5, 18)
	p.SetDrawColor(168, 201, 91)
	p.Line(left, footerY+38, left+footerW, footerY+38)
	p.SetFont("M", "", 8.5)
	p.Text(left, footerY+43, "ADMISSION PASS")

	// The encoded image includes its four-module quiet zone. Keep the complete
	// image inside a white card so the decorative border cannot affect scanning.
	png, err := qrcode.Encode(qr, qrcode.Medium, 720)
	if err != nil {
		return nil, fmt.Errorf("pass QR: %w", err)
	}
	p.SetFillColor(255, 255, 255)
	p.SetDrawColor(168, 201, 91)
	p.RoundedRect(qrX, qrY, qrSize, qrSize, 1.6, "1234", "DF")
	p.RegisterImageOptionsReader("qr", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
	p.ImageOptions("qr", qrX+1, qrY+1, qrSize-2, qrSize-2, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	var out bytes.Buffer
	err = p.Output(&out)
	return out.Bytes(), err
}

// drawPassText fits any real registration value inside a fixed print area with
// two controlled rules, applied in order: (1) wrap onto more lines, (2) step the
// font size down in 0.25pt increments. It keeps the field's first baseline, so
// the surrounding grid never moves, and never ellipsises text (full pass IDs
// included). Below minSize it draws best effort rather than failing a pass.
func drawPassText(p *fpdf.Fpdf, value, family string, x, baseline, width, bottom, maxSize float64) {
	if nonASCII(value) {
		family = "F"
	}
	const minSize = 4.0
	for size := maxSize; ; size -= 0.25 {
		p.SetFont(family, "", size)
		leading := size * 0.42
		lines := p.SplitText(value, width)
		fits := baseline+float64(len(lines)-1)*leading <= bottom
		for _, line := range lines {
			fits = fits && p.GetStringWidth(line) <= width
		}
		if fits || size <= minSize {
			for i, line := range lines {
				p.Text(x, baseline+float64(i)*leading, line)
			}
			return
		}
	}
}

// drawSprig draws a vector botanical/molecular motif below the category word.
func drawSprig(p *fpdf.Fpdf, stripW, h float64, c [3]int) {
	p.SetDrawColor(c[0], c[1], c[2])
	p.SetLineWidth(0.22)
	p.SetAlpha(0.72, "Normal")
	x := stripW / 2
	p.CurveBezierCubic(7, h, 15, h-22, 8, h-37, 17, h-63, "D")
	leaf := func(sx, sy, ex, ey float64) {
		p.CurveBezierCubic(sx, sy, sx-1, ey, ex-5, ey, ex, ey, "D")
		p.CurveBezierCubic(ex, ey, ex-1, sy, sx+4, sy, sx, sy, "D")
		p.Line(sx, sy, ex-2, ey+2)
	}
	leaf(12, h-25, 26, h-31)
	leaf(12, h-33, 4, h-44)
	leaf(16, h-56, 27, h-67)
	for _, branch := range [][4]float64{
		{12, h - 22, 5, h - 16}, {11, h - 38, 4, h - 53},
		{14, h - 47, 25, h - 53}, {17, h - 63, 8, h - 68},
		{8, h - 68, 6, h - 77}, {8, h - 68, 15, h - 75},
		{10, h - 9, 24, h - 18}, {24, h - 18, 28, h - 24},
	} {
		p.Line(branch[0], branch[1], branch[2], branch[3])
		p.Circle(branch[2], branch[3], 1.4, "D")
	}
	p.Circle(x+1, h-47, 1.8, "D")
	p.SetAlpha(1, "Normal")
}

func (a *App) passPDF(ctx context.Context, pid string) ([]byte, error) {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var name, inst, des, catID, cat, num, qr, rid string
	var file *string
	e = tx.QueryRow(ctx, `SELECT a.name,r.institution,a.designation,c.id,c.label,p.number,p.qr_id,p.registration_id,p.file_id FROM passes p JOIN attendees a ON a.id=p.attendee_id JOIN registrations r ON r.id=p.registration_id JOIN categories c ON c.id=r.category_id WHERE p.id=$1 AND p.revoked_at IS NULL AND r.status='approved' FOR UPDATE OF p`, pid).Scan(&name, &inst, &des, &catID, &cat, &num, &qr, &rid, &file)
	if e != nil {
		return nil, e
	}
	if file != nil {
		b, e := a.Storage.Get(ctx, *file)
		if e != nil {
			return nil, e
		}
		return b, tx.Commit(ctx)
	}
	b, e := renderPass(name, inst, des, catID, cat, num, qr)
	if e != nil {
		return nil, e
	}
	fid := id()
	if e = a.Storage.Put(ctx, fid, b, "application/pdf"); e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, "INSERT INTO files(id,registration_id,kind,object_key,mime,size) VALUES($1,$2,'pass',$1,'application/pdf',$3)", fid, rid, len(b))
	if e == nil {
		_, e = tx.Exec(ctx, "UPDATE passes SET file_id=$2 WHERE id=$1", pid, fid)
	}
	if e != nil {
		return nil, e
	}
	return b, tx.Commit(ctx)
}
func (a *App) passDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	var pid string
	e := a.DB.QueryRow(r.Context(), `SELECT p.id FROM passes p JOIN registrations r ON r.id=p.registration_id WHERE p.download_hash=$1 AND p.revoked_at IS NULL AND r.status='approved'`, hash(token)).Scan(&pid)
	if e != nil {
		fail(w, 404, "pass not found or revoked")
		return
	}
	b, e := a.passPDF(r.Context(), pid)
	if e != nil {
		fail(w, 503, "pass is being prepared; retry shortly")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="Bio-Connect-4.0-pass.pdf"`)
	w.Write(b)
}
func (a *App) pack(ctx context.Context, rid string) ([]byte, error) {
	rows, e := a.DB.Query(ctx, "SELECT id,number FROM passes WHERE registration_id=$1 AND revoked_at IS NULL ORDER BY number", rid)
	if e != nil {
		return nil, e
	}
	type entry struct{ id, number string }
	var all []entry
	for rows.Next() {
		var p entry
		if e = rows.Scan(&p.id, &p.number); e != nil {
			rows.Close()
			return nil, e
		}
		all = append(all, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(all) == 0 {
		return nil, errors.New("no active passes")
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, p := range all {
		pdf, e := a.passPDF(ctx, p.id)
		if e != nil {
			return nil, e
		}
		f, e := z.Create(p.number + ".pdf")
		if e != nil {
			return nil, e
		}
		if _, e = f.Write(pdf); e != nil {
			return nil, e
		}
	}
	if e = z.Close(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}
func (a *App) packDownload(w http.ResponseWriter, r *http.Request) {
	var rid string
	if e := a.DB.QueryRow(r.Context(), "SELECT id FROM registrations WHERE contact_pack_hash=$1 AND status='approved'", hash(r.PathValue("token"))).Scan(&rid); e != nil {
		fail(w, 404, "pack not found or revoked")
		return
	}
	b, e := a.pack(r.Context(), rid)
	if e != nil {
		fail(w, 503, "pack unavailable; retry shortly")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="Bio-Connect-4.0-passes.zip"`)
	w.Write(b)
}
func safeCell(s string) string {
	trim := strings.TrimLeft(s, " \t\r\n")
	if len(trim) > 0 && strings.ContainsAny(trim[:1], "=+-@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
func passLink(base, token string) string { return fmt.Sprintf("%s/passes/%s", base, token) }

// normalizePassNumber accepts a reference or pass number the way people retype
// it - lower case, spaces instead of hyphens, no separators at all - and
// reduces it to the bare symbols so it can be matched against a stored value.
func normalizePassNumber(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
