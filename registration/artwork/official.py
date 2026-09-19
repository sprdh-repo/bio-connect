#!/usr/bin/env python3
"""Turn the designer's "Speaker poster" PDF into studio template layers.

The PDF (Adobe Illustrator, 1080x1080, two pages: a light and a dark look) is
the real Bio Connect 4.0 speaker poster. This does not redraw it - it lifts the
artwork so the lettering, the logos and the geometry stay exactly as drawn,
including the Konsens type that is embedded as outlines and that we do not have
as a font.

Three things have to come out, because they are per-poster content rather than
template furniture: the speaker's cut-out portrait, the name/designation text in
the card, and the QR (the studio generates its own so the URL is editable).

The split into two layers matters: the badge, the name card and the QR card are
all drawn *over* the portrait, so they cannot live in the backdrop.

  backdrop = flat background + the lime arch
  overlay  = everything else, transparent elsewhere

Run from this directory, with the PDF path as the argument:

    python3 official.py "~/Downloads/Speaker poster.pdf"

Outputs into ../internal/app/artwork/ like compose.py does.
"""
from PIL import Image, ImageDraw
from scipy import ndimage
import json
import numpy as np
import os
import re
import subprocess
import sys
import tempfile

OUT = os.path.join("..", "internal", "app", "artwork")
LIME = (185, 220, 114)

# Measured from the artwork itself; see README for how.
PHOTO = (240, 248, 831, 986)      # the portrait's own placement in the PDF
CARD = (734, 467, 986, 686)       # name / designation / topic card
QR_CARD = (149, 618, 324, 860)    # white card with the SCAN & REGISTER tab
QR = (157, 670, 310, 823)         # the code itself, measured off the artwork
LOOKS = {
    # page, page background, colour of the name card's fill
    "light": (1, (250, 244, 232), (23, 72, 60)),
    "dark":  (2, (23, 72, 60), (250, 244, 232)),
}


def render_without_portrait(pdf, workdir):
    """Blank the portrait's draw operator, then rasterise both pages at 1080.

    qpdf --qdf leaves the content streams uncompressed, so the operator can be
    overwritten with spaces of the same length. Keeping the byte count identical
    means every offset and /Length in the file stays valid.

    The portrait is matched by its placement matrix, not by its XObject name:
    it is /Im0 on both pages, but so are the logos inside their own nested form
    XObjects, and blanking those would strip the logos too.
    """
    qdf = os.path.join(workdir, "qdf.pdf")
    subprocess.run(["qpdf", "--qdf", "--object-streams=disable", pdf, qdf],
                   check=False, capture_output=True)
    data = bytearray(open(qdf, "rb").read())
    pat = re.compile(rb"([-\d.]+) 0 0 ([-\d.]+) ([-\d.]+) ([-\d.]+) cm\s*"
                     rb"(?:/[A-Za-z0-9]+ gs\s*)?/Im0 Do")
    hits = 0
    for m in pat.finditer(bytes(data)):
        w, h = float(m.group(1)), float(m.group(2))
        if not (580 < w < 600 and 730 < h < 745):
            continue
        seg = bytes(data[m.start():m.end()])
        i = m.start() + seg.index(b"/Im0 Do")
        data[i:i + 7] = b"       "
        hits += 1
    # A source the designer has already emptied has no portrait to remove.
    if hits not in (0, 1, 2):
        raise SystemExit(f"unexpected portrait draw count: {hits}")
    stripped = os.path.join(workdir, "nophoto.pdf")
    open(stripped, "wb").write(bytes(data))
    subprocess.run(["pdftoppm", "-r", "72", "-png", "-scale-to", "1080",
                    stripped, os.path.join(workdir, "p")], check=True)
    return [os.path.join(workdir, f"p-{i}.png") for i in (1, 2)]


def arch_mask(rgb):
    """The lime arch, with the cards that sit on it filled back in.

    Two traps. A flood fill from the border is not enough: a card touching the
    arch's edge opens a notch to the outside, so the flood eats card-shaped
    bites out of it. And a plain colour test also catches lime in the logo and
    in the bottom bar's rule, which stretches any span fill far past the arch.

    So: largest connected lime region, then fill row and column spans inside its
    bounding box only. The arch is one convex-ish blob, so that restores an edge
    a card covers while the rounded top still follows each row's own span.
    """
    lime = np.abs(rgb.astype(int) - np.array(LIME)).sum(axis=2) < 30
    labels, n = ndimage.label(lime)
    if n == 0:
        raise SystemExit("no lime arch found")
    sizes = ndimage.sum(lime, labels, range(1, n + 1))
    arch = labels == (int(np.argmax(sizes)) + 1)

    ys, xs = np.nonzero(arch)
    y0, y1, x0, x1 = ys.min(), ys.max() + 1, xs.min(), xs.max() + 1
    box = arch[y0:y1, x0:x1]
    filled = np.zeros_like(box)
    cols = np.arange(box.shape[1])
    for y in range(box.shape[0]):
        if box[y].any():
            r = cols[box[y]]
            filled[y, r.min():r.max() + 1] = True
    rows = np.arange(box.shape[0])
    for x in range(box.shape[1]):
        if box[:, x].any():
            c = rows[box[:, x]]
            filled[c.min():c.max() + 1, x] |= True
    out = np.zeros_like(arch)
    out[y0:y1, x0:x1] = filled
    return out


def clear_regions(img, card_fill, clear_card):
    """Remove the per-speaker text and the sample QR.

    The card keeps its shape and shadow; only its inside is repainted, inset far
    enough to leave the rounded corners and the drop shadow untouched. A source
    the designer already supplied empty needs only the QR clearing.
    """
    d = ImageDraw.Draw(img)
    if clear_card:
        x0, y0, x1, y1 = CARD
        d.rectangle([x0 + 9, y0 + 9, x1 - 10, y1 - 10], fill=card_fill + (255,))
    # The QR sits inside a white card below a "SCAN & REGISTER" tab. Clear only
    # the code; the card and the tab are furniture the studio draws its QR onto.
    qx0, qy0, qx1, qy1 = QR_CARD
    d.rectangle([qx0 + 11, qy0 + 52, qx1 - 12, qy1 - 12], fill=(255, 255, 255, 255))
    return img


def build(sources):
    """sources: {look: (pdf path, 1-based page, already-blank?)}"""
    os.makedirs(os.path.join(OUT, "specs"), exist_ok=True)
    for look, (pdf, page, clear_card) in sources.items():
        _, bg, card_fill = LOOKS[look]
        work = tempfile.mkdtemp(prefix="official-")
        pages = render_without_portrait(pdf, work)
        if page > len(pages):
            raise SystemExit(f"{pdf} has {len(pages)} page(s), asked for {page}")
        src = Image.open(pages[page - 1]).convert("RGB")
        if src.size != (1080, 1080):
            raise SystemExit(f"page {page} rendered {src.size}, want 1080x1080")
        rgb = np.array(src)
        arch = arch_mask(rgb)

        bd = np.full_like(rgb, bg)
        bd[arch] = LIME

        # The arch's edge is anti-aliased: those pixels are blends of the page
        # background and the lime. Left in the overlay they are drawn *over* the
        # portrait and read as a pale halo tracing the arch. Any pixel that lies
        # on the background-to-lime line belongs to the backdrop, so bake the
        # blend in there and leave the overlay clear.
        a = np.array(bg, float)
        b = np.array(LIME, float) - a
        px = rgb.astype(float) - a
        t = np.clip((px @ b) / float(b @ b), 0.0, 1.0)
        residual = np.abs(px - t[..., None] * b).sum(axis=2)
        # Only on the arch's own rim. The test matches any background-to-lime
        # blend, and pure lime scores a perfect match, so a wider neighbourhood
        # also punches out the bottom bar's lime panel - which overlaps the
        # arch's rows - and lets the portrait show through it.
        rim = (ndimage.binary_dilation(arch, np.ones((5, 5)))
               & ~ndimage.binary_erosion(arch, np.ones((5, 5))))
        blend = (residual < 12) & rim
        bd[blend] = rgb[blend]
        backdrop = Image.fromarray(bd)

        # The overlay is every pixel the backdrop does not explain. Derived by
        # difference rather than colour-keyed, because the two looks reuse each
        # other's colours - the dark page's bottom bar is the same cream as the
        # light page's background, so a global key would erase it.
        diff = (np.abs(rgb.astype(int) - bd.astype(int)).sum(axis=2) > 12) & ~blend

        # An element has to be kept as a whole region, not pixel by pixel. Text
        # inside the bottom bar is drawn in the page's own background colour on
        # the dark look, so a plain difference test scores those glyphs as
        # backdrop and punches them out - and the portrait then shows through
        # the lettering. Closing and hole-filling makes each card, bar and logo
        # solid, so anything sitting inside one travels with it.
        diff = ndimage.binary_fill_holes(
            ndimage.binary_closing(diff, np.ones((5, 5))))
        overlay = Image.fromarray(
            np.dstack([rgb, np.where(diff, 255, 0).astype(np.uint8)]), "RGBA")
        overlay = clear_regions(overlay, card_fill, clear_card)

        family = f"speaker-official-{look}"
        backdrop.save(os.path.join(OUT, f"{family}-1x1-backdrop.png"), optimize=True)
        overlay.save(os.path.join(OUT, f"{family}-1x1-overlay.png"), optimize=True)
        write_spec(family, look, bg, card_fill)
        print(f"{family}: arch {100 * arch.mean():.1f}%, "
              f"overlay {100 * diff.mean():.1f}% painted")


def write_spec(family, look, bg, card_fill):
    hexof = lambda c: "#%02x%02x%02x" % c
    # The card is filled with the *other* look's background, so the page
    # background is exactly the contrasting ink for text on it. Using
    # card_fill here prints dark text on a dark card and it vanishes.
    ink = hexof(bg)
    cx0, cy0, cx1, cy1 = CARD
    qx0, qy0, qx1, qy1 = QR

    text = lambda key, label, box, **kw: dict(
        id=key, type="text", key=key, label=label,
        x=box[0], y=box[1], w=box[2] - box[0], h=box[3] - box[1],
        font=kw.get("font", "body"), weight=kw.get("weight", 400),
        size=kw["size"], color=ink, align="center",
        transform="upper" if kw.get("upper") else "none",
        tracking=kw.get("tracking", 0.0), line_height=kw.get("lh", 1.2),
        autofit=True)

    spec = dict(
        background=hexof(bg),
        defaults={"qr_url": "https://reg.bioconnect.kerala.gov.in/delegates",
                  "topic": "TOPIC:"},
        layers=[
            dict(id="backdrop", type="art", asset_id="__BACKDROP__",
                 label="Background & arch", x=0, y=0, w=1080, h=1080),
            dict(id="portrait", type="photo", key="photo",
                 label="Speaker cut-out (PNG with transparency)",
                 x=PHOTO[0], y=PHOTO[1], w=PHOTO[2] - PHOTO[0], h=PHOTO[3] - PHOTO[1],
                 fit="cover", radius=0, duotone=False),
            dict(id="decor", type="art", asset_id="__OVERLAY__",
                 label="Logos, badge, cards & bar", x=0, y=0, w=1080, h=1080),
            text("name", "Speaker name", (cx0 + 12, cy0 + 22, cx1 - 12, cy0 + 96),
                 font="display", weight=700, size=27, upper=True, tracking=0.02, lh=1.18),
            text("designation", "Designation",
                 (cx0 + 12, cy0 + 104, cx1 - 12, cy0 + 136),
                 weight=500, size=15, upper=True, tracking=0.06),
            text("organisation", "Organisation",
                 (cx0 + 12, cy0 + 140, cx1 - 12, cy0 + 184),
                 size=11, upper=True, tracking=0.04, lh=1.3),
            text("topic", "Topic line",
                 (cx0 + 12, cy0 + 190, cx1 - 12, cy1 - 14),
                 size=11, upper=True, tracking=0.04, lh=1.3),
            dict(id="qr", type="qr", key="qr_url", label="Registration QR",
                 x=qx0, y=qy0, w=qx1 - qx0, h=qy1 - qy0),
        ])
    with open(os.path.join(OUT, "specs", f"{family}-1x1.json"), "w") as f:
        json.dump(dict(family=family, name=f"Speaker reveal official - {look}",
                       size="1x1", spec=spec), f, indent=1)


if __name__ == "__main__":
    # look=<pdf>#<page>[#blank]   e.g.  dark="Speaker poster-1.pdf#1#blank"
    if len(sys.argv) < 2:
        raise SystemExit('usage: official.py look=<pdf>#<page>[#blank] ...')
    srcs = {}
    for arg in sys.argv[1:]:
        look, _, rest = arg.partition("=")
        parts = rest.split("#")
        if look not in LOOKS:
            raise SystemExit(f"unknown look {look!r}; expected one of {list(LOOKS)}")
        srcs[look] = (os.path.expanduser(parts[0]),
                      int(parts[1]) if len(parts) > 1 and parts[1] else 1,
                      "blank" not in parts)
    build(srcs)
