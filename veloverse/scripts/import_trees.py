"""Imports the trees prep_trees.py prepared into /Game/Trees/<kind>: the mesh, Nanite on, its
bark and leaf textures, and an instance of M_Tree_Bark or M_Tree_Leaf on each slot.

Two stages, as megascans.py, because Interchange rewrites a mesh's slots after the import returns:
    OC_STAGE=import bash scripts/ue_run.sh import_trees.py trees_import.log
    OC_STAGE=assign bash scripts/ue_run.sh import_trees.py trees_assign.log
OC_STAGE=wind rebuilds the two masters in place, their instances kept (to change the wind).
OC_ONLY narrows either to a comma list of kinds. The assign stage logs each mesh kept as
"TREES mesh /Game/Trees/<kind>/<name> W x D x H cm", which climb_world.py's trees read back from
the folder itself.

Real trees, to replace the two PVE sample trees whose cards turned to face the camera (the user,
14 Nov: "like they're 2d rotating"). The leaves are still cards, as on any game tree, but fixed in
the tree's own space.
"""

import importlib.util
import json
import os

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/import_trees.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
assets, tools, Graph = lib.assets, lib.tools, lib.Graph
PREP = "/workspace/ue/fab_prep/trees"
ROOT = "/Game/Trees"
STAGE = os.environ.get("OC_STAGE", "import")
ONLY = set(filter(None, (os.environ.get("OC_ONLY") or "").split(",")))


def log(message):
    unreal.log(f"TREES {message}")


def import_texture(path, folder, name, normal):
    task = unreal.AssetImportTask()
    task.filename = path
    task.destination_path = folder
    task.destination_name = name
    task.replace_existing = True
    task.automated = True
    task.save = False
    tools.import_asset_tasks([task])
    tex = unreal.load_asset(f"{folder}/{name}")
    if not isinstance(tex, unreal.Texture):
        raise RuntimeError(f"could not import {path}")
    tex.set_editor_property("srgb", not normal)
    tex.set_editor_property(
        "compression_settings",
        unreal.TextureCompressionSettings.TC_NORMALMAP
        if normal
        else unreal.TextureCompressionSettings.TC_DEFAULT,
    )
    assets.save_loaded_asset(tex, False)


def import_mesh(record, folder):
    """Interchange's default pipeline keeps each part of a pack (trunk, leaves) its own mesh,
    baked into the pack's one frame: climb_world.py places a kind's parts together (the birch
    pack's five trees are each a tree of their own). In 5.8 the combine switch is the mesh
    pipeline's combine_static_meshes_behavior; a probe (14 Nov) found it DO_NOT_COMBINE."""
    task = unreal.AssetImportTask()
    task.filename = record["fbx"]
    task.destination_path = folder
    task.automated = True
    task.replace_existing = True
    task.save = False
    tools.import_asset_tasks([task])


def import_one(record):
    kind = record["kind"]
    folder = f"{ROOT}/{kind}"
    for part in record["parts"]:
        src = f"{PREP}/{kind}/{part}"
        import_texture(f"{src}_D.png", folder, f"T_{kind}_{part}_D", False)
        if part in record["normals"]:
            import_texture(f"{src}_N.png", folder, f"T_{kind}_{part}_N", True)
    import_mesh(record, folder)
    assets.save_directory(folder, only_if_is_dirty=True, recursive=True)
    for path in assets.list_assets(folder, recursive=True):
        asset = unreal.load_asset(path)
        if isinstance(asset, unreal.StaticMesh):
            slots = [
                str(s.material_slot_name) for s in asset.get_editor_property("static_materials")
            ]
            log(f"imported {path} slots {slots}")


def graph_in_place(name):
    """A Graph on the master as it stands, emptied, or a new one: deleting it would orphan the
    instances on every tree's slots."""
    path = f"{ROOT}/Materials/{name}"
    if not assets.does_asset_exist(path):
        return Graph(name, f"{ROOT}/Materials")
    g = Graph.__new__(Graph)
    g.material, g.name, g.y = unreal.load_asset(path), name, 0
    unreal.MaterialEditingLibrary.delete_all_material_expressions(g.material)
    return g


def wind(g, rnd, leaf):
    """The tree swaying: the whole crown leans with slow gusts rolling across the land, more the
    higher it is above the tree's own base (height squared, so the trunk's foot stays put), each
    tree a little out of step. Leaves add a quick flutter of their own. Bark and leaf share the
    lean, or the crown would part from its branches. Faded out from 80 to 140 m: the components
    stop evaluating it at 150 (climb_world.TREE_WIND_TO)."""
    place = g.node(unreal.MaterialExpressionWorldPosition)
    base = g.node(unreal.MaterialExpressionObjectPositionWS)
    up = g.add(g.mask(place, "b"), g.mul(g.mask(base, "b"), -1.0))
    rise = g.saturate(
        g.mul(
            up, g.op(unreal.MaterialExpressionDivide, g.const(1.0), g.param("Wind height", 1800.0))
        )
    )
    time = g.mul(g.node(unreal.MaterialExpressionTime), g.param("Tree wind speed", 0.12))
    along = g.add(g.mul(g.mask(base, "r"), 1.0 / 4100), g.mul(g.mask(base, "g"), 1.0 / 6300))
    phase = g.add(g.add(time, along), g.mul(rnd, 0.35))
    gust = g.link(phase, g.node(unreal.MaterialExpressionSine, period=1.0))
    sway = g.link(g.add(g.mul(phase, 2.7), rnd), g.node(unreal.MaterialExpressionSine, period=1.0))
    # Leaning with the wind (+x, a little +y) and rocking about that lean.
    push = g.add(g.add(g.mul(gust, 0.45), g.mul(sway, 0.25)), 0.55)
    lean = g.mul(g.mul(push, g.mul(rise, rise)), g.param("Tree wind", 22.0))
    offset = g.mul(g.const((0.85, 0.5, 0.0)), lean)
    if leaf:
        quick = g.add(
            g.mul(g.node(unreal.MaterialExpressionTime), g.param("Leaf flutter speed", 1.7)),
            g.op(unreal.MaterialExpressionDotProduct, place, g.const((1 / 140, 1 / 190, 1 / 230))),
        )
        flutter = g.link(quick, g.node(unreal.MaterialExpressionSine, period=1.0))
        shake = g.mul(g.mul(flutter, rise), g.param("Leaf flutter", 3.0))
        offset = g.add(offset, g.mul(g.const((0.4, 0.3, 0.6)), shake))
    eye = g.node(unreal.MaterialExpressionCameraPositionWS)
    far = g.op(unreal.MaterialExpressionDistance, place, eye)
    near = g.saturate(g.add(g.mul(far, -1.0 / 6000), 14000 / 6000))
    offset = g.mul(offset, near)
    if not lib.edit.connect_material_property(
        offset, "", unreal.MaterialProperty.MP_WORLD_POSITION_OFFSET
    ):
        raise RuntimeError(f"{g.name}: could not wire the world position offset")


def master(kind):
    """M_Tree_Bark: opaque. M_Tree_Leaf: masked two-sided foliage, lit through by its own colour.
    Both take D and N and vary each tree's brightness and warmth a little, and sway in the
    wind."""
    name = {"bark": "M_Tree_Bark", "leaf": "M_Tree_Leaf"}[kind]
    g = graph_in_place(name)
    d = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900, parameter_name="D")
    d.set_editor_property("texture", unreal.load_asset("/Engine/EngineResources/DefaultTexture"))
    n = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900, parameter_name="N")
    n.set_editor_property("sampler_type", unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL)
    n.set_editor_property("texture", unreal.load_asset("/Engine/EngineMaterials/DefaultNormal"))
    rnd = g.node(unreal.MaterialExpressionPerInstanceRandom)
    bright = g.add(g.const(1.0), g.mul(g.add(rnd, -0.5), g.param("Variation", 0.3)))
    colour = g.mul(g.mul(g.mask(d, "rgb"), g.mask(g.param("Tint", (1.0, 1.0, 1.0)), "rgb")), bright)
    material = g.material
    if kind == "leaf":
        # One tree in "Warm share" yellower: a wood in summer isn't one green.
        second = g.link(g.mul(rnd, 5.31), g.node(unreal.MaterialExpressionFrac))
        warm = g.saturate(g.mul(g.add(second, g.add(g.param("Warm share", 0.25), -1.0)), 20.0))
        colour = g.lerp(colour, g.mul(colour, g.param("Warm tint", (1.25, 1.1, 0.7))), warm)
        sss = g.mul(colour, g.mask(g.param("TranslucencyTint", (0.6, 0.7, 0.35)), "rgb"))
        g.output(
            colour,
            g.mask(n, "rgb"),
            g.param("Roughness", 0.7),
            specular=g.param("Specular", 0.3),
            opacity_mask=g.mask(d, "a"),
            subsurface=sss,
        )
        material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_MASKED)
        material.set_editor_property(
            "shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE
        )
        material.set_editor_property("two_sided", True)
        material.set_editor_property("opacity_mask_clip_value", 0.4)
    else:
        g.output(colour, g.mask(n, "rgb"), g.param("Roughness", 0.85), specular=0.3)
    wind(g, rnd, kind == "leaf")
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    # Nanite culls by the mesh's bounds, so it needs to know how far the wind can move it.
    material.set_editor_property("max_world_position_offset_displacement", 60.0)
    return g.finish()


def part_of(record, slot):
    slots = record["slots"]
    if slot in slots:
        return slots[slot]
    for name, part in slots.items():
        if name.lower().strip("_") in slot.lower() or slot.lower().strip("_") in name.lower():
            return part
    log(f"{record['kind']}: slot {slot!r} not in {list(slots)}; bark")
    return "bark"


def assign_one(record, masters):
    kind = record["kind"]
    folder = f"{ROOT}/{kind}"
    mis = {}
    for part in record["parts"]:
        textures = {"D": unreal.load_asset(f"{folder}/T_{kind}_{part}_D")}
        if part in record["normals"]:
            textures["N"] = unreal.load_asset(f"{folder}/T_{kind}_{part}_N")
        parent = masters["leaf" if part == "leaf" else "bark"]
        mis[part] = lib.instance(f"MI_{kind}_{part}", parent, folder, textures)
    ours = {f"{folder}/T_{kind}_{p}_{m}" for p in record["parts"] for m in ("D", "N")}
    ours |= {mi.get_path_name().split(".")[0] for mi in mis.values()}
    stray = []
    kept = 0
    for path in assets.list_assets(folder, recursive=True):
        asset = unreal.load_asset(path)
        bare = path.split(".")[0]
        if isinstance(asset, unreal.StaticMesh):
            if record.get("drop") and record["drop"] in asset.get_name():
                stray.append(path)  # the pack's own lower LODs: Nanite makes its own
                continue
            slots = [
                unreal.StaticMaterial(
                    mis[part_of(record, str(s.material_slot_name))],
                    s.material_slot_name,
                    s.uv_channel_data,
                )
                for s in asset.get_editor_property("static_materials")
            ]
            asset.set_editor_property("static_materials", slots)
            settings = asset.get_editor_property("nanite_settings")
            settings.set_editor_property("enabled", True)
            if "leaf" in mis:
                settings.set_editor_property(
                    "shape_preservation", unreal.NaniteShapePreservation.PRESERVE_AREA
                )
            asset.set_editor_property("nanite_settings", settings)
            assets.save_loaded_asset(asset, False)
            box = asset.get_bounding_box()
            size = box.max - box.min
            log(
                f"mesh {bare} {size.x:.0f}x{size.y:.0f}x{size.z:.0f} cm "
                f"base {box.min.z:.0f} slots {[str(s.material_slot_name) for s in slots]}"
            )
            kept += 1
        elif bare not in ours and isinstance(asset, (unreal.MaterialInterface, unreal.Texture)):
            stray.append(path)  # what Interchange made from the FBX's own materials
    for path in stray:
        assets.delete_asset(path)
    if not kept:
        raise RuntimeError(f"{kind}: no mesh in {folder}")
    log(f"assigned {folder}: {kept} meshes, {len(stray)} strays removed")


def main():
    with open(f"{PREP}/index.json") as f:
        records = [r for r in json.load(f) if not ONLY or r["kind"] in ONLY]
    log(f"{STAGE}: {[r['kind'] for r in records]}")
    if STAGE == "import":
        for record in records:
            import_one(record)
    elif STAGE == "wind":
        # The masters rebuilt where they stand (their wind added), every instance kept.
        for kind in ("bark", "leaf"):
            master(kind)
    elif STAGE == "assign":
        masters = {
            kind: unreal.load_asset(path) if assets.does_asset_exist(path) else master(kind)
            for kind, path in (
                ("bark", f"{ROOT}/Materials/M_Tree_Bark"),
                ("leaf", f"{ROOT}/Materials/M_Tree_Leaf"),
            )
        }
        for record in records:
            assign_one(record, masters)
    else:
        raise RuntimeError(f"unknown OC_STAGE {STAGE}")


main()
