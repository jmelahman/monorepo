"""A/B crop stacks for a step's review: for each shot, the foreground of each stills folder at full
resolution, stacked top to bottom and labelled, with the numbers a look is judged by (clipped
and crushed pixels, mean brightness). Gemini's advice (23 Nov): give the reviewer the strips and
the numbers, not bare stills, and make them in the step so they're ready when it ends.

    step_stack.py OUT LABEL=DIR [LABEL=DIR ...] --shots "a,b,c"

Writes OUT/<shot>.jpg and OUT/stats.json; prints one line of numbers per shot and folder. The
crop is the lower 55% (the verge, the grass, the road's edge) of the middle 80%.
Run with: uv run -q --no-project --with numpy --with pillow python -I scripts/step_stack.py
"""

import argparse
import json
import os

import numpy as np
from PIL import Image, ImageDraw

CROP = (0.10, 0.45, 0.90, 1.0)  # left, top, right, bottom, as fractions


def stats(img):
    a = np.asarray(img.convert("RGB"), dtype=np.float32)
    lum = a @ np.array([0.2126, 0.7152, 0.0722], dtype=np.float32)
    return {
        "clipped": round(float(np.mean(a.max(axis=2) >= 250)) * 100, 2),
        "crushed": round(float(np.mean(lum < 8)) * 100, 2),
        "mean": round(float(lum.mean()), 1),
    }


def crop(img):
    w, h = img.size
    x0, y0, x1, y1 = CROP
    return img.crop((int(w * x0), int(h * y0), int(w * x1), int(h * y1)))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("out")
    ap.add_argument("dirs", nargs="+", metavar="LABEL=DIR")
    ap.add_argument("--shots", required=True)
    ns = ap.parse_args()
    dirs = [d.split("=", 1) for d in ns.dirs]
    os.makedirs(ns.out, exist_ok=True)
    report = {}
    for shot in [s.strip() for s in ns.shots.split(",") if s.strip()]:
        tiles, report[shot] = [], {}
        for label, d in dirs:
            path = f"{d}/{shot}.png"
            if not os.path.exists(path):
                continue
            full = Image.open(path).convert("RGB")
            tile = crop(full)
            s = report[shot][label] = {"whole": stats(full), "crop": stats(tile)}
            text = (
                f"{label}  clipped {s['whole']['clipped']}%  crushed {s['whole']['crushed']}%"
                f"  mean {s['whole']['mean']}"
            )
            ImageDraw.Draw(tile).text((12, 10), text, fill=(255, 255, 0))
            tiles.append(tile)
            print(f"{shot:24} {text}", flush=True)
        if not tiles:
            continue
        w = max(t.width for t in tiles)
        stack = Image.new("RGB", (w, sum(t.height for t in tiles)), (0, 0, 0))
        y = 0
        for t in tiles:
            stack.paste(t, (0, y))
            y += t.height
        stack.save(f"{ns.out}/{shot.replace(' ', '_')}.jpg", quality=92)
    with open(f"{ns.out}/stats.json", "w") as f:
        json.dump(report, f, indent=1)


if __name__ == "__main__":
    main()
