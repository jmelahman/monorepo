"""Builds /Game/Coast/L_Coast: a bike path through a small Japanese town, across the rice paddies,
into the jungle, and out along a cliff road above the sea under the mountains.

The geography is coast_layout's; the meshes come from coast_assets.py (Poly Haven models, terrain
tiles, sea) and from make_paths.py and make_houses.py (path, poles, houses, furniture), which this
imports first if they aren't in the project yet. Everything is placed from fixed seeds.

Settings:
    OC_SUN       "elevation,azimuth" degrees (default 34,-62: mid-afternoon, from the south-west)
    OC_SUN_LUX   sun illuminance (default 90000)
    OC_FOG       fog density (default 0.006)
    OC_DENSITY   scales every scatter (default 1)
    OC_REIMPORT  "1" re-imports the generated path and kit meshes
Run: bash scripts/ue_run.sh make_coast.py make_coast.log
"""

import glob
import importlib.util
import json
import math
import os
import random
import re

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/make_coast.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
layout = sibling("coast_layout")
log, Graph, assets, spawn = lib.log, lib.Graph, lib.assets, lib.spawn
PATH = layout.CoastPath()
LEVEL = "/Game/Coast/L_Coast"
MATS = f"{lib.FOLDER}/Materials"
GEN = "/workspace/ue/assets_gen"
DENSITY = float(os.environ.get("OC_DENSITY") or "1")
RAIN = os.environ.get("OC_DRY") != "1" and layout.raining()
REIMPORT = os.environ.get("OC_REIMPORT") == "1"


# --- materials ------------------------------------------------------------------------------


def mesh_uv(g, tile):
    """The mesh's own UVs (1 per metre from the generators) scaled to repeat every `tile` cm."""
    coords = g.node(
        unreal.MaterialExpressionTextureCoordinate, u_tiling=100.0 / tile, v_tiling=100.0 / tile
    )
    return coords


def vertex_rgb(g):
    return g.mask(g.node(unreal.MaterialExpressionVertexColor), "rgb")


def desaturate(g, colour):
    node = g.node(unreal.MaterialExpressionDesaturation)
    g.link(colour, node, "")
    return node


def ramp(g, value, a, b):
    """0 below a, 1 above b, linear between."""
    return g.saturate(g.mul(g.add(value, -a), 1.0 / (b - a)))


def ride_collection():
    """MPC_Ride: the rain falling and the wet on the ground where the rider is, 0 to 1. RidePawn
    sets them as it rides and the stills' sequence keys them, from the same weather rows, so any
    number of showers can fall anywhere on the loop. Both are 0 in the editor."""
    path = f"{MATS}/MPC_Ride"
    if assets.does_asset_exist(path):
        return unreal.load_asset(path)
    mpc = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
        "MPC_Ride",
        MATS,
        unreal.MaterialParameterCollection,
        unreal.MaterialParameterCollectionFactoryNew(),
    )
    params = []
    for name in ("Rain", "Wet"):
        param = unreal.CollectionScalarParameter()
        param.set_editor_property("parameter_name", name)
        param.set_editor_property("default_value", 0.0)
        params.append(param)
    mpc.set_editor_property("scalar_parameters", params)
    assets.save_loaded_asset(mpc, only_if_is_dirty=False)
    return mpc


def ride_value(g, name):
    """MPC_Ride's `name` in graph `g`."""
    return g.node(
        unreal.MaterialExpressionCollectionParameter,
        collection=ride_collection(),
        parameter_name=name,
    )


def wet_band(g):
    """How wet the world is, 0 to 1, from where the rider is on the ride."""
    return ride_value(g, "Wet") if RAIN else None


def wetten(g, colour, rough, puddles=False, upward=False, wet_rough=0.14):
    """Darkens and polishes a surface where it is wet, to `wet_rough`; flat ground gathers
    puddles."""
    wet = wet_band(g)
    if wet is None:
        return colour, rough
    if upward:
        # Rain wets what faces the sky; walls under eaves stay drier.
        nz = g.mask(g.node(unreal.MaterialExpressionVertexNormalWS), "b")
        wet = g.mul(wet, g.saturate(g.add(g.mul(nz, 0.7), 0.4)))
    colour = g.mul(colour, g.lerp(g.const(1.0), g.const(0.5), wet))
    rough = g.lerp(rough, g.const(wet_rough), g.mul(wet, 0.85))
    if puddles:
        pool = g.saturate(g.mul(g.add(g.world_noise(0.0035, 0.0, 1.0, 3), -0.55), 6.0))
        pool = g.mul(pool, wet)
        rough = g.lerp(rough, g.const(0.02), pool)
        colour = g.mul(colour, g.lerp(g.const(1.0), g.const(0.55), pool))
    return colour, rough


# --- wear: nothing in a real village is new --------------------------------------------------


def _noise(g, position, scale, low=0.0, high=1.0, levels=3):
    noise = g.node(
        unreal.MaterialExpressionNoise,
        scale=scale,
        quality=2,
        levels=levels,
        output_min=low,
        output_max=high,
    )
    g.link(position, noise, "World Position")
    return noise


def _height_above_pivot(g):
    """cm above the mesh's own pivot (its foot, for our buildings, posts and rails)."""
    cls = getattr(unreal, "MaterialExpressionLocalPosition", None)
    if cls is not None:
        return g.mask(g.node(cls), "b")
    world = g.mask(g.node(unreal.MaterialExpressionWorldPosition), "b")
    return g.add(world, g.mul(g.mask(g.node(unreal.MaterialExpressionObjectPositionWS), "b"), -1.0))


def wear(
    g,
    colour,
    rough,
    grime=0.0,
    streaks=0.0,
    splash=0.0,
    moss=0.0,
    rust=0.0,
    patches=0.0,
    chunked=False,
):
    """Age a surface: `grime` blotches and per-object fading, `streaks` of rain run down from
    ledges,
    `splash` cm of dirt kicked up at the foot, `moss` on what faces up and stays damp, `rust`
    bleeding
    from metal. Each is 0..1 (splash in cm), and every object gets its own share from its
    position."""
    if not (grime or streaks or splash or moss or rust or patches):
        return colour, rough
    world = g.node(unreal.MaterialExpressionWorldPosition)
    # 0..1 per object: an instance's random number, and a hash of where the object stands.
    # `chunked`: meshes cut into chunks along the path (road, kerbs, rail) vary over the world
    # instead,
    # or each chunk would change its grime at the seam.
    if chunked:
        vary = _noise(g, world, 0.0012, 0.0, 1.0, 2)
    else:
        per_instance = g.node(unreal.MaterialExpressionPerInstanceRandom)
        where = _noise(g, g.node(unreal.MaterialExpressionObjectPositionWS), 0.013, 0.0, 1.0, 1)
        vary = g.saturate(g.add(g.mul(per_instance, 0.6), g.mul(where, 0.7)))
    blot = _noise(g, world, 0.004, 0.0, 1.0, 3)
    fine = _noise(g, world, 0.03, 0.0, 1.0, 2)
    up = g.saturate(g.mask(g.node(unreal.MaterialExpressionVertexNormalWS), "b"))
    if grime:
        dirt = g.saturate(
            g.mul(g.add(g.add(g.mul(blot, 0.8), g.mul(vary, 0.6)), g.mul(fine, 0.25)), grime * 0.9)
        )
        colour = g.lerp(colour, g.mul(colour, g.const((0.6, 0.58, 0.54))), dirt)
        rough = g.add(rough, g.mul(dirt, 0.12))
    if streaks or rust:
        # Rain marks: noise squashed tall, so its features run down the wall.
        tall = g.mul(world, g.const((1.0, 1.0, 0.07)))
        run = _noise(g, tall, 0.05, 0.0, 1.0, 3)
        side = g.link(up, g.node(unreal.MaterialExpressionOneMinus))
    if streaks:
        mark = g.saturate(g.mul(g.add(run, -0.52), 4.0))
        mark = g.mul(g.mul(mark, side), g.mul(g.add(vary, 0.4), streaks))
        colour = g.lerp(colour, g.mul(colour, g.const((0.42, 0.40, 0.37))), g.saturate(mark))
    if splash:
        foot = g.link(
            ramp(g, _height_above_pivot(g), 0.0, splash), g.node(unreal.MaterialExpressionOneMinus)
        )
        foot = g.mul(foot, g.add(g.mul(fine, 0.5), 0.4))
        colour = g.lerp(colour, g.mul(colour, g.const((0.45, 0.38, 0.3))), g.saturate(foot))
    if moss:
        damp = g.saturate(
            g.mul(
                g.add(g.add(g.mul(blot, 0.9), g.mul(up, 0.6)), g.add(g.mul(fine, 0.4), -0.95)), 2.5
            )
        )
        damp = g.mul(damp, moss)
        colour = g.lerp(colour, g.const((0.07, 0.09, 0.035)), damp)
        rough = g.lerp(rough, g.const(0.9), damp)
    if rust:
        # Patches, not runs: squashed noise on a long horizontal rail tiles into chevrons.
        bleed = g.saturate(
            g.mul(
                g.add(g.add(g.mul(blot, 0.7), g.mul(fine, 0.6)), g.add(g.mul(vary, 0.3), -1.0)), 3.0
            )
        )
        bleed = g.mul(bleed, rust)
        colour = g.lerp(colour, g.const((0.2, 0.08, 0.03)), bleed)
        rough = g.lerp(rough, g.const(0.85), bleed)
    if patches:
        # Road repairs: hard-edged blobs of darker, smoother tar, and cracks where noise crosses
        # its middle.
        blob = _noise(g, world, 0.0025, 0.0, 1.0, 2)
        tar = g.saturate(g.mul(g.add(g.add(blob, g.mul(fine, 0.08)), -0.66), 30.0))
        tar = g.mul(tar, patches)
        colour = g.lerp(colour, g.mul(colour, g.const((0.55, 0.55, 0.56))), tar)
        rough = g.lerp(rough, g.mul(rough, 0.75), tar)
        vein = _noise(g, world, 0.012, -1.0, 1.0, 3)
        crack = g.saturate(
            g.add(g.mul(g.link(vein, g.node(unreal.MaterialExpressionAbs)), -25.0), 1.0)
        )
        crack = g.mul(g.mul(crack, g.saturate(g.add(blot, -0.3))), patches)
        colour = g.lerp(colour, g.mul(colour, 0.35), g.saturate(crack))
    return colour, g.saturate(rough)


def tinted(
    name, source, tile, gain=2.0, rough=None, metallic=None, specular=None, world=False, **aged
):
    """A texture's brightness times the vertex colour: one material for every tint of a surface."""
    g = Graph(name, MATS)
    t = g.texture_set(source, tile, uv=None if world else mesh_uv(g, tile))
    colour = g.mul(g.mul(desaturate(g, t["diff"]), vertex_rgb(g)), gain)
    r = t["rough"] if rough is None else g.const(rough)
    colour, r = wear(g, colour, r, **aged)
    colour, r = wetten(g, colour, r, upward=True)
    g.output(colour, t["nor"], r, specular, metallic)
    return finish(g)


def textured(
    name,
    source,
    tile,
    tint=None,
    rough_scale=1.0,
    rough_bias=0.0,
    world=True,
    specular=None,
    puddles=False,
    wet_rough=0.14,
    **aged,
):
    g = Graph(name, MATS)
    t = g.texture_set(source, tile, uv=None if world else mesh_uv(g, tile))
    colour = t["diff"] if tint is None else g.mul(t["diff"], g.const(tint))
    rough = t["rough"]
    if rough_scale != 1.0:
        rough = g.mul(rough, rough_scale)
    if rough_bias:
        rough = g.add(rough, rough_bias)
    colour, rough = wear(g, colour, rough, **aged)
    colour, rough = wetten(g, colour, g.saturate(rough), puddles, upward=True, wet_rough=wet_rough)
    g.output(colour, t["nor"], rough, specular)
    return finish(g)


def flat(name, colour, rough=0.5, metallic=0.0, specular=None, emissive=None, vertex=False, **aged):
    g = Graph(name, MATS)
    c = g.const(colour)
    if vertex:
        c = g.mul(c, vertex_rgb(g))
    lit = c
    if aged:
        c, rough = wear(g, c, g.const(rough), **aged)
    g.output(c, None, rough, specular, metallic, None if emissive is None else g.mul(lit, emissive))
    return finish(g)


def stripes(name):
    """The yellow and black band round the foot of a pole."""
    g = Graph(name, MATS)
    world = g.node(unreal.MaterialExpressionWorldPosition)
    z = g.mask(world, "b")
    xy = g.mask(world, "r")
    band = g.node(unreal.MaterialExpressionFrac)
    g.link(g.mul(g.add(z, g.mul(xy, 0.5)), 1.0 / 30.0), band)
    step = g.node(unreal.MaterialExpressionStep)
    g.link(g.const(0.5), step, "Y")
    g.link(band, step, "X")
    colour = g.lerp(g.const((0.02, 0.02, 0.02)), g.const((0.75, 0.55, 0.02)), step)
    g.output(colour, None, 0.5)
    return finish(g)


def finish(g):
    g.material.set_editor_property("used_with_nanite", True)
    g.material.set_editor_property("used_with_instanced_static_meshes", True)
    return g.finish()


def build_materials():
    m = {}
    m["PathTown"] = textured(
        "M_PathTown",
        "concrete_pavement",
        260,
        (0.9, 0.9, 0.88),
        grime=0.45,
        patches=0.6,
        chunked=True,
    )
    m["PathDike"] = textured(
        "M_PathDike", "brushed_concrete", 300, (0.85, 0.84, 0.8), grime=0.5, moss=0.25, chunked=True
    )
    m["PathAsphalt"] = textured(
        "M_PathAsphalt",
        "asphalt_02",
        300,
        (0.55, 0.55, 0.57),
        1.2,
        0.05,
        puddles=True,
        grime=0.35,
        patches=1.0,
        chunked=True,
    )
    m["Kerb"] = textured(
        "M_Kerb",
        "concrete_pavement",
        200,
        (0.6, 0.6, 0.58),
        world=False,
        grime=0.6,
        moss=0.45,
        chunked=True,
    )
    # Road paint worn into the surface: dimmer than new, dirtied in patches, so wet it stops
    # glowing.
    m["Line"] = flat("M_Line", (0.6, 0.6, 0.57), 0.6, grime=0.55, chunked=True)
    m["Gutter"] = textured(
        "M_Gutter",
        "brushed_concrete",
        150,
        (0.7, 0.7, 0.68),
        world=False,
        grime=0.7,
        moss=0.6,
        chunked=True,
    )
    m["Grate"] = flat("M_Grate", (0.02, 0.02, 0.02), 0.8, vertex=True)
    # Galvanised steel: pale, half metallic, dulled by road dirt. Rain streaks stay off the beam,
    # where noise squashed tall runs across it as long smears; the posts get them.
    m["Rail"] = flat("M_Rail", (0.56, 0.57, 0.56), 0.5, 0.6, grime=0.6, rust=0.25, chunked=True)
    m["RailPost"] = flat(
        "M_RailPost", (0.42, 0.44, 0.45), 0.6, 0.5, grime=0.7, streaks=0.4, rust=0.3, chunked=True
    )
    m["Pole"] = textured(
        "M_Pole",
        "brushed_concrete",
        120,
        (0.75, 0.75, 0.73),
        world=False,
        grime=0.5,
        streaks=0.6,
        splash=60,
        moss=0.2,
    )
    m["PoleBand"] = stripes("M_PoleBand")
    m["Crossarm"] = flat("M_Crossarm", (0.35, 0.36, 0.37), 0.5, 0.8, rust=0.6)
    m["Insulator"] = flat("M_Insulator", (0.75, 0.72, 0.65), 0.2)
    m["Transformer"] = flat(
        "M_Transformer", (0.5, 0.52, 0.52), 0.45, 0.6, grime=0.4, streaks=0.6, rust=0.3
    )
    m["Wire"] = flat("M_Wire", (0.02, 0.02, 0.02), 0.5)
    m["Plaster"] = tinted(
        "M_Plaster",
        "white_plaster_rough_01",
        200,
        1.6,
        grime=0.35,
        streaks=0.6,
        splash=60,
        moss=0.2,
    )
    m["Boards"] = tinted(
        "M_Boards", "weathered_plank_siding", 200, 2.2, grime=0.5, streaks=0.4, splash=50
    )
    m["RoofTile"] = tinted("M_RoofTile", "grey_roof_tiles", 160, 2.0, grime=0.5, moss=0.5)
    m["Base"] = textured(
        "M_Base",
        "brushed_concrete",
        200,
        (0.75, 0.74, 0.72),
        world=False,
        grime=0.6,
        streaks=0.6,
        splash=50,
        moss=0.45,
    )
    m["Glass"] = flat("M_Glass", (0.02, 0.025, 0.03), 0.06, 0.0, 0.8)
    m["Frame"] = flat("M_Frame", (0.62, 0.62, 0.6), 0.35, 1.0, grime=0.4)
    m["Shutter"] = tinted(
        "M_Shutter", "painted_metal_shutter", 150, 1.6, grime=0.45, streaks=0.6, splash=40, rust=0.3
    )
    m["Door"] = flat("M_Door", (0.12, 0.1, 0.08), 0.4, 0.5, vertex=True, grime=0.35)
    m["Fascia"] = flat("M_Fascia", (0.06, 0.055, 0.05), 0.6)
    m["AC"] = flat("M_AC", (0.82, 0.82, 0.78), 0.45, vertex=True, grime=0.6, streaks=0.7, rust=0.25)
    m["Block"] = textured(
        "M_Block",
        "concrete_block_wall",
        200,
        (0.85, 0.85, 0.83),
        world=False,
        grime=0.6,
        streaks=0.7,
        splash=50,
        moss=0.6,
    )
    m["Metal"] = tinted(
        "M_Metal", "brushed_concrete", 200, 1.8, rough=0.4, metallic=0.3, grime=0.4, rust=0.4
    )
    m["Red"] = flat(
        "M_Red", (0.75, 0.10, 0.03), 0.5, vertex=True, grime=0.4, streaks=0.4, splash=40
    )
    m["Stone"] = tinted("M_Stone", "rock_face", 120, 1.4, moss=0.6)
    m["Wood"] = tinted("M_Wood", "hinoki_planks", 150, 2.0, grime=0.45, streaks=0.3, splash=40)
    m["Sign"] = flat("M_Sign", (0.9, 0.95, 1.0), 0.3, emissive=2.5)
    m["Mirror"] = flat("M_Mirror", (0.95, 0.95, 0.95), 0.03, 1.0)
    m["Corrugated"] = textured(
        "M_Corrugated", "corrugated_iron", 150, world=False, grime=0.4, rust=0.6
    )
    m["Concrete"] = textured(
        "M_Concrete",
        "brushed_concrete",
        250,
        (0.72, 0.72, 0.7),
        world=False,
        grime=0.6,
        streaks=0.8,
        splash=60,
        moss=0.55,
    )
    m["Aze"] = textured("M_Aze", "leafy_grass", 180, (0.8, 0.85, 0.7))
    m["Cans"] = flat("M_Cans", (1.0, 1.0, 1.0), 0.35, emissive=0.6, vertex=True)
    m["Curtain"] = flat("M_Curtain", (1.0, 1.0, 1.0), 0.9, vertex=True)
    m["Culm"] = flat("M_Culm", (1.0, 1.0, 1.0), 0.35, vertex=True)
    m["BambooLeaves"] = card_leaves("bamboo", "Bamboo")
    m["SugiLeaves"] = card_leaves("sugi", "Sugi")
    m["SugiBark"] = tinted("M_SugiBark", "dark_wooden_planks", 120, 3.0)
    # The gorge village.
    m["Gravel"] = textured("M_Gravel", "gravel_ground_01", 120, (0.33, 0.32, 0.30), wet_rough=0.5)
    # On the bank's own UVs (one per 2 m from make_paths, so 80 here is a 160 cm repeat): a top-down
    # world projection smears a battered wall into vertical streaks, and wet it reads as black
    # glass.
    m["Ishigaki"] = textured(
        "M_Ishigaki",
        "japanese_stone_wall",
        80,
        (0.95, 0.95, 0.9),
        world=False,
        wet_rough=0.4,
        moss=0.45,
        grime=0.2,
    )
    m["Lattice"] = flat("M_Lattice", (1.0, 1.0, 1.0), 0.55, vertex=True, grime=0.4)
    m["Shoji"] = flat("M_Shoji", (1.0, 0.78, 0.5), 0.9, emissive=3.0)
    m["Interior"] = flat("M_Interior", (0.03, 0.025, 0.02), 0.9)
    m["Noren"] = flat("M_Noren", (1.0, 1.0, 1.0), 0.85, vertex=True)
    m["Lamp"] = flat("M_Lamp", (1.0, 0.85, 0.6), 0.8, emissive=4.0)
    m["Chochin"] = flat("M_Chochin", (1.0, 1.0, 1.0), 0.8, emissive=3.0, vertex=True)
    m["Paper"] = flat("M_Paper", (1.0, 0.82, 0.55), 0.8, emissive=5.0)
    m["MapleBark"] = (
        tinted("M_MapleBark", "bark_brown_02", 80, 1.6)
        if os.path.exists("/workspace/ue/textures/bark_brown_02_diff_2k.jpg")
        else tinted("M_MapleBark", "dark_wooden_planks", 80, 2.4)
    )
    m["MomijiGreen"] = card_leaves("momiji_green", "MomijiGreen")
    m["MomijiRed"] = card_leaves("momiji_red", "MomijiRed")
    m["Ajisai"] = flat("M_Ajisai", (1.0, 1.0, 1.0), 0.7, vertex=True)
    m["AjisaiLeaf"] = flat("M_AjisaiLeaf", (1.0, 1.0, 1.0), 0.45, vertex=True)
    m["Tuft"] = flat("M_Tuft", (1.0, 1.0, 1.0), 0.65, vertex=True)
    m["MatsuNeedles"] = flat("M_MatsuNeedles", (1.0, 1.0, 1.0), 0.85, vertex=True)
    m["Stream"] = stream_water()
    m["Rain"] = rain_material()
    return m


def stream_water():
    """Clear shallow water running over stones: lighter than the paddies, and moving."""
    g = Graph("M_Stream", MATS)
    waves = lib.texture("sea_waves_nor_dx", "normal", "/workspace/ue/textures/sea_waves_nor_dx.png")
    time = g.node(unreal.MaterialExpressionTime)
    uv = g.world_uv(250)
    yaw = math.radians(PATH.at(PATH.zone_start("village"))[3])
    drift = g.node(
        unreal.MaterialExpressionConstant2Vector, r=-0.25 * math.cos(yaw), g=-0.25 * math.sin(yaw)
    )
    n1 = g.mask(g.sample(waves, g.add(uv, g.mul(drift, time)), "normal"), "rgb")
    n2 = g.mask(
        g.sample(waves, g.add(g.mul(uv, 1.7), g.mul(drift, g.mul(time, 1.3))), "normal"), "rgb"
    )
    # Running water is broken up, not a mirror: a smooth stream under dark woods read as a black
    # glass panel.
    n = g.lerp(g.lerp(n1, n2, g.const(0.5)), g.const((0, 0, 1)), g.const(0.15))
    g.output(g.const((0.07, 0.09, 0.08)), n, 0.12, specular=0.5)
    return finish(g)


def rain_material():
    """Falling streaks on the camera's rain shells, only where it is raining, fading near
    surfaces."""
    g = Graph("M_Rain", MATS)
    streaks = lib.texture("rain_streaks", "mask", "/workspace/ue/textures/rain_streaks.png")
    time = g.node(unreal.MaterialExpressionTime)
    coords = g.node(unreal.MaterialExpressionTextureCoordinate)
    fall = g.node(unreal.MaterialExpressionConstant2Vector, r=0.0, g=-1.4)
    sway = g.node(unreal.MaterialExpressionConstant2Vector, r=0.03, g=0.0)
    uv = g.add(g.add(coords, g.mul(fall, time)), g.mul(sway, time))
    streak = g.mask(g.sample(streaks, uv, "mask"), "r")
    # Rain is wetness without the drying tail.
    raining = ride_value(g, "Rain")
    fade = g.node(unreal.MaterialExpressionDepthFade, fade_distance_default=150.0)
    g.link(g.mul(g.mul(streak, raining), 0.2), fade, "Opacity")
    g.output(None, None, None, None, None, g.const((0.42, 0.46, 0.5)), opacity=fade)
    g.material.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_UNLIT)
    return g.finish(blend_mode=unreal.BlendMode.BLEND_TRANSLUCENT, two_sided=True)


def card_leaves(kind, title):
    """A generated leaf-spray card (textures/<kind>_leaves_*.png), on the Poly Haven foliage
    master."""
    folder = f"{lib.FOLDER}/Gen/{title}"
    tex = "/workspace/ue/textures"
    return lib.instance(
        f"MI_{title}Leaves",
        unreal.load_asset(f"{lib.FOLDER}/Materials/M_PH_Foliage"),
        folder,
        textures={
            "Colour": lib.texture(
                f"{kind}_leaves_diff", source=f"{tex}/{kind}_leaves_diff.png", folder=folder
            ),
            "Alpha": lib.texture(
                f"{kind}_leaves_alpha", "mask", f"{tex}/{kind}_leaves_alpha.png", folder
            ),
            "Normal": lib.texture(
                f"{kind}_leaves_nor", "normal", f"{tex}/{kind}_leaves_nor.png", folder
            ),
            "ARM": lib.texture(
                f"{kind}_leaves_arm", "linear", f"{tex}/{kind}_leaves_arm.png", folder
            ),
        },
        vectors={"Tint": (1.0, 1.0, 1.0, 1.0)},
    )


# --- generated meshes ----------------------------------------------------------------------


def generated(folder, files, materials):
    """Imports (if needed) and returns {name: mesh} for generated glTF files, slots filled by
    name."""
    result = {}
    for path in files:
        name = os.path.basename(path)[:-5]
        dest = f"{folder}/{name}"
        mesh = find_mesh(dest)
        if mesh is None or REIMPORT:
            lib.import_file(path, dest)
            mesh = find_mesh(dest)
            if mesh is None:
                unreal.log_error(f"COAST no mesh from {path}")
                continue
            slots = mesh.get_editor_property("static_materials")
            for slot in slots:
                key = str(slot.get_editor_property("material_slot_name"))
                if key in materials:
                    slot.set_editor_property("material_interface", materials[key])
                else:
                    log(f"no material for slot {key} in {name}")
            mesh.set_editor_property("static_materials", slots)
            settings = mesh.get_editor_property("nanite_settings")
            settings.set_editor_property("enabled", True)
            mesh.set_editor_property("nanite_settings", settings)
            assets.save_directory(dest, only_if_is_dirty=False, recursive=True)
        result[name] = mesh
    return result


def find_mesh(folder):
    if not assets.does_directory_exist(folder):
        return None
    for path in assets.list_assets(folder, recursive=True):
        if path.split(".")[0].split("/")[-2] == "StaticMeshes" or True:
            asset = unreal.load_asset(path)
            if isinstance(asset, unreal.StaticMesh):
                return asset
    return None


def ph(model_id):
    """Every static mesh imported from one Poly Haven model."""
    folder = f"{lib.FOLDER}/PH/{model_id}"
    found = []
    for path in assets.list_assets(folder, recursive=True):
        asset = unreal.load_asset(path)
        # A model with LODs in its glTF imports each LOD as a mesh: keep LOD0 only.
        if isinstance(asset, unreal.StaticMesh) and not re.search(r"_LOD[1-9]", asset.get_name()):
            found.append(asset)
    if not found:
        log(f"WARNING no meshes for {model_id}")
    return found


# --- placement helpers ---------------------------------------------------------------------


def frame(s):
    x, y, z, yaw = PATH.at(s)
    r = math.radians(yaw)
    return (x, y, z), (math.cos(r), math.sin(r)), (-math.sin(r), math.cos(r)), yaw


def ground_at(s, lateral):
    (x, y, _), _, right, _ = frame(s)
    px, py = x + right[0] * lateral, y + right[1] * lateral
    return px, py, PATH.ground(px, py)


def clear_of_path(x, y, margin=80.0):
    s, lateral = PATH.locate(x, y)
    return abs(lateral) > PATH.width(s) / 2 + margin


_SPREAD = {}


def spread(mesh):
    """How far a mesh reaches sideways from its origin, in cm (unscaled)."""
    key = mesh.get_path_name()
    if key not in _SPREAD:
        box = mesh.get_bounding_box()
        _SPREAD[key] = max(abs(box.min.x), abs(box.max.x), abs(box.min.y), abs(box.max.y))
    return _SPREAD[key]


def bedded_rocks(instances, rocks, skirt, rng, s0, s1, count, lateral_range, scale):
    """Rocks bedded into the ground rather than set on it: each sits on the lowest ground under its
    footprint, sunk a third of its height, barely tilted, with moss, weeds and ferns grown up
    round its foot so no hard edge or gap shows where it meets the floor."""
    placed = 0
    for _ in range(int(count * DENSITY)):
        s = rng.uniform(s0, s1)
        side = rng.choice((-1, 1))
        near, far = lateral_range
        lateral = side * (near + (far - near) * rng.random() ** 1.6)
        x, y, _ = ground_at(s, lateral)
        k = rng.uniform(*scale)
        mesh = rng.choice(rocks)
        reach = spread(mesh) * k
        if not clear_of_path(x, y, 80 + reach):
            continue
        box = mesh.get_bounding_box()
        lowest = min(
            PATH.ground(x + math.cos(a) * reach * 0.7, y + math.sin(a) * reach * 0.7)
            for a in (0.0, 1.57, 3.14, 4.71)
        )
        lowest = min(lowest, PATH.ground(x, y))
        instances.add(
            mesh,
            (x, y, lowest - (box.max.z - box.min.z) * k * 0.33 - box.min.z * k),
            rng.uniform(0, 360),
            k,
            pitch=rng.uniform(-6, 6),
            roll=rng.uniform(-6, 6),
        )
        placed += 1
        if not skirt:
            continue
        for _ in range(rng.randint(3, 6)):
            a = rng.uniform(0, 2 * math.pi)
            r = reach * rng.uniform(0.75, 1.15)
            px, py = x + math.cos(a) * r, y + math.sin(a) * r
            if not clear_of_path(px, py, 40):
                continue
            plant = rng.choice(skirt)
            instances.add(
                plant,
                (px, py, PATH.ground(px, py) - 3),
                rng.uniform(0, 360),
                rng.uniform(0.8, 1.5),
                pitch=rng.uniform(-8, 8),
                roll=rng.uniform(-8, 8),
            )
    return placed


def scatter(
    instances,
    meshes,
    rng,
    s0,
    s1,
    count,
    lateral_range,
    scale,
    sides=(-1, 1),
    margin=80.0,
    sink=5.0,
    zone=None,
    tilt=4.0,
    keep=None,
):
    """Scatters `count` of `meshes` between s0 and s1, `lateral_range` (near, far) cm off the
    path."""
    if not meshes:
        return 0
    placed = 0
    for _ in range(int(count * DENSITY)):
        s = rng.uniform(s0, s1)
        if zone and PATH.zone(s) not in zone:
            continue
        side = rng.choice(sides)
        near, far = lateral_range
        # Denser near the near edge.
        lateral = side * (near + (far - near) * rng.random() ** 1.6)
        x, y, z = ground_at(s, lateral)
        if not clear_of_path(x, y, margin):
            continue
        if keep and not keep(x, y, z, s, lateral):
            continue
        k = rng.uniform(*scale)
        mesh = rng.choice(meshes)
        # Nothing reaches over the lane: keep the plant's own spread clear as well as the margin.
        if not clear_of_path(x, y, margin + spread(mesh) * k):
            continue
        instances.add(
            mesh,
            (x, y, z - sink * k),
            rng.uniform(0, 360),
            k,
            pitch=rng.uniform(-tilt, tilt),
            roll=rng.uniform(-tilt, tilt),
        )
        placed += 1
    return placed


def sward(kit, rng):
    """A carpet of grass clumps on the open verges near the path, so the ground there reads as grass
    and not as a painted texture: the coast's sea side to 12 m (its wooded side is litter beyond 3
    m),
    the paddies' bunds and the town's strips. Culled at 50 m; past that the terrain's colour
    carries it."""
    # By kind (V_tuft_N, kind N % 3): short bunches, knee-high fans, tall seeding stands.
    kinds = [
        [kit[k] for k in sorted(kit) if k.startswith("V_tuft_") and int(k[7:]) % 3 == n]
        for n in range(3)
    ]
    if not all(kinds):
        return 0
    reach = {
        "coast": {1: 1200, -1: 1400},
        "paddies": {1: 200, -1: 200},
        "town": {1: 150, -1: 150},
        "village": {1: 520, -1: 0},
    }
    cell = 45.0
    sward_ = lib.Instances("Sward")
    placed = 0
    s = 0.0
    while s < PATH.length:
        zone = PATH.zone(s)
        if zone in reach:
            half = PATH.width(s) / 2 + 30
            for side in (-1, 1):
                lat = 0.0
                while lat < reach[zone][side]:
                    # Patchy, not a lawn: density follows two crossed waves at odd scales.
                    patch = (
                        0.5
                        + 0.25 * math.sin(s * 0.0011 + lat * 0.004)
                        + 0.25 * math.sin(s * 0.00037 - lat * 0.0021 + 1.7)
                    )
                    # A finer wobble breaks the waves into tussocks and bare gaps; thin ground grows
                    # short bunches, lush ground the tall stands, and the clumps grow with it.
                    patch = min(
                        1.0,
                        max(
                            0.0,
                            patch
                            + 0.3
                            * math.sin(s * 0.0071 + lat * 0.013)
                            * math.sin(lat * 0.0057 - s * 0.0043),
                        ),
                    )
                    # Tussocks, not one clump to a cell: an even one per cell reads as a planted
                    # grid. A cell grows a cluster of one to four (2.5 on average) as often as it
                    # used to grow one clump, so the cover stays the same.
                    if rng.random() < 1.1 * patch**1.6 * min(1.0, DENSITY) / 2.5:
                        cs = s + rng.uniform(-cell, cell * 2)
                        cl = half + lat + rng.uniform(-cell * 0.5, cell * 1.5)
                        for _ in range(rng.randint(1, 4)):
                            ls = cs + rng.gauss(0, cell * 0.45)
                            ll = side * max(half - 10, cl + rng.gauss(0, cell * 0.45))
                            px, py, pz = ground_at(ls, ll)
                            pick = patch + rng.uniform(-0.35, 0.35)
                            kind = kinds[0 if pick < 0.45 else 1 if pick < 0.85 else 2]
                            size = (0.55 + 0.7 * patch) * rng.uniform(0.6, 1.3)
                            sward_.add(
                                rng.choice(kind),
                                (px, py, pz - 2),
                                rng.uniform(0, 360),
                                size,
                                pitch=rng.uniform(-8, 8),
                                roll=rng.uniform(-8, 8),
                            )
                            placed += 1
                    lat += cell
        s += cell
    sward_.commit(cull=5000)
    log(f"sward: {placed} clumps")
    return placed


def verges(rng):
    """Rough grass, weeds and wild flowers in clumps along the path, wherever it isn't jungle or
    village (they have their own floors): thick at walls and posts, thin on the open verge."""
    meshes = (
        ph("grass_medium_01") * 3
        + ph("grass_medium_02") * 3
        + ph("grass_bermuda_01") * 2
        + ph("weed_plant_02") * 2
        + ph("nettle_plant")
        + ph("dandelion_01") * 2
        + ph("celandine_01")
    )
    if not meshes:
        return 0
    # Strays are low plants: a lone tall weed in an open verge reads as a stalk stuck in the ground.
    low = ph("grass_bermuda_01") + ph("celandine_01") + ph("dandelion_01") or meshes
    tall = {m.get_name() for m in ph("weed_plant_02") + ph("nettle_plant")}
    clumps = lib.Instances("Verge")
    reach = {"town": (20, 380), "paddies": (20, 300), "coast": (30, 700)}
    placed = 0
    s = rng.uniform(0, 400)
    while s < PATH.length - 100:
        zone = PATH.zone(s)
        if zone in reach:
            for side in (-1, 1):
                if rng.random() < 0.35:
                    continue
                near, far = reach[zone]
                lateral = side * (PATH.width(s) / 2 + near + (far - near) * rng.random() ** 2.2)
                kind = rng.choice(meshes)  # a clump is mostly one plant, with strays
                # Weeds grow in thickets: three of them spread over two metres read as stalks.
                thicket = kind.get_name() in tall
                spacing = 45 if thicket else 90
                for _ in range(rng.randint(7, 12) if thicket else rng.randint(3, 11)):
                    cs = s + rng.gauss(0, spacing)
                    cl = lateral + side * abs(rng.gauss(0, spacing * 0.66))
                    x, y, z = ground_at(cs, cl)
                    mesh = kind if rng.random() < 0.75 else rng.choice(low)
                    k = rng.uniform(0.7, 1.6)
                    if mesh.get_name() in tall:
                        k = min(k, 1.05)
                    if not clear_of_path(x, y, 25 + spread(mesh) * k):
                        continue
                    clumps.add(
                        mesh,
                        (x, y, z - 3),
                        rng.uniform(0, 360),
                        k,
                        pitch=rng.uniform(-8, 8),
                        roll=rng.uniform(-8, 8),
                    )
                    placed += 1
        s += rng.uniform(250, 650)
    clumps.commit(cull=9000)
    log(f"verges: {placed} clumps")
    return placed


def facing(s, side):
    """Yaw that turns a kit piece's local +Y toward the path from `side`."""
    yaw = PATH.at(s)[3]
    return yaw if side < 0 else yaw + 180.0


# --- the level ---------------------------------------------------------------------------


def sky():
    elevation, azimuth = (float(v) for v in (os.environ.get("OC_SUN") or "34,-62").split(","))
    sun = spawn(unreal.DirectionalLight, (0, 0, 5000), (-elevation, azimuth, 0), "Sun", "Sky")
    light = sun.get_component_by_class(unreal.DirectionalLightComponent)
    light.set_intensity(float(os.environ.get("OC_SUN_LUX") or "90000"))
    light.set_editor_property("light_source_angle", 0.5357)
    light.set_editor_property("atmosphere_sun_light", True)
    light.set_editor_property("mobility", unreal.ComponentMobility.MOVABLE)
    light.set_editor_property("dynamic_shadow_distance_movable_light", 30000.0)
    spawn(unreal.SkyAtmosphere, label="Atmosphere", folder="Sky")
    skylight = spawn(
        unreal.SkyLight, (0, 0, 5000), label="SkyLight", folder="Sky"
    ).get_component_by_class(unreal.SkyLightComponent)
    skylight.set_editor_property("mobility", unreal.ComponentMobility.MOVABLE)
    skylight.set_editor_property("real_time_capture", True)
    fog = spawn(
        unreal.ExponentialHeightFog, (0, 0, -800), label="Fog", folder="Sky"
    ).get_component_by_class(unreal.ExponentialHeightFogComponent)
    fog.set_editor_property("fog_density", float(os.environ.get("OC_FOG") or "0.006"))
    fog.set_editor_property("fog_height_falloff", 0.05)
    fog.set_volumetric_fog(True)
    clouds = spawn(unreal.VolumetricCloud, label="Clouds", folder="Sky").get_component_by_class(
        unreal.VolumetricCloudComponent
    )
    # The default sample counts posterise the clouds into flat bands in a held shot.
    for prop, value in (
        ("view_sample_count_scale", 4.0),
        ("reflection_view_sample_count_scale", 2.0),
        ("shadow_view_sample_count_scale", 2.0),
    ):
        try:
            clouds.set_editor_property(prop, value)
        except Exception as exc:
            log(f"clouds: {prop}: {exc}")
    # Denser, more eroded than the engine's thin default, so cumulus shade themselves
    # (cloud_swap.py).
    density, e1, e2, coverage = (
        float(x) for x in (os.environ.get("OC_CLOUD") or "0.03,0.25,0.12,-0.2").split(",")
    )
    mi_path = "/Game/Coast/MI_Clouds"
    mi = (
        unreal.load_asset(mi_path)
        if unreal.EditorAssetLibrary.does_asset_exist(mi_path)
        else unreal.AssetToolsHelpers.get_asset_tools().create_asset(
            "MI_Clouds",
            "/Game/Coast",
            unreal.MaterialInstanceConstant,
            unreal.MaterialInstanceConstantFactoryNew(),
        )
    )
    mel = unreal.MaterialEditingLibrary
    mel.set_material_instance_parent(
        mi, unreal.load_asset("/Engine/EngineSky/VolumetricClouds/m_SimpleVolumetricCloud_Inst")
    )
    mel.set_material_instance_scalar_parameter_value(mi, "Cloud_GlobalDensity", density)
    mel.set_material_instance_scalar_parameter_value(mi, "Cloud_GlobalCoverage", coverage)
    mel.set_material_instance_vector_parameter_value(
        mi, "Noise_Strength", unreal.LinearColor(0.8, e1, e2, 2.5)
    )
    mel.update_material_instance(mi)
    unreal.EditorAssetLibrary.save_loaded_asset(mi)
    clouds.set_editor_property("material", mi)
    post = spawn(unreal.PostProcessVolume, label="Post", folder="Sky")
    post.set_editor_property("unbound", True)
    settings = post.get_editor_property("settings")
    settings.set_editor_property("override_auto_exposure_bias", True)
    settings.set_editor_property("auto_exposure_bias", 0.3)
    # The ride keys these from the weather's light (make_coast_ride.py), so the exposure cannot open
    # up in the shade of a jungle and burn the sunlit road white.
    for prop, value in (
        ("auto_exposure_min_brightness", -10.0),
        ("auto_exposure_max_brightness", 20.0),
    ):
        settings.set_editor_property(f"override_{prop}", True)
        settings.set_editor_property(prop, value)
    post.set_editor_property("settings", settings)


def ground():
    folder = f"{lib.FOLDER}/Terrain"
    count = 0
    for directory in sorted(assets.list_assets(folder, recursive=False, include_folder=True)):
        name = directory.rstrip("/").split("/")[-1]
        if not name.startswith("T_"):
            continue
        mesh = find_mesh(f"{folder}/{name}")
        if mesh is None:
            continue
        actor = lib.mesh_actor(mesh, label=name, folder="Ground")
        if name == "T_sea":
            actor.static_mesh_component.set_editor_property("cast_shadow", False)
        count += 1
    log(f"ground: {count} tiles")


def path_and_poles(materials):
    meshes = generated(
        f"{lib.FOLDER}/Gen/Paths", sorted(glob.glob(f"{GEN}/paths/P_*.gltf")), materials
    )
    for name, mesh in meshes.items():
        lib.mesh_actor(mesh, label=name, folder="Path")
    log(f"path: {len(meshes)} chunks")


def town(kit, rng, span=None):
    """Houses on lots either side of the lane, block walls along their fronts, gardens, a shrine."""
    houses = [mesh for name, mesh in kit.items() if name.startswith("H_")]
    start, end = span or PATH.spans("town")[0]
    end -= 1500
    garden = lib.Instances("Town/Gardens")
    walls = lib.Instances("Town/Walls")
    # Pruned garden pines, mostly; now and then a small broadleaf.
    pines = [kit[k] for k in sorted(kit) if k.startswith("K_matsu")]
    trees = pines * 3 + ph("tree_small_02")
    shrubs = ph("shrub_01") + ph("shrub_02") + ph("shrub_04")
    pots = ph("potted_plant_01") + ph("potted_plant_02") + ph("planter_pot_clay")
    clutter = ph("plastic_crate_01") + ph("wooden_bucket_01") + ph("Barrel_02") + ph("trashbag")
    placed = 0
    for side in (-1, 1):
        s = start + rng.uniform(0, 600)
        while s < end:
            mesh = rng.choice(houses)
            box = mesh.get_bounding_box()
            width = box.max.x - box.min.x
            depth = box.max.y - box.min.y
            half = PATH.width(s) / 2
            setback = half + 160 + rng.uniform(250, 600)
            centre = s + width / 2 + 120
            lateral = side * (setback + depth / 2)
            x, y, z = ground_at(centre, lateral)
            if rng.random() < 0.12:  # an empty lot: a vegetable patch, a kura, or nothing
                if rng.random() < 0.5:
                    kx, ky, kz = ground_at(centre, side * (setback + 300))
                    lib.mesh_actor(
                        kit["K_kura"],
                        (kx, ky, kz),
                        facing(centre, side) + rng.choice([0, 90, -90]),
                        label="Kura",
                        folder="Town",
                    )
                s += width + rng.uniform(300, 900)
                continue
            lib.mesh_actor(
                mesh,
                (x, y, z),
                facing(centre, side) + rng.uniform(-3, 3),
                label=f"House_{placed}",
                folder="Town/Houses",
            )
            placed += 1
            # A block wall along the front of the lot, with a gap for the gate. Each owner built
            # their
            # own: set back by their own amount, some lots open or hedged, and old walls settle.
            run = width + 240
            gate = rng.uniform(0.3, 0.7) * run
            wob = random.Random(int(s) * 31 + side)
            front = wob.random()
            wall_lat = side * (half + 160 + wob.uniform(-25, 70))
            lean = wob.uniform(-1.2, 1.2)
            for k in range(int(run // 200)):
                if abs(k * 200 + 100 - gate) < 180:
                    continue
                ws = s + k * 200 + 100
                wx, wy, wz = ground_at(ws, wall_lat + side * wob.uniform(-4, 4))
                if front < 0.15:
                    continue  # an open frontage
                if front < 0.3 and shrubs:
                    hedge, k = wob.choice(shrubs), wob.uniform(1.4, 1.9)
                    # A hedge is trimmed back from the lane: nothing reaches over it.
                    if clear_of_path(wx, wy, 20 + spread(hedge) * k):
                        garden.add(hedge, (wx, wy, wz), wob.uniform(0, 360), k)
                    continue
                walls.add(
                    kit["K_blockwall"],
                    (wx, wy, wz - wob.uniform(0, 6)),
                    facing(ws, side) + wob.uniform(-0.8, 0.8),
                    pitch=lean + wob.uniform(-0.4, 0.4),
                )
            # The garden between wall and house.
            for _ in range(rng.randint(1, 4)):
                gs = s + rng.uniform(0, run)
                gl = side * (half + 240 + rng.uniform(0, setback - half - 240))
                gx, gy, gz = ground_at(gs, gl)
                pick = rng.random()
                if pick < 0.3 and trees:
                    tree = rng.choice(trees)
                    k = rng.uniform(0.85, 1.2) if tree in pines else rng.uniform(0.55, 0.8)
                    if clear_of_path(gx, gy, 20 + spread(tree) * k):
                        garden.add(tree, (gx, gy, gz), rng.uniform(0, 360), k)
                elif pick < 0.7 and shrubs:
                    shrub, k = rng.choice(shrubs), rng.uniform(1.0, 1.8)
                    if clear_of_path(gx, gy, 20 + spread(shrub) * k):
                        garden.add(shrub, (gx, gy, gz), rng.uniform(0, 360), k)
                elif pots:
                    garden.add(rng.choice(pots), (gx, gy, gz), rng.uniform(0, 360), 1.0)
            if clutter and rng.random() < 0.6:
                cs = s + rng.uniform(0, width)
                cx, cy, cz = ground_at(cs, side * (setback - 60))
                garden.add(rng.choice(clutter), (cx, cy, cz), rng.uniform(0, 360), 1.0)
            # Life piles up by the door: crates, buckets, pots, a bag of rubbish.
            for _ in range(wob.choice([0, 1, 2, 3, 4]) if clutter else 0):
                cs = s + wob.uniform(0, width)
                cx, cy, cz = ground_at(cs, side * (setback - wob.uniform(30, 90)))
                garden.add(
                    wob.choice(clutter + pots),
                    (cx, cy, cz),
                    wob.uniform(0, 360),
                    wob.uniform(0.85, 1.1),
                )
            if rng.random() < 0.18:
                vs = s + gate
                vx, vy, vz = ground_at(vs, side * (half + 110))
                lib.mesh_actor(
                    kit["K_vending"], (vx, vy, vz), facing(vs, side), label="Vending", folder="Town"
                )
            s += run + rng.uniform(80, 400)
    walls.commit()
    garden.commit()
    # A curve mirror where the lane bends, a bus shelter where the town ends.
    for s in (start + 9000, start + 24000):
        mx, my, mz = ground_at(s, PATH.width(s) / 2 + 60)
        lib.mesh_actor(
            kit["K_mirror"], (mx, my, mz), facing(s, 1) + 30, label="Mirror", folder="Town"
        )
    bs = end + 600
    bx, by, bz = ground_at(bs, PATH.width(bs) / 2 + 220)
    lib.mesh_actor(kit["K_shelter"], (bx, by, bz), facing(bs, 1), label="BusStop", folder="Town")
    log(f"town: {placed} houses")


def paddies(materials, rng, span=None):
    """Flooded fields either side of the dike, planted in rows, with a strip of water over the
    mud."""
    start, end = span or PATH.spans("paddies")[0]
    rice = lib.Instances("Paddies/Rice")
    bunds = lib.Instances("Paddies/Aze")
    water = lib.Instances("Paddies/Water")
    plane = unreal.load_asset("/Engine/BasicShapes/Plane")
    cube = unreal.load_asset("/Engine/BasicShapes/Cube")
    grass = ph("grass_medium_02") + ph("grass_medium_01")
    if not grass:
        return
    # Fields: a grid of plots along the path; each plot's rows run its own way.
    plot = PATH.PLOT
    planted = 0
    s = start + 400
    while s < end - 400:
        for side in (-1, 1):
            for band in range(4):
                near = PATH.width(s) / 2 + 350 + band * plot
                if band > 0 and rng.random() < 0.15:
                    continue  # fallow
                angle = PATH.at(s)[3] + rng.choice([0, 0, 90, 4, -4])
                r = math.radians(angle)
                ux, uy = math.cos(r), math.sin(r)
                vx, vy = -uy, ux
                cx, cy, cz = ground_at(s + plot / 2, side * (near + plot / 2))
                if cz > PATH.height(s + plot / 2) - 40:
                    continue  # not a flooded field here
                cz = PATH.paddy_level(
                    s + plot / 2
                )  # the plot's own level, which the terrain shares
                # Where the fields meet a hillside, or the zone blends out, the ground isn't level
                # under
                # the whole plot, and its bunds would stand on stilts over the slope: leave it as
                # hill.
                corners = [
                    ground_at(s + plot * a, side * (near + plot * b))[2]
                    for a in (0.05, 0.95)
                    for b in (0.05, 0.95)
                ]
                if max(abs(z - cz) for z in corners) > 45:
                    continue
                # Aze: a grassy bund 45 cm wide round the plot, standing 25 cm out of the water.
                for k in range(4):
                    a = math.radians(angle + 90 * k)
                    ox, oy = math.cos(a) * plot / 2, math.sin(a) * plot / 2
                    bunds.add(
                        cube,
                        (cx + ox, cy + oy, cz - 40),
                        angle + 90 * k + 90,
                        (plot / 100 + 0.45, 0.45, 0.6),
                    )
                # Each plot holds its own water, a few cm over its mud, so no field stands dry.
                water.add(plane, (cx, cy, cz + 4), angle, (plot / 100, plot / 100, 1))
                spacing_u, spacing_v = 32.0 / math.sqrt(DENSITY), 45.0
                half = plot / 2 - 120
                v = -half
                while v < half:
                    u = -half + rng.uniform(0, spacing_u)
                    while u < half:
                        px = cx + ux * u + vx * v + rng.gauss(0, 3)
                        py = cy + uy * u + vy * v + rng.gauss(0, 3)
                        rice.add(
                            rng.choice(grass),
                            (px, py, cz - 2),
                            rng.uniform(0, 360),
                            rng.uniform(1.5, 2.1),
                        )
                        planted += 1
                        u += spacing_u
                    v += spacing_v
        s += plot
    rice.commit(shadows=True)
    bunds.commit(materials={cube.get_path_name(): {0: materials["Aze"]}})
    water.commit(
        shadows=False, materials={plane.get_path_name(): {0: unreal.load_asset(f"{MATS}/M_Paddy")}}
    )
    log(f"paddies: {planted} rice clumps")


def paddy_water():
    """Still water: darker and calmer than the sea, with a slow ripple."""
    g = Graph("M_Paddy", MATS)
    waves = lib.texture("sea_waves_nor_dx", "normal", "/workspace/ue/textures/sea_waves_nor_dx.png")
    time = g.node(unreal.MaterialExpressionTime)
    uv = g.world_uv(600)
    drift = g.node(unreal.MaterialExpressionConstant2Vector, r=0.004, g=0.003)
    n = g.mask(g.sample(waves, g.add(uv, g.mul(drift, time)), "normal"), "rgb")
    n = g.lerp(n, g.const((0, 0, 1)), g.const(0.8))
    g.output(g.const((0.03, 0.035, 0.025)), n, 0.04, specular=0.5)
    return finish(g)


def jungle(kit, rng, span=None):
    start, end = span or PATH.spans("jungle")[0]
    grove_start = start
    start, end = start - 1500, end + 600
    canopy = lib.Instances("Jungle/Canopy")
    under = lib.Instances("Jungle/Under")
    floor = lib.Instances("Jungle/Floor")
    big = ph("island_tree_01") + ph("island_tree_02") + ph("island_tree_03")
    mid = ph("tree_small_02") + ph("jacaranda_tree")
    sugi = [kit[k] for k in sorted(kit) if k.startswith("K_sugi")]
    ferns = ph("fern_02")
    shrubs = ph("shrub_01") + ph("shrub_02") + ph("shrub_03") + ph("shrub_04")
    weeds = ph("nettle_plant") + ph("weed_plant_02") + ph("celandine_01") + ph("grass_medium_01")
    moss = ph("moss_01")
    rocks = ph("rock_moss_set_01") + ph("boulder_01")
    logs = ph("root_cluster_01") + ph("dead_tree_trunk")
    length = end - start
    n = 0
    n += scatter(
        canopy, big, rng, start, end, length / 100 * 3.0, (350, 6000), (1.6, 2.6), sink=10, tilt=3
    )
    n += scatter(
        canopy, mid, rng, start, end, length / 100 * 1.4, (280, 3500), (1.0, 1.7), sink=5, tilt=4
    )
    n += scatter(
        under,
        ferns,
        rng,
        start,
        end,
        length / 100 * 14,
        (220, 2500),
        (2.0, 3.6),
        margin=60,
        sink=3,
        tilt=8,
    )
    n += scatter(
        under, shrubs, rng, start, end, length / 100 * 8, (240, 3000), (1.2, 2.2), margin=60, tilt=6
    )
    # Cedar stands among the broadleaves, as on Yakushima.
    if sugi:
        n += scatter(
            canopy,
            sugi,
            rng,
            start,
            end,
            length / 100 * 2.5,
            (320, 5000),
            (0.9, 1.25),
            sink=30,
            tilt=1,
        )
    n += scatter(
        floor, weeds, rng, start, end, length / 100 * 14, (160, 900), (1.0, 1.6), margin=40, tilt=10
    )
    n += scatter(
        floor,
        moss,
        rng,
        start,
        end,
        length / 100 * 10,
        (160, 1200),
        (1.0, 2.0),
        margin=40,
        sink=1,
        tilt=2,
    )
    n += bedded_rocks(
        floor,
        rocks,
        moss + weeds + ferns,
        rng,
        start,
        end,
        length / 100 * 1.2,
        (250, 2500),
        (0.6, 1.6),
    )
    n += scatter(
        floor, logs, rng, start, end, length / 100 * 0.6, (300, 2500), (0.9, 1.5), sink=5, tilt=6
    )
    bamboo = [kit[k] for k in sorted(kit) if k.startswith("K_bamboo")]
    if bamboo:
        grove_end = grove_start + 12000
        n += scatter(
            canopy,
            bamboo,
            rng,
            start,
            grove_end,
            (grove_end - start) / 100 * 6,
            (260, 2500),
            (0.9, 1.2),
            sink=20,
            tilt=2,
        )
    canopy.commit(cull=0)
    under.commit(cull=12000)
    floor.commit(cull=6000)
    log(f"jungle: {n}")


def hills(kit, rng, span=None, far=True):
    """Trees on the hills either side of the town and paddies, so the valley is wooded, not bald."""
    start, end = span or (PATH.spans("town")[0][0], PATH.spans("jungle")[0][0])
    woods = lib.Instances("Hills/Woods")
    broadleaf = (
        ph("island_tree_01") + ph("island_tree_02") + ph("island_tree_03") + ph("tree_small_02")
    )
    sugi = [kit[k] for k in sorted(kit) if k.startswith("K_sugi")]
    shrubs = ph("shrub_01") + ph("shrub_03")

    def wooded(x, y, z, s, lateral):
        return z > 150 and PATH.slope(x, y) < 0.9

    # Planted cedar on the near slopes, broadleaves mixed through it.
    n = scatter(
        woods,
        broadleaf,
        rng,
        start,
        end,
        (end - start) / 100 * 12,
        (4000, 25000),
        (1.8, 2.8),
        keep=wooded,
        sink=15,
    )
    if sugi:
        n += scatter(
            woods,
            sugi,
            rng,
            start,
            end,
            (end - start) / 100 * 30,
            (4000, 25000),
            (0.95, 1.3),
            keep=wooded,
            sink=30,
        )
    n += scatter(
        woods,
        shrubs,
        rng,
        start,
        end,
        (end - start) / 100 * 16,
        (3500, 20000),
        (2.0, 3.0),
        keep=wooded,
    )
    # The mountain and the far hills, the whole ride long: Japanese hills are forested to the top.
    if not far:
        woods.commit(cull=0)
        log(f"hills: {n}")
        return
    n += scatter(
        woods,
        broadleaf,
        rng,
        0,
        PATH.length,
        PATH.length / 100 * 18,
        (25000, 90000),
        (2.4, 3.4),
        keep=wooded,
        sink=20,
    )
    if sugi:
        n += scatter(
            woods,
            sugi,
            rng,
            0,
            PATH.length,
            PATH.length / 100 * 24,
            (25000, 90000),
            (1.2, 1.6),
            keep=wooded,
            sink=30,
        )
    woods.commit(cull=0)
    log(f"hills: {n}")


def coast(kit, rng, span=None):
    """The cliff road: rocks and cliffs down to the water, tetrapods in it, scrub on the land
    side."""
    start, end = span or PATH.spans("coast")[0]
    shore = lib.Instances("Coast/Shore")
    land = lib.Instances("Coast/Land")
    cliffs = ph("coastal_cliff_01") + ph("coastal_cliff_02") + ph("coastal_cliff_04")
    rocks = (
        ph("coast_rocks_01")
        + ph("coast_rocks_02")
        + ph("coast_rocks_05")
        + ph("coast_land_rocks_02")
    )
    small = ph("sand_rocks_small_01") + ph("boulder_01")
    faces = ph("rock_face_01") + ph("rock_face_02")
    trees = ph("island_tree_01") + ph("island_tree_02") + ph("island_tree_03") + ph("tree_small_02")
    scrub = (
        ph("shrub_01")
        + ph("shrub_02")
        + ph("shrub_03")
        + ph("shrub_04")
        + ph("grass_medium_01")
        + ph("fern_02")
    )
    n = 0
    s = start
    while s < end:
        gap = PATH.coast_gap(s)
        # Cliffs where land meets sea, sunk so their tops sit at the cliff edge.
        if cliffs and rng.random() < 0.8:
            lat = gap + rng.uniform(-600, 900)
            x, y, z = ground_at(s, lat)
            shore.add(
                rng.choice(cliffs),
                (x, y, layout.SEA_LEVEL - 600),
                facing(s, 1) + rng.uniform(-40, 40),
                rng.uniform(1.5, 3.0),
            )
            n += 1
        if rocks:
            for _ in range(2):
                lat = gap + rng.uniform(-200, 3000)
                x, y, _ = ground_at(s, lat)
                shore.add(
                    rng.choice(rocks),
                    (x, y, layout.SEA_LEVEL - rng.uniform(150, 450)),
                    rng.uniform(0, 360),
                    rng.uniform(0.4, 1.0),
                )
                n += 2
        if small:
            for _ in range(4):
                lat = gap + rng.uniform(-300, 1800)
                x, y, _ = ground_at(s, lat)
                shore.add(
                    rng.choice(small),
                    (x, y, layout.SEA_LEVEL - 60),
                    rng.uniform(0, 360),
                    rng.uniform(1.0, 3.0),
                    pitch=rng.uniform(-20, 20),
                )
        # Rock faces in the cut on the land side, where the hill rises steeply.
        if faces and rng.random() < 0.5:
            lat = -(PATH.width(s) / 2 + rng.uniform(700, 1400))
            x, y, z = ground_at(s, lat)
            if z > PATH.height(s) + 250:
                land.add(
                    rng.choice(faces),
                    (x, y, z - 350),
                    facing(s, -1) + rng.uniform(-25, 25),
                    rng.uniform(1.2, 2.0),
                )
        s += rng.uniform(1800, 3200)
    # Tetrapods: a breakwater of them along one stretch of the shore.
    tetra = kit.get("K_tetrapod")
    if tetra:
        s = start + (end - start) * 0.45
        while s < start + (end - start) * 0.62:
            gap = PATH.coast_gap(s)
            for row in range(3):
                x, y, _ = ground_at(s, gap + 500 + row * 260 + rng.uniform(-60, 60))
                shore.add(
                    tetra,
                    (x, y, layout.SEA_LEVEL - 120 + row * 40 + rng.uniform(-30, 30)),
                    rng.uniform(0, 360),
                    1.0,
                    pitch=rng.uniform(-25, 25),
                    roll=rng.uniform(-25, 25),
                )
            s += 230
    n += scatter(
        land,
        trees,
        rng,
        start,
        end,
        (end - start) / 100 * 3,
        (500, 9000),
        (1.6, 2.6),
        sides=(-1,),
        sink=12,
    )
    n += scatter(
        land,
        scrub,
        rng,
        start,
        end,
        (end - start) / 100 * 14,
        (180, 4000),
        (1.3, 2.6),
        sides=(-1,),
        margin=60,
    )
    n += scatter(
        land,
        scrub,
        rng,
        start,
        end,
        (end - start) / 100 * 6,
        (180, 900),
        (1.0, 2.0),
        sides=(1,),
        margin=70,
        keep=lambda x, y, z, s, lat: z > layout.SEA_LEVEL + 200,
    )
    # The cut above the road: Japanese road cuts are grown over with weeds, grass and fern, never
    # bare earth, so the first ten metres up it are thick with low cover.
    cover = (
        ph("grass_medium_01") * 2
        + ph("grass_medium_02") * 2
        + ph("fern_02") * 2
        + ph("weed_plant_02")
        + ph("nettle_plant")
        + ph("shrub_03")
    )
    n += scatter(
        land,
        cover,
        rng,
        start,
        end,
        (end - start) / 100 * 22,
        (150, 1300),
        (1.2, 2.4),
        sides=(-1,),
        margin=50,
        tilt=12,
    )
    shore.commit(cull=0)
    land.commit(cull=0)
    log(f"coast: {n}")


def shrine(kit, rng, at=None):
    """A torii and a little shrine where the jungle begins, stone lanterns either side."""
    s = (PATH.zone_start("jungle") if at is None else at) + 2200
    side = -1
    lat = -(PATH.width(s) / 2 + 450)
    x, y, z = ground_at(s, lat)
    yaw = PATH.at(s)[3]
    # The torii stands across a side path that leaves the lane at a right angle.
    lib.mesh_actor(kit["K_torii"], (x, y, z), yaw, label="Torii", folder="Shrine")
    for k, offset in enumerate((-260, 260)):
        lx, ly, lz = ground_at(s + offset, lat - 300)
        lib.mesh_actor(
            kit["K_lantern"], (lx, ly, lz), facing(s, side), label=f"Lantern{k}", folder="Shrine"
        )
    hx, hy, hz = ground_at(s, lat - 1100)
    lib.mesh_actor(kit["K_shrine"], (hx, hy, hz), facing(s, side), label="Hokora", folder="Shrine")


def point_light(
    where, candela, radius, colour=(1.0, 0.62, 0.32), label=None, folder="Village/Lights"
):
    actor = spawn(unreal.PointLight, where, label=label, folder=folder)
    light = actor.get_component_by_class(unreal.PointLightComponent)
    light.set_editor_property("intensity_units", unreal.LightUnits.CANDELAS)
    light.set_intensity(candela)
    light.set_light_color(unreal.LinearColor(*colour, 1.0))
    light.set_editor_property("attenuation_radius", radius)
    light.set_editor_property("cast_shadows", False)
    light.set_editor_property("mobility", unreal.ComponentMobility.MOVABLE)
    return actor


def local_to_world(centre, yaw, point):
    r = math.radians(yaw)
    lx, ly, lz = point
    return (
        centre[0] + lx * math.cos(r) - ly * math.sin(r),
        centre[1] + lx * math.sin(r) + ly * math.cos(r),
        centre[2] + lz,
    )


def village(kit, rng, span=None):
    """The gorge village: inns along the hillside of the road, the stream below a stone wall on the
    other, red lanterns down the kerb, maples meeting overhead, a shrine up a side path."""
    start, end = span or PATH.spans("village")[0]
    inns = [kit[k] for k in sorted(kit) if k.startswith("V_inn_")]
    lamps = {}
    for k in sorted(kit):
        if k.startswith("V_inn_"):
            with open(f"{GEN}/kit/{k}.json") as f:
                lamps[kit[k].get_path_name()] = json.load(f)["lamps"]
    maples = [kit[k] for k in sorted(kit) if k.startswith("V_maple_")]
    green = [m for m in maples if m.get_name() in ("V_maple_0", "V_maple_1", "V_maple_5")]
    turning = [m for m in maples if m not in green]
    hydrangeas = [kit[k] for k in sorted(kit) if k.startswith("V_ajisai_")]
    toro, fence = kit.get("V_toro"), kit.get("V_fence")
    chochin = [kit[k] for k in sorted(kit) if k.startswith("V_chochin_")]
    buildings = lib.Instances("Village/Inns")
    trees = lib.Instances("Village/Trees")
    small = lib.Instances("Village/Small")
    woods = lib.Instances("Village/Woods")
    shrine_s = start + (end - start) * 0.45
    n = 0

    # Inns down the hillside of the road, fronts on a line just off the edge, with gaps for gardens.
    s = start + 900
    while s < end - 900 and inns:
        if abs(s - shrine_s) < 700:
            s = shrine_s + 700
            continue
        mesh = rng.choice(inns)
        box = mesh.get_bounding_box()
        width = box.max.x - box.min.x
        half_depth = -box.min.y - 4
        mid = s + width / 2
        lat = -(PATH.width(mid) / 2 + 70 + half_depth)
        x, y, _ = ground_at(mid, lat)
        z = PATH.height(mid) + 2
        yaw = facing(mid, -1)
        buildings.add(mesh, (x, y, z), yaw)
        for k, lamp in enumerate(lamps.get(mesh.get_path_name(), [])):
            point_light(
                local_to_world((x, y, z), yaw, lamp), 60 if k == 0 else 120, 900 if k == 0 else 1300
            )
        n += 1
        s += width
        # A gap: a garden behind a bamboo fence, a maple, hydrangeas.
        gap = rng.choice([150, 250, 450, 700])
        if gap >= 400 and fence:
            g = s + 60
            while g < s + gap - 200:
                fx, fy, _ = ground_at(g + 100, -(PATH.width(g) / 2 + 90))
                small.add(fence, (fx, fy, PATH.height(g)), facing(g, -1))
                g += 200
            if maples:
                mx, my, mz = ground_at(s + gap / 2, -(PATH.width(s) / 2 + 300))
                trees.add(
                    rng.choice(green + turning),
                    (mx, my, mz - 10),
                    facing(s, -1) + rng.uniform(-25, 25),
                    rng.uniform(0.9, 1.15),
                )
        for _ in range(rng.choice([0, 1, 2, 2, 3]) if hydrangeas else 0):
            hx, hy, _ = ground_at(
                s + rng.uniform(30, max(60, gap - 30)), -(PATH.width(s) / 2 + rng.uniform(95, 130))
            )
            small.add(
                rng.choice(hydrangeas),
                (hx, hy, PATH.height(s)),
                rng.uniform(0, 360),
                rng.uniform(1.3, 1.7),
            )
        s += gap

    # Red lanterns along the kerb above the stream, lit; maples growing up out of the gorge.
    s = start + 500
    k = 0
    while s < end - 300 and toro:
        lat = PATH.width(s) / 2 + 12
        x, y, _ = ground_at(s, lat)
        z = PATH.height(s) + 14
        small.add(toro, (x, y, z), PATH.at(s)[3])
        if k % 2 == 0:
            point_light(local_to_world((x, y, z), 0, (0, 0, 190)), 25, 650, (1.0, 0.45, 0.22))
        k += 1
        s += 900
    # Daylight off the water: dim, cool, shadowless fill down in the gorge, so its bank and the
    # trunks out of it read at dusk in the rain instead of going to a flat black.
    lo, hi = layout.STREAM
    s = start + 400
    while s < end - 200:
        x, y, _ = ground_at(s, (lo + hi) / 2)
        point_light((x, y, PATH.height(s) - 60), 160, 1600, (0.62, 0.74, 1.0))
        s += 1100
    s = start + 300
    while s < end and maples:
        # They stand down the gorge bank, so a crown that clears the road from the bank is a taller
        # tree: size each so its lowest sprays (about 250 up the tree) are 300 over the road.
        lat = PATH.width(s) / 2 + rng.uniform(330, 520)
        x, y, z = ground_at(s, lat)
        scale = max(rng.uniform(1.25, 1.5), (PATH.height(s) + 300 - z) / 250)
        if scale > 2.0:
            lat += 250  # too deep to reach over: grow out of the bank further down, leaning in
            x, y, z = ground_at(s, lat)
            scale = min(2.0, max(1.4, (PATH.height(s) + 150 - z) / 250))
        trees.add(
            rng.choice(turning if rng.random() < 0.55 else green),
            (x, y, z - 10),
            facing(s, 1) + rng.uniform(-20, 20),
            scale,
            pitch=rng.uniform(-3, 3),
        )
        n += 1
        s += rng.uniform(450, 900)
    # Every so often a paper lantern on a pole bracket over the road.
    s = start + 1300
    while s < end - 500 and chochin:
        x, y, _ = ground_at(s, PATH.width(s) / 2 - 20)
        small.add(rng.choice(chochin), (x, y, PATH.height(s) + 330), PATH.at(s)[3] + 90)
        s += rng.uniform(1800, 3000)

    # The stream bed: boulders in it, ferns and moss on its banks, woods up the far side.
    lo, hi = layout.STREAM
    stones = ph("boulder_01") + ph("rock_moss_set_01")
    n += scatter(
        small,
        stones,
        rng,
        start - 500,
        end + 500,
        (end - start) / 100 * 3,
        (lo + 20, hi),
        (0.6, 1.8),
        sides=(1,),
        margin=200,
        sink=25,
        tilt=25,
    )
    n += scatter(
        small,
        ph("fern_02") + ph("moss_01"),
        rng,
        start,
        end,
        (end - start) / 100 * 6,
        (lo - 200, lo + 60),
        (1.5, 2.6),
        sides=(1,),
        margin=200,
        sink=3,
        tilt=15,
    )
    # The bank between the kerb and the stream: shrubs and ferns, so it isn't a bare dark slope. The
    # band is under 3 m wide and scatter drops whatever would spread over the lane, so the plants
    # are kept small and many: at 1.3-2.2 nearly every shrub was dropped and the bank stayed bare.
    n += scatter(
        small,
        ph("shrub_01") + ph("shrub_03") + ph("fern_02") + ph("grass_medium_01") + hydrangeas,
        rng,
        start,
        end,
        (end - start) / 100 * 16,
        (PATH.width(start) / 2 + 60, lo + 40),
        (0.9, 1.5),
        sides=(1,),
        margin=60,
        sink=4,
        tilt=20,
    )
    # The far side of the gorge, which is what the rider sees past the kerb: undergrowth under its
    # trees, or it is a bare dark wall behind the trunks.
    n += scatter(
        small,
        # Mostly green, a hydrangea here and there: every one of them read as an even purple dot.
        (ph("shrub_01") + ph("shrub_03") + ph("fern_02")) * 2 + ph("moss_01") + hydrangeas[:1],
        rng,
        start,
        end,
        (end - start) / 100 * 12,
        (hi + 20, 2200),
        (0.9, 3.0),
        sides=(1,),
        sink=4,
        tilt=20,
    )
    sugi = [kit[k] for k in sorted(kit) if k.startswith("K_sugi")]
    broadleaf = ph("island_tree_01") + ph("island_tree_02") + ph("tree_small_02")
    n += scatter(
        woods,
        maples,
        rng,
        start,
        end,
        (end - start) / 100 * 2.0,
        (hi + 100, 3500),
        (1.2, 1.6),
        sides=(1,),
        sink=10,
    )
    n += scatter(
        woods,
        sugi,
        rng,
        start - 500,
        end + 500,
        (end - start) / 100 * 5,
        (hi + 300, 9000),
        (1.0, 1.35),
        sides=(1,),
        sink=30,
    )
    n += scatter(
        woods,
        broadleaf,
        rng,
        start,
        end,
        (end - start) / 100 * 2,
        (hi + 300, 9000),
        (1.6, 2.4),
        sides=(1,),
        sink=15,
    )
    behind = layout.VILLAGE_LOTS + 300
    n += scatter(
        woods,
        sugi,
        rng,
        start - 500,
        end + 500,
        (end - start) / 100 * 6,
        (behind, 9000),
        (1.0, 1.35),
        sides=(-1,),
        sink=30,
    )
    n += scatter(
        woods,
        maples + broadleaf,
        rng,
        start,
        end,
        (end - start) / 100 * 2.5,
        (behind - 400, 6000),
        (1.1, 1.6),
        sides=(-1,),
        sink=10,
    )
    n += scatter(
        woods,
        ph("shrub_01") + ph("shrub_03") + ph("fern_02"),
        rng,
        start,
        end,
        (end - start) / 100 * 5,
        (behind - 600, 4000),
        (1.4, 2.4),
        sides=(-1,),
    )

    # The shrine: a torii off the road, lanterns up the approach, the hall among cedars.
    side_lat = -(PATH.width(shrine_s) / 2 + 250)
    tx, ty, _ = ground_at(shrine_s, side_lat)
    if "K_torii" in kit:
        lib.mesh_actor(
            kit["K_torii"],
            (tx, ty, PATH.height(shrine_s)),
            PATH.at(shrine_s)[3],
            1.1,
            label="VillageTorii",
            folder="Village/Shrine",
        )
    for j in range(5):
        for off in (-230, 230):
            lx, ly, lz = ground_at(shrine_s + off, side_lat - 250 - j * 280)
            if toro:
                small.add(toro, (lx, ly, max(lz, PATH.height(shrine_s))), facing(shrine_s, -1))
            if j % 2 == 0:
                point_light(
                    (lx, ly, max(lz, PATH.height(shrine_s)) + 190), 25, 600, (1.0, 0.45, 0.22)
                )
    if "K_shrine" in kit:
        hx, hy, hz = ground_at(shrine_s, side_lat - 1900)
        lib.mesh_actor(
            kit["K_shrine"],
            (hx, hy, hz),
            facing(shrine_s, -1),
            1.6,
            label="VillageHokora",
            folder="Village/Shrine",
        )

    buildings.commit(cull=0)
    trees.commit(cull=0)
    small.commit(cull=15000, shadows=True)
    woods.commit(cull=0)
    log(f"village: {n}")


def main():
    world = unreal.EditorLoadingAndSavingUtils.new_blank_map(False)
    materials = build_materials()
    paddy_water()
    kit = generated(f"{lib.FOLDER}/Gen/Kit", sorted(glob.glob(f"{GEN}/kit/*.gltf")), materials)
    log(f"kit: {sorted(kit)}")
    rng = random.Random(1859)
    sky()
    ground()
    path_and_poles(materials)
    # Each stretch of a zone gets its own builder call and dice; the first stretches keep the
    # seeds they always had, so the short ride is unchanged.
    for k, span in enumerate(PATH.spans("town")):
        town(kit, random.Random(11 + 100 * k), span)
    for k, span in enumerate(PATH.spans("paddies")):
        paddies(materials, random.Random(12 + 100 * k), span)
    hills(kit, random.Random(13))
    for k, (a, b) in enumerate(
        zip(PATH.spans("town")[1:], PATH.spans("paddies")[1:], strict=False)
    ):
        hills(kit, random.Random(113 + 100 * k), (a[0], b[1]), far=False)
    for k, span in enumerate(PATH.spans("jungle")):
        jungle(kit, random.Random(14 + 100 * k), span)
        shrine(kit, random.Random(15 + 100 * k), span[0])
    for k, span in enumerate(PATH.spans("village")):
        village(kit, random.Random(17 + 100 * k), span)
    for k, span in enumerate(PATH.spans("coast")):
        coast(kit, random.Random(16 + 100 * k), span)
    verges(random.Random(19))
    sward(kit, random.Random(23))
    del rng
    if not unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL):
        raise RuntimeError(f"could not save {LEVEL}")
    log(f"saved {LEVEL}")


main()
