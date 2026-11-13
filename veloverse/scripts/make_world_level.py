"""The golden ride: the whole world plan as one level, with its roads carved in.

The plan (OC_PLAN) is either make_chain.py's hub and islands, the default since 12 Nov, or
make_world.py's single island (OC_PLAN=/workspace/ue/world_gen/world, the 165 km ring). A chain
plan has a world.json, which gives its size, and its own biome weights; the single island's
come from make_world. This lays the plan out for Unreal at 4065 samples a side (32 x 32
landscape components): 7 m a sample for the 28 km chain, 14 m for the 57 km island. The plan's
roads are carved in as a bike path, except where they cross the sea on a bridge: the fewest of its
routes that between them ride every road (covering()), the others riding the same roads.
The output has make_climb.py's layout, so climb_world.py and oc_climb.py build it as
/Game/World/L_World:

    uv run -q --no-project --with numpy --with scipy --with pillow python -I \
        scripts/make_world_level.py
    OC_SRC=/workspace/ue/world_gen/ride_hub OC_LEVEL=/Game/World/L_World oc.py build-level ...

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
from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage
from scipy.spatial import cKDTree

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_island as mi
import make_world as mw
import road_mesh

# A two-lane bike path, 3.5 m across (11 Nov: the island's 6.4 m read as a road for cars). Set
# before anything reads it: carve, clear_road, the verge and the trees all keep clear of it.
mi.ROAD_HALF = 1.75
# The remote country's dirt path, m from the middle: narrower than the asphalt, its pieces cut to
# it (an opaque edge; a masked one crashes the shader compiler on Nanite).
PATH_HALF = 1.2
# m right of the centre line the rider and the stills ride: the middle of the asphalt's right
# lane (ROAD_HALF / 2), 35 cm inside the dirt path's edge.
RIDE_LANE = 0.85

PLAN = os.environ.get("OC_PLAN", "/workspace/ue/world_gen/hub")
# make_chain.py's world.json, or None for make_world.py's single island.
_WORLD = Path(PLAN, "world.json")
CHAIN = json.loads(_WORLD.read_text()) if _WORLD.exists() else None
OUT = os.environ.get("OC_RIDE_OUT", "/workspace/ue/world_gen/" + ("ride_hub" if CHAIN else "ride"))
# Components of 127 quads, plus one. 8129 (7 m) lost the GPU on 11 Nov: the editor's
# ChangeGridSize on 64 x 64 components sat 8 minutes on one frame with 7.5 GB free, then
# VK_ERROR_DEVICE_LOST. 4065 (32 x 32) built on 9 Nov, its grid step leaving 4.2 GB free.
RES = int(os.environ.get("OC_RES", "4065"))
EXTENT = CHAIN["extent_m"] if CHAIN else mw.EXTENT
SPACING = EXTENT / (RES - 1)  # 7 m for the chain, 14 for the island
# The ground flat beside the road. At 14 m a sample the climb's 3 m shoulder fell between
# samples, so the bank rose over the road (11 Nov): it takes at least a sample's width.
FLAT = max(mi.SHOULDER, SPACING * 1.1)
# The island's layers and Verge, the bare sand beside the road in the desert and on the beaches:
# a layer of its own because Desert and Sand grow plants (paint_band).
LAYERS = {**mi.LAYERS, "Verge": (215, 180, 130)}
# The road's earth bank, cm down (road_mesh.section): clear_road sinks whole 14 m cells, so
# beside the road the ground can sit tens of cm low, and a bare slab edge cast a dark band.
BANK = 80.0
REACH = math.ceil(SPACING * 1.5)  # road points either side of a cell's one, for carve()
HALF = (RES - 1) / 2
BAND = 512  # rows per band
SEED = 41
# Vael tops out at 1150 m since 13 Nov (760 before), the old island's 900 m-odd: one treeline.
TREELINE = 820.0
SNOWLINE = 900.0
WOODS = 160.0  # m from the ring: trees stand this far out
# Past WOODS, to FAR_WOODS, a sparse far forest every FAR_STEP m (trees_far.json): it casts no
# shadow and is only seen from a distance, so the hills to the horizon aren't bare.
FAR_WOODS, FAR_STEP, FAR_MAX = 3500.0, 42.0, 150000
TREE_STEP = 7.0  # m between candidate stems
PIECE = 1000  # road points (1 m apart) per road mesh
MIN_STRETCH = 3000.0  # m: a biome's run shorter than this joins its neighbour
# A route adds to the carved and meshed road if more than COVER_GAP of its metres lie over
# COVER_FAR m from the routes already taken (covering()).
COVER_FAR, COVER_GAP = 6.0, 200
# The road only a later route covers sits this far (cm) under the first's, so where the two meet
# at a junction the first's tarmac shows, not a flicker of both.
UNDER = 3.0
# The road's surface by stretch: asphalt near people, a compacted dirt path in the remote
# country (the user, 11 Nov: "it should stay a path in the more remote regions").
REMOTE = {"Mountain", "Heath", "Desert"}
# On the chain a stretch is named for its island, with the harbour town and its farmland apart.
CHAIN_NAMES = {
    "town": "Harbour",
    "meadow": "The Coast",
    "forest": "Greenwood",
    "heath": "The Moor",
    "mountain": "Vael",
    "desert": "Red Sands",
}
if CHAIN:
    REMOTE = {"The Moor", "Vael", "Red Sands"}
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
        self.spacing = EXTENT / (self.res - 1)
        land = self.h > 0
        self.inland = (ndimage.distance_transform_edt(land) * self.spacing).astype(np.float32)
        gy, gx = np.gradient(self.h, self.spacing)
        self.slope = np.hypot(gx, gy).astype(np.float32)
        self.w = None
        # The harbour, m (make_chain's landmark); the old single island's town is its centre.
        self.harbour = (0.0, 0.0)
        if os.path.exists(f"{PLAN}/world.json"):
            with open(f"{PLAN}/world.json") as f:
                self.harbour = tuple(json.load(f).get("landmarks", {}).get("harbour", (0, 0)))
        if CHAIN:
            with np.load(f"{PLAN}/weights.npz") as f:
                self.w = {k: f[k].astype(np.float32) for k in f.files}

    def at(self, a, x, y, order=1):
        col = x / self.spacing + (self.res - 1) / 2
        row = y / self.spacing + (self.res - 1) / 2
        return ndimage.map_coordinates(a, [row, col], order=order, mode="nearest").astype(
            np.float32
        )


def weights(plan, x, y):
    """Each biome's share of the ground at (x, y) m."""
    if plan.w:
        return {k: plan.at(v, x, y) for k, v in plan.w.items()}
    mi.REGIONS = mw.REGIONS
    return mi.region_weights(x / mw.K, y / mw.K)


# Rock benches (13 Nov, the AAA review: "terraces steep ground into benches so rock reads as
# strata"): STRATA m between ledges, on ground steeper than STEEP_ROCK in rocky country.
STRATA = 24.0
STEEP_ROCK = (0.55, 1.0)


def benches(x, y, base, slope, rocky):
    """How far (m, at most 0) to lower the ground so steep rocky slopes step down in ledges: a
    flat-ish bench, then a riser, the layers tilted by a broad noise so they aren't contours."""
    tilt = (mi.fbm(x, y, 900, 2, SEED + 31) - 0.5) * 2 * STRATA * 1.5
    hb = base + tilt
    f = hb / STRATA - np.floor(hb / STRATA)
    step = (mi.smoothstep(0.55, 0.95, f) - f) * STRATA  # -0.55..0 strata: the bench cut back
    where = mi.smoothstep(*STEEP_ROCK, slope) * rocky
    return np.minimum(step, 0.0) * 0.75 * where


def detail(x, y, base, slope):
    """Relief the 28 m plan can't hold: gullies, swells and crags on the steep and high
    ground. Nothing finer than the landscape's grid."""
    steep = np.clip(
        mi.smoothstep(0.15, 0.5, slope) + mi.smoothstep(TREELINE + 30, TREELINE + 230, base), 0, 1
    )
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
        if plan.w:
            rocky = plan.at(plan.w["mountain"], x, y) + plan.at(plan.w["desert"], x, y)
            rocky = rocky + 0.6 * plan.at(plan.w["heath"], x, y)
            h[rows] += benches(x, y, base, slope, np.clip(rocky, 0, 1))
    return h


def plan_routes():
    """The plan's routes, the level's default first: [{"key", "title", "blurb", "xyz" (every 10
    m), "bridges" ((first, last) indices into xyz)}]. make_chain.py writes them; make_world.py's
    single island has one, its "ring"."""
    with open(f"{PLAN}/roads.json") as f:
        roads = json.load(f)
    if "routes" in roads:
        return roads["routes"]
    return [
        {
            "key": "island",
            "title": "The Ring",
            "blurb": "Once round the island: coast, desert, forest, mountain and meadow.",
            "xyz": roads["ring"],
            "bridges": [],
        }
    ]


def ring(plan, h, route):
    """A route at 1 m, closed: the plan's line, its heights moved with the detail under it
    (smoothed over 400 m, so the grades stay the plan's). Also returns which of its points are on
    a bridge."""
    pts = np.asarray(route["xyz"], dtype=np.float64)
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
    on = np.zeros(len(loop))
    for a, b in route["bridges"]:
        on[a : b + 1] = 1.0
    bridge = np.interp(t, s, on) > 0.5
    print(
        f"{route['key']}: {len(line) / 1000:.1f} km, bridges {bridge.sum() / 1000:.1f} km",
        flush=True,
    )
    return line, bridge


def covering(lines):
    """The fewest routes whose roads cover every route's: each route in turn, if more than
    COVER_GAP of its points are over COVER_FAR from those already taken. On the hub plan that's
    the Grand Tour and the coast road."""
    taken = []
    for key, (line, _) in lines.items():
        if not taken:
            taken.append(key)
            continue
        tree = cKDTree(np.vstack([lines[k][0][:, :2] for k in taken]))
        d, _ = tree.query(line[:, :2])
        if (d > COVER_FAR).sum() > COVER_GAP:
            taken.append(key)
    return taken


def extra_runs(line, others, far=1.5):
    """[(first, end)] runs of a closed line's points more than `far` m from the `others`' points:
    the road only it covers, to be meshed on its own."""
    d, _ = cKDTree(others[:, :2]).query(line[:, :2])
    away = d > far
    if away.all():
        return [(0, len(line))]
    # Start at a covered point, so no run wraps.
    roll = int(np.argmin(away))
    out, start = [], None
    for k in range(roll, roll + len(line) + 1):
        on = away[k % len(line)]
        if on and start is None:
            start = k
        elif not on and start is not None:
            out.append((start, k))
            start = None
    return out


def carve(h, line, bridge):
    """make_island.carve in tiles: the ground cut and filled to the ring, but for the sea under
    a bridge (`bridge`: per point), which keeps its depth. Returns the distance (m) to the ring
    for each sample, capped at 1000."""
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
            pick = nearest
            for k in range(-REACH, REACH):
                a = (nearest + k) % n
                g, zz = along(a, after[a])
                closer = g < gap
                gap, bed = np.where(closer, g, gap), np.where(closer, zz, bed)
                pick = np.where(closer, a, pick)
            bed = (bed - 0.15).astype(np.float32)
            dist = np.sqrt(gap).astype(np.float32)
            sub = h[ra:rb, ca:cb]
            out = np.maximum(dist - mi.ROAD_HALF - FLAT, 0) * mi.BANK_SLOPE
            capped = np.maximum(np.minimum(sub, bed + out), bed - out)
            result = bed + (capped - bed) * mi.smoothstep(0, 4, out)
            keep = mi.smoothstep(mi.CORRIDOR * 0.6, mi.CORRIDOR, dist)
            result = result + (sub - result) * keep
            # No causeway: a bridge's sea stays sea. Its ends ashore are carved like the road.
            result = np.where(bridge[pick] & (sub < 0.5), sub, result)
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


# The road distance map (road_dist.png, read by oc_climb._road_hide): each sample's offset to the
# nearest road centre within DIST_NEAR m, which the plants' vertex shader reads at each plant's
# base to drop any plant standing on the tarmac. The grass map has a sample (7 m) per texel, so
# the paint alone can't stop grass at the road's edge: a verge wide enough to keep blades off the
# tarmac left 14..21 m of bare turf beside the road (12 Nov stills). Offsets, not distances,
# because they interpolate linearly across the road, so the bilinear sample is right to a few cm.
# They are stored premultiplied by a near flag (B), so a sample past DIST_NEAR doesn't drag its
# near neighbours' offsets: (R, G) = (flag * offset / DIST_NEAR + 1) / 2.
DIST_NEAR = 16.0
DIST_SIZE = 4096  # the texture, RES padded to a power of two; the padding is far


def road_distance(lines):
    """Writes OUT/road_dist.png from every route's centre line."""
    pts = []
    for line, _bridge in lines.values():
        p = line[:, :2]
        seg = np.roll(p, -1, axis=0) - p
        pts += [p + seg * t for t in (0.0, 0.25, 0.5, 0.75)]  # 0.25 m apart
    tree = cKDTree(np.vstack(pts))
    out = np.zeros((DIST_SIZE, DIST_SIZE, 3), np.uint8)
    out[..., :2] = 128
    cols = coords(np.arange(RES))
    near = 0
    for r0 in range(0, RES, BAND):
        r1 = min(r0 + BAND, RES)
        y, x = np.meshgrid(coords(np.arange(r0, r1)), cols, indexing="ij")
        here = np.column_stack([x.ravel(), y.ravel()]).astype(np.float64)
        dist, idx = tree.query(here, distance_upper_bound=DIST_NEAR, workers=-1)
        hit = np.isfinite(dist)
        v = np.zeros_like(here)
        v[hit] = tree.data[idx[hit]] - here[hit]
        enc = np.clip(np.round((v / DIST_NEAR + 1) / 2 * 255), 0, 255).astype(np.uint8)
        enc[~hit] = 128
        block = out[r0:r1, :RES]
        block[..., 0] = enc[:, 0].reshape(x.shape)
        block[..., 1] = enc[:, 1].reshape(x.shape)
        block[..., 2] = np.where(hit, 255, 0).reshape(x.shape)
        near += int(hit.sum())
    Image.fromarray(out, "RGB").save(f"{OUT}/road_dist.png")
    print(f"road distance: {near} samples within {DIST_NEAR:.0f} m", flush=True)


def paint_band(x, y, h, slope, d, w, inland):
    """Each layer's weight, 0..1, for one band."""
    n = lambda s, seed, octaves=3: mi.fbm(x, y, s, octaves, SEED + seed)  # noqa: E731
    shoulder = mi.smoothstep(mi.ROAD_HALF + 0.9, mi.ROAD_HALF + 0.2, d)
    rock = mi.smoothstep(0.75, 1.05, slope) * mi.smoothstep(10.0, 25.0, d)
    high = w["mountain"] + w["heath"]
    rock = np.maximum(rock, high * mi.smoothstep(0.45, 0.7, slope) * mi.smoothstep(15, 30, d))
    # The sea cliffs: their faces read as turf at 0.75 (v45 stills), so rock from 0.35 near water.
    sea = mi.smoothstep(80.0, 30.0, inland) * (h > 6)
    rock = np.maximum(rock, sea * mi.smoothstep(0.35, 0.6, slope) * mi.smoothstep(10, 25, d))
    snow = mi.smoothstep(SNOWLINE - 40, SNOWLINE + 40, h + 90 * (n(70, 1) - 0.5))
    snow *= 1 - mi.smoothstep(0.7, 1.1, slope)
    rock *= 1 - snow
    beach_top = 5.5 + 2.5 * n(300, 2)
    # A strip along the water, not a plain: 500..900 m painted the hub's low coast, and the coast
    # road 200 m in, as one sand flat like Red Sands (12 Nov stills); 120..260 m still reached
    # the road where it comes nearer the sea.
    beach = mi.smoothstep(beach_top + 1.5, beach_top - 1.0, h) * mi.smoothstep(150, 60, inland)
    beach = np.maximum(beach, h < 0.5)
    desert = w["desert"]
    # The verge is only the road's own shoulder: the plants' shaders drop whatever stands on the
    # tarmac (road_distance). Until 12 Nov it reached a whole sample past the road, to keep the
    # blades off the tarmac, and left 14..21 m of bare turf beside it. Dirt in the green
    # stretches, Verge (bare sand) in the desert and on the beaches.
    dry = np.clip(beach + desert, 0, 1)
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
    stunt = 1 - 0.5 * mi.smoothstep(TREELINE - 150, TREELINE + 20, zz)
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


def far_trees_band(rng, x, y, h, slope, d, forest, desert):
    """The far forest's stems in a band: [kind, x, y, z, yaw, scale] in cm, as trees.json."""
    stride = max(1, round(FAR_STEP / SPACING))
    pick = np.zeros(d.shape, bool)
    pick[::stride, ::stride] = True
    rows, cols = np.nonzero(pick & (d >= WOODS) & (d < FAR_WOODS))
    if not len(rows):
        return []
    jitter = rng.normal(0, 0.45, (2, len(rows))) * FAR_STEP
    px, py = x[rows, cols] + jitter[0], y[rows, cols] + jitter[1]
    zz = h[rows, cols]
    clump = mi.smoothstep(0.3, 0.62, mi.fbm(px, py, 60, 3, SEED + 70))
    wood = forest[rows, cols] * np.clip(0.15 + 1.1 * clump, 0, 1) * (1 - desert[rows, cols])
    keep = (
        (slope[rows, cols] < 1.0)
        & (rng.random(len(rows)) < wood * 0.8)
        & (rng.random(len(rows)) < mi.smoothstep(TREELINE + 45, TREELINE - 20, zz))
        & (zz > 6)
    )
    px, py, zz = px[keep], py[keep], zz[keep] - 0.6
    line = 260 + 120 * (mi.fbm(px, py, 900, 3, SEED + 60) - 0.5)
    spruce = rng.random(len(zz)) < mi.smoothstep(line - 80, line + 80, zz)
    size = rng.uniform(0.9, 1.35, len(zz)) * (
        1 - 0.5 * mi.smoothstep(TREELINE - 150, TREELINE + 20, zz)
    )
    yaw = rng.uniform(0, 360, len(zz))
    return [
        [
            "spruce" if s else "beech",
            round(float(a) * 100, 1),
            round(float(b) * 100, 1),
            round(float(c) * 100, 1),
            round(float(r), 1),
            round(float(k), 3),
        ]
        for a, b, c, s, r, k in zip(px, py, zz, spruce, yaw, size, strict=True)
    ]


def paint_and_trees(plan, h, dist):
    """Writes each layer's .r8 band by band; returns the trees and the preview's colours."""
    rng = np.random.default_rng(SEED + 100)
    for k in LAYERS:
        open(f"{OUT}/{k}.r8", "wb").close()
    names = list(LAYERS)
    colours = np.array([LAYERS[k] for k in names], dtype=np.uint8)
    small = []
    stems = []
    far = []
    cols = np.arange(RES)
    for r0 in range(0, RES, BAND):
        r1 = min(r0 + BAND, RES)
        a, b = max(r0 - 1, 0), min(r1 + 1, RES)  # a row either side, for the slope
        gy, gx = np.gradient(h[a:b], SPACING)
        slope = np.hypot(gx, gy)[r0 - a : r0 - a + (r1 - r0)]
        y, x = np.meshgrid(coords(np.arange(r0, r1)), coords(cols), indexing="ij")
        hb, db = h[r0:r1], dist[r0:r1]
        w = weights(plan, x, y)
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
        far += far_trees_band(rng, x, y, hb, slope, db, layers["Forest"], w["desert"])
        print(f"band {r0 // BAND + 1}/{math.ceil(RES / BAND)}: {len(stems)} trees", flush=True)
    if len(far) > FAR_MAX:
        far = [far[i] for i in sorted(rng.choice(len(far), FAR_MAX, replace=False))]
    print(f"far forest: {len(far)} trees", flush=True)
    with open(f"{OUT}/trees_far.json", "w") as f:
        json.dump(far, f)
    return stems, np.vstack(small)


def stretches(plan, line):
    """The ring's biomes in order from its start: [(name, metres)], short runs merged."""
    x, y = line[::50, 0], line[::50, 1]
    w = weights(plan, x, y)
    inland = plan.at(plan.inland, x, y)
    keys = list(w)
    best = np.array(keys)[np.argmax(np.stack([w[k] for k in keys]), axis=0)]
    if not CHAIN:  # the chain's coast is every island's edge, so name the island instead
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
    names = CHAIN_NAMES if CHAIN else NAMES
    return [(names.get(k, k.title()), round(m)) for k, m in runs]


# The town's bike trail (the user, 13 Nov, after Chicago's Bloomingdale Trail): the road out of
# the start and home again, while it is within TRAIL_REACH of the harbour, is a pale concrete
# trail with blue-grey rubber shoulders and a yellow dashed line, railed both sides, with
# swan-neck lamps along one (climb_world.trail_dressing, from trail.json).
TRAIL_REACH = 1500.0  # m from the harbour
TRAIL_START = 300.0  # m from the start, wherever the harbour is
TRAIL_RAIL_OUT = 0.35  # m outside the deck's edge
TRAIL_POST_EVERY = 2.5  # m
TRAIL_LAMP_OUT = 0.75  # m outside the deck's edge, behind the rail
TRAIL_LAMP_EVERY = 36  # m


def trail_mask(plan, line):
    """Which points of the ring `line` ((n, 3) m, a metre apart) are the town's trail: the
    stretch out of the start and the stretch home, while they are near the harbour."""
    hx, hy = plan.harbour
    near = np.hypot(line[:, 0] - hx, line[:, 1] - hy) < TRAIL_REACH
    near |= np.hypot(line[:, 0] - line[0, 0], line[:, 1] - line[0, 1]) < TRAIL_START
    out = np.zeros(len(line), bool)
    if near.all():
        return ~out
    if not near[0]:
        return out
    a, b = int(np.argmax(~near)), len(line) - int(np.argmax(~near[::-1]))
    out[:a] = True
    out[b:] = True
    return out


# Roundabouts where a covering route leaves the road before it (the user, 13 Nov: "a roundabout
# with a vigil in the middle"): every route through one is reshaped round its ring, which turns
# the way right-hand traffic does, the island on the rider's left. The ring is meshed whole, so
# its unridden side is road too, and climb_world dresses the island from roundabouts.json.
ROUND_R = 15.0  # m: the ring's centre line from the island's middle
ROUND_REACH = 42.0  # m from the middle where a route leaves its own line for the ring
ROUND_DEFLECT = 0.42  # radians round the ring from where a leg points to where it joins it
ROUND_APART = 900.0  # m between roundabouts at least
ROUND_RUN = 300  # m: a route's own road shorter than this is the smoothing, not a branch
ROUND_START = 600.0  # m from a ride's start
ROUND_BRIDGE = 12.0  # m between a ring's legs and a bridge's end, at least
ROUND_TRAIL = 300.0  # m from the town's trail
ROUND_GRADE = 0.06  # the steepest ground across a junction that takes a ring


def junctions(lines, trail_line, trail):
    """[(x, y, z)] m: where any route leaves or rejoins another's road (on the hub, the coast
    road at each end of each island's bridge), kept off bridges, the trail, the start and steep
    ground, and well apart."""
    found = []
    for a, (la, _) in lines.items():
        for b, (lb, _) in lines.items():
            if a == b:
                continue
            for s, e in extra_runs(la, lb):
                if ROUND_RUN <= e - s <= len(la) - ROUND_RUN:
                    found += [la[(s - 4) % len(la)], la[(e + 4) % len(la)]]
    keep = []
    for c in found:
        if any(np.hypot(*(c[:2] - q[:2])) < ROUND_APART for q in keep):
            continue
        clear = True
        for line, bridge in lines.values():
            d = np.hypot(line[:, 0] - c[0], line[:, 1] - c[1])
            near = line[d < ROUND_REACH]
            clear &= not (d[bridge] < ROUND_REACH + ROUND_BRIDGE).any()
            clear &= np.hypot(*(line[0, :2] - c[:2])) > ROUND_START
            clear &= not (len(near) > 2 and np.ptp(near[:, 2]) > ROUND_GRADE * 2 * ROUND_REACH)
        d0 = np.hypot(trail_line[:, 0] - c[0], trail_line[:, 1] - c[1])
        clear &= not (d0[trail] < ROUND_TRAIL).any()
        if clear:
            keep.append(c.copy())
    return keep


def _hermite(p0, t0, p1, t1, count):
    """`count` points from p0 to p1 (both included) on a cubic leaving p0 along t0 and arriving
    at p1 along t1, tangents scaled to the chord."""
    u = np.linspace(0, 1, count)[:, None]
    chord = float(np.hypot(*(p1 - p0)))
    h00, h10 = 2 * u**3 - 3 * u**2 + 1, u**3 - 2 * u**2 + u
    h01, h11 = -2 * u**3 + 3 * u**2, u**3 - u**2
    return h00 * p0 + h10 * t0 * chord + h01 * p1 + h11 * t1 * chord


def _arc_tangent(theta):
    """The ring's way round at angle `theta`: the angle falling, which puts the island on the
    left of travel as road_mesh has it ((dy, -dx))."""
    return np.array([np.sin(theta), -np.cos(theta)])


def round_through(line, bridge, centre):
    """A closed route (1 m points) and its bridge mask with every pass within ROUND_REACH of
    `centre` ((x, y, z) m) taken round the ring instead. Unchanged if it doesn't pass."""
    c, zc = np.asarray(centre[:2]), float(centre[2])
    d = np.hypot(line[:, 0] - c[0], line[:, 1] - c[1])
    inside = d < ROUND_REACH
    if d.min() > ROUND_R or inside.all() or inside[0]:
        return line, bridge
    pieces, flags, at = [], [], 0
    k = 0
    n = len(line)
    while k < n:
        if not inside[k]:
            k += 1
            continue
        i0 = k
        while k < n and inside[k]:
            k += 1
        i1 = k % n
        if d[i0:k].min() > ROUND_R:  # grazes the circle without passing the middle
            continue
        p0, p1 = line[i0 - 1], line[i1]
        t0 = p0[:2] - line[i0 - 3, :2]
        t1 = line[(i1 + 2) % n, :2] - p1[:2]
        t0, t1 = t0 / np.hypot(*t0), t1 / np.hypot(*t1)
        a0 = math.atan2(p0[1] - c[1], p0[0] - c[0])
        a1 = math.atan2(p1[1] - c[1], p1[0] - c[0])
        sweep = (a0 - a1) % (2 * np.pi)  # round the falling way
        deflect = min(ROUND_DEFLECT, max(sweep - 0.3, 0.0) / 2)
        e0, e1 = a0 - deflect, a0 - sweep + deflect
        on0 = c + ROUND_R * np.array([np.cos(e0), np.sin(e0)])
        on1 = c + ROUND_R * np.array([np.cos(e1), np.sin(e1)])
        leg = ROUND_REACH - ROUND_R
        into = _hermite(p0[:2], t0, on0, _arc_tangent(e0), int(leg * 1.3))
        angles = np.linspace(e0, e1, max(int((e0 - e1) * ROUND_R), 2))
        arc = c + ROUND_R * np.column_stack([np.cos(angles), np.sin(angles)])
        out = _hermite(on1, _arc_tangent(e1), p1[:2], t1, int(leg * 1.3))
        xy = np.vstack([into[1:], arc[1:-1], out[:-1]])
        seg = np.r_[0, np.cumsum(np.hypot(*np.diff(xy, axis=0).T))]
        t = np.arange(0, seg[-1], 1.0)
        xy = np.column_stack([np.interp(t, seg, xy[:, j]) for j in range(2)])
        # Level round the ring, easing to the route's own height along each leg.
        ease_in = mi.smoothstep(leg * 1.2, 0, t)
        ease_out = mi.smoothstep(seg[-1] - leg * 1.2, seg[-1], t)
        z = zc + (p0[2] - zc) * ease_in + (p1[2] - zc) * ease_out
        pieces.append(line[at:i0])
        flags.append(bridge[at:i0])
        pieces.append(np.column_stack([xy, z]))
        flags.append(np.zeros(len(xy), bool))
        at = i1
    if not pieces:
        return line, bridge
    pieces.append(line[at:])
    flags.append(bridge[at:])
    return np.vstack(pieces), np.concatenate(flags)


def roundabouts(plan, centres):
    """roundabouts.json: each island's middle (cm), its radius (cm) and what stands on it, the
    vigil on the one nearest the harbour and planting on the rest."""
    hx, hy = plan.harbour
    order = sorted(
        range(len(centres)), key=lambda i: np.hypot(centres[i][0] - hx, centres[i][1] - hy)
    )
    out = []
    for i, c in enumerate(centres):
        out.append(
            {
                "x": float(c[0] * 100),
                "y": float(c[1] * 100),
                "z": float(c[2] * 100),
                "island": float((ROUND_R - mi.ROAD_HALF - 0.4) * 100),
                "kind": "vigil" if order and i == order[0] else "planted",
            }
        )
    return out


def round_views(lines, centres, plan):
    """A rider's view of the vigil's roundabout: 80 m up the first route that rides through it,
    looking at the island's middle 4 m up."""
    hx, hy = plan.harbour
    if not centres:
        return []
    c = min(centres, key=lambda q: np.hypot(q[0] - hx, q[1] - hy))
    for line, _ in lines.values():
        d = np.hypot(line[:, 0] - c[0], line[:, 1] - c[1])
        if d.min() > ROUND_R + 1:
            continue
        k = int(np.argmax(d < ROUND_REACH)) - 40
        eye = lane_eye(line, k % len(line))[0]
        return [["roundabout", eye, [float(c[0]), float(c[1]), float(c[2]) + 4.0]]]
    return []


def ring_road(centre):
    """The ring itself, closed, a metre apart, level: (n, 3) m."""
    count = int(2 * np.pi * ROUND_R)
    a = np.linspace(0, -2 * np.pi, count, endpoint=False)
    return np.column_stack(
        [
            centre[0] + ROUND_R * np.cos(a),
            centre[1] + ROUND_R * np.sin(a),
            np.full(count, float(centre[2])),
        ]
    )


def surfaces(runs, points, trail=None):
    """[(surface, first point, end point)] round the ring: runs of stretches on one surface, the
    asphalt of `trail`'s points (a mask, see trail_mask) the trail's."""
    each = np.empty(points, object)
    start = 0
    for name, metres in runs:
        end = min(start + round(metres), points)
        each[start:end] = "path" if name in REMOTE else "asphalt"
        start = end
    each[start:] = each[start - 1] if start else "asphalt"
    if trail is not None:
        each[trail & (each == "asphalt")] = "trail"
    out = []
    for k, surface in enumerate(each):
        if out and out[-1][0] == surface:
            out[-1][2] = k + 1
        else:
            out.append([surface, k, k + 1])
    return [tuple(r) for r in out]


def trail_dressing(line, trail, bridges):
    """trail.json: the trail's rail posts, the spans between them and its lamps, in cm, from the
    ring `line` (m) where `trail` (a mask) holds and no bridge is."""
    n = len(line)
    on = trail.copy()
    for a, b in bridges:
        on[np.arange(a, b + 1) % n] = False
    if not on.any():
        return {"posts": [], "spans": [], "lamps": []}
    # The trail as one run of indices, through the ring's start when it wraps.
    first = int(np.argmax(~on)) if not on.all() else 0
    order = np.roll(np.arange(n), -first)
    order = order[on[order]]
    ahead = line[(order + 2) % n, :2] - line[(order - 2) % n, :2]
    ahead /= np.maximum(np.hypot(ahead[:, 0], ahead[:, 1]), 1e-6)[:, None]
    left = np.stack([ahead[:, 1], -ahead[:, 0]], 1)  # road_mesh's left of travel
    half = mi.ROAD_HALF
    posts, spans, lamps = [], [], []
    # A break in the run (a bridge, or the two ends of the trail): no span across it.
    gap = np.r_[False, np.diff(order) % n != 1]
    every = np.round(np.arange(0, len(order), TRAIL_POST_EVERY)).astype(int)
    every = every[every < len(order)]
    for sign in (1.0, -1.0):
        prev = None
        for i in every:
            x, y = line[order[i], :2] + sign * left[i] * (half + TRAIL_RAIL_OUT)
            here = [float(x * 100), float(y * 100), float(line[order[i], 2] * 100)]
            yaw = math.degrees(math.atan2(ahead[i, 1], ahead[i, 0]))
            posts.append([*here, yaw])
            if prev is not None and not gap[prev[1] + 1 : i + 1].any():
                spans.append([*prev[0], *here])
            prev = (here, i)
    for i in range(TRAIL_LAMP_EVERY // 2, len(order), TRAIL_LAMP_EVERY):
        x, y = line[order[i], :2] + left[i] * (half + TRAIL_LAMP_OUT)
        # The arm reaches over the trail: toward its middle, to the right of the lamp.
        yaw = math.degrees(math.atan2(-left[i, 1], -left[i, 0]))
        lamps.append([float(x * 100), float(y * 100), float(line[order[i], 2] * 100), yaw])
    return {"posts": posts, "spans": spans, "lamps": lamps}


def views(line, runs, plan):
    """Stills along the ride, [name, eye, target] in m: a rider's eye halfway along each
    stretch in the right lane looking 30 m up the road, and one from 120 m up over the first
    mountain stretch."""
    out, start = [], 0.0
    coast = np.zeros(len(line), bool)
    for name, metres in runs:
        if name == "The Coast":
            coast[(int(start) + np.arange(int(metres))) % len(line)] = True
        start += metres
    start = 0.0
    for i, (name, metres) in enumerate(runs):
        k = int(start + metres / 2) % len(line)
        ahead = line[(k + 30) % len(line)]
        # Eye and target moved by the same step to the right of the line between them, so the
        # view keeps pointing straight up the road (RideTrack::Point: +lateral is right).
        dx, dy = ahead[0] - line[k, 0], ahead[1] - line[k, 1]
        n = max(float(np.hypot(dx, dy)), 1e-6)
        rx, ry = -dy / n * RIDE_LANE, dx / n * RIDE_LANE
        out.append(
            [
                f"ride_{i:02d}_{name.lower()}",
                [float(line[k, 0] + rx), float(line[k, 1] + ry), float(line[k, 2] + 1.6)],
                [float(ahead[0] + rx), float(ahead[1] + ry), float(ahead[2] + 1.2)],
            ]
        )
        if name in ("Mountain", "Vael") and not any(v[0] == "above" for v in out):
            far = line[(k + 400) % len(line)]
            back = line[(k - 200) % len(line)]
            # Over the peaks, not 120 m up a gorge: since Vael was raised the ridges either side
            # of the road stood above the old eye and filled the frame with a wall of rock.
            t = np.linspace(0.0, 1.0, 64)
            under = plan.at(
                plan.h, back[0] + t * (far[0] - back[0]), back[1] + t * (far[1] - back[1])
            )
            lift = max(float(back[2] + 120), float(under.max()) + 80)
            out.append(
                [
                    "above",
                    [float(back[0]), float(back[1]), lift],
                    [float(far[0]), float(far[1]), float(far[2])],
                ]
            )
        start += metres
    out += cliff_views(line, plan, coast)
    # The best-kept road (road_wear) off the trail, nearest the harbour, and 250 m into the
    # town's trail: a rider's eye as above.
    trail = trail_mask(plan, line)
    if trail[250 % len(line)]:
        out.append(["trail", *lane_eye(line, 250)])
    wear = road_wear(plan, line) + trail
    hx, hy = plan.harbour
    k = int(np.argmin(wear + 1e-5 * np.hypot(line[:, 0] - hx, line[:, 1] - hy)))
    out.append(["road_kept", *lane_eye(line, k)])
    return out


def lane_eye(line, k):
    """[eye, target] m: a rider's eye in the right lane at line[k], looking 30 m up the road."""
    ahead = line[(k + 30) % len(line)]
    dx, dy = ahead[0] - line[k, 0], ahead[1] - line[k, 1]
    n = max(float(np.hypot(dx, dy)), 1e-6)
    rx, ry = -dy / n * RIDE_LANE, dx / n * RIDE_LANE
    return (
        [float(line[k, 0] + rx), float(line[k, 1] + ry), float(line[k, 2] + 1.6)],
        [float(ahead[0] + rx), float(ahead[1] + ry), float(ahead[2] + 1.2)],
    )


def cliff_views(line, plan, coast):
    """The coast road where it runs along a cliff top (high, near the sea): one view from the
    saddle and one from 350 m out to sea, 30 m up, looking back at the cliff and its stacks."""
    if plan.inland is None:
        return []
    inland = plan.at(plan.inland, line[:, 0], line[:, 1])
    tops = np.flatnonzero(coast & (inland < 120) & (line[:, 2] > 20))
    if not len(tops):
        return []
    k = int(tops[np.argmax(line[tops, 2] - 0.05 * inland[tops])])
    ahead = line[(k + 30) % len(line)]
    dx, dy = ahead[0] - line[k, 0], ahead[1] - line[k, 1]
    n = max(float(np.hypot(dx, dy)), 1e-6)
    rx, ry = -dy / n * RIDE_LANE, dx / n * RIDE_LANE
    # Seaward: the way round of 16 where the sea is nearest, 150 m out (a gradient measured 20 m
    # either side came out flat on a cliff top, and put the sea view on the road, 13 Nov).
    x0, y0 = float(line[k, 0]), float(line[k, 1])
    turn = np.linspace(0, 2 * np.pi, 16, endpoint=False)
    near = plan.at(plan.inland, x0 + 150 * np.cos(turn), y0 + 150 * np.sin(turn))
    # With no lead, away from the island's middle.
    flat = np.ptp(near) < 1e-3
    best = float(np.arctan2(y0, x0) if flat else turn[int(np.argmin(near))])
    sx, sy = float(np.cos(best)), float(np.sin(best))
    out_to = 350.0 + float(inland[k])
    return [
        [
            "ride_cliffs",
            [float(line[k, 0] + rx), float(line[k, 1] + ry), float(line[k, 2] + 1.6)],
            [float(ahead[0] + rx), float(ahead[1] + ry), float(ahead[2] + 1.2)],
        ],
        [
            "sea_cliffs",
            [float(line[k, 0] + sx * out_to), float(line[k, 1] + sy * out_to), 30.0],
            [float(line[k, 0]), float(line[k, 1]), float(line[k, 2] * 0.5)],
        ],
    ]


def bridge_spans(bridge):
    """[(first, last)] 1 m point indices of each bridge, for a loop that doesn't start on one."""
    edges = np.flatnonzero(np.diff(bridge.astype(np.int8)))
    return [(int(a) + 1, int(b)) for a, b in zip(edges[::2], edges[1::2], strict=True)]


def split_at_bridges(a, b, spans, n):
    """The run of points a..b (b may pass n, wrapping) cut where it meets a bridge:
    [(on a bridge, first, last)], each a point past its end so the pieces meet."""
    cuts = []
    for c, d in spans:
        for off in (0, n):  # a bridge on the far side of the wrap
            lo, hi = max(a, c + off), min(b, d + off)
            if lo < hi:
                cuts.append((lo, hi))
    cuts.sort()
    out, at = [], a
    for lo, hi in cuts:
        if lo > at:
            out.append((False, at, lo))
        out.append((True, lo, hi))
        at = hi
    if at < b:
        out.append((False, at, b))
    return out


def ride_of(route, runs):
    """info.json's ride entry for a route."""
    return {
        "key": route["key"],
        "title": route["title"],
        "blurb": route["blurb"],
        "closed": True,
        "stretches": runs,
        # Rides start past the harbour (the island's coast), not in the town's bare ground.
        "start": "Harbour" if CHAIN else "Coast",
    }


# The road's upkeep (the user, 13 Nov: "the closer to civilization, the more maintained"): near
# like new by the harbour, kept on the hub's flat low ground, old country tarmac out on the
# islands and up the climbs. Wear 0..1 per metre of road, eased along it so it never steps.
KEPT_TOWN = (3500.0, 1200.0)  # m from the harbour: worn past 3500, like new inside 1200
KEPT_FLAT = (0.035, 0.015)  # grade over 200 m: a climb is worn, the flat is kept
KEPT_LOW = (60.0, 25.0)  # m up: the hub's low ground, not its hilltops
KEPT_HUB = (5200.0, 4200.0)  # m from the hub's centre: the flat kept only on the hub
KEPT_FLAT_SHARE = 0.75  # how kept the hub's flat road is, at most
KEPT_EASE = 400  # m along the road the wear changes over


def road_wear(plan, line):
    """Wear, 0 (like new) to 1 (old), at each point of `line` ((n, 3) m, a metre apart)."""
    x, y, z = line[:, 0], line[:, 1], line[:, 2]
    hx, hy = plan.harbour
    town = mi.smoothstep(*KEPT_TOWN, np.hypot(x - hx, y - hy))
    grade = np.abs(ndimage.uniform_filter1d(np.gradient(z), 200, mode="wrap"))
    flat = mi.smoothstep(*KEPT_FLAT, grade) * mi.smoothstep(*KEPT_LOW, z)
    flat *= mi.smoothstep(*KEPT_HUB, np.hypot(x, y)) * KEPT_FLAT_SHARE
    kept = ndimage.uniform_filter1d(np.maximum(town, flat), KEPT_EASE, mode="wrap")
    return np.clip(1.0 - kept, 0.0, 1.0).astype(np.float32)


def mesh_run(centre, a, b, surface, spans, count, sink=0.0, wear=None):
    """Road pieces for centre[a..b] (wrapping), split where bridges are; `sink` cm lower."""
    n = len(centre)
    for kind, c, d in split_at_bridges(a, b, spans, n):
        here = "bridge" if kind else surface
        run = [(x, y, z - sink) for x, y, z in (centre[k % n] for k in range(c, d + 1))]
        worn = None if wear is None else [wear[k % n] for k in range(c, d + 1)]
        half = (PATH_HALF if here == "path" else mi.ROAD_HALF) * 100
        bank = 0.0 if kind else BANK
        for piece in road_mesh.road_pieces(run, half, False, PIECE, bank, worn):
            piece.write(f"{OUT}/road_{here}_{count[here]:03d}.gltf")
            count[here] += 1


def main():
    os.makedirs(OUT, exist_ok=True)
    plan = Plan()
    h = ground(plan)
    print(f"ground {h.min():.0f}..{h.max():.0f} m", flush=True)
    routes = plan_routes()
    lines = {r["key"]: ring(plan, h, r) for r in routes}
    cover = covering(lines)
    print(f"carved and meshed from {cover}", flush=True)
    centres = junctions(lines, lines[cover[0]][0], trail_mask(plan, lines[cover[0]][0]))
    for c in centres:
        lines = {k: round_through(*v, c) for k, v in lines.items()}
    rings = {f"ring{i}": ring_road(c) for i, c in enumerate(centres)}
    rings = {k: (r, np.zeros(len(r), bool)) for k, r in rings.items()}
    print(f"roundabouts: {[(round(c[0]), round(c[1])) for c in centres]}", flush=True)
    dist = None
    for key in cover:
        d = carve(h, *lines[key])
        dist = d if dist is None else np.minimum(dist, d)
    for ring_line, flat in rings.values():
        dist = np.minimum(dist, carve(h, ring_line, flat))
    for key in cover:
        clear_road(h, lines[key][0])
    for ring_line, _ in rings.values():
        clear_road(h, ring_line)
    road_distance({**lines, **rings})
    stems, small = paint_and_trees(plan, h, dist)
    del dist
    q = np.clip(np.round(32768 + h * 100 * 128 / mi.Z_SCALE), 0, 65535).astype("<u2")
    q.tofile(f"{OUT}/height.r16")
    del q
    Image.fromarray(small).save(f"{OUT}/layers.png")
    rides = [ride_of(r, stretches(plan, lines[r["key"]][0])) for r in routes]
    first = routes[0]["key"]
    line, bridge = lines[first]
    info = {
        "size": RES,
        "spacing": SPACING * 100,
        "z_scale": mi.Z_SCALE,
        "layers": list(LAYERS),
        "biome": "world",
        "window": None,
        "sea_cm": 0.0,
        "road_half": mi.ROAD_HALF,  # m: the road material's edge (climb_world.road_instance)
        # The world's asphalt: lighter and greyer than the woods' dark climb, with a dashed line
        # down the middle (climb_world.road_instance); the path's tarmac-free look is its own.
        # "ground blend" 0: the rim faded into the ground RVT's colour, a lime strip along both
        # edges where the RVT lagged the ground's look (12 Nov). The soil fray edges it alone.
        "road_look": {
            "darken": 0.34,
            "warm": [1.0, 1.0, 0.97],
            "line": 1.0,
            "ground blend": 0.0,
            # The kept road by the harbour (road_wear 0): blacker, with solid edge lines.
            "fresh": 0.78,
            "edge line": 1.0,
        },
        # The rider in the right lane (the user, 13 Nov), negative being right of centre.
        "keep_left": -RIDE_LANE * 100,
        # The default ride, and every ride (climb_world.track writes each as a track).
        "ride": rides[0],
        "rides": rides,
        "views": views(line, rides[0]["stretches"], plan),
    }
    info["views"] += round_views(lines, centres, plan)
    with open(f"{OUT}/info.json", "w") as f:
        json.dump(info, f)
    for name in os.listdir(OUT):
        if name.startswith(("road_", "far_")) and name.endswith(".gltf"):
            os.remove(f"{OUT}/{name}")
    count = {"asphalt": 0, "path": 0, "bridge": 0, "trail": 0}
    # The first covering route whole; each later one only where it leaves the earlier.
    extra, done = [], None
    for key in cover:
        here, here_bridge = lines[key]
        ride = next(r for r in rides if r["key"] == key)
        centre = [(a * 100, b * 100, c * 100 + 2) for a, b, c in here]
        spans = bridge_spans(here_bridge)
        wear = road_wear(plan, here)
        trail = trail_mask(plan, here)
        if done is None:
            for surface, a, b in surfaces(ride["stretches"], len(centre), trail):
                mesh_run(centre, a, b, surface, spans, count, wear=wear)
            with open(f"{OUT}/trail.json", "w") as f:
                json.dump(trail_dressing(here, trail, spans), f)
            done = here
            continue
        kinds = surfaces(ride["stretches"], len(centre), trail)
        for a, b in extra_runs(here, done):
            # Its surface where the run starts; the coast road's is asphalt all round.
            surface = next(s for s, c, d in kinds if c <= a % len(centre) < d)
            mesh_run(centre, a, b, surface, spans, count, sink=UNDER, wear=wear)
            extra.append(here[np.arange(a, b + 1) % len(here)].round(2).tolist())
        done = np.vstack([done, here])
    for ring_line, _ in rings.values():
        centre = [(a * 100, b * 100, c * 100 + 2) for a, b, c in ring_line]
        mesh_run(centre, 0, len(centre), "asphalt", [], count, sink=UNDER)
        extra.append(ring_line.round(2).tolist())
    with open(f"{OUT}/roundabouts.json", "w") as f:
        json.dump(roundabouts(plan, centres), f)
    print(f"road pieces {count}", flush=True)
    with open(f"{OUT}/routes.json", "w") as f:
        json.dump(
            {
                "island": line.round(2).tolist(),
                "bridges": bridge_spans(bridge),
                # Every route at 1 m, and the road only the later covering routes ride.
                "routes": {k: v[0].round(2).tolist() for k, v in lines.items()},
                "extra": extra,
            },
            f,
        )
    with open(f"{OUT}/trees.json", "w") as f:
        json.dump(stems, f)
    for name in ("rails", "props"):
        with open(f"{OUT}/{name}.json", "w") as f:
            json.dump([], f)
    grade = np.diff(line[:, 2])
    spruce = sum(t[0] == "spruce" for t in stems)
    print(
        f"{OUT}: ground {h.min():.0f}..{h.max():.0f} m; {first} {len(line) / 1000:.1f} km,"
        f" {line[:, 2].min():.0f}..{line[:, 2].max():.0f} m, steepest"
        f" {np.abs(ndimage.uniform_filter1d(grade, 100, mode='wrap')).max() * 100:.1f}% over 100 m;"
        f" {len(stems)} trees ({spruce} spruce)"
    )
    for r in rides:
        km = sum(m for _, m in r["stretches"]) / 1000
        print(f"ride {r['key']}: {r['title']}, {km:.1f} km, {r['stretches']}", flush=True)


if __name__ == "__main__":
    main()
