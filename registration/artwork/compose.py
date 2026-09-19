#!/usr/bin/env python3
"""Build every poster template's artwork and spec.

Four post types, each in a light and a dark look, each at three sizes: 24
templates, 48 PNGs. Hand-placing that many layouts is not maintainable, so the
sizes, looks, families and motif placements are declared below and rendered.

The motifs come from motif-sheet.png and motif-sheet-2.png, 3x3 grids of
generated illustrations. Image generation is good at those. The geometry -
panels, bands, the photo window, the event bar - is drawn here, because exact
pixel placement, a flat photo window and a controlled alpha channel are the
three things image generation is reliably bad at.

Outputs go to ../internal/app/artwork/, where the binary embeds them:
  <family>-<size>-backdrop.png   opaque, drawn below the photo
  <family>-<size>-overlay.png    RGBA, drawn above the photo
  specs/<family>-<size>.json     the template spec, asset ids as placeholders

Run from this directory:  python3 compose.py
"""
from PIL import Image, ImageDraw
import json
import os
import random
import shutil

OUT = os.path.join("..", "internal", "app", "artwork")

FOREST, DEEP = "#0b3329", "#051c17"
LIME, GOLD = "#b9dc72", "#e4ad54"
CREAM, PAPER, INK = "#f3f1e9", "#fbfaf5", "#10201b"


def rgb(h):
    return tuple(int(h[i:i + 2], 16) for i in (1, 3, 5))


# --------------------------------------------------------------------- sizes
# Absolute bands per size rather than one set of fractions: a square crop and a
# story are different enough that scaling one from the other reads as stretched.
# 9x16 keeps its content between y=250 and y=1700, because a story's top and
# bottom strips are covered by the platform's own UI.
SIZES = {
    "4x5": dict(w=1080, h=1350, logo=165, stage=(115, 195, 965, 1005),
                band=(920, 1159), bar=(1180, 1308), foot=(1308, 1350)),
    "1x1": dict(w=1080, h=1080, logo=132, stage=(115, 160, 965, 742),
                band=(672, 880), bar=(898, 1024), foot=(1024, 1080)),
    # 9x16's footer runs to the canvas bottom. Stopping it short left a quarter
    # of the story as dead page colour, which reads as a mistake rather than as
    # the safe area it was meant to be.
    "9x16": dict(w=1080, h=1920, logo=250, stage=(115, 300, 965, 1300),
                 band=(1220, 1560), bar=(1600, 1740), foot=(1740, 1920)),
}

# --------------------------------------------------------------------- looks
LOOKS = {
    "light": dict(page=CREAM, panel_a=FOREST, panel_b=LIME, window=PAPER,
                  band=GOLD, band_ink=FOREST, band_edge=FOREST,
                  bar=DEEP, bar_ink=PAPER, bar_accent=LIME,
                  foot=CREAM, foot_ink=FOREST, rule=GOLD, dots=GOLD),
    "dark": dict(page=FOREST, panel_a=DEEP, panel_b=LIME, window=DEEP,
                 band=LIME, band_ink=DEEP, band_edge=GOLD,
                 bar=CREAM, bar_ink=FOREST, bar_accent=GOLD,
                 foot=DEEP, foot_ink=LIME, rule=GOLD, dots=LIME),
}

# -------------------------------------------------------------------- motifs
SHEETS = ["motif-sheet.png", "motif-sheet-2.png"]
SHEET_NAMES = [
    ["frond-arching", "frond-full", "sprig-berries",
     "dna-helix", "molecule-cluster", "molecule-lattice",
     "petri-dish", "glassware", "microscopy"],
    ["palm-tree", "waves", "rising-sun",
     "hourglass", "clock", "microscope",
     "rosette", "laurel", "network-flow"],
]
_cache = {}


def motif(name, width, flip=False, angle=0):
    """Cut a motif from its grid cell, trimmed to its own alpha bounds."""
    if name not in _cache:
        for path, names in zip(SHEETS, SHEET_NAMES):
            if name in names and os.path.exists(path):
                sheet = Image.open(path).convert("RGBA")
                cell = sheet.width // 3
                row, col = divmod(names.index(name), 3)
                part = sheet.crop((col * cell, row * cell, (col + 1) * cell, (row + 1) * cell))
                _cache[name] = part.crop(part.getchannel("A").getbbox())
                break
        else:
            raise KeyError(f"motif {name!r} not on any available sheet")
    art = _cache[name]
    if flip:
        art = art.transpose(Image.FLIP_LEFT_RIGHT)
    if angle:
        art = art.rotate(angle, expand=True, resample=Image.BICUBIC)
    h = max(1, round(art.height * width / art.width))
    return art.resize((max(1, round(width)), h), Image.LANCZOS)


def stamp(img, name, width, xy, flip=False, angle=0):
    art = motif(name, width, flip, angle)
    img.paste(art, (round(xy[0]), round(xy[1])), art)


def grain(img, zone, colour, density=300):
    """Sparse paper grain. Seeded, so reruns are byte-identical."""
    rng = random.Random(20260918)
    px = img.load()
    x0, y0, x1, y1 = (round(v) for v in zone)
    lift = 5 if sum(rgb(colour)) > 330 else 11
    for _ in range(max(0, (x1 - x0) * (y1 - y0)) // density):
        x, y = rng.randrange(x0, x1), rng.randrange(y0, y1)
        r, g, b = px[x, y][:3]
        px[x, y] = tuple(min(255, c + lift) for c in (r, g, b))


# ------------------------------------------------------------------ furniture
# All three sizes are 1080 wide, so type sizes are constant and only the
# vertical bands move. That is why the size table carries y values only.
def text(key, label, box, *, font="body", weight=400, size=24, colour=FOREST,
         align="left", upper=False, tracking=0.0, line_height=1.25):
    x0, y0, x1, y1 = box
    return dict(id=key, type="text", key=key, label=label,
                x=round(x0), y=round(y0), w=round(x1 - x0), h=round(y1 - y0),
                font=font, weight=weight, size=size, color=colour, align=align,
                transform="upper" if upper else "none",
                tracking=tracking, line_height=line_height, autofit=True)


def photo(key, label, box, *, fit="cover", radius=0, duotone=False):
    x0, y0, x1, y1 = box
    return dict(id=key, type="photo", key=key, label=label,
                x=round(x0), y=round(y0), w=round(x1 - x0), h=round(y1 - y0),
                fit=fit, radius=round(radius), duotone=duotone)


def bar_boxes(S):
    """The event bar's three text columns and its QR plinth."""
    w, (by0, by1) = S["w"], S["bar"]
    side = min(154, by1 - by0 - 16)
    px1 = w - 28
    px0 = px1 - side
    d1, d2 = round(w * 0.417), round(w * 0.611)
    pad = (by1 - by0 - side) / 2
    return dict(
        dividers=(d1, d2), plinth=(px0, by0 + pad, px1, by1 - pad),
        tagline=(60, by0 + 38, d1 - 22, by1 - 26),
        dates=(d1 + 26, by0 + 30, d2 - 22, by0 + 78),
        month=(d1 + 26, by0 + 80, d2 - 18, by0 + 110),
        venue=(d2 + 30, by0 + 36, px0 - 22, by1 - 24),
    )


def furniture(bg, ov, S, L):
    """Everything every family shares: page, bar, footer, band, margin marks."""
    w, h = S["w"], S["h"]
    d, od = ImageDraw.Draw(bg), ImageDraw.Draw(ov)
    B = bar_boxes(S)
    by0, by1 = S["bar"]
    fy0, fy1 = S["foot"]

    d.rectangle([0, by0, w, by1], fill=rgb(L["bar"]))
    for x in B["dividers"]:
        d.rectangle([x, by0 + 30, x + 2, by1 - 30], fill=rgb(L["bar_accent"]))
    d.rounded_rectangle([round(v) for v in B["plinth"]], radius=12, fill=rgb(CREAM))

    d.rectangle([0, fy0, w, fy1], fill=rgb(L["foot"]))
    d.rectangle([0, fy0 - 2, w, fy0], fill=rgb(L["rule"]))

    # Dotted grids and hairlines in the side margins.
    sy0, sy1 = S["stage"][1], S["stage"][3]
    for gx, gy in ((22, sy0 + 0.28 * (sy1 - sy0)), (1020, sy0 + 0.38 * (sy1 - sy0))):
        for row in range(5):
            for col in range(4):
                cx, cy = gx + col * 13, gy + row * 13
                d.ellipse([cx, cy, cx + 3, cy + 3], fill=rgb(L["dots"]))
    d.rectangle([38, sy0 + 10, 39, sy0 + 0.24 * (sy1 - sy0)], fill=rgb(L["rule"]))
    d.rectangle([1044, sy0 + 80, 1045, sy0 + 0.32 * (sy1 - sy0)], fill=rgb(L["rule"]))

    # The caption band lives on the overlay, so it can straddle the stage's
    # bottom edge the way the reference poster's yellow band does.
    bx0, bx1 = S["stage"][0], S["stage"][2]
    cy0, cy1 = S["band"]
    od.rectangle([bx0, cy0, bx1 - 1, cy1], fill=rgb(L["band"]) + (255,))
    od.rectangle([bx0, cy0, bx0 + 8, cy1], fill=rgb(L["band_edge"]) + (255,))


def furniture_layers(S, L):
    """The spec layers matching furniture(): bar, QR and footer."""
    B = bar_boxes(S)
    px0, py0, px1, py1 = B["plinth"]
    qs = min(px1 - px0, py1 - py0) - 24
    fy0, fy1 = S["foot"]
    return [
        text("tagline", "Event tagline", B["tagline"], weight=500, size=19,
             colour=L["bar_ink"], upper=True, tracking=0.115, line_height=1.45),
        text("dates", "Dates", B["dates"], font="display", weight=700, size=38,
             colour=L["bar_ink"], tracking=0.02, line_height=1.1),
        text("month", "Month & year", B["month"], size=17,
             colour=L["bar_accent"], upper=True, tracking=0.1),
        text("venue", "Venue", B["venue"], size=19, colour=L["bar_ink"],
             line_height=1.35),
        dict(id="qr", type="qr", key="qr_url", label="Registration QR",
             x=round(px0 + (px1 - px0 - qs) / 2), y=round(py0 + (py1 - py0 - qs) / 2),
             w=round(qs), h=round(qs)),
        text("website", "Website", (150, fy0 + 12, 930, fy1 - 6), size=15,
             colour=L["foot_ink"], upper=True, tracking=0.24),
    ]


# -------------------------------------------------------------------- families
# Each family draws its own "stage" - the zone between the logo strip and the
# caption band - and declares the spec layers that fill it. Everything else is
# furniture(). Stage geometry is expressed against S["stage"] and the band's top
# edge, because the band overlaps the stage and whatever sits under it is lost.
def stage_box(S):
    x0, y0, x1, _ = S["stage"]
    return x0, y0, x1, S["band"][0] - 12


def panels(bg, S, L, box, spread=(48, 44, 22, 26)):
    """Offset rotated cards behind a slot, so it reads as mounted.

    Returns the lowest y the cards actually paint. Anything drawn underneath -
    a speaker name, a partner name - has to clear that, not the slot: the cards
    bleed well past the box, and text placed against the box lands on a dark
    panel in the dark colour and disappears.
    """
    x0, y0, x1, y1 = box
    bottom = y1
    for colour, pad, tilt, off in ((L["panel_a"], 70, -1.4, (spread[0], spread[1])),
                                   (L["panel_b"], 43, 1.1, (spread[2], spread[3]))):
        card = Image.new("RGBA", (round(x1 - x0 + pad), round(y1 - y0 + pad)),
                         rgb(colour) + (255,))
        card = card.rotate(tilt, expand=True, resample=Image.BICUBIC)
        top = round(y0 - off[1])
        bg.paste(card, (round(x0 - off[0]), top), card)
        bottom = max(bottom, top + card.height)
    return bottom


def top_corners(bg, S, left, right, lw=1.04, rw=1.12, flip_right=True):
    """Corner motifs sized to the logo strip.

    They must be scaled against S["logo"], not given absolute widths: the square
    crop has a shorter strip than the 4:5, and a motif sized for 4:5 reaches
    down into the photo window, which check() rejects.
    """
    strip = S["logo"]
    stamp(bg, left, strip * lw, (-0.13 * strip, 0.03 * strip))
    art = motif(right, strip * rw)
    bg.paste(art, (round(S["w"] - art.width + 25), round(-0.06 * strip)), art)


def speaker_reveal(bg, ov, S, L):
    box = stage_box(S)
    panels(bg, S, L, box)
    ImageDraw.Draw(bg).rectangle([box[0], box[1], box[2] - 1, box[3] - 1],
                                 fill=rgb(L["window"]))
    stamp(ov, "frond-arching", 244, (52, box[1] - 45))
    stamp(ov, "frond-full", 272, (788, box[1] - 53), flip=True)
    stamp(ov, "molecule-cluster", 152, (105, box[3] - 250))
    stamp(ov, "molecule-lattice", 112, (838, box[1] + 0.26 * (box[3] - box[1])))
    top_corners(bg, S, "frond-arching", "frond-full")
    stamp(bg, "dna-helix", 66, (16, box[1] + 0.44 * (box[3] - box[1])))
    stamp(bg, "petri-dish", 108, (952, S["band"][0] + 110))
    return [photo("photo", "Speaker portrait", box, duotone=True)]


def session_announce(bg, ov, S, L):
    x0, y0, x1, y1 = stage_box(S)
    dia = min(280, (y1 - y0) * 0.42)
    cy = y0 + 34
    lefts = (x0 + 75, x1 - 75 - dia)
    mounted = cy + dia
    for lx in lefts:
        mounted = max(mounted, panels(bg, S, L, (lx, cy, lx + dia, cy + dia),
                                      spread=(16, 14, 8, 9)))
        # Outset by 4px: the photo is clipped to a circle of exactly dia, and a
        # plinth drawn to the same bounds differs by a pixel at the rim, which
        # leaves panel colour under the clip edge.
        ImageDraw.Draw(bg).ellipse([lx - 4, cy - 4, lx + dia + 4, cy + dia + 4],
                                   fill=rgb(L["window"]))
    # Overlay motifs graze the outer rims of the circles. Anything aimed at the
    # gap between them fails at 1x1, where the circles shrink but the gap does
    # not grow with them.
    stamp(ov, "frond-arching", dia * 0.42, (lefts[0] - dia * 0.26, cy - dia * 0.18))
    stamp(ov, "molecule-cluster", dia * 0.40, (lefts[1] + dia * 0.80, cy + dia * 0.58))
    top_corners(bg, S, "palm-tree", "laurel", 0.91, 1.12)
    # The flow diagram sits under the names, sized so it always clears the
    # stage's bottom edge whatever the size.
    names_end = mounted + 72
    avail = y1 - names_end - 24
    flow_w = min(320, avail * 343 / 298)
    if flow_w > 80:
        stamp(bg, "network-flow", flow_w,
              (x0 + ((x1 - x0) - flow_w) / 2, names_end + 12))
    stamp(bg, "clock", 92, (16, y1 - 150))
    stamp(bg, "waves", 130, (938, y1 - 120))
    return [
        photo("photo_a", "First speaker portrait",
              (lefts[0], cy, lefts[0] + dia, cy + dia), radius=dia / 2, duotone=True),
        photo("photo_b", "Second speaker portrait",
              (lefts[1], cy, lefts[1] + dia, cy + dia), radius=dia / 2, duotone=True),
        text("speaker_a", "First speaker name",
             (lefts[0] - 20, mounted + 14, lefts[0] + dia + 20, mounted + 72),
             weight=500, size=23, colour=L["foot_ink"] if L is LOOKS["dark"] else FOREST,
             align="center", line_height=1.25),
        text("speaker_b", "Second speaker name",
             (lefts[1] - 20, mounted + 14, lefts[1] + dia + 20, mounted + 72),
             weight=500, size=23, colour=L["foot_ink"] if L is LOOKS["dark"] else FOREST,
             align="center", line_height=1.25),
    ]


def countdown(bg, ov, S, L):
    x0, y0, x1, y1 = stage_box(S)
    panels(bg, S, L, (x0, y0, x1, y1))
    ImageDraw.Draw(bg).rectangle([x0, y0, x1 - 1, y1 - 1], fill=rgb(L["window"]))
    # No photograph carries this one, so the motifs do more of the work.
    stamp(ov, "hourglass", 150, (x0 + 46, y0 + 30))
    stamp(ov, "rising-sun", 210, (x1 - 250, y1 - 190))
    stamp(ov, "frond-arching", 210, (26, y0 - 40))
    stamp(ov, "frond-full", 230, (836, y0 - 46), flip=True)
    top_corners(bg, S, "palm-tree", "molecule-lattice", 0.97, 0.91)
    stamp(bg, "waves", 210, (x0 + 0.5 * (x1 - x0) - 105, y1 - 96))
    ink = DEEP if L is LOOKS["light"] else LIME
    return [
        text("count", "Number (e.g. 20)",
             (x0 + 40, y0 + 0.30 * (y1 - y0), x1 - 40, y0 + 0.66 * (y1 - y0)),
             font="display", weight=700, size=210, colour=ink, align="center",
             line_height=1.0),
        text("count_label", "Under the number",
             (x0 + 40, y0 + 0.68 * (y1 - y0), x1 - 40, y0 + 0.76 * (y1 - y0)),
             weight=500, size=34, colour=ink, align="center", upper=True,
             tracking=0.3),
    ]


def sponsor_welcome(bg, ov, S, L):
    x0, y0, x1, y1 = stage_box(S)
    cw, chh = (x1 - x0) * 0.72, (y1 - y0) * 0.50
    cx0 = x0 + ((x1 - x0) - cw) / 2
    cy0 = y0 + (y1 - y0) * 0.22
    mounted = panels(bg, S, L, (cx0, cy0, cx0 + cw, cy0 + chh), spread=(26, 22, 12, 14))
    # A partner logo arrives on white far more often than not, so the card it
    # sits on is always the light one regardless of the look.
    ImageDraw.Draw(bg).rounded_rectangle(
        [cx0, cy0, cx0 + cw, cy0 + chh], radius=18, fill=rgb(PAPER))
    stamp(ov, "rosette", 128, (x1 - 150, cy0 - 64))
    # The laurel goes on the backdrop, not the overlay: at 250px it spans the
    # band the partner name occupies, and on the overlay it was drawn straight
    # over the name. Behind it, the name reads as sitting inside the wreath.
    laurel_w = min(300, max(0, (y1 - mounted - 16)) * 358 / 315)
    if laurel_w > 90:
        stamp(bg, "laurel", laurel_w,
              (x0 + ((x1 - x0) - laurel_w) / 2, mounted + 8))
    top_corners(bg, S, "sprig-berries", "sprig-berries", 0.91, 0.91)
    stamp(bg, "microscope", 96, (20, y1 - 140))
    ink = FOREST if L is LOOKS["light"] else LIME
    return [
        text("tier", "Tier (e.g. Platinum partner)",
             (x0 + 40, y0 + 34, x1 - 40, y0 + 34 + 46),
             weight=500, size=26, colour=ink, align="center", upper=True,
             tracking=0.26),
        photo("logo", "Partner logo (PNG with transparency)",
              (cx0 + 46, cy0 + 40, cx0 + cw - 46, cy0 + chh - 40), fit="contain"),
        text("partner", "Partner name",
             (x0 + 40, mounted + 26, x1 - 40, mounted + 122),
             font="display", weight=700, size=48, colour=ink, align="center",
             line_height=1.12),
    ]


EVENT = dict(tagline="Kerala's international\nlife sciences summit",
             dates="08-09", month="October 2026",
             venue="Hyatt Regency\nTrivandrum, Kerala",
             website="www.bioconnect.kerala.gov.in",
             qr_url="https://reg.bioconnect.kerala.gov.in/delegates")

FAMILIES = {
    "speaker-reveal": dict(
        title="Speaker reveal", stage=speaker_reveal,
        caption=lambda S, L: caption_block(S, L, "eyebrow", "name", "designation",
                                           "Eyebrow", "Speaker name",
                                           "Designation & organisation"),
        defaults=dict(eyebrow="MEET THE SPEAKER", **EVENT)),
    "session-announce": dict(
        title="Session announce", stage=session_announce,
        caption=lambda S, L: caption_block(S, L, "track", "session", "session_time",
                                           "Track", "Session title",
                                           "Time & hall"),
        defaults=dict(track="ON THE AGENDA", **EVENT)),
    "countdown": dict(
        title="Countdown", stage=countdown,
        caption=lambda S, L: caption_block(S, L, "eyebrow", "headline", "subline",
                                           "Eyebrow", "Headline", "Supporting line"),
        defaults=dict(eyebrow="SAVE THE DATE", count="20",
                      count_label="days to go", **EVENT)),
    "sponsor-welcome": dict(
        title="Partner welcome", stage=sponsor_welcome,
        caption=lambda S, L: caption_block(S, L, "eyebrow", "headline", "subline",
                                           "Eyebrow", "Headline", "Supporting line"),
        defaults=dict(eyebrow="PROUD TO PARTNER WITH", tier="PLATINUM PARTNER",
                      headline="Welcome aboard", **EVENT)),
}


def caption_block(S, L, k1, k2, k3, l1, l2, l3):
    """Eyebrow / headline / subline on the caption band, shared by every family."""
    x0, x1 = S["stage"][0] + 35, S["stage"][2] - 30
    cy0, cy1 = S["band"]
    span = cy1 - cy0
    return [
        text(k1, l1, (x0, cy0 + 0.16 * span, x1, cy0 + 0.16 * span + 42),
             weight=500, size=25, colour=L["band_ink"], upper=True, tracking=0.135),
        text(k2, l2, (x0, cy0 + 0.34 * span, x1, cy0 + 0.34 * span + 80),
             font="display", weight=700, size=56, colour=L["band_ink"],
             line_height=1.08),
        text(k3, l3, (x0, cy0 + 0.66 * span, x1, cy1 - 14),
             size=27, colour=L["band_ink"], line_height=1.3),
    ]


# ---------------------------------------------------------------------- build
def build(base, look_name, size_name):
    S, L = SIZES[size_name], LOOKS[look_name]
    fam = FAMILIES[base]
    w, h = S["w"], S["h"]

    bg = Image.new("RGB", (w, h), rgb(L["page"]))
    ov = Image.new("RGBA", (w, h), (0, 0, 0, 0))

    furniture(bg, ov, S, L)
    stage_layers = fam["stage"](bg, ov, S, L)
    grain(bg, (0, 0, w, S["logo"]), L["page"])
    grain(bg, (0, S["band"][1], w, S["bar"][0]), L["page"])

    family = f"{base}-{look_name}"
    stem = f"{family}-{size_name}"
    bg.save(os.path.join(OUT, f"{stem}-backdrop.png"), optimize=True)
    ov.save(os.path.join(OUT, f"{stem}-overlay.png"), optimize=True)

    art = lambda lid, label, ph: dict(id=lid, type="art", asset_id=ph, label=label,
                                      x=0, y=0, w=w, h=h)
    spec = dict(
        background=L["page"],
        defaults={k: v for k, v in fam["defaults"].items()},
        layers=[art("backdrop", "Backdrop", "__BACKDROP__")]
               + stage_layers
               + [art("decor", "Motifs & caption band", "__OVERLAY__")]
               + fam["caption"](S, L)
               + furniture_layers(S, L),
    )
    with open(os.path.join(OUT, "specs", f"{stem}.json"), "w") as f:
        json.dump(dict(family=family, name=f"{fam['title']} - {look_name}",
                       size=size_name, spec=spec), f, indent=1)
    return stem, spec, S


def check(stem, spec, S):
    """The failures that quietly ruin a poster, asserted rather than eyeballed."""
    w, h = S["w"], S["h"]
    back = Image.open(os.path.join(OUT, f"{stem}-backdrop.png"))
    over = Image.open(os.path.join(OUT, f"{stem}-overlay.png")).convert("RGBA")
    assert back.size == (w, h) and over.size == (w, h), f"{stem}: wrong size"
    assert back.mode == "RGB", f"{stem}: backdrop must be opaque"

    ids = [l["id"] for l in spec["layers"]]
    assert len(ids) == len(set(ids)), f"{stem}: duplicate layer ids"
    assert ids.index("decor") > ids.index("backdrop"), f"{stem}: decor below backdrop"

    alpha = over.getchannel("A")
    for l in spec["layers"]:
        if l["type"] != "photo":
            continue
        # A photo slot must sit on a flat patch of backdrop, or a portrait with
        # soft edges picks up whatever was drawn underneath. Test under the clip
        # shape, not the bounding box: a circular slot is *meant* to leave the
        # square's corners showing the panel ring behind it.
        win = back.crop((l["x"], l["y"], l["x"] + l["w"], l["y"] + l["h"]))
        clip = Image.new("L", win.size, 0)
        ImageDraw.Draw(clip).rounded_rectangle(
            [0, 0, win.width - 1, win.height - 1], radius=l["radius"], fill=255)
        # Inset by 2px so the mask's own anti-aliased rim is not sampled.
        clip = clip.point(lambda v: 255 if v == 255 else 0)
        under = {p for p, m in zip(win.convert("RGB").getdata(), clip.getdata()) if m}
        assert len(under) == 1, f"{stem}: {l['id']} sits on {len(under)} colours, want 1"
        # And its middle must not be under a decoration.
        mx, my = l["x"] + l["w"] * 0.5, l["y"] + l["h"] * 0.5
        gw, gh = l["w"] * 0.28, l["h"] * 0.28
        guard = alpha.crop((round(mx - gw), round(my - gh), round(mx + gw), round(my + gh)))
        assert max(guard.getdata()) == 0, f"{stem}: overlay covers the middle of {l['id']}"

    # Every layer that carries content must be inside the canvas.
    for l in spec["layers"]:
        if l["type"] == "art":
            continue
        assert 0 <= l["x"] and l["x"] + l["w"] <= w, f"{stem}: {l['id']} off canvas x"
        assert 0 <= l["y"] and l["y"] + l["h"] <= h, f"{stem}: {l['id']} off canvas y"

    # Per-post fields are meant to start empty; the event furniture is not -
    # nobody should have to retype the venue on every poster.
    keys = {l["key"] for l in spec["layers"] if l.get("key")}
    furniture_keys = {"tagline", "dates", "month", "venue", "website", "qr_url"}
    assert furniture_keys <= keys, f"{stem}: missing furniture layers"
    undefaulted = furniture_keys - set(spec["defaults"])
    assert not undefaulted, f"{stem}: furniture with no default: {sorted(undefaulted)}"
    return over


if __name__ == "__main__":
    absent = [s for s in SHEETS if not os.path.exists(s)]
    if absent:
        raise SystemExit(f"missing motif sheet(s): {', '.join(absent)}")

    # Remove only what this script generates. official.py writes its own
    # families into the same directory, and a blanket rmtree would delete them.
    os.makedirs(os.path.join(OUT, "specs"), exist_ok=True)
    mine = {f"{base}-{look}" for base in FAMILIES for look in LOOKS}
    for d, suffix in ((OUT, ".png"), (os.path.join(OUT, "specs"), ".json")):
        for f in os.listdir(d):
            if f.endswith(suffix) and any(f.startswith(m + "-") for m in mine):
                os.remove(os.path.join(d, f))

    total = 0
    for base in FAMILIES:
        for look in LOOKS:
            row = []
            for size in SIZES:
                stem, spec, S = build(base, look, size)
                over = check(stem, spec, S)
                clear = 100 * sum(1 for v in over.getchannel("A").getdata() if v == 0) \
                        / (S["w"] * S["h"])
                kb = os.path.getsize(os.path.join(OUT, f"{stem}-backdrop.png")) // 1024 \
                     + os.path.getsize(os.path.join(OUT, f"{stem}-overlay.png")) // 1024
                row.append(f"{size} {len(spec['layers']):2}L {clear:4.1f}%clear {kb:4}KB")
                total += 1
            print(f"{base + '-' + look:24} " + "  |  ".join(row))
    art_kb = sum(os.path.getsize(os.path.join(OUT, f)) for f in os.listdir(OUT)
                 if f.endswith(".png")) // 1024
    print(f"\n{total} templates, {total * 2} PNGs, {art_kb} KB embedded")
