"""Imports the Blender bike (bike/build_bike.py's bike.fbx) into /Game/Rider/Bike, a mesh a part.

    OC_STAGE=import bash scripts/ue_run.sh import_bike_parts.py bike_import.log
    OC_STAGE=assign bash scripts/ue_run.sh import_bike_parts.py bike_assign.log

Two stages, as import_trees.py, because Interchange rewrites a mesh's slots after import returns.
The FBX is regenerated, not committed: OC_BIKE_FBX names its folder (default
/workspace/ue/fab_prep/bike), which also holds the T_*.png textures.

Each part (SM_Bike_Frame, _Fork, _WheelFront, _WheelRear, _Cranks, _PedalL, _PedalR, _Chain) keeps
the bike's frame: origin on the ground under the bottom bracket, +X forward, the drive side +Y.
RidePawn turns the moving ones about their pivots (bike/README.md) and logs nothing if they're
missing: it falls back to make_bike.py's SM_Bike. The assign stage logs each part's bounds as
"BIKE part NAME centre (x, y, z) size (x, y, z)", to check the pivots and the drive side.
"""

import importlib.util
import os

import unreal

here = os.path.dirname(os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/x.py"))
spec = importlib.util.spec_from_file_location("ue_lib", os.path.join(here, "ue_lib.py"))
lib = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lib)

SRC = os.environ.get("OC_BIKE_FBX", "/workspace/ue/fab_prep/bike")
FOLDER = "/Game/Rider/Bike"
STAGE = os.environ.get("OC_STAGE", "import")
PARTS = ("Frame", "Fork", "WheelFront", "WheelRear", "Cranks", "PedalL", "PedalR", "Chain")

# Slot: (linear colour, roughness, metallic, specular), from build_bike.py's make_materials.
FLAT = {
    "M_FramePaint": ((0.010, 0.024, 0.075), 0.32, 0.75, 0.5),
    "M_Decal": ((0.80, 0.22, 0.05), 0.35, 0.0, 0.5),
    "M_Alloy": ((0.80, 0.80, 0.80), 0.28, 1.0, 0.5),
    "M_BlackPlastic": ((0.016, 0.016, 0.017), 0.55, 0.0, 0.4),
    "M_BarTape": ((0.012, 0.012, 0.012), 0.85, 0.0, 0.3),
}
# Slot: (texture stem, tint, roughness, metallic): the colour is the texture's times the tint.
TEXTURED = {
    "M_Carbon": ("Carbon", (1.0, 1.0, 1.0), 0.32, 0.0),
    "M_Rubber": ("Rubber", (1.0, 1.0, 1.0), 0.78, 0.0),
}


def log(message):
    unreal.log(f"BIKE {message}")


def texture(stem, kind):
    name = f"T_Bike_{stem}_{kind}"
    task = unreal.AssetImportTask()
    task.filename = f"{SRC}/T_{stem}_{'BaseColor' if kind == 'D' else 'Normal'}.png"
    task.destination_path = FOLDER
    task.destination_name = name
    task.replace_existing = True
    task.automated = True
    task.save = False
    lib.tools.import_asset_tasks([task])
    tex = unreal.load_asset(f"{FOLDER}/{name}")
    if not isinstance(tex, unreal.Texture):
        raise RuntimeError(f"could not import {task.filename}")
    normal = kind == "N"
    tex.set_editor_property("srgb", not normal)
    if normal:
        tex.set_editor_property("flip_green_channel", True)  # Blender's are OpenGL (Y+)
        tex.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_NORMALMAP
        )
    lib.assets.save_loaded_asset(tex, False)
    return tex


def import_stage():
    if lib.assets.does_directory_exist(FOLDER):
        lib.assets.delete_directory(FOLDER)
    for stem in ("Carbon", "Rubber"):
        texture(stem, "D")
        texture(stem, "N")
    task = unreal.AssetImportTask()
    task.filename = f"{SRC}/bike.fbx"
    task.destination_path = FOLDER
    task.automated = True
    task.replace_existing = True
    task.save = False
    lib.tools.import_asset_tasks([task])
    lib.assets.save_directory(FOLDER, only_if_is_dirty=True, recursive=True)
    for path in lib.assets.list_assets(FOLDER, recursive=True):
        asset = unreal.load_asset(path)
        log(f"imported {path} ({type(asset).__name__})")


def masters():
    flat = lib.Graph("M_BikeFlat", FOLDER)
    flat.output(
        flat.param("Colour", (0.5, 0.5, 0.5)),
        rough=flat.param("Rough", 0.5),
        metallic=flat.param("Metallic", 0.0),
        specular=flat.param("Specular", 0.5),
    )
    flat.material.set_editor_property("used_with_nanite", True)
    flat = flat.finish()
    g = lib.Graph("M_BikeTex", FOLDER)
    d = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900, parameter_name="D")
    d.set_editor_property("texture", unreal.load_asset("/Engine/EngineResources/DefaultTexture"))
    n = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900, parameter_name="N")
    n.set_editor_property("sampler_type", unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL)
    n.set_editor_property("texture", unreal.load_asset("/Engine/EngineMaterials/DefaultNormal"))
    g.output(
        g.mul(g.mask(d, "rgb"), g.mask(g.param("Tint", (1.0, 1.0, 1.0)), "rgb")),
        g.mask(n, "rgb"),
        g.param("Rough", 0.5),
        specular=0.5,
        metallic=g.param("Metallic", 0.0),
    )
    g.material.set_editor_property("used_with_nanite", True)
    return flat, g.finish()


def assign_stage():
    flat, tex = masters()
    mis = {}
    for slot, (colour, rough, metal, spec_) in FLAT.items():
        mis[slot] = lib.instance(
            f"MI_Bike_{slot[2:]}",
            flat,
            FOLDER,
            scalars={"Rough": rough, "Metallic": metal, "Specular": spec_},
            vectors={"Colour": (*colour, 1.0)},
        )
    for slot, (stem, tint, rough, metal) in TEXTURED.items():
        mis[slot] = lib.instance(
            f"MI_Bike_{slot[2:]}",
            tex,
            FOLDER,
            textures={
                "D": unreal.load_asset(f"{FOLDER}/T_Bike_{stem}_D"),
                "N": unreal.load_asset(f"{FOLDER}/T_Bike_{stem}_N"),
            },
            scalars={"Rough": rough, "Metallic": metal},
            vectors={"Tint": (*tint, 1.0)},
        )
    ours = {m.get_path_name().split(".")[0] for m in mis.values()}
    ours |= {f"{FOLDER}/T_Bike_{s}_{k}" for s in ("Carbon", "Rubber") for k in ("D", "N")}
    ours |= {f"{FOLDER}/{name}" for name in ("M_BikeFlat", "M_BikeTex")}
    found, stray = set(), []
    for path in lib.assets.list_assets(FOLDER, recursive=True):
        asset = unreal.load_asset(path)
        bare = path.split(".")[0]
        if isinstance(asset, unreal.StaticMesh):
            name = asset.get_name()
            part = next((p for p in PARTS if name == p or name.endswith(f"_{p}")), None)
            if part is None:
                stray.append(path)
                log(f"dropped {path}: not a part")
                continue
            slots, unpainted = [], []
            for s in asset.get_editor_property("static_materials"):
                label = str(s.material_slot_name)
                mi = next((mis[k] for k in mis if label == k or label.startswith(k)), None)
                if mi is None:
                    unpainted.append(label)
                    mi = mis["M_BlackPlastic"]
                slots.append(unreal.StaticMaterial(mi, s.material_slot_name, s.uv_channel_data))
            asset.set_editor_property("static_materials", slots)
            nanite = asset.get_editor_property("nanite_settings")
            nanite.set_editor_property("enabled", True)
            asset.set_editor_property("nanite_settings", nanite)
            lib.assets.save_loaded_asset(asset, False)
            final = f"{FOLDER}/SM_Bike_{part}"
            if bare != final and not lib.assets.rename_asset(bare, final):
                raise RuntimeError(f"could not move {bare} to {final}")
            box = unreal.load_asset(final).get_bounding_box()
            c, s = (box.min + box.max) * 0.5, box.max - box.min
            log(
                f"part {part} centre ({c.x:.1f}, {c.y:.1f}, {c.z:.1f}) "
                f"size ({s.x:.1f}, {s.y:.1f}, {s.z:.1f}) unpainted {unpainted or 'none'}"
            )
            found.add(part)
        elif bare not in ours and isinstance(asset, (unreal.MaterialInterface, unreal.Texture)):
            stray.append(path)  # what Interchange made from the FBX's own materials
    for path in stray:
        lib.assets.delete_asset(path)
    lib.assets.save_directory(FOLDER, only_if_is_dirty=False, recursive=True)
    missing = [p for p in PARTS if p not in found]
    if missing:
        raise RuntimeError(f"bike parts missing: {missing}")
    log(f"assigned {len(found)} parts, {len(stray)} strays removed")


if STAGE == "import":
    import_stage()
elif STAGE == "assign":
    assign_stage()
else:
    raise RuntimeError(f"unknown OC_STAGE {STAGE}")
