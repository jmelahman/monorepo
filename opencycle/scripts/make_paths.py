"""Writes everything built along the coast ride's path as glTF, in 200 m chunks:
assets_gen/paths/P_<n>.gltf. Each part is a material slot the level builder fills:

    PathTown, PathDike, PathAsphalt   the riding surface, by zone
    Kerb                              the skirt down to the ground either side
    Line                              white edge and centre lines (jungle and coast)
    Gutter, Grate                     the town's covered drains (dobu) and the dark gap under their
    lids
    Rail, RailPost                    the white guard rail on the sea side of the coast road
    Pole, Crossarm, Insulator, Transformer, Wire   utility poles and their sagging lines (town,
                                      paddies,
                                      and the village, where they stand on the stream side)
    Stream, Ishigaki                  the village's stream and the stone bank down to it

Corners go to the writer in the order whose cross product, in Unreal's numbers, points out of the
face, as Mesh.box does. Run with system Python: python3 -I scripts/make_paths.py
"""

import math
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import coast_layout
import gltf_writer

OUT = os.path.join(os.environ.get("OC_GEN") or "/workspace/ue/assets_gen", "paths")
CHUNK = 20000.0
STEP = 100.0
PATH = coast_layout.CoastPath()
SURFACE = {
    "town": "PathTown",
    "paddies": "PathDike",
    "jungle": "PathAsphalt",
    "village": "PathAsphalt",
    "coast": "PathAsphalt",
}
POLE_SPACING = 3500.0


def frame(s):
    x, y, z, yaw = PATH.at(s)
    r = math.radians(yaw)
    return (x, y, z), (math.cos(r), math.sin(r)), (-math.sin(r), math.cos(r))


def half(s):
    return PATH.width(s) / 2


def extrude(mesh, material, s0, s1, profile, uv_tile=100.0, step=STEP, color=(1, 1, 1, 1)):
    """Sweeps a profile of (lateral, dz) points along the path from s0 to s1. Each profile segment
    faces to its left, seen looking down the path: a profile running left to right faces up.
    A profile may be a function of the path's half-width, so it follows the path as it widens
    between zones; a fixed one, taken at a run's middle, would step sideways at every seam."""
    part = mesh.part(material)
    n = max(1, math.ceil((s1 - s0) / step))
    rows = []
    for k in range(n + 1):
        s = s0 + (s1 - s0) * k / n
        (x, y, z), _, right = frame(s)
        rows.append((s, x, y, z, right, profile(half(s)) if callable(profile) else profile))
    for i in range(len(rows[0][5]) - 1):
        ids = []
        for s, x, y, z, right, prof in rows:
            (l0, z0), (l1, z1) = prof[i], prof[i + 1]
            dl, dz = l1 - l0, z1 - z0
            length = math.hypot(dl, dz) or 1.0
            nl, nz = -dz / length, dl / length
            v0 = sum(
                math.hypot(prof[j + 1][0] - prof[j][0], prof[j + 1][1] - prof[j][1])
                for j in range(i)
            )
            normal = (right[0] * nl, right[1] * nl, nz)
            row = []
            for lat, dzz, v in ((l0, z0, v0), (l1, z1, v0 + length)):
                p = (x + right[0] * lat, y + right[1] * lat, z + dzz)
                row.append(part.vertex(p, normal, (s / uv_tile, v / uv_tile), color))
            ids.append(row)
        for k in range(len(ids) - 1):
            a, b = ids[k][0], ids[k + 1][0]
            c, d = ids[k + 1][1], ids[k][1]
            part.quad(a, b, c, d)


def zone_runs(s0, s1, test):
    """[(a, b)] stretches of [s0, s1) where test(zone) holds, at STEP resolution."""
    runs, start = [], None
    s = s0
    while s < s1:
        ok = test(PATH.zone(s + STEP / 2))
        if ok and start is None:
            start = s
        if not ok and start is not None:
            runs.append((start, s))
            start = None
        s += STEP
    if start is not None:
        runs.append((start, s1))
    return runs


def surface(mesh, s0, s1):
    for zone, material in SURFACE.items():
        for a, b in zone_runs(s0, s1, lambda z, zone=zone: z == zone):
            extrude(mesh, material, a, b, lambda w: [(-w, 0), (0, 0.6), (w, 0)], uv_tile=100.0)
            extrude(mesh, "Kerb", a, b, lambda w: [(-w - 18, -30), (-w, 0)], uv_tile=100.0)
            extrude(mesh, "Kerb", a, b, lambda w: [(w, 0), (w + 18, -30)], uv_tile=100.0)


def lines(mesh, s0, s1):
    for a, b in zone_runs(s0, s1, lambda z: z in ("jungle", "village", "coast")):
        for side in (-1, 1):
            extrude(
                mesh,
                "Line",
                a,
                b,
                lambda w, side=side: [(side * (w - 22) - 6, 1.2), (side * (w - 22) + 6, 1.2)],
            )
        # A lane this narrow has edge lines only: no centre line.


def gutters(mesh, s0, s1):
    """Concrete lids over the town's drains, with a dark slot between each and the next."""
    for a, b in zone_runs(s0, s1, lambda z: z == "town"):
        for side in (-1, 1):
            extrude(
                mesh,
                "Grate",
                a,
                b,
                lambda w, side=side: (
                    [(side * w, -6), (side * (w + 40), -6)]
                    if side > 0
                    else [(side * (w + 40), -6), (side * w, -6)]
                ),
            )
        s = a
        while s + 50 <= b:
            (x, y, z), fwd, right = frame(s + 25)
            yaw = math.degrees(math.atan2(fwd[1], fwd[0]))
            w = half(s + 25)
            for side in (-1, 1):
                lat = side * (w + 20)
                mesh.box(
                    "Gutter",
                    (x + right[0] * lat, y + right[1] * lat, z - 3),
                    (49, 38, 8),
                    yaw,
                    uv_scale=100.0,
                )
            s += 50


def rail(mesh, s0, s1):
    """A white W-beam on posts on the sea side of the coast road."""
    for a, b in zone_runs(s0, s1, lambda z: z == "coast"):
        # Front faces the path (its profile runs bottom to top on the right of the path), then the
        # back.
        def front(w):
            lat = w + 30
            return [(lat, 52), (lat - 6, 58), (lat - 2, 66), (lat - 6, 74), (lat, 80)]

        def back(w):
            lat = w + 30
            return [(lat, 80), (lat + 4, 80), (lat + 4, 52), (lat, 52)]

        extrude(mesh, "Rail", a, b, front, step=50.0)
        extrude(mesh, "Rail", a, b, back, step=50.0)
        s = math.ceil(a / 200.0) * 200.0
        while s <= b:
            (x, y, z), fwd, right = frame(s)
            yaw = math.degrees(math.atan2(fwd[1], fwd[0]))
            lat = half(s) + 30
            # Posts driven by hand, years ago: each a little off line, some sunk deeper than others.
            wob = random.Random(int(s))
            off = lat + 10 + wob.uniform(-2.5, 2.5)
            px, py = (
                x + right[0] * off + fwd[0] * wob.uniform(-6, 6),
                y + right[1] * off + fwd[1] * wob.uniform(-6, 6),
            )
            cylinder(mesh, "RailPost", (px, py, z - 30 - wob.uniform(0, 9)), 112, 7, 7, 12)
            # The beam bolts to the post through a spacer block.
            mesh.box(
                "RailPost",
                (x + right[0] * (lat + 3), y + right[1] * (lat + 3), z + 66),
                (14, 6, 26),
                yaw,
            )
            s += 200.0


def stream(mesh, s0, s1):
    """The village stream, a ribbon of water in its channel, and the dry-stone bank (ishigaki) that
    holds the road up above it."""
    lo, hi = coast_layout.STREAM
    depth = coast_layout.STREAM_DEPTH
    for a, b in zone_runs(s0, s1, lambda z: z == "village"):
        extrude(
            mesh, "Stream", a, b, [(lo - 120, -depth + 45), (hi + 300, -depth + 45)], uv_tile=300.0
        )
        # The bank: a battered wall from the road's edge down to the water.
        # It lies a hand's breadth proud of the ground's own slope, from the kerb to the water.
        extrude(
            mesh, "Ishigaki", a, b, lambda w: [(w + 30, -2), (lo + 10, -depth + 35)], uv_tile=200.0
        )
        # A low kerb along the top.
        extrude(
            mesh,
            "Kerb",
            a,
            b,
            lambda w: [(w, 0), (w, 14), (w + 25, 14), (w + 25, -2)],
            uv_tile=100.0,
        )
        # Gravel in front of the inns, from the road's edge to their sills, over the kerb's slope.
        extrude(mesh, "Gravel", a, b, lambda w: [(-w - 110, 1.0), (-w + 2, 1.0)], uv_tile=150.0)


def cylinder(mesh, material, base, height, r0, r1, sides=10):
    part = mesh.part(material)
    x, y, z = base
    rings = []
    for k in range(sides + 1):
        t = 2 * math.pi * k / sides
        c, s = math.cos(t), math.sin(t)
        lo = part.vertex((x + c * r0, y + s * r0, z), (c, s, 0), (k / sides, 0))
        hi = part.vertex(
            (x + c * r1, y + s * r1, z + height), (c, s, 0), (k / sides, height / 100.0)
        )
        rings.append((lo, hi))
    for k in range(sides):
        (a, b), (d, c) = rings[k], rings[k + 1]
        # Outward: going round counter-clockwise from above, then up.
        part.quad(a, d, c, b)
    cap = part.vertex((x, y, z + height), (0, 0, 1), (0.5, 0.5))
    for k in range(sides):
        part.triangle(rings[k][1], rings[k + 1][1], cap)


def poles():
    """Pole positions and their tops: [(s, base xyz, yaw)] along the town and the paddies."""
    result = []
    s = 800.0
    while s < PATH.length:
        zone = PATH.zone(s)
        if zone in ("town", "paddies", "village"):
            w = PATH.width(s) / 2
            (x, y, z), fwd, right = frame(s)
            lat = {"town": -(w + 110), "paddies": -(w + 90), "village": w + 60}[zone]
            ground = PATH.ground(x + right[0] * lat, y + right[1] * lat)
            result.append(
                (
                    s,
                    (x + right[0] * lat, y + right[1] * lat, min(ground, z)),
                    math.degrees(math.atan2(fwd[1], fwd[0])),
                )
            )
        s += POLE_SPACING
    return result


WIRES = [(-70, 1080), (0, 1080), (70, 1080), (-35, 820), (35, 820), (0, 640)]  # (across, height)


def pole(mesh, index, base, yaw):
    x, y, z = base
    cylinder(mesh, "Pole", (x, y, z - 50), 1220, 17, 10)
    r = math.radians(yaw)
    across = (-math.sin(r), math.cos(r))
    mesh.box("Crossarm", (x, y, z + 1080), (9, 170, 9), yaw)
    mesh.box("Crossarm", (x, y, z + 820), (8, 100, 8), yaw)
    for lat, h in WIRES:
        mesh.box(
            "Insulator", (x + across[0] * lat, y + across[1] * lat, z + h + 9), (8, 8, 14), yaw
        )
    if index % 3 == 1:
        cylinder(
            mesh, "Transformer", (x + across[0] * -38, y + across[1] * -38, z + 880), 95, 24, 24, 12
        )
    # A yellow-and-black guard band at the foot, as on every pole by a road.
    cylinder(mesh, "PoleBand", (x, y, z), 180, 17.6, 16.9)


def wire(mesh, p0, p1, sag):
    part = mesh.part("Wire")
    n = 16
    points = []
    for k in range(n + 1):
        t = k / n
        points.append(
            (
                p0[0] + (p1[0] - p0[0]) * t,
                p0[1] + (p1[1] - p0[1]) * t,
                p0[2] + (p1[2] - p0[2]) * t - sag * 4 * t * (1 - t),
            )
        )
    dx, dy = p1[0] - p0[0], p1[1] - p0[1]
    length = math.hypot(dx, dy) or 1.0
    side = (-dy / length * 1.2, dx / length * 1.2)
    for k in range(n):
        a, b = points[k], points[k + 1]
        # Two crossed ribbons so the line has width from any angle; each drawn both ways.
        for off in ((side[0], side[1], 0), (0, 0, 1.2)):
            q = [
                part.vertex((a[0] - off[0], a[1] - off[1], a[2] - off[2])),
                part.vertex((b[0] - off[0], b[1] - off[1], b[2] - off[2])),
                part.vertex((b[0] + off[0], b[1] + off[1], b[2] + off[2])),
                part.vertex((a[0] + off[0], a[1] + off[1], a[2] + off[2])),
            ]
            part.quad(q[0], q[1], q[2], q[3])
            part.quad(q[3], q[2], q[1], q[0])


def main():
    os.makedirs(OUT, exist_ok=True)
    chunks = math.ceil(PATH.length / CHUNK)
    meshes = [gltf_writer.Mesh() for _ in range(chunks)]
    for i, mesh in enumerate(meshes):
        s0, s1 = i * CHUNK, min(PATH.length, (i + 1) * CHUNK)
        surface(mesh, s0, s1)
        lines(mesh, s0, s1)
        gutters(mesh, s0, s1)
        rail(mesh, s0, s1)
        stream(mesh, s0, s1)
    standing = poles()
    for k, (s, base, yaw) in enumerate(standing):
        mesh = meshes[min(chunks - 1, int(s // CHUNK))]
        pole(mesh, k, base, yaw)
        if k + 1 < len(standing) and standing[k + 1][0] - s < POLE_SPACING * 1.5:
            nxt = standing[k + 1]
            for lat, h in WIRES:
                ends = []
                for (bx, by, bz), yw in ((base, yaw), (nxt[1], nxt[2])):
                    r = math.radians(yw)
                    ends.append((bx - math.sin(r) * lat, by + math.cos(r) * lat, bz + h + 16))
                wire(mesh, ends[0], ends[1], 45 + 0.2 * h / 10)
    for i, mesh in enumerate(meshes):
        mesh.write(os.path.join(OUT, f"P_{i:02d}.gltf"))
        print(f"P_{i:02d}", flush=True)
    print(f"{len(standing)} poles")


if __name__ == "__main__":
    main()
