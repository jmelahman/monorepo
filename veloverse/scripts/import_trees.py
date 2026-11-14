"""Imports the trees prep_trees.py prepared into /Game/Trees/<kind>: the mesh, Nanite on, its
bark and leaf textures, and an instance of M_Tree_Bark or M_Tree_Leaf on each slot.

Two stages, as megascans.py, because Interchange rewrites a mesh's slots after the import returns:
    OC_STAGE=import bash scripts/ue_run.sh import_trees.py trees_import.log
    OC_STAGE=assign bash scripts/ue_run.sh import_trees.py trees_assign.log
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
# Each pack whose tree comes in parts (trunk, leaves) is one mesh; the birch pack is five trees.
SEPARATE = {"birch"}


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
    """Through Interchange with the default pipeline, its meshes combined unless the pack is
    several trees; through a plain import task if this engine's Python lacks the pipeline."""
    path = record["fbx"]
    combine = record["kind"] not in SEPARATE
    try:
        pipeline = unreal.load_asset("/Interchange/Pipelines/DefaultAssetsPipeline")
        pipeline.get_editor_property("mesh_pipeline").set_editor_property(
            "combine_static_meshes", combine
        )
        params = unreal.ImportAssetParameters()
        params.set_editor_property("is_automated", True)
        params.set_editor_property("replace_existing", True)
        params.get_editor_property("override_pipelines").append(
            unreal.SoftObjectPath(pipeline.get_path_name())
        )
        manager = unreal.InterchangeManager.get_interchange_manager_scripted()
        source = unreal.InterchangeManager.create_source_data(path)
        manager.import_asset(folder, source, params)
        log(f"{record['kind']}: Interchange, combine {combine}")
    except Exception as e:  # any failure of the override falls back
        log(f"{record['kind']}: no Interchange override ({e}); a plain import")
        task = unreal.AssetImportTask()
        task.filename = path
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


def master(kind):
    """M_Tree_Bark: opaque. M_Tree_Leaf: masked two-sided foliage, lit through by its own colour.
    Both take D and N and vary each tree's brightness and warmth a little."""
    name = {"bark": "M_Tree_Bark", "leaf": "M_Tree_Leaf"}[kind]
    g = Graph(name, f"{ROOT}/Materials")
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
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
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
