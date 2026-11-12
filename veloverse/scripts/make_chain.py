"""The world as an island chain: one island per biome round an inner sea, joined by bridges, and
one loop road over all of them (12 Nov, the user: "go the Wattopia approach where each biome is a
separate island in a island chain", with "a lot more elevation changes" and an 80-odd km ride).

This is a world plan in make_world.py's layout, so make_world_level.py builds it as L_World
(OC_PLAN=/workspace/ue/world_gen/chain, its default). It is laid out at the built world's own
size: 28.4 km a side, which the landscape's 4065 samples cover at 7 m.

Run: uv run -q --no-project --with numpy --with scipy --with pillow python -I
scripts/make_chain.py. Takes about a minute. Writes to OUT:
- height.npy: heights in metres, float32, RES x RES, rows along Y (X east, Y south, the origin in
  the middle, as Unreal's);
- weights.npz: each biome's share of the ground, float16, at the same grid;
- roads.json: {"ring": [[x, y, z], ...]}, the loop every STEP metres;
- world.json: the grid, the islands, the bridges (metres along the loop), the landmarks;
- preview.png and preview_small.jpg: the map shaded and coloured by biome, with the loop.

How it fits together:
- `ISLANDS` places each island: a warped ellipse, with lobes and coves from noise, and its own
  ground (`island_heights`): low rolling farmland round the harbour, wooded hills, a high moor
  ending in cliffs, a mountain with ridges, and desert with mesas.
- `lay_loop` crosses from island to island at their nearest shores (`bridge_ends`) and, on each
  island, goes the long way round from where it lands to where it leaves, through the far side's
  hills, winding. The mountain's leg climbs to a saddle under the peak.
- The road's height is the ground's, smoothed and held to make_island.MAX_GRADE; a bridge's deck
  runs level-ish between its ends, at least BRIDGE_CLEAR over the sea.
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

OUT = os.environ.get("OC_CHAIN_OUT", "/workspace/ue/world_gen/chain")
EXTENT = 4064 * 7.0  # m: the landscape's 4065 samples at 7 m
RES = 2033  # 14 m a sample; make_world_level adds the finer detail
SPACING = EXTENT / (RES - 1)
STEP = 10.0  # m between road points
SEED = 12
MAP_EDGE = EXTENT / 2 - 500  # no land past this from the middle

# name: centre (m), radii (m), turn (degrees), biome. In order round the loop, clockwise from the
# harbour: the loop leaves each island for the next.
ISLANDS = [
    ("Harbour Isle", (4600, 7900), (5000, 3400), -12, "town"),
    ("Greenwood", (-6300, 5600), (4600, 4000), 25, "forest"),
    ("The Moor", (-7800, -3600), (3500, 4200), -5, "heath"),
    ("Vael", (700, -7700), (5600, 4000), 4, "mountain"),
    ("Red Sands", (8300, -1300), (4000, 4800), 12, "desert"),
]
# Peninsulas: island, the angle (degrees, in its own frame) and radius they leave from, length and
# half-width (m).
CAPES = [
    (0, 200, 0.75, 2600, 650),
    (0, 330, 0.8, 1800, 500),
    (1, 150, 0.8, 3000, 700),
    (1, 250, 0.75, 1600, 450),
    (2, 175, 0.75, 2400, 600),
    (2, 300, 0.8, 1500, 420),
    (3, 255, 0.85, 2200, 800),
    (3, 20, 0.85, 1800, 520),
    (4, 0, 0.8, 2800, 650),
    (4, 290, 0.8, 1500, 420),
]
BIOMES = ("town", "meadow", "forest", "heath", "mountain", "desert")
COLOURS = {
    "town": (170, 150, 140),
    "meadow": (120, 160, 80),
    "forest": (55, 100, 50),
    "heath": (150, 122, 132),
    "mountain": (125, 120, 118),
    "desert": (205, 160, 100),
}
# The harbour: its bay, cut into Harbour Isle's south shore (centre m, radii m), and the town
# round it (radius m).
HARBOUR_BAY = ((4900, 9700), (1300, 1000))
TOWN_REACH = 2200.0
PEAK = 760.0  # m: Vael's summit
BRIDGE_CLEAR = 9.0  # m: a bridge's deck over the sea, at least
LANDFALL = 260.0  # m inland of the shore where a bridge's road meets the island's
# The road's sideways waves on an island: wavelength, m.
WANDER = ((420, 22), (900, 40), (1700, 60))


def grid():
    c = (np.arange(RES, dtype=np.float32) - (RES - 1) / 2) * SPACING
    return np.meshgrid(c, c, indexing="xy")


def local(x, y, island):
    _, (cx, cy), (rx, ry), turn, _ = island
    a = math.radians(turn)
    dx, dy = x - cx, y - cy
    u = dx * math.cos(a) + dy * math.sin(a)
    v = -dx * math.sin(a) + dy * math.cos(a)
    return u / rx, v / ry


def island_field(x, y, i):
    """Under 1 on island i, about 1 per radius out: a warped ellipse with lobes and coves."""
    island = ISLANDS[i]
    s = SEED + 20 * i
    wx = x + 1000 * (mi.fbm(x, y, 3000, 4, s) - 0.5) * 2
    wy = y + 1000 * (mi.fbm(x, y, 3000, 4, s + 1) - 0.5) * 2
    u, v = local(wx, wy, island)
    d = np.hypot(u, v)
    for k, angle, at, length, half in CAPES:
        if k != i:
            continue
        # A capsule from the island out: its field is 1 at `half` from its line.
        a = math.radians(angle)
        p0 = ellipse_point(i, a, at)
        p1 = ellipse_point(i, a, at + length / min(island[2]))
        seg = p1 - p0
        t = np.clip(((wx - p0[0]) * seg[0] + (wy - p0[1]) * seg[1]) / (seg @ seg), 0, 1)
        gap = np.hypot(wx - p0[0] - seg[0] * t, wy - p0[1] - seg[1] * t)
        d = np.minimum(d, gap / (half * (1.15 - 0.55 * t)))
    d = d + 0.36 * (mi.fbm(x, y, 3400, 3, s + 2) - 0.5) * 2  # lobes and bays
    d = d + 0.14 * (mi.fbm(x, y, 900, 4, s + 3) - 0.5) * 2  # coves and points
    if island[4] == "town":
        (bx, by), (brx, bry) = HARBOUR_BAY
        d = d + 2.2 * np.exp(-(((x - bx) / brx) ** 2 + ((y - by) / bry) ** 2))
    return d.astype(np.float32)


def island_heights(x, y, i, d):
    """Island i's ground (m) before the coast falls to the sea; `d` is its field."""
    biome = ISLANDS[i][4]
    s = SEED + 20 * i + 7
    inner = np.clip(1 - d, 0, 1)  # 0 at the shore, 1 in the middle
    rise = mi.smoothstep(0.0, 0.55, inner)
    roll = lambda w, a, k: a * (mi.fbm(x, y, w, 4, s + k) - 0.5) * 2  # noqa: E731
    if biome == "town":
        # Farmland: long low swells and rolling fields, highest inland of the harbour.
        h = 12 + 75 * rise * mi.fbm(x, y, 2200, 3, s) + roll(800, 22, 1) + roll(380, 7, 2)
    elif biome == "forest":
        h = (
            25
            + 170 * rise * (0.35 + mi.fbm(x, y, 2400, 4, s))
            + roll(900, 30, 1)
            + roll(420, 10, 2)
        )
    elif biome == "heath":
        # A moor: a high rolling plateau that ends in sea cliffs, with tors (crags) on its tops.
        h = 60 + 190 * mi.smoothstep(0.0, 0.3, inner) + 70 * mi.fbm(x, y, 1600, 4, s)
        h = h + roll(700, 22, 1) + 18 * mi.ridged(x, y, 300, 3, s + 2) * rise
    elif biome == "mountain":
        u, v = local(x, y, ISLANDS[i])
        peak = np.exp(-((u + 0.05) ** 2 / 0.20 + (v + 0.12) ** 2 / 0.30))
        ridges = mi.ridged(x, y, 1700, 5, s)
        h = 50 + 140 * rise + (PEAK - 120) * peak * (0.65 + 0.35 * ridges)
        h = h + 160 * rise * ridges * (1 - peak) + roll(800, 18, 1)
    else:  # desert: a rolling plain under mesas
        h = 30 + 90 * rise * mi.fbm(x, y, 2600, 3, s) + roll(650, 14, 1) + roll(260, 4, 2)
        h = h + mesas(x, y, i)
    return h


def mesas(x, y, i):
    """Red Sands' mesas and buttes: flat tops on steep walls, (centre as a fraction of the radii,
    radius m, height m)."""
    out = np.zeros_like(x)
    for (fu, fv), radius, height in (
        ((0.35, -0.45), 900, 110),
        ((-0.3, -0.15), 520, 80),
        ((0.15, 0.35), 700, 95),
        ((-0.45, 0.45), 300, 70),
        ((0.55, 0.2), 260, 60),
    ):
        _, (cx, cy), (rx, ry), turn, _ = ISLANDS[i]
        a = math.radians(turn)
        px = cx + fu * rx * math.cos(a) - fv * ry * math.sin(a)
        py = cy + fu * rx * math.sin(a) + fv * ry * math.cos(a)
        r = np.hypot(x - px, y - py) * (1 + 0.25 * (mi.fbm(x, y, 500, 3, SEED + 99) - 0.5))
        out = np.maximum(out, height * mi.smoothstep(radius * 1.15, radius * 0.85, r))
    return out


def terrain():
    """Heights, land, the biome weights and each sample's island (-1 at sea)."""
    x, y = grid()
    fields = np.stack([island_field(x, y, i) for i in range(len(ISLANDS))])
    which = np.argmin(fields, axis=0)
    d = np.take_along_axis(fields, which[None], 0)[0]
    # A strait wherever two islands' fields meet, so no two islands touch.
    second = np.partition(fields, 1, axis=0)[1]
    land = (d < 1.0) & (second - d > 0.2) & (np.maximum(np.abs(x), np.abs(y)) < MAP_EDGE)
    land = ndimage.binary_fill_holes(land)  # no inland pits under the sea plane
    inland = ndimage.distance_transform_edt(land) * SPACING
    offshore = ndimage.distance_transform_edt(~land) * SPACING
    h = np.zeros_like(x)
    for i in range(len(ISLANDS)):
        mine = which == i
        h[mine] = island_heights(x[mine], y[mine], i, d[mine])
    # Cliffs on the moor and the mountain's seaward sides, beaches elsewhere.
    cliffy = np.isin(which, [k for k, isl in enumerate(ISLANDS) if isl[4] in ("heath", "mountain")])
    cliff = np.clip(cliffy * (0.55 + 0.9 * (mi.fbm(x, y, 1400, 3, SEED + 61) - 0.5)), 0, 1)
    fall = 700 + (110 - 700) * cliff
    beach = 2.5 + 1.5 * mi.fbm(x, y, 400, 2, SEED + 62)
    h = beach + (np.maximum(h, 4) - beach) * mi.smoothstep(0, 1, inland / fall)
    sea = -1.0 - 30 * mi.smoothstep(0, 1500, offshore)
    h = np.where(land, h, sea).astype(np.float32)
    weights = {b: np.zeros_like(x) for b in BIOMES}
    for i, isl in enumerate(ISLANDS):
        share = np.exp(-np.maximum(fields[i] - d, 0) * 12)  # 1 on its own island, fading off
        biome = isl[4]
        if biome == "town":
            (bx, by), _ = HARBOUR_BAY
            town = mi.smoothstep(TOWN_REACH, TOWN_REACH * 0.6, np.hypot(x - bx, y - by))
            weights["town"] += share * town
            weights["meadow"] += share * (1 - town)
        else:
            weights[biome] += share
    total = sum(weights.values()) + 1e-6
    weights = {k: (v / total).astype(np.float32) for k, v in weights.items()}
    return h, land, which.astype(np.int8), weights, cliff, inland


def sample(a, xy, order=1):
    col = xy[:, 0] / SPACING + (RES - 1) / 2
    row = xy[:, 1] / SPACING + (RES - 1) / 2
    return ndimage.map_coordinates(a, [row, col], order=order, mode="nearest")


def bridge_ends(land, which, a, b):
    """The nearest shores of islands a and b: two points LANDFALL inland, and the shore points."""
    pa = np.argwhere((which == a) & land)
    pb = np.argwhere((which == b) & land)
    # Their shores: land samples with sea beside them.
    sea = ~land
    edge = ndimage.binary_dilation(sea) & land
    sa = pa[edge[pa[:, 0], pa[:, 1]]]
    sb = pb[edge[pb[:, 0], pb[:, 1]]]
    from scipy.spatial import cKDTree

    tree = cKDTree(sb)
    dist, k = tree.query(sa)
    best = int(np.argmin(dist))
    ra, rb = sa[best], sb[k[best]]

    def to_xy(rc):
        return np.array([(rc[1] - (RES - 1) / 2) * SPACING, (rc[0] - (RES - 1) / 2) * SPACING])

    shore_a, shore_b = to_xy(ra), to_xy(rb)
    u = (shore_b - shore_a) / np.linalg.norm(shore_b - shore_a)
    return shore_a - u * LANDFALL, shore_b + u * LANDFALL, shore_a, shore_b


def ellipse_point(i, angle, radius):
    _, (cx, cy), (rx, ry), turn, _ = ISLANDS[i]
    a = math.radians(turn)
    u, v = radius * math.cos(angle) * rx, radius * math.sin(angle) * ry
    return np.array(
        [cx + u * math.cos(a) - v * math.sin(a), cy + u * math.sin(a) + v * math.cos(a)]
    )


def angle_on(i, p):
    _, (cx, cy), (rx, ry), turn, _ = ISLANDS[i]
    a = math.radians(turn)
    dx, dy = p[0] - cx, p[1] - cy
    u = dx * math.cos(a) + dy * math.sin(a)
    v = -dx * math.sin(a) + dy * math.cos(a)
    return math.atan2(v / ry, u / rx)


def onshore(p, i, inland, margin=400.0):
    """p moved toward island i's middle until it is `margin` m inland, and off the harbour bay."""
    centre = np.array(ISLANDS[i][1], float)
    if ISLANDS[i][4] == "town":
        (bx, by), (brx, bry) = HARBOUR_BAY
        if ((p[0] - bx) / brx) ** 2 + ((p[1] - by) / (bry + 900)) ** 2 < 1.6:
            p = np.array([p[0], min(p[1], by - bry - 450)])
    for _ in range(40):
        if sample(inland, p[None])[0] >= margin:
            break
        p = p + (centre - p) * 0.06
    return p


def island_leg(i, enter, leave, rng, inland):
    """Control points (m) across island i from `enter` to `leave`, the long way round: the side
    away from the inner sea, through its hills."""
    a0, a1 = angle_on(i, enter), angle_on(i, leave)
    centre = np.array(ISLANDS[i][1])
    # Both ways round; the long way is the one whose middle is further from the map's middle.
    up = (a1 - a0) % (2 * math.pi)
    down = up - 2 * math.pi
    mids = [ellipse_point(i, a0 + span / 2, 0.6) for span in (up, down)]
    span = up if np.linalg.norm(mids[0]) > np.linalg.norm(mids[1]) else down
    biome = ISLANDS[i][4]
    count = max(3, int(abs(span) * 3.4))
    points = [enter]
    for k in range(1, count):
        t = k / count
        radius = rng.uniform(0.55, 0.8)
        if biome == "mountain":
            radius = 0.62 - 0.32 * math.sin(math.pi * t)  # up to a saddle under the summit
        points.append(onshore(ellipse_point(i, a0 + span * t, radius), i, inland))
    if biome == "town":
        # Down through the harbour town, on the bay's inland side.
        (bx, by), (_, bry) = HARBOUR_BAY
        quay = np.array([bx, by - bry - 450])
        k = int(np.argmin([np.linalg.norm(p - quay) for p in points[1:]])) + 1
        points[k] = quay
    points.append(leave)
    del centre
    # Drop waypoints that crowd the one before (two pulled to the same spot tie a knot).
    kept = [points[0]]
    for k, p in enumerate(points[1:], 1):
        if k == len(points) - 1 or np.linalg.norm(p - kept[-1]) > 700:
            kept.append(p)
    return kept


def even(xy, closed, step=STEP):
    loop = np.vstack([xy, xy[:1]]) if closed else xy
    seg = np.linalg.norm(np.diff(loop, axis=0), axis=1)
    at = np.concatenate([[0], np.cumsum(seg)])
    return np.stack([np.interp(np.arange(0, at[-1], step), at, loop[:, k]) for k in (0, 1)], 1)


def catmull(points, step):
    """A closed centripetal Catmull-Rom through the points (m), every `step` m."""
    return mi.catmull([p / 1000 for p in points], True, step=step)


def lay_loop(h, land, which, inland):
    """The loop: [x, y] every STEP m, and the bridges as (first, last) point indices."""
    rng = np.random.default_rng(SEED)
    n = len(ISLANDS)
    crossings = [bridge_ends(land, which, i, (i + 1) % n) for i in range(n)]
    control, spans = [], []
    for i in range(n):
        enter = crossings[i - 1][1]
        leave = crossings[i][0]
        leg = island_leg(i, enter, leave, rng, inland)
        control += leg
        spans.append(len(control) - 1)  # the leg's last point: the bridge starts here
    # The bridges are straight: lay each island's leg as a curve, the crossings as lines.
    pieces, bridges, start = [], [], 0
    total = 0
    for i in range(n):
        leg = control[start : spans[i] + 1]
        start = spans[i] + 1
        curve = mi.catmull([p / 1000 for p in leg], False, step=STEP)
        curve = np.vstack([curve, leg[-1]])
        curve = wander(even(curve, False), land)
        pieces.append(curve)
        total += len(curve)
        a, b = leg[-1], control[start % len(control)]
        length = np.linalg.norm(b - a)
        k = max(int(length / STEP), 2)
        line = a + (b - a) * (np.arange(1, k) / k)[:, None]
        bridges.append((total, total + len(line) - 1))
        pieces.append(line)
        total += len(line)
    xy = np.vstack(pieces)
    return xy, bridges, crossings


def wander(xy, land):
    """Sideways waves, none at the ends (the bridges) or near the shore."""
    s = np.arange(len(xy)) * STEP
    t = np.gradient(xy, axis=0)
    t /= np.linalg.norm(t, axis=1, keepdims=True) + 1e-9
    rng = np.random.default_rng(SEED + len(xy))
    off = sum(a * np.sin(2 * math.pi * s / w + rng.uniform(0, 6.28)) for w, a in WANDER)
    off = off * mi.smoothstep(0, 900, np.minimum(s, s[-1] - s))
    moved = xy + np.stack([-t[:, 1], t[:, 0]], axis=1) * off[:, None]
    return even(moved, False)


def heights(h, xy, bridges):
    """The road's heights: the ground smoothed, the bridges' decks over the sea, held to the
    grade."""
    soft = ndimage.gaussian_filter(h, 40 / SPACING)
    z = sample(soft, xy).astype(np.float64)
    for a, b in bridges:
        za, zb = z[a - 1], z[(b + 1) % len(z)]
        t = np.linspace(0, 1, b - a + 1)
        arch = 4 * t * (1 - t) * min(12.0, (b - a) * STEP * 0.02)
        z[a : b + 1] = np.maximum(za + (zb - za) * t + arch, BRIDGE_CLEAR)
    z = mi.smooth(z, 6, True)
    z = np.maximum(z, 3.0)
    # Pin the decks: the grade fit would otherwise lift them toward the hills beyond.
    fixed = {}
    for a, b in bridges:
        for i in range(a, b + 1, 5):
            fixed[i] = z[i] / STEP
        fixed[b] = z[b] / STEP
    return mi.grade_fit(z / STEP, fixed, True) * STEP


def landings(h, land, crossings):
    """Low ground where each bridge comes ashore, rising inland at about 4.5%, so a bridge spans
    the strait near the water rather than from clifftop to clifftop."""
    x, y = grid()
    for _, _, shore_a, shore_b in crossings:
        for p in (shore_a, shore_b):
            d = np.hypot(x - p[0], y - p[1])
            low = 6.0 + 0.045 * np.maximum(d - 150, 0)
            w = mi.smoothstep(3200, 1200, d) * land
            h = np.where(w > 0, h + (np.minimum(h, low) - h) * w, h)
    return h.astype(np.float32)


def keep_land(h, land, xy, bridges):
    """Land under every road point that isn't a bridge: a coast's cove can't cut the road."""
    on = np.ones(len(xy), bool)
    for a, b in bridges:
        on[a : b + 1] = False
    rc = np.round(xy[on][:, ::-1] / SPACING + (RES - 1) / 2).astype(int)
    mark = np.zeros_like(land)
    mark[rc[:, 0], rc[:, 1]] = True
    near = ndimage.binary_dilation(mark, iterations=int(140 / SPACING))
    added = near & ~land
    print(f"land kept under the road: {added.sum()} samples added", flush=True)
    return added


def landmarks(h, which, xy, bridges, crossings):
    out = []
    for i, (name, centre, _, _, biome) in enumerate(ISLANDS):
        mine = np.where(which == i, h, -1e9)
        r, c = np.unravel_index(int(np.argmax(mine)), h.shape)
        top = [
            float((c - (RES - 1) / 2) * SPACING),
            float((r - (RES - 1) / 2) * SPACING),
            float(h[r, c]),
        ]
        out.append({"name": name, "biome": biome, "centre": list(centre), "top": top})
    (bx, by), _ = HARBOUR_BAY
    return {"islands": out, "harbour": [bx, by]}


def preview(h, land, weights, xy, bridges):
    gy, gx = np.gradient(h, SPACING)
    shade = np.clip(0.75 + (gx + gy) * 0.7, 0.35, 1.25)  # lit from the north-west (-X, -Y)
    col = sum(weights[b][..., None] * np.array(COLOURS[b], np.float32) for b in BIOMES)
    img = np.where(land[..., None], col * shade[..., None], np.array([30, 60, 100], np.float32))
    img = np.clip(img, 0, 255).astype(np.uint8)
    rc = np.round(xy[:, ::-1] / SPACING + (RES - 1) / 2).astype(int)
    on_bridge = np.zeros(len(xy), bool)
    for a, b in bridges:
        on_bridge[a : b + 1] = True
    for (r, c), br in zip(rc, on_bridge, strict=True):
        img[max(r - 1, 0) : r + 2, max(c - 1, 0) : c + 2] = (
            (255, 200, 60) if br else (255, 255, 255)
        )
    Image.fromarray(img).save(f"{OUT}/preview.png")
    Image.fromarray(img).resize((1024, 1024), Image.LANCZOS).save(
        f"{OUT}/preview_small.jpg", quality=88
    )


def profile_stats(xy, z):
    seg = np.linalg.norm(np.diff(np.vstack([xy, xy[:1]]), axis=0), axis=1)
    dz = np.diff(np.concatenate([z, z[:1]]))
    grade = ndimage.uniform_filter1d(dz / np.maximum(seg, 1e-6), 10, mode="wrap")
    return {
        "km": round(float(seg.sum()) / 1000, 1),
        "climb_m": round(float(np.clip(dz, 0, None).sum())),
        "top_m": round(float(z.max())),
        "steepest_100m": round(float(np.abs(grade).max()), 3),
        "over_3pct": round(float(np.mean(np.abs(grade) > 0.03)), 2),
    }


def main():
    os.makedirs(OUT, exist_ok=True)
    h, land, which, weights, _cliff, inland = terrain()
    print(f"terrain: {h.min():.0f}..{h.max():.0f} m, land {land.mean() * 100:.0f}%", flush=True)
    xy, bridges, crossings = lay_loop(h, land, which, inland)
    h = landings(h, land, crossings)
    added = keep_land(h, land, xy, bridges)
    if added.any():
        # Raise the new land to a low bank and blend it in.
        h = np.where(added, np.maximum(h, 4.0), h)
        land = land | added
    z = heights(h, xy, bridges)
    stats = profile_stats(xy, z)
    print("loop", stats, flush=True)
    for k, (a, b) in enumerate(bridges):
        print(
            f"bridge {k}: {(b - a) * STEP:.0f} m,"
            f" deck {z[a : b + 1].min():.1f}..{z[a : b + 1].max():.1f} m",
            flush=True,
        )
    np.save(f"{OUT}/height.npy", h)
    np.savez_compressed(
        f"{OUT}/weights.npz", **{k: v.astype(np.float16) for k, v in weights.items()}
    )
    with open(f"{OUT}/roads.json", "w") as f:
        json.dump({"ring": np.column_stack([xy, z]).round(2).tolist()}, f)
    info = {
        "extent_m": EXTENT,
        "res": RES,
        "spacing_m": SPACING,
        "kind": "chain",
        "islands": [
            {"name": n, "centre": list(c), "radii": list(r), "turn": t, "biome": b}
            for n, c, r, t, b in ISLANDS
        ],
        "bridges": [[a * STEP, b * STEP] for a, b in bridges],
        "landmarks": landmarks(h, which, xy, bridges, crossings),
        "loop": stats,
    }
    with open(f"{OUT}/world.json", "w") as f:
        json.dump(info, f, indent=1)
    preview(h, land, weights, xy, bridges)
    print(f"{OUT}: done", flush=True)


if __name__ == "__main__":
    main()
