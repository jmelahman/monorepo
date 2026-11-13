"""Draws the island map the route screen shows: a hillshaded relief of the world's heightmap,
cropped to the land, and beside it how world positions land on it.

    Content/Menu/map.jpg    the picture, 1232 px wide
    Content/Menu/map.json   {"width", "height", "scale", "x0", "y0", "labels"}: a point at world
                            (x, y) cm is at (x * scale + x0, y * scale + y0) px, and each island's
                            name sits at its labels entry, in px

The game reads both at run time (DirectoriesToAlwaysStageAsNonUFS in DefaultGame.ini packages
them). Run with uv run --with numpy --with pillow python -I scripts/make_menu_map.py
"""

import json
import os

import numpy as np
from PIL import Image

HUB = os.environ.get("OC_HUB") or "/workspace/ue/world_gen/ride_hub"
OUT = os.environ.get("OC_MENU") or "/workspace/ue/VeloverseUE/Content/Menu"
WIDTH = 1232
PAD = 120  # heightmap pixels of sea kept round the land
COAST = "The Coast"


def relief(height, spacing_m):
    """Land shaded from green lowland to pale rock by height and lit from the north west; the sea
    darkens with depth."""
    gy, gx = np.gradient(height, spacing_m)
    azimuth, altitude = np.radians(315), np.radians(40)
    slope = np.arctan(np.hypot(gx, gy))
    aspect = np.arctan2(-gx, gy)
    shade = np.sin(altitude) * np.cos(slope)
    shade += np.cos(altitude) * np.sin(slope) * np.cos(azimuth - aspect)
    shade = np.clip(shade, 0, 1)[..., None]
    t = np.clip(height / 800, 0, 1)[..., None] ** 0.6
    low = np.array([0x3E, 0x52, 0x3F], np.float32)
    high = np.array([0xC9, 0xC2, 0xB2], np.float32)
    land = (low * (1 - t) + high * t) * (0.45 + 0.75 * shade)
    depth = np.clip(-height / 30, 0, 1)[..., None]
    shallow = np.array([0x1D, 0x33, 0x40], np.float32)
    deep = np.array([0x0F, 0x1C, 0x26], np.float32)
    sea = shallow * (1 - depth) + deep * depth
    return np.where((height > 0.5)[..., None], land, sea)


def main():
    with open(os.path.join(HUB, "info.json")) as f:
        info = json.load(f)
    with open(os.path.join(HUB, "routes.json")) as f:
        routes = json.load(f)["routes"]
    n, spacing_m = info["size"], info["spacing"] / 100
    raw = np.fromfile(os.path.join(HUB, "height.r16"), "<u2").reshape(n, n).astype(np.float32)
    height = (raw - 32768) * info["z_scale"] / 128 / 100
    rows, cols = np.where(height > 0.5)
    r0, r1 = max(rows.min() - PAD, 0), min(rows.max() + PAD, n - 1)
    c0, c1 = max(cols.min() - PAD, 0), min(cols.max() + PAD, n - 1)
    picture = relief(height[r0:r1, c0:c1], spacing_m)
    tall = round(WIDTH * (r1 - r0) / (c1 - c0))
    image = Image.fromarray(np.clip(picture, 0, 255).astype(np.uint8))
    image = image.resize((WIDTH, tall), Image.LANCZOS)

    # The heightmap's centre is the world's origin; routes.json is in metres.
    px = WIDTH / (c1 - c0)
    scale = px / (100 * spacing_m)
    x0 = ((n - 1) / 2 - c0) * px
    y0 = ((n - 1) / 2 - r0) * px

    # Each island's label goes at the middle of the grand tour's stretch on it.
    labels = {}
    rides = {r["key"]: r for r in info["rides"]}
    tour = np.array(routes["tour"])[:, :2] * 100
    along = np.concatenate([[0], np.cumsum(np.hypot(*np.diff(tour, axis=0).T))])
    stretches = rides["tour"]["stretches"]
    stretch_scale = along[-1] / sum(length for _, length in stretches)
    done = 0.0
    for name, length in stretches:
        a = np.searchsorted(along, done * stretch_scale)
        b = max(np.searchsorted(along, (done + length) * stretch_scale), a + 1)
        done += length
        if name != COAST and name not in labels:
            x, y = tour[a:b].mean(0)
            labels[name] = [round(x * scale + x0, 1), round(y * scale + y0, 1)]

    os.makedirs(OUT, exist_ok=True)
    image.save(os.path.join(OUT, "map.jpg"), quality=88)
    meta = {"width": WIDTH, "height": tall, "scale": scale, "x0": x0, "y0": y0, "labels": labels}
    with open(os.path.join(OUT, "map.json"), "w") as f:
        json.dump(meta, f, indent=1)
    print(f"wrote {OUT}/map.jpg ({WIDTH}x{tall}) and map.json, labels {sorted(labels)}")


if __name__ == "__main__":
    main()
