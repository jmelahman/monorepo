"""The open world: an island about 7 km across, with a harbour town, coast, forest, farmland,
a mountain and a desert, and roads between them.

Run with system Python: python3 -I scripts/make_island.py. It needs numpy and scipy, which it
looks for in $OC_PYLIB (default /opt/oc-pylib). OC_RES sets the grid: 1025 (the default) makes a
quick preview at 8 m; 8129 is the landscape, a metre a sample. Writes to
/workspace/ue/world_gen/island[_<res>]:
- preview.png: the map, shaded, coloured by region, with the roads;
- profile.png: each road's height along its length;
- height.r16: 16-bit heights, RES x RES, rows along Y, 32768 at sea level, Z_SCALE / 128 cm a
  step (Unreal's landscape import);
- <Layer>.r8: a paint layer's weight per sample, the layers summing to 255;
- layers.png: the strongest layer at each sample, to check them by eye;
- roads.json: each road's centreline every metre, and info.json: the grid;
- routes.json: each ride's centreline every metre, stitched from the roads, with its length and
  climb;
- road_<name>_<k>.gltf: each road's surface in 200 m pieces, laid on the carved ground.

Coordinates are Unreal's: X east, Y south, metres here, the origin in the middle of the map.
"""

import json
import math
import os
import sys

sys.path.append(os.environ.get("OC_PYLIB", "/opt/oc-pylib"))
import numpy as np
from PIL import Image
from scipy import ndimage

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import road_mesh

RES = int(os.environ.get("OC_RES", "1025"))
EXTENT = 8128.0  # metres across the map
SPACING = EXTENT / (RES - 1)
OUT = "/workspace/ue/world_gen/island" + ("" if RES == 8129 else f"_{RES}")
SEED = 7

# Regions: centre (km, X east, Y south), reach (km). Coast is not here: it is the band along the
# shore, whatever region lies behind it.
REGIONS = {
    "mountain": ((-0.3, -2.75), 1.3),
    "desert": ((2.4, -0.5), 1.5),
    "forest": ((-1.9, 0.3), 1.5),
    "meadow": ((0.2, 0.8), 1.4),
    "town": ((1.7, 2.3), 0.9),
}
COLOURS = {
    "mountain": (128, 122, 118),
    "desert": (214, 170, 110),
    "forest": (52, 92, 48),
    "meadow": (124, 156, 80),
    "town": (170, 150, 140),
    "beach": (226, 212, 170),
    "sea": (40, 80, 120),
}

# Roads: control points in km. Each later road starts and ends on an earlier one; its ends take
# that road's height there.
ROADS = {
    # The ring, clockwise from the harbour: the south coast and its beaches, the west cliffs, the
    # forest, over the mountain's shoulder, down across the desert and back to town.
    "ring": [
        (1.55, 2.5),
        (0.9, 3.3),
        (0.5, 3.31),
        (0.1, 3.47),
        (-0.3, 3.49),
        (-0.8, 3.25),
        (-1.35, 2.68),
        (-1.95, 2.45),
        (-2.55, 2.2),
        (-3.15, 1.35),
        (-3.4, 0.45),
        (-3.25, -0.45),
        (-2.7, -1.25),
        (-1.95, -1.5),
        (-1.35, -1.35),
        (-0.85, -1.6),
        (-0.3, -1.45),
        (0.3, -1.7),
        (0.95, -1.5),
        (1.6, -1.3),
        (2.35, -1.15),
        (2.95, -0.5),
        (3.1, 0.4),
        (2.75, 1.2),
        (2.2, 1.9),
        (1.55, 2.5),
    ],
    # Across the middle: from the forest through the farmland to the desert.
    "cross": [
        (-2.7, -1.25),
        (-2.0, -0.3),
        (-1.1, 0.35),
        (-0.1, 0.65),
        (0.9, 0.45),
        (1.8, 0.05),
        (2.95, -0.5),
    ],
    # Up from the farmland to the town.
    "valley": [(-0.1, 0.65), (0.4, 1.5), (0.95, 2.1), (1.55, 2.5)],
    # Behind the beaches: from the ring at the south-west cove, along the back of the coast to
    # the valley road above the town.
    "coast_back": [(-1.35, 2.68), (-0.9, 2.25), (-0.3, 2.3), (0.4, 2.35), (0.95, 2.1)],
    # Through the farmland, from the cross road down to the coast road.
    "lane": [(-0.6, 0.52), (-0.55, 1.1), (-0.35, 1.7), (-0.3, 2.3)],
    # Through the forest, from the cross road down to the ring by the west cliffs.
    "forest": [(-1.1, 0.35), (-1.55, 0.95), (-2.05, 1.5), (-2.55, 2.2)],
    # From the ring below the mountain down past the first mesa to the cross road.
    "mesa": [(0.95, -1.5), (0.85, -1.0), (0.95, -0.45), (0.9, 0.45)],
    # From the cross road through the dunes, between two mesas, down to the ring.
    "dunes": [(1.8, 0.05), (2.0, 0.5), (2.08, 0.95), (2.25, 1.3), (2.5, 1.55)],
}
# Roads found rather than drawn: from a point on the ring, up the mountain's south face to its
# summit at no more than CLIMB_GRADE, switching back where the face is steep.
CLIMBS = {"summit": (0.3, -1.7)}
CLIMB_GRADE = 0.065
CLIMB_CELL = 16.0  # metres: the planning grid, about the width of the road and its shoulders
CLIMB_SMOOTH = 120.0  # metres: the climb follows the mountain's broad shape, not its crags
# Rides: each a loop of legs (road, from, to), points in km taken as the nearest on the road.
# An open road runs either way; a closed one runs forward, the way its points go. A ride whose
# last leg doesn't come back to its first is ridden there and back.
ROUTES = {
    "island": [("ring", (1.55, 2.5), (1.55, 2.5))],
    "coast": [
        ("ring", (1.55, 2.5), (-1.35, 2.68)),
        ("coast_back", (-1.35, 2.68), (0.95, 2.1)),
        ("valley", (0.95, 2.1), (1.55, 2.5)),
    ],
    "forest": [
        ("cross", (-2.7, -1.25), (-1.1, 0.35)),
        ("forest", (-1.1, 0.35), (-2.55, 2.2)),
        ("ring", (-2.55, 2.2), (-2.7, -1.25)),
    ],
    "meadow": [
        ("cross", (-0.6, 0.52), (-0.1, 0.65)),
        ("valley", (-0.1, 0.65), (0.95, 2.1)),
        ("coast_back", (0.95, 2.1), (-0.3, 2.3)),
        ("lane", (-0.3, 2.3), (-0.6, 0.52)),
    ],
    "desert": [
        ("ring", (0.95, -1.5), (2.95, -0.5)),
        ("cross", (2.95, -0.5), (0.9, 0.45)),
        ("mesa", (0.9, 0.45), (0.95, -1.5)),
    ],
    "dunes": [
        ("cross", (1.8, 0.05), (2.95, -0.5)),
        ("ring", (2.95, -0.5), (2.5, 1.55)),
        ("dunes", (2.5, 1.55), (1.8, 0.05)),
    ],
    "summit": [("summit", (0.3, -1.7), "end")],
}
CLOSED = {"ring"}
ROAD_HALF = 3.2  # metres: two narrow lanes
SHOULDER = 3.0
BANK_SLOPE = 0.7  # rise per metre of the cut and fill banks either side
CORRIDOR = 140.0  # metres from a road beyond which the ground is left alone
MAX_GRADE = 0.08
Z_SCALE = 512.0  # the landscape's Z scale: +-1310 m
SNOW_LINE = 640.0  # metres: the summit climb's last hairpins and viewpoint are in snow
# Paint layers and their colours in layers.png.
LAYERS = {
    "Grass": (110, 150, 70),
    "Forest": (50, 75, 40),
    "Rock": (120, 115, 110),
    "Snow": (240, 240, 245),
    "Desert": (200, 150, 95),
    "Sand": (228, 214, 170),
    "Dirt": (130, 105, 80),
}


# --- noise ---------------------------------------------------------------------------------


def _hash(ix, iy, seed):
    h = (ix.astype(np.uint32) * np.uint32(374761393)) ^ (
        iy.astype(np.uint32) * np.uint32(668265263)
    )
    h = (h ^ np.uint32(seed * 2654435761 & 0xFFFFFFFF)) * np.uint32(1274126177)
    h ^= h >> np.uint32(16)
    return (h & np.uint32(0xFFFFFF)).astype(np.float32) / np.float32(0xFFFFFF)


def value(x, y, seed):
    """Gradient (Perlin) noise, 0..1: smooth and free of value noise's square cells."""
    ix, iy = np.floor(x), np.floor(y)
    fx, fy = (x - ix).astype(np.float32), (y - iy).astype(np.float32)
    ix, iy = ix.astype(np.int64), iy.astype(np.int64)

    def grad(cx, cy, dx, dy):
        angle = _hash(cx, cy, seed) * np.float32(2 * np.pi)
        return np.cos(angle) * dx + np.sin(angle) * dy

    sx = fx * fx * fx * (fx * (fx * 6 - 15) + 10)
    sy = fy * fy * fy * (fy * (fy * 6 - 15) + 10)
    a = grad(ix, iy, fx, fy)
    b = grad(ix + 1, iy, fx - 1, fy)
    c = grad(ix, iy + 1, fx, fy - 1)
    d = grad(ix + 1, iy + 1, fx - 1, fy - 1)
    top = a + (b - a) * sx
    n = top + ((c + (d - c) * sx) - top) * sy
    return np.clip(n * 0.75 + 0.5, 0, 1)


def fbm(x, y, scale, octaves=5, seed=0):
    """0..1, features about `scale` metres across."""
    total = np.zeros_like(x, dtype=np.float32)
    amp, norm, f = 1.0, 0.0, 1.0 / scale
    for k in range(octaves):
        total += amp * value(x * f, y * f, seed + k * 101)
        norm += amp
        amp *= 0.5
        f *= 2.0
    return total / norm


def ridged(x, y, scale, octaves=6, seed=0):
    """0..1, sharp crests: mountain ridges."""
    total = np.zeros_like(x, dtype=np.float32)
    amp, norm, f, weight = 1.0, 0.0, 1.0 / scale, np.ones_like(x, dtype=np.float32)
    for k in range(octaves):
        n = 1.0 - np.abs(value(x * f, y * f, seed + k * 131) * 2 - 1)
        n = n * n * weight
        weight = np.clip(n * 1.6, 0, 1)
        total += amp * n
        norm += amp
        amp *= 0.5
        f *= 2.1
    return total / norm


def smoothstep(a, b, x):
    t = np.clip((x - a) / (b - a), 0, 1)
    return t * t * (3 - 2 * t)


# --- the land ------------------------------------------------------------------------------


def grid():
    c = (np.arange(RES, dtype=np.float32) - (RES - 1) / 2) * SPACING
    x, y = np.meshgrid(c, c)  # rows along Y, columns along X
    return x, y


def land_mask(x, y):
    """True on land: a warped ellipse with a bay for the harbour."""
    wx = x + 900 * (fbm(x, y, 1900, 4, SEED) - 0.5) * 2
    wy = y + 900 * (fbm(x, y, 1900, 4, SEED + 50) - 0.5) * 2
    d = np.hypot(wx / 3450, (wy - 150) / 3300)
    d -= 0.22 * (fbm(x, y, 600, 5, SEED + 9) - 0.5)
    # The harbour's bay in the south-east, and a headland running out south-west of the beaches.
    bay = np.exp(-(((x - 2350) / 650) ** 2 + ((y - 3150) / 520) ** 2))
    along = (x + 2300) * 0.6 + (y - 3000) * 0.8
    across = (x + 2300) * 0.8 - (y - 3000) * 0.6
    headland = np.exp(-((along / 1100) ** 2 + (across / 260) ** 2))
    return (d + 0.45 * bay - 0.35 * headland) < 1.0


def region_weights(x, y):
    wx = x + 500 * (fbm(x, y, 1500, 4, SEED + 3) - 0.5) * 2
    wy = y + 500 * (fbm(x, y, 1500, 4, SEED + 4) - 0.5) * 2
    w = {}
    for name, ((cx, cy), reach) in REGIONS.items():
        d2 = ((wx - cx * 1000) ** 2 + (wy - cy * 1000) ** 2) / (reach * 1000) ** 2
        w[name] = np.exp(-d2 * 1.6)
    total = sum(w.values()) + 1e-6
    return {k: v / total for k, v in w.items()}


# The desert's landmarks, placed clear of the roads. Mesas: centre (km), radius and height (m).
MESAS = [
    ((1.35, -0.75), 190, 70),
    ((2.05, -1.62), 260, 95),
    ((2.4, 0.88), 170, 60),
    ((1.75, 1.0), 120, 45),
    ((2.8, -1.65), 140, 55),
]
# One canyon, winding between the ring and the cross road, seen from both.
CANYON = [
    (1.1, 0.0),
    (1.35, -0.18),
    (1.6, -0.08),
    (1.85, -0.3),
    (2.1, -0.2),
    (2.3, -0.45),
    (2.5, -0.62),
]
DUNES = ((2.25, 0.55), 620)  # centre (km), reach (m)


def mesas(x, y):
    h = np.zeros_like(x)
    for i, ((cx, cy), r, top) in enumerate(MESAS):
        near = np.abs(x - cx * 1000) < r * 2.5
        near &= np.abs(y - cy * 1000) < r * 2.5
        if not near.any():
            continue
        xs, ys = x[near], y[near]
        # Each one long on its own bearing, its edge broken into spurs and bays.
        a = 0.9 * i + 0.4
        u = (xs - cx * 1000) * np.cos(a) + (ys - cy * 1000) * np.sin(a)
        v = -(xs - cx * 1000) * np.sin(a) + (ys - cy * 1000) * np.cos(a)
        d = np.hypot(u / 1.5, v / 0.75) / r
        d = d * (1 + 0.9 * (fbm(xs, ys, r * 0.9, 4, SEED + 70 + i) - 0.5))
        cap = smoothstep(1.0, 0.93, d)  # the cliff under the caprock
        ledge = smoothstep(1.3, 1.18, d)  # a lower step
        talus = smoothstep(1.9, 1.15, d)
        h[near] = top * (0.55 * cap + 0.25 * ledge + 0.2 * talus**1.5)
    return h


def canyon(x, y):
    """Depth (positive) of the canyon: a floor and a terrace below the rim, deepest mid-way."""
    path = catmull(CANYON, False, step=SPACING * 0.5)
    along = np.linspace(0, 1, len(path), dtype=np.float32)
    col = np.round(path[:, 0] / SPACING + (RES - 1) / 2).astype(int)
    row = np.round(path[:, 1] / SPACING + (RES - 1) / 2).astype(int)
    mark = np.full(x.shape, np.nan, dtype=np.float32)
    mark[row, col] = along
    dist, (ri, ci) = ndimage.distance_transform_edt(np.isnan(mark), return_indices=True)
    dist = dist * SPACING
    s = mark[ri, ci]
    reach = np.clip(np.sin(np.pi * s), 0, 1) ** 0.6
    width = 25 + 45 * reach
    dist = dist + 30 * (fbm(x, y, 160, 4, SEED + 80) - 0.5)
    floor = 1 - smoothstep(width * 0.45, width, dist)
    terrace = 1 - smoothstep(width * 1.35, width * 1.75, dist)
    return 70 * reach * (0.6 * floor + 0.4 * terrace)


def dunes(x, y):
    (cx, cy), reach = DUNES
    fade = 1 - smoothstep(reach * 0.5, reach, np.hypot(x - cx * 1000, y - cy * 1000))
    # Ridges across a westerly wind, gentle to windward and steep to lee.
    wx = x + 140 * (fbm(x, y, 500, 3, SEED + 90) - 0.5)
    phase = (wx * 0.94 + y * 0.34) / 150
    f = phase - np.floor(phase)
    ridge = np.where(f < 0.78, f / 0.78, (1 - f) / 0.22) ** 1.6
    return fade * (9 * ridge * (0.5 + fbm(x, y, 700, 2, SEED + 91)))


def region_heights(x, y):
    (mx, my), _ = REGIONS["mountain"]
    peak = np.exp(-(((x - mx * 1000) ** 2 + ((y - my * 1000) * 1.3) ** 2) / 1150**2))
    return {
        "mountain": 110 + 1000 * peak * ridged(x, y, 1100, 6, SEED + 11) + 120 * peak,
        "desert": 120
        + 40 * fbm(x, y, 900, 4, SEED + 23)
        + mesas(x, y)
        + dunes(x, y)
        - canyon(x, y),
        "forest": 45 + 110 * fbm(x, y, 800, 5, SEED + 31),
        "meadow": 25 + 45 * fbm(x, y, 1300, 4, SEED + 41),
        "town": 6 + 30 * fbm(x, y, 1200, 3, SEED + 51),
    }


def terrain(x, y):
    land = land_mask(x, y)
    inland = ndimage.distance_transform_edt(land) * SPACING  # metres from the shore, on land
    offshore = ndimage.distance_transform_edt(~land) * SPACING
    weights = region_weights(x, y)
    heights = region_heights(x, y)
    h = sum(weights[k] * heights[k] for k in weights)
    # The shore: cliffs along the west and north, beaches in the south. `cliff` 0..1.
    cliff = smoothstep(-1800, -2600, x) * smoothstep(2400, 1200, y) + smoothstep(-2200, -3000, y)
    cliff = np.clip(cliff + 0.5 * (fbm(x, y, 900, 3, SEED + 61) - 0.5), 0, 1)
    fall = 380 + (35 - 380) * cliff
    beach = 2.5 + 1.5 * fbm(x, y, 400, 2, SEED + 62)
    rise = smoothstep(0, 1, inland / fall)
    h = beach + (h - beach) * rise
    sea = -1.0 - 28 * smoothstep(0, 900, offshore) - 6 * smoothstep(0, 60, offshore) * cliff
    h = np.where(land, h, sea)
    return h.astype(np.float32), land, inland, weights, cliff


# --- roads ---------------------------------------------------------------------------------


def catmull(points, closed, step=1.0):
    """A centripetal Catmull-Rom through the control points (metres), resampled every `step`."""
    p = [np.array(q, dtype=np.float64) * 1000 for q in points]
    if closed:
        p = p[:-1] if np.allclose(p[0], p[-1]) else p
        ext = [p[-1], *p, p[0], p[1]]
    else:
        ext = [2 * p[0] - p[1], *p, 2 * p[-1] - p[-2]]
    dense = []
    for i in range(1, len(ext) - 2):
        p0, p1, p2, p3 = ext[i - 1 : i + 3]
        t0 = 0.0
        t1 = t0 + np.linalg.norm(p1 - p0) ** 0.5
        t2 = t1 + np.linalg.norm(p2 - p1) ** 0.5
        t3 = t2 + np.linalg.norm(p3 - p2) ** 0.5
        for t in np.linspace(t1, t2, 200, endpoint=False):
            a1 = (t1 - t) / (t1 - t0) * p0 + (t - t0) / (t1 - t0) * p1
            a2 = (t2 - t) / (t2 - t1) * p1 + (t - t1) / (t2 - t1) * p2
            a3 = (t3 - t) / (t3 - t2) * p2 + (t - t2) / (t3 - t2) * p3
            b1 = (t2 - t) / (t2 - t0) * a1 + (t - t0) / (t2 - t0) * a2
            b2 = (t3 - t) / (t3 - t1) * a2 + (t - t1) / (t3 - t1) * a3
            dense.append((t2 - t) / (t2 - t1) * b1 + (t - t1) / (t2 - t1) * b2)
    dense.append(ext[-2])
    dense = np.array(dense)
    seg = np.linalg.norm(np.diff(dense, axis=0), axis=1)
    s = np.concatenate([[0], np.cumsum(seg)])
    at = np.arange(0, s[-1], step)
    return np.stack([np.interp(at, s, dense[:, 0]), np.interp(at, s, dense[:, 1])], axis=1)


def sample(h, xy):
    """Bilinear heights at points (metres)."""
    col = xy[:, 0] / SPACING + (RES - 1) / 2
    row = xy[:, 1] / SPACING + (RES - 1) / 2
    return ndimage.map_coordinates(h, [row, col], order=1, mode="nearest")


def grade_fit(z, fixed, closed):
    """Heights with no metre steeper than MAX_GRADE: the mean of the highest such profile on or
    under `z` (all cut) and the lowest on or over it (all fill), so cuts and fills balance, then
    eased. Each bound keeps the grade and a mean or a moving average of them can't exceed it.
    `fixed` maps indices to heights they must meet; each is eased in over enough road to add no
    more than 0.5% to the grade."""
    # Leave room for the 0.5% the fixed heights may add.
    g = MAX_GRADE - 0.007
    n = len(z)
    laps = 2 if closed else 1
    at = np.arange(n)

    def gap(i):
        d = np.abs(at - i)
        return np.minimum(d, n - d) if closed else d

    # Keep the ground within reach of each fixed height at the grade, so easing them in after
    # has little left to make up.
    for i, v in fixed.items():
        z = np.clip(z, v - g * gap(i), v + g * gap(i))

    def envelope(sign):
        e = z.astype(np.float64).copy()
        for _ in range(laps):
            for i in range(1, n):
                e[i] = sign * min(sign * e[i], sign * e[i - 1] + g)
            if closed:
                e[0] = sign * min(sign * e[0], sign * e[-1] + g)
            for i in range(n - 2, -1, -1):
                e[i] = sign * min(sign * e[i], sign * e[i + 1] + g)
            if closed:
                e[-1] = sign * min(sign * e[-1], sign * e[0] + g)
        return e

    out = smooth((envelope(1) + envelope(-1)) / 2, 25, closed)
    for i, v in fixed.items():
        reach = min(max(abs(v - out[i]) / 0.005, 1.0), n / 2)
        out = out + (v - out[i]) * np.clip(1 - gap(i) / reach, 0, 1)
    return out


def smooth(z, span, closed):
    k = np.ones(span * 2 + 1) / (span * 2 + 1)
    if closed:
        return np.convolve(np.concatenate([z[-span:], z, z[:span]]), k, "valid")
    padded = np.concatenate([np.full(span, z[0]), z, np.full(span, z[-1])])
    return np.convolve(padded, k, "valid")


def climb(h, start, roads):
    """Control points (km) for a road from `start` to the highest ground near it: a least-cost
    path over a CLIMB_CELL grid of the smoothed heights, where a step steeper than CLIMB_GRADE
    costs many times its length, so the path winds and switches back rather than going straight
    up. Going down on the way up, steep ground across the road and turning cost extra too: each
    state is a cell and the way the road is heading, and a bend sharper than 90 degrees in one
    step isn't allowed, so hairpins are drawn out over a few cells rather than folding back."""
    import heapq

    x0, x1, y0, y1 = start[0] - 1.6, start[0] + 1.6, start[1] - 1.7, start[1] + 0.15
    cx = np.arange(x0 * 1000, x1 * 1000, CLIMB_CELL)
    cy = np.arange(y0 * 1000, y1 * 1000, CLIMB_CELL)
    gx, gy = np.meshgrid(cx, cy)
    soft = ndimage.gaussian_filter(h, CLIMB_SMOOTH / SPACING)
    g = sample(soft, np.stack([gx.ravel(), gy.ravel()], axis=1)).reshape(gx.shape)
    side = np.hypot(*np.gradient(g, CLIMB_CELL))
    rows, cols = g.shape
    # The summit: the highest cell well inside the coast.
    # Other roads, to keep clear of once the climb has left the one it starts from.
    near = np.zeros(g.shape, dtype=bool)
    for road in roads.values():
        col = np.round((road["xy"][:, 0] - cx[0]) / CLIMB_CELL).astype(int)
        row = np.round((road["xy"][:, 1] - cy[0]) / CLIMB_CELL).astype(int)
        ok = (col >= 0) & (col < len(cx)) & (row >= 0) & (row < len(cy))
        near[row[ok], col[ok]] = True
    near = ndimage.distance_transform_edt(~near) * CLIMB_CELL < 150
    near &= np.hypot(gx - start[0] * 1000, gy - start[1] * 1000) > 200
    usable = g > 40
    usable = ndimage.binary_erosion(usable, iterations=4)
    goal = np.unravel_index(np.argmax(np.where(usable, g, -1e9)), g.shape)
    begin = (
        round((start[1] * 1000 - cy[0]) / CLIMB_CELL),
        round((start[0] * 1000 - cx[0]) / CLIMB_CELL),
    )
    steps = [
        (dr, dc)
        for dr in range(-2, 3)
        for dc in range(-2, 3)
        if (dr, dc) != (0, 0) and math.gcd(abs(dr), abs(dc)) == 1
    ]
    angle = [math.atan2(dr, dc) for dr, dc in steps]
    turns = [
        [abs((angle[j] - angle[i] + math.pi) % (2 * math.pi) - math.pi) for j in range(len(steps))]
        for i in range(len(steps))
    ]
    best = np.full((*g.shape, len(steps)), np.inf)
    came = {}
    todo = []
    for d in range(len(steps)):
        best[(*begin, d)] = 0.0
        todo.append((0.0, (*begin, d)))
    end = None
    while todo:
        cost, state = heapq.heappop(todo)
        r, c, d = state
        if (r, c) == goal:
            end = state
            break
        if cost > best[state]:
            continue
        for e, (dr, dc) in enumerate(steps):
            turn = turns[d][e]
            if turn > math.pi / 2 + 1e-6:
                continue
            nr, nc = r + dr, c + dc
            if not (0 <= nr < rows and 0 <= nc < cols) or g[nr, nc] < 15:
                continue
            length = CLIMB_CELL * math.hypot(dr, dc)
            rise = (g[nr, nc] - g[r, c]) / length
            steep = max(0.0, abs(rise) - CLIMB_GRADE) / CLIMB_GRADE
            down = max(0.0, -rise) / CLIMB_GRADE
            across = max(0.0, side[nr, nc] - 0.45)
            crowd = 20.0 if near[nr, nc] else 0.0
            step = length * (1 + 30 * steep + 80 * steep * steep + 1.5 * down + 4 * across + crowd)
            step += 12 * (turn / (math.pi / 2)) ** 2
            nxt = (nr, nc, e)
            if cost + step < best[nxt]:
                best[nxt] = cost + step
                came[nxt] = state
                heapq.heappush(todo, (cost + step, nxt))
    path = [end]
    while path[-1][:2] != begin:
        path.append(came[path[-1]])
    path = [p[:2] for p in reversed(path)]
    # Stop below the summit, where the path would otherwise wind round and round its cone.
    top = np.array(goal) * CLIMB_CELL
    short = [np.hypot(*(np.array(p) * CLIMB_CELL - top)) < 220 for p in path]
    path = path[: short.index(True) + 1]
    pts = np.array([(cx[c], cy[r]) for r, c in path]) / 1000
    pts[0] = start
    return [tuple(p) for p in simplify(pts, 6 / 1000)]


def simplify(pts, tolerance):
    """Ramer-Douglas-Peucker: the fewest of `pts` within `tolerance` of the line through all."""
    keep = np.zeros(len(pts), dtype=bool)
    keep[[0, -1]] = True
    stack = [(0, len(pts) - 1)]
    while stack:
        a, b = stack.pop()
        if b - a < 2:
            continue
        d = pts[b] - pts[a]
        n = np.hypot(*d) or 1.0
        off = np.abs(
            d[0] * (pts[a + 1 : b, 1] - pts[a, 1]) - d[1] * (pts[a + 1 : b, 0] - pts[a, 0])
        )
        k = int(np.argmax(off))
        if off[k] / n > tolerance:
            keep[a + 1 + k] = True
            stack += [(a, a + 1 + k), (a + 1 + k, b)]
    return pts[keep]


def soften(h, xy):
    """The ground near a climbed road eased toward its broad shape: crags scaled to a quarter
    within 60 m of the road, coming back in full by 300 m, so the road runs on a flank it can
    follow rather than cutting through every crest."""
    on = np.zeros(h.shape, dtype=bool)
    col = np.round(xy[:, 0] / SPACING + (RES - 1) / 2).astype(int)
    row = np.round(xy[:, 1] / SPACING + (RES - 1) / 2).astype(int)
    ok = (col >= 0) & (col < RES) & (row >= 0) & (row < RES)
    on[row[ok], col[ok]] = True
    dist = ndimage.distance_transform_edt(~on) * SPACING
    soft = ndimage.gaussian_filter(h, CLIMB_SMOOTH / SPACING)
    keep = 0.25 + 0.75 * smoothstep(60, 300, dist)
    return np.where(h > 15, soft + (h - soft) * keep, h).astype(np.float32)


def lay_roads(h):
    """Each road's centreline every metre with its height: the ground smoothed along it, kept off
    the sea and graded; later roads meet earlier ones at their height. Returns the roads and the
    ground, eased around any climbed road."""
    roads = {}
    plans = {**ROADS, **{name: None for name in CLIMBS}}
    for name, points in plans.items():
        closed = name in CLOSED
        points = climb(h, CLIMBS[name], roads) if points is None else list(points)
        # An open road's ends on the nearest point of an earlier road, so the two meet.
        for end in (0, -1) if not closed else ():
            for other in roads.values():
                d = np.linalg.norm(other["xy"] - np.array(points[end]) * 1000, axis=1)
                k = int(np.argmin(d))
                if d[k] < 300:
                    points[end] = tuple(other["xy"][k] / 1000)
                    break
        xy = catmull(points, closed)
        if name in CLIMBS:
            h = soften(h, xy)
        z = np.maximum(smooth(sample(h, xy), 90, closed), 3.0)
        # Where a road leaves another it shares that road's tarmac for a while; it keeps the
        # other's height until their verges part, or the two surfaces cross at different heights.
        fixed, shared = {}, {}
        for end, step in ((0, 1), (len(xy) - 1, -1)) if not closed else ():
            for other in roads.values():
                i, run = end, {}
                while 0 <= i < len(xy):
                    d = np.linalg.norm(other["xy"] - xy[i], axis=1)
                    k = int(np.argmin(d))
                    if d[k] >= (30 if i == end else 2 * (ROAD_HALF + SHOULDER)):
                        break
                    run[i] = other["z"][k]
                    i += step
                if run:
                    last = i - step
                    fixed[last] = run[last]
                    shared.update(run)
                    break
        z = grade_fit(z, fixed, closed)
        for i, v in shared.items():
            z[i] = v
        roads[name] = {"xy": xy, "z": z, "closed": closed}
    return roads, h


def stitch(roads):
    """Each ride's centreline (metres, with heights), from its legs."""
    rides = {}
    for name, legs in ROUTES.items():
        parts = []
        for road_name, a, b in legs:
            road = roads[road_name]
            xy, z = road["xy"], road["z"]
            ia = int(np.argmin(np.linalg.norm(xy - np.array(a) * 1000, axis=1)))
            ib = (
                len(xy) - 1
                if b == "end"
                else int(np.argmin(np.linalg.norm(xy - np.array(b) * 1000, axis=1)))
            )
            if road["closed"]:
                idx = (ia + np.arange((ib - ia) % len(xy) or len(xy))) % len(xy)
            else:
                idx = np.arange(ia, ib + 1) if ia <= ib else np.arange(ia, ib - 1, -1)
            parts.append(np.column_stack([xy[idx], z[idx]]))
        line = np.concatenate(parts)
        if np.linalg.norm(line[0, :2] - line[-1, :2]) > 50:
            line = np.concatenate([line, line[::-1][1:]])  # there and back
        rides[name] = line
        gaps = np.linalg.norm(np.diff(line[:, :2], axis=0), axis=1)
        climbed = np.clip(np.diff(line[:, 2]), 0, None).sum()
        print(
            f"route {name}: {gaps.sum() / 1000:.1f} km, climbs {climbed:.0f} m,"
            f" top {line[:, 2].max():.0f} m, widest gap {gaps.max():.0f} m"
        )
    return rides


def carve(h, roads):
    """The ground cut and filled to the roads: level across the road and its shoulders, then
    banks at BANK_SLOPE back up or down to the hills."""
    # Every road's centreline points in one list, with each point's neighbours along its road.
    xy = np.concatenate([r["xy"] for r in roads.values()])
    z = np.concatenate([r["z"] for r in roads.values()])
    before, after = [], []
    start = 0
    for road in roads.values():
        k = np.arange(start, start + len(road["xy"]))
        if road["closed"]:
            before.append(np.roll(k, 1))
            after.append(np.roll(k, -1))
        else:
            before.append(np.concatenate([k[:1], k[:-1]]))
            after.append(np.concatenate([k[1:], k[-1:]]))
        start += len(k)
    before, after = np.concatenate(before), np.concatenate(after)
    # Each sample under a centreline marks one of its points.
    target = np.full(h.shape, -1, dtype=np.int64)
    col = np.round(xy[:, 0] / SPACING + (RES - 1) / 2).astype(int)
    row = np.round(xy[:, 1] / SPACING + (RES - 1) / 2).astype(int)
    ok = (col >= 0) & (col < RES) & (row >= 0) & (row < RES)
    target[row[ok], col[ok]] = np.flatnonzero(ok)
    dist, (ri, ci) = ndimage.distance_transform_edt(target < 0, return_indices=True)
    dist = (dist * SPACING).astype(np.float32)
    # The bed takes the road's height where the sample meets the centreline square on, between
    # the nearest point and a neighbour. The nearest point's height alone steps a climb's bed up
    # a sample at a time, a rise that pokes through the tarmac 10 cm above it.
    nearest = target[ri, ci]
    px = (np.arange(RES) - (RES - 1) / 2) * SPACING
    p = np.stack(np.broadcast_arrays(px[None, :], px[:, None]), axis=-1)

    def along(p, a, b):
        d = xy[b] - xy[a]
        length2 = np.maximum((d * d).sum(-1), 1e-9)
        t = np.clip(((p - xy[a]) * d).sum(-1) / length2, 0, 1)
        off = p - (xy[a] + d * t[..., None])
        return (off * off).sum(-1), z[a] + (z[b] - z[a]) * t

    gap_back, z_back = along(p, before[nearest], nearest)
    gap_on, z_on = along(p, nearest, after[nearest])
    bed = (np.where(gap_back < gap_on, z_back, z_on) - 0.1).astype(np.float32)
    out = np.maximum(dist - ROAD_HALF - SHOULDER, 0) * BANK_SLOPE
    upper = bed + out
    lower = bed - out
    capped = np.maximum(np.minimum(h, upper), lower)
    blend = smoothstep(0, 4, out)  # sharp edges eased over the first few metres of bank
    result = bed + (capped - bed) * blend
    # Past the corridor the hills are left as they were, so a bank too tall for it ends in a
    # face rather than reaching across the map.
    keep = smoothstep(CORRIDOR * 0.6, CORRIDOR, dist)
    result = result + (h - result) * keep
    return result.astype(np.float32), dist


# --- output --------------------------------------------------------------------------------


def preview(h, land, weights, roads, road_dist, cliff):
    gy, gx = np.gradient(h, SPACING)
    light = np.array([-0.6, -0.6, 0.55])
    light /= np.linalg.norm(light)
    n = np.stack([-gx, -gy, np.ones_like(h)])
    shade = np.clip((n * light[:, None, None]).sum(0) / np.linalg.norm(n, axis=0), 0, 1)
    rgb = np.zeros((*h.shape, 3), dtype=np.float32)
    for name, w in weights.items():
        rgb += w[..., None] * np.array(COLOURS[name], dtype=np.float32)
    snow = smoothstep(SNOW_LINE - 70, SNOW_LINE + 30, h)[..., None]
    rgb = rgb * (1 - snow) + 245 * snow
    beach = (land & (h < 5.5) & (cliff < 0.5))[..., None]
    rgb = np.where(beach, np.array(COLOURS["beach"], dtype=np.float32), rgb)
    rgb *= (0.35 + 0.8 * shade)[..., None]
    depth = smoothstep(0, -30, h)[..., None]
    sea = np.array(COLOURS["sea"], dtype=np.float32) * (1.2 - 0.6 * depth)
    rgb = np.where(land[..., None], rgb, sea)
    rgb = np.where((road_dist < max(ROAD_HALF, SPACING * 0.75))[..., None], 250.0, rgb)
    c = (np.arange(RES) - (RES - 1) / 2) * SPACING
    lines = np.abs(c - np.round(c / 1000) * 1000) < SPACING / 2
    rgb[lines, :, :] = rgb[lines, :, :] * 0.6 + 40
    rgb[:, lines, :] = rgb[:, lines, :] * 0.6 + 40
    Image.fromarray(np.clip(rgb, 0, 255).astype(np.uint8)).save(f"{OUT}/preview.png")


def profile(roads, ground):
    """Each road's height (white) over the ground before carving (grey), 1 px a 10 m."""
    rows = []
    for name, road in roads.items():
        z, g = road["z"], ground[name]
        w = math.ceil(len(z) / 10)
        img = np.zeros((260, w, 3), dtype=np.uint8)
        top = max(z.max(), g.max(), 50)
        for i in range(w):
            k = i * 10
            gz = int(250 - g[k] / top * 240)
            rz = int(250 - z[k] / top * 240)
            img[max(gz, 0) :, i] = (70, 70, 70)
            img[max(rz - 1, 0) : rz + 2, i] = (255, 255, 255)
        rows.append(img)
        climb = np.clip(np.diff(z), 0, None).sum()
        cut = (g - z).max()
        fill = (z - g).max()
        print(
            f"{name}: {len(z) / 1000:.1f} km, {z.min():.0f}..{z.max():.0f} m, climbs {climb:.0f} m,"
            f" deepest cut {cut:.0f} m, highest fill {fill:.0f} m"
        )
    width = max(r.shape[1] for r in rows)
    sheet = np.zeros((sum(r.shape[0] + 10 for r in rows), width, 3), dtype=np.uint8)
    y = 0
    for r in rows:
        sheet[y : y + r.shape[0], : r.shape[1]] = r
        y += r.shape[0] + 10
    Image.fromarray(sheet).save(f"{OUT}/profile.png")


def paint(x, y, h, land, inland, weights, cliff, road_dist):
    """Each paint layer's weight, 0..1, summing to 1 on land."""
    gy, gx = np.gradient(h, SPACING)
    slope = np.hypot(gx, gy)
    del gx, gy
    rock = smoothstep(0.55, 0.85, slope)
    rock = np.maximum(
        rock, weights["mountain"] * smoothstep(260, 420, h) * smoothstep(0.2, 0.45, slope)
    )
    line = SNOW_LINE + 60 * (fbm(x, y, 300, 3, SEED + 95) - 0.5)
    # Snow lies on all but cliffs: the peak is steep enough to be rock nearly everywhere.
    snow = smoothstep(line - 60, line + 60, h) * smoothstep(1.4, 0.9, slope)
    rock = rock * (1 - snow)
    soft = 1 - rock - snow
    sand = smoothstep(4.5, 2.5, h) * (1 - cliff) + (~land)
    width = 75 + 60 * fbm(x, y, 700, 3, SEED + 96)
    sand = np.clip(sand + smoothstep(width, width - 25, inland) * (1 - cliff) * (h < 12), 0, 1)
    soft_rest = soft * (1 - sand)
    # A gravel shoulder a metre or two wide: wider, it reads as a sand verge along every road.
    dirt = smoothstep(ROAD_HALF + 3.0, ROAD_HALF + 0.5, road_dist)
    out = {
        "Grass": weights["meadow"] + weights["town"] + weights["mountain"],
        "Forest": weights["forest"],
        "Desert": weights["desert"],
    }
    total = sum(out.values()) + 1e-6
    out = {k: v / total * soft_rest for k, v in out.items()}
    out["Sand"] = soft * sand
    out["Rock"] = rock
    out["Snow"] = snow
    out = {k: v * (1 - dirt) for k, v in out.items()}
    out["Dirt"] = dirt
    return {k: out[k] for k in LAYERS}


# Rock landmarks scattered where the riders see them: (Megascans biome/slug, how many, region,
# nearest and farthest from a road (m), scale range, the mesh's half-width (m) at scale 1).
LANDMARKS = (
    ("desert/desert_western_spire_layered_xl_04", 4, "desert", (60, 320), (1.0, 1.6), 14),
    ("desert/canyon_sandstone_hoodoo", 6, "desert", (45, 260), (0.9, 1.6), 10),
    ("desert/desert_western_ledge_rock_layered_xl_01", 4, "desert", (40, 240), (0.8, 1.3), 11),
    ("desert/huge_canyon_sandstone_mesa", 3, "desert", (50, 300), (1.0, 1.5), 8),
    ("desert/sandstone_rock_formation", 14, "desert", (12, 140), (1.2, 3.0), 2),
    ("mountain/massive_tundra_rock_formation", 5, "mountain", (50, 320), (0.8, 1.4), 13),
    ("mountain/sharp_cliff", 3, "mountain", (60, 320), (0.8, 1.2), 10),
    ("mountain/tundra_boulder", 16, "mountain", (10, 140), (1.0, 2.6), 1.5),
    ("forest/nordic_forest_cliff_large", 4, "forest", (30, 200), (0.9, 1.4), 7),
)


def landmarks(h, weights, road_dist, roads):
    """Where each landmark stands: in its region on gentle ground, clear of the roads by more
    than its own half-width and apart from the others, sunk to the lowest ground under it so it
    doesn't float on a slope. Plus the summit's viewpoint: a rail of barriers across the end of
    the climb with snow banked either side."""
    rng = np.random.default_rng(SEED + 200)
    gy, gx = np.gradient(h, SPACING)
    gentle = np.hypot(gx, gy) < 0.35
    del gx, gy
    placed, out = [], []

    def put(slug, xm, ym, zm, yaw, scale):
        out.append({"mesh": slug, "x": xm, "y": ym, "z": zm, "yaw": yaw, "scale": scale})

    def at(xm, ym):
        return round(ym / SPACING + (RES - 1) / 2), round(xm / SPACING + (RES - 1) / 2)

    for slug, count, region, (near, far), (lo, hi), half in LANDMARKS:
        clear = max(near, half * hi + ROAD_HALF + 4)
        rows, cols = np.nonzero(
            (weights[region] > 0.7) & gentle & (road_dist > clear) & (road_dist < far) & (h > 2)
        )
        found = 0
        for k in rng.permutation(len(rows))[:5000]:
            r, c = rows[k], cols[k]
            xm, ym = (c - (RES - 1) / 2) * SPACING, (r - (RES - 1) / 2) * SPACING
            gap = max(60.0, half * hi * 4)
            if any(math.hypot(xm - px, ym - py) < max(gap, pg) for px, py, pg in placed):
                continue
            scale = float(rng.uniform(lo, hi))
            reach = max(1, int(half * scale * 0.6 / SPACING))
            zm = float(h[r - reach : r + reach + 1, c - reach : c + reach + 1].min()) - 0.3 * scale
            put(slug, xm, ym, zm, float(rng.uniform(0, 360)), scale)
            placed.append((xm, ym, gap))
            found += 1
            if found == count:
                break
        print(f"landmark {slug}: {found} of {count}")

    # The viewpoint: past the top of the summit road, square across the way it arrives.
    xy, z = roads["summit"]["xy"], roads["summit"]["z"]
    top = int(np.argmax(z))
    back = xy[max(top - 15, 0)]
    ahead = xy[top] - back
    ahead = ahead / (np.linalg.norm(ahead) + 1e-6)
    side = np.array([-ahead[1], ahead[0]])
    facing = math.degrees(math.atan2(side[1], side[0]))
    for k in (-1.5, -0.5, 0.5, 1.5):  # four 3.2 m barriers in a row across the road's end
        p = xy[top] + ahead * (ROAD_HALF + 4) + side * k * 3.3
        put("urban/old_concrete_barrier", *map(float, p), float(h[at(*p)]) - 0.05, facing, 1.0)
    for k, slug in ((-1, "mountain/snow_pile_xgxhcjxqx"), (1, "mountain/snow_pile_xeslbfbqx")):
        p = xy[top] + side * k * (ROAD_HALF + 7)
        put(slug, *map(float, p), float(h[at(*p)]) - 0.4, facing, 1.3)
    print(f"viewpoint at {xy[top].round(0).tolist()}, {z[top]:.0f} m")
    return out


def write_maps(h, land, layers):
    q = np.clip(np.round(32768 + h * 100 * 128 / Z_SCALE), 0, 65535).astype("<u2")
    q.tofile(f"{OUT}/height.r16")
    total = sum(layers.values()) + 1e-6
    q8 = {k: np.round(v / total * 255).astype(np.uint8) for k, v in layers.items()}
    for name, v in q8.items():
        v.tofile(f"{OUT}/{name}.r8")
    names = list(q8)
    top = np.argmax(np.stack([q8[n] for n in names]), axis=0)
    colours = np.array([LAYERS[n] for n in names], dtype=np.uint8)
    sea = np.uint8([40, 80, 120])
    img = Image.fromarray(np.where(land[..., None], colours[top], sea))
    if RES > 2049:
        img = img.resize((2048, 2048), Image.Resampling.NEAREST)
    img.save(f"{OUT}/layers.png")
    with open(f"{OUT}/info.json", "w") as f:
        json.dump({"size": RES, "spacing": SPACING * 100, "z_scale": Z_SCALE, "layers": names}, f)


def main():
    os.makedirs(OUT, exist_ok=True)
    x, y = grid()
    h, land, inland, weights, cliff = terrain(x, y)
    roads, h = lay_roads(h)
    ground = {name: sample(h, road["xy"]) for name, road in roads.items()}
    h, road_dist = carve(h, roads)
    preview(h, land, weights, roads, road_dist, cliff)
    profile(roads, ground)
    write_maps(h, land, paint(x, y, h, land, inland, weights, cliff, road_dist))
    with open(f"{OUT}/landmarks.json", "w") as f:
        json.dump(landmarks(h, weights, road_dist, roads), f)
    print(f"{OUT}: {RES}x{RES} at {SPACING:.2f} m, heights {h.min():.0f}..{h.max():.0f} m")
    pieces = 0
    for name, r in roads.items():
        # The tarmac 2 cm under the centreline: at the road's height, 10 cm over its bed.
        centre = [
            (a * 100, b * 100, c * 100 + 2) for (a, b), c in zip(r["xy"], r["z"], strict=True)
        ]
        for k, piece in enumerate(road_mesh.road_pieces(centre, ROAD_HALF * 100, r["closed"])):
            piece.write(f"{OUT}/road_{name}_{k}.gltf")
            pieces += 1
    print(f"{pieces} road pieces")
    rides = stitch(roads)
    with open(f"{OUT}/routes.json", "w") as f:
        json.dump({name: line.round(2).tolist() for name, line in rides.items()}, f)
    with open(f"{OUT}/roads.json", "w") as f:
        json.dump(
            {
                name: {
                    "closed": r["closed"],
                    "centre": [
                        [float(a), float(b), float(c)]
                        for (a, b), c in zip(r["xy"], r["z"], strict=True)
                    ],
                }
                for name, r in roads.items()
            },
            f,
        )


if __name__ == "__main__":
    main()
