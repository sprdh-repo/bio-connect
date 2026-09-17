#!/usr/bin/env python3
"""Crop a headshot to the site's 4:5 speaker/leader portrait format and
apply the forest-green duotone used across assets/speakers/ and the two
committee.html leadership portraits.

Usage:
    python3 scripts/portrait.py SOURCE OUTPUT.webp [--face X,Y,W,H]

Without --face, the largest frontal face found by OpenCV's Haar cascade is
used. Pass --face to override detection (pixel coordinates in the source
image) when it picks the wrong face or misses one.
"""
import argparse
import sys

import cv2
import numpy as np
from PIL import Image, ImageOps

OW, OH = 480, 600
FACE_RATIO = 0.36  # face height as a fraction of crop height
CY = 0.42  # face-centre position from the top of the crop, as a fraction of crop height

# Duotone gradient stops: (position 0-1, RGB). Sampled from the leadership
# portraits so every speaker/leader photo reads as one set regardless of
# source lighting.
STOPS = [(0.0, (5, 28, 22)), (0.55, (118, 150, 122)), (1.0, (232, 238, 216))]


def build_lut():
    lut = np.zeros((256, 3))
    for i in range(256):
        t = i / 255
        for (a, ca), (b, cb) in zip(STOPS, STOPS[1:]):
            if a <= t <= b:
                k = (t - a) / (b - a)
                lut[i] = np.array(ca) * (1 - k) + np.array(cb) * k
    return lut.astype(np.uint8)


def duotone(im, lut):
    g = np.asarray(im.convert("L")).astype(float)
    lo, hi = np.percentile(g, 1), np.percentile(g, 99)
    g = np.clip((g - lo) / max(hi - lo, 1) * 255, 0, 255).astype(np.uint8)
    return Image.fromarray(lut[g])


def detect_face(path):
    img = cv2.imread(path)
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    cascade = cv2.CascadeClassifier(
        "/usr/share/opencv5/haarcascades/haarcascade_frontalface_default.xml"
    )
    faces = cascade.detectMultiScale(gray, scaleFactor=1.05, minNeighbors=5, minSize=(100, 100))
    if len(faces) == 0:
        sys.exit("No face detected; pass --face X,Y,W,H")
    # Largest face wins if several are found.
    return max(faces, key=lambda f: f[2] * f[3])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source")
    parser.add_argument("output")
    parser.add_argument("--face", help="X,Y,W,H face box in source pixels, overrides detection")
    parser.add_argument("--face-ratio", type=float, default=FACE_RATIO)
    parser.add_argument("--cy", type=float, default=CY)
    args = parser.parse_args()

    if args.face:
        x, y, w, h = (int(v) for v in args.face.split(","))
    else:
        x, y, w, h = detect_face(args.source)

    im = ImageOps.exif_transpose(Image.open(args.source)).convert("RGB")
    W, H = im.size

    ch = h / args.face_ratio
    cw = ch * OW / OH
    s = min(1, W / cw, H / ch)
    cw *= s
    ch *= s
    cx, cy = x + w / 2, y + h / 2
    left = min(max(cx - cw / 2, 0), W - cw)
    top = min(max(cy - args.cy * ch, 0), H - ch)
    box = tuple(round(v) for v in (left, top, left + cw, top + ch))

    crop = im.crop(box).resize((OW, OH), Image.LANCZOS)
    duotone(crop, build_lut()).save(args.output, "WEBP", quality=82, method=6)
    print(f"face={x},{y},{w},{h} box={box} -> {args.output}")


if __name__ == "__main__":
    main()
