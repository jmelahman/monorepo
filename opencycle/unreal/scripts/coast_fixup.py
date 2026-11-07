"""Assigns the coast's materials to imported mesh slots, after Interchange has finished with them.

Interchange finishes an import asynchronously and rewrites a mesh's slots with the materials it
made, so slots set straight after import don't stick. This pass runs on its own:
    Gen meshes   slot X -> /Game/Coast/Materials/M_X (BambooLeaves -> MI_BambooLeaves, RailPost ->
    M_Rail)
    PH meshes    slot X -> <model>/Foliage/MI_X where one exists
and then gives any parent material still used the Nanite and instanced-mesh usage flags, since in
-game a material without them falls back to the default checker.
"""

import unreal

assets = unreal.EditorAssetLibrary
ROOT = "/Game/Coast"
SPECIAL = {
    "BambooLeaves": f"{ROOT}/Gen/Bamboo/MI_BambooLeaves",
    "MomijiGreen": f"{ROOT}/Gen/MomijiGreen/MI_MomijiGreenLeaves",
    "MomijiRed": f"{ROOT}/Gen/MomijiRed/MI_MomijiRedLeaves",
    "SugiLeaves": f"{ROOT}/Gen/Sugi/MI_SugiLeaves",
    "RailPost": f"{ROOT}/Materials/M_Rail",
}


def log(message):
    unreal.log(f"FIX {message}")


def meshes(folder):
    for path in assets.list_assets(folder, recursive=True):
        if (
            path.split(".")[-1]
            and assets.find_asset_data(path).asset_class_path.asset_name == "StaticMesh"
        ):
            yield unreal.load_asset(path)


def model_folder(mesh):
    parts = mesh.get_path_name().split("/")
    return "/".join(parts[:5])  # /Game/Coast/PH/<model>


changed = 0
used = set()
for mesh in list(meshes(f"{ROOT}/Gen")) + list(meshes(f"{ROOT}/PH")):
    slots = mesh.get_editor_property("static_materials")
    dirty = False
    for index, slot in enumerate(slots):
        name = str(slot.get_editor_property("material_slot_name"))
        if "/Gen/" in mesh.get_path_name():
            target = SPECIAL.get(name, f"{ROOT}/Materials/M_{name}")
        else:
            target = f"{model_folder(mesh)}/Foliage/MI_{name}"
        current = slot.get_editor_property("material_interface")
        if assets.does_asset_exist(target):
            material = unreal.load_asset(target)
            if current != material:
                mesh.set_material(index, material)
                dirty = True
            used.add(material)
        else:
            if "/Gen/" in mesh.get_path_name():
                log(f"no material {target} for {mesh.get_name()}")
            if current:
                used.add(current)
    if dirty:
        assets.save_loaded_asset(mesh, False)
        changed += 1
log(f"reassigned slots on {changed} meshes")

# Usage flags on every base material behind what is used.
bases = {}
for material in used:
    base = material.get_base_material() if hasattr(material, "get_base_material") else material
    if base:
        bases[base.get_path_name()] = base
flagged = 0
for path, base in sorted(bases.items()):
    need = not base.get_editor_property("used_with_nanite") or not base.get_editor_property(
        "used_with_instanced_static_meshes"
    )
    if need:
        base.set_editor_property("used_with_nanite", True)
        base.set_editor_property("used_with_instanced_static_meshes", True)
        unreal.MaterialEditingLibrary.recompile_material(base)
        ok = assets.save_loaded_asset(base, False)
        flagged += 1
        log(f"flags on {path} saved={ok}")
log(f"bases {len(bases)}, flagged {flagged}")
log("done")
