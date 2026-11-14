"""Imports the Blender rider (rider/build_rider.py's rider.fbx) into /Game/Rider/Body.

    OC_STAGE=import bash scripts/ue_run.sh import_rider.py rider_import.log
    OC_STAGE=assign bash scripts/ue_run.sh import_rider.py rider_assign.log

Two stages, as import_bike_parts.py, because Interchange rewrites a mesh's slots after import
returns. The FBX is regenerated, not committed: OC_RIDER_FBX names its folder (default
/workspace/ue/fab_prep/rider), which also holds the T_*.png textures.

The skeletal mesh lands as SK_Rider and its one take as A_Rider_Pedal: a crank revolution, frames
0-59 at 30 fps, which RidePawn scrubs to its crank angle. The rider keeps the bike's frame (the
ground under the bottom bracket, +X forward, the drive side +Y), so it sits under Bike at the bike
parts' origin. The assign stage logs the mesh's bounds, bones and the take's length.
"""

import importlib.util
import os

import unreal

here = os.path.dirname(os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/x.py"))
spec = importlib.util.spec_from_file_location("ue_lib", os.path.join(here, "ue_lib.py"))
lib = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lib)

SRC = os.environ.get("OC_RIDER_FBX", "/workspace/ue/fab_prep/rider")
FOLDER = "/Game/Rider/Body"
STAGE = os.environ.get("OC_STAGE", "import")
STEMS = ("Body", "Helmet", "Shoe")

# Slot: (linear colour, roughness, metallic), from rider/README.md's materials table.
FLAT = {
    "M_Lens": ((0.010, 0.011, 0.014), 0.05, 0.55),
    "M_Strap": ((0.012, 0.012, 0.013), 0.70, 0.0),
    "M_Hair": ((0.030, 0.019, 0.012), 0.60, 0.0),
}
# Slot: (texture stem, roughness). The kit is painted on the body, so it shares the skin's maps.
TEXTURED = {
    "M_Skin": ("Body", 0.50),
    "M_Kit": ("Body", 0.62),
    "M_Helmet": ("Helmet", 0.30),
    "M_Shoe": ("Shoe", 0.38),
}


def log(message):
    unreal.log(f"RIDER {message}")


def texture(stem, kind):
    name = f"T_Rider_{stem}_{kind}"
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
    for stem in STEMS:
        texture(stem, "D")
        texture(stem, "N")
    task = unreal.AssetImportTask()
    task.filename = f"{SRC}/rider.fbx"
    task.destination_path = FOLDER
    task.automated = True
    task.replace_existing = True
    task.save = False
    lib.tools.import_asset_tasks([task])
    lib.assets.save_directory(FOLDER, only_if_is_dirty=True, recursive=True)
    for path in lib.assets.list_assets(FOLDER, recursive=True):
        log(f"imported {path} ({type(unreal.load_asset(path)).__name__})")


def masters():
    flat = lib.Graph("M_RiderFlat", FOLDER)
    flat.output(
        flat.param("Colour", (0.5, 0.5, 0.5)),
        rough=flat.param("Rough", 0.5),
        metallic=flat.param("Metallic", 0.0),
        specular=0.5,
    )
    flat.material.set_editor_property("used_with_skeletal_mesh", True)
    flat = flat.finish()
    g = lib.Graph("M_RiderTex", FOLDER)
    d = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900, parameter_name="D")
    d.set_editor_property("texture", unreal.load_asset("/Engine/EngineResources/DefaultTexture"))
    n = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900, parameter_name="N")
    n.set_editor_property("sampler_type", unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL)
    n.set_editor_property("texture", unreal.load_asset("/Engine/EngineMaterials/DefaultNormal"))
    g.output(g.mask(d, "rgb"), g.mask(n, "rgb"), g.param("Rough", 0.5), specular=0.5)
    g.material.set_editor_property("used_with_skeletal_mesh", True)
    return flat, g.finish()


def assign_stage():
    flat, tex = masters()
    mis = {}
    for slot, (colour, rough, metal) in FLAT.items():
        mis[slot] = lib.instance(
            f"MI_Rider_{slot[2:]}",
            flat,
            FOLDER,
            scalars={"Rough": rough, "Metallic": metal},
            vectors={"Colour": (*colour, 1.0)},
        )
    for slot, (stem, rough) in TEXTURED.items():
        mis[slot] = lib.instance(
            f"MI_Rider_{slot[2:]}",
            tex,
            FOLDER,
            textures={
                "D": unreal.load_asset(f"{FOLDER}/T_Rider_{stem}_D"),
                "N": unreal.load_asset(f"{FOLDER}/T_Rider_{stem}_N"),
            },
            scalars={"Rough": rough},
        )
    ours = {m.get_path_name().split(".")[0] for m in mis.values()}
    ours |= {f"{FOLDER}/T_Rider_{s}_{k}" for s in STEMS for k in ("D", "N")}
    ours |= {f"{FOLDER}/{name}" for name in ("M_RiderFlat", "M_RiderTex")}
    meshes, takes, stray = [], [], []
    for path in lib.assets.list_assets(FOLDER, recursive=True):
        asset = unreal.load_asset(path)
        bare = path.split(".")[0]
        if isinstance(asset, unreal.SkeletalMesh):
            meshes.append((bare, asset))
        elif isinstance(asset, unreal.AnimSequence):
            takes.append((bare, asset))
        elif bare not in ours and isinstance(asset, (unreal.MaterialInterface, unreal.Texture)):
            stray.append(path)  # what Interchange made from the FBX's own materials
    if len(meshes) != 1:
        raise RuntimeError(f"expected one skeletal mesh, found {[m for m, _ in meshes]}")
    pedal = [t for t in takes if "pedal" in t[0].lower()] or takes
    if len(pedal) != 1:
        raise RuntimeError(f"expected one Pedal take, found {[t for t, _ in takes]}")
    bare, mesh = meshes[0]
    slots, unpainted = [], []
    for s in mesh.get_editor_property("materials"):
        label = str(s.material_slot_name)
        mi = next((mis[k] for k in mis if label == k or label.startswith(k)), None)
        if mi is None:
            unpainted.append(label)
            mi = mis["M_Strap"]
        s.set_editor_property("material_interface", mi)
        slots.append(s)
    mesh.set_editor_property("materials", slots)
    lib.assets.save_loaded_asset(mesh, False)
    for path in stray:
        lib.assets.delete_asset(path)
    for old, new in ((bare, "SK_Rider"), (pedal[0][0], "A_Rider_Pedal")):
        if old != f"{FOLDER}/{new}" and not lib.assets.rename_asset(old, f"{FOLDER}/{new}"):
            raise RuntimeError(f"could not move {old} to {FOLDER}/{new}")
    lib.assets.save_directory(FOLDER, only_if_is_dirty=False, recursive=True)
    mesh = unreal.load_asset(f"{FOLDER}/SK_Rider")
    take = unreal.load_asset(f"{FOLDER}/A_Rider_Pedal")
    box = mesh.get_bounds()
    c, e = box.origin, box.box_extent
    log(
        f"mesh centre ({c.x:.1f}, {c.y:.1f}, {c.z:.1f}) size ({2 * e.x:.1f}, {2 * e.y:.1f}, "
        f"{2 * e.z:.1f}) slots {len(slots)} unpainted {unpainted or 'none'}"
    )
    log(f"Pedal {take.get_play_length():.3f} s, {len(stray)} strays removed")


if STAGE == "import":
    import_stage()
elif STAGE == "assign":
    assign_stage()
else:
    raise RuntimeError(f"unknown OC_STAGE {STAGE}")
