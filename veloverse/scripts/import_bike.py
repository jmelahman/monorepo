"""Imports make_bike.py's bike and cockpit into /Game/Rider and paints their parts.

Makes M_Rider (a two-sided master with colour, roughness, metallic, specular and a glow for the
computer's screen), one instance per part, and SM_Bike and SM_Cockpit with each glTF material slot
given its part's instance. RidePawn loads the meshes by path; DefaultGame.ini cooks /Game/Rider.

Run: scripts/ue_run.sh import_bike.py LOG (after make_bike.py). OC_BIKE_OUT is make_bike's folder.
"""

import importlib.util
import os

import unreal

SRC = os.environ.get("OC_BIKE_OUT", "/workspace/ue/world_gen/rider")
FOLDER = "/Game/Rider"
here = os.path.dirname(os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/x.py"))
spec = importlib.util.spec_from_file_location("ue_lib", os.path.join(here, "ue_lib.py"))
lib = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lib)

# Part: (linear colour, roughness, metallic, specular, glow).
PARTS = {
    "paint": ((0.012, 0.045, 0.032), 0.28, 0.0, 0.6, 0.0),  # deep green, clear-coated
    "carbon": ((0.012, 0.012, 0.013), 0.32, 0.0, 0.5, 0.0),
    "tyre": ((0.016, 0.015, 0.014), 0.78, 0.0, 0.3, 0.0),
    "alloy": ((0.42, 0.43, 0.45), 0.38, 1.0, 0.5, 0.0),
    "steel": ((0.55, 0.56, 0.58), 0.25, 1.0, 0.5, 0.0),
    "chainring": ((0.08, 0.08, 0.085), 0.4, 1.0, 0.5, 0.0),
    "chain": ((0.3, 0.3, 0.31), 0.45, 1.0, 0.5, 0.0),
    "spoke": ((0.02, 0.02, 0.022), 0.4, 0.6, 0.5, 0.0),
    "tape": ((0.018, 0.018, 0.018), 0.85, 0.0, 0.3, 0.0),
    "hood": ((0.022, 0.022, 0.022), 0.7, 0.0, 0.35, 0.0),
    "saddle": ((0.016, 0.016, 0.016), 0.6, 0.0, 0.4, 0.0),
    "pedal": ((0.02, 0.02, 0.02), 0.5, 0.0, 0.4, 0.0),
    "cage": ((0.02, 0.02, 0.02), 0.45, 0.0, 0.4, 0.0),
    "bottle": ((0.62, 0.58, 0.5), 0.5, 0.0, 0.4, 0.0),
    "screen": ((0.005, 0.02, 0.02), 0.15, 0.0, 0.5, 0.6),
}


def master():
    g = lib.Graph("M_Rider", FOLDER)
    colour = g.param("Colour", (0.5, 0.5, 0.5))
    glow = g.param("Glow", 0.0)
    g.output(
        colour,
        rough=g.param("Rough", 0.5),
        metallic=g.param("Metallic", 0.0),
        specular=g.param("Specular", 0.5),
        emissive=g.op(unreal.MaterialExpressionMultiply, colour, glow),
    )
    return g.finish(two_sided=True)


def import_mesh(name, file):
    task = unreal.AssetImportTask()
    task.filename = f"{SRC}/{file}"
    task.destination_path = FOLDER
    task.destination_name = name
    task.replace_existing = True
    task.automated = True
    task.save = False
    lib.tools.import_asset_tasks([task])
    imported = [unreal.load_asset(p) for p in task.get_editor_property("imported_object_paths")]
    mesh = next(a for a in imported if isinstance(a, unreal.StaticMesh))
    return mesh, [a for a in imported if isinstance(a, unreal.MaterialInterface)]


def main():
    parent = master()
    paints = {
        part: lib.instance(
            f"MI_Rider_{part}",
            parent,
            FOLDER,
            scalars={"Rough": rough, "Metallic": metal, "Specular": spec_, "Glow": glow},
            vectors={"Colour": (*colour, 1.0)},
        )
        for part, (colour, rough, metal, spec_, glow) in PARTS.items()
    }
    for name, file in (("SM_Bike", "bike.gltf"), ("SM_Cockpit", "cockpit.gltf")):
        mesh, strays = import_mesh(name, file)
        nanite = mesh.get_editor_property("nanite_settings")
        nanite.set_editor_property("enabled", False)
        mesh.set_editor_property("nanite_settings", nanite)
        missing = []
        for i, slot in enumerate(mesh.get_editor_property("static_materials")):
            label = str(slot.get_editor_property("material_slot_name"))
            part = next((p for p in paints if label == p or label.startswith(f"{p}_")), None)
            if part is None:
                missing.append(label)
                continue
            mesh.set_material(i, paints[part])
        lib.assets.save_loaded_asset(mesh, False)
        # The glTF import also made a material per slot; ours replaced them.
        mine = {m.get_path_name() for m in paints.values()}
        for stray in strays:
            if stray.get_path_name() not in mine:
                lib.assets.delete_asset(stray.get_path_name())
        box = mesh.get_bounding_box()
        lib.log(
            f"BIKE {name}: {len(mesh.get_editor_property('static_materials'))} slots, "
            f"bounds {box.min} .. {box.max}, unpainted {missing or 'none'}"
        )
    lib.assets.save_directory(FOLDER, only_if_is_dirty=False, recursive=True)
    lib.log("BIKE done")


main()
