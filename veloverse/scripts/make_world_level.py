"""The golden ride: the whole world plan as one level, with the ring road carved in.

make_world.py plans the world at 27.8 m a sample. This lays it out for Unreal at 14 m a sample
(4065 a side, 32 x 32 landscape components; OC_RES=8129 gives 7 m) with the ring, the 165 km
loop round the island, carved in as a bike path. Its output has make_climb.py's layout, so
climb_world.py and oc_climb.py build it as /Game/World/L_World:

    uv run -q --no-project --with numpy --with scipy --with pillow python -I \
        scripts/make_world_level.py
    OC_SRC=/workspace/ue/world_gen/ride OC_LEVEL=/game/World/L_World oc.py build-level ...

The seven paint layers mean what biomes.py's "world" preset says: Desert is the desert's sand,
Sand the beaches, the rest as on the alpine climb. Trees stand only in a corridor along the
ring: the woods past it are the ground's colour, seen from kilometres off.

Everything is done in bands of rows: a whole-map float array is 264 MB, and the editor may be
running beside this.
"""

import json
import math
import os
import sys

import numpy as np
from PIL import Image
from scipy import ndimage

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_island as mi
import make_world as mw
import road_mesh

PLAN = "/workspace/ue/world_gen/world"
OUT = os.environ.get("OC_RIDE_OUT", "/workspace/ue/world_gen/ride")
# Components of 127 quads, plus one. 8129 (7 m) lost the GPU on 11 Nov: the editor's
# ChangeGridSize on 64 x 64 components sat 8 minutes on one frame with 7.5 GB free, then
# VK_ERROR_DEVICE_LOST. 4065 (32 x 32) built on 9 Nov, its grid step leaving 4.2 GB free.
RES = int(os.environ.get("OC_RES", "4065"))
SPACING = mw.EXTENT / (RES - 1)  # 14.0 m at 4065
# The ground flat beside the road, and the bare verge (Dirt: no grass) on it. At 14 m a sample
# the climb's 3 m shoulder and 0.9 m verge fall between samples, so the bank rose over the road
# and the Grass layer's blades grew through it (11 Nov). Both take at least a sample's width.
FLAT = max(mi.SHOULDER, SPACING * 1.1)
VERGE = max(0.9, SPACING)
# The island's layers and Verge, the bare sand beside the road in the desert and on the beaches:
# a layer of its own because Desert and Sand grow plants (paint_band).
LAYERS = {**mi.LAYERS, "Verge": (215, 180, 130)}
REACH = math.ceil(SPACING * 1.5)  # road points either side of a cell's one, for carve()
HALF = (RES - 1) / 2
BAND = 512  # rows per band
SEED = 41
TREELINE = 820.0
SNOWLINE = 900.0
WOODS = 160.0  # m from the ring: trees stand this far out
TREE_STEP = 7.0  # m between candidate stems
PIECE = 1000  # road points (1 m apart) per road mesh
MIN_STRETCH = 3000.0  # m: a biome's run shorter than this joins its neighbour
NAMES = {
    "coast": "Coast",
    "desert": "Desert",
    "forest": "Forest",
    "mountain": "Mountain",
    "meadow": "Meadow",
    "town": "Town",
    "heath": "Heath",
}


def coords(idx):
    return ((np.asarray(idx, dtype=np.float64) - HALF) * SPACING).astype(np.float32)


class Plan:
    """The world plan's heights and what goes with them, sampled anywhere in world metres."""

    def __init__(self):
        self.h = np.load(f"{PLAN}/height.npy")
        self.res = self.h.shape[0]
        self.spacing = mw.EXTENT / (self.res - 1)
        land = self.h > 0
        self.inland = (ndimage.distance_transform_edt(land) * self.spacing).astype(np.float32)
        gy, gx = np.gradient(self.h, self.spacing)
        self.slope = np.hypot(gx, gy).astype(np.float32)

    def at(self, a, x, y, order=1):
        col = x / self.spacing + (self.res - 1) / 2
        row = y / self.spacing + (self.res - 1) / 2
        return ndimage.map_coordinates(a, [row, col], order=order, mode="nearest").astype(
            np.float32
        )


def weights(x, y):
    mi.REGIONS = mw.REGIONS
    return mi.region_weights(x / mw.K, y / mw.K)


def detail(x, y, base, slope):
    """Relief the 28 m plan can't hold: gullies, swells and crags on the steep and high
    ground. Nothing finer than the landscape's grid."""
    steep = np.clip(mi.smoothstep(0.15, 0.5, slope) + mi.smoothstep(850, 1050, base), 0, 1)
    gully = (mi.fbm(x * 1.0, y * 0.35, 220, 4, SEED) - 0.5) * 14
    swell = (mi.fbm(x, y, 520, 3, SEED + 7) - 0.5) * 18
    crag = (mi.ridged(x, y, 120, 4, SEED + 13) - 0.35) * 12 * steep
    fine = (mi.fbm(x, y, 45, 3, SEED + 21) - 0.5) * 3
    land = mi.smoothstep(2.0, 12.0, base)
    return (gully + swell + crag + fine) * land


def ground(plan):
    """Heights at SPACING: the plan bicubic plus detail, band by band."""
    h = np.empty((RES, RES), dtype=np.float32)
    cols = np.arange(RES)
    for r0 in range(0, RES, BAND):
        rows = np.arange(r0, min(r0 + BAND, RES))
        y, x = np.meshgrid(coords(rows), coords(cols), indexing="ij")
        base = plan.at(plan.h, x, y, order=3)
        slope = plan.at(plan.slope, x, y)
        h[rows] = base + detail(x, y, base, slope)
    return h


def ring(plan, h):
    """The ring at 1 m: the plan's line, its heights moved with the detail under it (smoothed
    over 400 m, so the grades stay the plan's)."""
    with open(f"{PLAN}/roads.json") as f:
        pts = np.asarray(json.load(f)["ring"], dtype=np.float64)
    xy, z = pts[:, :2], pts[:, 2]
    fine = ndimage.map_coordinates(
        h, [xy[:, 1] / SPACING + HALF, xy[:, 0] / SPACING + HALF], order=1, mode="nearest"
    )
    coarse = plan.at(plan.h, xy[:, 0], xy[:, 1], order=3)
    shift = ndimage.uniform_filter1d(fine - coarse, 41, mode="wrap")  # 10 m points: 400 m
    z = np.maximum(z + shift, 3.0)
    # Every metre, closed: the last point runs back to the first.
    loop = np.vstack([np.column_stack([xy, z]), [[*xy[0], z[0]]]])
    seg = np.linalg.norm(np.diff(loop[:, :2], axis=0), axis=1)
    s = np.concatenate([[0], np.cumsum(seg)])
    t = np.arange(0, s[-1] - 0.5, 1.0)
    line = np.column_stack([np.interp(t, s, loop[:, k]) for k in range(3)])
    # Ease the 10 m points' corners and the grade over 30 m.
    for k in range(3):
        line[:, k] = ndimage.uniform_filter1d(line[:, k], 31, mode="wrap")
    return line


def carve(h, line):
    """make_island.carve in tiles: the ground cut and filled to the ring. Returns the distance
    (m) to the ring for each sample, capped at 1000."""
    xy, z = line[:, :2], line[:, 2]
    n = len(xy)
    after = np.roll(np.arange(n), -1)
    col = np.round(xy[:, 0] / SPACING + HALF).astype(int)
    row = np.round(xy[:, 1] / SPACING + HALF).astype(int)
    dist_all = np.full((RES, RES), 1000.0, dtype=np.float32)
    tile, margin = 1024, int(WOODS / SPACING) + 8
    for r0 in range(0, RES, tile):
        for c0 in range(0, RES, tile):
            ra, rb = max(r0 - margin, 0), min(r0 + tile + margin, RES)
            ca, cb = max(c0 - margin, 0), min(c0 + tile + margin, RES)
            inside = np.flatnonzero((row >= ra) & (row < rb) & (col >= ca) & (col < cb))
            if not len(inside):
                continue
            target = np.full((rb - ra, cb - ca), -1, dtype=np.int64)
            target[row[inside] - ra, col[inside] - ca] = inside
            dist, (ri, ci) = ndimage.distance_transform_edt(target < 0, return_indices=True)
            dist = (dist * SPACING).astype(np.float32)
            nearest = target[ri, ci]
            py, px = np.meshgrid(
                coords(np.arange(ra, rb)), coords(np.arange(ca, cb)), indexing="ij"
            )
            p = np.stack([px, py], axis=-1).astype(np.float64)

            def along(a, b, p=p):
                d = xy[b] - xy[a]
                length2 = np.maximum((d * d).sum(-1), 1e-9)
                t = np.clip(((p - xy[a]) * d).sum(-1) / length2, 0, 1)
                off = p - (xy[a] + d * t[..., None])
                return (off * off).sum(-1), z[a] + (z[b] - z[a]) * t

            # The road's nearest grid cell holds one of the ~14 metre points through it, often
            # not the one nearest the sample: checking only that point's two segments measured
            # samples 10 m out as 20 (11 Nov), so the verge stopped short and grass grew through
            # the tarmac. Every segment within a cell and a half of it is checked.
            gap, bed = np.full(nearest.shape, np.inf), np.zeros(nearest.shape)
            for k in range(-REACH, REACH):
                a = (nearest + k) % n
                g, zz = along(a, after[a])
                closer = g < gap
                gap, bed = np.where(closer, g, gap), np.where(closer, zz, bed)
            bed = (bed - 0.15).astype(np.float32)
            dist = np.sqrt(gap).astype(np.float32)
            sub = h[ra:rb, ca:cb]
            out = np.maximum(dist - mi.ROAD_HALF - FLAT, 0) * mi.BANK_SLOPE
            capped = np.maximum(np.minimum(sub, bed + out), bed - out)
            result = bed + (capped - bed) * mi.smoothstep(0, 4, out)
            keep = mi.smoothstep(mi.CORRIDOR * 0.6, mi.CORRIDOR, dist)
            result = result + (sub - result) * keep
            # Only the tile's own samples: the margin belongs to its neighbours.
            ir = slice(r0 - ra, r0 - ra + min(tile, RES - r0))
            ic = slice(c0 - ca, c0 - ca + min(tile, RES - c0))
            h[r0 : r0 + tile, c0 : c0 + tile] = result[ir, ic]
            dist_all[r0 : r0 + tile, c0 : c0 + tile] = np.minimum(dist[ir, ic], 1000.0)
    return dist_all


def clear_road(h, line, below=0.1, rounds=8):
    """Lowers the ground wherever it would show through the road: carve() sets each sample to
    its nearest road point's height, but on bends the bilinear ground between 14 m samples rose
    up to 0.8 m over the tarmac (11 Nov, 5% of the ring). Samples the road's centre and edges
    every metre and drops each offending cell's corners until the ground is `below` m under."""
    xy, z = line[:, :2], line[:, 2]
    d = np.roll(xy, -1, axis=0) - np.roll(xy, 1, axis=0)
    side = np.stack([-d[:, 1], d[:, 0]], axis=-1) / np.maximum(np.hypot(*d.T), 1e-9)[:, None]
    offsets = (-mi.ROAD_HALF - 0.5, -mi.ROAD_HALF / 2, 0.0, mi.ROAD_HALF / 2, mi.ROAD_HALF + 0.5)
    p = np.concatenate([xy + side * o for o in offsets])
    top = np.tile(z, len(offsets)) - below
    c, r = p[:, 0] / SPACING + HALF, p[:, 1] / SPACING + HALF
    r0, c0 = np.floor(r).astype(int), np.floor(c).astype(int)
    for _ in range(rounds):
        over = ndimage.map_coordinates(h, [r, c], order=1) - top
        k = np.flatnonzero(over > 0.005)
        if not len(k):
            break
        for dr in (0, 1):
            for dc in (0, 1):
                rr, cc = r0[k] + dr, c0[k] + dc
                np.minimum.at(h, (rr, cc), h[rr, cc] - over[k] - 0.01)
    over = ndimage.map_coordinates(h, [r, c], order=1) - top - below
    print(f"road cleared: ground at most {over.max():.2f} m over the tarmac", flush=True)


def paint_band(x, y, h, slope, d, w, inland):
    """Each layer's weight, 0..1, for one band."""
    n = lambda s, seed, octaves=3: mi.fbm(x, y, s, octaves, SEED + seed)  # noqa: E731
    shoulder = mi.smoothstep(mi.ROAD_HALF + 0.9, mi.ROAD_HALF + 0.2, d)
    rock = mi.smoothstep(0.75, 1.05, slope) * mi.smoothstep(10.0, 25.0, d)
    high = w["mountain"] + w["heath"]
    rock = np.maximum(rock, high * mi.smoothstep(0.45, 0.7, slope) * mi.smoothstep(15, 30, d))
    snow = mi.smoothstep(SNOWLINE - 40, SNOWLINE + 40, h + 90 * (n(70, 1) - 0.5))
    snow *= 1 - mi.smoothstep(0.7, 1.1, slope)
    rock *= 1 - snow
    beach_top = 5.5 + 2.5 * n(300, 2)
    beach = mi.smoothstep(beach_top + 1.5, beach_top - 1.0, h) * mi.smoothstep(900, 500, inland)
    beach = np.maximum(beach, h < 0.5)
    desert = w["desert"]
    # The verge reaches a whole sample past the road, as any sample nearer feeds the road's grass
    # (11 Nov: at 85% Dirt the Grass layer's last 15% still grew tufts through the tarmac). In
    # the green stretches it is Dirt, turf with no blades (biomes "world"); in the desert and on
    # the beaches it is Verge, bare sand: Desert and Sand grow shrubs, cacti, boulders and marram,
    # and with the narrow verge they kept, those stood on the tarmac.
    verge = np.maximum(shoulder, mi.smoothstep(mi.ROAD_HALF + VERGE * 1.6, mi.ROAD_HALF + VERGE, d))
    dry = np.clip(beach + desert, 0, 1)
    shoulder = verge
    # Woods: the forest's and the mountain's below the treeline, copses in the meadows, with
    # clearings; open grass in the meadows, the town, the heath and above the trees.
    alpine = mi.smoothstep(TREELINE - 30, TREELINE + 15, h + 40 * (n(90, 3) - 0.5))
    clearing = mi.smoothstep(0.62, 0.72, n(160, 4))
    copse = mi.smoothstep(0.6, 0.68, n(240, 5))
    wooded = (w["forest"] + w["mountain"]) * (1 - clearing) + w["meadow"] * copse
    wooded = wooded + w["heath"] * mi.smoothstep(0.66, 0.72, n(200, 6))
    wooded = np.clip(wooded, 0, 1) * (1 - alpine)
    soft = (1 - rock) * (1 - snow) * (1 - shoulder)
    out = {
        "Sand": soft * beach,
        "Desert": soft * (1 - beach) * desert,
    }
    green = soft * (1 - beach) * (1 - desert)
    out["Forest"] = green * wooded
    out["Grass"] = green * (1 - wooded)
    out["Rock"] = rock * (1 - shoulder)
    out["Snow"] = snow * (1 - shoulder)
    out["Dirt"] = shoulder * (1 - dry)
    out["Verge"] = shoulder * dry
    return {k: out[k].astype(np.float32) for k in LAYERS}


def trees_band(rng, x, y, h, slope, d, forest, desert):
    """Candidate stems every TREE_STEP m near the ring, kept where the band is woods."""
    near = d < WOODS
    if not near.any():
        return []
    stride = max(1, round(TREE_STEP / SPACING))
    pick = np.zeros_like(near)
    pick[::stride, ::stride] = True
    rows, cols = np.nonzero(near & pick)
    jitter = rng.normal(0, 0.45, (2, len(rows))) * TREE_STEP
    px, py = x[rows, cols] + jitter[0], y[rows, cols] + jitter[1]
    dd, zz, ww = d[rows, cols], h[rows, cols], forest[rows, cols]
    clump = mi.smoothstep(0.3, 0.62, mi.fbm(px, py, 60, 3, SEED + 70))
    wood = ww * np.clip(0.15 + 1.1 * clump, 0, 1) * (1 - desert[rows, cols])
    far = np.where(dd < 50, 1.0, 0.3)
    keep = (
        (dd > mi.ROAD_HALF + 2.5)
        & (rng.random(len(dd)) < mi.smoothstep(mi.ROAD_HALF + 2.5, mi.ROAD_HALF + 9, dd))
        & (slope[rows, cols] < 1.3)
        & (rng.random(len(dd)) < wood * 0.9 * far)
        & (rng.random(len(dd)) < mi.smoothstep(TREELINE + 45, TREELINE - 20, zz))
        & (zz > 6)
    )
    px, py, zz = px[keep], py[keep], zz[keep] - 0.4
    line = 260 + 120 * (mi.fbm(px, py, 900, 3, SEED + 60) - 0.5)
    spruce = rng.random(len(zz)) < mi.smoothstep(line - 80, line + 80, zz)
    stunt = 1 - 0.5 * mi.smoothstep(TREELINE - 30, TREELINE + 20, zz)
    out = []
    for xm, ym, zm, s, small in zip(px, py, zz, spruce, stunt, strict=True):
        roll = rng.random()
        size = (
            rng.uniform(1.25, 1.5)
            if roll < 0.08
            else rng.uniform(0.55, 0.8)
            if roll < 0.35
            else rng.uniform(0.85, 1.2)
        )
        out.append(
            [
                "spruce" if s else "beech",
                round(float(xm) * 100, 1),
                round(float(ym) * 100, 1),
                round(float(zm) * 100, 1),
                round(float(rng.uniform(0, 360)), 1),
                round(float(size * small), 3),
            ]
        )
    return out


def paint_and_trees(plan, h, dist):
    """Writes each layer's .r8 band by band; returns the trees and the preview's colours."""
    rng = np.random.default_rng(SEED + 100)
    for k in LAYERS:
        open(f"{OUT}/{k}.r8", "wb").close()
    names = list(LAYERS)
    colours = np.array([LAYERS[k] for k in names], dtype=np.uint8)
    small = []
    stems = []
    cols = np.arange(RES)
    for r0 in range(0, RES, BAND):
        r1 = min(r0 + BAND, RES)
        a, b = max(r0 - 1, 0), min(r1 + 1, RES)  # a row either side, for the slope
        gy, gx = np.gradient(h[a:b], SPACING)
        slope = np.hypot(gx, gy)[r0 - a : r0 - a + (r1 - r0)]
        y, x = np.meshgrid(coords(np.arange(r0, r1)), coords(cols), indexing="ij")
        hb, db = h[r0:r1], dist[r0:r1]
        w = weights(x, y)
        inland = plan.at(plan.inland, x, y)
        layers = paint_band(x, y, hb, slope, db, w, inland)
        total = sum(layers.values()) + 1e-6
        q8 = {k: np.round(v / total * 255).astype(np.uint8) for k, v in layers.items()}
        for k in names:
            with open(f"{OUT}/{k}.r8", "ab") as f:
                q8[k].tofile(f)
        top = np.argmax(np.stack([q8[k] for k in names]), axis=0)
        img = np.where((hb > 0)[..., None], colours[top], np.uint8([40, 80, 120]))
        small.append(img[::4, ::4])
        stems += trees_band(rng, x, y, hb, slope, db, layers["Forest"], w["desert"])
        print(f"band {r0 // BAND + 1}/{math.ceil(RES / BAND)}: {len(stems)} trees", flush=True)
    return stems, np.vstack(small)


def stretches(plan, line):
    """The ring's biomes in order from its start: [(name, metres)], short runs merged."""
    x, y = line[::50, 0], line[::50, 1]
    w = weights(x, y)
    inland = plan.at(plan.inland, x, y)
    keys = list(w)
    best = np.array(keys)[np.argmax(np.stack([w[k] for k in keys]), axis=0)]
    best = np.where(inland < 1200, "coast", best)
    runs = []
    for k in best:
        if runs and runs[-1][0] == k:
            runs[-1][1] += 50.0
        else:
            runs.append([str(k), 50.0])
    while len(runs) > 1:
        short = min(range(len(runs)), key=lambda i: runs[i][1])
        if runs[short][1] >= MIN_STRETCH:
            break
        # Into the longer neighbour.
        prev, nxt = (short - 1) % len(runs), (short + 1) % len(runs)
        into = prev if runs[prev][1] >= runs[nxt][1] else nxt
        runs[into][1] += runs[short][1]
        del runs[short]
        merged = []
        for r in runs:
            if merged and merged[-1][0] == r[0]:
                merged[-1][1] += r[1]
            else:
                merged.append(r)
        runs = merged
    return [(NAMES.get(k, k.title()), round(m)) for k, m in runs]


def views(line, runs):
    """Stills along the ride, [name, eye, target] in m: a rider's eye halfway along each
    stretch looking 30 m up the road, and one from 120 m up over the first mountain stretch."""
    out, start = [], 0.0
    for i, (name, metres) in enumerate(runs):
        k = int(start + metres / 2) % len(line)
        ahead = line[(k + 30) % len(line)]
        out.append(
            [
                f"ride_{i:02d}_{name.lower()}",
                [float(line[k, 0]), float(line[k, 1]), float(line[k, 2] + 1.6)],
                [float(ahead[0]), float(ahead[1]), float(ahead[2] + 1.2)],
            ]
        )
        if name == "Mountain" and not any(v[0] == "above" for v in out):
            far = line[(k + 400) % len(line)]
            back = line[(k - 200) % len(line)]
            out.append(
                [
                    "above",
                    [float(back[0]), float(back[1]), float(back[2] + 120)],
                    [float(far[0]), float(far[1]), float(far[2])],
                ]
            )
        start += metres
    return out


def main():
    os.makedirs(OUT, exist_ok=True)
    plan = Plan()
    h = ground(plan)
    print(f"ground {h.min():.0f}..{h.max():.0f} m", flush=True)
    line = ring(plan, h)
    dist = carve(h, line)
    clear_road(h, line)
    print(f"ring {len(line) / 1000:.1f} km, carved", flush=True)
    stems, small = paint_and_trees(plan, h, dist)
    del dist
    q = np.clip(np.round(32768 + h * 100 * 128 / mi.Z_SCALE), 0, 65535).astype("<u2")
    q.tofile(f"{OUT}/height.r16")
    del q
    Image.fromarray(small).save(f"{OUT}/layers.png")
    runs = stretches(plan, line)
    info = {
        "size": RES,
        "spacing": SPACING * 100,
        "z_scale": mi.Z_SCALE,
        "layers": list(LAYERS),
        "biome": "world",
        "window": None,
        "sea_cm": 0.0,
        "road_half": mi.ROAD_HALF,  # m: the road material's edge (climb_world.road_instance)
        "ride": {
            "title": "The Ring",
            "blurb": "Once round the island: coast, desert, forest, mountain and meadow.",
            "closed": True,
            "stretches": runs,
            # Rides start on the coast past the harbour, not in the town's bare ground.
            "start": "Coast",
        },
        "views": views(line, runs),
    }
    with open(f"{OUT}/info.json", "w") as f:
        json.dump(info, f)
    for name in os.listdir(OUT):
        if name.startswith(("road_", "far_")) and name.endswith(".gltf"):
            os.remove(f"{OUT}/{name}")
    centre = [(a * 100, b * 100, c * 100 + 2) for a, b, c in line]
    for k, piece in enumerate(road_mesh.road_pieces(centre, mi.ROAD_HALF * 100, True, PIECE)):
        piece.write(f"{OUT}/road_ring_{k:03d}.gltf")
    with open(f"{OUT}/routes.json", "w") as f:
        json.dump({"island": line.round(2).tolist()}, f)
    with open(f"{OUT}/trees.json", "w") as f:
        json.dump(stems, f)
    for name in ("rails", "props"):
        with open(f"{OUT}/{name}.json", "w") as f:
            json.dump([], f)
    grade = np.diff(line[:, 2])
    spruce = sum(t[0] == "spruce" for t in stems)
    print(
        f"{OUT}: ground {h.min():.0f}..{h.max():.0f} m; ring {len(line) / 1000:.1f} km,"
        f" {line[:, 2].min():.0f}..{line[:, 2].max():.0f} m, steepest"
        f" {np.abs(ndimage.uniform_filter1d(grade, 100, mode='wrap')).max() * 100:.1f}% over 100 m;"
        f" {len(stems)} trees ({spruce} spruce); stretches {runs}"
    )


if __name__ == "__main__":
    main()
