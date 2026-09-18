package app

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strings"
)

// Shipped poster templates. The artwork is template *content*, so it normally
// lives in the database and object storage rather than in the binary - but an
// empty studio on a fresh environment is useless, and asking an operator to
// re-upload 48 PNGs by hand is worse. So the built artwork rides along and
// SeedPosters installs it.
//
// Regenerate with registration/artwork/compose.py; this package only reads.
//
//go:embed artwork
var posterArt embed.FS

// SeedResult is one template's outcome, for the CLI to print.
type SeedResult struct {
	Family, Size, Status string
}

type seedSpec struct {
	Family string          `json:"family"`
	Name   string          `json:"name"`
	Size   string          `json:"size"`
	Spec   json.RawMessage `json:"spec"`
}

// SeedPosters installs every shipped template that is not already present.
//
// It never touches a template that already exists unless replace is set: staff
// can relay a template out in the builder, and a deploy must not silently undo
// that. Assets are keyed by a hash of their bytes, so re-running after changing
// the artwork uploads the new file and leaves the old one for any poster still
// referencing it.
func (a *App) SeedPosters(ctx context.Context, replace bool) ([]SeedResult, error) {
	names, err := fs.Glob(posterArt, "artwork/specs/*.json")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no poster specs embedded; run artwork/compose.py")
	}
	sort.Strings(names)

	out := make([]SeedResult, 0, len(names))
	for _, name := range names {
		raw, err := posterArt.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var s seedSpec
		if err = json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		status, err := a.seedTemplate(ctx, s, replace)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Family+" "+s.Size, err)
		}
		out = append(out, SeedResult{s.Family, s.Size, status})
	}
	return out, nil
}

func (a *App) seedTemplate(ctx context.Context, s seedSpec, replace bool) (string, error) {
	dim, ok := posterSizes[s.Size]
	if !ok {
		return "", fmt.Errorf("unknown size %q", s.Size)
	}

	var existing, origin string
	err := a.DB.QueryRow(ctx,
		"SELECT id,origin FROM poster_templates WHERE family=$1 AND size=$2 AND active",
		s.Family, s.Size).Scan(&existing, &origin)
	if err == nil {
		if !replace {
			return "kept", nil
		}
		// --replace rolls out new artwork; it must not undo a layout staff
		// changed in the builder. Saving an edit sets origin='staff', which is
		// the signal to leave this one alone.
		if origin == "staff" {
			return "kept (edited)", nil
		}
	}

	// Resolve the artwork placeholders to real asset ids before validating, so
	// validateSpec sees exactly what the browser will be handed.
	body := string(s.Spec)
	for placeholder, role := range map[string]string{"__BACKDROP__": "backdrop", "__OVERLAY__": "overlay"} {
		if !strings.Contains(body, placeholder) {
			continue
		}
		id, err := a.seedAsset(ctx, fmt.Sprintf("artwork/%s-%s-%s.png", s.Family, s.Size, role),
			fmt.Sprintf("%s %s %s", s.Family, s.Size, role))
		if err != nil {
			return "", err
		}
		body = strings.ReplaceAll(body, placeholder, id)
	}

	spec, err := validateSpec(json.RawMessage(body), dim[0], dim[1])
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx,
		"UPDATE poster_templates SET active=false,updated_at=now() WHERE family=$1 AND size=$2 AND active",
		s.Family, s.Size); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx,
		"INSERT INTO poster_templates(id,family,name,size,width,height,spec,origin) VALUES($1,$2,$3,$4,$5,$6,$7,'seed')",
		id(), s.Family, s.Name, s.Size, dim[0], dim[1], encoded); err != nil {
		return "", err
	}
	if err = audit(ctx, tx, "", "", "poster_template_seeded", s.Family+" "+s.Size); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	if existing != "" {
		return "replaced", nil
	}
	return "created", nil
}

// seedAsset stores one embedded PNG, reusing the row from a previous run when
// the bytes are unchanged. The label carries the content hash, so identical
// artwork is never uploaded twice and changed artwork always lands as a new
// object rather than mutating one a saved poster may already point at.
func (a *App) seedAsset(ctx context.Context, path, label string) (string, error) {
	b, err := posterArt.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("embedded artwork %s: %w", path, err)
	}
	sum := sha256.Sum256(b)
	tag := "seed:" + label + ":" + hex.EncodeToString(sum[:])[:16]

	// Reuse a previous run's row only if its object is still readable. A row
	// whose bytes have gone - a restore that brought the database back without
	// the bucket, a storage directory that moved - would otherwise be handed
	// out forever and every poster would render without its artwork.
	var existing, key string
	if err := a.DB.QueryRow(ctx,
		"SELECT id,object_key FROM poster_assets WHERE label=$1", tag).Scan(&existing, &key); err == nil {
		if _, err := a.Storage.Get(ctx, key); err == nil {
			return existing, nil
		}
		if err := a.Storage.Put(ctx, key, b, http.DetectContentType(b)); err != nil {
			return "", fmt.Errorf("restoring %s: %w", path, err)
		}
		return existing, nil
	}

	mime := http.DetectContentType(b)
	if mime != "image/png" {
		return "", fmt.Errorf("%s is %s, want image/png", path, mime)
	}
	_, w, h, err := validatePosterImage(b)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	aid := id()
	if err = a.Storage.Put(ctx, aid, b, mime); err != nil {
		return "", err
	}
	_, err = a.DB.Exec(ctx,
		"INSERT INTO poster_assets(id,kind,label,object_key,mime,width,height,size) VALUES($1,'art',$2,$1,$3,$4,$5,$6)",
		aid, tag, mime, w, h, len(b))
	if err != nil {
		return "", err
	}
	return aid, nil
}
