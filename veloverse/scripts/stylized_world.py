"""Builds /Game/Stylized/L_Stylized from make_stylized.py's meshes: the stylized test valley.

Imports the meshes (Nanite off: they are light), gives them one vertex-colour material, stands
the props as instances, and writes the road as the level's ride track ("Valley"). The sky, sun,
sky light and fog are the default template's; its volumetric clouds go, as the valley has its own.
Run in the editor after make_stylized.py:
    OC_EDITOR=1 scripts/ue_run.sh stylized_world.py out/stylized_world.log
"""

import importlib.util
import json
import os
import traceback

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/stylized_world.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
assets, actors, edit = lib.assets, lib.actors, lib.edit
SRC = "/workspace/ue/world_gen/stylized"
FOLDER = "/Game/Stylized"
LEVEL = f"{FOLDER}/L_Stylized"
# Draw distances in cm: the woods fade out past 1.6 km, small things sooner.
CULL = {"bush": 60000, "rock": 80000, "fence": 50000, "cloud": 0}


def log(message):
    unreal.log(f"STYLIZED {message}")


def material():
    """Vertex RGB is the colour, alpha the roughness; a little less specular than the default."""
    m = lib.fresh("M_Stylized", FOLDER, unreal.Material, unreal.MaterialFactoryNew())
    vc = edit.create_material_expression(m, unreal.MaterialExpressionVertexColor, -400, 0)
    spec = edit.create_material_expression(m, unreal.MaterialExpressionConstant, -400, 200)
    spec.set_editor_property("r", 0.3)
    edit.connect_material_property(vc, "", unreal.MaterialProperty.MP_BASE_COLOR)
    edit.connect_material_property(vc, "A", unreal.MaterialProperty.MP_ROUGHNESS)
    edit.connect_material_property(spec, "", unreal.MaterialProperty.MP_SPECULAR)
    edit.recompile_material(m)
    assets.save_loaded_asset(m, False)
    return m


def mesh(name, path, mat):
    folder = f"{FOLDER}/Meshes/{name}"
    if assets.does_directory_exist(folder):
        assets.delete_directory(folder)
    made = [a for a in lib.import_file(path, folder) if isinstance(a, unreal.StaticMesh)]
    if not made:
        raise RuntimeError(f"no static mesh from {path}")
    m = made[0]
    settings = m.get_editor_property("nanite_settings")
    settings.set_editor_property("enabled", False)
    m.set_editor_property("nanite_settings", settings)
    m.set_material(0, mat)
    assets.save_loaded_asset(m, False)
    return m


def track(centre):
    cls = unreal.load_class(None, "/Script/VeloverseUE.RideTrack")
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", cls)
    t = lib.fresh("DA_RideTrack", f"{FOLDER}/Ride", cls, factory)
    t.set_editor_property(
        "centre", [unreal.Vector(x * 100, y * 100, z * 100 - 6) for x, y, z in centre]
    )
    t.set_editor_property("closed", True)
    t.set_editor_property("title", "Valley")
    t.set_editor_property("blurb", "Round the stylized valley, past the farm and the lighthouse.")
    t.set_editor_property("step", 100.0)
    t.set_editor_property("keep_left", 160.0)
    t.set_editor_property("sun_azimuth", -62.0)
    coast = unreal.load_asset("/Game/Coast/Ride/DA_RideTrack")
    for prop in ("rain_shell", "collection"):
        t.set_editor_property(prop, coast.get_editor_property(prop))
    row = unreal.RideWeather()
    row.set_editor_property("metres", 0.0)
    t.set_editor_property("weather", [row])
    stretch = unreal.RideStretch()
    stretch.set_editor_property("name", "Valley")
    t.set_editor_property("stretches", [stretch])
    assets.save_loaded_asset(t, False)
    log(f"track: {len(centre)} m")


def main():
    mat = material()
    big = {n: mesh(n, f"{SRC}/{n}.gltf", mat) for n in ("terrain", "sea", "road")}
    props = {
        f[:-5]: mesh(f[:-5], f"{SRC}/props/{f}", mat)
        for f in sorted(os.listdir(f"{SRC}/props"))
        if f.endswith(".gltf")
    }
    log(f"meshes: {len(big) + len(props)}")
    if assets.does_asset_exist(LEVEL):
        assets.delete_asset(LEVEL)
    world = unreal.EditorLoadingAndSavingUtils.new_map_from_template(
        "/Engine/Maps/Templates/Template_Default", False
    )
    for actor in actors.get_all_level_actors():
        if isinstance(actor, unreal.VolumetricCloud) or (
            isinstance(actor, unreal.StaticMeshActor) and "Floor" in actor.get_actor_label()
        ):
            actors.destroy_actor(actor)
    for name, m in big.items():
        lib.mesh_actor(m, label=name.title(), folder="Ground")
    with open(f"{SRC}/place.json") as f:
        placed = json.load(f)
    groups = {}
    for name, x, y, z, yaw, scale in placed:
        groups.setdefault(name, lib.Instances(f"Props/{name}")).add(
            props[name], (x, y, z), yaw, scale
        )
    for name, inst in groups.items():
        inst.commit(cull=CULL.get(name, 160000) or None, shadows=name != "cloud")
    post = actors.spawn_actor_from_class(unreal.PostProcessVolume, unreal.Vector(0, 0, 0))
    post.set_editor_property("unbound", True)
    post.set_actor_label("Post")
    with open(f"{SRC}/route.json") as f:
        track(json.load(f))
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    if not unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL):
        raise RuntimeError(f"could not save {LEVEL}")
    log(f"saved {LEVEL}")


try:
    main()
except Exception:
    unreal.log_error(f"STYLIZED failed: {traceback.format_exc()}")
    raise
finally:
    unreal.SystemLibrary.quit_editor()
