"""Builds /Game/Climb/L_Hero from make_climb.py's output: the climb's hero stretch, a kilometre of
mountain road on a wooded flank. 2 km of landscape at a metre a sample, painted and grassed with
the island's ground material, the road, guard rail on the drop side, beech and spruce, and
the ride up and back down as the level's ride track.

Run after make_climb.py and megascans.py:
    OC_EDITOR=1 bash scripts/ue_run.sh climb_world.py climb_world.log

OC_STAGE picks what to build, so a change rebuilds only its part:
- all (default): everything, the level made afresh (saved after each part; tight on memory, so
  ground then dress, as two runs, is the safer way to a whole build);
- ground: the ground and road materials and the landscape in a fresh level, nothing on it;
- dress: the saved level's road, rails, trees and ride track, cleared and placed again from
  make_climb.py's output. The ground and its materials are kept, so it runs in minutes:
    OC_STAGE=dress OC_EDITOR=1 bash scripts/ue_run.sh climb_world.py climb_dress.log
- stills: only the stills, from the saved level.
Every stage but ground ends by rendering stills from fixed cameras along the road to OC_STILLS
(default /workspace/ue/out/climb_stills), so each run ends in pictures.

The ground is island_world.py's material, built afresh here as M_ClimbGround with only the grass
this stretch grows: the island's grass types load every biome's scans, and building their meshes
(1.5-3 GB each, the rocks, trunks and cacti) together gets the editor killed for memory.

Trees: OC_SPRUCE and OC_BEECH name the meshes (defaults: the vegetation plugin's sample trees). A
static mesh goes into one instanced component for every tree of its kind. A skeletal one (the
plugin's Nanite-assembly trees) can't be instanced, so it is placed as an actor per tree, only
within OC_NEAR metres of the road (default 60); beyond that there are none.
"""

import importlib.util
import json
import math
import os
import traceback

import unreal


def sibling(name, library=False):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/climb_world.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    module.OC_LIBRARY = library  # island_world.py then skips its main()
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
island = sibling("island_world", library=True)
assets, actors = lib.assets, lib.actors

SRC = os.environ.get("OC_SRC", "/workspace/ue/world_gen/climb")
STAGE = os.environ.get("OC_STAGE") or "all"
STILLS = os.environ.get("OC_STILLS") or "/workspace/ue/out/climb_stills"
SHOTS = (0.08, 0.3, 0.5, 0.72, 0.95)  # fractions of the way up, for the road-level stills
FOLDER = "/Game/Climb"
LEVEL = f"{FOLDER}/L_Hero"
QUADS = 127
GRID = 4  # components a side per streaming proxy
PVE = "/ProceduralVegetationEditor/SampleAssets/StarterContent"
TREES = {
    "spruce": os.environ.get("OC_SPRUCE") or f"{PVE}/ConiferTree_01/PVE_Conifer_01",
    "beech": os.environ.get("OC_BEECH") or f"{PVE}/DeciduousTree_01/PVE_Deciduous_Tree_01",
}
NEAR = float(os.environ.get("OC_NEAR") or "60") * 100.0
TREE_CULL = 60000.0  # cm: instanced trees past this aren't drawn
RAIL_HEIGHT = 60.0  # cm, the beam's middle above the road
# Grass per layer, as island_world.GRASS: grasses and flowers in the verges and clearings, ferns
# under the trees. No rocks, trunks or logs yet: their scans are the ones too big to build.
GRASS = {
    "Grass": island.GRASS["Grass"],
    "Forest": tuple(row for row in island.GRASS["Forest"] if "fern" in row[1] or "grass" in row[1]),
}


def log(message):
    unreal.log(f"CLIMB {message}")


def materials():
    """The ground, its grass and the road, island_world.py's made in this folder. Saved ones are
    reused (building the grass types alone takes over ten minutes, on one core) unless
    OC_REBUILD is set."""
    island.FOLDER = FOLDER
    ground = unreal.load_asset(f"{FOLDER}/M_ClimbGround")
    road = unreal.load_asset(f"{FOLDER}/Road/M_IslandRoad")
    if ground and road and not os.environ.get("OC_REBUILD"):
        log("materials: reusing the saved ones")
        return ground, road
    island.GRASS = GRASS
    grass = {layer: island.grass_type(layer) for layer in GRASS}
    return island.Ground("M_ClimbGround").build(grass), island.road_material()


def landscape(world, ground):
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    size, spacing, z_scale, names = info["size"], info["spacing"], info["z_scale"], info["layers"]
    if (size - 1) % QUADS:
        raise RuntimeError(f"{size} samples a side isn't whole components of {QUADS} quads")
    comps = (size - 1) // QUADS
    layers = [
        unreal.WorldBuilderLibrary.make_layer_info(f"{FOLDER}/Layers/LI_{n}", n) for n in names
    ]
    half = (size - 1) * spacing / 2
    land = unreal.WorldBuilderLibrary.make_landscape(
        world,
        f"{SRC}/height.r16",
        comps,
        comps,
        QUADS,
        unreal.Vector(-half, -half, 0),
        unreal.Vector(spacing, spacing, z_scale),
        ground,
        layers,
        [f"{SRC}/{n}.r8" for n in names],
        GRID,
    )
    if not land:
        raise RuntimeError("no landscape")
    log(f"landscape: {comps}x{comps} components of {QUADS} quads at {spacing:.0f} cm")


def roads(material):
    count = 0
    for name in sorted(os.listdir(SRC)):
        if not (name.startswith("road_") and name.endswith(".gltf")):
            continue
        task = unreal.AssetImportTask()
        task.filename = f"{SRC}/{name}"
        task.destination_path = f"{FOLDER}/Road"
        task.destination_name = f"SM_{name[:-5]}"
        task.replace_existing = True
        task.automated = True
        task.save = False
        lib.tools.import_asset_tasks([task])
        mesh = next(
            a
            for a in map(unreal.load_asset, task.get_editor_property("imported_object_paths"))
            if isinstance(a, unreal.StaticMesh)
        )
        nanite = mesh.get_editor_property("nanite_settings")
        nanite.set_editor_property("enabled", True)
        mesh.set_editor_property("nanite_settings", nanite)
        assets.save_loaded_asset(mesh, False)
        lib.mesh_actor(mesh, label=name[:-5], folder="Road", materials={0: material})
        count += 1
    log(f"roads: {count} pieces")


def road_points():
    """The road's centreline every 5 m, in cm, for distances to it."""
    with open(f"{SRC}/routes.json") as f:
        ride = json.load(f)["island"]
    half = ride[: len(ride) // 2 + 1]
    return [(x * 100, y * 100) for x, y, _z in half[::5]]


def trees():
    with open(f"{SRC}/trees.json") as f:
        stems = json.load(f)
    meshes = {kind: unreal.load_asset(path) for kind, path in TREES.items()}
    for kind, mesh in meshes.items():
        if mesh is None:
            raise RuntimeError(f"no {kind} mesh at {TREES[kind]}")
        log(f"{kind}: {TREES[kind]} ({type(mesh).__name__})")
    if any(isinstance(m, unreal.SkeletalMesh) for m in meshes.values()):
        # The plugin's materials read wind and tree data from this actor; without it the trees
        # fall back to proxy cards.
        bp = unreal.load_asset(
            "/ProceduralVegetationEditor/SampleAssets/Materials/GlobalFoliageActor/"
            "BP_GlobalFoliageActor_UE5"
        )
        lib.spawn(bp.generated_class(), label="Global Foliage")
    centre = road_points()
    instances = lib.Instances("Trees")
    placed = {kind: 0 for kind in meshes}
    for kind, x, y, z, yaw, scale in stems:
        mesh = meshes[kind]
        if isinstance(mesh, unreal.SkeletalMesh):
            if min(math.hypot(x - cx, y - cy) for cx, cy in centre) > NEAR:
                continue
            actor = lib.spawn(unreal.SkeletalMeshActor, (x, y, z), (0, yaw, 0), folder="Trees")
            actor.skeletal_mesh_component.set_skeletal_mesh_asset(mesh)
            actor.set_actor_scale3d(unreal.Vector(scale, scale, scale))
        else:
            instances.add(mesh, (x, y, z), yaw, scale)
        placed[kind] += 1
    instances.commit(cull=TREE_CULL)
    log(f"trees: {placed}")


def rail_material():
    """Galvanised steel: grey, metallic, a little rough."""
    g = lib.Graph("M_GuardRail", f"{FOLDER}/Rail")
    g.output((0.48, 0.5, 0.52), rough=0.45, metallic=0.9)
    return g.finish()


def rails(material):
    """Each run of rails.json as a beam of short straight pieces, on posts every 2 m."""
    with open(f"{SRC}/rails.json") as f:
        runs = json.load(f)
    cube = unreal.load_asset("/Engine/BasicShapes/Cube")  # 100 cm, centred
    parts = lib.Instances("Rail")
    for run in runs:
        for k in range(len(run) - 1):
            (x0, y0, z0), (x1, y1, z1) = run[k], run[k + 1]
            span = math.hypot(x1 - x0, y1 - y0)
            if span < 1.0:
                continue
            yaw = math.degrees(math.atan2(y1 - y0, x1 - x0))
            pitch = math.degrees(math.atan2(z1 - z0, span))
            mid = ((x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2 + RAIL_HEIGHT)
            parts.add(cube, mid, yaw, ((span + 4) / 100, 0.04, 0.32), pitch=pitch)
            if k % 2 == 0:
                parts.add(cube, (x0, y0, z0 + 30), yaw, (0.1, 0.06, 0.9))
    parts.commit(materials={cube.get_path_name(): {0: material}})
    log(f"rails: {len(runs)} runs")


def track():
    """The ride as the level's DA_RideTrack: up the stretch and back down."""
    with open(f"{SRC}/routes.json") as f:
        centre = json.load(f)["island"]
    cls = unreal.load_class(None, "/Script/OpenCycleUE.RideTrack")
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", cls)
    t = lib.fresh("DA_RideTrack", f"{FOLDER}/Ride", cls, factory)
    # The rider's line 6 cm under the tarmac, as on the island.
    t.set_editor_property(
        "centre", [unreal.Vector(x * 100, y * 100, z * 100 - 6) for x, y, z in centre]
    )
    t.set_editor_property("closed", False)
    t.set_editor_property("title", "Hero stretch")
    t.set_editor_property("blurb", "A kilometre up through the beech and spruce, and back down.")
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
    stretch.set_editor_property("name", "Hero stretch")
    t.set_editor_property("stretches", [stretch])
    assets.save_loaded_asset(t, False)
    log(f"track: {len(centre)} m, out and back")


DRESSING = ("Road", "Rail", "Trees")  # outliner folders the dress stage owns


def fresh_level(ground):
    if assets.does_asset_exist(LEVEL):
        assets.delete_asset(LEVEL)
    world = unreal.EditorLoadingAndSavingUtils.new_map_from_template(
        "/Engine/Maps/Templates/OpenWorld", False
    )
    for actor in actors.get_all_level_actors():
        if isinstance(actor, unreal.LandscapeProxy):
            actors.destroy_actor(actor)
    if not unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL):
        raise RuntimeError(f"could not save {LEVEL}")
    landscape(world, ground)
    return world


def undress():
    """Open the saved level with every actor loaded, and remove the dressing."""
    if not assets.does_asset_exist(LEVEL):
        raise RuntimeError(f"no {LEVEL} to dress: build it with OC_STAGE=all or ground first")
    world = unreal.EditorLoadingAndSavingUtils.load_map(LEVEL)
    # World partition loads only what an editor region asks for; the dressing is spread over
    # all of it, so load every actor first or most of it would survive.
    wp = getattr(unreal, "WorldPartitionBlueprintLibrary", None)
    if wp is not None:
        wp.load_actors([d.guid for d in wp.get_actor_descs()])
    else:
        unreal.log_warning("CLIMB no WorldPartitionBlueprintLibrary: unloaded dressing stays")
    gone = 0
    for actor in actors.get_all_level_actors():
        folder = str(actor.get_folder_path())
        if folder.split("/")[0] in DRESSING or actor.get_actor_label() == "Global Foliage":
            actors.destroy_actor(actor)
            gone += 1
    log(f"undressed: {gone} actors")
    return world


def main():
    """The build as steps. Each yield hands the editor frames (GAP, or the number yielded) before
    the next step."""
    log(f"stage {STAGE}")
    if STAGE not in ("all", "ground", "dress", "stills"):
        raise RuntimeError(f"OC_STAGE {STAGE!r}: use all, ground, dress or stills")
    if STAGE == "stills":
        yield from stills(open_level())
        return
    if STAGE == "dress":
        world = undress()
        island.FOLDER = FOLDER
        road = unreal.load_asset(f"{FOLDER}/Road/M_IslandRoad") or island.road_material()
    else:
        ground, road = materials()
        yield
        world = fresh_level(ground)
    yield
    save(world)
    if STAGE == "ground":
        return
    yield
    roads(road)
    rails(rail_material())
    track()
    yield
    save(world)
    yield
    # The trees last, saved on their own: thousands of actors on top of a fresh landscape
    # got the editor killed for memory as it saved them all together.
    trees()
    yield
    save(world)
    yield from stills(world)


def cameras():
    """(name, position, look-at) in cm: a rider's eye along the road, looking 30 m up it, and
    one from above the hairpin back down the lower traverse."""
    with open(f"{SRC}/routes.json") as f:
        ride = json.load(f)["island"]
    up = ride[: len(ride) // 2 + 1]
    out = []
    for frac in SHOTS:
        k = int(frac * (len(up) - 31))
        (x, y, z), (ax, ay, az) = up[k], up[k + 30]
        out.append((f"road_{round(frac * 100):02d}", (x, y, z + 1.6), (ax, ay, az + 1.2)))
    x, y, z = up[len(up) // 2]
    ax, ay, az = up[len(up) // 5]
    out.append(("hairpin_above", (x + 30, y + 40, z + 45), (ax, ay, az)))
    return [(n, tuple(v * 100 for v in p), tuple(v * 100 for v in a)) for n, p, a in out]


def stills(world):
    os.makedirs(STILLS, exist_ok=True)
    camera = lib.spawn(unreal.CameraActor, label="Stills camera")
    camera.camera_component.set_editor_property("field_of_view", 75.0)
    yield 60  # let the level's streaming, Nanite and lighting settle
    for name, where, target in cameras():
        eye = unreal.Vector(*where)
        rot = unreal.MathLibrary.find_look_at_rotation(eye, unreal.Vector(*target))
        camera.set_actor_location_and_rotation(eye, rot, False, False)
        yield 90  # textures and shadows stream in around the new view
        unreal.AutomationLibrary.take_high_res_screenshot(1280, 720, f"{STILLS}/{name}.png", camera)
        yield 30  # the shot is written a few frames later
        log(f"still {STILLS}/{name}.png")
    actors.destroy_actor(camera)


def open_level():
    world = unreal.EditorLoadingAndSavingUtils.load_map(LEVEL)
    wp = getattr(unreal, "WorldPartitionBlueprintLibrary", None)
    if wp is not None:
        wp.load_actors([d.guid for d in wp.get_actor_descs()])
    return world


def save(world):
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
    log(f"saved {LEVEL}")


# Run from a tick callback, not in one go: a whole build inside one Python call is one editor
# frame, and the Radeon Vulkan driver crashed (vkAllocateCommandBuffers) saving the landscape at
# the end of one. Between steps the editor ticks and its renderer catches up.
GAP = 10  # frames between steps
steps, wait, handle = None, 0, None


busy = False


def tick(_delta):
    global wait, busy
    # A step can pump the editor's frames itself (material builds, progress dialogs), which
    # calls this again inside it: skip those.
    if busy:
        return
    if wait > 0:
        wait -= 1
        return
    busy = True
    try:
        frames = next(steps)
        wait = GAP if frames is None else frames
    except StopIteration:
        finish()
    except Exception:
        # Logged here: quitting first leaves the editor no time to log the error itself.
        unreal.log_error(f"CLIMB failed: {traceback.format_exc()}")
        finish()
    finally:
        busy = False


def finish():
    unreal.unregister_slate_post_tick_callback(handle)
    unreal.SystemLibrary.quit_editor()


# Loaded by a ue_serve.py command (OC_LIBRARY set), it only defines main() for the command to run.
if not globals().get("OC_LIBRARY"):
    # -ExecutePythonScript closes the editor once the script returns unless told not to.
    unreal.EditorPythonScriptingLibrary.set_keep_python_script_alive(True)
    steps = main()
    handle = unreal.register_slate_post_tick_callback(tick)
