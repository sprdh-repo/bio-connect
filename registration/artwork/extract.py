#!/usr/bin/env python3
"""Cut the nine motifs out of the generated sheet into individual PNGs.

The generator lays them on a 3x3 grid, so the cells are known; the work here is
cleaning the stray low-alpha pixels it leaves behind, snapping the fills to the
exact brand palette, and confirming no two motifs merged into one blob.
"""
from PIL import Image
import numpy as np

BRAND = np.array([
    (11, 51, 41),    # forest
    (5, 28, 23),     # deep forest
    (185, 220, 114), # lime
    (228, 173, 84),  # gold
    (243, 241, 233), # cream
    (251, 250, 245), # paper
    (16, 32, 27),    # ink
], dtype=np.int16)

NAMES = [
    "frond-arching", "frond-full", "sprig-berries",
    "dna-helix", "molecule-cluster", "molecule-lattice",
    "petri-dish", "glassware", "microscopy",
]

sheet = Image.open("raw-sheet.png").convert("RGBA")
a = np.array(sheet).astype(np.int16)

# The generator leaves a haze of near-zero alpha across the background, which
# would defeat a bbox crop and show as grime over a photograph.
a[..., 3][a[..., 3] < 20] = 0

# Snap solid fills to the exact palette. Only fully-opaque pixels: partially
# transparent ones are anti-aliased edges, and quantising those would leave
# hard jaggies where the motif meets the photograph.
solid = a[..., 3] >= 200
# int32 throughout: a channel difference of 242 squares to 58564, which wraps
# negative in int16 and makes argmin pick the *furthest* colour. That silently
# mapped every dark green to cream.
rgb = a[solid][:, :3].astype(np.int32)
dist = ((rgb[:, None, :] - BRAND.astype(np.int32)[None, :, :]) ** 2).sum(axis=2)
assert dist.min() >= 0, "distance overflowed"
nearest = np.argmin(dist, axis=1)
a[solid] = np.concatenate([BRAND[nearest], a[solid][:, 3:]], axis=1)

cleaned = Image.fromarray(a.astype(np.uint8), "RGBA")
cleaned.save("motif-sheet.png")

H, W = a.shape[0], a.shape[1]
ch, cw = H // 3, W // 3
print(f"sheet {W}x{H}, cells {cw}x{ch}\n")

def components(mask):
    """Count connected blobs with a flood fill, to catch merged motifs."""
    seen = np.zeros_like(mask, dtype=bool)
    n = 0
    hs, ws = mask.shape
    for sy in range(hs):
        for sx in range(ws):
            if not mask[sy, sx] or seen[sy, sx]:
                continue
            n += 1
            stack = [(sy, sx)]
            seen[sy, sx] = True
            while stack:
                y, x = stack.pop()
                for dy, dx in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                    ny, nx = y + dy, x + dx
                    if 0 <= ny < hs and 0 <= nx < ws and mask[ny, nx] and not seen[ny, nx]:
                        seen[ny, nx] = True
                        stack.append((ny, nx))
    return n

for i, name in enumerate(NAMES):
    row, col = divmod(i, 3)
    cell = cleaned.crop((col * cw, row * ch, (col + 1) * cw, (row + 1) * ch))
    alpha = np.array(cell.getchannel("A"))

    # Does this motif touch the cell edge? If so it may be clipped or merged
    # with its neighbour, which is the one failure the brief warned about.
    edge = max(alpha[0].max(), alpha[-1].max(), alpha[:, 0].max(), alpha[:, -1].max())

    ys, xs = np.nonzero(alpha)
    if len(ys) == 0:
        print(f"{name:18} EMPTY CELL")
        continue
    box = (xs.min(), ys.min(), xs.max() + 1, ys.max() + 1)
    motif = cell.crop(box)
    motif.save(f"motif-{i + 1:02d}-{name}.png")

    # Downsample before the flood fill; full resolution is needlessly slow and
    # a merge would survive the downsample anyway.
    small = np.array(motif.getchannel("A").resize((motif.width // 4, motif.height // 4))) > 40
    blobs = components(small)
    fill = 100 * (alpha > 0).mean()
    print(f"{name:18} {motif.width:3}x{motif.height:3}  fill {fill:4.1f}%  "
          f"blobs {blobs:2}  edge-alpha {edge:3}"
          f"{'  <-- TOUCHES CELL EDGE' if edge > 40 else ''}")
