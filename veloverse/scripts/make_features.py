"""The island chain's landmarks, from make_world_level.py's output: rock formations by biome,
wind turbines on The Moor, power lines, the harbour (piers, boats, a quay and a lighthouse) and
the five bridges. Writes, into the ride folder (OC_SRC, default /workspace/ue/world_gen/ride_chain):

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
import sys

import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage
from scipy.spatial import cKDTree

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import itertools

import gltf_writer

PLAN = os.environ.get("OC_PLAN", "/workspace/ue/world_gen/chain")
SRC = os.environ.get("OC_SRC", "/workspace/ue/world_gen/ride_chain")
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
        (ms("desert", "huge_canyon_sandstone_mesa"), 16.6, (1.8, 3.5), 220, 0.8, 0, 0),
        (ms("desert", "canyon_sandstone_hoodoo"), 20.9, (0.6, 1.1), 60, 1.2, 0, 0),
        (ms("desert", "desert_western_spire_layered_xl_04"), 27.9, (0.5, 0.9), 80, 0.8, 0, 0),
        (ms("desert", "sandstone_rock_formation"), 3.6, (1.0, 2.4), 12, 5.0, 0, 0),
        (ms("desert", "canyon_sandstone_boulder"), 1.0, (1.5, 3.5), 8, 6.0, 0, 0),
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
                *(round(v * 100, 1) for v in (x, y, z)),
                round(float(yaw), 2),
                [round(float(v), 3) for v in scale]
                if isinstance(scale, tuple)
                else round(scale, 3),
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
        self.line = np.array(r["island"], dtype=float)
        self.spans = r.get("bridges", [])
        self.tree = cKDTree(self.line[::5, :2])

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


def power_line(out, g, road, rng, taken, name, a, b, side):
    """Pylons ~280 m apart, set off the road by `side` m along the road's points a..b (metres
    along it), and the cables between them as one mesh built where it stands."""
    line = road.line
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


def harbour(out, g, bio, road, rng, taken):
    """Piers into the bay with boats tied up beside them, more at anchor, a stone quay, and a
    lighthouse on the bay's outer point."""
    out.group("Harbour", cull=0)
    out.group("Quay", cull=0)
    hx, hy = bio.world["landmarks"]["harbour"]
    for name, spec in BOATS.items():
        out.gltf(f"boat_{name}", boat(*spec))
    for length in (60, 90):
        out.gltf(f"pier_{length}", pier(length))
    out.gltf("lighthouse", lighthouse())
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
    print(f"harbour: {len(piers)} piers, {boats} boats, quay {len(quay) // 2} blocks", flush=True)


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
            sweep(
                steel,
                local,
                [
                    (sgn * (edge + 5), 100),
                    (sgn * (edge + 5), 110),
                    (sgn * (edge + 20), 110),
                    (sgn * (edge + 20), 100),
                ][::sgn],
            )
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


def main():
    rng = np.random.default_rng(SEED)
    for name in os.listdir(SRC):
        if name.startswith("feat_") and name.endswith(".gltf"):
            os.remove(f"{SRC}/{name}")
    g, bio, road = Ground(), Biomes(), Road()
    out = Out()
    bridges(out, g, road)
    taken = []  # (x, y, radius m) of what stands, so nothing is placed through it
    harbour(out, g, bio, road, rng, taken)
    farm = turbines(out, g, bio, road, rng, taken)
    out.group("Pylons", cull=0)
    out.group("Cables", cull=0, shadows=False)
    out.gltf("pylon", pylon())
    n = len(road.line)
    # Lines: across Harbour Isle from the town; out of The Moor's wind farm; along Greenwood.
    stretch_at = _stretch_starts()
    for name, (start, length, side) in {
        "harbour": (stretch_at["Farmland"][1] + 1500, 6000, 160),
        "moor": (stretch_at["The Moor"][0] + 2500, 9000, -220),
        "greenwood": (stretch_at["Greenwood"][0] + 4000, 7000, 140),
    }.items():
        power_line(out, g, road, rng, taken, name, int(start) % n, int(start) % n + length, side)
    rocks(out, g, bio, road, rng)
    out.items.sort(key=lambda r: (r[0], r[1]))
    with open(f"{SRC}/features.json", "w") as f:
        json.dump(
            {
                "paints": PAINTS,
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


def _stretch_starts():
    """{stretch name: [metres where each of its runs starts]} from info.json."""
    with open(f"{SRC}/info.json") as f:
        runs = json.load(f)["ride"]["stretches"]
    out, at = {}, 0
    for name, metres in runs:
        out.setdefault(name, []).append(at)
        at += metres
    return out


if __name__ == "__main__":
    main()
