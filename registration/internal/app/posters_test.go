package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func samplePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func specJSON(layers ...string) json.RawMessage {
	return json.RawMessage(`{"layers":[` + strings.Join(layers, ",") + `]}`)
}

const (
	textLayer  = `{"id":"t1","type":"text","key":"name","x":60,"y":900,"w":700,"h":120,"font":"display","weight":600,"size":58,"color":"#0b3329","align":"left","transform":"none","tracking":0,"line_height":1.15,"autofit":true}`
	photoLayer = `{"id":"p1","type":"photo","key":"photo","x":60,"y":180,"w":960,"h":760,"fit":"cover","radius":0}`
	qrLayer    = `{"id":"q1","type":"qr","key":"qr_url","x":880,"y":1150,"w":140,"h":140}`
)

// --- template validation ---------------------------------------------------

// Every shape the picker offers has to survive a round trip, and the pre-shape
// form - a rect carrying radius = w/2, which is how the seeded session-announce
// slots express a circle - has to stay valid. Losing it would turn every
// circular portrait in production back into a square.
func TestPosterPhotoShapes(t *testing.T) {
	for _, shape := range []string{"", "rect", "rounded", "circle", "arch"} {
		spec := fmt.Sprintf(`{"layers":[{"id":"p","type":"photo","key":"photo","x":0,"y":0,"w":210,"h":210,"fit":"cover","shape":%q}]}`, shape)
		out, err := validateSpec(json.RawMessage(spec), 1080, 1080)
		if err != nil {
			t.Fatalf("rejected shape %q: %v", shape, err)
		}
		if out.Layers[0].Shape != shape {
			t.Errorf("shape %q came back as %q", shape, out.Layers[0].Shape)
		}
	}

	seeded := `{"layers":[{"id":"photo_a","type":"photo","key":"photo_a","x":190,"y":194,"w":210,"h":210,"fit":"cover","radius":105,"duotone":true}]}`
	out, err := validateSpec(json.RawMessage(seeded), 1080, 1080)
	if err != nil {
		t.Fatalf("rejected a seeded circular slot saved before shapes existed: %v", err)
	}
	if out.Layers[0].Shape != "" || out.Layers[0].Radius != 105 {
		t.Errorf("got shape %q radius %v, want the radius left untouched", out.Layers[0].Shape, out.Layers[0].Radius)
	}
}

func TestPosterSpecValidation(t *testing.T) {
	good, err := validateSpec(specJSON(textLayer, photoLayer, qrLayer), 1080, 1350)
	if err != nil {
		t.Fatalf("rejected a valid spec: %v", err)
	}
	if len(good.Layers) != 3 {
		t.Fatalf("kept %d layers, want 3", len(good.Layers))
	}
	// An unset background must not render as transparent; it falls back to the
	// site's page colour.
	if good.Background != "#f3f1e9" {
		t.Fatalf("background defaulted to %q, want #f3f1e9", good.Background)
	}

	for _, c := range []struct{ name, spec string }{
		{"no layers", `{"layers":[]}`},
		{"unknown layer type", `{"layers":[{"id":"x","type":"video","x":0,"y":0,"w":10,"h":10}]}`},
		{"unknown property", `{"layers":[{"id":"x","type":"text","key":"a","x":0,"y":0,"w":10,"h":10,"wobble":3}]}`},
		{"duplicate ids", `{"layers":[` + textLayer + `,` + textLayer + `]}`},
		{"zero size", `{"layers":[{"id":"x","type":"photo","key":"p","x":0,"y":0,"w":0,"h":10,"fit":"cover"}]}`},
		{"art with no source", `{"layers":[{"id":"x","type":"art","x":0,"y":0,"w":10,"h":10}]}`},
		{"art with both sources", `{"layers":[{"id":"x","type":"art","asset_id":"abc","builtin":"ksidc","x":0,"y":0,"w":10,"h":10}]}`},
		{"unknown builtin logo", `{"layers":[{"id":"x","type":"art","builtin":"../../etc/passwd","x":0,"y":0,"w":10,"h":10}]}`},
		{"bad colour", `{"layers":[{"id":"x","type":"text","key":"a","x":0,"y":0,"w":10,"h":10,"font":"body","weight":400,"size":20,"color":"red","align":"left","line_height":1.2}]}`},
		{"bad background", `{"background":"rgb(1,2,3)","layers":[` + textLayer + `]}`},
		{"oversized text", `{"layers":[{"id":"x","type":"text","key":"a","x":0,"y":0,"w":10,"h":10,"font":"body","weight":400,"size":900,"color":"#000000","align":"left","line_height":1.2}]}`},
		{"non-square QR", `{"layers":[{"id":"x","type":"qr","key":"q","x":0,"y":0,"w":100,"h":140}]}`},
		{"photo without a key", `{"layers":[{"id":"x","type":"photo","x":0,"y":0,"w":10,"h":10,"fit":"cover"}]}`},
		{"unknown fit", `{"layers":[{"id":"x","type":"photo","key":"p","x":0,"y":0,"w":10,"h":10,"fit":"stretch"}]}`},
		{"unknown shape", `{"layers":[{"id":"x","type":"photo","key":"p","x":0,"y":0,"w":10,"h":10,"fit":"cover","shape":"star"}]}`},
	} {
		if _, err := validateSpec(json.RawMessage(c.spec), 1080, 1350); err == nil {
			t.Errorf("accepted %s", c.name)
		}
	}

	// The layer cap has to hold, or one template can make the editor unusable.
	many := make([]string, posterLayerMax+1)
	for i := range many {
		many[i] = fmt.Sprintf(`{"id":"l%d","type":"qr","key":"q","x":0,"y":0,"w":10,"h":10}`, i)
	}
	if _, err := validateSpec(specJSON(many...), 1080, 1350); err == nil {
		t.Errorf("accepted %d layers, cap is %d", len(many), posterLayerMax)
	}
}

func TestPosterImageValidation(t *testing.T) {
	if _, _, _, err := validatePosterImage(nil); err == nil {
		t.Fatal("accepted an empty image")
	}
	if _, _, _, err := validatePosterImage([]byte("<svg onload=alert(1)>")); err == nil {
		t.Fatal("accepted an SVG as poster artwork")
	}
	if _, _, _, err := validatePosterImage(bytes.Repeat([]byte{0}, posterUploadMax+1)); err == nil {
		t.Fatal("accepted an upload over the poster cap")
	}
	mime, w, h, err := validatePosterImage(samplePNG(t, 64, 48))
	if err != nil {
		t.Fatalf("rejected a valid PNG: %v", err)
	}
	if mime != "image/png" || w != 64 || h != 48 {
		t.Fatalf("got %s %dx%d, want image/png 64x48", mime, w, h)
	}

	// A long, thin image is barely any pixels but breaks the per-side limit,
	// and the message has to name the side rather than say "oversized".
	_, _, _, err = validatePosterImage(samplePNG(t, posterSideMax+1, 2))
	if err == nil {
		t.Fatalf("accepted an image %d pixels wide", posterSideMax+1)
	}
	if !strings.Contains(err.Error(), "each side") {
		t.Errorf("side-limit error does not say which limit was hit: %v", err)
	}
}

// A 4500x4500 artboard is what a designer exports and is 20.25 MP, which the
// registration path's 20 MP ceiling rejected with "invalid or oversized image".
// That read as a storage fault and cost a round of debugging in production.
func TestPosterImageAcceptsAPrintResolutionArtboard(t *testing.T) {
	if testing.Short() {
		t.Skip("decodes 20 megapixels")
	}
	_, w, h, err := validatePosterImage(samplePNG(t, 4500, 4500))
	if err != nil {
		t.Fatalf("rejected a 4500x4500 artboard: %v", err)
	}
	if w != 4500 || h != 4500 {
		t.Fatalf("got %dx%d, want 4500x4500", w, h)
	}
}

// --- QR --------------------------------------------------------------------

func TestPosterQRRejectsUnsafePayloads(t *testing.T) {
	for _, data := range []string{
		"", "http://reg.bioconnect.kerala.gov.in", "javascript:alert(1)",
		"data:text/html,<script>", "https://x.test/" + strings.Repeat("a", qrPayloadMax),
	} {
		rr := httptest.NewRecorder()
		posterQR(rr, httptest.NewRequest("GET", "/api/v1/admin/posters/qr?data="+data, nil))
		if rr.Code != 400 {
			t.Errorf("payload %q returned %d, want 400", data, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	posterQR(rr, httptest.NewRequest("GET", "/api/v1/admin/posters/qr?data=https%3A%2F%2Freg.bioconnect.kerala.gov.in%2Fdelegates", nil))
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("valid payload returned %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}
	if _, err := png.Decode(bytes.NewReader(rr.Body.Bytes())); err != nil {
		t.Fatalf("QR response is not a decodable PNG: %v", err)
	}
}

// --- storage ---------------------------------------------------------------

// The poster editor draws artwork into a canvas and then reads it back with
// toDataURL. A redirect to a presigned S3 URL taints that canvas and breaks
// every export, so poster assets must always come back as bytes.
func TestPosterAssetIsServedInlineNotRedirected(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "poster-reviewer@bioconnect.test", "reviewer")
	body := samplePNG(t, 120, 90)

	up := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/posters/assets?kind=art&label=backdrop", bytes.NewReader(body))
	a.uploadPosterAsset(up, req, principal{ID: sid, Role: "reviewer"})
	if up.Code != 201 {
		t.Fatalf("upload returned %d: %s", up.Code, up.Body.String())
	}
	var created struct {
		ID            string `json:"id"`
		Width, Height int
	}
	if err := json.Unmarshal(up.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Width != 120 || created.Height != 90 {
		t.Fatalf("stored %dx%d, want 120x90", created.Width, created.Height)
	}

	get := httptest.NewRecorder()
	a.posterAsset(get, httptest.NewRequest("GET", "/api/v1/admin/posters/assets/"+created.ID, nil), created.ID)
	if get.Code != 200 {
		t.Fatalf("asset fetch returned %d, want 200", get.Code)
	}
	if loc := get.Header().Get("Location"); loc != "" {
		t.Fatalf("asset fetch redirected to %q; this taints the export canvas", loc)
	}
	if get.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("content type %q, want image/png", get.Header().Get("Content-Type"))
	}
	if !bytes.Equal(get.Body.Bytes(), body) {
		t.Fatal("served bytes differ from the uploaded image")
	}

	missing := httptest.NewRecorder()
	a.posterAsset(missing, httptest.NewRequest("GET", "/api/v1/admin/posters/assets/nope", nil), "nope")
	if missing.Code != 404 {
		t.Fatalf("unknown asset returned %d, want 404", missing.Code)
	}
}

// --- templates and posters -------------------------------------------------

func saveTemplate(t *testing.T, a *App, sid, family, size string, spec json.RawMessage) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"family": family, "name": "Speaker reveal", "size": size, "spec": spec})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/posters/templates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	a.savePosterTemplate(rr, req, principal{ID: sid, Role: "reviewer"})
	if rr.Code != 200 {
		t.Fatalf("save template returned %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		ID            string `json:"id"`
		Width, Height int
	}
	json.Unmarshal(rr.Body.Bytes(), &out)
	if out.Width == 0 || out.Height == 0 {
		t.Fatal("save template did not return canvas dimensions")
	}
	return out.ID
}

// Relaying out a size must not need a separate delete step, and the partial
// unique index must not start rejecting saves once one exists.
func TestSavingATemplateRetiresThePreviousActiveSize(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "poster-reviewer@bioconnect.test", "reviewer")

	first := saveTemplate(t, a, sid, "speaker-reveal", "4x5", specJSON(textLayer))
	second := saveTemplate(t, a, sid, "speaker-reveal", "4x5", specJSON(textLayer, photoLayer))
	if first == second {
		t.Fatal("second save reused the first template id")
	}
	if n := count(t, a, "SELECT count(*) FROM poster_templates WHERE family='speaker-reveal' AND size='4x5' AND active"); n != 1 {
		t.Fatalf("%d active 4x5 templates, want 1", n)
	}
	if n := count(t, a, "SELECT count(*) FROM poster_templates WHERE id=$1 AND NOT active", first); n != 1 {
		t.Fatal("the superseded template was not retired")
	}

	// Other sizes in the same family are untouched.
	saveTemplate(t, a, sid, "speaker-reveal", "9x16", specJSON(textLayer))
	if n := count(t, a, "SELECT count(*) FROM poster_templates WHERE family='speaker-reveal' AND active"); n != 2 {
		t.Fatalf("%d active sizes in the family, want 2", n)
	}

	bad := httptest.NewRecorder()
	b, _ := json.Marshal(map[string]any{"family": "x", "name": "x", "size": "3x4", "spec": specJSON(textLayer)})
	req := httptest.NewRequest("POST", "/api/v1/admin/posters/templates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	a.savePosterTemplate(bad, req, principal{ID: sid, Role: "reviewer"})
	if bad.Code != 400 {
		t.Fatalf("unknown size returned %d, want 400", bad.Code)
	}
}

func TestPosterRoundTripsValuesAndFraming(t *testing.T) {
	a := mustApp(t)
	sid, _ := addStaff(t, a, "poster-reviewer@bioconnect.test", "reviewer")
	saveTemplate(t, a, sid, "speaker-reveal", "4x5", specJSON(textLayer, photoLayer))

	content := `{"values":{"name":"Dr. Jayakrishna Ambati"},"transforms":{"photo":{"4x5":{"x":-20,"y":-60,"scale":1.18},"9x16":{"x":0,"y":-140,"scale":1.42}}}}`
	b, _ := json.Marshal(map[string]any{"family": "speaker-reveal", "title": "Ambati reveal", "content": json.RawMessage(content)})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/posters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	a.savePoster(rr, req, principal{ID: sid, Role: "reviewer"})
	if rr.Code != 200 {
		t.Fatalf("save poster returned %d: %s", rr.Code, rr.Body.String())
	}
	var saved struct{ ID string }
	json.Unmarshal(rr.Body.Bytes(), &saved)

	get := httptest.NewRecorder()
	a.poster(get, httptest.NewRequest("GET", "/api/v1/admin/posters/"+saved.ID, nil), saved.ID)
	if get.Code != 200 {
		t.Fatalf("read poster returned %d", get.Code)
	}
	var out struct {
		Title   string          `json:"title"`
		Content json.RawMessage `json:"content"`
	}
	json.Unmarshal(get.Body.Bytes(), &out)
	if out.Title != "Ambati reveal" {
		t.Fatalf("title = %q", out.Title)
	}
	// The per-size framing is the whole reason content is stored rather than a
	// rendered file: a 4:5 crop and a 9:16 crop of one portrait differ.
	var round struct {
		Transforms map[string]map[string]struct{ X, Y, Scale float64 } `json:"transforms"`
	}
	if err := json.Unmarshal(out.Content, &round); err != nil {
		t.Fatal(err)
	}
	if round.Transforms["photo"]["4x5"].Scale != 1.18 || round.Transforms["photo"]["9x16"].Y != -140 {
		t.Fatalf("framing did not round-trip: %+v", round.Transforms)
	}

	// Poster saves are attributable, like every other staff action.
	if n := count(t, a, "SELECT count(*) FROM audit_events WHERE staff_id=$1 AND action='poster_saved'", sid); n != 1 {
		t.Fatalf("%d poster_saved audit rows, want 1", n)
	}
}

// --- access ----------------------------------------------------------------

// Posters sit above the reviewer gate, so both roles reach them while the
// registration routes stay separated.
func TestPosterRoutesOpenToBothStaffRolesButNotAnonymously(t *testing.T) {
	a := mustApp(t)
	h := a.Handler()
	ctx := context.Background()

	anon := httptest.NewRecorder()
	h.ServeHTTP(anon, httptest.NewRequest("GET", "/api/v1/admin/posters/templates", nil))
	if anon.Code != 401 {
		t.Fatalf("anonymous poster listing returned %d, want 401", anon.Code)
	}

	for _, role := range []string{"reviewer", "manager"} {
		sid, _ := addStaff(t, a, role+"@bioconnect.test", role)
		token := id() + id()
		if _, err := a.DB.Exec(ctx,
			"INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')",
			hash(token), sid, hash("csrf-"+role)); err != nil {
			t.Fatal(err)
		}
		call := func(path string) int {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", path, nil)
			req.AddCookie(&http.Cookie{Name: "bc_session", Value: token})
			h.ServeHTTP(rr, req)
			return rr.Code
		}
		if code := call("/api/v1/admin/posters/templates"); code != 200 {
			t.Errorf("%s reading poster templates returned %d, want 200", role, code)
		}
		if code := call("/api/v1/admin/posters"); code != 200 {
			t.Errorf("%s listing posters returned %d, want 200", role, code)
		}
		wantRegistrations := 200
		if role == "manager" {
			wantRegistrations = 403
		}
		if code := call("/api/v1/admin/registrations"); code != wantRegistrations {
			t.Errorf("%s reading registrations returned %d, want %d", role, code, wantRegistrations)
		}
	}
}

// Every builtin logo a template may name must actually be embedded, or a saved
// template renders a broken layer in the browser with no server-side error.
func TestBuiltinLogosAreEmbedded(t *testing.T) {
	for name := range builtinLogos {
		if _, err := resources.ReadFile("web/logos/" + name + ".png"); err != nil {
			t.Errorf("builtin logo %q is offered but not embedded: %v", name, err)
		}
	}
	if len(logoNames()) != len(builtinLogos) {
		t.Fatal("logoNames dropped an entry")
	}
	for i := 1; i < len(logoNames()); i++ {
		if logoNames()[i] < logoNames()[i-1] {
			t.Fatal("logoNames is not stably sorted; the picker would reshuffle between loads")
		}
	}
}
