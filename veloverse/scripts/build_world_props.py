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


BUILDERS = {
    "bridge_pylon": build_bridge_pylon,
    "alpine_pavilion": build_alpine_pavilion,
    "wood_signpost": build_wood_signpost,
    "stone_milestone": build_stone_milestone,
    "desert_arch": build_desert_arch,
    "moor_cairn": build_moor_cairn,
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
