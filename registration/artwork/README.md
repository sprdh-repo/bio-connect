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

## The designer's poster (`official.py`)

`speaker-official-light` and `-dark` are not generated art - they are the
designer's own Illustrator poster, lifted so the lettering, the logos and the
geometry stay exactly as drawn. The type is Konsens, embedded as outlines and
not a font we hold, so redrawing it was never an option.

| Source | |
|---|---|
| `speaker-poster-blank.pdf` | the designer's empty template. **Preferred.** 1 page, the dark look. |
| `speaker-poster-filled.pdf` | an earlier filled sample, 2 pages. The light look is derived from its page 1. |

```sh
python3 official.py \
  'dark=speaker-poster-blank.pdf#1#blank' \
  'light=speaker-poster-filled.pdf#1'
```

`#blank` says the source is already empty, so only the sample QR is cleared.
Without it the portrait is removed and the name card is repainted too.

Removing the portrait is done in the PDF, not the pixels: `qpdf --qdf` leaves the
content streams uncompressed, so its draw operator is overwritten with spaces of
the same length, which keeps every byte offset and `/Length` valid. It is matched
by its placement matrix rather than its name - it is `/Im0` on both pages, but so
are the logos inside their own nested form XObjects, and blanking those strips
the logos.

### Why two layers, and three traps in splitting them

The badge, the name card and the QR card are all drawn **over** the portrait, so
they cannot sit in the backdrop. The split is backdrop = background + lime arch,
overlay = everything else. Each of these cost a wrong render:

- **The arch mask.** A flood fill from the border leaks through any notch a card
  opens at the arch's edge and eats card-shaped bites out of it. A plain colour
  test also catches lime in the logo and the bottom bar, stretching a span fill
  far past the arch. It takes the largest connected lime region, span-filled
  inside its own bounding box.
- **The arch's anti-aliased rim.** Those pixels are background-to-lime blends;
  left in the overlay they draw over the portrait as a pale halo. They belong to
  the backdrop - but only on the rim, because pure lime matches the same test and
  a wider neighbourhood punches the bottom bar's lime panel out of the overlay.
- **Elements are regions, not pixels.** The dark look's bar text is the page's
  own background colour, so a per-pixel difference test scores those glyphs as
  backdrop and the portrait shows through the lettering. Closing and hole-filling
  keeps each card, bar and logo solid.

### What this template expects

The portrait slot is a **cut-out**: upload a PNG with the background removed, as
the designer's own sample is. A rectangular photo fills the slot and covers the
arch. Duotone is off - the design uses a plain greyscale portrait.

The name, designation, organisation and topic render in Manrope, not Konsens.
That is the one visible departure from the original and the reason the name card
is the only region that does not match it closely.

Only 1:1 so far, and only the dark look comes from the designer's blank; the
light look is derived from the older filled sample and so is missing the bio360
logo that the blank carries. Ask for the light blank and rerun to fix that.
