"""The world plan: an island about 57 km across, so each biome holds a one to two hour ride and
the ring road is a five to six hour lap.

This is a plan, not a landscape: a coarse height map (28 m a sample), the regions, the
landforms that give the map its character, named landmarks to ride toward, the roads every
10 m with graded heights, and the rides with their length, climb and time. Unreal never loads
it whole. A stretch of it is cut as a 2 km window (`make_window.py`) and built through the
climb pipeline (`make_climb.py`, `oc.py build-level`) with that region's biome preset;
`windows` in world.json names one such stretch per biome.

Run: uv run -q --no-project --with numpy --with scipy --with pillow python -I
scripts/make_world.py. Takes about 80 s. Writes to /workspace/ue/world_gen/world:
- albedo.png: the map's colours alone (no relief or roads), for a stretch's far horizon;
- preview.png (and preview_small.jpg): the map, shaded, coloured by region, with the roads,
  the landmarks (dark dots) and a 5 km grid;
- world.json: the grid, the regions, the landmarks, each ride's length, climb, time and biome
  shares, each road's cut and fill, and a hero window per biome;
- roads.json: each road's centreline every 10 m as [x, y, z] in metres;
- height.npy: the heights in metres, float32, RES x RES, rows along Y.

Coordinates are Unreal's: X east, Y south, metres, the origin in the middle of the map. The
broad shape (the coast, the regions' blend and heights, the mesas and the canyon) is
`make_island.py`'s at K times the size. Everything else is this file's own and in world
kilometres: the landforms (`FJORD`, `SPINE`, `HORN`, `HEADLANDS`), the roads (`ROADS`), the
rides (`ROUTES`) and the summit climb (`CLIMB`).

How the pieces fit:
- `terrain` builds the ground: the island's shape, a fjord cut into the west coast, a spur
  ridge (the Spine) running south off the mountain between forest and farmland with two low
  passes, a second peak (the Horn) and two headlands.
- `lay_roads` lays each road in order. An open road's ends snap onto an earlier road and take
  its height there; that shared point is a junction, named "<road>.a" (start) or "<road>.b"
  (end). The summit climb is not drawn: `climb` walks it up the mountain in hairpins.
- `stitch` joins a ride's legs at those junctions, so legs share a point (gaps of one step).
- `audit` prints how far each road cuts or fills against the raw ground, and any road point
  over the sea, so bad stretches can be fixed by moving control points.
"""

import heapq
import json
import math
import os
import sys

import numpy as np
from PIL import Image
from scipy import ndimage

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_island as mi

K = 7.0  # world metres per island metre
RES = 2049
EXTENT = 8128.0 * K
SPACING = EXTENT / (RES - 1)
STEP = 10.0  # metres between road points
OUT = "/workspace/ue/world_gen/world"
SEED = 7
SNOW_LINE = 900.0
# Riding speed in km/h by grade, for the ride times: 30 on the flat, 11 up 7%, 50 down 5%.
FLAT_SPEED, UP_SLOW, DOWN_FAST = 30.0, 270.0, 400.0

# --- the layout -------------------------------------------------------------------------------

# Regions added to the island's: centre (island km), reach (island km). The rest are
# make_island's REGIONS, in island km (they are blended in the island's own metres).
REGIONS = {**mi.REGIONS, "heath": ((-2.0, -1.9), 0.9)}
COLOURS = {**mi.COLOURS, "heath": (150, 122, 132)}
# Metres of mid-scale relief (2 km hills) each region adds to the island's broad shape.
RELIEF = {"mountain": 160, "desert": 50, "forest": 90, "meadow": 35, "town": 15, "heath": 70}

# Landforms, in world km. The fjord: its line from the open sea to its head, and its half-width
# (m) at the mouth and at the head. It splits the forest from the heath, and the ring runs in
# along its south shore, round its head and out along its north shore.
FJORD = [(-30.0, -4.6), (-26.0, -5.0), (-22.0, -4.2), (-18.5, -5.4), (-15.0, -4.9), (-12.4, -5.6)]
FJORD_WIDTH = (1100.0, 300.0)
# The Spine: a spur ridge running south off the mountain between the forest and the farmland.
# Its line, its height (m) above the ground, its half-width (m), and its passes: where each is
# (km), its radius (m) and how much of the ridge's height it takes away.
SPINE = [(-5.5, -15.5), (-6.4, -10.0), (-6.6, -4.0), (-6.4, 1.5), (-8.0, 8.0), (-10.5, 13.5)]
SPINE_HEIGHT, SPINE_HALF = 300.0, 1500.0
SPINE_PASSES = {"Heath Gap": ((-6.2, -11.8), 1600.0, 0.8), "Spine Gap": ((-6.6, 2.6), 1500.0, 0.85)}
# The Horn: a second, lower peak in the north-east. Centre (km), radius (m), height (m).
HORN = ((9.5, -16.8), 2600.0, 620.0)
# Headlands added to the coast: base (km), tip (km), half-width (m).
HEADLANDS = [((21.0, 0.6), (26.8, 2.8), 1900.0)]

# Roads: control points in world km. Each later open road starts and ends on an earlier road
# (its ends snap to the nearest point of an earlier road within SNAP metres) and takes that
# road's height there. Comments name the places each passes.
SNAP = 2500.0
ROADS = {
    # The ring, clockwise from the harbour: west along the south coast and its beaches, a loop
    # round Gull Point, north up the west cliffs, in along the fjord and out again, over the
    # Spine at Heath Gap, under the mountain and the Horn, round Beacon Head and down the east
    # coast back to the harbour.
    "ring": [
        (10.85, 17.5),
        (6.3, 23.1),
        (3.5, 23.2),
        (0.7, 24.3),
        (-2.1, 24.4),
        (-5.6, 22.75),
        (-7.6, 22.3),
        (-8.8, 20.6),
        (-10.4, 19.5),
        (-12.4, 19.6),
        (-13.3, 21.4),
        (-13.1, 23.2),
        (-12.6, 25.0),
        (-13.6, 25.5),
        (-14.6, 24.0),
        (-15.6, 22.2),
        (-16.6, 19.6),
        (-17.85, 15.4),
        (-21.2, 9.45),
        (-23.0, 3.5),
        (-22.6, -2.0),
        (-19.0, -3.4),
        (-15.0, -3.6),
        (-12.2, -3.8),
        (-10.6, -5.6),
        (-12.2, -6.9),
        (-15.5, -6.7),
        (-17.8, -7.4),
        (-16.6, -10.6),
        (-14.0, -12.4),
        (-9.6, -11.6),
        (-5.95, -11.8),
        (-2.1, -10.6),
        (2.1, -11.9),
        (6.65, -11.0),
        (11.2, -10.2),
        (16.45, -8.05),
        (20.65, -3.5),
        (22.6, -0.6),
        (24.6, 1.2),
        (26.4, 2.5),
        (24.8, 2.7),
        (22.9, 3.2),
        (21.6, 4.8),
        (19.25, 8.4),
        (15.4, 13.3),
        (10.85, 17.5),
    ],
    # Across the middle: from the fjord head through the forest's edge, over the Spine at Spine
    # Gap, through the farmland and out across the desert to the east coast.
    "cross": [
        (-10.6, -5.6),
        (-9.6, -2.0),
        (-8.4, 1.4),
        (-6.4, 2.6),
        (-3.5, 3.9),
        (-0.7, 4.55),
        (3.0, 3.6),
        (6.3, 3.15),
        (12.6, 0.35),
        (16.5, -1.4),
        (20.65, -3.5),
    ],
    # Up from the farmland to the town.
    "valley": [(-0.7, 4.55), (2.8, 10.5), (6.65, 14.7), (10.85, 17.5)],
    # Behind the beaches: from the ring at the south-west cove along the back of the coast to
    # the valley road above the town.
    "coast_back": [(-9.0, 19.6), (-6.3, 15.75), (-2.1, 16.1), (2.8, 16.45), (6.65, 14.7)],
    # Through the farmland, from the cross road down to the coast road.
    "lane": [(-4.2, 3.64), (-3.85, 7.7), (-2.45, 11.9), (-2.1, 16.1)],
    # Through the forest, from the cross road at the foot of Spine Gap to the west coast.
    "forest": [(-8.4, 1.4), (-11.0, 4.6), (-15.0, 7.0), (-18.5, 8.0), (-21.2, 9.45)],
    # From the ring between the mountain and the Horn down past the mesas to the cross road.
    "mesa": [(6.65, -11.0), (5.95, -7.0), (6.65, -3.15), (6.3, 3.15)],
    # From the cross road through the dunes, between two mesas, down to the ring.
    "dunes": [(12.6, 0.35), (14.0, 3.5), (14.56, 6.65), (15.75, 9.1), (17.5, 10.85)],
}
CLOSED = {"ring"}
# Roads that don't follow the ground's wander (the climb is walked, not drawn).
WANDER = ((1700, 70), (2900, 110), (4600, 150))  # the road's sideways waves: wavelength, m
# The summit climb, found by `climb`: its search settings.
CLIMB = {
    "start": (2.1, -11.9),  # km, snapped onto the ring
    "grade": 0.066,  # the most the search allows without a heavy penalty
    "cell": 60.0,  # m: the search grid
    "smooth": 200.0,  # m: the mountain is smoothed this much first (the road follows its shape)
    "margin": 2500.0,  # m: the search window around the start and the summit
    "band": 650.0,  # m: the corridor's half-width; hairpins keep the road inside it
    "hairpin": 600.0,  # m of road one hairpin costs
    "leg": 6,  # cells: the least road between two hairpins
    "radius": 22.0,  # m: a hairpin's radius
    "reach": 220.0,  # m: the road ends this close to the summit
}
# Rides: each a loop of legs (road, from, to). A from/to is a junction ("<road>.a" is where
# that road starts, "<road>.b" where it ends), or "end" for the end of an open road. An open
# road runs either way; a closed one runs forward, the way its points go. A ride whose last
# leg doesn't come back to its first is ridden there and back.
ROUTES = {
    "island": [("ring", "valley.b", "valley.b")],
    "coast": [
        ("ring", "valley.b", "coast_back.a"),
        ("coast_back", "coast_back.a", "coast_back.b"),
        ("valley", "coast_back.b", "valley.b"),
    ],
    "forest": [
        ("cross", "cross.a", "forest.a"),
        ("forest", "forest.a", "forest.b"),
        ("ring", "forest.b", "cross.a"),
    ],
    "meadow": [
        ("cross", "lane.a", "valley.a"),
        ("valley", "valley.a", "coast_back.b"),
        ("coast_back", "coast_back.b", "lane.b"),
        ("lane", "lane.b", "lane.a"),
    ],
    "desert": [
        ("ring", "mesa.a", "cross.b"),
        ("cross", "cross.b", "mesa.b"),
        ("mesa", "mesa.b", "mesa.a"),
    ],
    "dunes": [
        ("cross", "dunes.a", "cross.b"),
        ("ring", "cross.b", "dunes.b"),
        ("dunes", "dunes.b", "dunes.a"),
    ],
    "summit": [("summit", "summit.a", "end")],
}
# Landmarks to ride toward: name, where (km, or None when found from the ground), what it is.
LANDMARKS = [
    ("Mount Vael", None, "the island's summit, snow-capped; the climb's hairpins end below it"),
    ("The Horn", None, "a lone second peak in the north-east, seen from the ring and the desert"),
    ("Skarfjord", (-17.0, -5.1), "a cliff-walled fjord the ring runs in along and back out"),
    ("Fjordhead", (-10.6, -5.6), "where the ring turns round the fjord and the cross road starts"),
    ("The Spine", (-6.6, -4.0), "a spur ridge off the mountain between forest and farmland"),
    ("Heath Gap", None, "the ring's pass over the north end of the Spine"),
    ("Spine Gap", None, "the cross road's pass over the Spine, from forest to farmland"),
    ("Gull Point", (-12.5, 25.4), "the south-west spit; the ring loops round its tip"),
    ("Beacon Head", (27.0, 2.8), "the east headland; the ring rounds it under a lighthouse"),
    ("Red Table", None, "the largest mesa, beside the ring under the Horn"),
    ("Dry Gorge", None, "the desert canyon, winding between the ring and the cross road"),
    ("The Harbour", (11.9, 17.5), "the town and its bay, where the ring starts and ends"),
]

# --- the ground -------------------------------------------------------------------------------


def grid():
    c = (np.arange(RES, dtype=np.float32) - (RES - 1) / 2) * SPACING
    return np.meshgrid(c, c)


def sample(a, xy, order=1):
    col = xy[:, 0] / SPACING + (RES - 1) / 2
    row = xy[:, 1] / SPACING + (RES - 1) / 2
    return ndimage.map_coordinates(a, [row, col], order=order, mode="nearest")


def along(points, x, y):
    """Distance (m) from each sample to a polyline (world km), and how far along it (0..1) the
    nearest point lies."""
    path = mi.catmull(points, False, step=SPACING * 0.5)
    s = np.linspace(0, 1, len(path), dtype=np.float32)
    col = np.clip(np.round(path[:, 0] / SPACING + (RES - 1) / 2).astype(int), 0, RES - 1)
    row = np.clip(np.round(path[:, 1] / SPACING + (RES - 1) / 2).astype(int), 0, RES - 1)
    mark = np.full(x.shape, np.nan, dtype=np.float32)
    mark[row, col] = s
    dist, (ri, ci) = ndimage.distance_transform_edt(np.isnan(mark), return_indices=True)
    return (dist * SPACING).astype(np.float32), mark[ri, ci]


def spine(x, y):
    """The Spine's height (m): a ridge with a ragged crest, fading in at the mountain and out
    at its south end, and dipping at its passes."""
    d, s = along(SPINE, x, y)
    half = SPINE_HALF * (0.8 + 0.4 * mi.fbm(x, y, 6000, 2, SEED + 101))
    body = np.exp(-((d / half) ** 2))
    crest = 0.75 + 0.5 * mi.ridged(x, y, 1800, 4, SEED + 102)
    ends = mi.smoothstep(0.0, 0.08, s) * mi.smoothstep(1.0, 0.85, s)
    for (px, py), radius, depth in SPINE_PASSES.values():
        r = np.hypot(x - px * 1000, y - py * 1000)
        ends = ends * (1 - depth * np.exp(-((r / radius) ** 2)))
    return SPINE_HEIGHT * body * crest * ends


def horn(x, y):
    (cx, cy), radius, top = HORN
    r = np.hypot(x - cx * 1000, y - cy * 1000)
    cone = np.exp(-((r / radius) ** 2)) + 0.25 * np.exp(-((r / (radius * 0.35)) ** 2))
    return top / 1.25 * cone * (0.7 + 0.45 * mi.ridged(x, y, 1500, 5, SEED + 111))


def land_mask(x, y):
    """The island's coast with the headlands added and the fjord cut out."""
    land = mi.land_mask(x / K, y / K)
    for (bx, by), (tx, ty), half in HEADLANDS:
        b, t = np.array([bx, by]) * 1000, np.array([tx, ty]) * 1000
        u = (t - b) / np.linalg.norm(t - b)
        a = (x - b[0]) * u[0] + (y - b[1]) * u[1]
        across = -(x - b[0]) * u[1] + (y - b[1]) * u[0]
        length = np.linalg.norm(t - b)
        taper = half * (1 - 0.5 * np.clip(a / length, 0, 1))
        edge = 250 * (mi.fbm(x, y, 900, 3, SEED + 121) - 0.5)
        land |= (a > -500) & (a < length) & (np.abs(across) + edge < taper)
        land |= np.hypot(x - t[0], y - t[1]) + edge < half * 0.5  # a round tip
    d, s = along(FJORD, x, y)
    width = FJORD_WIDTH[0] + (FJORD_WIDTH[1] - FJORD_WIDTH[0]) * s
    width = width * (1 + 0.4 * (mi.fbm(x, y, 1200, 3, SEED + 122) - 0.5))
    return land & ~(d < width)


def terrain(x, y):
    xi, yi = x / K, y / K  # the island's own metres
    # make_island's canyon places its path with its own grid; give it this one.
    mi.RES, mi.SPACING = RES, SPACING / K
    mi.REGIONS = REGIONS
    land = land_mask(x, y)
    inland = ndimage.distance_transform_edt(land) * SPACING
    offshore = ndimage.distance_transform_edt(~land) * SPACING
    weights = mi.region_weights(xi, yi)
    heights = mi.region_heights(xi, yi)
    heights["heath"] = 300 + 140 * mi.fbm(xi, yi, 700, 4, SEED + 71)
    hills = mi.fbm(x, y, 2200, 5, SEED + 81) - 0.5
    h = sum(weights[k] * (heights[k] + 2 * RELIEF[k] * hills) for k in weights)
    h = h + spine(x, y) + horn(x, y)
    cliff = mi.smoothstep(-1800, -2600, xi) * mi.smoothstep(2400, 1200, yi)
    cliff = cliff + mi.smoothstep(-2200, -3000, yi)
    near_fjord, _ = along(FJORD, x, y)
    cliff = cliff + mi.smoothstep(3500, 1500, near_fjord)  # the fjord's walls
    cliff = np.clip(cliff + 0.5 * (mi.fbm(xi, yi, 900, 3, SEED + 61) - 0.5), 0, 1)
    fall = 1500 + (120 - 1500) * cliff
    beach = 2.5 + 1.5 * mi.fbm(x, y, 400, 2, SEED + 62)
    h = beach + (np.maximum(h, 4) - beach) * mi.smoothstep(0, 1, inland / fall)
    sea = -1.0 - 40 * mi.smoothstep(0, 3000, offshore)
    return np.where(land, h, sea).astype(np.float32), land, inland, weights, cliff


# --- the roads --------------------------------------------------------------------------------


def even(xy, closed):
    """The line stepped every STEP metres, keeping an open line's exact last point (it may be a
    junction on another road)."""
    seg = np.linalg.norm(np.diff(xy, axis=0), axis=1)
    at = np.concatenate([[0], np.cumsum(seg)])
    out = np.stack([np.interp(np.arange(0, at[-1], STEP), at, xy[:, i]) for i in (0, 1)], axis=1)
    if not closed and np.linalg.norm(out[-1] - xy[-1]) > 0.5:
        out = np.vstack([out, xy[-1]])
    return out


def wander(xy, closed, inland):
    """The line moved sideways by a few slow waves, less near the shore and not at all at an
    open road's ends or the ring's seam, then stepped evenly again."""
    s = np.arange(len(xy)) * STEP
    t = np.gradient(xy, axis=0)
    t /= np.linalg.norm(t, axis=1, keepdims=True) + 1e-9
    rng = np.random.default_rng(SEED + len(xy))
    off = sum(a * np.sin(2 * math.pi * s / w + rng.uniform(0, 6.28)) for w, a in WANDER)
    off = off * mi.smoothstep(400, 1500, sample(inland, xy))
    off = off * mi.smoothstep(0, 1500, np.minimum(s, s[-1] - s))
    moved = xy + np.stack([-t[:, 1], t[:, 0]], axis=1) * off[:, None]
    moved[[0, -1]] = xy[[0, -1]]
    return even(moved, closed)


def rotate(v, angle):
    c, s = math.cos(angle), math.sin(angle)
    return np.array([c * v[0] - s * v[1], s * v[0] + c * v[1]])


def climb(h, start, summit):
    """The summit road (metres) from `start` up to near `summit`: an A* search over CELL-metre
    cells and 16 headings on the mountain smoothed to SMOOTH metres.

    A step costs its length, many times more where it is steeper than the target grade, a
    little more for turning, going downhill, or straying outside a corridor of half-width
    `band` around the line from the start to the summit. A step may turn at most about 45
    degrees; a hairpin is a separate move that reverses the heading in place for a fixed cost.
    Where the face is steeper than the grade the cheapest road traverses it, and the corridor
    makes it turn back in hairpins instead of running far out to the side. The state carries
    a countdown so two hairpins are at least `leg` cells apart. The search ends within `reach`
    metres of the summit."""
    c = CLIMB
    cell, grade, band, reach = c["cell"], c["grade"], c["band"], c["reach"]
    soft = ndimage.gaussian_filter(h, c["smooth"] / SPACING)
    lo = np.minimum(start, summit) - c["margin"]
    hi = np.maximum(start, summit) + c["margin"]
    cx, cy = np.arange(lo[0], hi[0], cell), np.arange(lo[1], hi[1], cell)
    gx, gy = np.meshgrid(cx, cy)
    g = sample(soft, np.stack([gx.ravel(), gy.ravel()], axis=1)).reshape(gx.shape)
    rows, cols = g.shape
    axis = (summit - start) / np.linalg.norm(summit - start)
    lateral = np.abs(-(gx - start[0]) * axis[1] + (gy - start[1]) * axis[0])
    outside = 1 + 4 * mi.smoothstep(band, band + 400, lateral)
    goal_z = float(sample(soft, summit[None])[0])
    near_top = np.hypot(gx - summit[0], gy - summit[1]) < reach
    steps = [
        (dr, dc)
        for dr in range(-2, 3)
        for dc in range(-2, 3)
        if (dr, dc) != (0, 0) and math.gcd(abs(dr), abs(dc)) == 1
    ]
    angle = [math.atan2(dr, dc) for dr, dc in steps]
    turn = [[abs((b - a + math.pi) % (2 * math.pi) - math.pi) for b in angle] for a in angle]
    onward = [[e for e in range(16) if turn[d][e] < math.radians(46)] for d in range(16)]
    back = [[e for e in range(16) if turn[d][e] > math.radians(134)] for d in range(16)]

    def guess(r, cc):  # a lower bound on the road left: the distance, or the rise at the grade
        dist = math.hypot(gx[r, cc] - summit[0], gy[r, cc] - summit[1]) - reach
        return max(dist, (goal_z - g[r, cc]) / grade, 0.0)

    r0 = round((start[1] - cy[0]) / cell)
    c0 = round((start[0] - cx[0]) / cell)
    best = np.full((rows, cols, 16, c["leg"] + 1), np.inf)
    came, todo = {}, []
    for d in range(16):
        best[r0, c0, d, 0] = 0.0
        heapq.heappush(todo, (guess(r0, c0), 0.0, (r0, c0, d, 0)))
    end = None
    while todo:
        _, cost, state = heapq.heappop(todo)
        r, cc, d, wait = state
        if cost > best[state]:
            continue
        if near_top[r, cc]:
            end = state
            break
        moves = [(r, cc, e, c["hairpin"], c["leg"]) for e in back[d]] if wait == 0 else []
        for e in onward[d]:
            dr, dc = steps[e]
            nr, nc = r + dr, cc + dc
            if not (0 <= nr < rows and 0 <= nc < cols) or g[nr, nc] < 15:
                continue
            length = cell * math.hypot(dr, dc)
            rise = (g[nr, nc] - g[r, cc]) / length
            over = max(0.0, abs(rise) - grade) / grade
            step = length * outside[nr, nc] * (1 + 40 * over + 400 * over * over)
            step += length * 2 * max(0.0, -rise) / grade + 20 * (turn[d][e] / 0.8) ** 2
            moves.append((nr, nc, e, step, max(wait - 1, 0)))
        for nr, nc, e, step, left in moves:
            nxt = (nr, nc, e, left)
            if cost + step < best[nxt]:
                best[nxt] = cost + step
                came[nxt] = state
                heapq.heappush(todo, (cost + step + guess(nr, nc), cost + step, nxt))
    path = [end]
    while path[-1] in came:
        path.append(came[path[-1]])
    path.reverse()
    # The cells as a polyline; a hairpin (a reversal in place) becomes an arc of `radius`
    # turning through the uphill side.
    pts, hairpins = [], 0
    for i, (r, cc, d, _) in enumerate(path):
        p = np.array([cx[cc], cy[r]])
        if i and path[i - 1][:2] == (r, cc):
            u = np.array(steps[path[i - 1][2]][::-1], dtype=float)
            v = np.array(steps[d][::-1], dtype=float)
            u, v = u / np.linalg.norm(u), v / np.linalg.norm(v)
            side = 1.0 if u[0] * v[1] - u[1] * v[0] > 0 else -1.0
            q, heading = pts[-1].copy(), u
            for _ in range(int(math.pi * c["radius"] / 5)):
                if np.dot(heading, v) > math.cos(5 / c["radius"]):
                    break
                heading = rotate(heading, side * 5 / c["radius"])
                q = q + 5 * heading
                pts.append(q.copy())
            hairpins += 1
            continue
        pts.append(p)
    pts[0] = start
    pts = np.array(pts)
    print(f"climb: {hairpins} hairpins, {len(path)} states, top within {reach:.0f} m of summit")
    return pts


def summit_point(h, weights):
    """The mountain's highest sample (m) and its height."""
    k = np.argmax(np.where(weights["mountain"] > 0.5, h, -1e9))
    r, c = np.unravel_index(k, h.shape)
    return np.array([(c - (RES - 1) / 2) * SPACING, (r - (RES - 1) / 2) * SPACING]), float(h[r, c])


def lay_roads(h, inland, weights):
    """Each road every STEP metres with a graded height. An open road's ends snap onto the
    nearest earlier road (within SNAP) and take its height there. Returns the roads and the
    junctions: "<road>.a" / "<road>.b", the exact point each road starts and ends at."""
    roads, junctions = {}, {}

    def snap(point):
        best = None
        for other in roads.values():
            d = np.linalg.norm(other["xy"] - point, axis=1)
            k = int(np.argmin(d))
            if d[k] < SNAP and (best is None or d[k] < best[0]):
                best = (d[k], other["xy"][k], float(other["z"][k]))
        return best

    for name, points in [*ROADS.items(), ("summit", None)]:
        closed = name in CLOSED
        ends = {}
        if points is None:  # the summit climb
            _, at, ends[0] = snap(np.array(CLIMB["start"]) * 1000)
            top, _ = summit_point(h, weights)
            xy = even(mi.catmull(climb(h, at, top) / 1000, False, step=STEP / 2), False)
            xy[0] = at
            soft = ndimage.gaussian_filter(h, 100 / SPACING)
            ends[-1] = float(sample(soft, top[None])[0])
            z = mi.smooth(sample(h, xy), 10, False)
        else:
            points = [np.array(p) * 1000 for p in points]
            for end in (0, -1) if not closed else ():
                found = snap(points[end])
                if found:
                    _, points[end], ends[end] = found
            xy = mi.catmull([p / 1000 for p in points], closed, step=STEP)
            if not closed:
                xy = np.vstack([xy, points[-1]])
            xy = wander(even(xy, closed), closed, inland)
            z = mi.smooth(sample(h, xy), 30, closed)
        z = np.maximum(z, 3.0)
        fixed = {(len(z) - 1 if end else 0): v / STEP for end, v in ends.items()}
        z = mi.grade_fit(z / STEP, fixed, closed) * STEP
        roads[name] = {"xy": xy, "z": z, "closed": closed}
        junctions[f"{name}.a"], junctions[f"{name}.b"] = xy[0], xy[-1]
    return roads, junctions


def stitch(roads, junctions):
    """Each ride's centreline [x, y, z] (m), from its legs, joined at shared junctions."""
    rides = {}
    for name, legs in ROUTES.items():
        parts = []
        for road_name, a, b in legs:
            road = roads[road_name]
            xy, z = road["xy"], road["z"]
            ia = int(np.argmin(np.linalg.norm(xy - junctions[a], axis=1)))
            ib = (
                len(xy) - 1
                if b == "end"
                else int(np.argmin(np.linalg.norm(xy - junctions[b], axis=1)))
            )
            if road["closed"]:
                idx = (ia + np.arange((ib - ia) % len(xy) or len(xy))) % len(xy)
            else:
                idx = np.arange(ia, ib + 1) if ia <= ib else np.arange(ia, ib - 1, -1)
            part = np.column_stack([xy[idx], z[idx]])
            if parts and np.linalg.norm(parts[-1][-1, :2] - part[0, :2]) < 0.5:
                part = part[1:]  # the junction, once
            parts.append(part)
        line = np.concatenate(parts)
        if np.linalg.norm(line[0, :2] - line[-1, :2]) > 50:
            line = np.concatenate([line, line[::-1][1:]])  # there and back
        rides[name] = line
        gaps = np.linalg.norm(np.diff(line[:, :2], axis=0), axis=1)
        print(f"ride {name}: {gaps.sum() / 1000:.1f} km, widest gap {gaps.max():.0f} m")
    return rides


def audit(roads, h, land):
    """How far each road sits off the raw ground (cut where it is below, fill above), and any
    point over the sea. Prints the worst place on each road so it can be moved."""
    out = {}
    wet_map = (~land).astype(np.float32)
    for name, road in roads.items():
        xy, z = road["xy"], road["z"]
        dz = z - sample(h, xy)
        k = int(np.argmax(np.abs(dz)))
        wet = sample(wet_map, xy) > 0.5
        out[name] = {
            "max_m": round(float(np.abs(dz).max()), 1),
            "mean_m": round(float(np.abs(dz).mean()), 1),
            "over_100m_km": round(float((np.abs(dz) > 100).sum() * STEP / 1000), 1),
            "worst_at_km": [round(float(v) / 1000, 2) for v in xy[k]],
            "over_sea_points": int(wet.sum()),
        }
        o = out[name]
        print(
            f"road {name}: {len(xy) * STEP / 1000:.1f} km, |road - ground| max {o['max_m']} m"
            f" ({'fill' if dz[k] > 0 else 'cut'} at {o['worst_at_km']} km), mean {o['mean_m']} m,"
            f" {o['over_100m_km']} km over 100 m, {o['over_sea_points']} points over the sea"
        )
    return out


def hours(line):
    d = np.linalg.norm(np.diff(line[:, :2], axis=0), axis=1)
    grade = np.diff(line[:, 2]) / np.maximum(d, 1e-6)
    speed = np.where(grade > 0, FLAT_SPEED - UP_SLOW * grade, FLAT_SPEED - DOWN_FAST * grade)
    return float((d / 1000 / np.clip(speed, 9, 55)).sum())


def shares(line, weights, inland):
    """The share of a ride in each biome: the strongest region, or coast within 1.2 km of sea."""
    names = list(weights)
    w = np.stack([sample(weights[n], line[:, :2]) for n in names])
    best = np.array(names)[np.argmax(w, axis=0)]
    best = np.where(sample(inland, line[:, :2]) < 1200, "coast", best)
    return {n: round(float((best == n).mean()), 2) for n in [*names, "coast"] if (best == n).any()}


def windows(rides, weights, inland):
    """One 2 km stretch per biome to build first: where a ride is deepest in that biome (and,
    for all but the coast, at least 3 km from the sea, so the window is that biome alone)."""
    out = {}
    for biome in [*weights, "coast"]:
        top = (-1.0, None)
        for name, line in rides.items():
            if biome == "coast":
                score = 1 - sample(inland, line[:, :2]) / 1200
            else:
                score = sample(weights[biome], line[:, :2])
                score = score * mi.smoothstep(1500, 3000, sample(inland, line[:, :2]))
            k = int(np.argmax(score))
            if score[k] > top[0]:
                top = (float(score[k]), (name, k, line[k]))
        name, k, p = top[1]
        out[biome] = {
            "ride": name,
            "at_km": round(k * STEP / 1000, 1),
            "centre": [round(float(v)) for v in p],
        }
    return out


def landmarks(h, weights):
    """LANDMARKS with their positions (m) and ground heights filled in."""
    found = {
        "Mount Vael": summit_point(h, weights)[0],
        **{name: np.array(p) * 1000 for name, (p, _, _) in SPINE_PASSES.items()},
        "Red Table": np.array(mi.MESAS[1][0]) * 1000 * K,  # make_island's biggest mesa
        "Dry Gorge": np.array(mi.CANYON[len(mi.CANYON) // 2]) * 1000 * K,
    }
    (cx, cy), radius, _ = HORN
    x, y = grid()
    near = np.hypot(x - cx * 1000, y - cy * 1000) < radius
    r, c = np.unravel_index(np.argmax(np.where(near, h, -1e9)), h.shape)
    found["The Horn"] = np.array([x[r, c], y[r, c]])
    out = []
    for name, at, what in LANDMARKS:
        p = found[name] if at is None else np.array(at) * 1000
        z = float(sample(h, np.array([p]))[0])
        xyz = [round(float(p[0])), round(float(p[1])), round(z)]
        out.append({"name": name, "xyz": xyz, "what": what})
    return out


def preview(h, land, weights, roads, cliff, marks):
    gy, gx = np.gradient(h, SPACING)
    light = np.array([-0.6, -0.6, 0.55])
    light /= np.linalg.norm(light)
    n = np.stack([-gx * 4, -gy * 4, np.ones_like(h)])  # relief exaggerated to read at 28 m
    shade = np.clip((n * light[:, None, None]).sum(0) / np.linalg.norm(n, axis=0), 0, 1)
    rgb = np.zeros((*h.shape, 3), dtype=np.float32)
    for name, w in weights.items():
        rgb += w[..., None] * np.array(COLOURS[name], dtype=np.float32)
    snow = mi.smoothstep(SNOW_LINE - 100, SNOW_LINE + 50, h)[..., None]
    rgb = rgb * (1 - snow) + 245 * snow
    beach = (land & (h < 5.5) & (cliff < 0.5))[..., None]
    rgb = np.where(beach, np.array(COLOURS["beach"], dtype=np.float32), rgb)
    # The colours without relief, roads or marks: a stretch's far horizon is coloured from it.
    albedo = np.where(land[..., None], rgb, np.array(COLOURS["sea"], dtype=np.float32))
    Image.fromarray(np.clip(albedo, 0, 255).astype(np.uint8)).save(f"{OUT}/albedo.png")
    rgb *= (0.35 + 0.8 * shade)[..., None]
    sea = (
        np.array(COLOURS["sea"], dtype=np.float32)
        * (1.2 - 0.6 * mi.smoothstep(0, -30, h))[..., None]
    )
    rgb = np.where(land[..., None], rgb, sea)

    def cells(xy):
        col = np.round(xy[:, 0] / SPACING + (RES - 1) / 2).astype(int)
        row = np.round(xy[:, 1] / SPACING + (RES - 1) / 2).astype(int)
        return np.clip(row, 0, RES - 1), np.clip(col, 0, RES - 1)

    on = np.zeros(h.shape, dtype=bool)
    for road in roads.values():
        on[cells(road["xy"])] = True
    rgb = np.where(ndimage.binary_dilation(on)[..., None], 250.0, rgb)
    dots = np.zeros(h.shape, dtype=bool)
    dots[cells(np.array([m["xyz"][:2] for m in marks], dtype=float))] = True
    dots = ndimage.binary_dilation(dots, iterations=5)
    rgb = np.where(dots[..., None], np.array([150.0, 20.0, 20.0]), rgb)
    c = (np.arange(RES) - (RES - 1) / 2) * SPACING
    lines = np.abs(c - np.round(c / 5000) * 5000) < SPACING / 2
    rgb[lines, :, :] = rgb[lines, :, :] * 0.6 + 40
    rgb[:, lines, :] = rgb[:, lines, :] * 0.6 + 40
    image = Image.fromarray(np.clip(rgb, 0, 255).astype(np.uint8))
    image.save(f"{OUT}/preview.png")
    image.resize((1024, 1024), Image.LANCZOS).save(f"{OUT}/preview_small.jpg", quality=88)


def ride_info(line, weights, inland):
    d = np.linalg.norm(np.diff(line[:, :2], axis=0), axis=1)
    grade = np.diff(line[:, 2]) / np.maximum(d, 1e-6)
    return {
        "km": round(float(d.sum()) / 1000, 1),
        "climb_m": round(float(np.clip(np.diff(line[:, 2]), 0, None).sum())),
        "top_m": round(float(line[:, 2].max())),
        "steepest": round(float(np.abs(grade[d > 1]).max()), 3),
        "widest_gap_m": round(float(d.max()), 1),
        "hours": round(hours(line), 2),
        "biomes": shares(line, weights, inland),
    }


def main():
    os.makedirs(OUT, exist_ok=True)
    x, y = grid()
    h, land, inland, weights, cliff = terrain(x, y)
    del x, y
    roads, junctions = lay_roads(h, inland, weights)
    rides = stitch(roads, junctions)
    marks = landmarks(h, weights)
    info = {
        "extent_m": EXTENT,
        "res": RES,
        "spacing_m": SPACING,
        "scale": K,
        "regions": {
            n: {"centre_km": [c[0] * K, c[1] * K], "reach_km": r * K}
            for n, (c, r) in REGIONS.items()
        },
        "landmarks": marks,
        "rides": {name: ride_info(line, weights, inland) for name, line in rides.items()},
        "roads": audit(roads, h, land),
        "windows": windows(rides, weights, inland),
    }
    preview(h, land, weights, roads, cliff, marks)
    np.save(f"{OUT}/height.npy", h)
    for name, ride in info["rides"].items():
        print(name, ride)
    with open(f"{OUT}/world.json", "w") as f:
        json.dump(info, f, indent=1)
    with open(f"{OUT}/roads.json", "w") as f:
        json.dump(
            {n: np.column_stack([r["xy"], r["z"]]).round(1).tolist() for n, r in roads.items()}, f
        )
    print(
        f"{OUT}: {EXTENT / 1000:.0f} km at {SPACING:.0f} m, heights {h.min():.0f}..{h.max():.0f} m"
    )
    print("windows", info["windows"])


if __name__ == "__main__":
    main()
