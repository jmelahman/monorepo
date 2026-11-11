"""Writes a stylized test valley: low-poly, flat-shaded meshes coloured per face, no textures.

A small island about 2.4 km across: a meadow valley ringed by a 4 km road, woods on the hills, a
snowy ridge to the north, a beach and a lighthouse to the south, a farm by the road. It is a test of
the look (simple shapes, painted colour) and of what it costs to draw, beside the scanned island.

    world_gen/stylized/terrain.gltf    the ground, coloured by height, slope and patches
    world_gen/stylized/sea.gltf        the sea, turquoise in the shallows, out to the horizon
    world_gen/stylized/road.gltf       the road with its edge lines, centre dashes and verges
    world_gen/stylized/props/*.gltf    trees, bushes, rocks, houses, a barn, a lighthouse, clouds
    world_gen/stylized/place.json      where each prop stands: [mesh, x, y, z, yaw, scale] in cm
    world_gen/stylized/route.json      the road's centreline, 1 m apart, in m

Every mesh has one material slot, Stylized: vertex RGB is the colour and alpha the roughness.
Run with system Python: python3 -I scripts/make_stylized.py
"""

import json
import math
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer

OUT = "/workspace/ue/world_gen/stylized"
SLOT = "Stylized"
HALF = 1300.0  # m, the ground's half width
STEP = 10.0  # m between ground vertices
ROAD_HALF = 3.0  # m, half the tarmac's width
MAX_GRADE = 0.06


def lin(rgb, rough=0.9):
    """An sRGB colour as the linear vertex colour the material reads."""
    return (*(c**2.2 for c in rgb), rough)


def smooth(a, b, x):
    t = min(1.0, max(0.0, (x - a) / (b - a)))
    return t * t * (3 - 2 * t)


def mix(a, b, t):
    return tuple(p + (q - p) * t for p, q in zip(a, b, strict=True))


def shade(rgb, k):
    return tuple(min(1.0, c * k) for c in rgb)


def noise(x, y, seed=0.0):
    """Smooth patches about 100 m across, -1..1: a few crossed sines are plenty here."""
    return (
        math.sin(x / 97 + seed) * math.cos(y / 83 - seed * 1.7)
        + 0.6 * math.sin((x + y) / 51 + seed * 2.3)
        + 0.4 * math.cos((x - 2 * y) / 37 + seed * 0.7)
    ) / 2.0


def land(x, y):
    """Height in m before the road is cut in; below 0 is sea."""
    cy = -250.0
    angle = math.atan2(y - cy, x)
    radius = math.hypot(x, (y - cy) / 0.85)
    edge = 920 + 70 * math.sin(3 * angle + 0.4) + 40 * math.sin(5 * angle + 1.9)
    inland = smooth(edge + 140, edge - 160, radius)
    hills = (
        13 * math.sin(x / 160 + 1.3) * math.sin(y / 140 + 0.4)
        + 7 * math.sin(x / 73 + y / 91)
        + 3 * math.sin(x / 37 - 2) * math.cos(y / 45)
    )
    ridge = smooth(-300, -780, y) * (190 + 70 * math.sin(x / 190 + 0.8) + 25 * math.sin(x / 61))
    return -14 + (26 + hills + ridge) * inland


# The road ----------------------------------------------------------------------------------------


def centreline():
    """The loop, 1 m apart: (x, y, z) in m, its grade held under MAX_GRADE."""
    ring = []
    for k in range(4000):
        t = 2 * math.pi * k / 4000
        rx = 500 + 45 * math.sin(3 * t + 0.5) + 20 * math.sin(7 * t)
        ry = 330 + 35 * math.sin(2 * t + 1.0)
        ring.append((rx * math.cos(t), ry * math.sin(t)))
    pts = [ring[0]]
    for p in ring[1:] + ring[:1]:
        last = pts[-1]
        d = math.dist(p, last)
        while d >= 1.0:
            f = 1.0 / d
            last = (last[0] + (p[0] - last[0]) * f, last[1] + (p[1] - last[1]) * f)
            pts.append(last)
            d = math.dist(p, last)
    pts.pop()  # it ends where it started
    n = len(pts)
    raw = [max(3.0, land(x, y)) for x, y in pts]
    window = 80
    z = [
        sum(raw[(i + j) % n] for j in range(-window, window + 1)) / (2 * window + 1)
        for i in range(n)
    ]
    for _ in range(6):  # forward and back round the loop until the grade holds both ways
        for i in range(2 * n):
            a, b = z[(i - 1) % n], z[i % n]
            z[i % n] = min(max(b, a - MAX_GRADE), a + MAX_GRADE)
        for i in range(2 * n, 0, -1):
            a, b = z[(i + 1) % n], z[i % n]
            z[i % n] = min(max(b, a - MAX_GRADE), a + MAX_GRADE)
    return [(x, y, h) for (x, y), h in zip(pts, z, strict=True)]


class RoadIndex:
    """The nearest centreline point to anywhere, through a grid of 20 m cells."""

    CELL = 20.0

    def __init__(self, line):
        self.line = line
        self.cells = {}
        for i, (x, y, _) in enumerate(line):
            self.cells.setdefault((int(x // self.CELL), int(y // self.CELL)), []).append(i)

    def nearest(self, x, y, reach=60.0):
        """(distance, index), or (inf, None) beyond reach."""
        best, found = math.inf, None
        r = int(reach // self.CELL) + 1
        cx, cy = int(x // self.CELL), int(y // self.CELL)
        for i in range(cx - r, cx + r + 1):
            for j in range(cy - r, cy + r + 1):
                for k in self.cells.get((i, j), ()):
                    px, py, _ = self.line[k]
                    d = (px - x) ** 2 + (py - y) ** 2
                    if d < best:
                        best, found = d, k
        return math.sqrt(best), found


def ground(x, y, road):
    """Height with the road's bed cut in: flat under the road, easing back by 35 m out."""
    h = land(x, y)
    d, k = road.nearest(x, y, 40.0)
    if k is None:
        return h
    bed = road.line[k][2] - 0.3
    return bed + (h - bed) * smooth(9.0, 35.0, d)


# The ground --------------------------------------------------------------------------------------

MEADOW = (0.42, 0.66, 0.27)
MEADOW_LIGHT = (0.60, 0.75, 0.30)
MEADOW_DEEP = (0.27, 0.52, 0.24)
WOOD_FLOOR = (0.24, 0.42, 0.20)
DRY = (0.66, 0.66, 0.38)
ROCK = (0.56, 0.54, 0.52)
SNOW = (0.94, 0.96, 0.99)
SAND = (0.91, 0.83, 0.62)
SEABED = (0.62, 0.70, 0.55)


def ground_colour(cx, cy, cz, slope, wood):
    if cz < 0.6:
        return SEABED if cz < -1.5 else SAND
    if cz < 2.6:
        return mix(SAND, MEADOW, smooth(1.6, 2.6, cz))
    patch = noise(cx, cy, 1.0)
    c = mix(MEADOW, MEADOW_LIGHT if patch > 0 else MEADOW_DEEP, min(1.0, abs(patch) * 1.4))
    c = mix(c, WOOD_FLOOR, wood)
    c = mix(c, DRY, smooth(70, 140, cz + 15 * noise(cx, cy, 4.0)))
    c = mix(c, ROCK, smooth(0.55, 0.75, slope))
    snow_line = 175 + 12 * noise(cx, cy, 6.0)
    if cz > snow_line and slope < 0.85:
        c = SNOW
    return c


def woodland(x, y):
    """0..1, how wooded: patches on the hills, thicker towards the ridge."""
    return smooth(0.05, 0.45, noise(x, y, 2.5) + 0.35 * smooth(150, -500, y))


def face_normal(a, b, c):
    ux, uy, uz = (b[i] - a[i] for i in range(3))
    vx, vy, vz = (c[i] - a[i] for i in range(3))
    n = (uy * vz - uz * vy, uz * vx - ux * vz, ux * vy - uy * vx)
    length = math.sqrt(sum(v * v for v in n)) or 1.0
    return tuple(v / length for v in n)


def flat_triangle(part, a, b, c, rgb, rough=0.9, jitter=0.0, rng=None):
    """One face, its own three vertices, so it shades flat. a, b, c in m, anticlockwise from
    above (UE's left-handed view), as gltf_writer wants."""
    n = face_normal(a, b, c)
    if n[2] < 0:
        b, c = c, b
        n = face_normal(a, b, c)
    k = 1.0 + (rng.uniform(-jitter, jitter) if rng else 0.0)
    colour = lin(shade(rgb, k), rough)
    ids = [part.vertex(tuple(v * 100 for v in p), n, (0, 0), colour) for p in (a, b, c)]
    part.triangle(*ids)


def write_terrain(road):
    rng = random.Random(7)
    n = int(2 * HALF / STEP) + 1
    xs = [-HALF + i * STEP for i in range(n)]
    heights = [[ground(x, y, road) for x in xs] for y in xs]
    mesh = gltf_writer.Mesh()
    part = mesh.part(SLOT)
    for j in range(n - 1):
        for i in range(n - 1):
            x0, x1, y0, y1 = xs[i], xs[i + 1], xs[j], xs[j + 1]
            p00 = (x0, y0, heights[j][i])
            p10 = (x1, y0, heights[j][i + 1])
            p01 = (x0, y1, heights[j + 1][i])
            p11 = (x1, y1, heights[j + 1][i + 1])
            if max(p[2] for p in (p00, p10, p01, p11)) < -6:
                continue  # deep under the sea
            # Alternate the diagonal so the facets don't all lean one way.
            tris = (
                ((p00, p01, p11), (p00, p11, p10))
                if (i + j) % 2
                else ((p00, p01, p10), (p10, p01, p11))
            )
            for a, b, c in tris:
                n_ = face_normal(a, b, c)
                n_ = n_ if n_[2] >= 0 else tuple(-v for v in n_)
                cx, cy, cz = ((a[k] + b[k] + c[k]) / 3 for k in range(3))
                slope = math.sqrt(max(0.0, 1 - n_[2] ** 2))
                colour = ground_colour(cx, cy, cz, slope, woodland(cx, cy) * 0.7)
                flat_triangle(part, a, b, c, colour, 0.95, 0.035, rng)
    mesh.write(f"{OUT}/terrain.gltf")
    return heights, xs


def write_sea():
    """A grid out to 4 km, its colour from the depth of the ground beneath, then a skirt of big
    quads to 40 km so the horizon is all sea."""
    mesh = gltf_writer.Mesh()
    part = mesh.part(SLOT)
    shallow, deep, far = (0.30, 0.78, 0.78), (0.10, 0.42, 0.62), (0.08, 0.32, 0.52)
    reach, step = 4000.0, 50.0
    n = int(2 * reach / step) + 1
    coords = [-reach + i * step for i in range(n)]

    def colour(x, y):
        depth = -land(x, y) if max(abs(x), abs(y)) <= HALF else 60.0
        return lin(mix(shallow, deep, smooth(2, 16, depth)), 0.08)

    ids = [
        [part.vertex((x * 100, y * 100, 0), (0, 0, 1), (0, 0), colour(x, y)) for x in coords]
        for y in coords
    ]
    for j in range(n - 1):
        for i in range(n - 1):
            part.quad(ids[j][i], ids[j][i + 1], ids[j + 1][i + 1], ids[j + 1][i])
    big = 40000.0
    ring_in = [(-reach, -reach), (reach, -reach), (reach, reach), (-reach, reach)]
    ring_out = [(-big, -big), (big, -big), (big, big), (-big, big)]
    c = lin(far, 0.08)
    for k in range(4):
        a, b = ring_in[k], ring_in[(k + 1) % 4]
        d, e = ring_out[k], ring_out[(k + 1) % 4]
        vs = [part.vertex((p[0] * 100, p[1] * 100, 0), (0, 0, 1), (0, 0), c) for p in (a, d, e, b)]
        part.quad(*vs)
    mesh.write(f"{OUT}/sea.gltf")


def write_road(line):
    """Tarmac 6 m wide, white edge lines, a dashed centre line and gravel verges, every 2 m."""
    mesh = gltf_writer.Mesh()
    part = mesh.part(SLOT)
    tarmac, verge, paint = (
        lin((0.30, 0.31, 0.34), 0.75),
        lin((0.62, 0.57, 0.46)),
        lin((0.95, 0.95, 0.92), 0.6),
    )
    n = len(line)
    idx = list(range(0, n, 2))

    def frame(i):
        x, y, z = line[i]
        ax, ay, _ = line[(i - 2) % n]
        bx, by, _ = line[(i + 2) % n]
        tx, ty = bx - ax, by - ay
        length = math.hypot(tx, ty) or 1.0
        return x, y, z, -ty / length, tx / length  # point and its left

    def strip(offsets, colour, lift=0.0, keep=None):
        """A band across the road between two offsets (m, left positive) and heights (m)."""
        (o0, h0), (o1, h1) = offsets
        for a, b in zip(idx, idx[1:] + idx[:1], strict=True):
            if keep and not keep(a):
                continue
            quad = []
            for i, o, h in ((a, o0, h0), (a, o1, h1), (b, o1, h1), (b, o0, h0)):
                x, y, z, lx, ly = frame(i)
                quad.append(
                    part.vertex(
                        ((x + lx * o) * 100, (y + ly * o) * 100, (z + h + lift) * 100),
                        (0, 0, 1),
                        (0, 0),
                        colour,
                    )
                )
            part.quad(*quad)

    strip(((ROAD_HALF, 0.0), (-ROAD_HALF, 0.0)), tarmac)
    strip(((ROAD_HALF + 1.4, -0.35), (ROAD_HALF, 0.0)), verge)
    strip(((-ROAD_HALF, 0.0), (-ROAD_HALF - 1.4, -0.35)), verge)
    for o in (ROAD_HALF - 0.25, -ROAD_HALF + 0.4):
        strip(((o, 0.0), (o - 0.15, 0.0)), paint, 0.015)
    strip(((0.07, 0.0), (-0.07, 0.0)), paint, 0.015, keep=lambda i: (i // 2) % 5 < 2)
    mesh.write(f"{OUT}/road.gltf")


# The props ---------------------------------------------------------------------------------------


def _midpoint(verts, mids, a, b):
    """The index of the unit vector halfway between verts a and b, added once per edge."""
    key = (min(a, b), max(a, b))
    if key not in mids:
        p = tuple((verts[a][k] + verts[b][k]) / 2 for k in range(3))
        length = math.sqrt(sum(v * v for v in p))
        verts.append(tuple(v / length for v in p))
        mids[key] = len(verts) - 1
    return mids[key]


def icosphere(subdiv=1):
    t = (1 + math.sqrt(5)) / 2
    verts = [
        (-1, t, 0), (1, t, 0), (-1, -t, 0), (1, -t, 0), (0, -1, t), (0, 1, t),
        (0, -1, -t), (0, 1, -t), (t, 0, -1), (t, 0, 1), (-t, 0, -1), (-t, 0, 1),
    ]  # fmt: skip
    faces = [
        (0, 11, 5), (0, 5, 1), (0, 1, 7), (0, 7, 10), (0, 10, 11), (1, 5, 9), (5, 11, 4),
        (11, 10, 2), (10, 7, 6), (7, 1, 8), (3, 9, 4), (3, 4, 2), (3, 2, 6), (3, 6, 8),
        (3, 8, 9), (4, 9, 5), (2, 4, 11), (6, 2, 10), (8, 6, 7), (9, 8, 1),
    ]  # fmt: skip
    verts = [tuple(c / math.sqrt(sum(v * v for v in p)) for c in p) for p in verts]
    for _ in range(subdiv):
        mids, out = {}, []
        for a, b, c in faces:
            ab, bc, ca = (_midpoint(verts, mids, i, j) for i, j in ((a, b), (b, c), (c, a)))
            out += [(a, ab, ca), (b, bc, ab), (c, ca, bc), (ab, bc, ca)]
        faces = out
    return verts, faces


def blob(
    part, centre, radius, rgb, rng, squash=1.0, lumpy=0.18, subdiv=1, rough=0.9, top_light=0.18
):
    """A faceted lump: an icosphere with its vertices pushed in and out. Faces facing up are a
    little lighter, as painted foliage is."""
    verts, faces = icosphere(subdiv)
    pts = [
        (
            centre[0] + v[0] * radius * (1 + rng.uniform(-lumpy, lumpy)),
            centre[1] + v[1] * radius * (1 + rng.uniform(-lumpy, lumpy)),
            centre[2] + v[2] * radius * squash * (1 + rng.uniform(-lumpy, lumpy)),
        )
        for v in verts
    ]
    for a, b, c in faces:
        p, q, r = pts[a], pts[b], pts[c]
        n = face_normal(p, q, r)
        mid = [(p[k] + q[k] + r[k]) / 3 - centre[k] for k in range(3)]
        if sum(n[k] * mid[k] for k in range(3)) < 0:
            q, r = r, q
            n = face_normal(p, q, r)
        k = 1.0 + top_light * n[2] + rng.uniform(-0.04, 0.04)
        colour = lin(shade(rgb, k), rough)
        ids = [part.vertex(tuple(v * 100 for v in s), n, (0, 0), colour) for s in (p, q, r)]
        part.triangle(*ids)


def prism(part, x, y, z0, z1, r0, r1, sides, rgb, rng=None, rough=0.9, cap=True):
    """A tapered n-sided prism or cone (r1 = 0) from z0 to z1, flat-shaded."""
    ring0 = [
        (x + r0 * math.cos(2 * math.pi * k / sides), y + r0 * math.sin(2 * math.pi * k / sides), z0)
        for k in range(sides)
    ]
    ring1 = [
        (x + r1 * math.cos(2 * math.pi * k / sides), y + r1 * math.sin(2 * math.pi * k / sides), z1)
        for k in range(sides)
    ]
    for k in range(sides):
        a, b = ring0[k], ring0[(k + 1) % sides]
        c, d = ring1[(k + 1) % sides], ring1[k]
        for tri in ((a, d, c), (a, c, b)) if r1 > 0 else ((a, d, b),):
            n = face_normal(*tri)
            if n[0] * (tri[0][0] - x) + n[1] * (tri[0][1] - y) < 0:
                tri = (tri[0], tri[2], tri[1])
                n = face_normal(*tri)
            k_ = 1.0 + 0.12 * n[2] + (rng.uniform(-0.04, 0.04) if rng else 0.0)
            colour = lin(shade(rgb, k_), rough)
            part.triangle(*[part.vertex(tuple(v * 100 for v in s), n, (0, 0), colour) for s in tri])
    if cap and r1 > 0:
        top = (x, y, z1)
        for k in range(sides):
            tri = (top, ring1[k], ring1[(k + 1) % sides])
            if face_normal(*tri)[2] < 0:
                tri = (top, ring1[(k + 1) % sides], ring1[k])
            part.triangle(
                *[
                    part.vertex(tuple(v * 100 for v in s), (0, 0, 1), (0, 0), lin(rgb, rough))
                    for s in tri
                ]
            )


def box(part, x0, y0, z0, x1, y1, z1, rgb, rough=0.9):
    c = lin(rgb, rough)
    corners = lambda z: [(x0, y0, z), (x1, y0, z), (x1, y1, z), (x0, y1, z)]  # noqa: E731
    lo, hi = corners(z0), corners(z1)
    faces = [(hi[0], hi[1], hi[2], hi[3]), (lo[3], lo[2], lo[1], lo[0])]
    for k in range(4):
        faces.append((lo[k], lo[(k + 1) % 4], hi[(k + 1) % 4], hi[k]))
    cx, cy, cz = (x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2
    for quad in faces:
        n = face_normal(quad[0], quad[1], quad[2])
        out = sum(n[i] * (quad[0][i] - (cx, cy, cz)[i]) for i in range(3))
        if out < 0:
            quad = quad[::-1]
            n = face_normal(quad[0], quad[1], quad[2])
        ids = [part.vertex(tuple(v * 100 for v in p), n, (0, 0), c) for p in quad]
        part.triangle(ids[0], ids[1], ids[2])
        part.triangle(ids[0], ids[2], ids[3])


def gable(part, x0, y0, x1, y1, z0, ridge, overhang, rgb, rough=0.85):
    """A pitched roof over the box x0..x1, y0..y1 from eaves z0 to a ridge along x."""
    o = overhang
    ym = (y0 + y1) / 2
    a, b = (x0 - o, y0 - o, z0), (x1 + o, y0 - o, z0)
    c, d = (x1 + o, ym, ridge), (x0 - o, ym, ridge)
    e, f = (x1 + o, y1 + o, z0), (x0 - o, y1 + o, z0)
    roof = lin(rgb, rough)
    for quad in ((a, b, c, d), (f, d, c, e)):
        n = face_normal(quad[0], quad[1], quad[2])
        if n[2] < 0:
            quad = quad[::-1]
            n = face_normal(quad[0], quad[1], quad[2])
        ids = [part.vertex(tuple(v * 100 for v in p), n, (0, 0), roof) for p in quad]
        part.triangle(ids[0], ids[1], ids[2])
        part.triangle(ids[0], ids[2], ids[3])
    return ym


def gable_ends(part, x0, x1, y0, y1, z0, ridge, rgb):
    ym = (y0 + y1) / 2
    for x in (x0, x1):
        tri = ((x, y0, z0), (x, y1, z0), (x, ym, ridge))
        n = face_normal(*tri)
        if n[0] * (x - (x0 + x1) / 2) < 0:
            tri = (tri[0], tri[2], tri[1])
            n = face_normal(*tri)
        part.triangle(*[part.vertex(tuple(v * 100 for v in p), n, (0, 0), lin(rgb)) for p in tri])


def write_prop(name, build):
    mesh = gltf_writer.Mesh()
    build(mesh.part(SLOT), random.Random(name))
    mesh.write(f"{OUT}/props/{name}.gltf")


TRUNK = (0.45, 0.31, 0.20)
LEAF = {
    "green": (0.30, 0.58, 0.22),
    "lime": (0.52, 0.70, 0.22),
    "amber": (0.88, 0.58, 0.18),
}
PINE = (0.16, 0.40, 0.27)


def round_tree(colour):
    def build(part, rng):
        prism(part, 0, 0, -0.5, 4.2, 0.38, 0.26, 6, TRUNK, rng)
        rgb = LEAF[colour]
        blob(part, (0, 0, 6.2), 3.0, rgb, rng, 0.9)
        blob(part, (1.5, 0.6, 5.0), 2.0, shade(rgb, 0.92), rng, 0.9)
        blob(part, (-1.2, -0.9, 5.4), 2.2, shade(rgb, 1.04), rng, 0.9)

    return build


def pine(part, rng):
    prism(part, 0, 0, -0.5, 2.6, 0.32, 0.22, 6, TRUNK, rng)
    for k, (z, r) in enumerate(((2.0, 2.8), (4.4, 2.3), (6.6, 1.7), (8.6, 1.1))):
        prism(part, 0, 0, z, z + 3.6 - 0.3 * k, r, 0.0, 7, shade(PINE, 1 + 0.05 * k), rng)


def bush(part, rng):
    blob(part, (0, 0, 0.6), 1.2, (0.34, 0.55, 0.24), rng, 0.75, 0.22, top_light=0.2)


def rock(part, rng):
    blob(part, (0, 0, 0.4), 1.5, ROCK, rng, 0.65, 0.3, 0, 0.95, 0.12)


def cloud(part, rng):
    for _ in range(7):
        r = rng.uniform(12, 24)
        blob(
            part,
            (rng.uniform(-35, 35), rng.uniform(-14, 14), rng.uniform(0, 8)),
            r,
            (0.98, 0.98, 1.0),
            rng,
            0.6,
            0.12,
            1,
            1.0,
            0.08,
        )


def cottage(walls, roof):
    def build(part, _rng):
        box(part, -5, -3.5, -1.0, 5, 3.5, 3.2, walls)
        gable(part, -5, -3.5, 5, 3.5, 3.2, 6.4, 0.5, roof)
        gable_ends(part, -5, 5, -3.5, 3.5, 3.2, 6.4, walls)
        box(part, 2.2, -1.0, 4.2, 3.2, 0.0, 7.2, (0.70, 0.40, 0.32))  # chimney
        dark = (0.18, 0.22, 0.28)
        box(part, -0.6, -3.62, 0.0, 0.6, -3.45, 2.1, (0.42, 0.26, 0.16))  # door
        for x in (-3.4, 2.6):
            box(part, x - 0.6, -3.6, 1.0, x + 0.6, -3.45, 2.2, dark, 0.3)
            box(part, x - 0.6, 3.45, 1.0, x + 0.6, 3.6, 2.2, dark, 0.3)

    return build


def barn(part, _rng):
    red, white = (0.66, 0.20, 0.16), (0.94, 0.92, 0.86)
    box(part, -9, -6, -1.0, 9, 6, 5.0, red)
    gable(part, -9, -6, 9, 6, 5.0, 10.0, 0.6, (0.32, 0.30, 0.30))
    gable_ends(part, -9, 9, -6, 6, 5.0, 10.0, red)
    box(part, -9.1, -2.2, 0.0, -8.95, 2.2, 4.0, white)  # doors
    box(part, -9.2, -0.1, 0.0, -9.05, 0.1, 4.0, red)


def lighthouse(part, rng):
    for k in range(6):
        z0 = k * 3.2
        r0, r1 = 3.0 - 0.22 * k, 3.0 - 0.22 * (k + 1)
        prism(
            part,
            0,
            0,
            z0 - (1.0 if k == 0 else 0),
            z0 + 3.2,
            r0,
            r1,
            12,
            (0.85, 0.18, 0.16) if k % 2 else (0.97, 0.96, 0.93),
            rng,
            cap=False,
        )
    prism(part, 0, 0, 19.2, 19.6, 2.6, 2.6, 12, (0.20, 0.22, 0.24))
    prism(part, 0, 0, 19.6, 21.4, 1.5, 1.5, 12, (1.0, 0.92, 0.55), rough=0.2, cap=False)
    prism(part, 0, 0, 21.4, 23.2, 1.9, 0.0, 12, (0.20, 0.22, 0.24))
    box(part, 2.0, -3.0, -1.0, 8.0, 3.0, 3.0, (0.97, 0.96, 0.93))
    gable(part, 2.0, -3.0, 8.0, 3.0, 3.0, 5.0, 0.3, (0.85, 0.18, 0.16))
    gable_ends(part, 2.0, 8.0, -3.0, 3.0, 3.0, 5.0, (0.97, 0.96, 0.93))


def fence(part, _rng):
    wood = (0.62, 0.48, 0.34)
    for x in (-2.0, 2.0):
        box(part, x - 0.08, -0.08, -0.4, x + 0.08, 0.08, 1.2, wood)
    for z in (0.55, 1.0):
        box(part, -2.05, -0.04, z, 2.05, 0.04, z + 0.12, wood)


PROPS = {
    "tree_green": round_tree("green"),
    "tree_lime": round_tree("lime"),
    "tree_amber": round_tree("amber"),
    "pine": pine,
    "bush": bush,
    "rock": rock,
    "cloud": cloud,
    "cottage_white": cottage((0.95, 0.93, 0.86), (0.76, 0.33, 0.22)),
    "cottage_yellow": cottage((0.96, 0.84, 0.55), (0.36, 0.40, 0.48)),
    "barn": barn,
    "lighthouse": lighthouse,
    "fence": fence,
}


# Placing them ------------------------------------------------------------------------------------


def place(road, line):
    rng = random.Random(11)
    out = []

    def put(name, x, y, yaw=None, scale=1.0, z=None):
        h = ground(x, y, road) if z is None else z
        out.append(
            [
                name,
                round(x * 100),
                round(y * 100),
                round(h * 100),
                round(rng.uniform(0, 360) if yaw is None else yaw, 1),
                round(scale, 3),
            ]
        )

    def clear(x, y, gap):
        return road.nearest(x, y, gap + 1)[0] > gap

    # Woods: a jittered 7 m grid kept where the patches are thick, off the road and the beach.
    for gx in range(int(-HALF), int(HALF), 7):
        for gy in range(int(-HALF), int(HALF), 7):
            x, y = gx + rng.uniform(-3, 3), gy + rng.uniform(-3, 3)
            h = land(x, y)
            if h < 3.5 or h > 168:
                continue
            w = woodland(x, y)
            if rng.random() > w * 0.85 or not clear(x, y, 9):
                continue
            if h > 70 or y < -420:
                name = "pine" if rng.random() < 0.9 else "tree_green"
            else:
                name = rng.choices(("tree_green", "tree_lime", "tree_amber", "pine"), (5, 3, 1, 2))[
                    0
                ]
            put(name, x, y, scale=rng.uniform(0.8, 1.3))
    # Lone trees and bushes across the meadows, which the woods leave open.
    for _ in range(900):
        x, y = rng.uniform(-HALF, HALF), rng.uniform(-HALF, HALF)
        h = land(x, y)
        if h < 3.5 or h > 120 or woodland(x, y) > 0.3 or not clear(x, y, 7):
            continue
        if rng.random() < 0.25:
            put(rng.choice(("tree_green", "tree_lime")), x, y, scale=rng.uniform(0.9, 1.4))
        else:
            put("bush", x, y, scale=rng.uniform(0.7, 1.5))
    # Rocks on the steep ground and along the shore.
    for _ in range(1400):
        x, y = rng.uniform(-HALF, HALF), rng.uniform(-HALF, HALF)
        h = land(x, y)
        steep = abs(land(x + 4, y) - land(x - 4, y)) + abs(land(x, y + 4) - land(x, y - 4))
        if (steep > 4 or -0.5 < h < 2.5) and h > -1 and clear(x, y, 6):
            put("rock", x, y, scale=rng.uniform(0.6, 2.6))
    # The farm and a hamlet beside the road, facing it.
    for frac, name, side in (
        (0.18, "barn", 1), (0.205, "cottage_white", 1), (0.22, "cottage_yellow", -1),
        (0.62, "cottage_white", 1), (0.635, "cottage_yellow", 1), (0.65, "cottage_white", -1),
        (0.665, "cottage_yellow", -1),
    ):  # fmt: skip
        i = int(frac * len(line))
        x, y, z = line[i]
        ax, ay, _ = line[i - 3]
        bx, by, _ = line[(i + 3) % len(line)]
        yaw = math.degrees(math.atan2(by - ay, bx - ax))
        lx, ly = -(by - ay), bx - ax
        length = math.hypot(lx, ly)
        off = 20 if name == "barn" else 15
        put(
            name,
            x + side * lx / length * off,
            y + side * ly / length * off,
            yaw + (0 if side > 0 else 180),
            z=z - 0.3,
        )
        # A fence along the road in front of it.
        for k in range(-3, 4):
            fx = x + side * lx / length * 6.2 + (bx - ax) / 6 * 4 * k
            fy = y + side * ly / length * 6.2 + (by - ay) / 6 * 4 * k
            put("fence", fx, fy, yaw, z=z - 0.3)
    # The lighthouse on the south shore.
    for x in range(-200, 201, 4):
        for y in range(380, 700, 4):
            if land(x, y) < 6:
                put("lighthouse", x, y - 25, 0, z=land(x, y - 25))
                break
        else:
            continue
        break
    # Clouds.
    for _ in range(36):
        a, r = rng.uniform(0, 2 * math.pi), rng.uniform(300, 3500)
        out.append(
            [
                "cloud",
                round(r * math.cos(a) * 100),
                round(r * math.sin(a) * 100),
                round(rng.uniform(380, 650) * 100),
                round(rng.uniform(0, 360), 1),
                round(rng.uniform(0.7, 1.6), 2),
            ]
        )
    return out


def main():
    os.makedirs(f"{OUT}/props", exist_ok=True)
    line = centreline()
    road = RoadIndex(line)
    grades = [abs(line[(i + 50) % len(line)][2] - line[i][2]) / 50 for i in range(len(line))]
    low, high = min(p[2] for p in line), max(p[2] for p in line)
    print(f"road {len(line)} m, {low:.0f}-{high:.0f} m high, steepest {max(grades) * 100:.1f}%")
    write_terrain(road)
    write_sea()
    write_road(line)
    for name, build in PROPS.items():
        write_prop(name, build)
    placed = place(road, line)
    counts = {}
    for p in placed:
        counts[p[0]] = counts.get(p[0], 0) + 1
    print("placed", counts)
    with open(f"{OUT}/place.json", "w") as f:
        json.dump(placed, f)
    with open(f"{OUT}/route.json", "w") as f:
        json.dump([[round(v, 3) for v in p] for p in line], f)


if __name__ == "__main__":
    main()
