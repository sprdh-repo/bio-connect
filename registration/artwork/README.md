# Poster template artwork

Source artwork for the social poster studio in the staff console.
The studio itself needs none of this: a template can be built from a background
colour, an embedded logo, text and a QR.
These files exist so the shipped speaker-reveal template is reproducible rather
than a one-off upload nobody can regenerate.

## Files

| File | |
|---|---|
| `motif-sheet.png`, `motif-sheet-2.png` | the parts catalogues: nine motifs each on a 3x3 grid, transparent, exact brand palette |
| `extract.py`, `extract2.py` | clean a freshly generated sheet into the committed one |
| `compose.py` | builds every template's artwork **and** its spec |
| `../internal/app/artwork/` | the 48 generated PNGs and 24 specs, embedded in the binary |

Eighteen motifs, in sheet order:

1. arching palm frond, fuller frond, botanical sprig with gold berries
2. DNA helix fragment, molecular cluster, hexagonal lattice
3. petri dish, flask and pipette, microscopy cell field
4. coconut palm tree, backwater waves, rising sun
5. hourglass, clock, microscope
6. award rosette, laurel arc, network flow

`compose.py` cuts each from its cell by name and trims it to its own alpha
bounds, so adding a motif to a sheet makes it available everywhere.

## What it builds

Four post types, each in a light and a dark look, each at three sizes - 24
templates, 48 PNGs, about 4.4 MB embedded:

```
speaker-reveal-{light,dark}     portrait, name, designation
session-announce-{light,dark}   two circular portraits, session title, time
countdown-{light,dark}          a numeral, no photograph
sponsor-welcome-{light,dark}    a partner logo on a light card
                                x  {4x5, 1x1, 9x16}
```

Two looks are two *families* rather than one family with a variant column,
because the schema has `UNIQUE (family, size) WHERE active`. The studio lists
them as separate pickable families, which is also how staff think about them.

## Regenerating

```sh
cd registration/artwork && python3 compose.py
```

Pillow only, deterministic (the grain is seeded). It asserts the failures that
quietly ruin a poster rather than leaving them to the eye:

- both layers are exactly the canvas size,
- every photo slot sits on **one** flat colour **under its clip shape** - a
  circular slot is meant to leave its bounding box's corners showing,
- no overlay decoration covers the middle of a photo slot,
- no content layer falls outside the canvas,
- the event furniture all has defaults.

Those checks have already caught a frond two pixels into a face guard, a corner
motif reaching into the photo window at 1x1, and a flow diagram that covered a
face once the square crop shrank the circles.

## The division of labour

**Generate the illustration, compose the geometry.** Exact pixel placement, a
flat photo window and a controlled alpha channel are the three things image
generation is reliably bad at, so panels, bands, the event bar and the photo
windows are drawn in `compose.py`. Asking a generator for a whole poster
produces garbled text and a photo window you cannot use.

Three traps, all of which cost real time here:

- Motifs must not touch each other or a cell edge, or they cannot be cut apart.
- A sheet arrives with a haze of near-zero alpha and slightly off-palette
  greens. Alpha below 20 is zeroed and opaque pixels are snapped to the nearest
  brand colour **in int32** - a channel difference of 242 squares to 58564,
  which wraps negative in int16 and silently maps every dark green to cream.
- Text placed against a slot's box lands on the mounting panels, which bleed
  well past it. `panels()` returns its real bottom for that reason.

## Installing

The binary carries the artwork; `bioconnect poster-seed` installs it. See the
"Social posters" section of [`../README.md`](../README.md).
