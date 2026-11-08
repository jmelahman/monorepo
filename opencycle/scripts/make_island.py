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
}
CLOSED = {"ring"}
ROAD_HALF = 3.2  # metres: two narrow lanes
SHOULDER = 3.0
BANK_SLOPE = 0.7  # rise per metre of the cut and fill banks either side
CORRIDOR = 140.0  # metres from a road beyond which the ground is left alone
MAX_GRADE = 0.08
Z_SCALE = 512.0  # the landscape's Z scale: +-1310 m
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
    """Heights with no stretch steeper than MAX_GRADE, by smoothing only where it is too steep:
    tops come down and hollows come up alike, so cuts and fills stay balanced. `fixed` maps
    indices to heights they must keep."""
    z = z.copy()
    for _ in range(3000):
        for i, v in fixed.items():
            z[i] = v
        g = np.diff(np.concatenate([z, z[:1]]) if closed else z)
        bad = np.abs(g) > MAX_GRADE * 0.98
        if not bad.any():
            break
        near = ndimage.binary_dilation(bad, iterations=60, border_value=0)
        near = near[: len(z)] if closed else np.concatenate([near, near[-1:]])
        z = np.where(near, smooth(z, 6, closed), z)
    for i, v in fixed.items():
        z[i] = v
    return z


def smooth(z, span, closed):
    k = np.ones(span * 2 + 1) / (span * 2 + 1)
    if closed:
        return np.convolve(np.concatenate([z[-span:], z, z[:span]]), k, "valid")
    padded = np.concatenate([np.full(span, z[0]), z, np.full(span, z[-1])])
    return np.convolve(padded, k, "valid")


def lay_roads(h):
    """Each road's centreline every metre with its height: the ground smoothed along it, kept off
    the sea and graded; later roads meet earlier ones at their height."""
    roads = {}
    for name, points in ROADS.items():
        closed = name in CLOSED
        xy = catmull(points, closed)
        z = np.maximum(smooth(sample(h, xy), 90, closed), 3.0)
        fixed = {}
        for end in (0, len(xy) - 1) if not closed else ():
            for other in roads.values():
                d = np.linalg.norm(other["xy"] - xy[end], axis=1)
                k = int(np.argmin(d))
                if d[k] < 30:
                    fixed[end] = other["z"][k]
        z = grade_fit(z, fixed, closed)
        roads[name] = {"xy": xy, "z": z, "closed": closed}
    return roads


def carve(h, roads):
    """The ground cut and filled to the roads: level across the road and its shoulders, then
    banks at BANK_SLOPE back up or down to the hills."""
    target = np.full(h.shape, np.nan, dtype=np.float32)
    for road in roads.values():
        col = np.round(road["xy"][:, 0] / SPACING + (RES - 1) / 2).astype(int)
        row = np.round(road["xy"][:, 1] / SPACING + (RES - 1) / 2).astype(int)
        ok = (col >= 0) & (col < RES) & (row >= 0) & (row < RES)
        target[row[ok], col[ok]] = road["z"][ok]
    on = ~np.isnan(target)
    dist, (ri, ci) = ndimage.distance_transform_edt(~on, return_indices=True)
    dist = (dist * SPACING).astype(np.float32)
    bed = target[ri, ci] - 0.1
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
    snow = smoothstep(650, 750, h)[..., None]
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
    line = 820 + 60 * (fbm(x, y, 300, 3, SEED + 95) - 0.5)
    snow = smoothstep(line - 60, line + 60, h) * (1 - rock * 0.8)
    rock = rock * (1 - snow)
    soft = 1 - rock - snow
    sand = smoothstep(4.5, 2.5, h) * (1 - cliff) + (~land)
    width = 75 + 60 * fbm(x, y, 700, 3, SEED + 96)
    sand = np.clip(sand + smoothstep(width, width - 25, inland) * (1 - cliff) * (h < 12), 0, 1)
    soft_rest = soft * (1 - sand)
    dirt = smoothstep(ROAD_HALF + SHOULDER + 2.5, ROAD_HALF + 0.5, road_dist)
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
    roads = lay_roads(h)
    ground = {name: sample(h, road["xy"]) for name, road in roads.items()}
    h, road_dist = carve(h, roads)
    preview(h, land, weights, roads, road_dist, cliff)
    profile(roads, ground)
    write_maps(h, land, paint(x, y, h, land, inland, weights, cliff, road_dist))
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
