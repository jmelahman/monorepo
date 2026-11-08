"""Writes the coast ride's buildings and street furniture as glTF into assets_gen/kit.

Houses (H_00..H_n) are seeded variants of the rural Japanese house: a concrete footing, plaster
or board walls, aluminium-framed windows, a tiled skirt roof (hisashi) between floors, and a
gable or hipped kawara roof. Their front faces local +Y, the origin is the footprint's centre at
ground level, and vertex colours tint walls and roofs (the materials multiply by them).

Kit pieces (K_*) are the furniture: block wall, vending machines, curve mirror, torii, stone
lantern, kura storehouse, bus shelter, a little shrine.

Slots: Plaster, Boards, RoofTile, Base, Glass, Frame, Shutter, Door, Fascia, AC, Grate, Block,
Metal, Red, Stone, Wood, Sign, Mirror.

Run with system Python: python3 -I scripts/make_houses.py
"""

import math
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer

OUT = "/workspace/ue/assets_gen/kit"
WHITE = (1, 1, 1, 1)


def sub(a, b):
    return (a[0] - b[0], a[1] - b[1], a[2] - b[2])


def cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def norm(v):
    length = math.sqrt(v[0] ** 2 + v[1] ** 2 + v[2] ** 2) or 1.0
    return (v[0] / length, v[1] / length, v[2] / length)


def face(mesh, material, points, outward, color=WHITE, uv_scale=100.0):
    """A flat polygon (3 or 4 corners) facing `outward`, with planar UVs on its own plane."""
    n = norm(outward)
    c = cross(sub(points[1], points[0]), sub(points[2], points[0]))
    if c[0] * n[0] + c[1] * n[1] + c[2] * n[2] < 0:
        points = list(reversed(points))
    # UV axes: across the face horizontally, then up it.
    u_axis = norm(cross((0, 0, 1), n)) if abs(n[2]) < 0.95 else (1, 0, 0)
    v_axis = cross(n, u_axis)
    part = mesh.part(material)
    ids = [
        part.vertex(
            p,
            n,
            (
                (p[0] * u_axis[0] + p[1] * u_axis[1] + p[2] * u_axis[2]) / uv_scale,
                -(p[0] * v_axis[0] + p[1] * v_axis[1] + p[2] * v_axis[2]) / uv_scale,
            ),
            color,
        )
        for p in points
    ]
    if len(ids) == 3:
        part.triangle(*ids)
    else:
        part.quad(*ids)


def box(mesh, material, x0, y0, z0, x1, y1, z1, color=WHITE, uv_scale=100.0):
    mesh.box(
        material,
        ((x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2),
        (x1 - x0, y1 - y0, z1 - z0),
        0.0,
        uv_scale,
        color,
    )


# --- houses -------------------------------------------------------------------------------

# Ceramic siding and plaster: creams, beiges and tans, the odd pale blue-grey. Grey reads as
# concrete.
WALLS = [
    (0.97, 0.93, 0.84, 1),
    (0.93, 0.85, 0.70, 1),
    (0.86, 0.76, 0.60, 1),
    (0.99, 0.97, 0.93, 1),
    (0.80, 0.69, 0.55, 1),
    (0.80, 0.84, 0.86, 1),
    (0.90, 0.88, 0.80, 1),
    # Painted siding and coloured render, as common out of town as plain cream.
    (0.45, 0.62, 0.80, 1),
    (0.58, 0.72, 0.46, 1),
    (0.98, 0.70, 0.36, 1),
    (0.84, 0.54, 0.46, 1),
    (0.52, 0.76, 0.68, 1),
]
ROOFS = [
    (0.42, 0.44, 0.48, 1),
    (0.30, 0.32, 0.36, 1),
    (0.55, 0.30, 0.20, 1),
    (0.22, 0.26, 0.34, 1),
    (0.50, 0.50, 0.50, 1),
]
BOARDS = [(0.45, 0.36, 0.28, 1), (0.30, 0.24, 0.20, 1), (0.22, 0.20, 0.19, 1)]


def window(mesh, side, centre_u, z0, width, height, w, d, shutter=False):
    """A framed window on one wall. `side` is 'front', 'back', 'left' or 'right'."""
    out = 4.0
    if side in ("front", "back"):
        y = d / 2 if side == "front" else -d / 2
        sign = 1 if side == "front" else -1
        box(
            mesh,
            "Frame",
            centre_u - width / 2 - 5,
            min(y, y + sign * out),
            z0 - 5,
            centre_u + width / 2 + 5,
            max(y, y + sign * out),
            z0 + height + 5,
        )
        box(
            mesh,
            "Glass",
            centre_u - width / 2,
            min(y + sign * out, y + sign * (out + 1)),
            z0,
            centre_u + width / 2,
            max(y + sign * out, y + sign * (out + 1)),
            z0 + height,
        )
        curtain(
            mesh,
            centre_u,
            z0,
            width,
            height,
            min(y + sign * (out + 1), y + sign * (out + 1.4)),
            max(y + sign * (out + 1), y + sign * (out + 1.4)),
            True,
        )
        # A mullion: two sliding panes.
        box(
            mesh,
            "Frame",
            centre_u - 2.5,
            min(y + sign * out, y + sign * (out + 2)),
            z0,
            centre_u + 2.5,
            max(y + sign * out, y + sign * (out + 2)),
            z0 + height,
        )
        if shutter:
            box(
                mesh,
                "Shutter",
                centre_u + width / 2 + 6,
                min(y, y + sign * 12),
                z0 - 10,
                centre_u + width / 2 + 6 + width / 2,
                max(y, y + sign * 12),
                z0 + height + 10,
            )
    else:
        x = w / 2 if side == "right" else -w / 2
        sign = 1 if side == "right" else -1
        box(
            mesh,
            "Frame",
            min(x, x + sign * out),
            centre_u - width / 2 - 5,
            z0 - 5,
            max(x, x + sign * out),
            centre_u + width / 2 + 5,
            z0 + height + 5,
        )
        box(
            mesh,
            "Glass",
            min(x + sign * out, x + sign * (out + 1)),
            centre_u - width / 2,
            z0,
            max(x + sign * out, x + sign * (out + 1)),
            centre_u + width / 2,
            z0 + height,
        )
        curtain(
            mesh,
            centre_u,
            z0,
            width,
            height,
            min(x + sign * (out + 1), x + sign * (out + 1.4)),
            max(x + sign * (out + 1), x + sign * (out + 1.4)),
            False,
        )


CURTAINS = [(0.92, 0.9, 0.82, 1), (0.85, 0.88, 0.9, 1), (0.9, 0.82, 0.7, 1), (0.75, 0.8, 0.72, 1)]


def curtain(mesh, centre_u, z0, width, height, d0, d1, along_x):
    """Most windows have a curtain or a paper blind drawn across one of the two panes. The panel
    sits a hair proud of the glass so the dark pane reads as a room behind it."""
    pick = random.Random(int(centre_u * 7 + z0 * 13 + d0 * 3))
    if pick.random() < 0.25:
        return
    colour = pick.choice(CURTAINS)
    half = width / 2 - 3
    lo = centre_u - width / 2 if pick.random() < 0.5 else centre_u + 3
    top = z0 + height - (0 if pick.random() < 0.6 else pick.uniform(0.2, 0.6) * height)
    if along_x:
        box(mesh, "Curtain", lo, d0, max(z0, top - height), lo + half, d1, top, colour)
    else:
        box(mesh, "Curtain", d0, lo, max(z0, top - height), d1, lo + half, top, colour)


def storey(mesh, rng, w, d, z0, height, wall, boards, ground_floor, full=False):
    """A storey's walls with windows on every side; `full` boards it to the eaves."""
    if boards and full:
        box(mesh, "Boards", -w / 2, -d / 2, z0, w / 2, d / 2, z0 + height, boards)
    elif boards:
        # Dark boards below, plaster above: the commonest rural finish.
        box(mesh, "Boards", -w / 2, -d / 2, z0, w / 2, d / 2, z0 + height * 0.55, boards)
        box(
            mesh,
            "Plaster",
            -w / 2 + 0.5,
            -d / 2 + 0.5,
            z0 + height * 0.55,
            w / 2 - 0.5,
            d / 2 - 0.5,
            z0 + height,
            wall,
        )
    else:
        box(mesh, "Plaster", -w / 2, -d / 2, z0, w / 2, d / 2, z0 + height, wall)
    sill = 90 if not ground_floor else 80
    count = max(1, int(w // 300))
    for k in range(count):
        u = -w / 2 + w * (k + 0.5) / count
        if ground_floor and k == count // 2:
            # The genkan: a sliding front door under a little canopy.
            box(mesh, "Door", u - 85, d / 2, z0, u + 85, d / 2 + 4, z0 + 205)
            box(mesh, "Frame", u - 90, d / 2, z0 + 205, u + 90, d / 2 + 5, z0 + 212)
            box(mesh, "Fascia", u - 120, d / 2, z0 + 225, u + 120, d / 2 + 90, z0 + 233)
            continue
        if rng.random() < 0.85:
            window(
                mesh,
                "front",
                u,
                z0 + sill,
                rng.choice([120, 160, 170]),
                rng.choice([110, 120, 130]),
                w,
                d,
                rng.random() < 0.4,
            )
        if rng.random() < 0.6:
            window(mesh, "back", u, z0 + sill + 20, rng.choice([80, 120]), 90, w, d)
    for side in ("left", "right"):
        if rng.random() < 0.7:
            window(
                mesh,
                side,
                rng.uniform(-d / 4, d / 4),
                z0 + sill + 10,
                rng.choice([60, 80, 120]),
                90,
                w,
                d,
            )


def skirt(mesh, w, d, z, reach, colour):
    """The tiled skirt roof (hisashi) round a storey, sloping out from the wall."""
    drop = reach * 0.45
    corners = [(-w / 2, -d / 2), (w / 2, -d / 2), (w / 2, d / 2), (-w / 2, d / 2)]
    outer = [
        (-w / 2 - reach, -d / 2 - reach),
        (w / 2 + reach, -d / 2 - reach),
        (w / 2 + reach, d / 2 + reach),
        (-w / 2 - reach, d / 2 + reach),
    ]
    for k in range(4):
        a, b = corners[k], corners[(k + 1) % 4]
        oa, ob = outer[k], outer[(k + 1) % 4]
        ox, oy = ((0, -1), (1, 0), (0, 1), (-1, 0))[k]
        up = norm((ox * drop, oy * drop, reach))
        pts = [(a[0], a[1], z), (b[0], b[1], z), (ob[0], ob[1], z - drop), (oa[0], oa[1], z - drop)]
        face(mesh, "RoofTile", pts, up, colour)
        under = [(p[0], p[1], p[2] - 3) for p in pts]
        face(mesh, "Fascia", under, (-up[0], -up[1], -up[2]))
        # Eave board.
        face(
            mesh,
            "Fascia",
            [
                (oa[0], oa[1], z - drop),
                (ob[0], ob[1], z - drop),
                (ob[0], ob[1], z - drop - 12),
                (oa[0], oa[1], z - drop - 12),
            ],
            (up[0], up[1], 0),
        )


def gable_roof(mesh, w, d, z, pitch, over, colour, wall):
    """Ridge along X. Slopes face front and back; plaster gable triangles on the ends."""
    rise = (d / 2) * math.tan(math.radians(pitch))
    x0, x1 = -w / 2 - over, w / 2 + over
    yo = d / 2 + over
    zo = z - over * math.tan(math.radians(pitch))
    ridge = z + rise
    for sign in (1, -1):
        up = norm((0, sign * math.sin(math.radians(pitch)), math.cos(math.radians(pitch))))
        pts = [(x0, 0, ridge), (x1, 0, ridge), (x1, sign * yo, zo), (x0, sign * yo, zo)]
        face(mesh, "RoofTile", pts, up, colour)
        face(mesh, "Fascia", [(p[0], p[1], p[2] - 6) for p in pts], (-up[0], -up[1], -up[2]))
        face(
            mesh,
            "Fascia",
            [
                (x0, sign * yo, zo),
                (x1, sign * yo, zo),
                (x1, sign * yo, zo - 15),
                (x0, sign * yo, zo - 15),
            ],
            (0, sign, 0),
        )
    for sx in (-1, 1):
        face(
            mesh,
            "Plaster",
            [(sx * w / 2, -d / 2, z), (sx * w / 2, d / 2, z), (sx * w / 2, 0, ridge)],
            (sx, 0, 0),
            wall,
        )
        # Barge boards up the gable.
        for sign in (1, -1):
            face(
                mesh,
                "Fascia",
                [
                    (sx * (w / 2 + over), 0, ridge + 4),
                    (sx * (w / 2 + over), sign * yo, zo + 4),
                    (sx * (w / 2 + over), sign * yo, zo - 14),
                    (sx * (w / 2 + over), 0, ridge - 14),
                ],
                (sx, 0, 0),
            )
    darker = (*tuple(c * 0.75 for c in colour[:3]), 1)
    box(mesh, "RoofTile", x0, -14, ridge - 6, x1, 14, ridge + 18, darker)


def hip_roof(mesh, w, d, z, pitch, over, colour):
    rise = (d / 2) * math.tan(math.radians(pitch))
    x0, x1 = -w / 2 - over, w / 2 + over
    y0, y1 = -d / 2 - over, d / 2 + over
    zo = z - over * math.tan(math.radians(pitch))
    ridge = z + rise
    inset = d / 2 + over
    rx0, rx1 = min(0, x0 + inset), max(0, x1 - inset)
    t = math.tan(math.radians(pitch))
    slopes = [
        ([(x0, y1, zo), (x1, y1, zo), (rx1, 0, ridge), (rx0, 0, ridge)], (0, t, 1)),
        ([(x1, y0, zo), (x0, y0, zo), (rx0, 0, ridge), (rx1, 0, ridge)], (0, -t, 1)),
        ([(x1, y1, zo), (x1, y0, zo), (rx1, 0, ridge)], (t, 0, 1)),
        ([(x0, y0, zo), (x0, y1, zo), (rx0, 0, ridge)], (-t, 0, 1)),
    ]
    for pts, up in slopes:
        if len(pts) == 4 and abs(pts[2][0] - pts[3][0]) < 1:
            pts = pts[:3]
        face(mesh, "RoofTile", pts, up, colour)
        face(mesh, "Fascia", [(p[0], p[1], p[2] - 6) for p in pts], (-up[0], -up[1], -up[2]))
    for (ax, ay), (bx, by), n in (
        ((x0, y1), (x1, y1), (0, 1, 0)),
        ((x1, y0), (x0, y0), (0, -1, 0)),
        ((x1, y1), (x1, y0), (1, 0, 0)),
        ((x0, y0), (x0, y1), (-1, 0, 0)),
    ):
        face(mesh, "Fascia", [(ax, ay, zo), (bx, by, zo), (bx, by, zo - 15), (ax, ay, zo - 15)], n)
    darker = (*tuple(c * 0.75 for c in colour[:3]), 1)
    if rx1 > rx0:
        box(mesh, "RoofTile", rx0, -14, ridge - 6, rx1, 14, ridge + 18, darker)


def house(seed):
    rng = random.Random(seed * 7919 + 13)
    mesh = gltf_writer.Mesh()
    w = rng.choice([760, 850, 910, 1000, 1090])
    d = rng.choice([640, 730, 820])
    two = rng.random() < 0.65
    wall = rng.choice(WALLS)
    roof = rng.choice(ROOFS)
    boards = rng.choice(BOARDS) if rng.random() < 0.55 else None
    # About one house in five is dark stained boards all over, the old farmhouse way.
    full = boards is not None and rng.random() < 0.4
    base_top = 45
    box(mesh, "Base", -w / 2 - 6, -d / 2 - 6, -60, w / 2 + 6, d / 2 + 6, base_top)
    h1, h2 = 285, 265
    storey(mesh, rng, w, d, base_top, h1, wall, boards, True, full)
    top = base_top + h1
    pitch = rng.choice([22, 25, 28])
    hip = rng.random() < 0.45
    if two:
        w2 = w if rng.random() < 0.5 else w - rng.choice([180, 270])
        d2 = d if rng.random() < 0.6 else d - 90
        skirt(mesh, w, d, top + 10, 70, roof)
        mesh_shift = (w - w2) / 2 * rng.choice([-1, 1])
        upper = gltf_writer.Mesh()
        storey(upper, rng, w2, d2, top, h2, wall, boards if full else None, False, full)
        if rng.random() < 0.5:
            # A balcony across the front, with a laundry pole.
            box(upper, "Base", -w2 / 2 + 40, d2 / 2, top - 12, w2 / 2 - 40, d2 / 2 + 110, top + 6)
            box(
                upper,
                "Frame",
                -w2 / 2 + 40,
                d2 / 2 + 104,
                top + 6,
                w2 / 2 - 40,
                d2 / 2 + 110,
                top + 105,
            )
            box(
                upper,
                "Metal",
                -w2 / 2 + 50,
                d2 / 2 + 70,
                top + 170,
                w2 / 2 - 50,
                d2 / 2 + 74,
                top + 174,
            )
        if hip:
            hip_roof(upper, w2, d2, top + h2, pitch, 60, roof)
        else:
            gable_roof(upper, w2, d2, top + h2, pitch, 55, roof, wall)
        merge(mesh, upper, (mesh_shift, -(d - d2) / 2, 0))
    else:
        if hip:
            hip_roof(mesh, w, d, top, pitch, 70, roof)
        else:
            gable_roof(mesh, w, d, top, pitch, 65, roof, wall)
    # Outside: an air conditioner and its pipe, a downpipe, a gas meter.
    side = rng.choice([-1, 1])
    ax = side * (w / 2 + 35)
    ay = rng.uniform(-d / 4, d / 4)
    box(mesh, "AC", ax - 15, ay - 40, base_top - 40, ax + 15, ay + 40, base_top + 20)
    box(
        mesh,
        "Grate",
        ax + side * 15 - 1,
        ay - 25,
        base_top - 30,
        ax + side * 15 + 1,
        ay + 25,
        base_top + 10,
    )
    box(
        mesh,
        "AC",
        side * (w / 2) - 3,
        ay - 3,
        base_top + 20,
        side * (w / 2) + 6,
        ay + 3,
        base_top + 230,
    )
    box(mesh, "Metal", w / 2 - 15, d / 2, base_top, w / 2 - 7, d / 2 + 8, top + (h2 if two else 0))
    return mesh


def merge(into, other, offset):
    for material, part in other.parts.items():
        target = into.part(material)
        base = len(target.positions)
        for p, n, uv, c in zip(part.positions, part.normals, part.uvs, part.colors, strict=False):
            target.vertex((p[0] + offset[0], p[1] + offset[1], p[2] + offset[2]), n, uv, c)
        for i in range(0, len(part.indices), 3):
            target.triangle(*(base + j for j in part.indices[i : i + 3]))


# --- the kit --------------------------------------------------------------------------------


def block_wall():
    """2 m of concrete block wall, 1.2 m high, with a cap; runs along X, its face toward +Y."""
    mesh = gltf_writer.Mesh()
    box(mesh, "Base", -100, -12, -40, 100, 12, 5)
    box(mesh, "Block", -100, -8, 5, 100, 8, 125, uv_scale=200.0)
    box(mesh, "Base", -102, -10, 125, 102, 10, 131)
    return mesh


def vending():
    """Two vending machines side by side, lit fronts toward +Y: a display of drinks behind glass,
    a row of buttons under each shelf, a lit header, the coin and bill slots and the take-out
    flap."""
    mesh = gltf_writer.Mesh()
    rng = random.Random(11)
    drinks = [
        (0.9, 0.85, 0.2, 1),
        (0.85, 0.15, 0.1, 1),
        (0.15, 0.35, 0.8, 1),
        (0.95, 0.95, 0.9, 1),
        (0.2, 0.6, 0.25, 1),
        (0.95, 0.55, 0.1, 1),
        (0.35, 0.2, 0.1, 1),
        (0.1, 0.1, 0.1, 1),
    ]
    for k, colour in enumerate([(0.85, 0.12, 0.10, 1), (0.92, 0.93, 0.95, 1)]):
        x = -55 + k * 110
        box(mesh, "Metal", x - 52, -40, 0, x + 52, 36, 183, colour)
        # The lit display: a pale back panel, three shelves of drinks, a glass front.
        box(mesh, "Sign", x - 44, 34, 92, x + 44, 36, 172)
        for _row, z in enumerate((97, 122, 147)):
            box(mesh, "Metal", x - 44, 36, z - 2, x + 44, 41, z, (0.7, 0.7, 0.7, 1))
            for c in range(8):
                cx = x - 39 + c * 11.1
                h = rng.choice((13, 16, 19))
                gltf_writer_cyl(mesh, "Cans", cx, 38.5, z, h, 3.6, 8, rng.choice(drinks))
            # The buttons under each shelf.
            for c in range(8):
                cx = x - 39 + c * 11.1
                box(mesh, "Cans", cx - 3, 41, z - 7, cx + 3, 42.5, z - 3, (0.2, 0.9, 0.3, 1))
        box(mesh, "Glass", x - 45, 41.5, 90, x + 45, 42.0, 174, (1, 1, 1, 1))
        # The frame round the display, the header and the panel below.
        box(mesh, "Metal", x - 50, 36, 172, x + 50, 43, 182, colour)
        box(mesh, "Sign", x - 45, 43, 174, x + 45, 44, 180)
        box(mesh, "Metal", x - 50, 36, 0, x + 50, 43, 90, colour)
        box(mesh, "Frame", x + 22, 43, 58, x + 40, 46, 84)  # coin and bill slots
        box(mesh, "Grate", x + 26, 46, 72, x + 36, 46.5, 80)
        box(mesh, "Frame", x - 38, 43, 12, x + 38, 46, 34)  # the take-out flap
        box(mesh, "Grate", x - 34, 46, 15, x + 34, 46.5, 31)
        box(mesh, "Metal", x - 50, -40, -5, x + 50, 43, 2, (0.3, 0.3, 0.3, 1))
    box(mesh, "Metal", -110, -40, 183, 110, 50, 190, (0.6, 0.6, 0.6, 1))
    # Recycling bins for cans and bottles beside them.
    for k, colour in enumerate([(0.1, 0.3, 0.6, 1), (0.1, 0.45, 0.2, 1)]):
        x = 125 + k * 34
        box(mesh, "Metal", x - 15, 10, 0, x + 15, 40, 80, colour)
        box(mesh, "Grate", x - 6, 40, 62, x + 6, 41, 70)
    return mesh


def curve_mirror():
    mesh = gltf_writer.Mesh()
    gltf_writer_cyl(mesh, "Red", 0, 0, 0, 330, 4.5)
    box(mesh, "Red", -40, -6, 300, 40, 2, 380, (1, 0.45, 0.1, 1))
    box(mesh, "Mirror", -34, 2, 306, 34, 4, 374)
    return mesh


def gltf_writer_cyl(mesh, material, x, y, z, height, r, sides=10, color=WHITE):
    part = mesh.part(material)
    ring = []
    for k in range(sides + 1):
        t = 2 * math.pi * k / sides
        c, s = math.cos(t), math.sin(t)
        ring.append(
            (
                part.vertex((x + c * r, y + s * r, z), (c, s, 0), (k / sides, 0), color),
                part.vertex(
                    (x + c * r, y + s * r, z + height), (c, s, 0), (k / sides, height / 100), color
                ),
            )
        )
    for k in range(sides):
        part.quad(ring[k][0], ring[k + 1][0], ring[k + 1][1], ring[k][1])


def torii():
    """A vermilion torii, 4 m wide, its opening along Y (the path runs through it along Y)."""
    mesh = gltf_writer.Mesh()
    red = (1, 1, 1, 1)
    for sx in (-1, 1):
        gltf_writer_cyl(mesh, "Red", sx * 200, 0, -30, 450, 16, 14, red)
        box(mesh, "Wood", sx * 200 - 22, -22, -30, sx * 200 + 22, 22, 40, (0.1, 0.1, 0.1, 1))
    box(mesh, "Red", -250, -14, 340, 250, 14, 368, red)  # nuki
    box(mesh, "Red", -290, -22, 410, 290, 22, 440, red)  # kasagi
    box(mesh, "Wood", -305, -26, 440, 305, 26, 462, (0.08, 0.08, 0.08, 1))  # black top beam
    box(mesh, "Red", -14, -10, 368, 14, 10, 410, red)  # gakuzuka
    return mesh


def lantern():
    mesh = gltf_writer.Mesh()
    box(mesh, "Stone", -35, -35, 0, 35, 35, 20)
    box(mesh, "Stone", -12, -12, 20, 12, 12, 110)
    box(mesh, "Stone", -28, -28, 110, 28, 28, 125)
    box(mesh, "Stone", -22, -22, 125, 22, 22, 165)
    box(mesh, "Glass", -14, -23, 135, 14, 23, 158, (1, 0.8, 0.5, 1))
    face(mesh, "Stone", [(-40, -40, 165), (40, -40, 165), (0, 0, 205)], (0, -1, 1))
    face(mesh, "Stone", [(40, -40, 165), (40, 40, 165), (0, 0, 205)], (1, 0, 1))
    face(mesh, "Stone", [(40, 40, 165), (-40, 40, 165), (0, 0, 205)], (0, 1, 1))
    face(mesh, "Stone", [(-40, 40, 165), (-40, -40, 165), (0, 0, 205)], (-1, 0, 1))
    box(mesh, "Stone", -6, -6, 200, 6, 6, 218)
    return mesh


def kura():
    """A white storehouse: black tiled base, thick plaster walls, heavy gable roof."""
    mesh = gltf_writer.Mesh()
    w, d = 540, 450
    box(mesh, "Base", -w / 2 - 8, -d / 2 - 8, -50, w / 2 + 8, d / 2 + 8, 40)
    box(
        mesh, "Boards", -w / 2 - 2, -d / 2 - 2, 40, w / 2 + 2, d / 2 + 2, 160, (0.12, 0.12, 0.13, 1)
    )
    box(mesh, "Plaster", -w / 2, -d / 2, 160, w / 2, d / 2, 560, (0.97, 0.96, 0.93, 1))
    box(mesh, "Door", -70, d / 2, 40, 70, d / 2 + 10, 270, (0.35, 0.33, 0.3, 1))
    box(mesh, "Shutter", -40, d / 2, 400, 40, d / 2 + 14, 470, (0.9, 0.9, 0.88, 1))
    gable_roof(mesh, w, d, 560, 27, 60, (0.25, 0.26, 0.28, 1), (0.97, 0.96, 0.93, 1))
    return mesh


def shelter():
    """A bus stop shelter: corrugated roof on steel posts, a bench, a sign pole."""
    mesh = gltf_writer.Mesh()
    for x in (-180, 180):
        for y in (-60, 60):
            box(mesh, "Metal", x - 5, y - 5, 0, x + 5, y + 5, 240, (0.7, 0.72, 0.7, 1))
    face(
        mesh,
        "Corrugated",
        [(-210, -90, 250), (210, -90, 250), (210, 90, 236), (-210, 90, 236)],
        (0, 0.08, 1),
    )
    face(
        mesh,
        "Corrugated",
        [(-210, -90, 248), (210, -90, 248), (210, 90, 234), (-210, 90, 234)],
        (0, -0.08, -1),
    )
    box(mesh, "Boards", -210, -62, 0, 210, -58, 200, (0.55, 0.45, 0.35, 1))
    box(mesh, "Wood", -150, -50, 40, 150, -10, 46, (0.6, 0.45, 0.3, 1))
    for x in (-130, 130):
        box(mesh, "Metal", x - 3, -40, 0, x + 3, -20, 40)
    box(mesh, "Metal", 250, 30, 0, 256, 36, 230)
    gltf_writer_cyl(mesh, "Sign", 253, 33, 230, 4, 32, 16)
    return mesh


def shrine():
    """A little wayside shrine (hokora) on a stone plinth, roof facing +Y."""
    mesh = gltf_writer.Mesh()
    box(mesh, "Stone", -70, -60, 0, 70, 60, 60)
    box(mesh, "Wood", -40, -35, 60, 40, 35, 150, (0.55, 0.42, 0.30, 1))
    box(mesh, "Door", -25, 35, 70, 25, 37, 135, (0.45, 0.33, 0.22, 1))
    gable_roof(mesh, 80, 70, 150, 35, 22, (0.35, 0.37, 0.40, 1), (0.55, 0.42, 0.30, 1))
    box(mesh, "Red", -4, 36, 60, 4, 50, 60, (1, 1, 1, 1))
    return mesh


def leg(mesh, material, a, b, r0, r1, sides=12):
    """A tapered cylinder from a to b, capped at b."""
    axis = norm(sub(b, a))
    helper = (0, 0, 1) if abs(axis[2]) < 0.9 else (1, 0, 0)
    u = norm(cross(axis, helper))
    v = cross(axis, u)
    part = mesh.part(material)
    ring = []
    for k in range(sides + 1):
        t = 2 * math.pi * k / sides
        d = tuple(math.cos(t) * u[i] + math.sin(t) * v[i] for i in range(3))
        ring.append(
            (
                part.vertex(tuple(a[i] + d[i] * r0 for i in range(3)), d, (k / sides, 0)),
                part.vertex(tuple(b[i] + d[i] * r1 for i in range(3)), d, (k / sides, 1)),
            )
        )
    for k in range(sides):
        p0, p1 = part.positions[ring[k][0]], part.positions[ring[k + 1][0]]
        q1 = part.positions[ring[k + 1][1]]
        n = cross(sub(p1, p0), sub(q1, p0))
        out = part.normals[ring[k][0]]
        quad = (ring[k][0], ring[k + 1][0], ring[k + 1][1], ring[k][1])
        if n[0] * out[0] + n[1] * out[1] + n[2] * out[2] < 0:
            quad = tuple(reversed(quad))
        part.quad(*quad)
    cap = part.vertex(b, axis, (0.5, 0.5))
    for k in range(sides):
        tri = (ring[k][1], ring[k + 1][1], cap)
        p0, p1 = part.positions[tri[0]], part.positions[tri[1]]
        n = cross(sub(p1, p0), sub(b, p0))
        if n[0] * axis[0] + n[1] * axis[1] + n[2] * axis[2] < 0:
            tri = (tri[1], tri[0], cap)
        part.triangle(*tri)


def tetrapod():
    """A concrete tetrapod: four tapered legs out to a tetrahedron's corners, 2.4 m across."""
    mesh = gltf_writer.Mesh()
    corners = [
        (0, 0, 1),
        (0.9428, 0, -0.3333),
        (-0.4714, 0.8165, -0.3333),
        (-0.4714, -0.8165, -0.3333),
    ]
    for c in corners:
        leg(mesh, "Concrete", (0, 0, 0), tuple(v * 125 for v in c), 48, 30)
    return mesh


def main():
    os.makedirs(OUT, exist_ok=True)
    for k in range(10):
        house(k).write(os.path.join(OUT, f"H_{k:02d}.gltf"))
    for name, build in (
        ("blockwall", block_wall),
        ("vending", vending),
        ("mirror", curve_mirror),
        ("torii", torii),
        ("lantern", lantern),
        ("kura", kura),
        ("shelter", shelter),
        ("shrine", shrine),
        ("tetrapod", tetrapod),
    ):
        build().write(os.path.join(OUT, f"K_{name}.gltf"))
    print(sorted(os.listdir(OUT)))


if __name__ == "__main__":
    main()
