"""Writes the gorge village's kit as glTF into assets_gen/kit, with the textures it needs.

    V_inn_<n>         inns (ryokan) facing local +Y: dark timber ground floor with lattice (koshi)
                      over lit
                      paper screens, a noren in the doorway, a tiled eave, a lit upper floor with a
                      rail,
                      and a tiled roof. V_inn_<n>.json lists its lamps (local x, y, z) for point
                      lights.
    V_toro            the red wooden lantern on a post that lines the road and the shrine steps
    V_chochin_<n>     a paper lantern hanging from a bracket (red, white)
    V_fence           2 m of bamboo fence (misu-gaki) along X, facing +Y
    V_maple_<n>       Japanese maple (momiji) 5-8 m, leaning over local +Y: green, red, and green
                      turning
    V_ajisai_<n>      a hydrangea bush in flower
    V_rain            three nested open cylinders round the camera, for falling rain
    textures/momiji_{green,red}_leaves_*.png and textures/rain_streaks.png

Slots: Wood, Lattice, Shoji, Noren, Lamp, Chochin, Plaster, RoofTile, Fascia, Stone, Red, Paper,
Culm,
MapleBark, MomijiGreen, MomijiRed, Ajisai, AjisaiLeaf, Rain, Interior.
Run with system Python: python3 -I scripts/make_village.py
"""

import json
import math
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer
import make_bamboo
import make_houses
from make_houses import box, cross, face, norm, sub

TEX = "/workspace/ue/textures"
OUT = "/workspace/ue/assets_gen/kit"
SIZE = 512
WHITE = (1, 1, 1, 1)


# --- textures -----------------------------------------------------------------------------


def momiji_textures():
    """Palmate seven-lobed leaves on fine twigs, drawn once for shape, coloured twice: summer green
    and autumn red (with orange and the odd leaf still yellow-green)."""
    rng = random.Random(23)
    leaves = []
    twigs = []
    for k in range(6):
        x0 = SIZE * (0.12 + 0.76 * k / 5) + rng.uniform(-20, 20)
        angle = math.radians(rng.uniform(55, 125))
        length = rng.uniform(280, 460)
        twigs.append((x0, 6.0, angle, length))
        for _ in range(16):
            t = rng.uniform(0.05, 1.0)
            bx, by = x0 + math.cos(angle) * length * t, 6 + math.sin(angle) * length * t
            a = angle + rng.choice([-1, 1]) * math.radians(rng.uniform(25, 80))
            stalk = rng.uniform(10, 26)
            leaves.append(
                (
                    bx + math.cos(a) * stalk,
                    by + math.sin(a) * stalk,
                    a,
                    rng.uniform(22, 34),
                    rng.random(),
                    rng.uniform(0.8, 1.1),
                )
            )
    index = [[-1] * SIZE for _ in range(SIZE)]
    shade = [[0.0] * SIZE for _ in range(SIZE)]
    for n, (cx, cy, a, r, _, light) in enumerate(leaves):
        for py in range(int(max(0, cy - r)), int(min(SIZE, cy + r + 1))):
            for px in range(int(max(0, cx - r)), int(min(SIZE, cx + r + 1))):
                dx, dy = px - cx, py - cy
                d = math.hypot(dx, dy)
                if d > r:
                    continue
                # Angle from the leaf's tip direction; the stalk end (behind) has no lobe.
                theta = math.atan2(dy, dx) - a
                theta = (theta + math.pi) % (2 * math.pi) - math.pi
                if abs(theta) > math.radians(155):
                    continue
                lobe = abs(math.cos(theta * 7 / 2.4))  # seven lobes across the fan
                reach = r * (0.32 + 0.68 * lobe**2.2) * (1 - 0.25 * abs(theta) / math.pi)
                if d > reach:
                    continue
                index[py][px] = n
                # Veins: darker along each lobe's midline, lighter between.
                shade[py][px] = light * (0.82 + 0.18 * (1 - lobe)) * (1.05 - 0.25 * d / r)
    for x0, y0, angle, length in twigs:
        for k in range(int(length)):
            px, py = int(x0 + math.cos(angle) * k), int(y0 + math.sin(angle) * k)
            if 0 <= px < SIZE and 0 <= py < SIZE and index[py][px] < 0:
                index[py][px] = -2
    palettes = {
        "momiji_green": lambda p: (0.36, 0.58, 0.12) if p < 0.85 else (0.48, 0.62, 0.14),
        "momiji_red": lambda p: (
            (0.78, 0.10, 0.04)
            if p < 0.55
            else (0.85, 0.30, 0.04)
            if p < 0.85
            else (0.75, 0.62, 0.10)
        ),
    }
    for kind, palette in palettes.items():
        rows, alpha = [], []
        for y in range(SIZE):
            row, arow = [], []
            for x in range(SIZE):
                n = index[y][x]
                if n == -2:
                    row += [70, 45, 35]
                    arow.append(255)
                elif n >= 0:
                    c = palette(leaves[n][4])
                    s = shade[y][x]
                    row += [int(min(255, 255 * c[i] * s)) for i in range(3)]
                    arow.append(255)
                else:
                    row += [40, 30, 20]
                    arow.append(0)
            rows.append(row)
            alpha.append(arow)
        make_bamboo.png(f"{TEX}/{kind}_leaves_diff.png", SIZE, SIZE, rows, 3)
        make_bamboo.png(f"{TEX}/{kind}_leaves_alpha.png", SIZE, SIZE, alpha, 1)
        make_bamboo.png(
            f"{TEX}/{kind}_leaves_nor.png", 4, 4, [[128, 128, 255] * 4 for _ in range(4)], 3
        )
        make_bamboo.png(
            f"{TEX}/{kind}_leaves_arm.png", 4, 4, [[255, 190, 0] * 4 for _ in range(4)], 3
        )


def rain_texture():
    """Thin falling streaks, bright at the head and fading up the tail, on black: 256 x 1024."""
    w, h = 256, 1024
    rng = random.Random(5)
    img = [[0] * w for _ in range(h)]
    for _ in range(170):
        x = rng.randrange(w)
        length = rng.uniform(60, 220)
        head = rng.uniform(0, h)
        strength = rng.uniform(0.35, 1.0)
        for k in range(int(length)):
            y = int(head - k) % h
            v = strength * (1 - k / length) ** 0.7
            img[y][x] = max(img[y][x], int(255 * v))
            if rng.random() < 0.5:
                img[y][(x + 1) % w] = max(img[y][(x + 1) % w], int(120 * v))
    make_bamboo.png(f"{TEX}/rain_streaks.png", w, h, img, 1)


# --- shapes -------------------------------------------------------------------------------


def card(mesh, material, at, out, across, length, width):
    """A two-sided card with its top edge centred at `at`, reaching `length` along `out`."""
    part = mesh.part(material)
    corners = []
    for u, v in ((0, 0), (1, 0), (1, 1), (0, 1)):
        a = (u - 0.5) * width
        corners.append(
            (tuple(at[i] + across[i] * a + out[i] * v * length for i in range(3)), (u, v))
        )
    normal = norm(cross(across, out))
    if normal[2] < 0:
        normal = tuple(-n for n in normal)
    front = [part.vertex(p, normal, uv) for p, uv in corners]
    back = [part.vertex(p, tuple(-n for n in normal), uv) for p, uv in corners]
    cr = cross(sub(corners[1][0], corners[0][0]), sub(corners[2][0], corners[0][0]))
    if sum(cr[i] * normal[i] for i in range(3)) >= 0:
        part.quad(*front)
        part.quad(back[3], back[2], back[1], back[0])
    else:
        part.quad(front[3], front[2], front[1], front[0])
        part.quad(*back)


def tube(mesh, material, points, radii, colour, sides=8):
    part = mesh.part(material)
    prev = None
    for k, (p, r) in enumerate(zip(points, radii, strict=False)):
        q = points[min(k + 1, len(points) - 1)]
        o = points[max(k - 1, 0)]
        axis = norm(sub(q, o)) if q != o else (0, 0, 1)
        side = norm(cross(axis, (0, 0, 1) if abs(axis[2]) < 0.9 else (1, 0, 0)))
        up = cross(side, axis)
        ring = []
        for j in range(sides + 1):
            a = 2 * math.pi * j / sides
            n = tuple(side[i] * math.cos(a) + up[i] * math.sin(a) for i in range(3))
            ring.append(
                part.vertex(
                    tuple(p[i] + n[i] * r for i in range(3)), n, (j / sides, k * 0.5), colour
                )
            )
        if prev:
            for j in range(sides):
                part.quad(prev[j], ring[j], ring[j + 1], prev[j + 1])
        prev = ring


def blob(mesh, material, centre, radius, colour, rng, squash=1.0, rings=5, sides=8):
    """A lumpy, faceted ball (flower heads, leaf masses)."""
    part = mesh.part(material)
    cx, cy, cz = centre
    grid = []
    for i in range(rings + 1):
        phi = math.pi * i / rings
        row = []
        for j in range(sides + 1):
            th = 2 * math.pi * j / sides
            n = (math.sin(phi) * math.cos(th), math.sin(phi) * math.sin(th), math.cos(phi))
            r = radius * (0.85 + 0.3 * rng.random()) if 0 < i < rings and j < sides else radius
            row.append(
                part.vertex(
                    (cx + n[0] * r, cy + n[1] * r, cz + n[2] * r * squash),
                    n,
                    (j / sides, i / rings),
                    colour,
                )
            )
        row[sides] = row[0]
        grid.append(row)
    for i in range(rings):
        for j in range(sides):
            a, b, c, d = grid[i][j], grid[i][j + 1], grid[i + 1][j + 1], grid[i + 1][j]
            part.quad(a, d, c, b)


def cyl(mesh, material, x, y, z, height, r, sides=10, colour=WHITE, r_top=None):
    part = mesh.part(material)
    r_top = r if r_top is None else r_top
    ring = []
    for k in range(sides + 1):
        t = 2 * math.pi * k / sides
        c, s = math.cos(t), math.sin(t)
        ring.append(
            (
                part.vertex((x + c * r, y + s * r, z), (c, s, 0), (k / sides, 0), colour),
                part.vertex(
                    (x + c * r_top, y + s * r_top, z + height),
                    (c, s, 0),
                    (k / sides, height / 100),
                    colour,
                ),
            )
        )
    for k in range(sides):
        (a, b), (d, c) = ring[k], ring[k + 1]
        part.quad(a, d, c, b)
    cap = part.vertex((x, y, z + height), (0, 0, 1), (0.5, 0.5), colour)
    for k in range(sides):
        part.triangle(ring[k][1], ring[k + 1][1], cap)


# --- the inn --------------------------------------------------------------------------------

TIMBER = [(0.20, 0.13, 0.08, 1), (0.16, 0.11, 0.08, 1), (0.26, 0.17, 0.10, 1)]
NOREN = [(0.10, 0.14, 0.32, 1), (0.42, 0.10, 0.08, 1), (0.85, 0.80, 0.70, 1), (0.30, 0.22, 0.14, 1)]
UPPER = [(0.93, 0.88, 0.76, 1), (0.85, 0.78, 0.64, 1), (0.30, 0.22, 0.15, 1)]
ROOF = [(0.22, 0.23, 0.25, 1), (0.30, 0.31, 0.33, 1), (0.18, 0.19, 0.20, 1)]


def lattice_bay(mesh, x0, x1, z0, z1, y, timber, lit=True, pitch=9.0):
    """Koshi: thin vertical slats over a paper screen lit from inside."""
    if lit:
        box(mesh, "Shoji", x0, y - 14, z0, x1, y - 12, z1)
    else:
        box(mesh, "Interior", x0, y - 14, z0, x1, y - 12, z1)
    x = x0 + 3
    while x < x1 - 3:
        box(mesh, "Lattice", x - 1.6, y - 6, z0, x + 1.6, y - 1, z1, timber)
        x += pitch
    box(
        mesh,
        "Lattice",
        x0,
        y - 7,
        z0 + (z1 - z0) * 0.5 - 2,
        x1,
        y - 1,
        z0 + (z1 - z0) * 0.5 + 2,
        timber,
    )


def inn(seed):
    rng = random.Random(seed * 31 + 7)
    mesh = gltf_writer.Mesh()
    lamps = []
    w = rng.choice([780, 900, 1080, 1260])
    d = rng.choice([800, 900])
    timber = rng.choice(TIMBER)
    upper_wall = rng.choice(UPPER)
    roof = rng.choice(ROOF)
    y = d / 2
    base = 35
    h1, h2 = 290, 250
    # Footing and the ground floor's shell.
    box(mesh, "Stone", -w / 2 - 4, -d / 2 - 4, -60, w / 2 + 4, y + 4, base)
    box(mesh, "Wood", -w / 2, -d / 2, base, w / 2, y - 15, base + h1, timber, uv_scale=200.0)
    # Front: posts every bay, lattice between, one bay the doorway.
    bays = max(3, round(w / 180))
    bay = w / bays
    door = rng.randrange(1, bays - 1)
    for k in range(bays + 1):
        x = -w / 2 + k * bay
        box(mesh, "Wood", x - 8, y - 16, base, x + 8, y, base + h1, timber)
    for k in range(bays):
        x0, x1 = -w / 2 + k * bay + 8, -w / 2 + (k + 1) * bay - 8
        if k == door:
            # A dark doorway with a noren across its top and a lantern beside it.
            box(mesh, "Interior", x0, y - 60, base, x1, y - 58, base + 210)
            box(mesh, "Shoji", x0 + 20, y - 62, base + 20, x1 - 20, y - 60, base + 190)
            colour = rng.choice(NOREN)
            strips = 3 if x1 - x0 > 130 else 2
            for j in range(strips):
                sx0 = x0 + (x1 - x0) * j / strips + 1.5
                sx1 = x0 + (x1 - x0) * (j + 1) / strips - 1.5
                box(
                    mesh,
                    "Noren",
                    sx0,
                    y + 2,
                    base + 210 - rng.uniform(95, 115),
                    sx1,
                    y + 4,
                    base + 215,
                    colour,
                )
            box(mesh, "Wood", x0 - 6, y, base + 213, x1 + 6, y + 8, base + 222, timber)
            lx = x1 + 30 if k < bays - 1 else x0 - 30
            chochin(
                mesh,
                (lx, y + 40, base + 205),
                (0.80, 0.12, 0.06, 1) if rng.random() < 0.6 else (0.95, 0.92, 0.85, 1),
            )
            lamps.append((lx, y + 60, base + 200))
        else:
            # A low panel, then lattice to the lintel.
            box(mesh, "Wood", x0, y - 15, base, x1, y - 5, base + 55, timber)
            lattice_bay(mesh, x0, x1, base + 55, base + 225, y, timber, lit=rng.random() < 0.8)
    box(mesh, "Wood", -w / 2, y - 16, base + 225, w / 2, y + 2, base + h1, timber)
    # The tiled eave (hisashi) across the front at the first floor.
    z = base + h1 + 10
    reach = 95
    up = norm((0, reach * 0.45, reach))
    pts = [
        (-w / 2 - 20, y, z),
        (w / 2 + 20, y, z),
        (w / 2 + 20, y + reach, z - reach * 0.45),
        (-w / 2 - 20, y + reach, z - reach * 0.45),
    ]
    face(mesh, "RoofTile", pts, up, roof)
    face(mesh, "Fascia", [(p[0], p[1], p[2] - 4) for p in pts], (0, -up[1], -up[2]))
    face(
        mesh,
        "Fascia",
        [
            (-w / 2 - 20, y + reach, z - reach * 0.45),
            (w / 2 + 20, y + reach, z - reach * 0.45),
            (w / 2 + 20, y + reach, z - reach * 0.45 - 12),
            (-w / 2 - 20, y + reach, z - reach * 0.45 - 12),
        ],
        (0, 1, 0),
    )
    # The upper floor, set back a little, with lit windows behind a wooden rail.
    z0 = base + h1
    setback = rng.choice([0, 30, 60])
    yu = y - setback
    box(mesh, "Plaster", -w / 2, -d / 2, z0, w / 2, yu - 6, z0 + h2, upper_wall, uv_scale=200.0)
    windows = max(2, int(w // 260))
    for k in range(windows):
        cx = -w / 2 + w * (k + 0.5) / windows
        ww = min(220, w / windows - 50)
        lit = rng.random() < 0.75
        box(
            mesh,
            "Shoji" if lit else "Interior",
            cx - ww / 2,
            yu - 8,
            z0 + 60,
            cx + ww / 2,
            yu - 6,
            z0 + 210,
        )
        # A grid of thin bars over the paper.
        for j in range(5):
            bx = cx - ww / 2 + ww * j / 4
            box(mesh, "Lattice", bx - 2.5, yu - 6, z0 + 60, bx + 2.5, yu - 2, z0 + 210, timber)
        for zz in (z0 + 60, z0 + 135, z0 + 210):
            box(
                mesh,
                "Lattice",
                cx - ww / 2,
                yu - 6,
                zz - 2.5,
                cx + ww / 2,
                yu - 2,
                zz + 2.5,
                timber,
            )
    for k in range(windows + 1):
        x = -w / 2 + w * k / windows
        box(mesh, "Wood", x - 7, yu - 8, z0, x + 7, yu, z0 + h2, timber)
    box(mesh, "Wood", -w / 2, yu - 8, z0 + h2 - 18, w / 2, yu, z0 + h2, timber)
    if rng.random() < 0.6:
        # A rail along the upper windows.
        ry = yu + 35
        box(mesh, "Wood", -w / 2 + 10, yu - 4, z0 + 30, w / 2 - 10, ry, z0 + 38, timber)
        box(mesh, "Wood", -w / 2 + 10, ry - 5, z0 + 95, w / 2 - 10, ry, z0 + 101, timber)
        x = -w / 2 + 14
        while x < w / 2 - 10:
            box(mesh, "Wood", x - 2, ry - 5, z0 + 38, x + 2, ry - 1, z0 + 95, timber)
            x += 14
    pitch = rng.choice([24, 27, 30])
    if rng.random() < 0.6:
        make_houses.gable_roof(mesh, w, d - setback, z0 + h2, pitch, 75, roof, upper_wall)
    else:
        make_houses.hip_roof(mesh, w, d - setback, z0 + h2, pitch, 75, roof)
    # The gable roof is centred on the box; shift isn't needed for small setbacks.
    # A sign board over the eave on some.
    if rng.random() < 0.5:
        sx = rng.uniform(-w / 4, w / 4)
        box(
            mesh,
            "Wood",
            sx - 70,
            y - 4,
            base + h1 - 10,
            sx + 70,
            y + 4,
            base + h1 + 32,
            (0.12, 0.08, 0.05, 1),
        )
        box(mesh, "Lamp", sx - 60, y + 4, base + h1 - 2, sx + 60, y + 5, base + h1 + 24)
    # Light spilling from the front through the lattice.
    lamps.append((0, y + 120, base + 180))
    return mesh, lamps


def chochin(mesh, at, colour):
    """A paper lantern hanging from a bracket: a ribbed barrel with dark caps."""
    x, y, z = at
    box(mesh, "Fascia", x - 2, y - 40, z + 8, x + 2, y + 2, z + 12)
    box(mesh, "Fascia", x - 1, y - 1, z - 2, x + 1, y + 1, z + 10)
    part = mesh.part("Chochin")
    rings = []
    for i in range(9):
        t = i / 8
        r = 18 * math.sin(math.pi * (0.12 + 0.76 * t))
        zz = z - 4 - 50 * t
        ring = []
        for j in range(13):
            a = 2 * math.pi * j / 12
            n = (math.cos(a), math.sin(a), (0.5 - t) * 0.8)
            ring.append(
                part.vertex((x + math.cos(a) * r, y + math.sin(a) * r, zz), n, (j / 12, t), colour)
            )
        rings.append(ring)
    for i in range(8):
        for j in range(12):
            part.quad(rings[i][j], rings[i + 1][j], rings[i + 1][j + 1], rings[i][j + 1])
    cyl(mesh, "Fascia", x, y, z - 8, 6, 10, 10)
    cyl(mesh, "Fascia", x, y, z - 60, 6, 10, 10)


def chochin_kit(colour):
    mesh = gltf_writer.Mesh()
    chochin(mesh, (0, 0, 0), colour)
    return mesh


def toro():
    """The red lantern post: a square red post, a lantern box with paper sides, a little cap."""
    mesh = gltf_writer.Mesh()
    red = (0.70, 0.08, 0.03, 1)
    box(mesh, "Red", -7, -7, -30, 7, 7, 160, red)
    box(mesh, "Red", -20, -20, 160, 20, 20, 166, red)
    box(mesh, "Paper", -16, -16, 166, 16, 16, 206)
    for sx in (-1, 1):
        for sy in (-1, 1):
            box(
                mesh,
                "Red",
                sx * 17 - 2.5,
                sy * 17 - 2.5,
                166,
                sx * 17 + 2.5,
                sy * 17 + 2.5,
                206,
                red,
            )
    box(mesh, "Red", -20, -20, 206, 20, 20, 211, red)
    # A pyramid cap.
    for k, n in enumerate(((0, -1, 1), (1, 0, 1), (0, 1, 1), (-1, 0, 1))):
        corners = [(-25, -25), (25, -25), (25, 25), (-25, 25)]
        a, b = corners[k], corners[(k + 1) % 4]
        face(mesh, "Fascia", [(a[0], a[1], 211), (b[0], b[1], 211), (0, 0, 232)], n)
    cyl(mesh, "Fascia", 0, 0, 230, 8, 3, 6)
    return mesh


def fence():
    """Two metres of bamboo fence: close split-bamboo slats between posts and three rails."""
    mesh = gltf_writer.Mesh()
    rng = random.Random(4)
    post = (0.45, 0.36, 0.22, 1)
    for x in (-100, 100):
        cyl(mesh, "Culm", x, 0, -20, 175, 5, 8, post)
    x = -96
    while x < 96:
        tone = rng.uniform(0.85, 1.1)
        c = (0.62 * tone, 0.52 * tone, 0.30 * tone, 1)
        box(mesh, "Culm", x, -2, 0, x + 3.6, 2, 150 + rng.uniform(-3, 3), c)
        x += 4.0
    for z in (25, 85, 140):
        cyl_x(mesh, "Culm", -100, 100, 3.5, z, 2.4, (0.40, 0.34, 0.20, 1))
    return mesh


def cyl_x(mesh, material, x0, x1, y, z, r, colour, sides=8):
    part = mesh.part(material)
    ring = []
    for k in range(sides + 1):
        t = 2 * math.pi * k / sides
        n = (0, math.cos(t), math.sin(t))
        ring.append(
            (
                part.vertex((x0, y + n[1] * r, z + n[2] * r), n, (0, k / sides), colour),
                part.vertex(
                    (x1, y + n[1] * r, z + n[2] * r), n, ((x1 - x0) / 100, k / sides), colour
                ),
            )
        )
    for k in range(sides):
        (a, b), (d, c) = ring[k], ring[k + 1]
        part.quad(a, b, c, d)


# --- plants ---------------------------------------------------------------------------------


def maple(seed, mix):
    """A Japanese maple: a short leaning trunk forking into limbs that rise and then arch out and
    down,
    finer branches off them, and many small leaf sprays round the ends, so the crown is a layered
    dome of lace. `mix` is the share of red sprays: 0 green, 1 red."""
    rng = random.Random(seed * 101 + 9)
    mesh = gltf_writer.Mesh()
    bark = (0.30, 0.28, 0.25, 1)
    height = rng.uniform(260, 360)
    lean = math.radians(rng.uniform(10, 20))
    trunk = [
        (rng.gauss(0, 3), math.sin(lean) * height * t, -20 + height * t)
        for t in (0, 0.25, 0.5, 0.75, 1.0)
    ]
    tube(mesh, "MapleBark", trunk, [16, 13, 11, 9, 7], bark, 9)
    top = trunk[-1]
    sprays = []

    def limb(base, yaw, reach, rise, radius, depth):
        # Up and out, then arching over and down at the tip.
        pts = [base]
        for t in (0.3, 0.6, 0.85, 1.0):
            arch = rise * t * (1.5 - 0.6 * t)  # rises all the way, flattening at the tip
            pts.append(
                (
                    base[0] + math.cos(yaw) * reach * t + rng.uniform(-8, 8),
                    base[1] + math.sin(yaw) * reach * t + rng.uniform(-8, 8),
                    base[2] + arch,
                )
            )
        tube(
            mesh,
            "MapleBark",
            pts,
            [radius, radius * 0.75, radius * 0.5, radius * 0.32, radius * 0.18],
            bark,
            6,
        )
        for p in pts[2:]:
            sprays.append(p)
        if depth > 0:
            for _ in range(rng.randint(2, 3)):
                k = rng.randint(1, 3)
                limb(
                    pts[k],
                    yaw + rng.uniform(-0.9, 0.9),
                    reach * rng.uniform(0.35, 0.55),
                    rise * 0.5,
                    radius * 0.45,
                    depth - 1,
                )

    for k in range(rng.randint(4, 6)):
        yaw = 2 * math.pi * k / 5 + rng.uniform(-0.35, 0.35)
        bias = 1.0 + 0.5 * max(0.0, math.sin(yaw))
        limb(top, yaw, rng.uniform(170, 250) * bias, rng.uniform(110, 180), 4.5, 1)
    sprays.append((top[0], top[1], top[2] + 160))
    for tip in sprays:
        for _ in range(rng.randint(4, 6)):
            yaw = rng.uniform(0, 2 * math.pi)
            tilt = math.radians(rng.uniform(-35, 5))
            out = (math.cos(yaw) * math.cos(tilt), math.sin(yaw) * math.cos(tilt), math.sin(tilt))
            across = (-math.sin(yaw), math.cos(yaw), rng.uniform(-0.3, 0.3))
            at = (
                tip[0] + rng.uniform(-35, 35),
                tip[1] + rng.uniform(-35, 35),
                tip[2] + rng.uniform(-15, 30),
            )
            red = rng.random() < mix
            card(
                mesh,
                "MomijiRed" if red else "MomijiGreen",
                at,
                out,
                norm(across),
                rng.uniform(70, 105),
                rng.uniform(75, 110),
            )
    return mesh


def ajisai(seed):
    """A hydrangea: a mound of broad folded leaves under mophead flowers, each head a dome of small
    four-petalled florets, blue to violet (pink on some)."""
    rng = random.Random(seed * 7 + 2)
    mesh = gltf_writer.Mesh()
    hues = [
        (0.25, 0.38, 0.85, 1),
        (0.42, 0.40, 0.88, 1),
        (0.55, 0.38, 0.80, 1),
        (0.30, 0.55, 0.85, 1),
    ]
    if seed % 3 == 2:
        hues = [(0.85, 0.45, 0.65, 1), (0.78, 0.40, 0.72, 1), (0.90, 0.60, 0.75, 1)]
    leaves = mesh.part("AjisaiLeaf")

    def two_sided(part, pts, normal, colour):
        front = [part.vertex(p, normal, (0, 0), colour) for p in pts]
        # Both faces share one normal, so an underside seen from the path isn't black.
        back = [part.vertex(p, normal, (0, 0), colour) for p in pts]
        for i in range(1, len(pts) - 1):
            part.triangle(front[0], front[i], front[i + 1])
            part.triangle(back[0], back[i + 1], back[i])

    for _ in range(90):
        # A pointed oval folded along its midrib, angled out and up from the mound.
        a = rng.uniform(0, 2 * math.pi)
        r = rng.uniform(5, 55)
        base = (math.cos(a) * r * 0.6, math.sin(a) * r * 0.6, rng.uniform(10, 55))
        rise = math.radians(rng.uniform(5, 45))
        yaw = a + rng.uniform(-0.6, 0.6)
        out = (math.cos(yaw) * math.cos(rise), math.sin(yaw) * math.cos(rise), math.sin(rise))
        side = (-math.sin(yaw), math.cos(yaw), 0)
        length, width = rng.uniform(13, 19), rng.uniform(5, 7)
        tip = tuple(base[i] + out[i] * length for i in range(3))
        mid = tuple(base[i] + out[i] * length * 0.45 for i in range(3))
        fold = 1.5
        left = tuple(mid[i] + side[i] * width + (0, 0, fold)[i] for i in range(3))
        rr = tuple(mid[i] - side[i] * width + (0, 0, fold)[i] for i in range(3))
        normal = norm(cross(out, side))
        if normal[2] < 0:
            normal = tuple(-n for n in normal)
        normal = norm(tuple(normal[i] + out[i] * 0.5 + (0, 0, 0.5)[i] for i in range(3)))
        g = rng.uniform(0.8, 1.15)
        colour = (0.13 * g, 0.28 * g, 0.08 * g, 1)
        two_sided(leaves, [base, left, tip], normal, colour)
        two_sided(leaves, [base, tip, rr], normal, colour)

    flowers = mesh.part("Ajisai")
    for _ in range(rng.randint(14, 20)):
        a = rng.uniform(0, 2 * math.pi)
        r = rng.uniform(5, 50)
        centre = (math.cos(a) * r, math.sin(a) * r, rng.uniform(52, 72) - r * 0.3)
        radius = rng.uniform(9, 12)
        hue = rng.choice(hues)
        # A solid dome under the florets, so a head reads as one mass from the path, not loose
        # specks.
        core = (*tuple(c * 0.75 for c in hue[:3]), 1)
        rings = []
        for k in range(5):
            el = math.radians(-30 + k * 30)
            rings.append(
                [
                    tuple(
                        centre[i]
                        + (math.cos(el) * math.cos(t), math.cos(el) * math.sin(t), math.sin(el))[i]
                        * radius
                        * 0.9
                        for i in range(3)
                    )
                    for t in (j * 2 * math.pi / 10 for j in range(10))
                ]
            )
        for k in range(4):
            for j in range(10):
                a0, a1 = rings[k][j], rings[k][(j + 1) % 10]
                b0, b1 = rings[k + 1][j], rings[k + 1][(j + 1) % 10]
                q = [
                    flowers.vertex(p, norm(tuple(p[i] - centre[i] for i in range(3))), (0, 0), core)
                    for p in (a0, a1, b1, b0)
                ]
                # Both windings: the cap is closed by the florets, and a stray miss shows nothing.
                for t in ((0, 2, 1), (0, 3, 2), (0, 1, 2), (0, 2, 3)):
                    flowers.triangle(q[t[0]], q[t[1]], q[t[2]])
        for _ in range(34):
            # Florets over the upper three quarters of a ball, each a flat cross of four petals.
            z = rng.uniform(-0.5, 1)
            th = rng.uniform(0, 2 * math.pi)
            n = (math.sqrt(1 - z * z) * math.cos(th), math.sqrt(1 - z * z) * math.sin(th), z)
            at = tuple(centre[i] + n[i] * radius for i in range(3))
            u = norm(cross(n, (0, 0, 1)) if abs(n[2]) < 0.95 else (1, 0, 0))
            v = cross(n, u)
            spin = rng.uniform(0, math.pi / 2)
            u, v = (
                tuple(u[i] * math.cos(spin) + v[i] * math.sin(spin) for i in range(3)),
                tuple(v[i] * math.cos(spin) - u[i] * math.sin(spin) for i in range(3)),
            )
            size = rng.uniform(3.6, 4.8)
            shade = rng.uniform(0.85, 1.12)
            colour = (*tuple(min(1.0, c * shade) for c in hue[:3]), 1)
            for axis in (u, v):
                for sign in (1, -1):
                    tip = tuple(at[i] + axis[i] * sign * size for i in range(3))
                    w = tuple((v if axis is u else u)[i] * size * 0.45 for i in range(3))
                    mid = tuple(at[i] + axis[i] * sign * size * 0.5 for i in range(3))
                    two_sided(
                        flowers,
                        [
                            at,
                            tuple(mid[i] + w[i] for i in range(3)),
                            tip,
                            tuple(mid[i] - w[i] for i in range(3)),
                        ],
                        n,
                        colour,
                    )
    return mesh


def rain():
    """Nested open cylinders round the camera, their UVs in metres round and up each."""
    mesh = gltf_writer.Mesh()
    part = mesh.part("Rain")
    for radius, tile in ((250, 150), (600, 220), (1400, 320)):
        sides = 40
        z0, z1 = -900, 1500
        ring = []
        for k in range(sides + 1):
            a = 2 * math.pi * k / sides
            n = (-math.cos(a), -math.sin(a), 0)
            u = radius * a / tile
            ring.append(
                (
                    part.vertex(
                        (math.cos(a) * radius, math.sin(a) * radius, z0), n, (u, -z0 / (tile * 4))
                    ),
                    part.vertex(
                        (math.cos(a) * radius, math.sin(a) * radius, z1), n, (u, -z1 / (tile * 4))
                    ),
                )
            )
        for k in range(sides):
            (a, b), (d, c) = ring[k], ring[k + 1]
            part.quad(a, b, c, d)
            part.quad(d, c, b, a)
    return mesh


def tuft(seed):
    """A knee-high clump of rough grass: 40-60 tapered blades fanning up and out from a tight base,
    mostly green with a few straw-dry ones, so an uncut verge reads as grass from the saddle."""
    rng = random.Random(seed * 13 + 5)
    mesh = gltf_writer.Mesh()
    part = mesh.part("Tuft")
    # Three kinds, by seed % 3, so a verge isn't one clump repeated: a short dense bunch, the
    # knee-high fan, and a tall sparse stand going to seed, drier and more upright.
    kind = seed % 3
    blades, spread, tall, leans, dry = (
        ((90, 140), 6, (10, 28), (15, 60), 0.10)
        if kind == 0
        else ((60, 110), 11, (25, 65), (8, 50), 0.18)
        if kind == 1
        else ((40, 70), 16, (38, 70), (6, 35), 0.40)
    )
    for _ in range(rng.randint(*blades)):
        a = rng.uniform(0, 2 * math.pi)
        r = rng.uniform(0, spread) * rng.uniform(0.4, 1.0)
        base = (math.cos(a) * r, math.sin(a) * r, 0.0)
        yaw = a + rng.uniform(-0.8, 0.8)
        lean = math.radians(rng.uniform(*leans))
        height = rng.uniform(*tall)
        width = rng.uniform(1.2, 2.0) * (0.8 if kind == 2 else 1.0)
        out = (math.cos(yaw), math.sin(yaw), 0.0)
        side = (-math.sin(yaw), math.cos(yaw), 0.0)
        if rng.random() < dry:
            g = rng.uniform(0.8, 1.1)
            tint = (0.42 * g, 0.36 * g, 0.18 * g)
        else:
            g = rng.uniform(0.6, 1.0)
            hue = rng.uniform(-0.03, 0.04) + (seed % 4) * 0.012
            tint = ((0.12 + hue) * g, 0.22 * g, (0.06 + hue * 0.3) * g)
        # Shared up-and-out normal, so both faces light alike and the clump has one soft shading.
        normal = norm((out[0] * 0.6, out[1] * 0.6, 1.0))
        rows = []
        n = 5
        for k in range(n + 1):
            t = k / n
            # The blade arches: it leans further the higher it gets.
            bend = lean * (0.3 + 0.9 * t)
            d = height * t
            centre = (
                base[0] + out[0] * math.sin(bend) * d,
                base[1] + out[1] * math.sin(bend) * d,
                math.cos(bend) * d,
            )
            w = width * (1 - t) + 0.05
            shade = 0.55 + 0.45 * t
            c = (tint[0] * shade, tint[1] * shade, tint[2] * shade, 1)
            rows.append(
                [
                    part.vertex(
                        tuple(centre[i] + side[i] * w * sgn for i in range(3)), normal, (0, 0), c
                    )
                    for sgn in (-1, 1)
                ]
            )
        for k in range(n):
            a0, a1 = rows[k]
            b0, b1 = rows[k + 1]
            for tri in ((a0, a1, b1), (a0, b1, b0)):
                part.triangle(*tri)
                part.triangle(tri[0], tri[2], tri[1])
    return mesh


def main():
    os.makedirs(OUT, exist_ok=True)
    momiji_textures()
    rain_texture()
    for k in range(5):
        mesh, lamps = inn(k)
        mesh.write(f"{OUT}/V_inn_{k}.gltf")
        with open(f"{OUT}/V_inn_{k}.json", "w") as f:
            json.dump({"lamps": lamps}, f)
    toro().write(f"{OUT}/V_toro.gltf")
    chochin_kit((0.80, 0.12, 0.06, 1)).write(f"{OUT}/V_chochin_0.gltf")
    chochin_kit((0.95, 0.92, 0.85, 1)).write(f"{OUT}/V_chochin_1.gltf")
    fence().write(f"{OUT}/V_fence.gltf")
    for k, mix in enumerate((0.0, 0.0, 1.0, 0.85, 0.3, 0.1)):
        maple(k, mix).write(f"{OUT}/V_maple_{k}.gltf")
    for k in range(3):
        ajisai(k).write(f"{OUT}/V_ajisai_{k}.gltf")
    for k in range(12):
        tuft(k).write(f"{OUT}/V_tuft_{k}.gltf")
    rain().write(f"{OUT}/V_rain.gltf")
    print("village kit written")


if __name__ == "__main__":
    main()
