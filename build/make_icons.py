#!/usr/bin/env python3
import os
import sys

from PIL import Image, ImageDraw

OUT = sys.argv[1] if len(sys.argv) > 1 else "payload"


def icon(size, gray=False):
    img = Image.new("RGBA", (size, size), (15, 16, 38, 255))
    d = ImageDraw.Draw(img)
    margin = size // 8
    col = (150, 150, 150, 255) if gray else (109, 158, 255, 255)
    d.ellipse([margin, margin, size - margin, size - margin], outline=col, width=max(2, size // 12))
    cx, cy = size // 2, size // 2
    r = size // 5
    d.polygon([(cx - r, cy - r), (cx + r, cy), (cx - r, cy + r)], fill=col)
    return img


def main():
    os.makedirs(OUT, exist_ok=True)
    for name, sz in ((".qpkg_icon.gif", 64), (".qpkg_icon_80.gif", 80), (".qpkg_icon_gray.gif", 64)):
        icon(sz, gray="gray" in name).convert("P").save(os.path.join(OUT, name))
        print("wrote", os.path.join(OUT, name))


if __name__ == "__main__":
    main()
