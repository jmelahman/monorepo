"""Builds handcrafted fantasy open-world roadside props & biome landmarks in Blender and
writes self-contained glTF 2.0 meshes (`feat_*.gltf`) into OC_SRC/world_props.

Run headless with Blender 5.x:
    blender -b -P scripts/build_world_props.py -- --out /path/to/dir [--render]
"""

import argparse
import math
import os
import sys

import bpy  # ty: ignore[unresolved-import]
from mathutils import Vector  # ty: ignore[unresolved-import]

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)

from build_harbour_kit import KitBuilder, clear_scene, make_mat  # noqa: E402


def build_bridge_pylon():
    """Monumental 5.8 m medieval stone bridgehead pylon with wrought-iron brazier lantern and
    heraldic swallowtail banner."""
    b = KitBuilder("bridge_pylon")

    # Deep footing + 3-stepped chamfered stone plinth
    b.box("stone", (0, 0, -0.7), (1.7, 1.7, 1.6), bevel=0.04)
    b.box("stone", (0, 0, 0.25), (1.55, 1.55, 0.50), bevel=0.05)
    b.box("stone", (0, 0, 0.65), (1.30, 1.30, 0.34), bevel=0.04)
    b.box("concrete", (0, 0, 0.88), (1.16, 1.16, 0.14), bevel=0.03)

    # Tapered ashlar pillar shaft (4 stacked stone tiers with subtle recessed bands)
    tiers = [
        (1.42, 0.96, 0.96),
        (2.38, 0.88, 0.96),
        (3.30, 0.82, 0.88),
        (4.12, 0.76, 0.78),
    ]
    for zc, w, h in tiers:
        b.box("stone", (0, 0, zc), (w, w, h), bevel=0.035)
        b.box("concrete", (0, 0, zc + h * 0.5), (w + 0.06, w + 0.06, 0.07), bevel=0.015)

    # Moulded overhanging cornice cap
    b.box("stone", (0, 0, 4.62), (1.06, 1.06, 0.22), bevel=0.04)
    b.box("slate", (0, 0, 4.80), (0.86, 0.86, 0.16), bevel=0.03)

    # Wrought-iron lantern housing with glowing warm glass core
    b.box("steel", (0, 0, 4.92), (0.62, 0.62, 0.08), bevel=0.015)
    for sx in (-0.23, 0.23):
        for sy in (-0.23, 0.23):
            b.cyl("steel", (sx, sy, 4.94), (sx, sy, 5.56), 0.025, 0.022, seg=8)
    b.box("warm_glass", (0, 0, 5.25), (0.36, 0.36, 0.52), bevel=0.02)
    # Pagoda-style copper lantern cap + finial spire
    b.cyl("copper", (0, 0, 5.55), (0, 0, 5.88), 0.44, 0.05, seg=4, smooth=False)
    b.cyl("steel", (0, 0, 5.86), (0, 0, 6.18), 0.03, 0.008, seg=8)

    # Heraldic banner hanging down the +X face (facing oncoming cyclists)
    bar_z = 3.88
    b.box("steel", (0.50, 0, bar_z), (0.28, 0.06, 0.06), bevel=0.01)
    b.cyl("steel", (0.58, -0.44, bar_z), (0.58, 0.44, bar_z), 0.025, 0.025, seg=8)
    for sy in (-0.45, 0.45):
        b.cyl(
            "copper", (0.58, sy, bar_z), (0.58, sy + (0.07 if sy > 0 else -0.07), bar_z), 0.04, 0.01
        )
    # Banner cloth (rich navy with crimson side trim & center stripe, cleanly separated)
    b.box("navy", (0.58, 0, 2.95), (0.028, 0.68, 1.82), bevel=0.004)
    for side_x in (0.562, 0.598):
        b.box("red", (side_x, 0, 2.95), (0.008, 0.14, 1.76))
        b.box("red", (side_x, -0.30, 2.95), (0.008, 0.055, 1.80))
        b.box("red", (side_x, 0.30, 2.95), (0.008, 0.055, 1.80))
    # Swallowtail tips at bottom
    b.box("navy", (0.58, -0.20, 1.92), (0.026, 0.24, 0.30))
    b.box("navy", (0.58, 0.20, 1.92), (0.026, 0.24, 0.30))
    return b


def build_alpine_pavilion():
    """Scenic Nordic-alpine timber rest shelter / wayside overlook with stone plinth, knee-braced
    heavy timber posts, pitched slate roof, oak bench, and warm hanging lantern."""
    b = KitBuilder("alpine_pavilion")
    w, d = 4.2, 2.8  # X = width along road, Y = depth back from road
    hx, hy = w * 0.5, d * 0.5

    # Buried stone foundation + stepped stone floor slab
    b.box("stone", (0, 0, -0.65), (w + 0.5, d + 0.5, 1.5), bevel=0.04)
    b.box("concrete", (0, 0, 0.18), (w + 0.36, d + 0.36, 0.16), bevel=0.03)
    # Stone step on the road-facing (-Y) front
    b.box("stone", (0, -hy - 0.32, 0.04), (2.2, 0.38, 0.22), bevel=0.03)

    # 4 chamfered heavy oak corner posts + stone column bases
    eave_z = 2.65
    for sx in (-hx + 0.22, hx - 0.22):
        for sy in (-hy + 0.22, hy - 0.22):
            b.box("stone", (sx, sy, 0.38), (0.34, 0.34, 0.28), bevel=0.03)
            b.box(
                "boards",
                (sx, sy, 0.52 + (eave_z - 0.52) * 0.5),
                (0.20, 0.20, eave_z - 0.52),
                bevel=0.02,
            )
            # Diagonal knee braces
            dx_in = -0.55 if sx > 0 else 0.55
            dy_in = -0.50 if sy > 0 else 0.50
            b.cyl(
                "boards",
                (sx, sy, eave_z - 0.62),
                (sx + dx_in, sy, eave_z - 0.08),
                0.065,
                0.065,
                seg=6,
                smooth=False,
            )
            b.cyl(
                "boards",
                (sx, sy, eave_z - 0.58),
                (sx, sy + dy_in, eave_z - 0.08),
                0.065,
                0.065,
                seg=6,
                smooth=False,
            )

    # Top timber tie beams around all 4 sides
    b.box("boards", (0, -hy + 0.22, eave_z), (w - 0.1, 0.20, 0.22), bevel=0.02)
    b.box("boards", (0, hy - 0.22, eave_z), (w - 0.1, 0.20, 0.22), bevel=0.02)
    b.box("boards", (-hx + 0.22, 0, eave_z), (0.20, d - 0.1, 0.22), bevel=0.02)
    b.box("boards", (hx - 0.22, 0, eave_z), (0.20, d - 0.1, 0.22), bevel=0.02)

    # Back & side timber balustrades (waist height)
    rail_z = 1.05
    b.box("wood", (0, hy - 0.22, rail_z), (w - 0.44, 0.12, 0.10), bevel=0.015)
    b.box("wood", (0, hy - 0.22, 0.58), (w - 0.44, 0.09, 0.08), bevel=0.01)
    for bx in (-1.35, -0.75, -0.15, 0.45, 1.05, 1.50):
        b.box("wood", (bx, hy - 0.22, 0.64), (0.07, 0.07, 0.78), bevel=0.01)
    for sx in (-hx + 0.22, hx - 0.22):
        b.box("wood", (sx, 0, rail_z), (0.12, d - 0.44, 0.10), bevel=0.015)
        b.box("wood", (sx, 0, 0.58), (0.09, d - 0.44, 0.08), bevel=0.01)

    # Built-in heavy oak resting bench along the back wall
    b.box("deck", (0, hy - 0.58, 0.68), (w - 0.90, 0.54, 0.09), bevel=0.015)
    for lx in (-1.3, 0.0, 1.3):
        b.box("boards", (lx, hy - 0.58, 0.44), (0.12, 0.46, 0.44), bevel=0.015)

    # Pitched dark-slate roof with overhanging eaves (ridge along X)
    ridge_z = eave_z + 1.35
    ov_x, ov_y = 0.48, 0.45
    slope_run = hy + ov_y
    slope_rise = ridge_z - (eave_z - 0.08)
    slope_len = math.hypot(slope_run, slope_rise)
    pitch_deg = math.degrees(math.atan2(slope_rise, slope_run))

    for sign in (-1.0, 1.0):
        yc = sign * slope_run * 0.5
        zc = (eave_z - 0.08) + slope_rise * 0.5 + 0.08
        b.box(
            "slate",
            (0, yc, zc),
            (w + ov_x * 2, slope_len + 0.10, 0.16),
            rot=(-sign * pitch_deg, 0, 0),
            bevel=0.02,
        )
        # Exposed timber rafters underneath
        for rx in (-hx - 0.15, -hx * 0.5, 0.0, hx * 0.5, hx + 0.15):
            b.box(
                "boards",
                (rx, yc, zc - 0.11),
                (0.11, slope_len - 0.05, 0.12),
                rot=(-sign * pitch_deg, 0, 0),
            )

    # Carved ridge beam + copper ridge cap + timber gable infill triangles
    b.box("boards", (0, 0, ridge_z - 0.05), (w + ov_x * 2 + 0.25, 0.16, 0.22), bevel=0.02)
    b.box("copper", (0, 0, ridge_z + 0.11), (w + ov_x * 2 + 0.10, 0.22, 0.10), bevel=0.02)
    for sx in (-hx + 0.22, hx - 0.22):
        # Horizontal gable timber planks stepping up into the triangle
        for k in range(5):
            frac = (k + 0.5) / 5.0
            gz = eave_z + 0.10 + frac * (ridge_z - eave_z - 0.18)
            gw = (1.0 - frac) * (d - 0.50)
            if gw > 0.18:
                b.box("siding", (sx, 0, gz), (0.14, gw, 0.22), bevel=0.01)

    # Hanging wrought-iron lantern centered under the front tie beam
    b.cyl(
        "steel", (0, -hy + 0.22, eave_z - 0.10), (0, -hy + 0.22, eave_z - 0.45), 0.015, 0.015, seg=6
    )
    b.box("steel", (0, -hy + 0.22, eave_z - 0.48), (0.28, 0.28, 0.05))
    b.box("warm_glass", (0, -hy + 0.22, eave_z - 0.66), (0.20, 0.20, 0.30), bevel=0.015)
    b.box("steel", (0, -hy + 0.22, eave_z - 0.83), (0.26, 0.26, 0.05))
    return b


def build_wood_signpost():
    """Nordic-fantasy roadside timber fingerpost with stone collar, 3 pointed arrow boards,
    and a hanging warm iron lantern."""
    b = KitBuilder("wood_signpost")

    # Stone collar base (with buried footing)
    b.cyl("stone", (0, 0, -0.55), (0, 0, 0.34), 0.44, 0.34, seg=8, smooth=False)
    b.cyl("concrete", (0, 0, 0.34), (0, 0, 0.42), 0.36, 0.28, seg=8, smooth=False)

    # Heavy chamfered oak post + tapered cap
    b.box("boards", (0, 0, 1.55), (0.19, 0.19, 2.95), bevel=0.025)
    b.cyl("copper", (0, 0, 3.02), (0, 0, 3.22), 0.16, 0.02, seg=4, smooth=False)

    # 3 pointed wooden arrow signboards at varied heights & yaws
    arrows = [
        (2.48, 0.0, 0.92, "deck"),
        (2.14, 155.0, 0.86, "wood"),
        (1.80, -42.0, 0.80, "deck"),
    ]
    for z, yaw_deg, length, mat in arrows:
        rad = math.radians(yaw_deg)
        ux, uy = math.cos(rad), math.sin(rad)
        # Board body extending outward from post
        cx, cy = ux * (length * 0.45), uy * (length * 0.45)
        b.box(mat, (cx, cy, z), (length * 0.82, 0.055, 0.22), rot=(0, 0, yaw_deg), bevel=0.01)
        # Pointed arrow tip
        tx, ty = ux * (length * 0.88), uy * (length * 0.88)
        b.box(mat, (tx, ty, z), (0.17, 0.052, 0.17), rot=(0, 45, yaw_deg), bevel=0.008)
        # Iron mounting strap collar around post
        b.box("steel", (0, 0, z), (0.22, 0.22, 0.05), rot=(0, 0, yaw_deg))

    # Wrought-iron lantern bracket + hanging glowing lantern at top
    b.box("steel", (0.28, 0, 2.86), (0.56, 0.045, 0.05), bevel=0.008)
    b.cyl("steel", (0.08, 0, 2.52), (0.44, 0, 2.85), 0.02, 0.02, seg=6)
    b.cyl("steel", (0.52, 0, 2.84), (0.52, 0, 2.68), 0.014, 0.014, seg=6)
    b.cyl("copper", (0.52, 0, 2.68), (0.52, 0, 2.58), 0.04, 0.15, seg=6, smooth=False)
    b.box("warm_glass", (0.52, 0, 2.43), (0.17, 0.17, 0.26), bevel=0.015)
    b.box("steel", (0.52, 0, 2.28), (0.21, 0.21, 0.04))
    return b


def build_stone_milestone():
    """Carved granite roadside km-stone / waymarker (1.2 m tall) with stepped plinth, chamfered
    obelisk shaft, recessed inscription tablet, and weathered bronze cap band."""
    b = KitBuilder("stone_milestone")
    b.box("stone", (0, 0, -0.35), (0.72, 0.56, 0.80), bevel=0.04)
    b.box("stone", (0, 0, 0.14), (0.64, 0.48, 0.22), bevel=0.04)
    b.box("stone", (0, 0, 0.62), (0.48, 0.34, 0.78), bevel=0.045)
    # Rounded/chamfered stone dome cap
    b.box("stone", (0, 0, 1.08), (0.40, 0.28, 0.18), bevel=0.05)
    # Recessed lighter carved stone inscription plaque on road-facing side
    b.box("plaster", (0.17, 0, 0.70), (0.04, 0.26, 0.38), bevel=0.01)
    # Iron/copper collar ring
    b.box("steel", (0, 0, 0.32), (0.51, 0.37, 0.045), bevel=0.008)
    return b


def build_desert_arch():
    """Monumental Red Sands sun-bleached sandstone ruin obelisk/archway (5.8 m tall) with
    stepped plinth, twin carved pillars, and terracotta frieze."""
    b = KitBuilder("desert_arch")
    span = 2.8
    # Stepped sandstone plinths
    for sx in (-span * 0.5, span * 0.5):
        b.box("ochre", (sx, 0, -0.75), (1.55, 1.55, 1.65), bevel=0.04)
        b.box("ochre", (sx, 0, 0.22), (1.40, 1.40, 0.36), bevel=0.04)
        # Fluted/stepped sandstone column shaft
        b.box("ochre", (sx, 0, 1.65), (1.02, 1.02, 2.55), bevel=0.04)
        b.box("clay", (sx, 0, 2.98), (1.10, 1.10, 0.18), bevel=0.02)
        b.box("ochre", (sx, 0, 3.85), (0.94, 0.94, 1.58), bevel=0.04)
        # Carved capital
        b.box("ochre", (sx, 0, 4.78), (1.28, 1.28, 0.30), bevel=0.04)

    # Massive carved sandstone lintel beam + stepped desert cornice + terracotta frieze
    b.box("ochre", (0, 0, 5.18), (span + 1.85, 1.24, 0.54), bevel=0.04)
    b.box("clay", (0, 0, 5.52), (span + 1.55, 1.10, 0.18), bevel=0.02)
    b.box("ochre", (0, 0, 5.82), (span + 2.05, 1.42, 0.42), bevel=0.05)
    # Weathered ruin break — stepped crenellation blocks on top
    for bx, bh in ((-1.7, 0.48), (-0.6, 0.32), (0.7, 0.52), (1.8, 0.38)):
        b.box("ochre", (bx, 0, 6.02 + bh * 0.45), (0.68, 0.88, bh), bevel=0.04)
    return b


def build_moor_cairn():
    """Highland weathered standing-stone cairn / ancient dolmen waymarker (2.8 m tall) for
    The Moor heath and Vael ridges."""
    b = KitBuilder("moor_cairn")
    # Clustered irregular stone mound base (deep footing + angled boulders)
    b.cyl("stone", (0.0, 0.0, -0.85), (0.0, 0.0, 0.22), 1.35, 1.05, seg=7, smooth=False)
    stones = [
        ((-0.55, 0.32, 0.32), (1.15, 0.95, 0.65), (4, -6, 28)),
        ((0.52, -0.24, 0.28), (1.20, 0.88, 0.58), (-5, 7, -18)),
        ((0.05, 0.48, 0.42), (0.95, 0.82, 0.55), (6, 3, 45)),
        ((-0.18, -0.42, 0.38), (1.02, 0.78, 0.52), (-4, -5, -34)),
    ]
    for pos, sz, rot in stones:
        b.box("stone", pos, sz, rot=rot, bevel=0.09)
    # Tall leaning sentinel monolith + companion standing stone
    b.box("stone", (-0.12, 0.04, 1.52), (0.68, 0.44, 2.25), rot=(4, -6, 14), bevel=0.08)
    b.box("stone", (0.56, 0.16, 1.08), (0.50, 0.38, 1.38), rot=(-6, 11, -22), bevel=0.07)
    # Stacked flat capstone + small summit marker stone
    b.box("concrete", (-0.08, 0.02, 2.55), (0.48, 0.36, 0.24), rot=(2, -3, 25), bevel=0.05)
    return b


def build_moor_standing_stones():
    """Ancient weathered stone circle (~7 m diameter) of 5 tall tilted irregular monoliths
    (2.6-4.2 m tall, tapered, slightly leaning) around a low flat central altar slab."""
    b = KitBuilder("moor_standing_stones")

    # Central low flat weathered altar slab + buried footing + rough base plinth
    b.box("stone", (0.0, 0.0, -0.45), (1.95, 1.35, 1.0), bevel=0.05)
    b.box("stone", (0.0, 0.0, 0.22), (1.85, 1.25, 0.36), rot=(0.0, 0.0, 12.0), bevel=0.06)
    b.box("concrete", (0.0, 0.0, 0.50), (2.22, 1.48, 0.24), rot=(1.5, -1.0, 12.0), bevel=0.045)
    # Small flanking offering/hearth stones beside altar
    b.box("stone", (-1.18, -0.48, 0.14), (0.58, 0.44, 0.30), rot=(4.0, -5.0, 32.0), bevel=0.06)
    b.box("stone", (1.12, 0.52, 0.12), (0.52, 0.40, 0.26), rot=(-5.0, 6.0, -24.0), bevel=0.05)

    # 5 tall irregular tapered monoliths arranged on ~3.5 m radius (~7 m diameter)
    monoliths = [
        # (angle_deg, radius, height, base_w, base_d, lean_radial_deg, lean_tan_deg)
        (14.0, 3.45, 4.20, 0.96, 0.62, -6.5, 4.0),
        (86.0, 3.30, 3.05, 0.84, 0.56, 7.5, -5.0),
        (158.0, 3.55, 3.85, 0.92, 0.60, -5.0, -6.5),
        (228.0, 3.35, 2.65, 0.80, 0.52, 8.5, 4.5),
        (300.0, 3.50, 3.45, 0.88, 0.58, -7.0, 5.5),
    ]
    for ang_deg, rad, height, bw, bd, lean_r, lean_t in monoliths:
        ang = math.radians(ang_deg)
        ux, uy = math.cos(ang), math.sin(ang)
        cx, cy = ux * rad, uy * rad
        yaw_deg = ang_deg + 18.0

        # Deep buried footing + basal cairn boulders
        b.cyl("stone", (cx, cy, -0.95), (cx, cy, 0.18), bw * 0.78, bw * 0.64, seg=7, smooth=False)
        for off_a, off_r, sz, dz in (
            (0.6, 0.52, (0.72, 0.58, 0.42), 0.12),
            (-0.8, 0.48, (0.64, 0.52, 0.36), 0.10),
        ):
            bx = cx + math.cos(ang + off_a) * off_r
            by = cy + math.sin(ang + off_a) * off_r
            b.box("stone", (bx, by, dz), sz, rot=(4.0, -5.0, ang_deg + off_a * 30.0), bevel=0.07)

        # Tapered irregular monolith shaft built from leaning core + cleft side slabs
        tilt_rad = math.radians(lean_r)
        tan_rad = math.radians(lean_t)
        dx_top = height * math.sin(tilt_rad) * ux - height * math.sin(tan_rad) * uy
        dy_top = height * math.sin(tilt_rad) * uy + height * math.sin(tan_rad) * ux
        b.cyl(
            "stone",
            (cx, cy, -0.20),
            (cx + dx_top, cy + dy_top, height),
            bw * 0.56,
            bw * 0.24,
            seg=6,
            smooth=False,
        )
        # Overlapping angular stone face shards for rugged cleft profile
        rot_tuple = (lean_t, -lean_r, yaw_deg)
        b.box(
            "stone",
            (cx + dx_top * 0.36, cy + dy_top * 0.36, height * 0.38),
            (bw * 0.86, bd * 0.82, height * 0.74),
            rot=rot_tuple,
            bevel=0.07,
        )
        b.box(
            "stone",
            (cx + dx_top * 0.68, cy + dy_top * 0.68, height * 0.72),
            (bw * 0.58, bd * 0.58, height * 0.48),
            rot=(lean_t + 3.0, -lean_r - 2.5, yaw_deg + 16.0),
            bevel=0.06,
        )
    return b


def build_vael_shrine():
    """Weathered Nordic-alpine stone wayside shrine (~2.4 m wide, 3.6 m tall) with stepped
    stone plinth, twin stone pillars, pitched timber/slate roof gable, and warm recessed lantern."""
    b = KitBuilder("vael_shrine")
    w, d = 2.40, 1.56

    # Buried stone foundation + 2-stepped stone plinth + altar sill
    b.box("stone", (0.0, 0.0, -0.52), (w + 0.16, d + 0.16, 1.15), bevel=0.04)
    b.box("stone", (0.0, 0.0, 0.14), (w, d, 0.28), bevel=0.035)
    b.box("stone", (0.0, 0.0, 0.39), (w - 0.32, d - 0.26, 0.24), bevel=0.03)
    b.box("concrete", (0.0, -0.04, 0.56), (w - 0.48, d - 0.38, 0.14), bevel=0.025)

    # Back stone niche wall framing the glowing alcove (open toward -Y road side)
    b.box("stone", (0.0, 0.32, 1.48), (1.76, 0.42, 1.72), bevel=0.035)
    # Recessed dark backing slab inside alcove
    b.box("slate", (0.0, 0.11, 1.44), (0.94, 0.08, 1.38), bevel=0.015)

    # Twin carved stone pillars with stepped bases & capitals
    pillar_y = -0.26
    for sx in (-0.70, 0.70):
        b.box("stone", (sx, pillar_y, 0.73), (0.38, 0.38, 0.20), bevel=0.025)
        b.box("stone", (sx, pillar_y, 1.49), (0.28, 0.28, 1.36), bevel=0.03)
        b.box("stone", (sx, pillar_y, 2.24), (0.38, 0.38, 0.18), bevel=0.025)
        # Side stone wing walls tying pillars to back wall
        b.box("stone", (sx, 0.04, 1.48), (0.24, 0.52, 1.66), bevel=0.025)

    # Heavy carved stone lintel & cornice beam spanning the shrine
    b.box("stone", (0.0, 0.02, 2.45), (2.06, 0.98, 0.26), bevel=0.035)

    # Recessed warm glowing shrine lantern on stone pedestal inside alcove
    lx, ly = 0.0, -0.02
    b.box("stone", (lx, ly, 0.78), (0.42, 0.36, 0.32), bevel=0.025)
    b.box("steel", (lx, ly, 0.97), (0.32, 0.32, 0.05), bevel=0.01)
    for px in (-0.12, 0.12):
        for py in (-0.12, 0.12):
            b.cyl("steel", (lx + px, ly + py, 0.99), (lx + px, ly + py, 1.44), 0.015, 0.014, seg=6)
    b.box("warm_glass", (lx, ly, 1.21), (0.21, 0.21, 0.38), bevel=0.015)
    b.cyl("copper", (lx, ly, 1.43), (lx, ly, 1.64), 0.22, 0.03, seg=4, smooth=False)
    b.cyl("steel", (lx, ly, 1.63), (lx, ly, 1.76), 0.02, 0.008, seg=6)

    # Pitched timber & slate gable roof (ridge running front-to-back along Y, peaking ~3.60 m)
    eave_z = 2.58
    roof_w = 2.02
    roof_d = 1.18
    pitch_deg = 40.0
    pitch = math.radians(pitch_deg)
    rise = (roof_w * 0.5) * math.tan(pitch)
    ridge_z = eave_z + rise
    over_x = 0.24
    over_y = 0.26
    span_x = roof_w * 0.5 + over_x
    slope_len = span_x / math.cos(pitch)
    full_y = roof_d + 2.0 * over_y

    # Timber gable infill
    b.prism_gable("siding", 0.0, 0.02, eave_z, roof_w - 0.08, roof_d - 0.08, rise - 0.04, axis="Y")
    b.box("boards", (0.0, 0.02, eave_z + 0.04), (roof_w + 0.10, roof_d + 0.10, 0.12), bevel=0.015)

    for sx in (-1.0, 1.0):
        mid_x = sx * (span_x * 0.5)
        mid_z = ridge_z - (span_x * 0.5) * math.tan(pitch) + 0.08 / math.cos(pitch)
        b.box(
            "slate",
            (mid_x, 0.02, mid_z),
            (slope_len, full_y, 0.14),
            rot=(0.0, sx * pitch_deg, 0.0),
            bevel=0.02,
        )
        # Exposed timber bargeboards on front & back gable ends
        for sy in (-1.0, 1.0):
            by = 0.02 + sy * (full_y * 0.5 - 0.03)
            b.box(
                "boards",
                (mid_x, by, mid_z - 0.04),
                (slope_len + 0.08, 0.08, 0.18),
                rot=(0.0, sx * pitch_deg, 0.0),
                bevel=0.01,
            )

    # Carved timber ridge beam, copper ridge cap & summit finial
    b.box("boards", (0.0, 0.02, ridge_z + 0.02), (0.16, full_y + 0.12, 0.18), bevel=0.015)
    b.box("copper", (0.0, 0.02, ridge_z + 0.14), (0.20, full_y + 0.06, 0.08), bevel=0.015)
    b.cyl(
        "steel",
        (0.0, -full_y * 0.42, ridge_z + 0.16),
        (0.0, -full_y * 0.42, 3.68),
        0.022,
        0.01,
        seg=6,
    )
    return b


BUILDERS = {
    "bridge_pylon": build_bridge_pylon,
    "alpine_pavilion": build_alpine_pavilion,
    "wood_signpost": build_wood_signpost,
    "stone_milestone": build_stone_milestone,
    "desert_arch": build_desert_arch,
    "moor_cairn": build_moor_cairn,
    "moor_standing_stones": build_moor_standing_stones,
    "vael_shrine": build_vael_shrine,
}


def render_preview(out_dir, built):
    clear_scene()
    scene = bpy.context.scene
    scene.render.engine = "CYCLES"
    scene.cycles.device = "CPU"
    scene.cycles.samples = 24
    scene.cycles.use_denoising = True
    scene.render.resolution_x = 1600
    scene.render.resolution_y = 900

    layout = [
        ("bridge_pylon", (-8.5, 0, 0), -25),
        ("alpine_pavilion", (-3.5, 0, 0), -15),
        ("wood_signpost", (1.2, -1.2, 0), -20),
        ("stone_milestone", (3.2, -1.6, 0), -25),
        ("moor_cairn", (5.8, 0, 0), -10),
        ("desert_arch", (10.2, 0.6, 0), -18),
    ]
    for name, loc, yaw in layout:
        ob = built[name].to_blender_object()
        ob.location = loc
        ob.rotation_euler = (0, 0, math.radians(yaw))

    bpy.ops.mesh.primitive_plane_add(size=80, location=(1, 0, 0))
    ground = bpy.context.active_object
    ground.data.materials.append(make_mat("ground_prev", (0.16, 0.20, 0.17), 0.85))

    world = bpy.data.worlds.new("PropWorld")
    scene.world = world
    if world.node_tree:
        bg = world.node_tree.nodes.get("Background")
        if bg:
            bg.inputs["Color"].default_value = (0.35, 0.48, 0.68, 1.0)
            bg.inputs["Strength"].default_value = 0.75
    sun_data = bpy.data.lights.new("Sun", type="SUN")
    sun_data.energy = 3.8
    sun_obj = bpy.data.objects.new("Sun", sun_data)
    scene.collection.objects.link(sun_obj)
    sun_obj.rotation_euler = (math.radians(48), math.radians(14), math.radians(-32))

    cam_data = bpy.data.cameras.new("Cam")
    cam_data.lens = 38.0
    cam_obj = bpy.data.objects.new("Cam", cam_data)
    scene.collection.objects.link(cam_obj)
    cam_obj.location = (1.0, -24.0, 6.2)
    direction = Vector((1.0, 0.0, 2.3)) - cam_obj.location
    cam_obj.rotation_euler = direction.to_track_quat("-Z", "Y").to_euler()
    scene.camera = cam_obj

    preview_path = os.path.join(out_dir, "world_props_preview.png")
    scene.render.filepath = preview_path
    bpy.ops.render.render(write_still=True)
    print(f"Rendered preview: {preview_path}", flush=True)


def main():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--out", default="/home/jamison/.cache/opencycle/world_gen/ride_hub/world_props"
    )
    ap.add_argument("--render", action="store_true")
    args = ap.parse_args(argv)
    os.makedirs(args.out, exist_ok=True)

    built = {}
    for name, fn in BUILDERS.items():
        kb = fn()
        out_path = os.path.join(args.out, f"feat_{name}.gltf")
        kb.write_gltf(out_path)
        print(
            f"world_props: {name:16s} {len(kb.bm.verts):5d}v {len(kb.bm.faces):5d}f -> {out_path}",
            flush=True,
        )
        built[name] = kb

    if args.render:
        render_preview(args.out, built)
    for kb in built.values():
        kb.bm.free()


if __name__ == "__main__":
    main()
