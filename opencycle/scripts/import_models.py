"""Imports glTF files into the project as static meshes, with Nanite on.

OC_IMPORT is a ';'-separated list of FILE=DEST_FOLDER pairs, e.g.
    /workspace/ue/assets_ph/models/fern_02/fern_02_1k.gltf=/Game/Coast/PH/fern_02
Prints one IMPORT line per file with the meshes made and their bounds in cm.
OC_NANITE=0 leaves Nanite off (the generated terrain keeps it on by default).
"""

import os

import unreal

tools = unreal.AssetToolsHelpers.get_asset_tools()
nanite = (os.environ.get("OC_NANITE") or "1") != "0"


def import_one(path, folder):
    task = unreal.AssetImportTask()
    task.set_editor_property("filename", path)
    task.set_editor_property("destination_path", folder)
    task.set_editor_property("automated", True)
    task.set_editor_property("replace_existing", True)
    task.set_editor_property("save", False)
    tools.import_asset_tasks([task])
    meshes = []
    for asset_path in unreal.EditorAssetLibrary.list_assets(folder, recursive=True):
        asset = unreal.load_asset(asset_path)
        if isinstance(asset, unreal.StaticMesh):
            meshes.append(asset)
            settings = asset.get_editor_property("nanite_settings")
            if settings.get_editor_property("enabled") != nanite:
                settings.set_editor_property("enabled", nanite)
                asset.set_editor_property("nanite_settings", settings)
    unreal.EditorAssetLibrary.save_directory(folder, only_if_is_dirty=False, recursive=True)
    for mesh in meshes:
        box = mesh.get_bounding_box()
        slots = [
            str(m.get_editor_property("material_slot_name"))
            for m in mesh.get_editor_property("static_materials")
        ]
        unreal.log(f"IMPORT {mesh.get_path_name()} min {box.min} max {box.max} slots {slots}")
    if not meshes:
        unreal.log_error(f"IMPORT {path}: no static mesh in {folder}")


for pair in (os.environ.get("OC_IMPORT") or "").split(";"):
    if pair.strip():
        source, folder = pair.split("=", 1)
        import_one(source.strip(), folder.strip())
