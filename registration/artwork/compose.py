#!/usr/bin/env python3
"""Compose the Bio Connect 4.0 speaker-reveal template artwork.

The motifs come from motif-sheet.png, a 3x3 grid of generated illustrations -
fronds, a botanical sprig, a DNA helix, molecular clusters, a petri dish,
glassware, a microscopy field. Image generation is genuinely good at those.

The geometry - panels, bands, the photo window, the event bar - is laid out here
instead, because exact pixel placement, a flat photo window and a controlled
alpha channel are the three things image generation is bad at.

Outputs, both 1080x1350:
  backdrop-4x5.png  opaque, drawn below the portrait
  overlay-4x5.png   RGBA, drawn above the portrait
"""
from PIL import Image, ImageDraw
import random

W, H = 1080, 1350
FOREST, DEEP = (11, 51, 41), (5, 28, 23)
LIME, GOLD = (185, 220, 114), (228, 173, 84)
CREAM, PAPER = (243, 241, 233), (251, 250, 245)

# The portrait window. Nearly square so a 4:5 speaker portrait in "cover" fit
# lands the face around the upper third without the operator having to reframe.
PX0, PY0, PX1, PY1 = 115, 195, 965, 1005

SHEET = Image.open("motif-sheet.png").convert("RGBA")
CELL = SHEET.width // 3
# Reading order of the sheet's 3x3 grid.
MOTIFS = ["frond-arching", "frond-full", "sprig-berries",
          "dna-helix", "molecule-cluster", "molecule-lattice",
          "petri-dish", "glassware", "microscopy"]


def motif(name, size, flip=False, angle=0):
    """Cut one motif from its grid cell, trimmed to its own alpha bounds."""
    i = MOTIFS.index(name)
    row, col = divmod(i, 3)
    cell = SHEET.crop((col * CELL, row * CELL, (col + 1) * CELL, (row + 1) * CELL))
    art = cell.crop(cell.getchannel("A").getbbox())
    if flip:
        art = art.transpose(Image.FLIP_LEFT_RIGHT)
    if angle:
        art = art.rotate(angle, expand=True, resample=Image.BICUBIC)
    return art.resize(size, Image.LANCZOS)


def grain(img, zone, density=260):
    """Sparse paper grain. Seeded, so reruns are byte-identical."""
    rng = random.Random(20260918)
    px = img.load()
    x0, y0, x1, y1 = zone
    for _ in range((x1 - x0) * (y1 - y0) // density):
        x, y = rng.randrange(x0, x1), rng.randrange(y0, y1)
        r, g, b = px[x, y][:3]
        px[x, y] = (min(255, r + 4), min(255, g + 4), min(255, b + 3))


def stamp(img, art, xy):
    img.paste(art, xy, art)


# ---------------------------------------------------------------- backdrop
bg = Image.new("RGB", (W, H), CREAM)
d = ImageDraw.Draw(bg)

# Stacked panels behind the portrait, peeking past every edge so the photo
# reads as mounted on cards rather than pasted onto the page.
for colour, pad, tilt, at in ((FOREST, 70, -1.4, (PX0 - 48, PY0 - 44)),
                              (LIME, 43, 1.1, (PX0 - 12, PY0 - 6))):
    panel = Image.new("RGBA", (PX1 - PX0 + pad, PY1 - PY0 + pad), colour + (255,))
    panel = panel.rotate(tilt, expand=True, resample=Image.BICUBIC)
    bg.paste(panel, at, panel)

# The window itself: flat, so nothing shows through a transparent-edged photo.
d.rectangle([PX0, PY0, PX1 - 1, PY1 - 1], fill=PAPER)

# Event bar, split into three compartments like the reference's bottom band,
# with a plinth on the right for the QR. The dividers sit where they do because
# the venue is the longest of the three fields and needs the widest column;
# an even three-way split runs it under the QR.
d.rectangle([0, 1180, W, 1308], fill=DEEP)
for x in (450, 660):
    d.rectangle([x, 1210, x + 2, 1278], fill=LIME)
d.rounded_rectangle([898, 1188, 1052, 1300], radius=12, fill=CREAM)

# Footer strip for the website address.
d.rectangle([0, 1308, W, H], fill=CREAM)
d.rectangle([0, 1306, W, 1308], fill=FOREST)

# Thin gold rule opening the caption zone.
d.rectangle([115, 1010, 400, 1013], fill=GOLD)

# Dotted grids and hairlines in the side margins.
for gx, gy in ((22, 430), (1020, 500)):
    for row in range(5):
        for col in range(4):
            cx, cy = gx + col * 13, gy + row * 13
            d.ellipse([cx, cy, cx + 3, cy + 3], fill=GOLD)
d.rectangle([38, 200, 39, 410], fill=GOLD)
d.rectangle([1044, 270, 1045, 480], fill=GOLD)

# Motifs where they are actually visible - flanking the logo strip and in the
# margins. Two different fronds rather than one mirrored, so the corners do not
# read as a symmetrical stencil. Nothing goes behind the photo window.
stamp(bg, motif("frond-arching", (172, 170)), (-22, 4))
stamp(bg, motif("frond-full", (185, 182), flip=True), (922, -10))
stamp(bg, motif("dna-helix", (66, 117)), (16, 555))
stamp(bg, motif("petri-dish", (108, 106)), (952, 1030))

grain(bg, (0, 0, W, 195))
grain(bg, (0, 1010, W, 1180))
bg.save("backdrop-4x5.png")

# ----------------------------------------------------------------- overlay
ov = Image.new("RGBA", (W, H), (0, 0, 0, 0))

# Fronds and molecules crossing in front of the portrait's corners. This layer
# is why a template is a stack and not a flat background.
stamp(ov, motif("frond-arching", (244, 242)), (52, 150))
stamp(ov, motif("frond-full", (272, 268), flip=True), (788, 142))
stamp(ov, motif("molecule-cluster", (152, 145)), (105, 758))
stamp(ov, motif("molecule-lattice", (112, 127)), (838, 405))

# The caption band. Like the reference poster's yellow band it straddles the
# photo's bottom edge, so it can only live on the layer above the portrait.
od = ImageDraw.Draw(ov)
od.rectangle([PX0, 920, PX1 - 1, 1159], fill=GOLD + (255,))
od.rectangle([PX0, 920, PX0 + 8, 1159], fill=FOREST + (255,))

ov.save("overlay-4x5.png")

# ------------------------------------------------------------------ checks
for name in ("backdrop-4x5.png", "overlay-4x5.png"):
    im = Image.open(name)
    assert im.size == (W, H), (name, im.size)
    print(f"{name}  {im.size}  {im.mode}")

alpha = Image.open("overlay-4x5.png").getchannel("A")
clear = sum(1 for v in alpha.getdata() if v == 0)
print(f"overlay transparent: {100 * clear / (W * H):.1f}%")

# The speaker's face must never be under a decoration.
face = Image.open("overlay-4x5.png").crop((300, 240, 780, 880)).getchannel("A")
assert max(face.getdata()) == 0, "overlay covers the face area"
print("face area fully clear: True")

# The window must stay one flat colour, or a photo with soft edges shows grime.
win = Image.open("backdrop-4x5.png").crop((PX0, PY0, PX1, PY1))
assert len(win.getcolors(maxcolors=8) or [(0, 0)]) == 1, "photo window is not flat"
print("photo window flat: True")
