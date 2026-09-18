# Poster template artwork

Source artwork for the social poster studio in the staff console.
The studio itself needs none of this: a template can be built from a background
colour, an embedded logo, text and a QR.
These files exist so the shipped speaker-reveal template is reproducible rather
than a one-off upload nobody can regenerate.

## Files

| File | |
|---|---|
| `motif-sheet.png` | the parts catalogue: nine motifs on a 3x3 grid, transparent, exact brand palette |
| `compose.py` | builds the two template layers from `motif-sheet.png` |
| `backdrop-4x5.png` | 1080x1350 opaque, drawn **below** the portrait |
| `overlay-4x5.png` | 1080x1350 RGBA, drawn **above** the portrait |
| `speaker-reveal-4x5.json` | the template spec, with the two asset ids left as placeholders |

`motif-sheet.png` was produced with OpenAI image generation, the same route as
`assets/hero-biotech.webp` on the marketing site. Its nine cells, in reading
order, are: an arching palm frond, a fuller frond angled the other way, a
botanical sprig with gold berries, a DNA helix fragment, a molecular
node-and-connector cluster, a hexagonal molecular lattice, a petri dish, flask
and pipette glassware, and a microscopy cell field. `compose.py` cuts each one
from its cell and trims it to its own alpha bounds, so adding a motif to the
sheet makes it available by name.

Only four of the nine are used by the 4:5 speaker reveal. The sprig, glassware,
microscopy field and one molecule are spare, deliberately - they are there for
the session-announce, countdown and sponsor families.

The division of labour is the point: **generate the illustration, compose the
geometry.** Exact pixel placement, a flat photo window and a controlled alpha
channel are the three things image generation is reliably bad at, so the panels,
bands, event bar and photo window are drawn in `compose.py`. Asking a generator
for the whole poster produces garbled text and a photo window you cannot use.

Two gotchas if you regenerate the sheet:

- Motifs must not touch each other or a cell edge, or they cannot be cut apart.
- The sheet arrives with a haze of near-zero alpha and slightly off-palette
  greens. Both are cleaned before it lands here: alpha below 20 is zeroed, and
  fully-opaque pixels are snapped to the nearest brand colour **in int32** -
  a channel difference of 242 squares to 58564, which wraps negative in int16
  and silently maps every dark green to cream.

Regenerate the layers from `motif-sheet.png` rather than editing the PNGs by hand:

```sh
cd registration/artwork && python3 compose.py
```

It needs Pillow, is deterministic (the paper grain is seeded), and asserts the
three things that quietly ruin a poster: the overlay must not cover the face
area, the backdrop's photo window must stay one flat colour, and both layers
must be exactly 1080x1350. The face assertion has already caught a frond that
grew two pixels into the guard.

## Why two layers

Array order in a template spec is draw order. The decoration has to cross in
front of the portrait, and the gold caption band has to straddle the photo's
bottom edge, so both live on the layer above it:

```
backdrop-4x5.png  ->  portrait  ->  overlay-4x5.png  ->  text + QR
```

`backdrop-4x5.png` keeps the photo window (x=115..965, y=195..1005) a single flat
colour, because a photograph covers it.
`overlay-4x5.png` is ~84% transparent and keeps the face area completely clear.

## Installing it as a template

The artwork is template *content*, so it lives in the database and object storage
rather than in the binary. On a fresh environment, upload both layers and create
the template with their returned ids:

```sh
# signed in as staff; $CSRF is the bc_csrf cookie value
for f in backdrop overlay; do
  curl -s -b jar.txt -X POST \
    "$BASE/api/v1/admin/posters/assets?kind=art&label=speaker-reveal-$f-4x5" \
    -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/octet-stream' \
    --data-binary @$f-4x5.png
done
# substitute the two returned ids into speaker-reveal-4x5.json, then:
curl -s -b jar.txt -X POST "$BASE/api/v1/admin/posters/templates" \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
  -d @speaker-reveal-4x5.json
```

Staff can do the same thing through the template builder without any of this;
the JSON is here so the exact shipped layout can be restored.

## Only 4:5 so far

`1x1` (1080x1080) and `9x16` (1080x1920) are not built. Adapting `compose.py` is
the intended route: the palette, the motifs and the layer split all carry over,
only the geometry constants change. The other three template families - session
announce, countdown, sponsor welcome - have no artwork at all yet.
