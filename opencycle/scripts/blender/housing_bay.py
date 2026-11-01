"""One storey of one bay of the housing round the plaza, in several lettings: a gallery with a
ribbed parapet, and a real room behind each window, its light baked in.

blender -b --python scripts/blender/housing_bay.py

Seven metres wide and a storey high. The origin is the middle of the bay at floor level on the
line of the building's face, and the gallery faces -Y in Blender, which is +Z in Godot.
"""

import math
import random
import sys
from pathlib import Path

from mathutils import Vector

sys.path.insert(0, str(Path(__file__).parent))
import shapes

WIDE = 7.0
HIGH = 4.2
SLAB = 0.28
PIER = 0.45
EDGE = WIDE / 2
INNER = EDGE - PIER
WALL_Y = 2.2
WALL_THICK = 0.3
ROOM_Y = 5.3
ROOM_HIGH = 3.45
PARAPET = -0.72
PARAPET_THICK = 0.2
PARAPET_HIGH = 1.2
SILL = 0.95
HEAD = 3.0
DOOR_HEAD = 2.55
CONCRETE = (0.6, 0.61, 0.63)
SOFFIT = (0.36, 0.37, 0.39)
STEEL = (0.26, 0.27, 0.28)
RUST = (0.55, 0.42, 0.34)
GLASS = (0.4, 0.48, 0.5)
WARM = (1.0, 0.66, 0.34)
COOL = (0.72, 0.9, 1.0)
PAINTS = (
    (0.6, 0.65, 0.5),
    (0.72, 0.62, 0.42),
    (0.45, 0.58, 0.6),
    (0.7, 0.5, 0.42),
    (0.62, 0.6, 0.56),
)
CURTAINS = ((0.62, 0.22, 0.12), (0.2, 0.3, 0.42), (0.6, 0.46, 0.2), (0.45, 0.45, 0.42))
LAUNDRY = (
    (0.55, 0.14, 0.12),
    (0.5, 0.5, 0.48),
    (0.13, 0.2, 0.34),
    (0.5, 0.42, 0.14),
    (0.2, 0.3, 0.26),
)
# Each letting: its door and windows left to right, which rooms are lit, and its seed.
LETTINGS = {
    "bay_a": (
        [("window", -2.9, -1.0), ("door", -0.65, 0.35), ("window", 0.75, 2.85)],
        (True, False),
    ),
    "bay_b": ([("door", -2.8, -1.8), ("window", -1.3, 2.75)], (True,)),
    "bay_c": (
        [("window", -2.85, -0.4), ("door", 0.15, 1.15), ("window", 1.55, 2.9)],
        (False, True),
    ),
    "bay_d": (
        [("door", -2.8, -1.8), ("window", -1.35, 0.4), ("window", 0.85, 2.8)],
        (False, False),
    ),
    "bay_e": (
        [("window", -2.9, -1.0), ("door", -0.65, 0.35), ("window", 0.75, 2.85)],
        (False, False),
    ),
    "bay_f": ([("door", -2.8, -1.8), ("window", -1.3, 2.75)], (False,)),
    "bay_g": ([("window", -2.85, -0.4), ("door", 0.15, 1.15), ("window", 1.55, 2.9)], (True, True)),
}


def _frame(model):
    """The slab, the piers either side and the parapet."""
    model.box("concrete", (-EDGE, -1.0, -SLAB), (EDGE, WALL_Y + 0.05, 0.0), SOFFIT, 0.0)
    # A downstand along the edge, with a drip cast into it.
    model.box("concrete", (-INNER, -1.0, -0.52), (INNER, -0.68, -SLAB), CONCRETE, 0.03)
    for side in (-1, 1):
        inner = side * INNER
        outer = side * EDGE
        low, high = sorted((inner, outer))
        model.box("concrete", (low, -1.25, -SLAB), (high, WALL_Y, HIGH - SLAB), CONCRETE, 0.04)
        # The pier's face stepped in, so that it holds a line of shadow.
        step = inner + side * -0.1
        low, high = sorted((inner, step))
        model.box("concrete", (low, -0.95, -SLAB), (high, WALL_Y, HIGH - SLAB), SOFFIT, 0.02)
    # The parapet: a precast panel with a sloped coping and vertical ribs on its outer face.
    model.box(
        "concrete",
        (-INNER, PARAPET, 0.0),
        (INNER, PARAPET + PARAPET_THICK, PARAPET_HIGH - 0.1),
        CONCRETE,
        0.015,
    )
    model.box(
        "concrete",
        (-INNER - 0.02, PARAPET - 0.08, PARAPET_HIGH - 0.12),
        (INNER + 0.02, PARAPET + PARAPET_THICK + 0.05, PARAPET_HIGH),
        CONCRETE,
        0.025,
    )
    x = -INNER + 0.1
    while x < INNER - 0.12:
        model.box(
            "concrete",
            (x, PARAPET - 0.055, 0.06),
            (x + 0.12, PARAPET + 0.005, PARAPET_HIGH - 0.16),
            CONCRETE,
            0.012,
        )
        x += 0.235


def _rail(model):
    y = PARAPET + PARAPET_THICK * 0.5
    z = PARAPET_HIGH
    top = z + 0.32
    model.tube("steel", (-INNER, y, top), (INNER, y, top), 0.022, STEEL, 6)
    x = -INNER + 0.3
    while x < INNER:
        model.bar("steel", (x, y, z), (x, y, top), 0.03, STEEL)
        x += 1.45


def _wall(model, openings):
    """The back wall of the gallery, in strips between its openings, with deep reveals."""
    x = -INNER
    top = HIGH - SLAB
    colour = [c * 0.82 for c in CONCRETE]
    y0, y1 = WALL_Y, WALL_Y + WALL_THICK
    for kind, left, right in openings:
        model.box("concrete", (x, y0, 0.0), (left, y1, top), colour)
        low = SILL if kind == "window" else 0.0
        high = HEAD if kind == "window" else DOOR_HEAD
        if low > 0.0:
            model.box("concrete", (left, y0, 0.0), (right, y1, low), colour)
        model.box("concrete", (left, y0, high), (right, y1, top), colour)
        x = right
    model.box("concrete", (x, y0, 0.0), (INNER, y1, top), colour)


def _window(model, left, right, lit, lit_by, dice, curtain):
    y = WALL_Y + 0.13
    # A sill that throws the rain off, and a lintel with a stain under it.
    model.box(
        "concrete",
        (left - 0.08, WALL_Y - 0.1, SILL - 0.1),
        (right + 0.08, WALL_Y + 0.1, SILL),
        CONCRETE,
        0.012,
    )
    frame = 0.055
    for x in (left + frame / 2, right - frame / 2):
        model.bar("steel", (x, y, SILL), (x, y, HEAD), frame, STEEL)
    for z in (SILL + frame / 2, HEAD - frame / 2):
        model.bar("steel", (left, y, z), (right, y, z), frame, STEEL)
    # Mullions, and a transom across the top third where a vent opens.
    panes = max(2, round((right - left) / 0.8))
    for pane in range(1, panes):
        x = left + (right - left) * pane / panes
        model.bar("steel", (x, y, SILL), (x, y, HEAD), 0.035, STEEL)
    transom = HEAD - 0.62
    model.bar("steel", (left, y, transom), (right, y, transom), 0.035, STEEL)
    model.box("glass", (left, y - 0.005, SILL), (right, y + 0.005, HEAD), GLASS)
    # Bars someone welded on, some years ago, on some windows.
    if dice.random() < 0.4:
        x = left + 0.15
        while x < right:
            model.bar("rust", (x, WALL_Y - 0.03, SILL), (x, WALL_Y - 0.03, HEAD), 0.018, RUST)
            x += 0.2
    # Curtains, part drawn, in folds. They're inside the room, so the lamp lights them.
    drawn = dice.uniform(0.25, 0.62)
    span = (right - left) * drawn
    ripple = _folds(dice)
    start = left if dice.random() < 0.5 else right - span
    model.sheet(
        "room",
        (start, WALL_Y + WALL_THICK + 0.06, HEAD + 0.1),
        (span, 0.0, 0.0),
        (0.0, 0.0, -(HEAD - SILL + 0.25)),
        (max(6, round(span / 0.08)), 5),
        curtain,
        ripple,
        lit_by,
    )


def _folds(dice):
    phase = dice.uniform(0.0, 6.0)
    count = dice.uniform(7.0, 12.0)

    def ripple(u, v):
        return Vector((0.0, math.sin(u * count * math.tau + phase) * (0.03 + 0.015 * v), 0.0))

    return ripple


def _door(model, left, right, dice):
    y = WALL_Y + 0.14
    colour = dice.choice(
        ((0.18, 0.28, 0.3), (0.34, 0.15, 0.11), (0.14, 0.16, 0.2), (0.3, 0.27, 0.15))
    )
    model.box("paint", (left + 0.02, y, 0.0), (right - 0.02, y + 0.05, DOOR_HEAD - 0.02), colour)
    # An old steel security gate in front of it.
    for x in (left + 0.03, right - 0.03):
        model.bar("steel", (x, y - 0.1, 0.0), (x, y - 0.1, DOOR_HEAD - 0.02), 0.04, STEEL)
    for z in (0.08, 1.05, 1.15, DOOR_HEAD - 0.08):
        model.bar("steel", (left + 0.03, y - 0.1, z), (right - 0.03, y - 0.1, z), 0.03, STEEL)
    x = left + 0.13
    while x < right - 0.1:
        model.bar("rust", (x, y - 0.1, 0.08), (x, y - 0.1, DOOR_HEAD - 0.08), 0.012, RUST)
        x += 0.1
    # A letterbox, a light over the door, a meter box beside it.
    model.box(
        "steel", (right + 0.12, WALL_Y - 0.14, 1.1), (right + 0.5, WALL_Y, 1.62), STEEL, 0.008
    )
    model.box(
        "glow",
        ((left + right) / 2 - 0.13, WALL_Y - 0.08, DOOR_HEAD + 0.1),
        ((left + right) / 2 + 0.13, WALL_Y, DOOR_HEAD + 0.2),
        (0.9, 0.62, 0.32) if dice.random() < 0.55 else (0.03, 0.03, 0.03),
    )


def _room(model, left, right, lit, dice):
    """A room behind an opening, with a lamp, a colour on the wall, and furniture to be seen."""
    y0 = WALL_Y + WALL_THICK
    y1 = ROOM_Y
    x0 = left - 0.7
    x1 = right + 0.7
    z1 = ROOM_HIGH
    colour = dice.choice(PAINTS)
    warm = dice.random() < 0.72
    tint = WARM if warm else COOL
    power = dice.uniform(1.3, 2.4) if lit else 0.05
    lit_by = model.lamp(((left + right) / 2, (y0 + y1) / 2 + 0.1, z1 - 0.5), tint, power)
    across = x1 - x0
    deep = y1 - y0
    grid = (max(4, round(across / 0.4)), 8)
    # Floor, ceiling, back and sides, all facing inwards.
    model.sheet(
        "room",
        (x0, y0, 0.02),
        (across, 0.0, 0.0),
        (0.0, deep, 0.0),
        (grid[0], 6),
        (0.2, 0.17, 0.15),
        None,
        lit_by,
    )
    model.sheet(
        "room",
        (x0, y1, z1),
        (across, 0.0, 0.0),
        (0.0, -deep, 0.0),
        (grid[0], 6),
        (0.62, 0.6, 0.55),
        None,
        lit_by,
    )
    model.sheet(
        "room", (x0, y1, 0.02), (across, 0.0, 0.0), (0.0, 0.0, z1), grid, colour, None, lit_by
    )
    model.sheet(
        "room", (x0, y0, 0.02), (0.0, deep, 0.0), (0.0, 0.0, z1), (6, 8), colour, None, lit_by
    )
    model.sheet(
        "room", (x1, y1, 0.02), (0.0, -deep, 0.0), (0.0, 0.0, z1), (6, 8), colour, None, lit_by
    )
    # Dark bulk against the wall: a wardrobe, a shelf and a table, whichever fit.
    furniture = dice.randint(2, 4)
    for _ in range(furniture):
        wide = dice.uniform(0.6, 1.5)
        x = dice.uniform(x0, x1 - wide)
        tall = dice.choice((0.75, 1.2, 1.9, 2.2))
        back = y1 - dice.uniform(0.02, 0.2)
        tone = dice.uniform(0.09, 0.26)
        model.box(
            "room",
            (x, back - 0.55, 0.02),
            (x + wide, back, tall),
            (tone, tone * 0.85, tone * 0.7),
            0.0,
            None,
            lit_by,
        )
    # A shade on a flex. It's most of what you notice from outside.
    lamp = model.lamps[lit_by - 1][0]
    if lit:
        glow = tuple(c * 3.2 for c in tint)
        model.box("glow", lamp - Vector((0.2, 0.2, 0.1)), lamp + Vector((0.2, 0.2, 0.1)), glow)
    return lit, lit_by


def _gallery(model, dice):
    """What people keep out here, because the flat has no room for it."""
    # Soffit conduit, and a bulkhead lamp.
    y = 1.6
    z = HIGH - SLAB - 0.05
    model.tube("steel", (-INNER, y, z), (INNER, y, z), 0.02, STEEL, 5)
    model.tube("steel", (-INNER, y + 0.09, z), (INNER, y + 0.09, z), 0.014, STEEL, 5)
    if dice.random() < 0.5:
        x = dice.uniform(-1.5, 1.5)
        model.box("steel", (x - 0.18, y - 0.12, z - 0.14), (x + 0.18, y + 0.12, z), STEEL)
        model.box(
            "glow", (x - 0.14, y - 0.09, z - 0.16), (x + 0.14, y + 0.09, z - 0.13), (2.4, 1.8, 1.1)
        )
    # A downpipe in the corner, with its brackets.
    side = dice.choice((-1, 1))
    x = side * (INNER - 0.22)
    model.tube("rust", (x, 2.02, -SLAB), (x, 2.02, HIGH - SLAB), 0.055, RUST, 8)
    for z in (0.9, 2.6):
        model.bar("steel", (x - 0.08, 2.02, z), (x + 0.08, 2.02, z), 0.03, STEEL)
    # A box on brackets humming to itself.
    if dice.random() < 0.72:
        _air_con(model, dice)
    if dice.random() < 0.68:
        _laundry(model, dice)
    if dice.random() < 0.5:
        _planters(model, dice)
    # Things stacked against the wall.
    for _ in range(dice.randint(1, 4)):
        x = dice.uniform(-INNER + 0.3, INNER - 0.9)
        wide = dice.uniform(0.3, 0.8)
        tall = dice.uniform(0.25, 0.95)
        tone = dice.uniform(0.14, 0.36)
        material = dice.choice(("worn", "rust", "paint"))
        model.box(
            material,
            (x, WALL_Y - 0.55, 0.0),
            (x + wide, WALL_Y - 0.03, tall),
            (tone, tone, tone * 0.9),
            0.01,
        )
    # Cable, sagging from pier to pier.
    points = []
    z = dice.uniform(2.9, 3.5)
    y = dice.uniform(1.75, 2.1)
    for i in range(9):
        t = i / 8
        points.append((-INNER + 2 * INNER * t, y, z - 0.28 * (1.0 - (2 * t - 1) ** 2)))
    model.run("steel", points, 0.012, (0.05, 0.05, 0.05), 5)


def _air_con(model, dice):
    x = dice.uniform(-2.3, 1.4)
    z = dice.uniform(0.05, 2.6)
    y1 = WALL_Y - 0.02
    y0 = y1 - 0.34
    colour = dice.choice(((0.42, 0.42, 0.38), (0.5, 0.48, 0.42), (0.34, 0.36, 0.34)))
    model.box("steel", (x, y0, z), (x + 0.86, y1, z + 0.62), colour, 0.012)
    # The louvres, as actual slats.
    slat = z + 0.06
    while slat < z + 0.56:
        model.box(
            "steel", (x + 0.05, y0 - 0.018, slat), (x + 0.81, y0 + 0.004, slat + 0.022), STEEL
        )
        slat += 0.05
    if z > 0.5:
        for across in (x + 0.08, x + 0.78):
            model.bar("rust", (across, y1, z - 0.04), (across, y0 - 0.04, z - 0.04), 0.03, RUST)
    model.run(
        "steel",
        [(x + 0.8, y1, z + 0.5), (x + 1.05, y1 - 0.02, z + 0.7), (x + 1.05, y1, z + 1.1)],
        0.012,
        (0.1, 0.1, 0.1),
        5,
    )


def _laundry(model, dice):
    z = dice.uniform(2.35, 2.75)
    y = dice.uniform(0.35, 0.8)
    x = -INNER + 0.1
    model.tube("steel", (-INNER, y, z), (INNER, y, z), 0.012, STEEL, 5)
    while x < INNER - 0.5:
        wide = dice.uniform(0.35, 0.8)
        long = dice.uniform(0.5, 1.15)
        if dice.random() < 0.7:
            phase = dice.uniform(0.0, 6.0)

            def ripple(u, v, phase=phase):
                return Vector((0.0, math.sin(u * 9.0 + phase) * 0.03 * (0.3 + v), 0.0))

            model.sheet(
                "cloth",
                (x, y, z),
                (wide, 0.0, 0.0),
                (0.0, 0.0, -long),
                (5, 4),
                dice.choice(LAUNDRY),
                ripple,
            )
        x += wide + dice.uniform(0.05, 0.3)


def _planters(model, dice):
    z = PARAPET_HIGH
    y0, y1 = PARAPET - 0.25, PARAPET - 0.02
    x = dice.uniform(-2.5, 0.8)
    wide = dice.uniform(0.7, 1.4)
    model.box("rust", (x, y0 - 0.02, z - 0.28), (x + wide, y1, z - 0.02), RUST, 0.006)
    for _ in range(dice.randint(14, 26)):
        at = Vector((x + dice.uniform(0.0, wide), dice.uniform(y0, y1), z - 0.03))
        turn = dice.uniform(0.0, math.tau)
        out = dice.uniform(0.08, 0.22)
        tip = at + Vector((math.cos(turn) * out, math.sin(turn) * out, dice.uniform(0.1, 0.42)))
        droop = tip + Vector((math.cos(turn) * out, math.sin(turn) * out, -dice.uniform(0.05, 0.2)))
        green = (dice.uniform(0.04, 0.09), dice.uniform(0.1, 0.2), dice.uniform(0.03, 0.06))
        model.run("cloth", [at, tip, droop], 0.012, green, 4)


def _letting(name, openings, lit):
    dice = random.Random(name)
    model = shapes.Model(name)
    _frame(model)
    _rail(model)
    _wall(model, openings)
    windows = [o for o in openings if o[0] == "window"]
    for index, (_, left, right) in enumerate(windows):
        _, lit_by = _room(model, left, right, lit[index], dice)
        _window(model, left, right, lit[index], lit_by, dice, dice.choice(CURTAINS))
    for kind, left, right in openings:
        if kind == "door":
            _door(model, left, right, dice)
    _gallery(model, dice)
    model.refine(0.7, ["concrete"])
    model.weather("concrete", 0.008, 0.002)
    model.finish(age=0.85)


def build():
    shapes.clear()
    for name, (openings, lit) in LETTINGS.items():
        _letting(name, openings, lit)
    shapes.export("housing_bay")


build()
