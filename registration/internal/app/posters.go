package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"

	"github.com/skip2/go-qrcode"
)

// Social posters. A template is an ordered layer stack that staff lay out once
// on the uploaded artwork; a poster is a set of field values filled against one
// template family and exported at every size in it.
//
// Rendering happens in the browser, on a canvas at true output resolution, so
// the preview a staff member drags a portrait around in is the file they
// download. The server stores the definitions and the artwork and nothing else
// renders here, which is why there is no image encoder in go.mod.

const (
	posterUploadMax = 8 << 20 // artwork at 1080x1920 can exceed the 5 MB registration cap
	posterLayerMax  = 40
	qrPayloadMax    = 512
)

// builtinLogos are the brand marks shipped in the binary under web/logos, so a
// template can carry the partner header row without anyone uploading anything.
// Anything outside this list is rejected: a layer's builtin name reaches the
// browser as a /static/logos/ path.
//
// The Government of Kerala emblem is deliberately absent. The copy in the site's
// assets/ is Wikimedia's, CC BY-SA 4.0, which the marketing site satisfies by
// naming the photographer and linking the licence in committee.html. A social
// image has nowhere to carry that, and ShareAlike would reach the poster itself.
// Staff who need the official emblem upload it as artwork instead.
var builtinLogos = map[string]bool{
	"bio-connect": true, "bio-connect-mark": true, "bio360": true,
	"ksidc": true, "klip": true, "invest-kerala": true,
}

var posterSizes = map[string][2]int{"4x5": {1080, 1350}, "1x1": {1080, 1080}, "9x16": {1080, 1920}}

type posterLayer struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	AssetID string  `json:"asset_id,omitempty"`
	Builtin string  `json:"builtin,omitempty"`
	Key     string  `json:"key,omitempty"`
	Label   string  `json:"label,omitempty"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`

	// photo
	Fit     string  `json:"fit,omitempty"`
	Radius  float64 `json:"radius,omitempty"`
	Duotone bool    `json:"duotone,omitempty"`

	// text
	Font       string  `json:"font,omitempty"`
	Weight     int     `json:"weight,omitempty"`
	Size       float64 `json:"size,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	Transform  string  `json:"transform,omitempty"`
	Tracking   float64 `json:"tracking,omitempty"`
	LineHeight float64 `json:"line_height,omitempty"`
	Autofit    bool    `json:"autofit,omitempty"`
}

type posterSpec struct {
	Background string            `json:"background,omitempty"`
	Layers     []posterLayer     `json:"layers"`
	Defaults   map[string]string `json:"defaults,omitempty"`
}

func (a *App) posterAPI(w http.ResponseWriter, r *http.Request, p principal, path string) {
	head, rest, _ := strings.Cut(path, "/")
	switch {
	case head == "templates" && rest == "" && r.Method == "GET":
		a.posterTemplates(w, r)
	case head == "templates" && rest == "" && r.Method == "POST":
		a.savePosterTemplate(w, r, p)
	case head == "templates" && rest == "duplicate" && r.Method == "POST":
		a.duplicatePosterFamily(w, r, p)
	case head == "templates" && rest == "retire" && r.Method == "POST":
		a.retirePosterTemplates(w, r, p)
	case head == "assets" && rest == "" && r.Method == "POST":
		a.uploadPosterAsset(w, r, p)
	case head == "assets" && rest == "" && r.Method == "GET":
		a.posterAssets(w, r)
	case head == "assets" && rest != "" && r.Method == "GET":
		a.posterAsset(w, r, rest)
	case head == "qr" && r.Method == "GET":
		posterQR(w, r)
	case head == "" && r.Method == "GET":
		a.posterList(w, r)
	case head == "" && r.Method == "POST":
		a.savePoster(w, r, p)
	case head != "" && rest == "" && r.Method == "GET":
		a.poster(w, r, head)
	default:
		fail(w, 404, "unknown poster route")
	}
}

// validateSpec keeps a stored template renderable. The browser draws whatever
// it is handed, so bounds and enums are checked once here rather than being
// re-guessed by the renderer on every draw.
func validateSpec(raw json.RawMessage, width, height int) (posterSpec, error) {
	var s posterSpec
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&s); e != nil {
		// Staff-only endpoint describing the caller's own payload, so the
		// decoder's message is more use than a generic one when a layer
		// property and this struct drift apart.
		return s, fmt.Errorf("invalid template layers: %v", e)
	}
	if len(s.Layers) == 0 {
		return s, errors.New("a template needs at least one layer")
	}
	if len(s.Layers) > posterLayerMax {
		return s, fmt.Errorf("a template holds at most %d layers", posterLayerMax)
	}
	if s.Background == "" {
		s.Background = "#f3f1e9" // --cream, the site's page colour
	}
	if !hexColor(s.Background) {
		return s, errors.New("background colour must be #rrggbb")
	}
	if len(s.Defaults) > 40 {
		return s, errors.New("too many default values")
	}
	for k, v := range s.Defaults {
		if len(k) > 64 || len(v) > 600 {
			return s, errors.New("default values are capped at 600 characters")
		}
	}
	seen := map[string]bool{}
	for i := range s.Layers {
		l := &s.Layers[i]
		if l.ID == "" || len(l.ID) > 64 || seen[l.ID] {
			return s, errors.New("each layer needs a unique id")
		}
		seen[l.ID] = true
		if len(l.Label) > 64 {
			return s, errors.New("layer labels are capped at 64 characters")
		}
		// A layer may bleed off the canvas, but not far enough to be lost.
		for _, v := range []float64{l.X, l.Y} {
			if v < float64(-2*max(width, height)) || v > float64(2*max(width, height)) {
				return s, errors.New("layer position is off the canvas")
			}
		}
		if l.W <= 0 || l.H <= 0 || l.W > float64(4*width) || l.H > float64(4*height) {
			return s, errors.New("layer size is out of range")
		}
		switch l.Type {
		case "art":
			if (l.AssetID == "") == (l.Builtin == "") {
				return s, errors.New("an art layer needs either an uploaded asset or a builtin logo")
			}
			if l.Builtin != "" && !builtinLogos[l.Builtin] {
				return s, errors.New("unknown builtin logo")
			}
		case "photo":
			if l.Key == "" || len(l.Key) > 64 {
				return s, errors.New("a photo layer needs a field key")
			}
			if l.Fit != "cover" && l.Fit != "contain" {
				return s, errors.New(`photo fit must be "cover" or "contain"`)
			}
			if l.Radius < 0 || l.Radius > l.W || l.Radius > l.H {
				return s, errors.New("photo corner radius is out of range")
			}
		case "text":
			if l.Key == "" || len(l.Key) > 64 {
				return s, errors.New("a text layer needs a field key")
			}
			if l.Font != "display" && l.Font != "body" {
				return s, errors.New(`text font must be "display" or "body"`)
			}
			if l.Weight != 400 && l.Weight != 500 && l.Weight != 600 && l.Weight != 700 {
				return s, errors.New("text weight must be 400, 500, 600 or 700")
			}
			if l.Size < 6 || l.Size > 400 {
				return s, errors.New("text size must be between 6 and 400")
			}
			if !hexColor(l.Color) {
				return s, errors.New("text colour must be #rrggbb")
			}
			switch l.Align {
			case "left", "center", "right":
			default:
				return s, errors.New(`text align must be "left", "center" or "right"`)
			}
			switch l.Transform {
			case "", "none", "upper":
			default:
				return s, errors.New(`text transform must be "none" or "upper"`)
			}
			if l.Tracking < -0.2 || l.Tracking > 1 {
				return s, errors.New("letter spacing is out of range")
			}
			if l.LineHeight < 0.6 || l.LineHeight > 3 {
				return s, errors.New("line height must be between 0.6 and 3")
			}
		case "qr":
			if l.Key == "" || len(l.Key) > 64 {
				return s, errors.New("a QR layer needs a field key")
			}
			if l.W != l.H {
				return s, errors.New("a QR layer must be square")
			}
		default:
			return s, errors.New("unknown layer type")
		}
	}
	return s, nil
}

func hexColor(v string) bool {
	if len(v) != 7 || v[0] != '#' {
		return false
	}
	for _, c := range v[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func (a *App) posterTemplates(w http.ResponseWriter, r *http.Request) {
	items, e := a.queryMaps(r, "SELECT id,family,name,size,width,height,spec,origin,updated_at FROM poster_templates WHERE active ORDER BY family,CASE size WHEN '4x5' THEN 1 WHEN '1x1' THEN 2 ELSE 3 END")
	if e != nil {
		fail(w, 503, "templates unavailable")
		return
	}
	respond(w, 200, map[string]any{"items": items, "builtin_logos": logoNames(), "sizes": posterSizes})
}

func logoNames() []string {
	out := make([]string, 0, len(builtinLogos))
	for name := range builtinLogos {
		out = append(out, name)
	}
	// Map iteration is random; the picker should not reshuffle between loads.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (a *App) savePosterTemplate(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		ID     string          `json:"id"`
		Family string          `json:"family"`
		Name   string          `json:"name"`
		Size   string          `json:"size"`
		Spec   json.RawMessage `json:"spec"`
	}
	if !decode(w, r, &in) {
		return
	}
	dim, ok := posterSizes[in.Size]
	if !ok {
		fail(w, 400, "size must be 4x5, 1x1 or 9x16")
		return
	}
	in.Family, in.Name = strings.TrimSpace(in.Family), strings.TrimSpace(in.Name)
	if in.Family == "" || len(in.Family) > 64 || in.Name == "" || len(in.Name) > 120 {
		fail(w, 400, "give the template a family and a name")
		return
	}
	spec, e := validateSpec(in.Spec, dim[0], dim[1])
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	encoded, e := json.Marshal(spec)
	if e != nil {
		fail(w, 400, "invalid template layers")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "save unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	tid := in.ID
	if tid == "" {
		tid = id()
		// The partial unique index keeps one active template per family and
		// size; retire the previous one rather than rejecting the save, so
		// relaying out a template is not a two-step dance for staff.
		if _, e = tx.Exec(r.Context(), "UPDATE poster_templates SET active=false,updated_at=now() WHERE family=$1 AND size=$2 AND active", in.Family, in.Size); e != nil {
			fail(w, 503, "save unavailable")
			return
		}
		_, e = tx.Exec(r.Context(), "INSERT INTO poster_templates(id,family,name,size,width,height,spec,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", tid, in.Family, in.Name, in.Size, dim[0], dim[1], encoded, p.ID)
	} else {
		// Editing a shipped template makes it staff-owned. Without this the
		// next "poster-seed --replace" would quietly revert the change.
		ct, ee := tx.Exec(r.Context(), "UPDATE poster_templates SET name=$2,spec=$3,origin='staff',updated_at=now() WHERE id=$1 AND active", tid, in.Name, encoded)
		e = ee
		if e == nil && ct.RowsAffected() == 0 {
			fail(w, 404, "template not found")
			return
		}
	}
	if e == nil {
		e = audit(r.Context(), tx, p.ID, "", "poster_template_saved", in.Family+" "+in.Size)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "template could not be saved; retry")
		return
	}
	respond(w, 200, map[string]any{"id": tid, "width": dim[0], "height": dim[1]})
}

// validatePosterImage mirrors validateUpload's checks but returns the decoded
// dimensions the layout needs, and allows the larger artwork cap. Registration
// uploads keep their own 5 MB limit.
func validatePosterImage(b []byte) (string, int, int, error) {
	if len(b) == 0 || len(b) > posterUploadMax {
		return "", 0, 0, fmt.Errorf("image must be between 1 byte and %d MB", posterUploadMax>>20)
	}
	mime := http.DetectContentType(b)
	if mime != "image/png" && mime != "image/jpeg" {
		return "", 0, 0, errors.New("upload a PNG or JPEG image")
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 6000 || cfg.Height > 6000 || int64(cfg.Width)*int64(cfg.Height) > 20000000 {
		return "", 0, 0, errors.New("invalid or oversized image")
	}
	if _, _, e = image.Decode(bytes.NewReader(b)); e != nil {
		return "", 0, 0, errors.New("invalid image")
	}
	return mime, cfg.Width, cfg.Height, nil
}

func (a *App) uploadPosterAsset(w http.ResponseWriter, r *http.Request, p principal) {
	kind := r.URL.Query().Get("kind")
	if kind != "art" && kind != "photo" && kind != "logo" {
		fail(w, 400, "kind must be art, photo or logo")
		return
	}
	label := strings.TrimSpace(r.URL.Query().Get("label"))
	if len(label) > 120 {
		label = label[:120]
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, posterUploadMax))
	if e != nil {
		fail(w, 413, fmt.Sprintf("image exceeds %d MB", posterUploadMax>>20))
		return
	}
	mime, iw, ih, e := validatePosterImage(b)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	var count int
	if e = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM poster_assets").Scan(&count); e != nil || count >= 2000 {
		fail(w, 400, "poster asset limit reached; delete unused artwork")
		return
	}
	aid := id()
	if e = a.Storage.Put(r.Context(), aid, b, mime); e != nil {
		fail(w, 503, "upload failed; please retry")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "upload unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	_, e = tx.Exec(r.Context(), "INSERT INTO poster_assets(id,kind,label,object_key,mime,width,height,size,created_by) VALUES($1,$2,$3,$1,$4,$5,$6,$7,$8)", aid, kind, label, mime, iw, ih, len(b), p.ID)
	if e == nil {
		e = audit(r.Context(), tx, p.ID, "", "poster_asset_uploaded", kind+" "+label)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "upload could not be saved; retry")
		return
	}
	respond(w, 201, map[string]any{"id": aid, "width": iw, "height": ih, "mime": mime})
}

func (a *App) posterAssets(w http.ResponseWriter, r *http.Request) {
	q := "SELECT id,kind,label,width,height,created_at FROM poster_assets"
	args := []any{}
	if kind := r.URL.Query().Get("kind"); kind != "" {
		q += " WHERE kind=$1"
		args = append(args, kind)
	}
	items, e := a.queryMaps(r, q+" ORDER BY created_at DESC,id LIMIT 200", args...)
	if e != nil {
		fail(w, 503, "assets unavailable")
		return
	}
	respond(w, 200, map[string]any{"items": items})
}

func (a *App) posterAsset(w http.ResponseWriter, r *http.Request, aid string) {
	var key, mime string
	if e := a.DB.QueryRow(r.Context(), "SELECT object_key,mime FROM poster_assets WHERE id=$1", aid).Scan(&key, &mime); e != nil {
		fail(w, 404, "asset not found")
		return
	}
	a.serveInline(w, r, key, mime)
}

// posterQR reuses the pass encoder so a poster's code and a pass's code are
// produced by the same library at the same error-correction level. It is served
// same-origin for the canvas, like every other poster image.
func posterQR(w http.ResponseWriter, r *http.Request) {
	data := r.URL.Query().Get("data")
	if !strings.HasPrefix(data, "https://") || len(data) > qrPayloadMax {
		fail(w, 400, fmt.Sprintf("QR data must be an https URL of at most %d characters", qrPayloadMax))
		return
	}
	png, e := qrcode.Encode(data, qrcode.Medium, 720)
	if e != nil {
		fail(w, 400, "could not encode this QR payload")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

func (a *App) posterList(w http.ResponseWriter, r *http.Request) {
	items, e := a.queryMaps(r, "SELECT p.id,p.family,p.title,p.updated_at,s.email AS created_by FROM posters p LEFT JOIN staff s ON s.id=p.created_by ORDER BY p.updated_at DESC,p.id LIMIT 100")
	if e != nil {
		fail(w, 503, "posters unavailable")
		return
	}
	respond(w, 200, map[string]any{"items": items})
}

func (a *App) poster(w http.ResponseWriter, r *http.Request, pid string) {
	var family, title string
	var content json.RawMessage
	if e := a.DB.QueryRow(r.Context(), "SELECT family,title,content FROM posters WHERE id=$1", pid).Scan(&family, &title, &content); e != nil {
		fail(w, 404, "poster not found")
		return
	}
	respond(w, 200, map[string]any{"id": pid, "family": family, "title": title, "content": content})
}

func (a *App) savePoster(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		ID      string          `json:"id"`
		Family  string          `json:"family"`
		Title   string          `json:"title"`
		Content json.RawMessage `json:"content"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Family, in.Title = strings.TrimSpace(in.Family), strings.TrimSpace(in.Title)
	if in.Family == "" || len(in.Family) > 64 || len(in.Title) > 200 {
		fail(w, 400, "give the poster a family and a title of at most 200 characters")
		return
	}
	var probe map[string]json.RawMessage
	if e := json.Unmarshal(in.Content, &probe); e != nil {
		fail(w, 400, "poster values must be a JSON object")
		return
	}
	if len(probe) > 60 {
		fail(w, 400, "too many poster fields")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "save unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	pid := in.ID
	if pid == "" {
		pid = id()
		_, e = tx.Exec(r.Context(), "INSERT INTO posters(id,family,title,content,created_by) VALUES($1,$2,$3,$4,$5)", pid, in.Family, in.Title, in.Content, p.ID)
	} else {
		ct, ee := tx.Exec(r.Context(), "UPDATE posters SET title=$2,content=$3,updated_at=now() WHERE id=$1", pid, in.Title, in.Content)
		e = ee
		if e == nil && ct.RowsAffected() == 0 {
			fail(w, 404, "poster not found")
			return
		}
	}
	if e == nil {
		e = audit(r.Context(), tx, p.ID, "", "poster_saved", in.Family+": "+in.Title)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "poster could not be saved; retry")
		return
	}
	respond(w, 200, map[string]string{"id": pid})
}

// duplicatePosterFamily copies every active size of one family into a new one.
// It is how staff customise a shipped template safely: the copy is staff-owned
// from birth, so no rollout of new artwork can touch it, and the original stays
// available to everyone else.
func (a *App) duplicatePosterFamily(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		Family string `json:"family"`
		Name   string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Family == "" || in.Name == "" || len(in.Name) > 120 {
		fail(w, 400, "choose a template and give the copy a name")
		return
	}
	// The new family slug comes from the name, so staff never type one.
	slug := slugify(in.Name)
	if slug == "" {
		fail(w, 400, "give the copy a name with some letters or digits in it")
		return
	}

	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "duplicate unavailable")
		return
	}
	defer tx.Rollback(r.Context())

	var clash int
	if e = tx.QueryRow(r.Context(), "SELECT count(*) FROM poster_templates WHERE family=$1 AND active", slug).Scan(&clash); e != nil {
		fail(w, 503, "duplicate unavailable")
		return
	}
	if clash > 0 {
		fail(w, 409, "a template called that already exists; pick another name")
		return
	}

	rows, e := tx.Query(r.Context(), "SELECT size,width,height,spec FROM poster_templates WHERE family=$1 AND active", in.Family)
	if e != nil {
		fail(w, 503, "duplicate unavailable")
		return
	}
	type copyOf struct {
		size          string
		width, height int
		spec          []byte
	}
	var sizes []copyOf
	for rows.Next() {
		var c copyOf
		if e = rows.Scan(&c.size, &c.width, &c.height, &c.spec); e != nil {
			rows.Close()
			fail(w, 503, "duplicate unavailable")
			return
		}
		sizes = append(sizes, c)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		fail(w, 503, "duplicate unavailable")
		return
	}
	if len(sizes) == 0 {
		fail(w, 404, "that template has no sizes to copy")
		return
	}

	// The copy points at the same artwork rows. Nothing mutates an asset, so
	// sharing them is safe and avoids duplicating megabytes per copy.
	for _, c := range sizes {
		if _, e = tx.Exec(r.Context(),
			"INSERT INTO poster_templates(id,family,name,size,width,height,spec,origin,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,'staff',$8)",
			id(), slug, in.Name, c.size, c.width, c.height, c.spec, p.ID); e != nil {
			fail(w, 503, "duplicate could not be saved; retry")
			return
		}
	}
	if e = audit(r.Context(), tx, p.ID, "", "poster_family_duplicated", in.Family+" -> "+slug); e != nil {
		fail(w, 503, "duplicate could not be saved; retry")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 503, "duplicate could not be saved; retry")
		return
	}
	respond(w, 200, map[string]any{"family": slug, "name": in.Name, "sizes": len(sizes)})
}

// retirePosterTemplates hides a template without deleting it: posters already
// made from it keep their saved values, and the row stays for the audit trail.
func (a *App) retirePosterTemplates(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		Family string `json:"family"`
		Size   string `json:"size"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Family == "" {
		fail(w, 400, "choose a template to retire")
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "retire unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	q := "UPDATE poster_templates SET active=false,updated_at=now() WHERE family=$1 AND active"
	args := []any{in.Family}
	if in.Size != "" {
		q += " AND size=$2"
		args = append(args, in.Size)
	}
	ct, e := tx.Exec(r.Context(), q, args...)
	if e != nil {
		fail(w, 503, "retire unavailable")
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "template not found")
		return
	}
	if e = audit(r.Context(), tx, p.ID, "", "poster_template_retired", in.Family+" "+in.Size); e != nil {
		fail(w, 503, "retire could not be saved; retry")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 503, "retire could not be saved; retry")
		return
	}
	respond(w, 200, map[string]any{"retired": ct.RowsAffected()})
}

// slugify turns a staff-typed name into a family key. Mirrors the client's own
// slug so the name they type and the family they get agree.
func slugify(v string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(v) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
