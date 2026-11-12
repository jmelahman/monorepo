"""The world as islands round a hub: Harbour Isle, the coast and harbour island, in the middle,
and one island per biome round it, each joined to it by two bridges (12 Nov, the user: "go the
Wattopia approach where each biome is a separate island", then "put the beach/coast island in
the middle which allows for more varied routes", such as forest to coast to mountain without the
desert).

This is a world plan in make_world.py's layout, so make_world_level.py builds it as L_World
(OC_PLAN=/workspace/ue/world_gen/hub, its default). It is laid out at the built world's own size:
28.4 km a side, which the landscape's 4065 samples cover at 7 m.

Run: uv run -q --no-project --with numpy --with scipy --with pillow python -I
scripts/make_chain.py. Takes about a minute. Writes to OUT:
- height.npy: heights in metres, float32, RES x RES, rows along Y (X east, Y south, the origin in
  the middle, as Unreal's);
- weights.npz: each biome's share of the ground, float16, at the same grid;
- roads.json: {"routes": [{"key", "title", "blurb", "islands", "xyz": [[x, y, z], ...] every
  STEP m from the harbour quay, "bridges": [[first, last] indices]}]}, the default first;
- world.json: the grid, the islands, the landmarks, each route's length and climb;
- preview.png and preview_small.jpg: the map shaded and coloured by biome, with the roads.

How it fits together:
- `ISLANDS` places each island: a warped ellipse, with lobes and coves from noise, and its own
  ground (`island_heights`): low rolling coast round the harbour, wooded hills, a high moor
  ending in cliffs, a mountain with ridges, and desert with mesas.
- `coast_road` rings Harbour Isle COAST_IN from its shore, through the harbour quay. Each petal
  has a `Spur`: off the coast road over one bridge (sited where the strait is narrowest, clear of
  the harbour), the long way round the island through its hills, and back over the other.
- Each road's heights are worked out once (`heights`), the spurs pinned to the coast road, so
  every route over a road agrees. A route (`assemble`) is the coast road from the quay, turning
  onto the spurs of the islands it visits, its junctions rounded: any set of islands is a loop
  with no road ridden twice.
- A road's height is the ground's, smoothed and held to make_island.MAX_GRADE; a bridge's deck
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

OUT = os.environ.get("OC_CHAIN_OUT", "/workspace/ue/world_gen/hub")
EXTENT = 4064 * 7.0  # m: the landscape's 4065 samples at 7 m
RES = 2033  # 14 m a sample; make_world_level adds the finer detail
SPACING = EXTENT / (RES - 1)
STEP = 10.0  # m between road points
SEED = 12
MAP_EDGE = EXTENT / 2 - 500  # no land past this from the middle


# name: centre (m), radii (m), turn (degrees), biome. Harbour Isle is the hub, in the middle;
# the others are its petals, round it clockwise from the harbour (the user, 12 Nov: "put the
# beach/coast island in the middle which allows for more varied routes"). A petal's first radius
# runs along the hub's shore, its second away from it.
def _petal(name, angle, distance, radii, biome):
    a = math.radians(angle)
    return (name, (distance * math.cos(a), distance * math.sin(a)), radii, angle - 90, biome)


ISLANDS = [
    ("Harbour Isle", (0.0, 0.0), (4600, 4300), 0, "town"),
    _petal("Greenwood", 142, 6900, (4300, 3300), "forest"),
    _petal("Vael", 225, 7500, (4700, 3700), "mountain"),
    _petal("The Moor", 315, 7200, (4200, 3300), "heath"),
    _petal("Red Sands", 38, 7100, (4300, 3300), "desert"),
]
HUB = 0
# Peninsulas: island, the angle (degrees, in its own frame) and radius they leave from, length and
# half-width (m). A petal's 90 degrees points away from the hub; none point at it.
CAPES = [
    (0, 0, 0.8, 1800, 600),
    (0, 180, 0.8, 1800, 600),
    (0, 270, 0.85, 1400, 500),
    (1, 40, 0.8, 2200, 650),
    (1, 150, 0.8, 1600, 450),
    (2, 60, 0.85, 2000, 700),
    (2, 160, 0.85, 1500, 520),
    (3, 30, 0.8, 1800, 600),
    (3, 130, 0.8, 1500, 420),
    (4, 50, 0.8, 2400, 650),
    (4, 150, 0.8, 1500, 420),
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
HARBOUR_BAY = ((200, 2600), (1100, 850))
TOWN_REACH = 2200.0
PEAK = 760.0  # m: Vael's summit
BRIDGE_CLEAR = 9.0  # m: a bridge's deck over the sea, at least
LANDFALL = 260.0  # m inland of a petal's shore where its bridge's road meets the island's
COAST_IN = 200.0  # m: the hub's coast road in from the shore
# m: how far either side of the line from the hub to a petal its two bridges may run; each takes
# the narrowest crossing in that range.
BRIDGE_APART = (600.0, 2800.0)
HARBOUR_CLEAR = 500.0  # m: no bridge comes ashore this near the harbour bay's edge
LEAD = 60.0  # m of road from the coast road to a bridge's deck
CORNER = 50.0  # m: a junction's turn rounded over this either side
# The routes, the first the level's default: key, title, blurb, the petals they visit.
ROUTES = [
    (
        "tour",
        "The Grand Tour",
        "Every island: out from the harbour to Greenwood, Vael, The Moor and Red Sands in turn.",
        ("Greenwood", "Vael", "The Moor", "Red Sands"),
    ),
    ("harbour", "Harbour Isle", "Once round the hub island's coast road, through the harbour.", ()),
    ("greenwood", "Greenwood Loop", "Over to the forest island and back.", ("Greenwood",)),
    ("vael", "Vael Loop", "Up the mountain island's saddle under the summit and back.", ("Vael",)),
    ("moor", "Moor Loop", "The high moor's tors and wind farm.", ("The Moor",)),
    ("sands", "Red Sands Loop", "Among the desert island's mesas.", ("Red Sands",)),
    (
        "woods_peaks",
        "Woods and Peaks",
        "The forest, back over Harbour Isle, then the mountain: no desert.",
        ("Greenwood", "Vael"),
    ),
    ("highlands", "The Highlands", "The mountain, then the moor.", ("Vael", "The Moor")),
    ("east", "Heath and Sand", "The moor, then the desert.", ("The Moor", "Red Sands")),
]
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
    land = (d < 1.0) & (second - d > 0.08) & (np.maximum(np.abs(x), np.abs(y)) < MAP_EDGE)
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


def onshore(p, i, inland, margin=400.0, which=None):
    """p moved toward island i's middle until it is `margin` m inland, and off the harbour bay."""
    centre = np.array(ISLANDS[i][1], float)
    if ISLANDS[i][4] == "town":
        (bx, by), (brx, bry) = HARBOUR_BAY
        if ((p[0] - bx) / brx) ** 2 + ((p[1] - by) / (bry + 900)) ** 2 < 1.6:
            p = np.array([p[0], min(p[1], by - bry - 450)])
    for _ in range(40):
        mine = which is None or sample(which, p[None], order=0)[0] == i
        if mine and sample(inland, p[None])[0] >= margin:
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
        radius = rng.uniform(0.5, 0.72)
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


def coast_road(inland, which):
    """Harbour Isle's coast road, closed, COAST_IN from the shore, starting at the harbour quay
    and running clockwise (toward +angle, Y being south): [x, y] every STEP m."""
    rng = np.random.default_rng(SEED + 5)
    (bx, by), (_, bry) = HARBOUR_BAY
    quay = np.array([bx, by - bry - 450.0])
    a0 = math.atan2(quay[1], quay[0])
    points = [quay]
    count = 30
    for k in range(1, count):
        p = ellipse_point(HUB, a0 + 2 * math.pi * k / count, 1.1)
        p = onshore(p, HUB, inland, COAST_IN + rng.uniform(-40, 140), which)
        last = k == count - 1 and np.linalg.norm(p - quay) <= 450
        if np.linalg.norm(p - points[-1]) > 450 and not last:
            points.append(p)
    return even(catmull(points, STEP), True)


def usable(site, land):
    """Whether a bridge site comes ashore on the hub away from the harbour bay, with its junction
    there too, and land all the way from the junction to the hub's shore."""
    _, line, (hub_shore, _) = site
    (bx, by), (brx, bry) = HARBOUR_BAY
    clear = max(brx, bry) + HARBOUR_CLEAR
    if min(np.hypot(*(hub_shore - (bx, by))), np.hypot(*(line[0] - (bx, by)))) < clear:
        return False
    ashore = line[np.linalg.norm(line - line[0], axis=1) < np.linalg.norm(hub_shore - line[0])]
    return bool(sample(land.astype(np.float32), ashore, order=0).min() > 0.5)


class Spur:
    """A petal's road from the coast road and back: out over one bridge, the long way round the
    island, home over the other. `xy` runs from junction to junction, both on the coast road at
    ring indices `enter` and `leave`; `bridges` are (first, last) indices into xy."""

    def __init__(self, p, ring, land, which, inland, rng):
        self.p = p
        sites = [
            min(
                (
                    site
                    for apart in np.linspace(*BRIDGE_APART, 17)
                    if usable(site := self.site(p, side * apart, ring, land, which), land)
                ),
                key=lambda site: np.linalg.norm(site[1][-1] - site[1][0]),
            )
            for side in (-1, 1)
        ]
        sites.sort(key=lambda site: site[0])
        assert sites[1][0] - sites[0][0] < len(ring) / 3, (
            f"{ISLANDS[p][0]}'s bridges straddle the quay"
        )
        (self.enter, out_in, shore_in), (self.leave, out_back, shore_back) = sites
        self.shores = shore_in + shore_back
        leg = island_leg(p, out_in[-1], out_back[-1], rng, inland)
        curve = mi.catmull([q / 1000 for q in leg], False, step=STEP)
        curve = wander(even(np.vstack([curve, leg[-1]]), False), land)
        there, back = out_in, out_back[::-1]
        self.xy = np.vstack([ring[self.enter], there[1:], curve[1:-1], back[:-1], ring[self.leave]])
        a = 1 + int(LEAD / STEP)
        b = len(there) - 1
        c = b + len(curve) - 1
        d = len(self.xy) - 1 - int(LEAD / STEP)
        self.bridges = [(a, b), (c, d)]

    @staticmethod
    def site(p, offset, ring, land, which):
        """Where one of the petal's bridges runs: (the coast road's index there, the straight from
        it to LANDFALL inland on the petal, [the hub's shore, the petal's shore])."""
        centre = np.array(ISLANDS[p][1], float)
        u = centre / np.linalg.norm(centre)
        n = np.array([-u[1], u[0]])
        o = offset * n
        t = np.arange(0, 14000, 5.0)
        pts = o + t[:, None] * u
        rc = np.round(pts[:, ::-1] / SPACING + (RES - 1) / 2).astype(int).clip(0, RES - 1)
        here, own = land[rc[:, 0], rc[:, 1]], which[rc[:, 0], rc[:, 1]]
        first = int(np.argmax(here & (own == p)))
        hub = np.flatnonzero(here[:first] & (own[:first] == HUB))
        hub_shore, petal_shore = pts[hub[-1]], pts[first]
        rel = ring - o
        along, across = rel @ u, np.abs(rel @ n)
        j = int(np.argmin(np.where((along > 0) & (along < t[first]), across, np.inf)))
        end = petal_shore + u * LANDFALL
        length = np.linalg.norm(end - ring[j])
        k = max(int(length / STEP), 2)
        line = ring[j] + (end - ring[j]) * (np.arange(0, k + 1) / k)[:, None]
        return j, line, [hub_shore, petal_shore]


def lay_network(land, which, inland):
    """The coast road and every petal's spur."""
    ring = coast_road(inland, which)
    rng = np.random.default_rng(SEED)
    spurs = {ISLANDS[p][0]: Spur(p, ring, land, which, inland, rng) for p in range(1, len(ISLANDS))}
    return ring, spurs


def assemble(ring, ring_z, spurs, names):
    """A route: the coast road from the quay, turning off onto each named petal's spur where it
    leaves and back on where it returns. [x, y, z] and its bridges as (first, last) indices."""
    chosen = sorted((spurs[n] for n in names), key=lambda sp: sp.enter)
    pieces, bridges, turns, at, total = [], [], [], 0, 0
    for sp in chosen:
        arc = np.column_stack([ring[at : sp.enter], ring_z[at : sp.enter]])
        pieces.append(arc)
        total += len(arc)
        turns += [total, total + len(sp.xy) - 1]
        bridges += [(a + total, b + total) for a, b in sp.bridges]
        pieces.append(np.column_stack([sp.xy, sp.z]))
        total += len(sp.xy)
        at = sp.leave + 1
    pieces.append(np.column_stack([ring[at:], ring_z[at:]]))
    xyz = np.vstack(pieces)
    m = int(CORNER / STEP)
    for k in turns:
        # The junction's corner as a quadratic Bezier over CORNER either side.
        p0, p1, p2 = xyz[k - m], xyz[k], xyz[k + m]
        t = np.linspace(0, 1, 2 * m + 1)[:, None]
        xyz[k - m : k + m + 1] = (1 - t) ** 2 * p0 + 2 * t * (1 - t) * p1 + t**2 * p2
    return xyz, bridges


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


def heights(h, xy, bridges, closed, ends=None):
    """The road's heights: the ground smoothed, the bridges' decks over the sea, held to the
    grade; an open road's `ends` (m) pinned."""
    soft = ndimage.gaussian_filter(h, 40 / SPACING)
    z = sample(soft, xy).astype(np.float64)
    if ends is not None:
        z[0], z[-1] = ends
    n = len(z)
    for a, b in bridges:
        za, zb = z[a - 1 if a else 0], z[(b + 1) % n if closed else min(b + 1, n - 1)]
        t = np.linspace(0, 1, b - a + 1)
        arch = 4 * t * (1 - t) * min(12.0, (b - a) * STEP * 0.02)
        z[a : b + 1] = np.maximum(za + (zb - za) * t + arch, BRIDGE_CLEAR)
    z = mi.smooth(z, 6, closed)
    z = np.maximum(z, 3.0)
    # Pin the decks: the grade fit would otherwise lift them toward the hills beyond.
    fixed = {}
    for a, b in bridges:
        for i in range(a, b + 1, 5):
            fixed[i] = z[i] / STEP
        fixed[b] = z[b] / STEP
    if ends is not None:
        fixed[0], fixed[n - 1] = ends[0] / STEP, ends[1] / STEP
    return mi.grade_fit(z / STEP, fixed, closed) * STEP


def landings(h, land, shores):
    """Low ground where each bridge comes ashore, rising inland at about 4.5%, so a bridge spans
    the strait near the water rather than from clifftop to clifftop."""
    x, y = grid()
    for p in shores:
        d = np.hypot(x - p[0], y - p[1])
        low = 6.0 + 0.045 * np.maximum(d - 150, 0)
        w = mi.smoothstep(3200, 1200, d) * land
        h = np.where(w > 0, h + (np.minimum(h, low) - h) * w, h)
    return h.astype(np.float32)


def keep_land(land, roads):
    """Land under every road point that isn't a bridge: a coast's cove can't cut the road.
    `roads`: [(xy, bridges)]."""
    mark = np.zeros_like(land)
    for xy, bridges in roads:
        on = np.ones(len(xy), bool)
        for a, b in bridges:
            on[a : b + 1] = False
        rc = np.round(xy[on][:, ::-1] / SPACING + (RES - 1) / 2).astype(int)
        mark[rc[:, 0], rc[:, 1]] = True
    near = ndimage.binary_dilation(mark, iterations=int(140 / SPACING))
    added = near & ~land
    print(f"land kept under the road: {added.sum()} samples added", flush=True)
    return added


def landmarks(h, which):
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


def preview(h, land, weights, roads):
    """`roads`: [(xy, bridges)], drawn white, bridges gold."""
    gy, gx = np.gradient(h, SPACING)
    shade = np.clip(0.75 + (gx + gy) * 0.7, 0.35, 1.25)  # lit from the north-west (-X, -Y)
    col = sum(weights[b][..., None] * np.array(COLOURS[b], np.float32) for b in BIOMES)
    img = np.where(land[..., None], col * shade[..., None], np.array([30, 60, 100], np.float32))
    img = np.clip(img, 0, 255).astype(np.uint8)
    for xy, bridges in roads:
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
    ring, spurs = lay_network(land, which, inland)
    h = landings(h, land, [q for sp in spurs.values() for q in sp.shores])
    added = keep_land(land, [(ring, [])] + [(sp.xy, sp.bridges) for sp in spurs.values()])
    if added.any():
        # Raise the new land to a low bank and blend it in.
        h = np.where(added, np.maximum(h, 4.0), h)
        land = land | added
    # Heights once for each piece of road, so every route over it agrees: the coast road, then
    # each spur pinned to it at both ends.
    ring_z = heights(h, ring, [], True)
    for sp in spurs.values():
        sp.z = heights(h, sp.xy, sp.bridges, False, (ring_z[sp.enter], ring_z[sp.leave]))
    routes, summary = [], []
    for key, title, blurb, names in ROUTES:
        xyz, bridges = assemble(ring, ring_z, spurs, names)
        stats = profile_stats(xyz[:, :2], xyz[:, 2])
        print(f"route {key}: {stats}", flush=True)
        routes.append(
            {
                "key": key,
                "title": title,
                "blurb": blurb,
                "islands": ["Harbour Isle", *names],
                "xyz": xyz.round(2).tolist(),
                "bridges": bridges,
            }
        )
        summary.append({"key": key, "title": title, **stats})
    for name, sp in spurs.items():
        for k, (a, b) in enumerate(sp.bridges):
            deck = sp.z[a : b + 1]
            print(
                f"bridge {name} {k}: {(b - a) * STEP:.0f} m,"
                f" deck {deck.min():.1f}..{deck.max():.1f} m",
                flush=True,
            )
    np.save(f"{OUT}/height.npy", h)
    np.savez_compressed(
        f"{OUT}/weights.npz", **{k: v.astype(np.float16) for k, v in weights.items()}
    )
    with open(f"{OUT}/roads.json", "w") as f:
        json.dump({"routes": routes}, f)
    info = {
        "extent_m": EXTENT,
        "res": RES,
        "spacing_m": SPACING,
        "kind": "hub",
        "islands": [
            {"name": n, "centre": list(c), "radii": list(r), "turn": t, "biome": b}
            for n, c, r, t, b in ISLANDS
        ],
        "landmarks": landmarks(h, which),
        "routes": summary,
    }
    with open(f"{OUT}/world.json", "w") as f:
        json.dump(info, f, indent=1)
    preview(h, land, weights, [(ring, [])] + [(sp.xy, sp.bridges) for sp in spurs.values()])
    print(f"{OUT}: done", flush=True)


if __name__ == "__main__":
    main()
