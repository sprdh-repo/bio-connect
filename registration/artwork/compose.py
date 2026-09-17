#!/usr/bin/env python3
"""Compose the Bio Connect 4.0 speaker-reveal template artwork.

The illustrated motifs (palm fronds, molecular node-and-connector clusters) come
from the Codex-generated v1 overlay, which is what image generation is genuinely
good at. The geometry - panels, bands, the photo window, the event bar - is laid
out here instead, because exact pixel placement and true alpha are what image
generation is bad at.

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

SRC = Image.open("motifs-src.png").convert("RGBA")


def motif(box, scale=1.0, angle=0):
    """Lift one generated motif out of the source art by its alpha."""
    part = SRC.crop(box)
    if scale != 1.0:
        part = part.resize((int(part.width * scale), int(part.height * scale)), Image.LANCZOS)
    if angle:
        part = part.rotate(angle, expand=True, resample=Image.BICUBIC)
    return part


FROND_L = motif((20, 160, 240, 420))
FROND_R = motif((835, 160, 1075, 475))
MOLECULE = motif((30, 805, 210, 965))


def grain(img, zone, density=260):
    """Sparse paper grain. Deterministic so reruns are byte-identical."""
    rng = random.Random(20260918)
    px = img.load()
    x0, y0, x1, y1 = zone
    for _ in range((x1 - x0) * (y1 - y0) // density):
        x, y = rng.randrange(x0, x1), rng.randrange(y0, y1)
        r, g, b = px[x, y][:3]
        px[x, y] = (min(255, r + 4), min(255, g + 4), min(255, b + 3))


def rounded(d, box, radius, fill):
    d.rounded_rectangle(box, radius=radius, fill=fill)


# ---------------------------------------------------------------- backdrop
bg = Image.new("RGB", (W, H), CREAM)
d = ImageDraw.Draw(bg)

# Stacked panels behind the portrait, peeking past every edge so the photo
# reads as mounted on cards rather than pasted onto the page.
forest_panel = Image.new("RGBA", (PX1 - PX0 + 70, PY1 - PY0 + 70), FOREST + (255,))
bg.paste(forest_panel.rotate(-1.4, expand=True, resample=Image.BICUBIC),
         (PX0 - 48, PY0 - 44), forest_panel.rotate(-1.4, expand=True, resample=Image.BICUBIC))
lime_panel = Image.new("RGBA", (PX1 - PX0 + 40, PY1 - PY0 + 46), LIME + (255,))
bg.paste(lime_panel.rotate(1.1, expand=True, resample=Image.BICUBIC),
         (PX0 - 12, PY0 - 6), lime_panel.rotate(1.1, expand=True, resample=Image.BICUBIC))

# The window itself: flat, so nothing shows through a transparent-edged photo.
d.rectangle([PX0, PY0, PX1 - 1, PY1 - 1], fill=PAPER)

# Event bar, split into three compartments like the reference's bottom band,
# with a plinth on the right for the QR. The dividers sit where they do because
# the venue is the longest of the three fields and needs the widest column;
# an even three-way split runs it under the QR.
d.rectangle([0, 1180, W, 1308], fill=DEEP)
for x in (450, 660):
    d.rectangle([x, 1210, x + 2, 1278], fill=LIME)
rounded(d, [898, 1188, 1052, 1300], 12, CREAM)

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

# Motifs where they are actually visible: flanking the logo strip, and in the
# caption zone's left margin. Nothing hidden behind the window.
for art, xy in ((FROND_L.resize((150, 178), Image.LANCZOS), (-18, 8)),
                (FROND_R.resize((160, 210), Image.LANCZOS), (938, -6)),
                (MOLECULE.resize((104, 92), Image.LANCZOS), (958, 1040))):
    bg.paste(art, xy, art)

grain(bg, (0, 0, W, 195))
grain(bg, (0, 1010, W, 1180))
bg.save("backdrop-4x5.png")

# ----------------------------------------------------------------- overlay
ov = Image.new("RGBA", (W, H), (0, 0, 0, 0))

# Fronds entering over the portrait's top corners, from the generated art.
tl = FROND_L.resize((250, 296), Image.LANCZOS)
tr = FROND_R.resize((265, 348), Image.LANCZOS)
ov.paste(tl, (60, 150), tl)
ov.paste(tr, (790, 140), tr)
mol = MOLECULE.resize((150, 133), Image.LANCZOS)
ov.paste(mol, (108, 770), mol)

# The caption band. This is the element the brief called out: like the
# reference's yellow band it straddles the photo's bottom edge, so it can only
# live on the layer above the portrait.
od = ImageDraw.Draw(ov)
od.rectangle([PX0, 920, PX1 - 1, 1159], fill=GOLD + (255,))
od.rectangle([PX0, 920, PX0 + 8, 1159], fill=FOREST + (255,))

ov.save("overlay-4x5.png")

for name in ("backdrop-4x5.png", "overlay-4x5.png"):
    im = Image.open(name)
    assert im.size == (W, H), (name, im.size)
    print(f"{name}  {im.size}  {im.mode}")

a = Image.open("overlay-4x5.png").getchannel("A")
clear = sum(1 for v in a.getdata() if v == 0)
print(f"overlay transparent: {100 * clear / (W * H):.1f}%")
face = Image.open("overlay-4x5.png").crop((300, 240, 780, 880)).getchannel("A")
print("face area fully clear:", max(face.getdata()) == 0)
