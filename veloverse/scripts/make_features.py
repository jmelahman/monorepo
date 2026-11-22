"""The islands' landmarks, from make_world_level.py's output: rock formations by biome, wind
turbines on The Moor, power lines, the harbour (piers, boats, a quay and a lighthouse) and the
bridges. Writes, into the ride folder (OC_SRC, default /workspace/ue/world_gen/ride_hub):

- feat_<name>.gltf: the meshes made here. Kit pieces (a turbine, a pylon, a boat) sit at their
  own origin and are placed many times; the bridges and the cables are built where they stand,
  relative to an origin given with them.
- features.json, which climb_world.py's features step places:
    {"paints": {slot: [r, g, b, roughness, metallic]},   the glTF slots' colours
     "meshes": {key: "gltf:<name>" | "/Game/...asset"},
     "groups": {group: {"cull": cm or 0, "shadows": bool}},
     "items": [[group, key, x, y, z, yaw, scale, pitch, roll]],   cm and degrees; the scale a
                                                                   number or [x, y, z]
     "spin": [[key, x, y, z, yaw, degrees a second, phase]]}       the turbines' rotors

Run after make_world_level.py (it reads height.r16, info.json and routes.json there, and the plan's
weights.npz and world.json):
    uv run --no-project --with numpy --with scipy --with pillow python -I scripts/make_features.py
It also draws features.png, the placements over the ground, to check by eye.
"""

import json
import math
import os
import shutil
import sys

import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage
from scipy.spatial import cKDTree

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import itertools

import gltf_writer

PLAN = os.environ.get("OC_PLAN", "/workspace/ue/world_gen/hub")
SRC = os.environ.get("OC_SRC", "/workspace/ue/world_gen/ride_hub")
SEED = 23
MS = "/Game/Megascans"
ROAD_HALF = 3.2  # m, make_island.ROAD_HALF: the bridge deck's road
PAINTS = {
    "concrete": [0.42, 0.41, 0.38, 0.85, 0.0],
    "white": [0.78, 0.79, 0.78, 0.45, 0.0],
    "steel": [0.36, 0.37, 0.38, 0.5, 0.8],
    "cable": [0.06, 0.06, 0.06, 0.6, 0.3],
    "wood": [0.2, 0.14, 0.09, 0.9, 0.0],
    "red": [0.45, 0.04, 0.03, 0.55, 0.0],
    "navy": [0.03, 0.06, 0.16, 0.5, 0.0],
    "green": [0.04, 0.16, 0.08, 0.55, 0.0],
    "glass": [0.05, 0.07, 0.08, 0.1, 0.0],
    "warm_glass": [0.95, 0.68, 0.32, 0.15, 0.0],
    "copper": [0.18, 0.34, 0.28, 0.42, 0.72],
    "deck": [0.32, 0.25, 0.17, 0.85, 0.0],
}


def ms(biome, slug):
    return f"{MS}/{biome}/{slug}/SM_{slug}"


# Rocks by biome: (Megascans key, its size m (index.json max_size), scale range, nearest to the
# road m, how many per km of road, wants steep ground, wants high ground).
ROCKS = {
    "mountain": [
        (ms("mountain", "massive_tundra_rock_formation"), 26.0, (0.5, 1.1), 90, 2.0, 1, 1),
        (ms("mountain", "sharp_cliff"), 19.0, (0.6, 1.0), 70, 1.2, 1, 0),
        (ms("mountain", "tundra_boulder"), 2.9, (0.8, 1.8), 12, 6.0, 0, 0),
        (ms("mountain", "limestone_rocks"), 2.3, (1.0, 2.4), 10, 6.0, 0, 0),
    ],
    "heath": [  # the moor's tors: tundra outcrops cut down, and boulders strewn about them
        (ms("mountain", "massive_tundra_rock_formation"), 26.0, (0.22, 0.42), 60, 2.5, 0, 1),
        (ms("mountain", "tundra_boulder"), 2.9, (0.7, 1.5), 12, 7.0, 0, 1),
        (ms("mountain", "icelandic_rock_plates"), 6.4, (0.6, 1.2), 25, 2.0, 0, 0),
    ],
    "forest": [
        (ms("forest", "mossy_rocks"), 3.0, (1.0, 2.2), 10, 5.0, 0, 0),
        (ms("forest", "nordic_forest_cluster_rock_medium"), 2.6, (1.0, 2.0), 10, 4.0, 0, 0),
        (ms("forest", "nordic_forest_cliff_large"), 13.3, (0.6, 1.0), 50, 0.8, 1, 0),
    ],
    "desert": [
        # Gap 6 (wf_70000, v59): at 30-58 m the mesas read as boulders and the 1.5-3.5 m
        # boulders, six a km, as bricks on the sand. Mesas now stand 60-130 m across, 350 m out.
        (ms("desert", "huge_canyon_sandstone_mesa"), 16.6, (3.6, 7.8), 350, 1.2, 0, 0),
        (ms("desert", "canyon_sandstone_hoodoo"), 20.9, (0.8, 1.6), 70, 1.2, 0, 0),
        (ms("desert", "desert_western_spire_layered_xl_04"), 27.9, (0.5, 0.9), 80, 0.8, 0, 0),
        (ms("desert", "sandstone_rock_formation"), 3.6, (1.6, 3.6), 14, 4.0, 0, 0),
        (ms("desert", "canyon_sandstone_boulder"), 1.0, (2.0, 4.0), 10, 2.5, 0, 0),
        (ms("desert", "desert_western_cluster_rock_medium_05"), 3.0, (1.0, 2.0), 10, 3.0, 0, 0),
    ],
    "meadow": [
        (ms("forest", "mossy_rocks"), 3.0, (0.8, 1.6), 15, 1.0, 0, 0),
    ],
    "coast": [  # every island's shore
        (ms("coast", "nordic_beach_rock_formation"), 7.1, (0.7, 1.5), 25, 2.0, 0, 0),
        (ms("coast", "nordic_beach_rocks"), 3.6, (0.8, 1.8), 12, 3.0, 0, 0),
    ],
}


# --- the ground -------------------------------------------------------------------------


class Ground:
    """The carved heights (height.r16), sampled in world metres."""

    def __init__(self):
        with open(f"{SRC}/info.json") as f:
            info = json.load(f)
        n = info["size"]
        self.spacing = info["spacing"] / 100.0
        q = np.fromfile(f"{SRC}/height.r16", dtype="<u2").reshape(n, n)
        self.h = ((q.astype(np.float32) - 32768) * info["z_scale"] / 128 / 100).astype(np.float32)
        self.n = n
        # Shore distance on land, and the slope.
        land = self.h > 0.3
        self.inland = (ndimage.distance_transform_edt(land) * self.spacing).astype(np.float32)
        self.sea = (ndimage.distance_transform_edt(~land) * self.spacing).astype(np.float32)
        gy, gx = np.gradient(self.h, self.spacing)
        self.slope = np.hypot(gx, gy).astype(np.float32)
        # How far a point stands above the ground 500 m round it: knolls and ridges.
        self.prominence = self.h - ndimage.uniform_filter(self.h, int(500 / self.spacing))

    def at(self, a, x, y, order=1):
        """`a` at (x, y) m: an array for arrays, a float for numbers."""
        x, y = np.asarray(x, float), np.asarray(y, float)
        col = np.atleast_1d(x / self.spacing + (self.n - 1) / 2)
        row = np.atleast_1d(y / self.spacing + (self.n - 1) / 2)
        v = ndimage.map_coordinates(a, [row, col], order=order, mode="nearest")
        return v.reshape(x.shape) if x.ndim else float(v[0])

    def z(self, x, y):
        return self.at(self.h, x, y)


class Biomes:
    def __init__(self):
        with open(f"{PLAN}/world.json") as f:
            self.world = json.load(f)
        with np.load(f"{PLAN}/weights.npz") as f:
            self.w = {k: f[k].astype(np.float32) for k in f.files}
        self.n = next(iter(self.w.values())).shape[0]
        self.spacing = self.world["extent_m"] / (self.n - 1)

    def best(self, x, y):
        scalar = np.ndim(x) == 0
        col = np.atleast_1d(np.asarray(x) / self.spacing + (self.n - 1) / 2)
        row = np.atleast_1d(np.asarray(y) / self.spacing + (self.n - 1) / 2)
        keys = list(self.w)
        stack = np.stack(
            [ndimage.map_coordinates(self.w[k], [row, col], order=1, mode="nearest") for k in keys]
        )
        best = np.array(keys)[np.argmax(stack, axis=0)]
        return str(best[0]) if scalar else best


# --- geometry ---------------------------------------------------------------------------


def _sub(a, b):
    return (a[0] - b[0], a[1] - b[1], a[2] - b[2])


def _cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def _dot(a, b):
    return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]


def face(part, corners, outward):
    """A flat quad (or triangle) facing `outward`, its own vertices so it shades flat."""
    a, b, c = corners[:3]
    n = _cross(_sub(b, a), _sub(c, a))
    if _dot(n, outward) < 0:
        corners = corners[::-1]
        n = (-n[0], -n[1], -n[2])
    ids = [part.vertex(p, n, (p[0] / 200 + p[2] / 200, p[1] / 200 + p[2] / 200)) for p in corners]
    if len(ids) == 4:
        part.quad(*ids)
    else:
        part.triangle(*ids)


def tube(part, path, radius, sides=6, cap=True):
    """A tube (cm) along `path`, `radius` a number or one per point."""
    radii = radius if isinstance(radius, (list, tuple)) else [radius] * len(path)
    rings = []
    for i, p in enumerate(path):
        a = path[max(i - 1, 0)]
        b = path[min(i + 1, len(path) - 1)]
        t = np.subtract(b, a).astype(float)
        t /= np.linalg.norm(t) or 1.0
        up = np.array([0.0, 0.0, 1.0]) if abs(t[2]) < 0.9 else np.array([1.0, 0.0, 0.0])
        u = np.cross(t, up)
        u /= np.linalg.norm(u)
        v = np.cross(t, u)
        rings.append(
            [
                tuple(
                    np.add(p, radii[i] * (math.cos(k * 2 * math.pi / sides) * u))
                    + radii[i] * math.sin(k * 2 * math.pi / sides) * v
                )
                for k in range(sides)
            ]
        )
    for i in range(len(path) - 1):
        centre = np.add(path[i], path[i + 1]) / 2
        for k in range(sides):
            k2 = (k + 1) % sides
            quad = [rings[i][k], rings[i][k2], rings[i + 1][k2], rings[i + 1][k]]
            mid = np.mean(quad, axis=0)
            face(part, quad, tuple(mid - centre))
    if cap:
        for i, sign in ((0, -1), (len(path) - 1, 1)):  # sign: the neighbour is i - sign
            out = tuple(
                np.subtract(path[i], path[i - sign]) if i else np.subtract(path[0], path[1])
            )
            ring = rings[i]
            for k in range(1, sides - 1):
                face(part, [ring[0], ring[k], ring[k + 1]], out)


def beam(part, a, b, width):
    """A square-ish bar from a to b (cm)."""
    tube(part, [a, b], width / 2 * math.sqrt(2), sides=4)


def sweep(part, path, profile, closed=False):
    """A cross-section swept along a path (cm), kept upright: `profile` is [(across, up)] in cm,
    across to the path's right. Faces point away from the profile's middle."""
    mid = np.mean(profile, axis=0)
    frames = []
    for i, p in enumerate(path):
        a = path[max(i - 1, 0)]
        b = path[min(i + 1, len(path) - 1)]
        dx, dy = b[0] - a[0], b[1] - a[1]
        length = math.hypot(dx, dy) or 1.0
        right = (-dy / length, dx / length)
        frames.append(
            [(p[0] + right[0] * s, p[1] + right[1] * s, p[2] + u) for s, u in profile]
            + [(p[0] + right[0] * mid[0], p[1] + right[1] * mid[0], p[2] + mid[1])]
        )
    m = len(profile)
    edges = range(m) if closed else range(m - 1)
    for i in range(len(path) - 1):
        for k in edges:
            k2 = (k + 1) % m
            quad = [frames[i][k], frames[i][k2], frames[i + 1][k2], frames[i + 1][k]]
            centre = np.add(frames[i][m], frames[i + 1][m]) / 2
            face(part, quad, tuple(np.mean(quad, axis=0) - centre))


def box(part, centre, size, yaw=0.0):
    """A box (cm), yawed about its centre."""
    cx, cy, cz = centre
    hx, hy, hz = (s / 2 for s in size)
    c, s = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))

    def at(x, y, z):
        return (cx + x * c - y * s, cy + x * s + y * c, cz + z)

    corners = [at(x, y, z) for z in (-hz, hz) for y in (-hy, hy) for x in (-hx, hx)]
    for axis, sign in ((0, -1), (0, 1), (1, -1), (1, 1), (2, -1), (2, 1)):
        pick = [p for i, p in enumerate(corners) if ((i >> axis) & 1) == (sign > 0)]
        quad = [pick[0], pick[1], pick[3], pick[2]]
        face(part, quad, tuple(np.mean(quad, axis=0) - np.array(centre)))


# --- the kit ----------------------------------------------------------------------------

HUB = 7400.0  # cm: the turbine's hub height
BLADE = 3800.0  # cm


def turbine_tower():
    mesh = gltf_writer.Mesh()
    white = mesh.part("white")
    steps = [i / 8 for i in range(9)]
    tube(white, [(0, 0, HUB * t - 50) for t in steps], [210 - 70 * t for t in steps], sides=16)
    box(white, (150, 0, HUB + 50), (1000, 360, 360))  # the nacelle, its rotor end at +X
    box(white, (-300, 0, HUB + 50), (300, 320, 330))
    return mesh


def turbine_rotor():
    """Three blades about +X, the hub at the origin; spun about X (roll) in the game."""
    mesh = gltf_writer.Mesh()
    white = mesh.part("white")
    tube(white, [(-120, 0, 0), (60, 0, 0), (200, 0, 0)], [150, 150, 20], sides=12, cap=True)
    for k in range(3):
        a = math.radians(k * 120)
        d = (0.0, math.cos(a), math.sin(a))
        side = (0.0, -math.sin(a), math.cos(a))
        # A flat, tapering blade: wide near the root, its leading edge along `side`.
        rows = [(r, w) for r, w in ((150, 160), (700, 260), (2000, 170), (BLADE, 30))]
        top, bottom = [], []
        for r, w in rows:
            base = np.multiply(d, r)
            lead = np.multiply(side, w * 0.35)
            trail = np.multiply(side, -w * 0.65)
            tilt = np.array([18.0 * (1 - r / BLADE) + 6, 0, 0])
            top.append((tuple(base + lead + tilt), tuple(base + trail - tilt * 0.3)))
            bottom.append((tuple(base + lead - tilt * 0.2), tuple(base + trail - tilt)))
        for i in range(len(rows) - 1):
            (a1, b1), (a2, b2) = top[i], top[i + 1]
            face(white, [a1, b1, b2, a2], (1, 0, 0))
            (c1, e1), (c2, e2) = bottom[i], bottom[i + 1]
            face(white, [c1, e1, e2, c2], (-1, 0, 0))
            face(white, [a1, a2, c2, c1], tuple(side))
            face(white, [b1, b2, e2, e1], tuple(-np.array(side)))
    return mesh


# The pylon's conductor points: (across m, up m), on both sides; and the earth wire on top.
PYLON_ARMS = [(-7.0, 25.0), (-5.5, 30.0), (5.5, 30.0), (7.0, 25.0)]
PYLON_TOP = (0.0, 36.0)


def pylon():
    """A lattice tower, the line along local X: four legs leaning in, cross bracing, two arms."""
    mesh = gltf_writer.Mesh()
    steel = mesh.part("steel")

    def half(z):  # the legs' half spread at height z m
        return 3.2 - 2.4 * min(z, 30.0) / 30.0 if z < 30 else 0.8

    levels = [0, 5, 10, 15, 20, 25, 30, 34]
    for sx in (-1, 1):
        for sy in (-1, 1):
            beam(
                steel,
                *[(sx * half(z) * 100, sy * half(z) * 100, z * 100) for z in (0, 30)],
                22,
            )
            beam(steel, (sx * 80, sy * 80, 3000), (sx * 40, sy * 40, 3600), 16)
    for z0, z1 in itertools.pairwise(levels):
        h0, h1 = half(z0) * 100, half(z1) * 100
        for sx, sy, ex, ey in ((-1, -1, 1, -1), (1, -1, 1, 1), (1, 1, -1, 1), (-1, 1, -1, -1)):
            beam(steel, (sx * h0, sy * h0, z0 * 100), (ex * h1, ey * h1, z1 * 100), 9)
            beam(steel, (ex * h0, ey * h0, z0 * 100), (sx * h1, sy * h1, z1 * 100), 9)
        beam(steel, (-h1, -h1, z1 * 100), (h1, -h1, z1 * 100), 9)
        beam(steel, (-h1, h1, z1 * 100), (h1, h1, z1 * 100), 9)
    for across, up in PYLON_ARMS:
        root = half(up) * 100 * (1 if across > 0 else -1)
        for sx in (-1, 1):
            beam(steel, (sx * 70, root, up * 100), (sx * 20, across * 100, up * 100), 14)
            beam(steel, (sx * 60, root, up * 100 + 260), (sx * 10, across * 100, up * 100), 9)
        tube(steel, [(0, across * 100, up * 100), (0, across * 100, up * 100 - 220)], 9, 6)
    return mesh


BOATS = {
    # name: (length m, beam m, hull paint, cabin, mast m)
    "sloop": (11.0, 3.6, "white", (3.5, 2.4, 1.1), 15.0),
    "launch": (8.0, 2.8, "navy", (2.6, 2.0, 1.6), 0.0),
    "trawler": (17.0, 5.4, "red", (5.0, 3.8, 2.6), 7.0),
    "dinghy": (4.5, 1.8, "green", None, 0.0),
}


def boat(length, beam_m, paint, cabin, mast):
    """A hull pointing along +X: lofted from a few sections, its waterline at z 0."""
    mesh = gltf_writer.Mesh()
    hull, white, wood = mesh.part(paint), mesh.part("white"), mesh.part("deck")
    length, half = length * 100, beam_m * 50
    freeboard, draft = 0.12 * length**0.75 + 40, 0.07 * length**0.8 + 20
    stations = []
    for t in np.linspace(0, 1, 9):
        x = (t - 0.5) * length
        width = half * (1 - (2 * t - 1) ** 2 * 0.15 - max(0.0, t - 0.6) ** 2 * 5.5) if t < 1 else 6
        width = max(width, 6)
        rise = 25 * (t - 0.5) ** 2 * 4 + (40 if t > 0.7 else 0) * (t - 0.7) / 0.3
        depth = draft * (1 - (2 * t - 1) ** 4 * 0.5)
        stations.append(
            [
                (x, -width, freeboard + rise),
                (x, -width * 0.92, 0),
                (x, -width * 0.5, -depth * 0.85),
                (x, 0, -depth),
                (x, width * 0.5, -depth * 0.85),
                (x, width * 0.92, 0),
                (x, width, freeboard + rise),
            ]
        )
    for i in range(len(stations) - 1):
        a, b = stations[i], stations[i + 1]
        for k in range(len(a) - 1):
            quad = [a[k], a[k + 1], b[k + 1], b[k]]
            mid = np.mean(quad, axis=0)
            face(hull, quad, (0, mid[1], mid[2] + 10 if k in (2, 3) else 0))
        face(wood, [a[0], b[0], b[-1], a[-1]], (0, 0, 1))
    stern = stations[0]
    for k in range(1, len(stern) - 1):
        face(hull, [stern[0], stern[k], stern[k + 1]], (-1, 0, 0))
    deck_z = freeboard
    if cabin:
        cl, cw, ch = (v * 100 for v in cabin)
        box(white, (-length * 0.08, 0, deck_z + ch / 2), (cl, cw, ch))
        box(
            mesh.part("glass"),
            (-length * 0.08 + cl / 2 + 1, 0, deck_z + ch * 0.7),
            (4, cw * 0.8, ch * 0.3),
        )
    if mast:
        tube(white, [(length * 0.08, 0, deck_z), (length * 0.08, 0, deck_z + mast * 100)], 9, 6)
        tube(white, [(length * 0.08, 0, deck_z + 200), (-length * 0.35, 0, deck_z + 220)], 6, 6)
    return mesh


def pier(length):
    """A timber pier along +X from its shore end at the origin: deck at z 0, piles to -6 m."""
    mesh = gltf_writer.Mesh()
    deck, wood = mesh.part("deck"), mesh.part("wood")
    box(deck, (length * 50, 0, -10), (length * 100, 400, 20))
    for x in np.arange(150, length * 100, 400):
        for y in (-180, 180):
            tube(wood, [(x, y, -600), (x, y, 30)], 16, 6)
    for y in (-190, 190):
        box(wood, (length * 50, y, 90), (length * 100, 8, 8))
        for x in np.arange(0, length * 100, 250):
            box(wood, (x, y, 45), (10, 10, 100))
    return mesh


def lighthouse():
    mesh = gltf_writer.Mesh()
    white, red, glass = mesh.part("white"), mesh.part("red"), mesh.part("glass")
    box(white, (0, 0, 60), (900, 900, 120))
    bands = [
        (0, 400, white),
        (400, 700, red),
        (700, 1100, white),
        (1100, 1400, red),
        (1400, 1700, white),
    ]
    for z0, z1, part in bands:
        r0, r1 = 300 - z0 * 0.07, 300 - z1 * 0.07
        tube(part, [(0, 0, z0 + 120), (0, 0, z1 + 120)], [r0, r1], 16, cap=False)
    tube(white, [(0, 0, 1820), (0, 0, 1840)], 230, 16)
    tube(glass, [(0, 0, 1840), (0, 0, 2100)], 140, 12)
    tube(red, [(0, 0, 2100), (0, 0, 2160), (0, 0, 2300)], [170, 150, 10], 12)
    return mesh


# --- placing ----------------------------------------------------------------------------


class Out:
    def __init__(self):
        self.meshes, self.items, self.spin, self.groups = {}, [], [], {}
        self.marks = []  # (x, y, colour) for features.png
        self.views = []  # [name, eye, target] m: stills of the landmarks
        self.clear = []  # [x, y, radius] m: ground the level's trees keep off

    def gltf(self, name, mesh):
        mesh.write(f"{SRC}/feat_{name}.gltf")
        self.meshes[name] = f"gltf:{name}"

    def group(self, name, cull=0, shadows=True):
        self.groups[name] = {"cull": cull, "shadows": shadows}

    def add(self, group, key, x, y, z, yaw=0.0, scale=1.0, pitch=0.0, roll=0.0, mark=None):
        self.meshes.setdefault(key, key)
        self.items.append(
            [
                group,
                key,
                *(round(float(v) * 100, 1) for v in (x, y, z)),
                round(float(yaw), 2),
                [round(float(v), 3) for v in scale]
                if isinstance(scale, tuple)
                else round(float(scale), 3),
                round(float(pitch), 2),
                round(float(roll), 2),
            ]
        )
        if mark:
            self.marks.append((x, y, mark))


class Road:
    def __init__(self):
        with open(f"{SRC}/routes.json") as f:
            r = json.load(f)
        # The default route, which crosses every bridge; the hub's coast road, if there is one;
        # and every metre of road, for distances to it.
        self.line = np.array(r["island"], dtype=float)
        self.spans = r.get("bridges", [])
        hub = r.get("routes", {}).get("harbour")
        self.hub = np.array(hub, dtype=float) if hub else None
        every = [self.line[::5, :2]] + [
            np.array(e, dtype=float)[::5, :2] for e in r.get("extra", [])
        ]
        self.tree = cKDTree(np.vstack(every))

    def dist(self, x, y):
        d, _ = self.tree.query(np.column_stack([np.ravel(x), np.ravel(y)]))
        return d.reshape(np.shape(x))


def rocks(out, g, bio, road, rng):
    """Scatter each biome's rocks near the road: tried in proportion to its share of the road,
    kept where the ground suits them and nothing else stands."""
    out.group("Rocks", cull=0)
    line = road.line[::25]
    kinds = bio.best(line[:, 0], line[:, 1])
    shore = g.at(g.inland, line[:, 0], line[:, 1])
    kinds = np.where(shore < 250, "coast_" + kinds, kinds)
    taken = []  # (x, y, radius)
    placed = {}
    for key in sorted(set(kinds)):
        biome = key.removeprefix("coast_")
        km = (kinds == key).sum() * 25 / 1000
        sets = ROCKS.get(biome, []) + (ROCKS["coast"] if key.startswith("coast_") else [])
        for mesh, size, (s0, s1), near, per_km, steep, high in sets:
            want = int(per_km * km)
            tries, got = want * 40, 0
            at = line[kinds == key]
            for _ in range(tries):
                if got >= want:
                    break
                p = at[rng.integers(len(at))]
                far = near + rng.exponential(120 if size > 10 else 60)
                a = rng.uniform(0, 2 * math.pi)
                x, y = p[0] + far * math.cos(a), p[1] + far * math.sin(a)
                scale = rng.uniform(s0, s1)
                r = size * scale * 0.5
                z = float(g.z(x, y))
                if z < (0.0 if mesh.endswith("beach_rocks") or "beach_rock" in mesh else 1.5):
                    continue
                if road.dist(x, y) < r + 8 or (bio.best(x, y) != biome and biome != "coast"):
                    continue
                if biome == "coast" and g.at(g.inland, x, y) > 60:
                    continue
                slope = float(g.at(g.slope, x, y))
                if steep and slope < 0.35:
                    continue
                if high and float(g.at(g.prominence, x, y)) < 4:
                    continue
                if any(math.hypot(x - tx, y - ty) < r + tr for tx, ty, tr in taken):
                    continue
                taken.append((x, y, r))
                # Sunk by a sixth of its size, on the slope's lower side.
                z = float(min(g.z(x + dx, y + dy) for dx in (-r, r) for dy in (-r, r)))
                out.add(
                    "Rocks",
                    mesh,
                    x,
                    y,
                    z - size * scale * 0.12,
                    rng.uniform(0, 360),
                    scale,
                    rng.uniform(-4, 4),
                    rng.uniform(-4, 4),
                    mark=(150, 140, 130) if size > 10 else (90, 85, 80),
                )
                got += 1
            placed[mesh.rsplit("/", 1)[1]] = placed.get(mesh.rsplit("/", 1)[1], 0) + got
    print(f"rocks: {sum(placed.values())} {placed}", flush=True)
    return taken


# Cliffs on the steep ground (13 Nov, the AAA review: "place Nanite cliff meshes along slope
# contours, sunk 30% and yawed to the aspect, and scree on the talus below"): per biome, the
# slope they need, how many at most, the faces (key, size m, scale range) and the scree.
CLIFFS = {
    "mountain": (
        0.45,
        2600,
        [
            (ms("mountain", "sharp_cliff"), 18.9, (0.9, 1.8)),
            (ms("mountain", "huge_icelandic_volcanic_cliff"), 15.6, (1.0, 2.2)),
            (ms("mountain", "massive_tundra_rock_formation"), 26.4, (0.6, 1.2)),
        ],
        [
            (ms("mountain", "volcanic_scree"), 2.2, (2.0, 4.2)),
            (ms("mountain", "limestone_rocks"), 2.9, (1.5, 3.2)),
        ],
    ),
    "desert": (
        0.40,
        1400,
        [
            (ms("desert", "desert_western_cliff_layered_xl_14"), 25.7, (0.8, 1.6)),
            (ms("desert", "desert_western_ledge_rock_layered_xl_01"), 22.6, (0.8, 1.5)),
            (ms("desert", "desert_western_spire_layered_xl_04"), 27.9, (0.5, 1.0)),
        ],
        [(ms("desert", "desert_western_cluster_rock_medium_05"), 3.0, (1.2, 2.5))],
    ),
    "heath": (
        0.42,
        700,
        [
            (ms("mountain", "icelandic_rock_plates"), 6.4, (1.5, 3.0)),
            (ms("mountain", "massive_tundra_rock_formation"), 26.4, (0.28, 0.55)),
        ],
        [(ms("mountain", "tundra_boulder"), 2.9, (0.8, 1.8))],
    ),
    "forest": (
        0.55,
        450,
        [(ms("forest", "nordic_forest_cliff_large"), 13.3, (0.9, 1.6))],
        [(ms("forest", "mossy_rocks"), 3.0, (1.0, 2.0))],
    ),
}
CLIFF_REACH = 1800.0  # m from the road: past it, no one sees a face's detail
# Sea stacks off the cliffed shores (make_chain's SEA_CLIFF on the hub, the Moor's and Vael's
# cliffs): (mesh, size m, scale range); how far out (m); how high the shore must stand within
# STACK_TOP[1] m; most of them; and rocks heaped at the cliff's foot.
# (mesh, size m, height m, scale range): heights from the FBX bounds (out/qa/probe/fbx_bounds.py),
# as v45 sank each by a share of its length and buried the flat beach slabs it stood (13 Nov).
STACKS = [(ms("coast", "huge_nordic_coastal_cliff"), 13.7, 8.9, (2.5, 4.0))]
STACK_FOOT = [
    (ms("coast", "nordic_beach_rock_formation"), 7.1, 3.0, (1.0, 2.0)),
    (ms("coast", "large_nordic_coastal_cliff"), 6.5, 2.2, (1.5, 3.0)),
]
# Rock on the sea cliffs' faces: the coastal cliff scaled to the face's height, base in the
# water, facing the sea, packed along the shore; the most of them.
SEA_FACE = (ms("coast", "huge_nordic_coastal_cliff"), 13.7, 8.9)
SEA_FACES_MOST = 1500
STACK_OUT = (30.0, 250.0)
STACK_TOP = (12.0, 150.0)
STACK_MOST = 90
STACK_BIOMES = ("meadow", "heath", "mountain")  # not "town": the harbour stays clear
CLIFF_STEP = 3  # ground samples (7 m) between candidates


def cliffs(out, g, bio, road, rng, taken):
    """Rock faces where the ground is steep, packed along the slope so they read as one wall,
    each with scree spilled below it; none within reach of the road's edge."""
    out.group("Cliffs", cull=0)
    s = CLIFF_STEP
    h, slope = g.h[::s, ::s], g.slope[::s, ::s]
    gy, gx = np.gradient(g.h, g.spacing)
    gx, gy = gx[::s, ::s], gy[::s, ::s]
    idx = (np.arange(h.shape[0]) * s - (g.n - 1) / 2) * g.spacing
    y, x = np.meshgrid(idx, idx, indexing="ij")
    pick = (slope > 0.38) & (h > 6) & (g.inland[::s, ::s] > 35)
    x, y, h, slope, gx, gy = (a[pick] for a in (x, y, h, slope, gx, gy))
    near = road.dist(x, y)
    keep = near < CLIFF_REACH
    x, y, slope, gx, gy, near = (a[keep] for a in (x, y, slope, gx, gy, near))
    kinds = bio.best(x, y)
    cell = 12.0  # m: the spatial hash's cell for the spacing test
    grid = {}

    def free(px, py, r):
        cx, cy = int(px // cell), int(py // cell)
        span = int(r // cell) + 3
        for i in range(cx - span, cx + span + 1):
            for j in range(cy - span, cy + span + 1):
                for qx, qy, qr in grid.get((i, j), ()):
                    if math.hypot(px - qx, py - qy) < 0.50 * (r + qr):
                        return False
        return True

    def claim(px, py, r):
        grid.setdefault((int(px // cell), int(py // cell)), []).append((px, py, r))

    for tx, ty, tr in taken:
        claim(tx, ty, tr)
    counts = {}
    # Prioritize slopes closest to the road (with jitter) so valley walls beside the rider
    # are densely dressed with rock faces rather than spending the budget 1.5 km away.
    order = np.argsort(near + rng.uniform(0.0, 140.0, len(near)))
    for k in order:
        biome = str(kinds[k])
        if biome not in CLIFFS:
            continue
        need, most, faces, scree = CLIFFS[biome]
        if slope[k] < need or counts.get(biome, 0) >= most:
            continue
        mesh, size, (s0, s1) = faces[rng.integers(len(faces))]
        scale = rng.uniform(s0, s1) * (1 + 0.4 * min(slope[k] - need, 1))
        r = size * scale * 0.5
        if near[k] < r + 7 or not free(x[k], y[k], r):
            continue
        claim(x[k], y[k], r)
        counts[biome] = counts.get(biome, 0) + 1
        # Facing downhill (the aspect), give or take; sunk by about a third into the slope.
        down = math.degrees(math.atan2(-gy[k], -gx[k]))
        base = float(min(g.z(x[k] + dx, y[k] + dy) for dx in (-r, r) for dy in (-r, r)))
        out.add(
            "Cliffs",
            mesh,
            x[k],
            y[k],
            base - size * scale * 0.3,
            down + rng.uniform(-30, 30),
            scale,
            rng.uniform(-6, 6),
            rng.uniform(-6, 6),
            mark=(170, 160, 150),
        )
        # Scree: two to four heaps down the fall line from the face's foot.
        fall = np.array([-gx[k], -gy[k]]) / max(math.hypot(gx[k], gy[k]), 1e-6)
        for _ in range(rng.integers(2, 5)):
            smesh, ssize, (t0, t1) = scree[rng.integers(len(scree))]
            d = r * rng.uniform(0.7, 1.6)
            side = rng.uniform(-0.8, 0.8) * r
            sx = x[k] + fall[0] * d - fall[1] * side
            sy = y[k] + fall[1] * d + fall[0] * side
            if road.dist(sx, sy) < 10:
                continue
            ss = rng.uniform(t0, t1)
            out.add(
                "Cliffs",
                smesh,
                sx,
                sy,
                g.z(sx, sy) - ssize * ss * 0.15,
                rng.uniform(0, 360),
                ss,
                mark=(120, 115, 110),
            )
    print(f"cliffs: {counts}", flush=True)


def sea_stacks(out, g, bio, road, rng):
    """Stacks standing in the sea off the cliffed shores, within sight of the road, and rocks
    heaped at those cliffs' feet."""
    out.group("Stacks", cull=0)
    top = ndimage.maximum_filter(g.h, int(2 * STACK_TOP[1] / g.spacing) + 1)
    s = 4
    idx = (np.arange(0, g.n, s) - (g.n - 1) / 2) * g.spacing
    y, x = np.meshgrid(idx, idx, indexing="ij")
    sea, high = g.sea[::s, ::s], top[::s, ::s]
    pick = (sea > STACK_OUT[0]) & (sea < STACK_OUT[1]) & (high > STACK_TOP[0])
    foot = (g.inland[::s, ::s] > 0) & (g.inland[::s, ::s] < 12) & (high > STACK_TOP[0])
    placed = []
    counts = {"stacks": 0, "foot": 0}
    for kind, mask, meshes, most, sink in (
        ("stacks", pick, STACKS, STACK_MOST, 0.05),
        ("foot", foot, STACK_FOOT, STACK_MOST * 3, 0.2),
    ):
        px, py = x[mask], y[mask]
        keep = road.dist(px, py) < CLIFF_REACH
        keep &= np.isin(bio.best(px, py), STACK_BIOMES)
        px, py = px[keep], py[keep]
        for k in rng.permutation(len(px)):
            if counts[kind] >= most:
                break
            mesh, size, tall, (s0, s1) = meshes[rng.integers(len(meshes))]
            scale = rng.uniform(s0, s1)
            r = size * scale * 0.5
            if road.dist(px[k : k + 1], py[k : k + 1])[0] < r + 15:
                continue
            if any(math.hypot(px[k] - qx, py[k] - qy) < 1.2 * (r + qr) for qx, qy, qr in placed):
                continue
            placed.append((px[k], py[k], r))
            counts[kind] += 1
            out.add(
                "Stacks",
                mesh,
                px[k],
                py[k],
                g.z(px[k], py[k]) - tall * scale * sink,
                rng.uniform(0, 360),
                scale,
                rng.uniform(-4, 4),
                rng.uniform(-4, 4),
                mark=(150, 145, 140),
            )
    print(f"sea stacks: {counts}", flush=True)
    return placed + sea_faces(out, g, bio, road, rng, placed)


def sea_faces(out, g, bio, road, rng, taken):
    """Rock along the sea cliffs: where the ground within 40 m of the water is steep and stands
    over 10 m, a coastal cliff scaled to that height, its base below the waterline, facing the
    sea, each overlapping the last so they read as one wall."""
    s = 2
    mesh, size, tall = SEA_FACE
    h = g.h[::s, ::s]
    top = ndimage.maximum_filter(h, int(2 * 40 / (g.spacing * s)) + 1)
    gy, gx = np.gradient(g.h, g.spacing)
    gx, gy = gx[::s, ::s], gy[::s, ::s]
    idx = (np.arange(h.shape[0]) * s - (g.n - 1) / 2) * g.spacing
    y, x = np.meshgrid(idx, idx, indexing="ij")
    inland = g.inland[::s, ::s]
    pick = (inland > 0) & (inland < 25) & (top > 10) & (g.slope[::s, ::s] > 0.4)
    x, y, top, gx, gy = (a[pick] for a in (x, y, top, gx, gy))
    keep = (road.dist(x, y) < CLIFF_REACH) & np.isin(bio.best(x, y), STACK_BIOMES)
    x, y, top, gx, gy = (a[keep] for a in (x, y, top, gx, gy))
    cell = 12.0
    grid = {}
    for tx, ty, tr in taken:
        grid.setdefault((int(tx // cell), int(ty // cell)), []).append((tx, ty, tr))
    placed = []
    for k in rng.permutation(len(x)):
        if len(placed) >= SEA_FACES_MOST:
            break
        scale = float(np.clip(top[k] / tall * rng.uniform(1.0, 1.25), 1.5, 5.0))
        r = size * scale * 0.5
        if road.dist(x[k : k + 1], y[k : k + 1])[0] < r + 6:
            continue
        cx, cy = int(x[k] // cell), int(y[k] // cell)
        span = int(r // cell) + 3
        if any(
            math.hypot(x[k] - qx, y[k] - qy) < 0.6 * (r + qr)
            for i in range(cx - span, cx + span + 1)
            for j in range(cy - span, cy + span + 1)
            for qx, qy, qr in grid.get((i, j), ())
        ):
            continue
        grid.setdefault((cx, cy), []).append((x[k], y[k], r))
        placed.append((x[k], y[k], r))
        down = math.degrees(math.atan2(-gy[k], -gx[k]))
        out.add(
            "Stacks",
            mesh,
            x[k],
            y[k],
            min(float(g.z(x[k], y[k])), 0.0) - 2.0,
            down + rng.uniform(-25, 25),
            scale,
            rng.uniform(-4, 4),
            rng.uniform(-4, 4),
            mark=(160, 150, 140),
        )
    print(f"sea faces: {len(placed)}", flush=True)
    return placed


def turbines(out, g, bio, road, rng, taken):
    """A wind farm on The Moor's high ground, a few hundred metres off the road, all facing the
    westerly."""
    out.group("Turbines", cull=0)
    out.gltf("turbine_tower", turbine_tower())
    out.gltf("turbine_rotor", turbine_rotor())
    line = road.line[::20]
    moor = line[bio.best(line[:, 0], line[:, 1]) == "heath"]
    spots = []
    for _ in range(6000):
        p = moor[rng.integers(len(moor))]
        a, d = rng.uniform(0, 2 * math.pi), rng.uniform(180, 900)
        x, y = p[0] + d * math.cos(a), p[1] + d * math.sin(a)
        if bio.best(x, y) != "heath" or g.at(g.inland, x, y) < 150 or road.dist(x, y) < 160:
            continue
        if float(g.at(g.slope, x, y)) > 0.2 or any(
            math.hypot(x - sx, y - sy) < 380 for sx, sy, _ in spots
        ):
            continue
        if any(math.hypot(x - tx, y - ty) < tr + 25 for tx, ty, tr in taken):
            continue
        spots.append((x, y, float(g.at(g.prominence, x, y))))
    spots.sort(key=lambda s: -s[2])
    spots = spots[:14]
    for x, y, _ in spots:
        z = float(g.z(x, y))
        yaw = 180 + rng.uniform(-6, 6)  # the rotor (+X) into the wind from the west
        out.add("Turbines", "turbine_tower", x, y, z - 0.5, yaw, mark=(255, 255, 255))
        c, s = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))
        hx, hy = x + c * 6.6, y + s * 6.6
        out.spin.append(
            [
                "turbine_rotor",
                round(hx * 100, 1),
                round(hy * 100, 1),
                round(z * 100 + HUB + 50, 1),
                round(yaw, 2),
                round(rng.uniform(70, 95), 1),
                round(rng.uniform(0, 120), 1),
            ]
        )
        taken.append((x, y, 45))
    print(f"turbines: {len(spots)}", flush=True)
    if spots:  # from the road nearest a turbine, looking up at its hub
        x, y, _ = spots[0]
        _, k = road.tree.query([x, y])
        rx, ry, rz = road.line[k * 5]
        out.views.append(["feat_turbines", [rx, ry, rz + 1.6], [x, y, float(g.z(x, y)) + 60]])
    return [(x, y) for x, y, _ in spots]


def power_line(out, g, road, rng, taken, name, a, b, side, line=None):
    """Pylons ~280 m apart, set off the road by `side` m (left of the riding direction) along its
    points a..b (metres along `line`, the default route if None), and the cables between them as
    one mesh built where it stands."""
    line = road.line if line is None else line
    n = len(line)
    idx = np.arange(a, b) % n
    smooth = np.column_stack(
        [ndimage.uniform_filter1d(line[:, k], 1500, mode="wrap")[idx] for k in range(2)]
    )
    d = np.gradient(smooth, axis=0)
    d /= np.linalg.norm(d, axis=1, keepdims=True)
    off = smooth + np.column_stack([-d[:, 1], d[:, 0]]) * side
    seg = np.r_[0, np.cumsum(np.linalg.norm(np.diff(off, axis=0), axis=1))]
    towers = []
    for s in np.arange(0, seg[-1], 280):
        x, y = (np.interp(s, seg, off[:, k]) for k in range(2))
        # Clear of the road and above the sea: slide along a little, or leave a gap.
        for nudge in (0, 40, -40, 80, -80):
            xx = x + nudge * d[min(np.searchsorted(seg, s), len(d) - 1), 0]
            yy = y + nudge * d[min(np.searchsorted(seg, s), len(d) - 1), 1]
            if (
                road.dist(xx, yy) > 30
                and float(g.z(xx, yy)) > 2
                and not any(math.hypot(xx - tx, yy - ty) < tr + 10 for tx, ty, tr in taken)
            ):
                towers.append((xx, yy, float(g.z(xx, yy))))
                break
    if len(towers) < 3:
        return
    mesh = gltf_writer.Mesh()
    cable = mesh.part("cable")
    ox, oy, oz = towers[0]
    yaws = []
    for i, (x, y, _z) in enumerate(towers):
        p = towers[max(i - 1, 0)]
        q = towers[min(i + 1, len(towers) - 1)]
        yaws.append(math.degrees(math.atan2(q[1] - p[1], q[0] - p[0])))
        out.add("Pylons", "pylon", x, y, towers[i][2] - 0.3, yaws[-1], mark=(230, 200, 60))
        taken.append((x, y, 12))

    def attach(i, across, up):
        x, y, z = towers[i]
        r = math.radians(yaws[i])
        return (
            (x - ox - math.sin(r) * across) * 100,
            (y - oy + math.cos(r) * across) * 100,
            (z - 0.3 - oz + up - 2.2) * 100,
        )

    for i in range(len(towers) - 1):
        span = math.dist(towers[i][:2], towers[i + 1][:2])
        if span > 420:  # a gap where a tower couldn't stand: no cable
            continue
        for across, up in [*PYLON_ARMS, PYLON_TOP]:
            p0, p1 = attach(i, across, up), attach(i + 1, across, up)
            sag = span * 100 * (0.03 if (across, up) == PYLON_TOP else 0.045)
            pts = [
                tuple(
                    np.add(p0, np.multiply(np.subtract(p1, p0), t)) - (0, 0, 4 * sag * t * (1 - t))
                )
                for t in np.linspace(0, 1, 14)
            ]
            tube(cable, pts, 3.5, sides=4, cap=False)
    out.gltf(f"cables_{name}", mesh)
    out.add("Cables", f"cables_{name}", ox, oy, oz, mark=None)
    print(f"power line {name}: {len(towers)} pylons", flush=True)


def _copy_harbour_kit(out, name, fallback_fn):
    """Uses the high-craft Blender mesh from SRC/harbour_kit if present, else builds fallback."""
    kit_file = f"{SRC}/harbour_kit/feat_{name}.gltf"
    if os.path.isfile(kit_file):
        shutil.copyfile(kit_file, f"{SRC}/feat_{name}.gltf")
        out.meshes[name] = f"gltf:{name}"
        return True
    if fallback_fn is not None:
        out.gltf(name, fallback_fn())
    return False


def harbour(out, g, bio, road, rng, taken):
    """Piers into the bay with boats tied up beside them, more at anchor, a stone quay,
    a coastal harbour town along the waterfront & bike trail, dockside cranes/bollards,
    stone gateway arches, and a lighthouse on the bay's outer point."""
    out.group("Harbour", cull=0)
    out.group("Quay", cull=0)
    out.group("HarbourTown", cull=0)
    out.group("HarbourProps", cull=35000)
    hx, hy = bio.world["landmarks"]["harbour"]
    for name, spec in BOATS.items():
        _copy_harbour_kit(out, f"boat_{name}", lambda s=spec: boat(*s))
    for length in (60, 90):
        _copy_harbour_kit(out, f"pier_{length}", lambda ln=length: pier(ln))
    _copy_harbour_kit(out, "lighthouse", lighthouse)
    has_houses = all(
        _copy_harbour_kit(out, f"harbour_house_{k}", None) for k in ("a", "b", "c", "d")
    )
    has_warehouse = _copy_harbour_kit(out, "harbour_warehouse", None)
    has_cargo = _copy_harbour_kit(out, "harbour_cargo", None)
    has_stall = _copy_harbour_kit(out, "harbour_market_stall", None)
    has_planter = _copy_harbour_kit(out, "harbour_planter", None)
    has_lake_pav = _copy_harbour_kit(out, "lakefront_pavilion", None)
    has_boathouse = _copy_harbour_kit(out, "modern_boathouse", None)
    has_crane = _copy_harbour_kit(out, "harbour_crane", None)
    has_arch = _copy_harbour_kit(out, "stone_arch", None)
    # The bay's shore: land points within 1.6 km whose water side is the bay.
    s = g.spacing
    r0 = int((hy / s) + (g.n - 1) / 2)
    c0 = int((hx / s) + (g.n - 1) / 2)
    k = int(2600 / s)
    win = g.h[r0 - k : r0 + k, c0 - k : c0 + k]
    edge = (win > 0.3) & ndimage.binary_dilation(win <= 0.3) & ~ndimage.binary_erosion(win > 0.3)
    rows, cols = np.nonzero(edge)
    xs = (cols + c0 - k - (g.n - 1) / 2) * s
    ys = (rows + r0 - k - (g.n - 1) / 2) * s
    # The lighthouse: on the islet in the bay's mouth (the window's smaller pieces of land),
    # at its top; else on the shore point furthest out.
    pieces, count = ndimage.label(win > 0.3)
    sizes = ndimage.sum(np.ones_like(win), pieces, range(1, count + 1))
    light = None
    for label in np.argsort(sizes)[::-1][1:] + 1:
        if sizes[label - 1] * s * s < 5000:  # rocks awash, not an islet
            break
        r, c = np.unravel_index(np.argmax(np.where(pieces == label, win, -1e9)), win.shape)
        x, y = ((c + c0 - k - (g.n - 1) / 2) * s, (r + r0 - k - (g.n - 1) / 2) * s)
        if math.hypot(x - hx, y - hy) < 2200:
            light = (x, y)
            break
    keep = np.hypot(xs - hx, (ys - hy) * 1.3) < 1500
    xs, ys = xs[keep], ys[keep]
    # Seaward: down the distance-to-land gradient.
    gy2, gx2 = np.gradient(g.inland)
    ax, ay = -g.at(gx2, xs, ys), -g.at(gy2, xs, ys)
    norm = np.hypot(ax, ay) + 1e-6
    ax, ay = ax / norm, ay / norm
    # The quay: the shore nearest the town (the bay's north side), concrete blocks along it.
    order = np.argsort(ys)
    quay = [(xs[i], ys[i]) for i in order if ys[i] < hy - 300 and abs(xs[i] - hx) < 420]
    for i, (x, y) in enumerate(quay):
        if i % 2:
            continue
        j = np.argmin(np.hypot(xs - x, ys - y))
        # The engine's 1 m cube: 3 m deep, 16 m along the shore, from 4 m under the sea to 1.5 m.
        yaw = math.degrees(math.atan2(ay[j], ax[j]))
        out.add(
            "Quay",
            "/Engine/BasicShapes/Cube",
            x + ax[j] * 1.5,
            y + ay[j] * 1.5,
            -1.25,
            yaw,
            (3.0, 16.0, 5.5),
            mark=(120, 120, 120),
        )
    # Piers from the quay out into the bay, a few hundred metres apart.
    piers = []
    for i in rng.permutation(len(quay)):
        x, y = quay[i]
        if any(math.hypot(x - px, y - py) < 160 for px, py, *_ in piers) or road.dist(x, y) < 25:
            continue
        j = np.argmin(np.hypot(xs - x, ys - y))
        yaw = math.degrees(math.atan2(ay[j], ax[j]))
        length = 90 if len(piers) % 2 == 0 else 60
        end = (x + ax[j] * length, y + ay[j] * length)
        if float(g.z(*end)) > -0.8:  # the beach shelves gently: deep enough for a boat
            continue
        piers.append((x, y, yaw, length))
        if len(piers) == 4:
            break
    names = list(BOATS)
    boats = 0
    for x, y, yaw, length in piers:
        out.add("Harbour", f"pier_{length}", x, y, 1.6, yaw, mark=(160, 110, 60))
        r = math.radians(yaw)
        along, across = (math.cos(r), math.sin(r)), (-math.sin(r), math.cos(r))
        for t in np.arange(18, length - 6, 16):
            for side in (-1, 1):
                if rng.random() < 0.25:
                    continue
                kind = names[rng.integers(3)]
                beam_m = BOATS[kind][1]
                cx = x + along[0] * t + across[0] * side * (2 + beam_m / 2 + 0.8)
                cy = y + along[1] * t + across[1] * side * (2 + beam_m / 2 + 0.8)
                out.add(
                    "Harbour",
                    f"boat_{kind}",
                    cx,
                    cy,
                    0.0,
                    yaw + rng.choice([0, 180]) + rng.uniform(-3, 3),
                    mark=(250, 250, 250),
                )
                boats += 1
    # At anchor out in the bay, all swung to the same breeze.
    for _ in range(400):
        if boats > 34:
            break
        x, y = hx + rng.normal(0, 500), hy + rng.normal(150, 350)
        if float(g.at(g.sea, x, y)) < 45 or float(g.z(x, y)) > -2:
            continue
        if any(math.hypot(x - tx, y - ty) < 30 for tx, ty, tr in taken):
            continue
        kind = names[rng.integers(len(names))]
        out.add("Harbour", f"boat_{kind}", x, y, 0.0, 200 + rng.normal(0, 12), mark=(250, 250, 250))
        taken.append((x, y, 12))
        boats += 1
    if light is None:
        far = np.argmax(np.hypot(xs - hx, ys - hy))
        light = (xs[far] - ax[far] * 14, ys[far] - ay[far] * 14)
    if light:
        x, y = light
        out.add("Harbour", "lighthouse", x, y, float(g.z(x, y)) - 0.5, 0, mark=(255, 60, 60))
    if piers:  # over the quay, out across the boats to the lighthouse
        x, y, *_ = piers[0]
        target = [*light, 8.0] if light else [hx, hy + 600, 0.0]
        out.views.append(["feat_harbour", [x, y - 60, float(g.z(x, y - 60)) + 20], target])
        # Dockside cranes & bollards/ropes along the quay edge near the piers
        bollard = ms("coast", "wooden_bollard")
        rope = ms("coast", "harbour_rope_hemp_old_02")
        for px, py, pyaw, _plen in piers:
            r = math.radians(pyaw)
            side_x, side_y = -math.sin(r), math.cos(r)
            if has_crane:
                cx, cy = px + side_x * 9.0, py + side_y * 9.0
                if road.dist(cx, cy) > 6.5:
                    out.add("Harbour", "harbour_crane", cx, cy, 1.5, pyaw, mark=(180, 140, 80))
                    taken.append((cx, cy, 3.5))
            for d_off in (-16, -8, 15, 23):
                bx, by = px + side_x * d_off, py + side_y * d_off
                if road.dist(bx, by) > 5.0 and g.z(bx, by) > -0.2:
                    out.add(
                        "HarbourProps",
                        bollard,
                        bx,
                        by,
                        max(1.4, float(g.z(bx, by))),
                        rng.uniform(0, 360),
                        rng.uniform(0.9, 1.15),
                    )
                    if rng.random() < 0.55:
                        out.add(
                            "HarbourProps",
                            rope,
                            bx + side_x * 0.8,
                            by + side_y * 0.8,
                            max(1.4, float(g.z(bx, by))),
                            rng.uniform(0, 360),
                            rng.uniform(0.8, 1.2),
                        )

    # Harbour Town: coastal houses, tavern, warehouses, benches & planters lining the waterfront
    # and the opening ~1.1 km of Harbour Isle's bike trail.
    town_houses = []
    if has_houses:
        house_specs = [
            ("harbour_house_a", 9.6, 7.2),
            ("harbour_house_b", 12.0, 7.8),
            ("harbour_house_c", 11.6, 9.2),
            ("harbour_house_d", 10.6, 6.0),
        ]
        line = road.hub if road.hub is not None else road.line
        bench = ms("urban", "park_bench_wood")
        planter = "harbour_planter" if has_planter else bench
        stone_wall = ms("coast", "mossy_stone_wall")
        # Walk along the harbour road (both out from start and wrapping near end)
        indices = list(range(25, min(1150, len(line)), 18)) + list(
            range(max(len(line) - 650, 1200), len(line) - 25, 22)
        )
        for k in indices:
            if len(town_houses) >= 34:
                break
            px, py = float(line[k, 0]), float(line[k, 1])
            if math.hypot(px - hx, py - hy) > 1300:
                continue
            tx = float(line[min(k + 4, len(line) - 1), 0] - line[max(k - 4, 0), 0])
            ty = float(line[min(k + 4, len(line) - 1), 1] - line[max(k - 4, 0), 1])
            norm_t = math.hypot(tx, ty) or 1.0
            nx, ny = -ty / norm_t, tx / norm_t
            yaw0 = math.degrees(math.atan2(ty, tx))
            for side in (-1.0, 1.0):
                hname, hw, hd = house_specs[int(rng.integers(len(house_specs)))]
                off = rng.uniform(9.5, 19.5)
                cx, cy = px + nx * side * off, py + ny * side * off
                # House front is -Y in Blender -> +Y in UE; rotate so front faces the road
                yaw = yaw0 + (0.0 if side < 0 else 180.0) + rng.uniform(-6.0, 6.0)
                c, s = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))
                corners = [
                    (cx + dx * c - dy * s, cy + dx * s + dy * c)
                    for dx in (-hw * 0.5, hw * 0.5)
                    for dy in (-hd * 0.5, hd * 0.5)
                ]
                zs = [float(g.z(qx, qy)) for qx, qy in corners]
                if min(zs) < 1.2 or max(zs) - min(zs) > 2.4:
                    continue
                if min(float(road.dist(qx, qy)) for qx, qy in corners) < 5.8:
                    continue
                reach = math.hypot(hw, hd) * 0.5
                if any(
                    math.hypot(cx - qx, cy - qy) < qr + reach + 1.8 for qx, qy, qr in town_houses
                ):
                    continue
                if any(math.hypot(cx - tx2, cy - ty2) < tr + reach + 1.5 for tx2, ty2, tr in taken):
                    continue
                z_base = float(np.percentile(zs, 35))
                out.add("HarbourTown", hname, cx, cy, z_base, yaw, mark=(240, 175, 80))
                town_houses.append((cx, cy, reach))
                taken.append((cx, cy, reach))
                out.clear.append([round(cx, 1), round(cy, 1), round(reach + 4.5, 1)])
                # Promenade bench or planter between house and road
                bx, by = px + nx * side * 4.8, py + ny * side * 4.8
                if road.dist(bx, by) > 3.6 and g.z(bx, by) > 1.0:
                    prop = bench if rng.random() < 0.6 else planter
                    out.add(
                        "HarbourProps",
                        prop,
                        bx,
                        by,
                        float(g.z(bx, by)) - 0.04,
                        yaw0 + (0.0 if side < 0 else 180.0),
                        rng.uniform(0.95, 1.1),
                    )
        # Also place stone seawall/harbour walls, waterfront warehouses, coastal houses & cargo
        # stacks along the inland quay edge behind the piers so the harbourfront reads as a port.
        quay_buildings = 0
        quay_cargo = 0
        for i in range(0, len(quay), 6):
            qx, qy = quay[i]
            j = np.argmin(np.hypot(xs - qx, ys - qy))
            wyaw = math.degrees(math.atan2(ay[j], ax[j])) + 90.0
            wx, wy = qx - ax[j] * 4.0, qy - ay[j] * 4.0
            if road.dist(wx, wy) > 5.0 and g.z(wx, wy) > 0.8:
                out.add("HarbourProps", stone_wall, wx, wy, float(g.z(wx, wy)) - 0.15, wyaw, 1.1)
            # Dockside cargo crate/barrel clusters and market stalls near the quay apron
            if has_cargo and i % 12 == 0:
                cx, cy = qx - ax[j] * rng.uniform(7.5, 13.0), qy - ay[j] * rng.uniform(7.5, 13.0)
                if road.dist(cx, cy) > 5.5 and g.z(cx, cy) > 1.0:
                    prop_name = (
                        "harbour_market_stall" if (has_stall and i % 24 == 0) else "harbour_cargo"
                    )
                    out.add(
                        "HarbourProps",
                        prop_name,
                        cx,
                        cy,
                        float(g.z(cx, cy)) - 0.05,
                        wyaw + rng.uniform(-18, 18),
                    )
                    quay_cargo += 1
        # Waterfront row of modern marina boathouses, lakefront pavilions, and warehouses standing
        # 18-34 m inland behind the quay (Chicago Lakefront marina vibe).
        quay_specs = []
        if has_boathouse:
            quay_specs.extend([("modern_boathouse", 15.4, 9.0), ("modern_boathouse", 15.4, 9.0)])
        if has_lake_pav:
            quay_specs.append(("lakefront_pavilion", 8.0, 4.8))
        if has_warehouse:
            quay_specs.append(("harbour_warehouse", 14.4, 9.0))
        if not quay_specs:
            quay_specs.extend(house_specs)
        for i in range(0, len(quay), 10):
            if quay_buildings >= 16:
                break
            qx, qy = quay[i]
            j = np.argmin(np.hypot(xs - qx, ys - qy))
            # Face the bay (+ax, +ay): front is -Y in Blender (+Y in UE), so yaw = sea_angle - 90
            sea_yaw = math.degrees(math.atan2(ay[j], ax[j])) - 90.0 + rng.uniform(-5.0, 5.0)
            hname, hw, hd = quay_specs[int(rng.integers(len(quay_specs)))]
            inland_off = rng.uniform(19.0, 34.0)
            cx, cy = qx - ax[j] * inland_off, qy - ay[j] * inland_off
            c, s = math.cos(math.radians(sea_yaw)), math.sin(math.radians(sea_yaw))
            corners = [
                (cx + dx * c - dy * s, cy + dx * s + dy * c)
                for dx in (-hw * 0.5, hw * 0.5)
                for dy in (-hd * 0.5, hd * 0.5)
            ]
            zs = [float(g.z(px2, py2)) for px2, py2 in corners]
            if min(zs) < 0.8 or max(zs) - min(zs) > 2.6:
                continue
            if min(float(road.dist(px2, py2)) for px2, py2 in corners) < 7.0:
                continue
            reach = math.hypot(hw, hd) * 0.5
            if any(math.hypot(cx - tx2, cy - ty2) < tr + reach + 2.5 for tx2, ty2, tr in taken):
                continue
            z_base = float(np.percentile(zs, 35))
            out.add("HarbourTown", hname, cx, cy, z_base, sea_yaw, mark=(240, 175, 80))
            taken.append((cx, cy, reach))
            out.clear.append([round(cx, 1), round(cy, 1), round(reach + 4.5, 1)])
            quay_buildings += 1
            if has_cargo:
                gx, gy = cx + ax[j] * (hd * 0.5 + 3.2), cy + ay[j] * (hd * 0.5 + 3.2)
                if road.dist(gx, gy) > 5.0 and g.z(gx, gy) > 0.8:
                    out.add(
                        "HarbourProps",
                        "harbour_cargo",
                        gx,
                        gy,
                        float(g.z(gx, gy)) - 0.05,
                        sea_yaw + rng.uniform(-18, 18),
                    )
                    quay_cargo += 1
        print(
            f"harbour quayfront: {quay_buildings} waterfront buildings, {quay_cargo} cargo stacks",
            flush=True,
        )

        if town_houses:
            # Eye-level view riding through Harbour Town
            k_view = min(140, len(line) - 35)
            p0, p1 = line[k_view], line[k_view + 30]
            out.views.append(
                [
                    "feat_harbour_town",
                    [float(p0[0]), float(p0[1]), float(p0[2]) + 1.8],
                    [float(p1[0]), float(p1[1]), float(p1[2]) + 2.8],
                ]
            )

    # Modern Chicago Lakefront park shelters spaced along the coastal bike trail
    if has_lake_pav:
        line = road.hub if road.hub is not None else road.line
        lake_pavs = 0
        for k in range(180, min(1400, len(line) - 20), 260):
            if lake_pavs >= 5:
                break
            px, py = float(line[k, 0]), float(line[k, 1])
            tx = float(line[min(k + 4, len(line) - 1), 0] - line[max(k - 4, 0), 0])
            ty = float(line[min(k + 4, len(line) - 1), 1] - line[max(k - 4, 0), 1])
            norm_t = math.hypot(tx, ty) or 1.0
            nx, ny = -ty / norm_t, tx / norm_t
            yaw0 = math.degrees(math.atan2(ty, tx))
            for side in (-1.0, 1.0):
                cx, cy = px + nx * side * 10.5, py + ny * side * 10.5
                cz = float(g.z(cx, cy))
                if (
                    cz > 1.2
                    and float(g.at(g.slope, cx, cy)) < 0.22
                    and road.dist(cx, cy) > 6.5
                    and not any(math.hypot(cx - tx2, cy - ty2) < tr + 5.5 for tx2, ty2, tr in taken)
                ):
                    yaw = yaw0 + (0.0 if side < 0 else 180.0)
                    out.add("HarbourTown", "lakefront_pavilion", cx, cy, cz - 0.05, yaw)
                    taken.append((cx, cy, 4.5))
                    out.clear.append([round(cx, 1), round(cy, 1), 6.0])
                    lake_pavs += 1
                    break

    # Landmark stone gateway arches: keep medieval stone arch on Vael's mountain approach (leave
    # Harbour Isle's modern Chicago Lakefront trail open with unobstructed water sightlines).
    arches = 0
    if has_arch:
        line = road.line
        arch_spots = []
        for first, _last in _stretch_range("Vael", road)[:1]:
            arch_spots.append(first + 180)
        for k in arch_spots:
            if k < 10 or k >= len(line) - 10:
                continue
            p = line[k]
            if float(p[2]) < 1.5:
                continue
            tx = float(line[k + 4, 0] - line[k - 4, 0])
            ty = float(line[k + 4, 1] - line[k - 4, 1])
            ayaw = math.degrees(math.atan2(ty, tx))
            out.add(
                "HarbourTown",
                "stone_arch",
                float(p[0]),
                float(p[1]),
                float(p[2]) - 0.05,
                ayaw,
                mark=(220, 220, 240),
            )
            taken.append((float(p[0]), float(p[1]), 6.5))
            out.clear.append([round(float(p[0]), 1), round(float(p[1]), 1), 8.0])
            arches += 1
            if arches == 1:
                p_eye = line[max(0, k - 32)]
                out.views.append(
                    [
                        "feat_stone_arch",
                        [float(p_eye[0]), float(p_eye[1]), float(p_eye[2]) + 1.7],
                        [float(p[0]), float(p[1]), float(p[2]) + 3.5],
                    ]
                )

    print(
        f"harbour: {len(piers)} piers, {boats} boats, quay {len(quay) // 2} blocks, "
        f"{len(town_houses)} town houses, {arches} stone arches",
        flush=True,
    )


def bridges(out, g, road):
    """Each bridge built where it stands: a box girder under the road deck, parapets, piers down
    to the sea floor; the longest is cable-stayed, from two towers."""
    out.group("Bridges", cull=0)
    longest = max(range(len(road.spans)), key=lambda i: road.spans[i][1] - road.spans[i][0])
    half = (ROAD_HALF + 0.2) * 100
    for i, (a, b) in enumerate(road.spans):
        mesh = gltf_writer.Mesh()
        concrete, steel, cable = mesh.part("concrete"), mesh.part("steel"), mesh.part("cable")
        pts = road.line[a : b + 1 : 4]
        o = pts[0]
        local = [((x - o[0]) * 100, (y - o[1]) * 100, (z - o[2]) * 100 + 2) for x, y, z in pts]
        sweep(
            concrete,
            local,
            [
                (-half - 40, -20),
                (-half - 40, -90),
                (-half + 60, -200),
                (half - 60, -200),
                (half + 40, -90),
                (half + 40, -20),
            ],
            closed=True,
        )
        for sgn in (-1, 1):
            edge = half + 20
            sweep(
                concrete,
                local,
                [
                    (sgn * edge, -22),
                    (sgn * edge, 75),
                    (sgn * (edge + 25), 75),
                    (sgn * (edge + 25), -22),
                ][::sgn],
            )
            # The top rail, a closed bar on posts from the parapet: open underneath and floating
            # with nothing under it, its inside showed from the road and its thin faces flickered
            # (13 Nov QA: "fuzzy artifacts around the top rail").
            sweep(
                steel,
                local,
                [
                    (sgn * (edge + 5), 100),
                    (sgn * (edge + 5), 112),
                    (sgn * (edge + 20), 112),
                    (sgn * (edge + 20), 100),
                ],
                closed=True,
            )
            for j, (x, y, z) in enumerate(local):
                d = np.subtract(local[min(j + 1, len(local) - 1)], local[max(j - 1, 0)])
                length = math.hypot(d[0], d[1]) or 1.0
                rx, ry = -d[1] / length, d[0] / length
                across = sgn * (edge + 12.5)
                yaw = math.degrees(math.atan2(d[1], d[0]))
                box(steel, (x + rx * across, y + ry * across, z + 87.5), (8, 8, 26), yaw)
        # Piers wherever the ground is well below the deck, about every 45 m.
        ground = g.z(pts[:, 0], pts[:, 1])
        seg = np.r_[0, np.cumsum(np.linalg.norm(np.diff(pts[:, :2], axis=0), axis=1))]
        stayed = i == longest
        water = np.flatnonzero(ground < 0)
        towers = []
        if stayed and len(water):
            w0, w1 = seg[water[0]], seg[water[-1]]
            towers = [w0 + (w1 - w0) * f for f in (0.28, 0.72)]
        spacing = 160 if stayed else 45
        for s in np.arange(spacing / 2, seg[-1], spacing):
            j = np.searchsorted(seg, s)
            if pts[j, 2] - ground[j] < 4:
                continue
            if any(abs(s - t) < 40 for t in towers):
                continue
            x, y, z = local[j]
            d = np.subtract(local[min(j + 1, len(local) - 1)], local[max(j - 1, 0)])
            yaw = math.degrees(math.atan2(d[1], d[0]))
            bottom = (ground[j] - o[2] - 3) * 100
            for side in (-1, 1):
                r = math.radians(yaw + 90)
                cx, cy = x + math.cos(r) * side * half * 0.5, y + math.sin(r) * side * half * 0.5
                tube(concrete, [(cx, cy, bottom), (cx, cy, z - 200)], 75, 10)
            box(concrete, (x, y, z - 230), (220, half * 2 + 60, 60), yaw)
        for t in towers:
            j = np.searchsorted(seg, t)
            x, y, z = local[j]
            d = np.subtract(local[min(j + 1, len(local) - 1)], local[max(j - 1, 0)])
            yaw = math.degrees(math.atan2(d[1], d[0]))
            r = math.radians(yaw + 90)
            bottom = (ground[j] - o[2] - 3) * 100
            top = z + 5500
            legs = []
            for side in (-1, 1):
                foot = (
                    x + math.cos(r) * side * (half + 250),
                    y + math.sin(r) * side * (half + 250),
                )
                head = (x + math.cos(r) * side * 120, y + math.sin(r) * side * 120)
                tube(concrete, [(*foot, bottom), (*foot, z - 100)], [140, 120], 10)
                tube(concrete, [(*foot, z - 100), (*head, top)], [120, 70], 10)
                legs.append(head)
            box(concrete, (x, y, top - 600), (160, 300, 160), yaw)
            box(concrete, (x, y, z - 220), (260, half * 2 + 700, 140), yaw)
            # Stays fan from the tower head to the deck edges, out to 200 m each way.
            for dist in np.arange(20, 205, 12):
                for direction in (-1, 1):
                    s2 = t + direction * dist
                    if s2 < 0 or s2 > seg[-1]:
                        continue
                    k = np.searchsorted(seg, s2)
                    kx, ky, kz = local[min(k, len(local) - 1)]
                    for side, head in zip((-1, 1), legs, strict=True):
                        hz = top - 400 - dist * 4
                        deck = (
                            kx + math.cos(r) * side * (half + 30),
                            ky + math.sin(r) * side * (half + 30),
                            kz + 60,
                        )
                        tube(cable, [(*head, hz), deck], 5, 4, cap=False)
        out.gltf(f"bridge_{i}", mesh)
        out.add("Bridges", f"bridge_{i}", o[0], o[1], o[2], mark=(200, 200, 210))
        length = seg[-1]
        if stayed:  # from the shore off to one side, the whole span in view
            mid = pts[len(pts) // 2]
            d = pts[-1, :2] - pts[0, :2]
            side = np.array([-d[1], d[0]]) / np.linalg.norm(d)
            eye = mid[:2] + side * 420
            out.views.append(["feat_bridge", [*eye, mid[2] + 25], mid.tolist()])
        print(f"bridge {i}: {length:.0f} m{', cable-stayed' if stayed else ''}", flush=True)


def preview(g, road, out):
    """features.png: the ground, the road, and every placement as a dot."""
    n = 2048
    step = g.n / n
    h = g.h[:: round(step), :: round(step)]
    img = np.zeros((*h.shape, 3), np.uint8)
    land = h > 0.3
    shade = np.clip(90 + h * 0.25, 60, 255)
    img[land] = np.stack([shade * 0.55, shade * 0.75, shade * 0.45], -1)[land].astype(np.uint8)
    img[~land] = (30, 60, 110)
    im = Image.fromarray(img)
    draw = ImageDraw.Draw(im)
    scale = h.shape[0] / (g.n * g.spacing)

    def px(x, y):
        return (x * scale + h.shape[1] / 2, y * scale + h.shape[0] / 2)

    pts = [px(x, y) for x, y, _ in road.line[::40]]
    draw.line(pts, fill=(230, 220, 200), width=1)
    for x, y, colour in out.marks:
        cx, cy = px(x, y)
        draw.ellipse((cx - 2, cy - 2, cx + 2, cy + 2), fill=colour)
    for item in out.spin:
        cx, cy = px(item[1] / 100, item[2] / 100)
        draw.ellipse((cx - 3, cy - 3, cx + 3, cy + 3), outline=(255, 255, 255))
    im.save(f"{SRC}/features.png")


# Cairns on the Moor (gap 12, signs of people): dry-stacked piles of flat stones on the knolls
# by the road, a walker's waymarks. Each stone is the tundra boulder (287 x 212 cm, 178 tall, its
# pivot 20 cm above its base) squashed flat; CAIRN_EVERY m of moor road apart at least.
CAIRN_STONE = ms("mountain", "tundra_boulder")
CAIRN_EVERY = 450.0
CAIRN_REACH = (6.0, 220.0)  # m from the road


def cairn(out, rng, x, y, z):
    """One cairn round (x, y) on ground z (m): rings of stones narrowing up to a capstone."""
    height = rng.uniform(1.2, 2.1)
    base = rng.uniform(0.55, 0.8)
    level, k = 0.0, 0
    while level < height - 0.15:
        r = base * (1 - level / height) ** 0.8
        flat = rng.uniform(0.09, 0.13)  # of 178 cm
        size = rng.uniform(0.15, 0.2)  # of 287 cm
        count = max(1, round(2 * math.pi * r / (2.87 * size * 0.8)))
        turn = rng.uniform(0, 2 * math.pi)
        for i in range(count if r > 0.12 else 1):
            a = turn + 2 * math.pi * i / count
            reach = r * 0.8 if r > 0.12 else 0.0
            out.add(
                "Cairns",
                CAIRN_STONE,
                x + reach * math.cos(a),
                y + reach * math.sin(a),
                z + level + 0.198 * flat * rng.uniform(0.6, 1.0),
                math.degrees(a) + 90 + rng.normal(0, 15),
                (size, size * rng.uniform(0.85, 1.15), flat),
                rng.normal(0, 4),
                rng.normal(0, 4) - 5 * (reach > 0),  # leaning in
            )
        level += 1.78 * flat * 0.72
        k += 1
    s = rng.uniform(0.1, 0.13)
    out.add("Cairns", CAIRN_STONE, x, y, z + level, rng.uniform(0, 360), (s, s * 0.8, s * 1.1))


def cairns(out, g, bio, road, rng, taken):
    """Cairns on the Moor's knolls near its roads, spaced along them."""
    out.group("Cairns", cull=0)
    pts = road.tree.data
    moor = pts[bio.best(pts[:, 0], pts[:, 1]) == "heath"]
    placed = []
    for k in rng.permutation(len(moor)):
        px, py = moor[k]
        if any(math.hypot(px - cx, py - cy) < CAIRN_EVERY for cx, cy in placed):
            continue
        # The highest of a few spots off the road here, if it stands up from its surroundings.
        best = None
        for _ in range(24):
            d = rng.uniform(*CAIRN_REACH)
            a = rng.uniform(0, 2 * math.pi)
            x, y = px + d * math.cos(a), py + d * math.sin(a)
            if road.dist(x, y) < CAIRN_REACH[0] or bio.best(x, y) != "heath":
                continue
            if g.at(g.slope, x, y) > 0.25 or g.z(x, y) < 5:
                continue
            if any(math.hypot(x - tx, y - ty) < tr + 2 for tx, ty, tr in taken):
                continue
            rise = g.at(g.prominence, x, y)
            if rise > 3 and (best is None or rise > best[0]):
                best = (rise, x, y)
        if best is None:
            continue
        _, x, y = best
        z = min(g.z(x + dx, y + dy) for dx in (-0.8, 0.8) for dy in (-0.8, 0.8))
        cairn(out, rng, x, y, z)
        placed.append((px, py))
        taken.append((x, y, 1.0))
        out.marks.append((x, y, (240, 240, 240)))
    print(f"cairns: {len(placed)}", flush=True)


# --- hamlets and the tower ----------------------------------------------------------------

# Gap 12, signs of people: a hamlet of cottages and a barn on each island off the harbour, and a
# ruined tower on a knoll of Vael. Their walls, roofs and stone are Poly Haven sets on the meshes'
# own UVs (in metres), climb_world.py's M_Feature_<slot>: [set, metres a repeat, tint].
TEXTURES = {
    "plaster": ["worn_mossy_plasterwall", 2.5, [0.95, 0.95, 0.92]],
    "ochre": ["worn_mossy_plasterwall", 2.5, [1.12, 0.88, 0.62]],
    "slate": ["grey_roof_tiles", 2.6, [1.0, 1.0, 1.0]],
    "clay": ["grey_roof_tiles_02", 2.4, [1.55, 0.8, 0.55]],
    "stone": ["japanese_stone_wall", 2.4, [1.0, 1.0, 1.0]],
    "siding": ["weathered_plank_siding", 1.8, [1.4, 1.35, 1.3]],
    "boards": ["dark_wooden_planks", 1.5, [1.0, 1.0, 1.0]],
}
# Each island's hamlet: its stretch on the default ride, the walls and roof its cottages take,
# and how many to try for.
HAMLETS = {
    "Greenwood": ("plaster", "slate", 7),
    "Vael": ("stone", "slate", 5),
    "The Moor": ("stone", "slate", 5),
    "Red Sands": ("ochre", "clay", 6),
}
HAMLET_REACH = (30.0, 140.0)  # m from the road to the hamlet's middle
HAMLET_SPREAD = 55.0  # m: the houses within this of the middle
HOUSE_ROAD = 9.0  # m: no wall nearer the road
TOWER_REACH = (40.0, 320.0)


def quad_uv(part, corners, uvs, outward):
    """A flat quad with its own UVs (m), turned to face `outward`."""
    a, b, c = corners[:3]
    n = _cross(_sub(b, a), _sub(c, a))
    if _dot(n, outward) < 0:
        corners, uvs = corners[::-1], uvs[::-1]
        n = (-n[0], -n[1], -n[2])
    length = math.sqrt(_dot(n, n)) or 1.0
    n = (n[0] / length, n[1] / length, n[2] / length)
    ids = [part.vertex(p, n, uv) for p, uv in zip(corners, uvs, strict=True)]
    if len(ids) == 4:
        part.quad(*ids)
    else:
        part.triangle(*ids)


def cottage(rng, walls, roof, w, d, h, barn=False):
    """A cottage (cm, its middle at the ground, the ridge along X): a stone plinth sunk into the
    slope, walls, gables, a pitched roof with eaves, a chimney, a door, windows with shutters.
    The barn: plank walls, a wide door, no chimney or windows."""
    mesh = gltf_writer.Mesh()
    base = 40.0
    mesh.box("stone", (0, 0, (base - 160) / 2), (w + 12, d + 12, base + 160))
    mesh.box(walls, (0, 0, base + (h - base) / 2), (w, d, h - base))
    pitch = math.radians(rng.uniform(36, 44) if not barn else rng.uniform(26, 32))
    rise = d / 2 * math.tan(pitch)
    wall = mesh.part(walls)
    for sx in (-1, 1):
        x = sx * w / 2
        quad_uv(
            wall,
            [(x, -d / 2, h), (x, d / 2, h), (x, 0, h + rise)],
            [(-d / 200, -h / 100), (d / 200, -h / 100), (0, -(h + rise) / 100)],
            (sx, 0, 0),
        )
    # The roof: each slope a slab 14 cm thick, out past the walls by its eaves.
    over, eave_x = 45.0, w / 2 + 35.0
    top = mesh.part(roof)
    under = mesh.part("boards")
    slope = math.hypot(d / 2 + over, (d / 2 + over) * math.tan(pitch))
    for sy in (-1, 1):
        ridge_z = h + rise + 14
        eave_y, eave_z = sy * (d / 2 + over), h + rise - (d / 2 + over) * math.tan(pitch) + 14
        corners = [
            (-eave_x, 0, ridge_z),
            (eave_x, 0, ridge_z),
            (eave_x, eave_y, eave_z),
            (-eave_x, eave_y, eave_z),
        ]
        uvs = [
            (-eave_x / 100, 0),
            (eave_x / 100, 0),
            (eave_x / 100, slope / 100),
            (-eave_x / 100, slope / 100),
        ]
        up = (0, sy * math.sin(pitch), math.cos(pitch))
        quad_uv(top, corners, uvs, up)
        low = [(x, y, z - 14) for x, y, z in corners]
        quad_uv(under, low, uvs, (0, -up[1], -up[2]))
        # The slab's edges: the eave's fascia and the verges at the gable ends.
        quad_uv(
            under,
            [corners[2], corners[3], low[3], low[2]],
            [(eave_x / 100, 0), (-eave_x / 100, 0), (-eave_x / 100, 0.14), (eave_x / 100, 0.14)],
            (0, sy, 0),
        )
        for sx, (i, j) in ((-1, (0, 3)), (1, (1, 2))):
            quad_uv(
                under,
                [corners[i], corners[j], low[j], low[i]],
                [(0, 0), (slope / 100, 0), (slope / 100, 0.14), (0, 0.14)],
                (sx, 0, 0),
            )
    if not barn:
        cx = rng.choice((-1, 1)) * (w / 2 - 60)
        mesh.box("stone", (cx, rng.uniform(-0.2, 0.2) * d, h + rise / 2 + 60), (70, 55, rise + 120))
    # The door on the front (-Y) face and the windows either side, shuttered.
    door_w = 260 if barn else 100
    door_x = 0.0 if barn else rng.uniform(-0.15, 0.15) * w
    door_h = 300 if barn else 205
    mesh.box("boards", (door_x, -d / 2 - 4, base + door_h / 2), (door_w, 10, door_h))
    if barn:
        return mesh
    for side in (-1, 1):
        y = side * (d / 2 + 3)
        spots = [
            x
            for x in np.linspace(-w / 2 + 110, w / 2 - 110, 3)
            if abs(x - door_x) > 120 or side > 0
        ]
        for x in spots:
            mesh.box("glass", (x, y, base + 150), (80, 8, 105))
            mesh.box("boards", (x, y + side * 1, base + 150 + 60), (100, 10, 12))  # the lintel
            for sx in (-1, 1):
                mesh.box("boards", (x + sx * 62, y + side * 2, base + 150), (40, 6, 108))
    return mesh


def ruin_tower(rng):
    """A round tower fallen in: a ring of stone 0.9 m thick, 13 m tall on its best side and down
    to a stump on the other, its top ragged, a doorway and two window slits."""
    mesh = gltf_writer.Mesh()
    part = mesh.part("stone")
    sides, r_out, r_in = 20, 340.0, 250.0
    fall = rng.uniform(0, 2 * math.pi)
    tops = []
    for i in range(sides):
        a = 2 * math.pi * (i + 0.5) / sides
        lean = 0.5 + 0.5 * math.cos(a - fall)  # 1 on the standing side, 0 on the fallen
        tops.append(250 + 1050 * lean**1.3 + rng.uniform(-90, 60))
    gaps = {0: (0, 230), sides // 3: (520, 680), 2 * sides // 3: (780, 900)}
    for i in range(sides):
        a0, a1 = 2 * math.pi * i / sides, 2 * math.pi * (i + 1) / sides
        top = tops[i]
        spans = [(-150.0, top)]
        if i in gaps and gaps[i][1] < top - 60:
            lo, hi = gaps[i]
            spans = [(-150.0, lo), (hi, top)] if lo > 0 else [(hi, top)]
        for z0, z1 in spans:
            o0 = (r_out * math.cos(a0), r_out * math.sin(a0))
            o1 = (r_out * math.cos(a1), r_out * math.sin(a1))
            i0 = (r_in * math.cos(a0), r_in * math.sin(a0))
            i1 = (r_in * math.cos(a1), r_in * math.sin(a1))
            u0, u1 = r_out * a0 / 100, r_out * a1 / 100
            mid = (math.cos((a0 + a1) / 2), math.sin((a0 + a1) / 2), 0)
            quad_uv(
                part,
                [(*o0, z0), (*o1, z0), (*o1, z1), (*o0, z1)],
                [(u0, -z0 / 100), (u1, -z0 / 100), (u1, -z1 / 100), (u0, -z1 / 100)],
                mid,
            )
            quad_uv(
                part,
                [(*i0, z0), (*i1, z0), (*i1, z1), (*i0, z1)],
                [(u0, -z0 / 100), (u1, -z0 / 100), (u1, -z1 / 100), (u0, -z1 / 100)],
                (-mid[0], -mid[1], 0),
            )
            for z, up in ((z1, 1), (z0, -1)):
                quad_uv(
                    part,
                    [(*o0, z), (*o1, z), (*i1, z), (*i0, z)],
                    [(u0, 0), (u1, 0), (u1, 0.9), (u0, 0.9)],
                    (0, 0, up),
                )
        # A doorway's or slit's jambs: the neighbours' faces across the opening.
        if len(spans) > 1 or spans[0][0] > -150:
            lo, hi = (spans[0][1], spans[1][0]) if len(spans) > 1 else (-150.0, spans[0][0])
            for a, sgn in ((a0, -1), (a1, 1)):
                o = (r_out * math.cos(a), r_out * math.sin(a))
                n = (r_in * math.cos(a), r_in * math.sin(a))
                facing = (sgn * math.sin(a), -sgn * math.cos(a), 0)
                quad_uv(
                    part,
                    [(*o, lo), (*n, lo), (*n, hi), (*o, hi)],
                    [(0, -lo / 100), (0.9, -lo / 100), (0.9, -hi / 100), (0, -hi / 100)],
                    facing,
                )
        # The ragged steps between neighbours' tops: close each side of the segment.
        for a, j in ((a0, i - 1), (a1, i + 1)):
            other = tops[j % sides]
            if other < top:
                o = (r_out * math.cos(a), r_out * math.sin(a))
                n = (r_in * math.cos(a), r_in * math.sin(a))
                side = (-math.sin(a), math.cos(a), 0) if j > i else (math.sin(a), -math.cos(a), 0)
                quad_uv(
                    part,
                    [(*o, other), (*n, other), (*n, top), (*o, top)],
                    [(0, -other / 100), (0.9, -other / 100), (0.9, -top / 100), (0, -top / 100)],
                    side,
                )
    return mesh, fall


def _stretch_range(name, road):
    """[(first, last) index into road.line] of each run of the default ride's `name` stretch,
    the line being about a metre a point."""
    with open(f"{SRC}/info.json") as f:
        runs = json.load(f)["ride"]["stretches"]
    seg = np.r_[0, np.cumsum(np.linalg.norm(np.diff(road.line[:, :2], axis=0), axis=1))]
    out, at = [], 0.0
    for stretch, metres in runs:
        if stretch == name:
            out.append((int(np.searchsorted(seg, at)), int(np.searchsorted(seg, at + metres))))
        at += metres
    return out


def _house_fits(g, road, taken, x, y, yaw, w, d, houses):
    c, s = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))
    corners = [
        (x + dx * c - dy * s, y + dx * s + dy * c)
        for dx in (-w / 2 - 0.5, w / 2 + 0.5)
        for dy in (-d / 2 - 0.5, d / 2 + 0.5)
    ]
    zs = [g.z(cx, cy) for cx, cy in corners]
    if max(zs) - min(zs) > 1.2 or min(zs) < 2.5:
        return None
    if min(float(road.dist(cx, cy)) for cx, cy in corners) < HOUSE_ROAD:
        return None
    reach = math.hypot(w, d) / 2
    if any(math.hypot(x - tx, y - ty) < tr + reach + 1 for tx, ty, tr in taken):
        return None
    if any(math.hypot(x - hx, y - hy) < hr + reach + 3.5 for hx, hy, hr in houses):
        return None
    return min(zs), reach


def _hamlet_at(g, road, taken, rng, cx, cy, yaw0, want):
    """Cottages round (cx, cy): turned to the road, or end on to it now and then."""
    houses, placed = [], []
    for _ in range(140):
        if len(placed) >= want + 1:
            break
        barn = len(placed) == want
        w = rng.uniform(11, 14) if barn else rng.uniform(7.5, 10.5)
        d = rng.uniform(7, 8.5) if barn else rng.uniform(5.2, 6.4)
        r = HAMLET_SPREAD * math.sqrt(rng.uniform(0.02, 1))
        a = rng.uniform(0, 2 * math.pi)
        x, y = cx + r * math.cos(a), cy + r * math.sin(a)
        yaw = yaw0 + (90 if rng.random() < 0.3 else 0) + (180 if rng.random() < 0.5 else 0)
        yaw += rng.normal(0, 5)
        fit = _house_fits(g, road, taken, x, y, yaw, w, d, houses)
        if fit is None:
            continue
        z, reach = fit
        houses.append((x, y, reach))
        placed.append((x, y, z, yaw, w, d, barn))
    return placed


def hamlets(out, g, bio, road, rng, taken):
    """A hamlet on each island off the harbour: the open, level spot off its road that fits the
    most cottages, a barn among them."""
    out.group("Hamlets", cull=0)
    kinds = {}
    for island, (walls, roof, want) in HAMLETS.items():
        best, best_score = [], -1.0
        for first, last in _stretch_range(island, road):
            if last - first < 3000:
                continue
            for _ in range(160):
                k = int(rng.integers(first + 1000, last - 1000))
                px, py = road.line[k, :2]
                tx, ty = (
                    road.line[min(k + 3, len(road.line) - 1), :2] - road.line[max(k - 3, 0), :2]
                )
                yaw0 = math.degrees(math.atan2(ty, tx))
                off = rng.uniform(*HAMLET_REACH) * rng.choice((-1, 1))
                nx, ny = -ty / (math.hypot(tx, ty) or 1), tx / (math.hypot(tx, ty) or 1)
                cx, cy = px + nx * off, py + ny * off
                if g.at(g.slope, cx, cy) > 0.12 or g.z(cx, cy) < 3 or g.at(g.inland, cx, cy) < 50:
                    continue
                placed = _hamlet_at(g, road, taken, rng, cx, cy, yaw0, want)
                # The most houses, and of those the nearest the road, to be seen from it.
                score = len(placed) - abs(off) / 200
                if score > best_score:
                    best, best_score = placed, score
        if len(best) < 3:
            print(f"hamlet {island}: no room", flush=True)
            continue
        for x, y, z, yaw, w, d, barn in best:
            walls_here = "siding" if barn else walls
            key = (walls_here, roof, round(w), round(d), barn)
            if key not in kinds:
                name = f"house_{len(kinds)}"
                h = rng.uniform(380, 440) if barn else rng.uniform(300, 340)
                out.gltf(name, cottage(rng, walls_here, roof, w * 100, d * 100, h, barn))
                kinds[key] = name
            out.add("Hamlets", kinds[key], x, y, z - 0.05, yaw, mark=(250, 140, 40))
            taken.append((x, y, math.hypot(w, d) / 2))
            # Its ground cleared of trees (climb_world.trees skips stems in out.clear), the
            # ones between the houses kept, as a hamlet in a wood keeps its garden trees.
            out.clear.append([round(x, 1), round(y, 1), round(math.hypot(w, d) / 2 + 4, 1)])
        mx = float(np.mean([p[0] for p in best]))
        my = float(np.mean([p[1] for p in best]))
        _, k = road.tree.query([mx, my])
        ex, ey = road.tree.data[k]
        out.views.append(
            [
                f"feat_hamlet_{island.split()[-1].lower()}",
                [float(ex), float(ey), g.z(ex, ey) + 1.7],
                [mx, my, g.z(mx, my) + 3],
            ]
        )
        print(f"hamlet {island}: {len(best)} houses round ({mx:.0f}, {my:.0f})", flush=True)


def tower(out, g, bio, road, rng, taken):
    """The ruined tower: on the knoll off Vael's road that stands highest over its surroundings,
    with some of its fallen stone round its foot."""
    out.group("Tower", cull=0)
    best = None
    for first, last in _stretch_range("Vael", road):
        for _ in range(900):
            k = int(rng.integers(first, last))
            px, py = road.line[k, :2]
            d, a = rng.uniform(*TOWER_REACH), rng.uniform(0, 2 * math.pi)
            x, y = px + d * math.cos(a), py + d * math.sin(a)
            if road.dist(x, y) < TOWER_REACH[0] or g.at(g.slope, x, y) > 0.3 or g.z(x, y) < 10:
                continue
            if any(math.hypot(x - tx, y - ty) < tr + 6 for tx, ty, tr in taken):
                continue
            rise = g.at(g.prominence, x, y)
            if best is None or rise > best[0]:
                best = (rise, x, y)
    if best is None:
        print("tower: no knoll", flush=True)
        return
    _, x, y = best
    z = min(g.z(x + dx, y + dy) for dx in (-3.4, 3.4) for dy in (-3.4, 3.4))
    mesh, fall = ruin_tower(rng)
    out.gltf("ruin_tower", mesh)
    out.add("Tower", "ruin_tower", x, y, z, 0, mark=(200, 60, 200))
    taken.append((x, y, 4.0))
    out.clear.append([round(x, 1), round(y, 1), 7.0])
    # The fallen stone, thrown out on the side it fell to.
    stone = ms("mountain", "limestone_rocks")
    for _ in range(26):
        a = fall + rng.normal(0, 0.6)
        r = rng.uniform(3.8, 11.0)
        sx, sy = x + r * math.cos(a), y + r * math.sin(a)
        size = rng.uniform(0.18, 0.45)
        out.add(
            "Tower",
            stone,
            sx,
            sy,
            g.z(sx, sy) - 0.1 * size,
            rng.uniform(0, 360),
            (size, size * rng.uniform(0.8, 1.2), size * rng.uniform(0.5, 0.8)),
        )
    _, k = road.tree.query([x, y])
    ex, ey = road.tree.data[k]
    out.views.append(["feat_tower", [float(ex), float(ey), g.z(ex, ey) + 1.7], [x, y, z + 8]])
    print(
        f"tower: on a {best[0]:.0f} m knoll at ({x:.0f}, {y:.0f}),"
        f" {float(road.dist(x, y)):.0f} m off the road",
        flush=True,
    )


def roadside_dressing(out, g, bio, road, rng, taken):
    """Close-range biome roadside details (4-20 m from tarmac): mossy dry-stone walls on The Moor
    & Harbour coast, mossy logs/stumps/boulders in Greenwood, alpine boulders/scree on Vael, and
    layered sandstone outcrops along Red Sands."""
    out.group("Roadside", cull=32000)
    line = road.line
    n = len(line)
    # 1. Scenic mossy/dry stone walls running parallel to the road on Moor, Coast & Vael
    wall_moss = ms("coast", "mossy_stone_wall")
    wall_dry = ms("coast", "mossy_stone_wall")
    walls = 0
    k = 450
    while k < n - 120:
        b = str(bio.best(float(line[k, 0]), float(line[k, 1])))
        wall_chance = {"heath": 0.48, "meadow": 0.34, "mountain": 0.36}.get(b, 0.0)
        if wall_chance > 0.0 and rng.random() < wall_chance:
            side = rng.choice((-1.0, 1.0))
            run_len = int(rng.integers(32, 80))
            wall_mesh = wall_moss if b == "meadow" else wall_dry
            step_m = 4.2
            s_idx = k
            while s_idx < min(k + run_len, n - 10):
                px, py = float(line[s_idx, 0]), float(line[s_idx, 1])
                tx = float(line[min(s_idx + 3, n - 1), 0] - line[max(s_idx - 3, 0), 0])
                ty = float(line[min(s_idx + 3, n - 1), 1] - line[max(s_idx - 3, 0), 1])
                norm_t = math.hypot(tx, ty) or 1.0
                nx, ny = -ty / norm_t, tx / norm_t
                wx, wy = px + nx * side * 5.2, py + ny * side * 5.2
                wz = float(g.z(wx, wy))
                if (
                    wz > 2.0
                    and float(g.at(g.slope, wx, wy)) < 0.38
                    and road.dist(wx, wy) > 4.2
                    and not any(math.hypot(wx - tx2, wy - ty2) < tr + 2.5 for tx2, ty2, tr in taken)
                ):
                    wyaw = math.degrees(math.atan2(ty, tx))
                    out.add("Roadside", wall_mesh, wx, wy, wz - 0.18, wyaw, rng.uniform(0.95, 1.15))
                    walls += 1
                s_idx += round(step_m)
            k += run_len + int(rng.integers(85, 210))
        else:
            k += 55

    # 2. Close-range biome boulders, mossy fallen logs & stumps right along road banks (5-26 m)
    props_by_biome = {
        "forest": [
            (ms("forest", "mossy_rocks"), (0.7, 1.6), 0.12),
            (ms("forest", "fallen_log"), (0.7, 1.3), 0.10),
            (ms("forest", "mossy_wood_stump"), (0.8, 1.4), 0.12),
            (ms("forest", "nordic_forest_cluster_rock_medium"), (0.7, 1.4), 0.12),
        ],
        "mountain": [
            (ms("mountain", "tundra_boulder"), (0.7, 1.8), 0.18),
            (ms("mountain", "limestone_rocks"), (0.9, 2.1), 0.18),
            (ms("mountain", "volcanic_scree"), (1.2, 2.6), 0.14),
            (ms("forest", "fallen_log"), (0.7, 1.2), 0.10),
        ],
        "heath": [
            (ms("mountain", "tundra_boulder"), (0.6, 1.6), 0.16),
            (ms("mountain", "icelandic_rock_plates"), (0.6, 1.3), 0.16),
            (ms("forest", "mossy_rocks"), (0.6, 1.4), 0.14),
        ],
        "desert": [
            (ms("desert", "sandstone_rock_formation"), (1.0, 2.4), 0.18),
            (ms("desert", "canyon_sandstone_boulder"), (1.3, 3.0), 0.18),
            (ms("desert", "desert_western_cluster_rock_medium_05"), (0.9, 2.0), 0.16),
        ],
        "meadow": [
            (ms("forest", "mossy_rocks"), (0.6, 1.3), 0.15),
        ],
    }
    placed = 0
    for k in range(30, n - 30, 9):
        px, py = float(line[k, 0]), float(line[k, 1])
        b = str(bio.best(px, py))
        specs = props_by_biome.get(b)
        prob = {
            "mountain": 0.72,
            "heath": 0.68,
            "desert": 0.72,
            "forest": 0.55,
            "meadow": 0.45,
        }.get(b, 0.5)
        if not specs or rng.random() > prob:
            continue
        tx = float(line[min(k + 3, n - 1), 0] - line[max(k - 3, 0), 0])
        ty = float(line[min(k + 3, n - 1), 1] - line[max(k - 3, 0), 1])
        norm_t = math.hypot(tx, ty) or 1.0
        nx, ny = -ty / norm_t, tx / norm_t
        side = rng.choice((-1.0, 1.0))
        off = rng.uniform(5.0, 24.0 if b in ("mountain", "desert", "heath") else 18.0)
        x, y = px + nx * side * off, py + ny * side * off
        z = float(g.z(x, y))
        if z < 1.5 or road.dist(x, y) < 4.4:
            continue
        mesh, (s0, s1), sink = specs[int(rng.integers(len(specs)))]
        sc = rng.uniform(s0, s1)
        out.add(
            "Roadside",
            mesh,
            x,
            y,
            z - sink * sc,
            rng.uniform(0, 360),
            sc,
            rng.uniform(-6, 6),
            rng.uniform(-6, 6),
        )
        placed += 1
    print(f"roadside: {walls} wall pieces, {placed} verge boulders/logs", flush=True)


def shore_distance(g):
    """Writes SRC/shore_dist.png (4096x4096 RGB) aligned identically to road_dist.png:
    R = distance offshore from nearest land (0..45 m -> 0..255),
    G = shallow water depth below sea level (0..6 m -> 0..255),
    B = water mask (255 in sea / shoreline band, 0 deep inland)."""
    size = 4096
    out = np.zeros((size, size, 3), np.uint8)
    out[..., 0] = 255
    out[..., 1] = 255
    n = g.n
    r_ch = np.clip(np.round(g.sea / 45.0 * 255.0), 0, 255).astype(np.uint8)
    g_ch = np.clip(np.round(np.maximum(0.0, -g.h) / 6.0 * 255.0), 0, 255).astype(np.uint8)
    b_ch = np.where(g.h < 1.5, 255, 0).astype(np.uint8)
    out[:n, :n, 0] = r_ch
    out[:n, :n, 1] = g_ch
    out[:n, :n, 2] = b_ch
    Image.fromarray(out).save(f"{SRC}/shore_dist.png")
    print(f"{SRC}/shore_dist.png written ({n}x{n} padded to {size})", flush=True)


def _copy_world_prop(out, name):
    """Copies a handcrafted Blender world prop from SRC/world_props if present."""
    prop_file = f"{SRC}/world_props/feat_{name}.gltf"
    if os.path.isfile(prop_file):
        shutil.copyfile(prop_file, f"{SRC}/feat_{name}.gltf")
        out.meshes[name] = f"gltf:{name}"
        return True
    return False


def world_landmarks(out, g, bio, road, rng, taken):
    """Places handcrafted Blender open-world landmarks & roadside props across all 5 islands:
    monumental bridgehead pylons with heraldic banners, scenic alpine wayside pavilions,
    timber junction signposts, carved stone km milestones, highland standing-stone cairns,
    and Red Sands sandstone ruin arches."""
    out.group("WorldLandmarks", cull=0)
    out.group("Waymarkers", cull=45000)
    has = {
        name: _copy_world_prop(out, name)
        for name in (
            "bridge_pylon",
            "alpine_pavilion",
            "wood_signpost",
            "stone_milestone",
            "desert_arch",
            "moor_cairn",
            "moor_standing_stones",
            "vael_shrine",
        )
    }
    line = road.line
    n = len(line)
    counts = dict.fromkeys(has, 0)

    def tangent(idx):
        p0 = line[max(idx - 4, 0), :2]
        p1 = line[min(idx + 4, n - 1), :2]
        dx, dy = float(p1[0] - p0[0]), float(p1[1] - p0[1])
        ln = math.hypot(dx, dy) or 1.0
        return dx / ln, dy / ln

    # 1. Twin bridgehead pylons flanking both ends of every inter-island bridge + signposts
    pylon_view_added = False
    for a, b in road.spans:
        for end_idx, inward in ((max(a - 8, 4), -1.0), (min(b + 8, n - 5), 1.0)):
            px, py = float(line[end_idx, 0]), float(line[end_idx, 1])
            tx, ty = tangent(end_idx)
            nx, ny = -ty, tx
            yaw_face = math.degrees(math.atan2(inward * ty, inward * tx))
            if has["bridge_pylon"]:
                for side in (-1.0, 1.0):
                    qx, qy = px + nx * side * 5.8, py + ny * side * 5.8
                    qz = float(g.z(qx, qy))
                    if qz > 0.4 and road.dist(qx, qy) > 4.4:
                        out.add(
                            "WorldLandmarks",
                            "bridge_pylon",
                            qx,
                            qy,
                            qz - 0.10,
                            yaw_face,
                            mark=(255, 210, 70),
                        )
                        taken.append((qx, qy, 2.2))
                        out.clear.append([round(qx, 1), round(qy, 1), 4.5])
                        counts["bridge_pylon"] += 1
                if not pylon_view_added and float(g.z(px, py)) > 1.0:
                    eye_idx = max(end_idx - 28, 0) if inward < 0 else min(end_idx + 28, n - 1)
                    ex, ey, ez = line[eye_idx]
                    out.views.append(
                        [
                            "feat_bridge_pylon",
                            [float(ex), float(ey), float(ez) + 1.8],
                            [px, py, float(line[end_idx, 2]) + 3.2],
                        ]
                    )
                    pylon_view_added = True
            # Rustic timber signpost ~22 m ahead of each bridgehead
            if has["wood_signpost"]:
                s_idx = int(np.clip(end_idx + (22 if inward > 0 else -22), 10, n - 10))
                sx, sy = float(line[s_idx, 0]), float(line[s_idx, 1])
                stx, sty = tangent(s_idx)
                snx, sny = -sty, stx
                qx, qy = sx + snx * 5.2, sy + sny * 5.2
                qz = float(g.z(qx, qy))
                if qz > 1.2 and road.dist(qx, qy) > 4.4:
                    out.add(
                        "Waymarkers",
                        "wood_signpost",
                        qx,
                        qy,
                        qz - 0.08,
                        math.degrees(math.atan2(sty, stx)),
                        mark=(210, 155, 70),
                    )
                    taken.append((qx, qy, 1.8))
                    counts["wood_signpost"] += 1

    # 2. Carved stone km milestones every ~900 m along the tarmac verge
    if has["stone_milestone"]:
        for k in range(450, n - 200, 900):
            px, py = float(line[k, 0]), float(line[k, 1])
            tx, ty = tangent(k)
            nx, ny = -ty, tx
            side = -1.0 if (k // 900) % 2 == 0 else 1.0
            qx, qy = px + nx * side * 4.7, py + ny * side * 4.7
            qz = float(g.z(qx, qy))
            if (
                qz > 1.8
                and float(g.at(g.slope, qx, qy)) < 0.35
                and road.dist(qx, qy) > 4.1
                and not any(math.hypot(qx - x0, qy - y0) < r0 + 2.0 for x0, y0, r0 in taken)
            ):
                # Front plaque (+X) faces diagonally toward oncoming rider
                yaw = math.degrees(math.atan2(-ty - ny * side * 0.4, -tx - nx * side * 0.4))
                out.add("Waymarkers", "stone_milestone", qx, qy, qz - 0.06, yaw)
                taken.append((qx, qy, 1.2))
                counts["stone_milestone"] += 1

    # 3. Scenic Nordic-alpine timber rest pavilions along Vael, Greenwood, The Moor, and Coast
    if has["alpine_pavilion"]:
        pavilion_view_added = False
        pav_spots = []
        for k in range(800, n - 400, 340):
            if counts["alpine_pavilion"] >= 14:
                break
            if any(a - 45 <= k <= b + 45 for a, b in road.spans):
                continue
            px, py = float(line[k, 0]), float(line[k, 1])
            b = str(bio.best(px, py))
            if b not in ("forest", "mountain", "heath", "meadow"):
                continue
            if any(math.hypot(px - x0, py - y0) < 950 for x0, y0 in pav_spots):
                continue
            tx, ty = tangent(k)
            nx, ny = -ty, tx
            yaw0 = math.degrees(math.atan2(ty, tx))
            for side in (-1.0, 1.0):
                qx, qy = px + nx * side * 9.5, py + ny * side * 9.5
                # Pavilion front is -Y in Blender -> +Y in UE; rotate so front faces road
                yaw = yaw0 + (0.0 if side < 0 else 180.0)
                c, s = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))
                corners = [
                    (qx + dx * c - dy * s, qy + dx * s + dy * c)
                    for dx in (-2.3, 2.3)
                    for dy in (-1.6, 1.6)
                ]
                zs = [float(g.z(cx, cy)) for cx, cy in corners]
                if min(zs) < 2.2 or max(zs) - min(zs) > 1.4:
                    continue
                if min(float(road.dist(cx, cy)) for cx, cy in corners) < 5.5:
                    continue
                if any(math.hypot(qx - x0, qy - y0) < r0 + 4.5 for x0, y0, r0 in taken):
                    continue
                z_base = float(np.percentile(zs, 30))
                out.add(
                    "WorldLandmarks",
                    "alpine_pavilion",
                    qx,
                    qy,
                    z_base,
                    yaw,
                    mark=(235, 190, 90),
                )
                taken.append((qx, qy, 4.0))
                out.clear.append([round(qx, 1), round(qy, 1), 6.0])
                pav_spots.append((px, py))
                counts["alpine_pavilion"] += 1
                if not pavilion_view_added and b in ("forest", "mountain"):
                    eye_idx = max(k - 24, 0)
                    ex, ey, ez = line[eye_idx]
                    out.views.append(
                        [
                            "feat_pavilion",
                            [float(ex), float(ey), float(ez) + 1.8],
                            [qx, qy, z_base + 1.8],
                        ]
                    )
                    pavilion_view_added = True
                break

    # 4. Highland standing-stone cairns on The Moor & Vael + sandstone ruin arches on Red Sands
    if has["moor_cairn"]:
        for k in range(400, n - 400, 420):
            if counts["moor_cairn"] >= 16:
                break
            px, py = float(line[k, 0]), float(line[k, 1])
            b = str(bio.best(px, py))
            if b not in ("heath", "mountain") or rng.random() > 0.65:
                continue
            tx, ty = tangent(k)
            nx, ny = -ty, tx
            side = rng.choice((-1.0, 1.0))
            off = rng.uniform(10.0, 26.0)
            qx, qy = px + nx * side * off, py + ny * side * off
            qz = float(g.z(qx, qy))
            if (
                qz > 4.0
                and float(g.at(g.slope, qx, qy)) < 0.30
                and road.dist(qx, qy) > 7.5
                and not any(math.hypot(qx - x0, qy - y0) < r0 + 3.5 for x0, y0, r0 in taken)
            ):
                out.add(
                    "WorldLandmarks",
                    "moor_cairn",
                    qx,
                    qy,
                    qz - 0.15,
                    rng.uniform(0, 360),
                    rng.uniform(0.9, 1.25),
                )
                taken.append((qx, qy, 2.5))
                counts["moor_cairn"] += 1

    if has["desert_arch"]:
        arches_placed = []
        for k in range(300, n - 300, 320):
            if counts["desert_arch"] >= 7:
                break
            px, py = float(line[k, 0]), float(line[k, 1])
            if str(bio.best(px, py)) != "desert":
                continue
            if any(math.hypot(px - x0, py - y0) < 600 for x0, y0 in arches_placed):
                continue
            tx, ty = tangent(k)
            nx, ny = -ty, tx
            side = rng.choice((-1.0, 1.0))
            off = rng.uniform(16.0, 38.0)
            qx, qy = px + nx * side * off, py + ny * side * off
            qz = float(g.z(qx, qy))
            if (
                qz > 3.0
                and float(g.at(g.slope, qx, qy)) < 0.25
                and road.dist(qx, qy) > 12.0
                and not any(math.hypot(qx - x0, qy - y0) < r0 + 5.0 for x0, y0, r0 in taken)
            ):
                yaw = math.degrees(math.atan2(ty, tx)) + rng.uniform(-35, 35)
                out.add(
                    "WorldLandmarks",
                    "desert_arch",
                    qx,
                    qy,
                    qz - 0.25,
                    yaw,
                    rng.uniform(1.0, 1.35),
                    mark=(240, 130, 60),
                )
                taken.append((qx, qy, 4.5))
                arches_placed.append((px, py))
                counts["desert_arch"] += 1

    # 5. Ancient megalithic stone circles on The Moor's knolls
    if has["moor_standing_stones"]:
        circles = []
        for k in range(300, n - 300, 360):
            if counts["moor_standing_stones"] >= 6:
                break
            px, py = float(line[k, 0]), float(line[k, 1])
            if str(bio.best(px, py)) != "heath":
                continue
            if any(math.hypot(px - x0, py - y0) < 900 for x0, y0 in circles):
                continue
            tx, ty = tangent(k)
            nx, ny = -ty, tx
            side = rng.choice((-1.0, 1.0))
            off = rng.uniform(14.0, 32.0)
            qx, qy = px + nx * side * off, py + ny * side * off
            qz = float(g.z(qx, qy))
            if (
                qz > 4.0
                and float(g.at(g.slope, qx, qy)) < 0.24
                and road.dist(qx, qy) > 10.0
                and not any(math.hypot(qx - x0, qy - y0) < r0 + 5.5 for x0, y0, r0 in taken)
            ):
                out.add(
                    "WorldLandmarks",
                    "moor_standing_stones",
                    qx,
                    qy,
                    qz - 0.15,
                    rng.uniform(0, 360),
                    mark=(180, 180, 210),
                )
                taken.append((qx, qy, 5.0))
                out.clear.append([round(qx, 1), round(qy, 1), 6.0])
                circles.append((px, py))
                counts["moor_standing_stones"] += 1

    # 6. Weathered Nordic stone wayside shrines along Vael's mountain climb
    if has["vael_shrine"]:
        shrines = []
        for k in range(300, n - 300, 300):
            if counts["vael_shrine"] >= 10:
                break
            if any(a - 35 <= k <= b + 35 for a, b in road.spans):
                continue
            px, py = float(line[k, 0]), float(line[k, 1])
            if str(bio.best(px, py)) != "mountain":
                continue
            if any(math.hypot(px - x0, py - y0) < 750 for x0, y0 in shrines):
                continue
            tx, ty = tangent(k)
            nx, ny = -ty, tx
            yaw0 = math.degrees(math.atan2(ty, tx))
            for side in (-1.0, 1.0):
                qx, qy = px + nx * side * 7.8, py + ny * side * 7.8
                qz = float(g.z(qx, qy))
                if (
                    qz > 5.0
                    and float(g.at(g.slope, qx, qy)) < 0.34
                    and road.dist(qx, qy) > 5.0
                    and not any(math.hypot(qx - x0, qy - y0) < r0 + 3.0 for x0, y0, r0 in taken)
                ):
                    yaw = yaw0 + (0.0 if side < 0 else 180.0)
                    out.add(
                        "WorldLandmarks",
                        "vael_shrine",
                        qx,
                        qy,
                        qz - 0.10,
                        yaw,
                        mark=(220, 200, 150),
                    )
                    taken.append((qx, qy, 2.5))
                    out.clear.append([round(qx, 1), round(qy, 1), 4.0])
                    shrines.append((px, py))
                    counts["vael_shrine"] += 1
                    break

    print(f"world_landmarks: {counts}", flush=True)


def _copy_skyline_prop(out, name):
    """Copies a handcrafted Blender skyscraper glTF from SRC/skyline_kit/ into SRC."""
    kit_file = f"{SRC}/skyline_kit/feat_{name}.gltf"
    if not os.path.exists(kit_file):
        return False
    shutil.copy2(kit_file, f"{SRC}/feat_{name}.gltf")
    out.meshes[name] = f"gltf:feat_{name}.gltf"
    return True


def city_skyline(out, g, bio, road, rng, taken):
    """Places a Chicago-inspired lakeside downtown skyline inland behind Harbour Isle's bike trail
    and bay, stepping from supertall hero towers in the core down to Lake Shore Drive mid-rises
    along the parkway edge (cull=0 so it reads across the bay and bridges)."""
    out.group("Skyline", cull=0)
    names = (
        "skyline_supertall_stepped",
        "skyline_tapered_braced",
        "skyline_glass_tower",
        "skyline_artdeco",
        "skyline_midrise",
    )
    has = {name: _copy_skyline_prop(out, name) for name in names}
    if not any(has.values()):
        return
    hx, hy = bio.world["landmarks"]["harbour"]
    # Downtown core sits inland north of Harbour Bay, spread ~900 m along the coast x ~420 m deep.
    cx0, cy0 = hx - 80.0, hy - 1180.0
    radii = {
        "skyline_supertall_stepped": 22.0,
        "skyline_tapered_braced": 20.0,
        "skyline_glass_tower": 18.0,
        "skyline_artdeco": 17.0,
        "skyline_midrise": 16.0,
    }
    placed = []

    def try_place(kind, x, y, sc_xy, sc_z, yaw):
        if not has.get(kind):
            return False
        z = float(g.z(x, y))
        if z < 2.0 or float(g.at(g.inland, x, y)) < 85.0:
            return False
        r = radii[kind] * sc_xy
        if float(road.dist(x, y)) < r + 32.0:
            return False
        if any(math.hypot(x - px, y - py) < pr + r + 8.0 for px, py, pr in placed):
            return False
        if any(math.hypot(x - tx, y - ty) < tr + r + 6.0 for tx, ty, tr in taken):
            return False
        # Ground using lowest corner of footprint so buried -10 m plinth sits cleanly
        zs = [
            float(g.z(x + dx, y + dy))
            for dx in (-r * 0.65, r * 0.65)
            for dy in (-r * 0.65, r * 0.65)
        ]
        z_base = min(zs)
        out.add(
            "Skyline",
            kind,
            x,
            y,
            z_base,
            yaw,
            (sc_xy, sc_xy, sc_z),
            mark=(110, 175, 255),
        )
        placed.append((x, y, r))
        taken.append((x, y, r + 4.0))
        out.clear.append([round(x, 1), round(y, 1), round(r + 12.0, 1)])
        return True

    # 1. Hero supertalls in the downtown core (Willis-style + Hancock-style anchors)
    hero_spots = [
        ("skyline_supertall_stepped", cx0 - 30.0, cy0 - 20.0, 1.05, 1.08),
        ("skyline_tapered_braced", cx0 + 135.0, cy0 + 25.0, 1.02, 1.05),
        ("skyline_supertall_stepped", cx0 - 180.0, cy0 + 40.0, 0.92, 0.88),
        ("skyline_tapered_braced", cx0 + 265.0, cy0 - 30.0, 0.90, 0.86),
    ]
    for kind, hx2, hy2, sxy, sz in hero_spots:
        for _ in range(18):
            jx, jy = hx2 + rng.normal(0, 28.0), hy2 + rng.normal(0, 28.0)
            if try_place(kind, jx, jy, sxy, sz, float(rng.choice([0, 90, 180, 270]))):
                break

    # 2. Inner high-rise cluster (glass towers + Art Deco towers)
    inner_pool = ["skyline_glass_tower", "skyline_glass_tower", "skyline_artdeco"]
    for _ in range(160):
        if len(placed) >= 18:
            break
        x = cx0 + rng.normal(0, 210.0)
        y = cy0 + rng.normal(0, 110.0)
        kind = inner_pool[int(rng.integers(len(inner_pool)))]
        sxy = float(rng.uniform(0.88, 1.15))
        sz = float(rng.uniform(0.82, 1.22))
        try_place(kind, x, y, sxy, sz, float(rng.choice([0, 90, 180, 270])) + rng.uniform(-8, 8))

    # 3. Outer Lake Shore Drive mid-rises & shoulder towers framing the skyline edge
    outer_pool = ["skyline_midrise", "skyline_midrise", "skyline_artdeco", "skyline_glass_tower"]
    for _ in range(240):
        if len(placed) >= 34:
            break
        x = cx0 + rng.uniform(-460.0, 460.0)
        y = cy0 + rng.uniform(-180.0, 220.0)
        kind = outer_pool[int(rng.integers(len(outer_pool)))]
        sxy = float(rng.uniform(0.85, 1.18))
        sz = float(rng.uniform(0.75, 1.15))
        try_place(kind, x, y, sxy, sz, float(rng.choice([0, 90, 180, 270])) + rng.uniform(-10, 10))

    if placed:
        # Scenic skyline view looking from the harbourfront bay across the trail toward downtown
        ex, ey = hx + 260.0, hy - 220.0
        ez = max(float(g.z(ex, ey)), 0.0) + 18.0
        out.views.append(["feat_skyline", [ex, ey, ez], [cx0, cy0, 68.0]])
    print(f"city_skyline: {len(placed)} towers placed around ({cx0:.0f}, {cy0:.0f})", flush=True)


def main():
    rng = np.random.default_rng(SEED)
    for name in os.listdir(SRC):
        if name.startswith("feat_") and name.endswith(".gltf"):
            os.remove(f"{SRC}/{name}")
    g, bio, road = Ground(), Biomes(), Road()
    shore_distance(g)
    out = Out()
    bridges(out, g, road)
    taken = []  # (x, y, radius m) of what stands, so nothing is placed through it
    harbour(out, g, bio, road, rng, taken)
    city_skyline(out, g, bio, road, np.random.default_rng(SEED + 17), taken)
    farm = turbines(out, g, bio, road, rng, taken)
    # No pylons or power lines since 13 Nov (the AAA review: "out of register with a fantasy
    # world"); power_line() stays for a later setting that wants them.
    taken = taken + rocks(out, g, bio, road, rng)
    cliffs(out, g, bio, road, rng, taken)
    sea_stacks(out, g, bio, road, rng)
    # Their own stream, so adding them moved nothing placed before.
    cairns(out, g, bio, road, np.random.default_rng(SEED + 12), taken)
    hamlets(out, g, bio, road, np.random.default_rng(SEED + 13), taken)
    tower(out, g, bio, road, np.random.default_rng(SEED + 14), taken)
    world_landmarks(out, g, bio, road, np.random.default_rng(SEED + 16), taken)
    roadside_dressing(out, g, bio, road, np.random.default_rng(SEED + 15), taken)
    out.items.sort(key=lambda r: (r[0], r[1]))
    with open(f"{SRC}/features.json", "w") as f:
        json.dump(
            {
                "paints": PAINTS,
                "textures": TEXTURES,
                "clear": out.clear,
                "meshes": out.meshes,
                "groups": out.groups,
                "items": out.items,
                "spin": out.spin,
                "views": out.views,
            },
            f,
        )
    preview(g, road, out)
    print(
        f"{SRC}/features.json: {len(out.items)} placed, {len(out.spin)} rotors,"
        f" {sum(v.startswith('gltf:') for v in out.meshes.values())} meshes made;"
        f" {len(farm)} turbines",
        flush=True,
    )


def _stretch_starts(key=None):
    """{stretch name: [metres where each of its runs starts]} on the default ride, or the ride
    `key`, from info.json."""
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    ride = next((r for r in info.get("rides", []) if r["key"] == key), info["ride"])
    runs = ride["stretches"]
    out, at = {}, 0
    for name, metres in runs:
        out.setdefault(name, []).append(at)
        at += metres
    return out


if __name__ == "__main__":
    main()
