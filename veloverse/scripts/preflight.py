"""Offline pre-flight for a hub build: seconds of numpy, not a ~25 min step, to find a bad verge.

    preflight.py [SRC]   (default /workspace/ue/world_gen/ride_hub)

Reads the layer weights (*.r8, a 7 m sample each) and road_dist.png that make_world_level.py
wrote, and profiles each layer's mean weight by distance from the nearest road centre, split into
the green stretches and the dry ones (desert, beach). It fails (exit 1) when the green verge
isn't carpet: bare layers (Dirt, Rock, Verge, Sand, Desert) over BARE_MAX of the weight between
the tarmac's edge and VERGE_OUT m, the strip the g1 work cleared (a7981716). Run it after
make_world_level.py and before queuing a build.
"""

import itertools
import sys
from pathlib import Path

import numpy as np
from PIL import Image

DIST_NEAR = 16.0  # make_world_level.DIST_NEAR: the offsets' range in road_dist.png
ROAD_EDGE, VERGE_OUT = 3.5, 10.0  # the verge checked, in m from the road centre
BARE_MAX = 0.15
GREEN = ("Grass", "Heath", "Forest")
BARE = ("Dirt", "Rock", "Verge", "Sand", "Desert")


def load(src):
    size = round(Path(src, "Grass.r8").stat().st_size ** 0.5)
    layers = {}
    for f in sorted(Path(src).glob("*.r8")):
        layers[f.stem] = np.fromfile(f, np.uint8).reshape(size, size).astype(np.float32) / 255
    enc = np.asarray(Image.open(Path(src, "road_dist.png")))[:size, :size]
    near = enc[..., 2] > 127
    off = (enc[..., :2].astype(np.float32) / 255 * 2 - 1) * DIST_NEAR
    dist = np.where(near, np.hypot(off[..., 0], off[..., 1]), np.inf)
    return layers, dist


def profile(layers, dist, mask, edges):
    rows = []
    for a, b in itertools.pairwise(edges):
        sel = mask & (dist >= a) & (dist < b)
        n = int(sel.sum())
        rows.append((a, b, n, {k: float(v[sel].mean()) if n else 0.0 for k, v in layers.items()}))
    return rows


def main(src):
    layers, dist = load(src)
    green_w = sum(layers[k] for k in GREEN if k in layers)
    bare_w = sum(layers[k] for k in BARE if k in layers)
    # Dry where desert and sand hold most of the sample's weight; green everywhere else.
    dry = (layers.get("Desert", 0) + layers.get("Sand", 0)) > 0.5
    green = ~dry
    edges = np.arange(0, DIST_NEAR + 1, 2.0)
    names = [k for k in (*GREEN, *BARE, "Snow") if k in layers]
    ok = True
    for label, mask in (("green", green), ("dry", dry)):
        print(f"{label} stretches, mean weight by distance from the road centre:")
        print("   m       n  " + " ".join(f"{k[:6]:>6}" for k in names))
        for a, b, n, w in profile(layers, dist, mask, edges):
            print(f"{a:4.0f}-{b:<3.0f}{n:7d}  " + " ".join(f"{w[k]:6.2f}" for k in names))
    sel = green & (dist >= ROAD_EDGE) & (dist < VERGE_OUT)
    if sel.any():
        bare = float(bare_w[sel].mean())
        carpet = float(green_w[sel].mean())
        worst = float(np.percentile(bare_w[sel], 95))
        verdict = "ok" if bare <= BARE_MAX else "FAIL"
        ok &= bare <= BARE_MAX
        print(
            f"green verge {ROAD_EDGE}-{VERGE_OUT} m: carpet {carpet:.2f}, bare {bare:.2f} "
            f"(95th percentile {worst:.2f}, limit {BARE_MAX}): {verdict}"
        )
    print("PREFLIGHT " + ("OK" if ok else "FAILED"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else "/workspace/ue/world_gen/ride_hub"))
