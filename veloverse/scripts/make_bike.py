"""The rider's road bike, built from its geometry, for the ride's bike views.

Writes to OUT (OC_BIKE_OUT, default /workspace/ue/world_gen/rider):
- bike.gltf: the whole bike, for the view from behind. Its origin is on the road under the rider's
  eyes, +X forward, +Z up, in cm: RidePawn puts it at the riding line's point at road level.
- cockpit.gltf: the front end the rider sees over the bars (bars, hoods, stem, computer, head
  tube, fork and front wheel), its origin at the rider's eyes, EYE cm up. RidePawn attaches it to
  the camera, so it moves with the rider's body.
- preview.png: the bike from the side, from above, and from the cockpit camera.

The numbers are a 56 cm endurance road bike: 73.5 degree seat and 73 degree head angles, 99 cm
of wheelbase, 700x28 tyres, a 42 cm drop bar on a 10 cm stem. Parts are glTF materials, which
import as named slots; import_bike.py paints them.

Run: uv run -q --no-project --with numpy --with matplotlib python -I scripts/make_bike.py
"""

# ruff: noqa: RUF005  (numpy array + [x, y, z] is a vector sum, not a list join)

import math
import os
import sys

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer

OUT = os.environ.get("OC_BIKE_OUT", "/workspace/ue/world_gen/rider")
EYE = 150.0  # cm: RidePawn's EyeHeight
# The cockpit camera, from the eyes, in the bike's frame (cm forward, cm up) and its pitch
# (degrees, down negative): back and down from the eyes, so the bars sit in the frame's foot.
COCKPIT_CAMERA = (-45.0, -38.0, -8.0)
FOV = 61.0  # degrees across: the rider profile's default

# The geometry (cm, in the bike's frame: the origin on the road under the eyes).
TYRE = 33.9  # the tyre's outer radius: a 622 mm bead seat and a 28 mm tyre
RIM = 31.1  # the bead seat's radius
RIM_DEPTH = 4.5
FRONT = np.array([40.0, 0.0, TYRE])
REAR = np.array([-59.0, 0.0, TYRE])
BB = np.array([-18.0, 0.0, TYRE - 7.0])  # the bottom bracket, 7 cm under the axles
SEAT_ANGLE, HEAD_ANGLE = 73.5, 73.0
SEAT_TUBE, SADDLE_HEIGHT = 52.0, 72.0  # from the bottom bracket, along the seat tube
FORK_LENGTH, FORK_OFFSET, HEAD_TUBE = 37.0, 4.5, 15.0
STEM, STEM_ANGLE, SPACERS = 10.0, -6.0, 3.0
BAR_HALF, BAR_REACH, BAR_DROP = 21.0, 8.0, 12.5
CRANK = 17.0


def unit(v):
    v = np.asarray(v, float)
    return v / (np.linalg.norm(v) or 1.0)


class Kit:
    """A mesh and its parts, with the shapes a bike is made of."""

    def __init__(self):
        self.mesh = gltf_writer.Mesh()

    def tube(self, material, path, radius, sides=12, cap=True, up=(0, 0, 1)):
        """A swept tube along `path`. `radius` is a number, one per point, or (across, up) pairs
        for an oval section, across being sideways of the path and up toward `up`."""
        part = self.mesh.part(material)
        path = [np.asarray(p, float) for p in path]
        n = len(path)
        if isinstance(radius, (int, float)):
            radii = [(radius, radius)] * n
        elif isinstance(radius, tuple):  # one oval section for the whole tube
            radii = [radius] * n
        else:  # a list: one radius or oval per point
            radii = [r if isinstance(r, (tuple, list)) else (r, r) for r in radius]
        rings, length = [], 0.0
        up = unit(up)
        prev_u = None
        for i, p in enumerate(path):
            t = unit(path[min(i + 1, n - 1)] - path[max(i - 1, 0)])
            hint = up if abs(t @ up) < 0.95 else np.array([1.0, 0.0, 0.0])
            u = unit(np.cross(t, hint))
            if prev_u is not None and u @ prev_u < 0:
                u = -u
            prev_u = u
            v = np.cross(u, t)
            if i:
                length += np.linalg.norm(p - path[i - 1])
            a, b = radii[i]
            ring = []
            for k in range(sides + 1):
                ang = 2 * math.pi * k / sides
                c, s = math.cos(ang), math.sin(ang)
                pos = p + a * c * u + b * s * v
                nor = unit((c / a) * u + (s / b) * v)
                ring.append(part.vertex(tuple(pos), tuple(nor), (k / sides, length / 10.0)))
            rings.append(ring)
        for i in range(n - 1):
            for k in range(sides):
                part.quad(rings[i][k], rings[i][k + 1], rings[i + 1][k + 1], rings[i + 1][k])
        if cap:
            for i, sign in ((0, -1.0), (n - 1, 1.0)):
                t = unit(path[min(i + 1, n - 1)] - path[max(i - 1, 0)]) * sign
                centre = part.vertex(tuple(path[i]), tuple(t), (0.5, 0.5))
                rim = [
                    part.vertex(part.positions[rings[i][k]], tuple(t), (0.5, 0.5))
                    for k in range(sides + 1)
                ]
                for k in range(sides):
                    part.triangle(centre, rim[k], rim[k + 1])

    def rod(self, material, a, b, radius, sides=8):
        self.tube(material, [a, b], radius, sides)

    def ring(self, material, centre, axis, radius, tube_r, segments=72, sides=10, side=None):
        """A torus round `axis` through `centre` (a tyre, a chainring's edge)."""
        axis = unit(axis)
        side = unit(side if side is not None else np.cross(axis, [0, 0, 1]))
        if np.linalg.norm(side) < 0.5:
            side = unit(np.cross(axis, [1, 0, 0]))
        other = np.cross(axis, side)
        path = [
            np.asarray(centre) + radius * (math.cos(a) * side + math.sin(a) * other)
            for a in np.linspace(0, 2 * math.pi, segments + 1)
        ]
        self.tube(material, path, tube_r, sides, cap=False, up=axis)

    def disc(self, material, centre, axis, r_in, r_out, width, segments=64):
        """A flat annulus with thickness, round `axis` (a rim's bed, a rotor, a sprocket)."""
        part = self.mesh.part(material)
        axis = unit(axis)
        side = unit(np.cross(axis, [0, 0, 1]))
        other = np.cross(axis, side)
        centre = np.asarray(centre, float)
        faces = [
            (r_out, r_out, -width / 2, width / 2, None),  # outer band
            (r_in, r_in, width / 2, -width / 2, None),  # inner band
            (r_in, r_out, width / 2, width / 2, axis),  # the faces
            (r_out, r_in, -width / 2, -width / 2, -axis),
        ]
        for ra, rb, wa, wb, normal in faces:
            rows = []
            for k in range(segments + 1):
                a = 2 * math.pi * k / segments
                radial = math.cos(a) * side + math.sin(a) * other
                nor = (radial if ra == r_out else -radial) if normal is None else normal
                p0 = centre + ra * radial + wa * axis
                p1 = centre + rb * radial + wb * axis
                rows.append(
                    (
                        part.vertex(tuple(p0), tuple(nor), (k / segments, 0)),
                        part.vertex(tuple(p1), tuple(nor), (k / segments, 1)),
                    )
                )
            for k in range(segments):
                part.quad(rows[k][0], rows[k + 1][0], rows[k + 1][1], rows[k][1])

    def block(self, material, centre, size, forward=(1, 0, 0), up=(0, 0, 1)):
        """A box along `forward` and `up`."""
        part = self.mesh.part(material)
        f = unit(forward)
        s = unit(np.cross(up, f))
        u = np.cross(f, s)
        half = np.asarray(size, float) / 2
        centre = np.asarray(centre, float)
        for axis, (a, b) in ((f, (s, u)), (s, (u, f)), (u, (f, s))):
            for sign in (-1, 1):
                k = 0 if axis is f else (1 if axis is s else 2)
                ka = 0 if a is f else (1 if a is s else 2)
                kb = 0 if b is f else (1 if b is s else 2)
                c = centre + sign * axis * half[k]
                corners = [
                    c + i * a * half[ka] + j * b * half[kb]
                    for i, j in ((-1, -1), (1, -1), (1, 1), (-1, 1))
                ]
                if sign < 0:
                    corners = corners[::-1]
                ids = [
                    part.vertex(tuple(p), tuple(sign * axis), (i % 2, i // 2))
                    for i, p in enumerate(corners)
                ]
                part.quad(*ids)


def along(origin, angle_deg, length, back=True):
    """The point `length` cm from `origin` up a tube at `angle_deg` from the ground, leaning
    back (-X) as seat and head tubes do."""
    a = math.radians(angle_deg)
    return np.asarray(origin, float) + length * np.array(
        [-math.cos(a) if back else math.cos(a), 0.0, math.sin(a)]
    )


def geometry():
    """The bike's key points."""
    g = {}
    g["seat_top"] = along(BB, SEAT_ANGLE, SEAT_TUBE)
    g["saddle"] = along(BB, SEAT_ANGLE, SADDLE_HEIGHT)
    # The steering axis passes FORK_OFFSET behind the front axle.
    axis = np.array([-math.cos(math.radians(HEAD_ANGLE)), 0.0, math.sin(math.radians(HEAD_ANGLE))])
    normal = np.array([axis[2], 0.0, -axis[0]])  # forward, square to the axis
    foot = FRONT - FORK_OFFSET * normal
    g["axis"] = axis
    g["crown"] = foot + axis * FORK_LENGTH
    g["head_top"] = foot + axis * (FORK_LENGTH + HEAD_TUBE)
    g["steerer"] = foot + axis * (FORK_LENGTH + HEAD_TUBE + SPACERS + 2.0)
    a = math.radians(STEM_ANGLE)
    g["clamp"] = g["steerer"] + STEM * np.array([math.cos(a), 0.0, math.sin(a)])
    return g


def drop_bar(k, g, sides=12):
    """The bar: its tops across at the clamp, and each side forward, round and down the drop,
    taped from the bend outward; the hoods on the bend's top."""
    c = g["clamp"]
    k.tube("alloy", [c + [0, -BAR_HALF + 3, 0], c + [0, BAR_HALF - 3, 0]], 1.59, sides)
    for y in (-1, 1):
        x0, z0 = c[0], c[2]
        w = BAR_HALF * y
        # The tops bend forward into the ramps, then round the hook and back under.
        hook = [
            (x0, w * 0.86, z0),
            (x0 + 1.5, w * 0.97, z0),
            (x0 + 4.0, w, z0 + 0.2),
            (x0 + BAR_REACH - 1.5, w, z0 - 0.2),
        ]
        for a in np.linspace(math.pi / 2, -math.pi / 2, 11)[1:]:
            r = BAR_DROP / 2
            hook.append((x0 + BAR_REACH - 1.5 + r * 0.8 * math.cos(a), w, z0 - r + r * math.sin(a)))
        hook += [(x0 - 0.5, w, z0 - BAR_DROP), (x0 - 4.5, w, z0 - BAR_DROP + 0.4)]
        k.tube("tape", hook, 1.32, sides)
        # The hood: a rubber body on the bend's top, its lever blade down the front of the hook.
        hood = [
            (x0 + BAR_REACH - 3.0, w, z0 + 1.0),
            (x0 + BAR_REACH + 0.5, w, z0 + 1.6),
            (x0 + BAR_REACH + 3.0, w, z0 + 2.4),
            (x0 + BAR_REACH + 4.3, w, z0 + 3.3),
            (x0 + BAR_REACH + 4.6, w, z0 + 2.3),
        ]
        k.tube("hood", hood, [(1.5, 1.6), (1.6, 2.0), (1.55, 1.9), (1.0, 1.2), (0.6, 0.6)], 12)
        lever = [
            (x0 + BAR_REACH + 3.6, w, z0 + 1.2),
            (x0 + BAR_REACH + 3.4, w, z0 - 4.0),
            (x0 + BAR_REACH + 1.8, w, z0 - 9.0),
            (x0 + BAR_REACH + 0.6, w, z0 - 12.5),
        ]
        k.tube("carbon", lever, [(0.9, 0.5), (0.8, 0.45), (0.6, 0.4), (0.5, 0.35)], 8)


def wheel(k, centre, front):
    """A deep carbon rim, a tyre, 24 bladed spokes and a hub; a rotor on the left."""
    k.ring("tyre", centre, (0, 1, 0), TYRE - 1.4, (1.35, 1.45), 96, 10)
    k.disc("carbon", centre, (0, 1, 0), RIM - RIM_DEPTH, RIM - 0.2, 2.4, 96)
    k.disc("alloy", centre, (0, 1, 0), RIM - 0.4, RIM, 2.2, 96)  # the brake track's lip
    k.tube("alloy", [centre + [0, -5.0, 0], centre + [0, 5.0, 0]], 1.0, 12)
    for y in (-1, 1):
        k.disc("alloy", centre + [0, y * 3.2, 0], (0, 1, 0), 1.0, 2.6, 0.3, 24)
    for i in range(24):
        a = 2 * math.pi * (i + 0.5 * (i % 2)) / 24
        y = -1 if i % 2 else 1
        tangent = 0.35 * (1 if i % 4 < 2 else -1)
        hub = centre + [2.4 * math.cos(a + tangent), y * 3.2, 2.4 * math.sin(a + tangent)]
        rim = centre + [(RIM - RIM_DEPTH) * math.cos(a), 0, (RIM - RIM_DEPTH) * math.sin(a)]
        k.rod("spoke", hub, rim, (0.14, 0.07), 4)
    # A disc rotor and its caliper, on the left.
    k.disc("steel", centre + [0, -5.6, 0], (0, 1, 0), 4.0, 8.0, 0.18, 48)
    caliper = centre + ([-4.0, -6.2, 6.8] if front else [-7.8, -6.2, 0.5])
    k.block("alloy", caliper, (5.0, 1.8, 2.6))


def frame(k, g):
    f, r = FRONT, REAR
    seat_top, head_top, crown = g["seat_top"], g["head_top"], g["crown"]
    # Main triangle: oval-section carbon tubes.
    k.tube("paint", [crown, head_top], 2.1, 16)
    k.tube("paint", [head_top - g["axis"] * 2.5, seat_top + [0, 0, -1.5]], (1.5, 2.1), 16)
    k.tube("paint", [crown + g["axis"] * 2.0, BB + [3.0, 0, 1.5]], (2.0, 2.6), 16)
    k.tube("paint", [BB, seat_top + [0.3, 0, 0.5]], 1.6, 16)
    k.tube("paint", [BB + [0, -3.6, 0], BB + [0, 3.6, 0]], 2.3, 16)
    # The rear triangle, each side.
    for y in (-1, 1):
        drop = r + [0, y * 6.4, 0]
        k.tube("paint", [BB + [-2, y * 3.0, -0.5], drop + [3, 0, 0], drop], (0.9, 1.3), 10)
        k.tube("paint", [seat_top + [0.5, y * 1.4, -4.5], drop + [1.5, 0, 1.5], drop], 0.85, 10)
        k.block("alloy", drop, (3.0, 0.6, 2.0))
    # The fork: a crown and two tapering blades to the dropouts.
    for y in (-1, 1):
        tip = f + [0, y * 5.0, 0]
        mid = (crown + tip) / 2 + [0.8, y * 0.6, 0]
        k.tube(
            "paint", [crown + [0, y * 2.6, 0], mid, tip], [(1.1, 1.5), (0.95, 1.35), (0.7, 0.9)], 10
        )
    k.block(
        "paint", crown + [0, 0, -0.5], (4.5, 8.0, 2.5), forward=-g["axis"][[2, 1, 0]] * [-1, 1, 1]
    )
    # The headset's spacers and top cap, the steerer, the stem and its faceplate.
    k.tube("carbon", [head_top, g["steerer"]], 1.75, 16)
    k.disc("alloy", g["steerer"] + g["axis"] * 0.3, g["axis"], 0.0, 1.6, 0.5, 24)
    clamp = g["clamp"]
    k.tube("alloy", [g["steerer"] - g["axis"] * 2.0, clamp - [1.5, 0, -0.15]], (1.5, 1.7), 12)
    k.tube("alloy", [g["steerer"] - g["axis"] * 3.6, g["steerer"] + g["axis"] * 0.2], 2.0, 16)
    k.block("alloy", clamp + [0.6, 0, 0], (2.2, 4.4, 3.6))


def seat(k, g):
    saddle = g["saddle"]
    k.tube("carbon", [g["seat_top"] + [0.2, 0, -3], saddle + [0, 0, -2.5]], 1.36, 16)
    k.block("alloy", saddle + [0, 0, -2.0], (3.0, 2.0, 1.6))
    # The saddle: oval sections from the nose to the wide back, a shallow crown on top.
    rows = [
        (-13, 6.0, 1.6),
        (-10, 7.0, 2.2),
        (-5, 5.5, 2.5),
        (0, 3.2, 2.3),
        (6, 2.0, 2.0),
        (13, 1.1, 1.5),
    ]
    path = [saddle + [x + 2.0, 0, 1.6 + (0.3 if x < 0 else -0.15 * x / 13)] for x, _, _ in rows]
    k.tube("saddle", path, [(w, h) for _, w, h in rows], 18, cap=True, up=(0, 0, 1))
    for y in (-1, 1):
        k.tube(
            "steel",
            [
                saddle + [-9, y * 3.0, 0.2],
                saddle + [-4, y * 2.6, -1.8],
                saddle + [6, y * 1.6, -1.8],
                saddle + [11, y * 0.8, 0.4],
            ],
            0.35,
            6,
        )


def drivetrain(k, g):
    """Two chainrings, the cranks (the right forward, the left back) and pedals, the cassette,
    the rear derailleur and the chain's two runs."""
    right = BB + [0, 4.6, 0]
    for teeth, y in ((50, 0.0), (34, -0.5)):
        r = teeth * 1.27 / (2 * math.pi)
        k.disc("chainring", right + [0, y, 0], (0, 1, 0), r - 1.6, r, 0.3, 96)
    k.disc("carbon", right + [0, 0.4, 0], (0, 1, 0), 0.0, 5.5, 0.6, 32)  # the spider
    for y, a in ((6.5, math.radians(-15)), (-6.5, math.radians(165))):
        tip = BB + [CRANK * math.cos(a), y, CRANK * math.sin(a)]
        k.tube("carbon", [BB + [0, y, 0], tip], (1.0, 1.6), 10)
        k.block("pedal", tip + [0, y * 0.6 + (1 if y > 0 else -1) * 4.0, 0], (8.0, 7.5, 1.4))
    cassette = REAR + [0, 3.0, 0]
    for i, teeth in enumerate((11, 12, 13, 14, 15, 17, 19, 21, 24, 27, 30)):
        r = teeth * 1.27 / (2 * math.pi)
        k.disc("steel", cassette + [0, -i * 0.38, 0], (0, 1, 0), 1.6, r, 0.18, 48)
    # The chain on the big ring and a middle sprocket: the top run, and the bottom run by way of
    # the derailleur's two pulleys.
    big, cog = 50 * 1.27 / (2 * math.pi), 19 * 1.27 / (2 * math.pi)
    y = 4.6
    top = [right + [0, 0, big], cassette + [0, y - 3.0 - 6 * 0.38 - 3.0 + 1.6, cog]]
    top = [p * [1, 0, 1] + [0, y - 0.6, 0] for p in top]
    k.tube("chain", top, (0.3, 0.4), 6)
    upper = REAR + [1.0, y, -4.5]
    lower = REAR + [3.5, y, -11.0]
    k.tube(
        "chain",
        [p * [1, 0, 1] + [0, y - 0.6, 0] for p in (right + [0, 0, -big], lower)],
        (0.3, 0.4),
        6,
    )
    k.tube(
        "chain",
        [p * [1, 0, 1] + [0, y - 0.6, 0] for p in (REAR + [0, 0, -cog], upper)],
        (0.3, 0.4),
        6,
    )
    for c in (upper, lower):
        k.disc("carbon", c + [0, 0.4, 0], (0, 1, 0), 0.3, 1.3, 0.5, 24)
    k.block("alloy", (upper + lower) / 2 + [0, 1.0, 0], (2.0, 0.5, 8.0), forward=(0.3, 0, 1))
    k.block("alloy", REAR + [-1.5, y + 1.2, -2.0], (3.0, 1.6, 3.0))


def computer(k, g):
    """A bike computer on an out-front mount, its screen tipped back toward the rider."""
    c = g["clamp"]
    k.tube("carbon", [c + [1.6, 0, -0.6], c + [8.0, 0, -0.6]], (0.9, 0.7), 10)
    face = unit([math.sin(math.radians(20)), 0, math.cos(math.radians(20))])
    centre = c + [9.5, 0, 0.6]
    k.block("carbon", centre, (8.5, 5.4, 1.8), forward=(face[2], 0, -face[0]), up=face)
    k.block(
        "screen", centre + face * 0.92, (7.2, 4.4, 0.08), forward=(face[2], 0, -face[0]), up=face
    )


def bottle(k, g):
    """A bottle and cage on the down tube."""
    d = unit(g["crown"] - BB)
    up = np.cross([0, 1, 0], d)  # off the tube's top
    base = BB + d * 14 + up * 4.0
    k.tube(
        "bottle",
        [base, base + d * 6, base + d * 19, base + d * 21.5, base + d * 23],
        [3.3, 3.5, 3.5, 2.2, 1.4],
        16,
        cap=True,
    )
    k.tube("cage", [base + d * 2 - up * 0.5, base + d * 16 + up * 3.4], 0.25, 6)


def build(parts):
    k = Kit()
    g = geometry()
    for name in parts:
        if name == "rear_wheel":
            wheel(k, REAR, False)
        elif name == "front_wheel":
            wheel(k, FRONT, True)
        else:
            globals()[name](k, g)
    return k.mesh


BIKE = (
    "frame",
    "drop_bar",
    "computer",
    "seat",
    "drivetrain",
    "bottle",
    "front_wheel",
    "rear_wheel",
)
COCKPIT = ("frame", "drop_bar", "computer", "front_wheel")


def shifted(mesh, offset):
    """The mesh moved by `offset` cm."""
    for part in mesh.parts.values():
        part.positions = [tuple(np.subtract(p, offset)) for p in part.positions]
    return mesh


def preview(bike, path):
    """The bike from the side, from above, and as the cockpit camera sees its front end."""
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    from matplotlib.collections import PolyCollection

    colours = {
        "paint": "#1d4a3f",
        "carbon": "#1a1a1c",
        "tyre": "#111",
        "alloy": "#9a9da3",
        "steel": "#b8bcc2",
        "tape": "#141414",
        "hood": "#222",
        "saddle": "#191919",
        "spoke": "#333",
        "chainring": "#555",
        "chain": "#666",
        "pedal": "#2a2a2a",
        "screen": "#0b2a2a",
        "bottle": "#d8d2c4",
        "cage": "#222",
    }
    fig, axes = plt.subplots(1, 3, figsize=(21, 7), facecolor="#f3eee4")
    cx, cz, tilt = COCKPIT_CAMERA
    cam = np.array([cx, 0.0, EYE + cz])
    pitch = math.radians(tilt)
    fwd = np.array([math.cos(pitch), 0, math.sin(pitch)])
    upv = np.array([-math.sin(pitch), 0, math.cos(pitch)])
    views = [
        ("side", lambda p: (p[:, 0], p[:, 2]), lambda p: -p[:, 1]),
        ("above", lambda p: (p[:, 0], -p[:, 1]), lambda p: p[:, 2]),
        ("cockpit", None, None),
    ]
    for ax, (name, proj, depth) in zip(axes, views, strict=True):
        polys, cols, keys = [], [], []
        for mat, part in bike.parts.items():
            pos = np.array(part.positions)
            idx = np.array(part.indices).reshape(-1, 3)
            tri = pos[idx]
            if name == "cockpit":
                rel = tri - cam
                d = rel @ fwd
                keep = (d > 5).all(axis=1)
                rel, d = rel[keep], d[keep]
                x = 120 * (rel[..., 1]) / d
                y = 120 * (rel @ upv) / d
                pts = np.stack([x, y], -1)
                key = -d.mean(axis=1)
            else:
                flat = tri.reshape(-1, 3)
                a, b = proj(flat)
                pts = np.stack([a, b], -1).reshape(-1, 3, 2)
                key = depth(flat).reshape(-1, 3).mean(axis=1)
            polys += list(pts)
            cols += [colours.get(mat, "#888")] * len(pts)
            keys += list(key)
        order = np.argsort(keys)
        ax.add_collection(
            PolyCollection(
                [polys[i] for i in order], facecolors=[cols[i] for i in order], edgecolors="none"
            )
        )
        ax.set_aspect("equal")
        ax.set_title(name)
        if name == "cockpit":
            # The frame at the game's default 61 degree field of view and 16:9.
            half = 120.0 * math.tan(math.radians(FOV / 2))
            ax.set_xlim(-half, half)
            ax.set_ylim(-half * 9 / 16, half * 9 / 16)
        else:
            ax.autoscale()
        ax.set_facecolor("#e8e2d6" if name != "cockpit" else "#9fb7c8")
    fig.savefig(path, dpi=80, bbox_inches="tight")


def main():
    os.makedirs(OUT, exist_ok=True)
    bike = build(BIKE)
    bike.write(f"{OUT}/bike.gltf")
    shifted(build(COCKPIT), (0.0, 0.0, EYE)).write(f"{OUT}/cockpit.gltf")
    tris = sum(len(p.indices) // 3 for p in bike.parts.values())
    print(f"bike: {tris} triangles, {len(bike.parts)} parts: {', '.join(bike.parts)}")
    g = geometry()
    print(
        "clamp {:.1f} ahead, {:.1f} up; saddle {:.1f}, {:.1f}".format(
            g["clamp"][0], g["clamp"][2], g["saddle"][0], g["saddle"][2]
        )
    )
    if "--no-preview" not in sys.argv:
        preview(bike, f"{OUT}/preview.png")
    print(f"{OUT}: done")


if __name__ == "__main__":
    main()
