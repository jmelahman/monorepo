"""The monolith in the plaza: a memorial to something nobody remembers, cast in lifts, cracked
open down one side, and strapped together with steel when it started to come apart.

blender -b --python scripts/blender/monolith.py

The front faces -Y in Blender, which is +Z in Godot, and the origin is the middle of its foot.
"""

import random
import sys
from pathlib import Path

import bmesh
from mathutils import Vector, noise

sys.path.insert(0, str(Path(__file__).parent))
import shapes

HIGH = 30.0
LIFTS = 7
PLINTH = 1.5
FOOT = (9.5, 3.8)
TOP = (6.4, 2.3)
LEAN = (0.6, 2.6)
CLEFT = (0.55, 2.05)
CONCRETE = (0.6, 0.61, 0.63)
DARK = (0.34, 0.35, 0.37)
RUST = (0.6, 0.48, 0.38)
STEEL = (0.32, 0.33, 0.35)
GLOW = (5.5, 2.5, 0.85)
EMBER = (3.2, 1.1, 0.3)


def _centre(z):
    """Where the body's axis has got to at this height, and how big it is there."""
    t = (z - PLINTH) / HIGH
    centre = Vector((LEAN[0] * t, LEAN[1] * t, z))
    size = (FOOT[0] + (TOP[0] - FOOT[0]) * t, FOOT[1] + (TOP[1] - FOOT[1]) * t)
    return centre, size


def _face_y(z):
    centre, size = _centre(z)
    return centre.y - size[1] / 2


def _plinth(model, dice):
    for step, (wide, deep, z0, z1) in enumerate(
        ((15.0, 10.5, 0.0, 0.5), (13.2, 8.8, 0.5, 1.0), (11.4, 7.1, 1.0, PLINTH))
    ):
        tone = 0.9 - step * 0.06
        model.box(
            "concrete",
            (-wide / 2, -deep / 2, z0),
            (wide / 2, deep / 2, z1),
            [c * tone for c in CONCRETE],
            0.05,
        )
    # Where a stair has been patched, the patches stand slightly proud.
    for _ in range(9):
        x = dice.uniform(-6.0, 6.0)
        wide = dice.uniform(0.5, 1.7)
        step = dice.randrange(3)
        y = -(10.5 - step * 1.7) / 2 - 0.01
        z = 0.5 * step
        model.box(
            "concrete",
            (x, y, z + 0.04),
            (x + wide, y + 0.5, z + dice.uniform(0.3, 0.46)),
            DARK,
            0.015,
        )


def _lifts(dice):
    """The body as stacked pours, each set slightly off the last, and every cut to come out."""
    pieces = []
    for lift in range(LIFTS):
        z0 = PLINTH + HIGH * lift / LIFTS
        z1 = PLINTH + HIGH * (lift + 1) / LIFTS
        low_c, low = _centre(z0)
        high_c, high = _centre(z1)
        shove = Vector((dice.uniform(-0.12, 0.12), dice.uniform(-0.1, 0.1), 0.0))
        low_c = low_c + shove
        high_c = high_c + shove
        piece = shapes.tapered(
            low,
            high,
            z1 - z0 - 0.0,
            (low_c.x, low_c.y, z0),
            (high_c.x - low_c.x, high_c.y - low_c.y),
        )
        pieces.append(piece)
    return pieces


def _cuts(dice, lift, z0, z1):
    """What's taken out of one lift: the cleft, the coffers on its left, and the broken top."""
    cuts = []
    wide = CLEFT[1] - CLEFT[0]
    low_y = _face_y(z0) - 1.0
    # Near the foot the cleft closes up, and it opens through the top lift.
    start = max(z0 - 0.5, PLINTH + 1.6)
    if start < z1:
        cuts.append(
            shapes.tapered(
                (wide, 3.6),
                (wide + 0.15, 3.6),
                z1 + 0.5 - start,
                ((CLEFT[0] + CLEFT[1]) / 2 + LEAN[0] * (z0 - PLINTH) / HIGH, low_y + 1.0, start),
                (0.05, 0.4),
            )
        )
    # A grid of coffers, all to one depth, with some left blank.
    rows = 4
    for row in range(rows):
        for col in range(4):
            if dice.random() < 0.16:
                continue
            high_z = z0 + 0.5 + (z1 - z0 - 1.0) * row / rows
            size_z = (z1 - z0 - 1.0) / rows - 0.22
            _, here = _centre(high_z)
            left = -here[0] / 2 + 0.45
            span = CLEFT[0] - left - 0.35
            x = left + span * col / 4 + LEAN[0] * (high_z - PLINTH) / HIGH
            y = _face_y(high_z)
            depth = dice.choice((0.28, 0.42, 0.6))
            cuts.append(
                shapes._cuboid(
                    (x, y - 1.2, high_z), (x + span / 4 - 0.18, y + depth, high_z + size_z), 0.0
                )
            )
    # The spalls: lumps gone from the corners, the rebar left showing.
    for _ in range(4 if lift < LIFTS - 1 else 8):
        z = dice.uniform(z0, z1)
        centre, here = _centre(z)
        sx = dice.choice((-1, 1))
        sy = dice.choice((-1, 1))
        at = centre + Vector((sx * here[0] / 2, sy * here[1] / 2, 0.0))
        cuts.append(_lump(dice, at, dice.uniform(0.3, 0.75)))
    if lift == LIFTS - 1:
        # The top came down once. It is a slope of rubble, not a neat finish.
        for _ in range(9):
            centre, here = _centre(z1)
            at = centre + Vector(
                (dice.uniform(-0.5, 0.5) * here[0], dice.uniform(-0.5, 0.5) * here[1], 0.0)
            )
            at.z += 0.5 + (at.x - centre.x) * 0.55
            cuts.append(_lump(dice, at, dice.uniform(1.2, 1.9)))
    return cuts


def _lump(dice, at, size):
    piece = bmesh.new()
    bmesh.ops.create_icosphere(piece, subdivisions=2, radius=size)
    offset = dice.uniform(0.0, 50.0)
    for vert in piece.verts:
        vert.co *= 0.6 + 0.8 * abs(noise.noise(vert.co * 2.1 + Vector((offset, 0.0, 0.0))))
        vert.co += at
    return piece


def _relief(model, dice):
    """Everything fixed on: ribs right of the cleft, straps, the tablet, what's in the cleft."""
    for lift in range(LIFTS):
        z0 = PLINTH + HIGH * lift / LIFTS + 0.4
        z1 = z0 + HIGH / LIFTS - 0.8
        rib = CLEFT[1] + 0.35
        _, here = _centre(z0)
        far = here[0] / 2 - 0.3
        while rib < far:
            y0 = _face_y(z0)
            y1 = _face_y(z1)
            shift0 = LEAN[0] * (z0 - PLINTH) / HIGH
            shift1 = LEAN[0] * (z1 - PLINTH) / HIGH
            start = (rib + shift0, y0 - 0.1, z0)
            end = (rib + shift1, y1 - 0.1, z1)
            model.bar("concrete", start, end, 0.26, CONCRETE, 0.025)
            rib += 0.56
    # Rungs across the cleft, and a seam of light behind them.
    z = PLINTH + 2.0
    while z < PLINTH + HIGH - 2.5:
        shift = LEAN[0] * (z - PLINTH) / HIGH
        y = _face_y(z) + 0.5
        model.bar(
            "rust",
            (CLEFT[0] - 0.1 + shift, y, z),
            (CLEFT[1] + 0.1 + shift, y, z),
            0.13,
            RUST,
        )
        z += 1.15 + dice.uniform(0.0, 0.5)
    for lift in range(LIFTS - 1):
        z0 = PLINTH + HIGH * lift / LIFTS + 0.2
        z1 = z0 + HIGH / LIFTS - 0.45
        seam = (CLEFT[0] + CLEFT[1]) / 2
        shift0 = LEAN[0] * (z0 - PLINTH) / HIGH
        shift1 = LEAN[0] * (z1 - PLINTH) / HIGH
        y0 = _face_y(z0) + 1.5
        y1 = _face_y(z1) + 1.5
        model.bar(
            "glow", (seam + shift0, y0, max(z0, PLINTH + 1.7)), (seam + shift1, y1, z1), 0.3, GLOW
        )
    _straps(model)
    _tablet(model, dice)
    _rebar(model, dice)
    _shrine(model, dice)


def _straps(model):
    """Steel bands round the body, added when it began to split."""
    for z in (PLINTH + 9.2, PLINTH + 18.9):
        centre, here = _centre(z)
        wide, deep = here[0] / 2 + 0.1, here[1] / 2 + 0.1
        x, y = centre.x, centre.y
        high = 0.55
        model.box("steel", (x - wide, y - deep, z), (x + wide, y - deep + 0.1, z + high), STEEL)
        model.box("steel", (x - wide, y + deep - 0.1, z), (x + wide, y + deep, z + high), STEEL)
        model.box("steel", (x - wide, y - deep, z), (x - wide + 0.1, y + deep, z + high), STEEL)
        model.box("steel", (x + wide - 0.1, y - deep, z), (x + wide, y + deep, z + high), STEEL)
        bolt = x - wide + 0.4
        while bolt < x + wide:
            for up in (0.13, 0.42):
                model.tube(
                    "rust",
                    (bolt, y - deep - 0.05, z + up),
                    (bolt, y - deep + 0.02, z + up),
                    0.045,
                    RUST,
                    6,
                )
            bolt += 0.9


def _tablet(model, dice):
    """A dedication nobody can read, in steel letters at the height of someone on the steps."""
    z0 = PLINTH + 0.75
    y = _face_y(z0) - 0.05
    x0, x1 = -4.1, -0.45
    model.box("rust", (x0 - 0.12, y + 0.0, z0 - 0.12), (x1 + 0.12, y + 0.06, z0 + 1.32), DARK)
    for line in range(6):
        x = x0 + 0.05
        z = z0 + 1.08 - line * 0.22
        while x < x1 - 0.16:
            wide = dice.choice((0.05, 0.09, 0.12))
            if dice.random() < 0.15:
                x += wide + 0.1
                continue
            for _stroke in range(dice.randint(1, 3)):
                up = dice.uniform(0.0, 0.1)
                tall = dice.uniform(0.02, 0.14 - up)
                sx = x + dice.uniform(0.0, wide * 0.5)
                model.box(
                    "steel",
                    (sx, y - 0.015, z + up),
                    (sx + wide * dice.uniform(0.3, 0.6), y + 0.01, z + up + tall),
                    (0.5, 0.48, 0.42),
                )
            x += wide + 0.05


def _rebar(model, dice):
    centre, here = _centre(PLINTH + HIGH)
    for _ in range(46):
        x = centre.x + dice.uniform(-0.48, 0.48) * here[0]
        y = centre.y + dice.uniform(-0.45, 0.45) * here[1]
        z = PLINTH + HIGH - dice.uniform(1.5, 3.0)
        rise = dice.uniform(1.8, 4.3)
        bend = Vector((dice.uniform(-0.3, 0.3), dice.uniform(-0.3, 0.3), rise * 0.65))
        tip = bend + Vector((dice.uniform(-0.7, 0.7), dice.uniform(-0.7, 0.7), rise * 0.35))
        start = Vector((x, y, z))
        model.run("rust", [start, start + bend, start + tip], 0.024, RUST, 5)


def _shrine(model, dice):
    """What people still leave on the top step: tins of light, and offerings on a shelf."""
    z = PLINTH
    y = _face_y(z) - 0.42
    for _ in range(34):
        x = dice.uniform(-4.2, 4.3)
        at = y - dice.uniform(0.0, 0.5)
        tall = dice.uniform(0.09, 0.2)
        model.tube("rust", (x, at, z), (x, at, z + tall), 0.05, RUST, 7)
        model.tube("glow", (x, at, z + tall), (x, at, z + tall + 0.02), 0.045, EMBER, 7)
    # Conduit climbing to where the lamp on the top used to be.
    back = _centre(PLINTH)[1][1] / 2
    points = []
    for up in range(9):
        here = PLINTH + (HIGH - 3.0) * up / 8
        centre, size = _centre(here)
        points.append((centre.x - 2.0, centre.y + size[1] / 2 + 0.1 + back * 0.0, here))
    model.run("steel", points, 0.05, STEEL, 6)


def _grime(at, normal):
    """Darker low down and under the straps' run-off, in long vertical stains on the walls."""
    if abs(normal.z) > 0.6:
        return 0.8 + 0.2 * noise.noise(at * 0.7)
    stain = noise.noise(Vector((at.x * 1.7, at.y * 1.7, at.z * 0.07)))
    wet = 0.8 + 0.42 * stain
    foot = 0.72 + 0.28 * min((at.z - PLINTH) / 6.0, 1.0)
    return max(0.35, wet * foot)


def build():
    shapes.clear()
    dice = random.Random(11)
    model = shapes.Model("monolith")
    _plinth(model, dice)
    for lift, body in enumerate(_lifts(dice)):
        z0 = PLINTH + HIGH * lift / LIFTS
        z1 = z0 + HIGH / LIFTS
        carved = shapes.carve(body, _cuts(dice, lift, z0, z1))
        shapes.chamfer_creases(carved, 0.03)
        # A deep shadow gap between pours, which sells them as separate lifts.
        model.take(carved, "concrete", CONCRETE)
    _relief(model, dice)
    model.refine(0.55, ["concrete"])
    model.weather("concrete", 0.05, 0.012)
    model.finish(age=0.95, grime=_grime)
    shapes.export("monolith")


build()
