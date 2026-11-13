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

The ground is island_world.py's material, built afresh here as the level's own (M_ClimbGround on
L_Hero) with only the grass this stretch grows: the island's grass types load every biome's scans,
and building their meshes (1.5-3 GB each, the rocks, trunks and cacti) together gets the editor
killed for memory.

Trees: OC_SPRUCE and OC_BEECH name the meshes. By default they are the static copies in
/Game/Climb/Trees if `oc.py static-trees` made them, else the vegetation plugin's sample trees. A
static mesh goes into one instanced component for every tree of its kind. A skeletal one (the
plugin's Nanite-assembly trees) can't be instanced, so it is placed as an actor per tree, only
within OC_NEAR metres of the road (default 60); beyond that there are none.
"""

import importlib.util
import json
import math
import os
import random
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
LEVEL = os.environ.get("OC_LEVEL") or f"{FOLDER}/L_Hero"  # e.g. /Game/World/L_World
# The ride track's folder: RidePawn loads <the level's folder>/Ride/DA_RideTrack.
RIDE = f"{LEVEL.rsplit('/', 1)[0]}/Ride"
# The level's own ground material, named as oc_climb._ground_names does: L_Hero keeps
# /Game/Climb/M_ClimbGround, another level gets M_<name>Ground in its folder (L_World's is
# /Game/World/M_WorldGround), so one level's build never re-bakes another's ground.
if LEVEL.lower() == f"{FOLDER}/L_Hero".lower():
    GROUND, MATERIAL, INSTANCE = FOLDER, "M_ClimbGround", "MI_ClimbGround"
else:
    _stem = LEVEL.rsplit("/", 1)[1].removeprefix("L_")
    GROUND, MATERIAL, INSTANCE = LEVEL.rsplit("/", 1)[0], f"M_{_stem}Ground", f"MI_{_stem}Ground"
# Its road's instance beside it, MI_ClimbRoad or MI_<name>Road: the shared M_ClimbRoad at the
# level's road width.
ROAD_INSTANCE = INSTANCE.removesuffix("Ground") + "Road"
QUADS = 127
GRID = 4  # components a side per streaming proxy
PVE = "/ProceduralVegetationEditor/SampleAssets/StarterContent"
# The plugin's skeletal trees, and their static Nanite copies (made by `oc.py static-trees`).
PVE_TREES = {
    "spruce": f"{PVE}/ConiferTree_01/PVE_Conifer_01",
    "beech": f"{PVE}/DeciduousTree_01/PVE_Deciduous_Tree_01",
}
STATIC_TREES = {kind: f"/Game/Climb/Trees/SM_{kind}" for kind in PVE_TREES}


def tree_mesh(kind):
    """OC_SPRUCE / OC_BEECH if set, else the static copy if it exists, else the skeletal tree."""
    chosen = os.environ.get(f"OC_{kind.upper()}")
    if chosen:
        return chosen
    static = STATIC_TREES[kind]
    return static if unreal.EditorAssetLibrary.does_asset_exist(static) else PVE_TREES[kind]


NEAR = float(os.environ.get("OC_NEAR") or "60") * 100.0
TREE_CULL = 200000.0  # cm: instanced trees past this aren't drawn (the map is 2 km across)
FAR_CULL = 800000.0  # cm: the world's far forest (trees_far.json), seen to 8 km
RAIL_HEIGHT = 85.0  # cm, the fence rail's middle above the path
# Grass per layer, as island_world.GRASS: grasses and flowers in the verges and clearings, ferns
# under the trees. No rocks, trunks or logs yet: their scans are the ones too big to build.
GRASS = {
    "Grass": island.GRASS["Grass"],
    "Forest": tuple(row for row in island.GRASS["Forest"] if "fern" in row[1] or "grass" in row[1]),
}


def log(message):
    unreal.log(f"CLIMB {message}")


# The island's rock (stony soil) renders red-brown on the cut banks and its dirt (gravel on soil
# with dirt road) as yellow sand on the shoulders: a mountain road's are grey granite and gravel.
SURFACES = {
    "Rock": ("mountain", "granite_rocky_ground", (0.85, 0.85, 0.85)),
    "Dirt": ("shared", "gravel", (0.8, 0.8, 0.8)),
}
MIXES = {"Rock": ("forest", "rocky_forest_path", "patches", 12.0)}


def landscape_loaded():
    """Whether a loaded landscape draws with this level's ground material, directly or through an
    instance. (The empty startup map has a landscape of its own, which a rebuild doesn't touch.)"""
    for actor in unreal.EditorActorSubsystem().get_all_level_actors():
        if not isinstance(actor, unreal.LandscapeProxy):
            continue
        material = actor.get_editor_property("landscape_material")
        while material is not None:
            if material.get_name() == MATERIAL:
                return True
            if not isinstance(material, unreal.MaterialInstance):
                break
            material = material.get_editor_property("parent")
    return False


def ground_material():
    """The level's ground material (MATERIAL), rebuilt with this climb's surfaces; the saved
    grass types are reused (building them takes over ten minutes, on one core) unless OC_REBUILD
    is set.

    Only with no landscape loaded: under one, the editor recompiled every section's material in
    one frame for six minutes and the Radeon driver lost the device (twice). To recolour, use
    tune(), which compiles nothing."""
    if landscape_loaded():
        raise RuntimeError(
            "ground_material: a landscape using it is loaded; rebuilding under it crashes the GPU. "
            "Restart the server (quit.py; ue_serve.sh brings it back on the empty map) first."
        )
    island.FOLDER = FOLDER
    island.GRASS = GRASS
    island.SURFACES.update(SURFACES)
    island.MIXES.pop("Dirt", None)
    island.MIXES.update(MIXES)
    grass = {}
    for layer in GRASS:
        saved = unreal.load_asset(f"{FOLDER}/Grass/GT_{layer}")
        grass[layer] = (
            saved if saved and not os.environ.get("OC_REBUILD") else island.grass_type(layer)
        )
    island.FOLDER = GROUND  # the material goes in the level's folder; its grass stays shared
    try:
        return island.Ground(MATERIAL).build(grass)
    finally:
        island.FOLDER = FOLDER


def ground_instance(parent):
    """INSTANCE, the landscape's material: MATERIAL with its tints set by tune()."""
    path = f"{GROUND}/{INSTANCE}"
    mi = unreal.load_asset(path)
    if mi is None:
        mi = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
            INSTANCE,
            GROUND,
            unreal.MaterialInstanceConstant,
            unreal.MaterialInstanceConstantFactoryNew(),
        )
    if mi.get_editor_property("parent") != parent:
        unreal.MaterialEditingLibrary.set_material_instance_parent(mi, parent)
    unreal.EditorAssetLibrary.save_loaded_asset(mi, False)
    return mi


def tune(tints):
    """Recolours the ground's surfaces on INSTANCE, e.g. tune({"Dirt": (0.6, 0.62, 0.65)}),
    keys being SURFACES' layers (or "<layer> mix" for a MIXES surface). Parameters only: nothing
    compiles, so it's safe, and quick, with the level open."""
    mi = unreal.load_asset(f"{GROUND}/{INSTANCE}")
    for name, rgb in tints.items():
        unreal.MaterialEditingLibrary.set_material_instance_vector_parameter_value(
            mi, f"{name} tint", unreal.LinearColor(*rgb, 1.0)
        )
    unreal.MaterialEditingLibrary.update_material_instance(mi)
    unreal.EditorAssetLibrary.save_loaded_asset(mi, False)
    log(f"tuned {tints}")
    return mi


def materials():
    """The ground, its grass and the road, island_world.py's made in this folder. Saved ones are
    reused unless OC_REBUILD is set."""
    island.FOLDER = FOLDER
    ground = unreal.load_asset(f"{GROUND}/{MATERIAL}")
    road = unreal.load_asset(f"{FOLDER}/Road/M_ClimbRoad")
    if ground and road and not os.environ.get("OC_REBUILD"):
        log("materials: reusing the saved ones")
        return ground_instance(ground), road
    return ground_instance(ground_material()), road_material()


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


# The ground's colour as a runtime virtual texture, shared by the levels (only the world's has a
# volume, so only its landscape writes it): the road's and path's edges fade into it.
GROUND_RVT = f"{FOLDER}/RVT_Ground"
# 2^12 tiles of 256 texels: about 5.5 cm a texel over the world's 57 km landscape.
RVT_TILES, RVT_TILE_SIZE = 12, 4


def ground_rvt():
    """GROUND_RVT, made once (base colour only; its pages render on demand where sampled)."""
    rvt = unreal.load_asset(GROUND_RVT)
    if rvt is None:
        folder, name = GROUND_RVT.rsplit("/", 1)
        rvt = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
            name, folder, unreal.RuntimeVirtualTexture, unreal.RuntimeVirtualTextureFactory()
        )
    rvt.set_editor_property("material_type", unreal.RuntimeVirtualTextureMaterialType.BASE_COLOR)
    rvt.set_editor_property("tile_count", RVT_TILES)
    rvt.set_editor_property("tile_size", RVT_TILE_SIZE)
    rvt.set_editor_property("adaptive", True)
    unreal.EditorAssetLibrary.save_loaded_asset(rvt, False)
    return rvt


def _rvt_colour(g):
    """The ground's colour here, from GROUND_RVT (black where no volume covers it)."""
    sample = g.node(unreal.MaterialExpressionRuntimeVirtualTextureSample)
    sample.set_editor_property("virtual_texture", ground_rvt())
    sample.set_editor_property("material_type", unreal.RuntimeVirtualTextureMaterialType.BASE_COLOR)
    sample.set_editor_property("adaptive", True)
    return g.link(
        sample,
        g.node(unreal.MaterialExpressionComponentMask, r=True, g=True, b=True),
        "",
        "BaseColor",
    )


def rvt_volume():
    """The world's landscape writes GROUND_RVT: a volume over its bounds, in the Road folder so the
    dress stage clears and places it with the road."""
    land = next(
        (a for a in actors.get_all_level_actors() if isinstance(a, unreal.LandscapeProxy)), None
    )
    if land is None:
        raise RuntimeError("rvt_volume: no landscape")
    rvt = ground_rvt()
    for proxy in actors.get_all_level_actors():
        if isinstance(proxy, unreal.LandscapeProxy):
            proxy.set_editor_property("runtime_virtual_textures", [rvt])
    # From the source, not the actor: under world partition the landscape's own bounds are empty.
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    half = (info["size"] - 1) * info["spacing"] / 2
    low, tall = -50000.0, 300000.0  # cm: below the sea floor to above the peaks
    extent = unreal.Vector(half, half, tall / 2)
    volume = actors.spawn_actor_from_class(
        unreal.RuntimeVirtualTextureVolume, unreal.Vector(-half, -half, low)
    )
    volume.set_actor_label("RVT_Ground")
    volume.set_folder_path("Road")
    volume.set_actor_scale3d(extent * 2.0)  # the volume's box is the unit cube from its origin
    component = volume.get_editor_property("virtual_texture_component")
    component.set_editor_property("virtual_texture", rvt)
    log(f"ground RVT over {extent.x * 2 / 100000:.1f} km")
    return volume


def road_instance(parent):
    """ROAD_INSTANCE, the level's road material: `parent` at this road's half width (info.json's
    road_half, m; the climb's path when it has none) and its road_look, scalars and colours by
    parameter name (the world's lighter asphalt with a centre line)."""
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    scalars = {"road half": float(info.get("road_half", ROAD_HALF))}
    vectors = {}
    for key, value in info.get("road_look", {}).items():
        (vectors if isinstance(value, list) else scalars)[key] = value
    return lib.instance(ROAD_INSTANCE, parent, GROUND, scalars=scalars, vectors=vectors)


def roads(material, path=None, trail=None):
    """The road pieces: road_path_*.gltf in `path` (the remote country's dirt path),
    road_trail_*.gltf in `trail` (the town's bike trail), the rest in the level's road instance
    of `material`."""
    material = road_instance(material)
    count = 0
    for name in sorted(os.listdir(SRC)):
        if not (name.startswith("road_") and name.endswith(".gltf")):
            continue
        surface = path if (path is not None and name.startswith("road_path_")) else material
        if trail is not None and name.startswith("road_trail_"):
            surface = trail
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
        actor = lib.mesh_actor(mesh, label=name[:-5], folder="Road", materials={0: surface})
        if path is not None:
            # The world's road stands on an earth bank (road_mesh.section): cast, the low sun
            # laid the tarmac's shadow down the bank's far side as a dark stripe.
            actor.static_mesh_component.set_editor_property("cast_shadow", False)
        count += 1
    log(f"roads: {count} pieces")


def _listed(name):
    """Whether the source's `name` (rails.json, props.json) lists anything."""
    path = f"{SRC}/{name}"
    if not os.path.exists(path):
        return False
    with open(path) as f:
        return bool(json.load(f))


def dressing_materials():
    """The rails' and props' materials, built before the level loads, and only when the source has
    any: compiled with the world open, the third of them crashed the Vulkan RHI (a descriptor
    assert) every time on 11 Nov. Returns (timber, posts, logs), None where unused."""
    fence, stacks = _listed("rails.json"), _listed("props.json")
    timber = rail_material() if fence or stacks else None
    return timber, post_material() if fence else None, log_material() if stacks else None


def _has_paths():
    """Whether the source has dirt-path pieces (the world's remote stretches)."""
    return any(n.startswith("road_path_") and n.endswith(".gltf") for n in os.listdir(SRC))


def road_points():
    """The road's centreline every 5 m, in cm, for distances to it."""
    with open(f"{SRC}/routes.json") as f:
        ride = json.load(f)["island"]
    half = ride[: len(ride) // 2 + 1]
    return [(x * 100, y * 100) for x, y, _z in half[::5]]


def sea_level():
    """info.json's sea_cm: set for a biome with a sea (make_climb.py's SHAPES), else None."""
    with open(f"{SRC}/info.json") as f:
        return json.load(f).get("sea_cm")


def sea_material():
    """Dark, smooth water. Built before the level loads, like the road's material."""
    if sea_level() is None:
        return None
    g = lib.Graph("M_ClimbSea", f"{FOLDER}/Sea")
    swell = g.world_noise(0.0015, 0.0, 1.0, 3)
    colour = g.lerp(g.const((0.006, 0.03, 0.04)), g.const((0.012, 0.06, 0.07)), swell)
    g.output(colour, rough=g.lerp(g.const(0.04), g.const(0.12), swell), specular=0.5)
    return g.finish()


def far_files():
    return sorted(n for n in os.listdir(SRC) if n.startswith("far_") and n.endswith(".gltf"))


def far_material():
    """The far horizon's ground: the plan's colours from its vertices. None without a
    horizon (only a window of the world plan has one)."""
    if not far_files():
        return None
    g = lib.Graph("M_ClimbFar", f"{FOLDER}/Far")
    vertex = g.node(unreal.MaterialExpressionVertexColor)
    # Alpha is roughness: matte land, a glossy sea that mirrors the sky as the near sea does.
    rough = g.link(vertex, g.node(unreal.MaterialExpressionMultiply, const_b=1.0), "A", "A")
    g.output(g.mask(vertex, "rgb"), rough=rough, specular=0.4)
    return g.finish(two_sided=True)


def far(material):
    """The world plan around the window (make_climb.far_horizon), as Nanite meshes."""
    count = 0
    for name in far_files():
        task = unreal.AssetImportTask()
        task.filename = f"{SRC}/{name}"
        task.destination_path = f"{FOLDER}/Far"
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
        actor = lib.mesh_actor(mesh, label=name[:-5], folder="Far", materials={0: material})
        actor.set_editor_property("is_spatially_loaded", False)
        count += 1
    log(f"far horizon: {count} pieces")


def sea(material):
    """A plane of water at the sea's level, over the whole map."""
    level = sea_level()
    if level is None:
        return
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    # The engine plane is 1 m: 4 km of water, or the map and a margin round it.
    metres = max(4000.0, info["size"] * info["spacing"] / 100 * 1.3)
    actor = lib.mesh_actor(
        unreal.load_asset("/Engine/BasicShapes/Plane"),
        (0, 0, level),
        scale=(metres, metres, 1),
        label="Sea",
        folder="Sea",
    )
    actor.static_mesh_component.set_material(0, material)
    actor.set_editor_property("is_spatially_loaded", False)
    log(f"sea at {level / 100:.0f} m")


def trees():
    with open(f"{SRC}/trees.json") as f:
        stems = json.load(f)
    if not stems:
        log("trees: none")
        return
    paths = {kind: tree_mesh(kind) for kind in PVE_TREES}
    meshes = {kind: unreal.load_asset(path) for kind, path in paths.items()}
    for kind, mesh in meshes.items():
        if mesh is None:
            raise RuntimeError(f"no {kind} mesh at {paths[kind]}")
        log(f"{kind}: {paths[kind]} ({type(mesh).__name__})")
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
    far_trees(meshes)


def far_trees(meshes):
    """The world's far forest, past the ring's woods (trees_far.json): instanced, casting no
    shadow (only seen from a distance; the shadow maps would pay for every one), to 8 km."""
    path = f"{SRC}/trees_far.json"
    if not os.path.exists(path):
        return
    with open(path) as f:
        stems = json.load(f)
    instances = lib.Instances("Trees/Far")
    for kind, x, y, z, yaw, scale in stems:
        if isinstance(meshes[kind], unreal.StaticMesh):
            instances.add(meshes[kind], (x, y, z), yaw, scale)
    instances.commit(cull=FAR_CULL, shadows=False)


def _noise_at(g, position, scale, low, high, levels=3):
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


def _weathered(g, colour, grain_space):
    """Old outdoor wood over `colour`: silvered in patches, lichen spots, moss on what faces up."""
    colour = g.mul(colour, _noise_at(g, grain_space, 0.1, 0.7, 1.15))
    silver = g.saturate(g.world_noise(0.004, -0.6, 1.4, 3))
    # Silvering and lichen only a shade off the timber: at 0.15 and 0.26 grey they speckled the
    # fence and the bench pale (QA, v13).
    colour = g.lerp(colour, g.const((0.1, 0.095, 0.09)), g.mul(silver, 0.5))
    lichen = g.saturate(g.mul(g.add(g.world_noise(0.15, 0.0, 1.0, 2), -0.8), 8.0))
    colour = g.lerp(colour, g.const((0.13, 0.14, 0.1)), g.mul(lichen, 0.4))
    up = g.mask(g.node(unreal.MaterialExpressionVertexNormalWS), "b")
    moss = g.mul(
        g.saturate(g.mul(g.add(up, -0.55), 4.0)),
        g.saturate(g.world_noise(0.012, -1.0, 2.0, 3)),
    )
    return g.lerp(colour, g.const((0.04, 0.065, 0.02)), moss), moss


def timber_material(name, stretch):
    """Weathered timber for the fence and the bench: grey-brown with grain along the local axis
    that `stretch` shortens, damp and darker near the ground."""
    g = lib.Graph(name, f"{FOLDER}/Rail")
    local = g.node(unreal.MaterialExpressionLocalPosition)
    grain_space = g.mul(local, g.const(stretch))
    colour, moss = _weathered(g, g.param("timber", (0.075, 0.062, 0.05)), grain_space)
    # The foot of a post: the instance's origin is 40 cm above the ground.
    world_z = g.mask(g.node(unreal.MaterialExpressionWorldPosition), "b")
    origin_z = g.mask(g.node(unreal.MaterialExpressionObjectPositionWS), "b")
    rel = g.add(world_z, g.mul(origin_z, -1.0))
    damp = g.saturate(g.mul(g.add(rel, 20.0), -1 / 20))
    colour = g.lerp(colour, g.mul(colour, g.const((0.5, 0.52, 0.45))), damp)
    rough = g.lerp(g.const(0.88), g.const(0.95), moss)
    g.output(colour, rough=rough, specular=0.3, metallic=0.0)
    return g.finish()


def rail_material():
    """The fence's rails: grain along their length (local X)."""
    return timber_material("M_FenceRail", (0.5, 1.0, 1.0))


def post_material():
    """The fence's round posts: grain up their length (the cylinder's local Z)."""
    return timber_material("M_FencePost", (1.0, 1.0, 0.5))


def log_material():
    """Cut logs in a stack: dark bark down the length (the cylinder's local Z), pale greyed cut
    ends with rings."""
    g = lib.Graph("M_StackedLog", f"{FOLDER}/Props")
    local = g.node(unreal.MaterialExpressionLocalPosition)
    bark = g.mul(
        g.const((0.055, 0.042, 0.032)),
        _noise_at(g, g.mul(local, g.const((1.0, 1.0, 0.15))), 0.25, 0.55, 1.3),
    )
    bark, _moss = _weathered(g, bark, g.mul(local, g.const((1.0, 1.0, 0.3))))
    to_local = g.node(
        unreal.MaterialExpressionTransform,
        transform_source_type=unreal.MaterialVectorCoordTransformSource.TRANSFORMSOURCE_WORLD,
        transform_type=unreal.MaterialVectorCoordTransform.TRANSFORM_LOCAL,
    )
    g.link(g.node(unreal.MaterialExpressionVertexNormalWS), to_local)
    along = g.link(g.mask(to_local, "b"), g.node(unreal.MaterialExpressionAbs))
    cap = g.saturate(g.mul(g.add(along, -0.8), 10.0))
    radius = g.link(g.mask(local, "rg"), g.node(unreal.MaterialExpressionLength))
    rings = g.link(g.mul(radius, 0.9), g.node(unreal.MaterialExpressionSine))
    end = g.mul(g.const((0.34, 0.27, 0.19)), g.add(g.mul(rings, 0.08), 0.92))
    end = g.mul(end, g.world_noise(0.05, 0.75, 1.1, 2))
    g.output(g.lerp(bark, end, cap), rough=0.9, metallic=0.0)
    return g.finish()


# The island's asphalt read washed-out grey on the climb (0.4 still pale, v13). v34: 0.3 read
# pale beige under the high sun.
ROAD_DARKEN = 0.19
# The scan's bright aggregate read as a regular white speckle (QA, v12): clamped before darkening.
ROAD_SPECK_CLAMP = 0.3
# The edge's soil_mud, darkened and browned to the ground's damp litter.
ROAD_EDGE_SOIL = (0.36, 0.3, 0.24)  # v25: its pebbles read as pale orange ovals
# Old country tarmac is warm grey-brown, not the scan's blue-grey (QA, v11).
# v25: (1.15, 1, 0.72) still read pink-grey (r/g 1.13 in v24). v34: less yellow for daylight.
ROAD_WARM = (1.03, 1.0, 0.86)
# The world's centre line: old white paint, a little dirty.
LINE_WHITE = (0.62, 0.61, 0.57)
# A kept road's paint, fresh: whiter, not the scan's pure white (0.8 linear is still bright).
LINE_FRESH = (0.72, 0.72, 0.7)
# The road material's default half width, m: the climb's bike path (make_climb.py's ROAD_HALF).
ROAD_HALF = 1.5


def road_material():
    """The island's Megascans asphalt, world-mapped, darkened for the bike path."""
    g = lib.Graph("M_ClimbRoad", f"{FOLDER}/Road")
    tile = island.tile_cm("urban", "asphalt_road")
    d, n, m = (island.ms_texture("urban", "asphalt_road", k) for k in ("D", "N", "M"))
    uv = g.world_uv(tile)
    masks = g.sample(m, uv, "masks")
    colour = g.mask(g.sample(d, uv), "rgb")
    colour = g.op(unreal.MaterialExpressionMin, colour, g.param("speck clamp", ROAD_SPECK_CLAMP))
    normal = g.mask(g.sample(n, uv, "normal"), "rgb")
    rough = g.op(unreal.MaterialExpressionMax, g.mask(masks, "g"), g.const(0.7))
    # Upkeep (road_mesh's vertex colour, make_world_level.road_wear): 1 old country tarmac, every
    # wear below at full; 0 kept like new by the harbour, black, clean, its paint fresh.
    wear = g.mask(g.node(unreal.MaterialExpressionVertexColor), "r")
    some = g.lerp(g.const(0.25), g.const(1.0), wear)
    # Wear: stretches of cracked old tarmac, darker patched repairs, soil and litter washed onto
    # the edges, and fallen leaves, most of them near the edges.
    ctile = island.tile_cm("shared", "cracked_asphalt")
    cd, cn = (island.ms_texture("shared", "cracked_asphalt", k) for k in ("D", "N"))
    cuv = g.world_uv(ctile, (0.21, 0.47))
    # Partly cracked, in patches a few metres across with soft edges: a hard mask at 8 m drew big
    # dark ellipses on the path.
    cracked = g.mul(g.saturate(g.mul(g.add(g.world_noise(0.004, -1.0, 2.0, 3), -0.4), 1.5)), 0.55)
    cracked = g.mul(cracked, wear)
    colour = g.lerp(colour, g.mul(g.mask(g.sample(cd, cuv), "rgb"), 0.9), cracked)
    normal = g.lerp(normal, g.mask(g.sample(cn, cuv, "normal"), "rgb"), cracked)
    # Repairs are hard-edged blocks a metre or two across, not soft blots: noise sampled on a
    # 1.6 m grid of world cells, so each cell is either patched or not.
    world = g.node(unreal.MaterialExpressionWorldPosition)
    cell = g.mul(g.link(g.mul(world, 1 / 160), g.node(unreal.MaterialExpressionFloor)), 160.0)
    noise = g.node(
        unreal.MaterialExpressionNoise,
        scale=0.004,
        quality=2,
        levels=1,
        output_min=0.0,
        output_max=1.0,
    )
    g.link(cell, noise, "World Position")
    # On v15's darker tarmac a 0.72 repair read as a black block (QA): rarer, and only a shade off.
    patch = g.mul(g.saturate(g.mul(g.add(noise, -0.86), 40.0)), g.saturate(g.mul(wear, 1.5)))
    colour = g.lerp(colour, g.mul(colour, 0.85), patch)
    # v21: wider, as QA read the tarmac as uniform grey: lighter worn and darker damp stretches.
    colour = g.mul(colour, g.lerp(g.const(1.0), g.world_noise(0.001, 0.85, 1.12, 3), some))
    colour = g.mul(colour, g.world_noise(0.0003, 0.9, 1.08, 2))
    # v25: riders wear the middle of a bike path smooth and a shade darker, in a band that wanders
    # a little; QA read the tarmac as one flat grey.
    wander = g.world_noise(0.003, -0.12, 0.12, 2)
    centre = g.link(
        g.mask(g.node(unreal.MaterialExpressionTextureCoordinate), "r"),
        g.node(unreal.MaterialExpressionAbs),
    )
    worn = g.saturate(g.mul(g.add(g.add(centre, wander), -0.45), -3.0))
    worn = g.mul(g.mul(worn, g.world_noise(0.002, 0.5, 1.0, 2)), wear)
    colour = g.lerp(colour, g.mul(colour, 0.82), worn)
    # 0.55 drew a wet, sky-bright stripe down the middle (v25): worn smooth, not polished.
    rough = g.lerp(rough, g.param("worn rough", 0.82), worn)
    # New tarmac is blacker than old, greyed by sun and traffic.
    colour = g.mul(colour, g.lerp(g.param("fresh", 1.0), g.const(1.0), wear))
    colour = g.mul(colour, g.param("darken", ROAD_DARKEN))
    colour = g.mul(colour, g.param("warm", ROAD_WARM))
    across = g.link(
        g.mask(g.node(unreal.MaterialExpressionTextureCoordinate), "r"),
        g.node(unreal.MaterialExpressionAbs),
    )
    # The edge frays: soil and gravel reach 20-60 cm in, raggedly, so the tarmac has no clean line.
    ragged = g.add(g.world_noise(0.02, -0.3, 0.3, 2), g.world_noise(0.15, -0.15, 0.15, 2))
    # Narrower than v13's (offset 0.95, x2.2): its pale soil pebbles spread halfway in and read as
    # a bright speckle of ovals on the tarmac.
    # Measured in from the tarmac's edge, "road half" metres out (U is metres across): the climb's
    # path is 1.5 m, the world's road 3.2 m, where an edge at 1 m browned two metres of each side.
    inside = g.add(across, g.mul(g.param("road half", ROAD_HALF), -1.0))
    edge = g.saturate(g.mul(g.add(g.add(inside, g.mul(ragged, 0.6)), 0.5), 3.0))
    stile = island.tile_cm("forest", "soil_mud")
    soil = g.mask(g.sample(island.ms_texture("forest", "soil_mud", "D"), g.world_uv(stile)), "rgb")
    soil = g.mul(soil, g.const(ROAD_EDGE_SOIL))
    colour = g.lerp(colour, soil, g.mul(g.mul(edge, 0.85), some))
    # Where the world's ground RVT covers it ("ground blend" 1), the last few cm fade into the
    # ground's own colour, so the tarmac doesn't end on a line against sand or turf.
    rim = g.saturate(g.mul(g.add(inside, g.add(g.mul(ragged, 0.3), 0.12)), 8.0))
    colour = g.lerp(colour, _rvt_colour(g), g.mul(rim, g.param("ground blend", 0.0)))
    leaves = g.saturate(g.mul(g.add(g.world_noise(0.07, 0.0, 1.0, 2), -0.8), 7.0))
    leaves = g.mul(g.mul(leaves, g.lerp(g.const(0.2), g.const(1.0), edge)), some)
    # Once the road went dark, leaves at 0.11 read as pale ovals (v14): a shade off the tarmac, and
    # fewer of them (threshold 0.8, was 0.72).
    leaf_colour = g.mul(g.const((0.06, 0.04, 0.025)), g.world_noise(0.03, 0.6, 1.4, 2))
    colour = g.lerp(colour, leaf_colour, leaves)
    rough = g.lerp(rough, g.const(0.92), g.saturate(g.add(edge, leaves)))
    # v35: rougher and less specular: seen against the high sun the tarmac shone pale (the
    # bench, v34).
    rough = g.op(unreal.MaterialExpressionMax, rough, g.const(0.9))
    # A dashed line down the middle, 3 m on and 3 m off, worn: off ("line" 0) on the climb's
    # one-lane path, on for the world's two-lane bike path. V is metres along the piece.
    along = g.mask(g.node(unreal.MaterialExpressionTextureCoordinate), "g")
    dash = g.link(g.mul(along, 1 / 6), g.node(unreal.MaterialExpressionFrac))
    dash = g.saturate(g.mul(g.add(dash, -0.5), -40.0))
    stripe = g.saturate(g.mul(g.add(across, -0.06), -60.0))
    paint = g.mul(g.mul(stripe, dash), g.param("line", 0.0))
    paint = g.mul(paint, g.lerp(g.const(1.0), g.world_noise(0.05, 0.55, 1.0, 2), some))
    # A kept road's solid edge lines, 10 cm wide and 25 cm in from the tarmac's edge ("edge
    # line" 0 on the climb's path): only where the wear is under about half.
    lip = g.link(g.add(inside, 0.3), g.node(unreal.MaterialExpressionAbs))
    side = g.saturate(g.mul(g.add(lip, -0.05), -60.0))
    side = g.mul(side, g.saturate(g.mul(g.add(wear, -0.55), -4.0)))
    paint = g.op(unreal.MaterialExpressionMax, paint, g.mul(side, g.param("edge line", 0.0)))
    fresh_white = g.lerp(g.const(LINE_FRESH), g.const(LINE_WHITE), wear)
    colour = g.lerp(colour, fresh_white, paint)
    rough = g.lerp(rough, g.const(0.7), paint)
    g.output(colour, normal, rough, specular=0.2, ao=g.mask(masks, "r"))
    g.material.set_editor_property("used_with_nanite", True)
    return g.finish()


# The path's compacted earth, a little greyed so it sits on sand, turf and granite alike.
PATH_TINT = (0.6, 0.58, 0.56)
# Its edge, m from the middle: the mesh's own (make_world_level.PATH_HALF), about 2.4 m across.
PATH_HALF = 1.2
# Its loose edge: the gravel, darker.
PATH_EDGE = (0.54, 0.52, 0.49)


def path_material():
    """The remote country's path: compacted dirt with patches of gravel, worn smoother in the
    middle, its edges loose and darker. Opaque: a masked Nanite material's rasteriser shader
    overflows the shader compiler's stack on this machine (the v0.1.1 world build crash)."""
    g = lib.Graph("M_ClimbPath", f"{FOLDER}/Road")
    d, n, m = (island.ms_texture("shared", "dirt_road", k) for k in ("D", "N", "M"))
    gd, gn = (island.ms_texture("shared", "gravel_on_soil", k) for k in ("D", "N"))
    # Two scales of the 0.87 m scan, so the tile doesn't print a grid down the path.
    uv, uv2 = g.world_uv(140.0), g.world_uv(410.0, (0.37, 0.11))
    mix = g.world_noise(0.01, 0.0, 1.0, 2)
    colour = g.lerp(
        g.mask(g.sample(d, uv), "rgb"), g.mask(g.sample(d, uv2), "rgb"), g.mul(mix, 0.5)
    )
    normal = g.mask(g.sample(n, uv, "normal"), "rgb")
    masks = g.sample(m, uv, "masks")
    gravel = g.saturate(g.mul(g.add(g.world_noise(0.006, 0.0, 1.0, 3), -0.55), 3.0))
    guv = g.world_uv(150.0, (0.21, 0.63))
    colour = g.lerp(colour, g.mask(g.sample(gd, guv), "rgb"), g.mul(gravel, 0.7))
    normal = g.lerp(normal, g.mask(g.sample(gn, guv, "normal"), "rgb"), g.mul(gravel, 0.7))
    colour = g.mul(colour, g.world_noise(0.0015, 0.88, 1.1, 3))
    across = g.link(
        g.mask(g.node(unreal.MaterialExpressionTextureCoordinate), "r"),
        g.node(unreal.MaterialExpressionAbs),
    )
    # Worn by tyres down the middle: darker and finer, wandering a little.
    wander = g.world_noise(0.003, -0.15, 0.15, 2)
    worn = g.saturate(g.mul(g.add(g.add(across, wander), -0.55), -2.5))
    colour = g.lerp(colour, g.mul(colour, 0.86), g.mul(worn, 0.8))
    colour = g.mul(colour, g.param("tint", PATH_TINT))
    # A dirt path is the country's own earth: part of its colour is the ground's here, from the
    # world's ground RVT, so it's grey on the heath and tan in the desert, not one bright sand.
    colour = g.lerp(colour, g.mul(_rvt_colour(g), 0.82), g.param("ground share", 0.65))
    rough = g.lerp(g.const(0.95), g.const(0.88), worn)
    # The last 20-50 cm, raggedly: loose gravel, darker, rougher, so the edge isn't a clean line.
    ragged = g.add(g.world_noise(0.02, -0.15, 0.15, 2), g.world_noise(0.15, -0.08, 0.08, 2))
    inside = g.add(g.param("path half", PATH_HALF), g.mul(across, -1.0))
    edge = g.saturate(g.mul(g.add(g.add(inside, ragged), -0.35), -4.0))
    loose = g.mul(g.mask(g.sample(gd, guv), "rgb"), g.const(PATH_EDGE))
    colour = g.lerp(colour, loose, g.mul(edge, 0.8))
    normal = g.lerp(normal, g.mask(g.sample(gn, guv, "normal"), "rgb"), edge)
    rough = g.lerp(rough, g.const(1.0), edge)
    # The outer 15-30 cm is the ground's own colour, from the world's ground RVT.
    rim = g.saturate(g.mul(g.add(g.add(inside, ragged), -0.22), -6.0))
    colour = g.lerp(colour, _rvt_colour(g), g.mul(rim, g.param("ground blend", 1.0)))
    g.output(colour, normal, rough, specular=0.25, ao=g.mask(masks, "r"))
    g.material.set_editor_property("used_with_nanite", True)
    return g.finish()


# The town's bike trail, after Chicago's Bloomingdale Trail (the 606): pale broom-finished
# concrete in 3 m slabs, blue-grey rubber running shoulders, a yellow dashed centre line.
TRAIL_CONCRETE = (0.13, 0.127, 0.12)  # 0.34 and 0.24 read as white under the hub sun
TRAIL_RUBBER = (0.014, 0.02, 0.032)  # 0.05 rendered mid grey (v51)
TRAIL_SHOULDER = 0.45  # m, each side
TRAIL_YELLOW = (0.62, 0.4, 0.03)
TRAIL_SLAB = 3.0  # m between saw-cut joints
TRAIL_DASH = (1.0, 4.0)  # m of yellow, m between dash starts
# Galvanised steel for its posts, rails and swan-neck lamps.
TRAIL_STEEL = (0.4, 0.41, 0.42)


def _has_trail():
    """Whether the source has the town's trail (make_world_level.trail_dressing)."""
    return any(n.startswith("road_trail_") and n.endswith(".gltf") for n in os.listdir(SRC))


def trail_materials():
    """(deck, steel, mesh), built before the level loads like every other material."""
    return trail_material(), steel_material(), fence_mesh_material()


def trail_material():
    """The trail's deck: U is metres across, V metres along (road_mesh)."""
    with open(f"{SRC}/info.json") as f:
        half = float(json.load(f).get("road_half", ROAD_HALF))
    g = lib.Graph("M_Trail", f"{FOLDER}/Road")
    coord = g.node(unreal.MaterialExpressionTextureCoordinate)
    across = g.link(g.mask(coord, "r"), g.node(unreal.MaterialExpressionAbs))
    along = g.mask(coord, "g")
    # Concrete: mottled over metres, a fine broom grain, slabs a shade apart.
    colour = g.mul(g.const(TRAIL_CONCRETE), g.world_noise(0.002, 0.9, 1.08, 3))
    colour = g.mul(colour, g.world_noise(0.08, 0.94, 1.05, 2))
    slab = g.link(g.mul(along, 1 / TRAIL_SLAB), g.node(unreal.MaterialExpressionFloor))
    # Each slab a shade apart: a sine hash of its index.
    hashed = g.link(g.mul(slab, 12.9898), g.node(unreal.MaterialExpressionSine))
    hashed = g.link(g.mul(hashed, 43.758), g.node(unreal.MaterialExpressionFrac))
    shade = g.add(g.mul(hashed, 0.1), 0.95)
    colour = g.mul(colour, shade)
    # Saw-cut joints across the slab ends, a centimetre and a half wide.
    frac = g.link(g.mul(along, 1 / TRAIL_SLAB), g.node(unreal.MaterialExpressionFrac))
    near = g.op(unreal.MaterialExpressionMin, frac, g.add(g.mul(frac, -1.0), 1.0))
    joint = g.saturate(g.mul(g.add(g.mul(near, TRAIL_SLAB), -0.012), -150.0))
    colour = g.lerp(colour, g.mul(colour, 0.4), joint)
    # The rubber shoulders, with a soft grain.
    edge = g.saturate(g.mul(g.add(across, -(half - TRAIL_SHOULDER)), 60.0))
    rubber = g.mul(g.const(TRAIL_RUBBER), g.world_noise(0.3, 0.9, 1.1, 1))
    colour = g.lerp(colour, rubber, edge)
    rough = g.lerp(g.const(0.82), g.const(0.95), edge)
    # The yellow dashed line down the middle, 10 cm wide.
    dash = g.link(g.mul(along, 1 / TRAIL_DASH[1]), g.node(unreal.MaterialExpressionFrac))
    dash = g.saturate(g.mul(g.add(dash, -TRAIL_DASH[0] / TRAIL_DASH[1]), -40.0))
    stripe = g.saturate(g.mul(g.add(across, -0.05), -60.0))
    paint = g.mul(stripe, dash)
    colour = g.lerp(colour, g.const(TRAIL_YELLOW), paint)
    rough = g.lerp(rough, g.const(0.6), paint)
    g.output(colour, rough=rough, specular=0.25, metallic=0.0)
    g.material.set_editor_property("used_with_nanite", True)
    return g.finish()


def steel_material():
    """Galvanised steel, a little mottled."""
    g = lib.Graph("M_TrailSteel", f"{FOLDER}/Props")
    colour = g.mul(g.const(TRAIL_STEEL), g.world_noise(0.02, 0.88, 1.08, 2))
    g.output(colour, rough=0.45, specular=0.5, metallic=0.85)
    g.material.set_editor_property("used_with_instanced_static_meshes", True)
    return g.finish()


def fence_mesh_material():
    """The rail's welded mesh panels: steel, see-through. Seen from a bike the wires are a haze, so
    a dithered mask (temporal AA resolves it), not wires that would shimmer."""
    g = lib.Graph("M_TrailMesh", f"{FOLDER}/Props")
    g.output(g.const(TRAIL_STEEL), rough=0.5, specular=0.5, metallic=0.85, opacity_mask=0.35)
    g.material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_MASKED)
    g.material.set_editor_property("dither_opacity_mask", True)
    g.material.set_editor_property("two_sided", True)
    g.material.set_editor_property("used_with_instanced_static_meshes", True)
    return g.finish()


# Its rail, cm: posts 4 cm round to 110 cm over the deck (60 cm into the ground), a 5 cm top rail
# at 105, mesh from 10 to 100. Swan-neck lamps: a 14 cm pole to 520 cm, an arm curving over
# the trail on a 90 cm radius, a flat head at its end.
TRAIL_POST = (4.0, 110.0, 60.0)
TRAIL_LAMP = (14.0, 520.0, 90.0)


def trail_dressing(steel, mesh):
    """trail.json's rail and lamps, from the engine's shapes."""
    with open(f"{SRC}/trail.json") as f:
        trail = json.load(f)
    cube, cylinder = unreal.load_asset(CUBE), unreal.load_asset(CYLINDER)
    sphere = unreal.load_asset(SPHERE)
    rigid, panels = lib.Instances("Trail"), lib.Instances("Trail/Mesh")
    width, top, under = TRAIL_POST
    for x, y, z, _yaw in trail["posts"]:
        tall = top + under
        rigid.add(cylinder, (x, y, z + top - tall / 2), 0.0, (width / 100, width / 100, tall / 100))
    for x0, y0, z0, x1, y1, z1 in trail["spans"]:
        span = math.hypot(x1 - x0, y1 - y0)
        if span < 1.0:
            continue
        yaw = math.degrees(math.atan2(y1 - y0, x1 - x0))
        pitch = math.degrees(math.atan2(z1 - z0, span))
        mx, my, mz = (x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2
        rigid.add(cube, (mx, my, mz + top - 5), yaw, (span / 100, 0.05, 0.05), pitch=pitch)
        panels.add(cube, (mx, my, mz + 55), yaw, (span / 100, 0.01, 0.9), pitch=pitch)
    thick, pole, radius = TRAIL_LAMP
    for x, y, z, yaw in trail["lamps"]:
        rigid.add(
            cylinder, (x, y, z + pole / 2 - 30), 0.0, (thick / 100, thick / 100, (pole + 60) / 100)
        )
        ux, uy = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))
        cx, cy, cz = x + ux * radius, y + uy * radius, z + pole
        # The arm: a chain of spheres round a semicircle from the pole's top, over and down.
        steps = int(math.pi * radius / 3)
        for k in range(steps + 1):
            phi = math.pi * (1 - 0.9 * k / steps)
            rigid.add(
                sphere,
                (
                    cx + ux * radius * math.cos(phi),
                    cy + uy * radius * math.cos(phi),
                    cz + radius * math.sin(phi),
                ),
                0.0,
                0.09,
            )
        phi = math.pi * 0.1
        hx, hy = cx + ux * radius * math.cos(phi), cy + uy * radius * math.cos(phi)
        rigid.add(cylinder, (hx, hy, cz + radius * math.sin(phi) - 8), 0.0, (0.42, 0.42, 0.1))
    rigid.commit(materials={p.get_path_name(): {0: steel} for p in (cube, cylinder, sphere)})
    panels.commit(materials={cube.get_path_name(): {0: mesh}}, shadows=False)
    log(f"trail: {len(trail['posts'])} posts, {len(trail['lamps'])} lamps")


CUBE = "/Engine/BasicShapes/Cube"  # 100 cm, centred
SPHERE = "/Engine/BasicShapes/Sphere"  # 100 cm across, centred
CYLINDER = "/Engine/BasicShapes/Cylinder"  # 100 cm across and tall, centred, along Z


def rails(material, posts_material=None):
    """Each run of rails.json as an old timber fence: round posts every 2 m, a top and a lower
    rail between them. Years of wear: posts lean and sit at uneven heights, rails sag a little,
    a few lower rails are gone and the odd top rail has broken and hangs from one end."""
    with open(f"{SRC}/rails.json") as f:
        runs = json.load(f)
    rng = random.Random(7)
    cube, cylinder = unreal.load_asset(CUBE), unreal.load_asset(CYLINDER)
    parts = lib.Instances("Rail")
    for run in runs:
        posts = run[::2]
        tops = []
        for x, y, z in posts:
            lean = rng.gauss(0, 1.6) if rng.random() > 0.05 else rng.choice((-1, 1)) * 7.0
            height = rng.uniform(-5, 4)
            parts.add(
                cylinder,
                (x, y, z + 35 + height / 2),
                rng.uniform(0, 360),
                (0.14 + rng.uniform(-0.015, 0.02), 0.14, (130 + height) / 100),
                pitch=lean,
                roll=rng.gauss(0, 1.6),
            )
            tops.append(z + height)
        for k in range(len(posts) - 1):
            (x0, y0, z0), (x1, y1, z1) = posts[k], posts[k + 1]
            span = math.hypot(x1 - x0, y1 - y0)
            if span < 50:
                continue
            yaw = math.degrees(math.atan2(y1 - y0, x1 - x0))
            for height, missing in ((RAIL_HEIGHT, 0.0), (45.0, 0.06)):
                if rng.random() < missing:
                    continue
                a = tops[k] - z0 + height + rng.uniform(-2, 2)
                b = tops[k + 1] - z1 + height + rng.uniform(-2, 2)
                pitch = math.degrees(math.atan2((z1 + b) - (z0 + a), span))
                mid = ((x0 + x1) / 2, (y0 + y1) / 2, (z0 + a + z1 + b) / 2 - rng.uniform(0, 2))
                length = span + 10
                if height == RAIL_HEIGHT and rng.random() < 0.03:
                    # Broken: what is left hangs from the first post.
                    length = span * rng.uniform(0.4, 0.65)
                    drop = rng.uniform(20, 40)
                    pitch = -drop
                    reach = length / 2 * math.cos(math.radians(drop))
                    dx, dy = (x1 - x0) / span, (y1 - y0) / span
                    mid = (
                        x0 + dx * reach,
                        y0 + dy * reach,
                        z0 + a - length / 2 * math.sin(math.radians(drop)),
                    )
                parts.add(
                    cube,
                    mid,
                    yaw + rng.uniform(-1.2, 1.2),
                    (length / 100, 0.1, 0.12),
                    pitch=pitch,
                    roll=rng.uniform(-4, 4),
                )
    parts.commit(
        materials={
            cube.get_path_name(): {0: material},
            cylinder.get_path_name(): {0: posts_material or material},
        }
    )
    log(f"rails: {len(runs)} runs")


def _turn(x, y, yaw):
    r = math.radians(yaw)
    return x * math.cos(r) - y * math.sin(r), x * math.sin(r) + y * math.cos(r)


def props(timber, logs):
    """props.json: a bench at the hairpin and stacks of cut logs by the path."""
    path = f"{SRC}/props.json"
    if not os.path.exists(path):
        log("props: none")
        return
    with open(path) as f:
        items = json.load(f)
    rng = random.Random(11)
    cube, cylinder = unreal.load_asset(CUBE), unreal.load_asset(CYLINDER)
    parts = lib.Instances("Props")
    stacks = lib.Instances("Props/Logs")

    def piece(group, mesh, x, y, z, yaw, local, scale, pitch=0.0, roll=0.0):
        dx, dy = _turn(local[0], local[1], yaw)
        group.add(mesh, (x + dx, y + dy, z + local[2]), yaw, scale, pitch=pitch, roll=roll)

    for kind, x, y, z, yaw in items:
        if kind == "bench":
            # Seat 45 cm up, 180 cm long along local Y; the sitter looks along +X.
            for lx in (-18, 18):
                for ly in (-75, 75):
                    piece(parts, cube, x, y, z, yaw, (lx, ly, 20), (0.08, 0.08, 0.5))
            for lx in (-13, 0, 13):
                piece(parts, cube, x, y, z, yaw, (lx, 0, 46), (0.115, 1.9, 0.04))
            for ly in (-75, 75):
                piece(parts, cube, x, y, z, yaw, (-24, ly, 70), (0.07, 0.07, 0.5), pitch=-12)
            for lz in (70, 86):
                piece(
                    parts,
                    cube,
                    x,
                    y,
                    z,
                    yaw,
                    (-27 - (lz - 70) * 0.2, 0, lz),
                    (0.035, 1.9, 0.12),
                    pitch=-12,
                )
        elif kind == "woodpile":
            # Logs along local X, in rows of 5, 4, 3 and 2, each resting in the dip below.
            for row, count in enumerate((5, 4, 3, 2)):
                for i in range(count):
                    d = rng.uniform(24, 32)
                    ly = (i - (count - 1) / 2) * 29 + rng.uniform(-2, 2)
                    lz = 14 + row * 25
                    length = rng.uniform(280, 320)
                    piece(
                        stacks,
                        cylinder,
                        x,
                        y,
                        z,
                        yaw,
                        (rng.uniform(-12, 12), ly, lz),
                        (d / 100, d / 100, length / 100),
                        pitch=90 + rng.uniform(-1.5, 1.5),
                        roll=rng.uniform(0, 360),
                    )
    parts.commit(materials={cube.get_path_name(): {0: timber}})
    stacks.commit(materials={cylinder.get_path_name(): {0: logs}})
    log(f"props: {[i[0] for i in items]}")


def _features():
    path = f"{SRC}/features.json"
    if not os.path.exists(path):
        return None
    with open(path) as f:
        return json.load(f)


def feature_materials():
    """features.json's paints as instances of one plain, two-sided master (thin blades, cables
    and open girders show either face), built before the level loads like the other dressing.
    Returns {slot name: instance}, empty without features."""
    spec = _features()
    if not spec:
        return {}
    folder = f"{LEVEL.rsplit('/', 1)[0]}/Features"
    g = lib.Graph("M_FeaturePaint", folder)
    colour = g.param("colour", (0.5, 0.5, 0.5))
    g.output(colour, rough=g.param("rough", 0.6), metallic=g.param("metal", 0.0), specular=0.4)
    master = g.finish(two_sided=True)
    out = {}
    for slot, (r, gr, b, rough, metal) in spec["paints"].items():
        out[slot] = lib.instance(
            f"MI_Feature_{slot}",
            master,
            folder,
            scalars={"rough": rough, "metal": metal},
            vectors={"colour": (r, gr, b, 1.0)},
        )
    return out


def _feature_mesh(name, folder):
    """feat_<name>.gltf from the source, imported with Nanite off (thin parts: cables, lattice)."""
    task = unreal.AssetImportTask()
    task.filename = f"{SRC}/feat_{name}.gltf"
    task.destination_path = folder
    task.destination_name = f"SM_feat_{name}"
    task.replace_existing = True
    task.automated = True
    task.save = False
    lib.tools.import_asset_tasks([task])
    mesh = next(
        a
        for a in map(unreal.load_asset, task.get_editor_property("imported_object_paths"))
        if isinstance(a, unreal.StaticMesh)
    )
    assets.save_loaded_asset(mesh, False)
    return mesh


def _slots(mesh, paints):
    """{slot index: paint} by the glTF material names the slots carry."""
    out = {}
    for i, m in enumerate(mesh.get_editor_property("static_materials")):
        name = str(m.get_editor_property("material_slot_name"))
        for paint, material in paints.items():
            if name == paint or name.startswith(f"{paint}_") or name.endswith(f"_{paint}"):
                out[i] = material
    return out


def features(paints):
    """features.json (make_features.py): the bridges, harbour, wind farm, power lines and rock
    formations. Kit meshes and the Megascans rocks go into instanced groups; the turbines'
    rotors are actors of their own, spun by a rotating movement."""
    spec = _features()
    if not spec:
        log("features: none")
        return
    folder = f"{LEVEL.rsplit('/', 1)[0]}/Features"
    meshes, materials = {}, {}
    for key, source in spec["meshes"].items():
        if source.startswith("gltf:"):
            mesh = _feature_mesh(source[5:], folder)
            materials[mesh.get_path_name()] = _slots(mesh, paints)
        else:
            mesh = unreal.load_asset(source)
            if mesh is None:
                unreal.log_warning(f"CLIMB features: no {source}")
                continue
            if source.startswith("/Engine/"):  # the quay's cubes: concrete
                materials[mesh.get_path_name()] = {0: paints["concrete"]}
        meshes[key] = mesh
    groups = {name: lib.Instances(f"Features/{name}") for name in spec["groups"]}
    count = 0
    for group, key, x, y, z, yaw, scale, pitch, roll in spec["items"]:
        if key not in meshes:
            continue
        groups[group].add(meshes[key], (x, y, z), yaw, scale, pitch=pitch, roll=roll)
        count += 1
    for name, instances in groups.items():
        g = spec["groups"][name]
        instances.commit(cull=g["cull"] or None, shadows=g["shadows"], materials=materials)
    for key, x, y, z, yaw, rate, phase in spec["spin"]:
        mesh = meshes[key]
        actor = lib.mesh_actor(
            mesh,
            (x, y, z),
            yaw,
            label="Rotor",
            folder="Features/Rotors",
            materials=materials.get(mesh.get_path_name()),
            roll=phase,
        )
        actor.static_mesh_component.set_editor_property(
            "mobility", unreal.ComponentMobility.MOVABLE
        )
        root = lib.subobjects.k2_gather_subobject_data_for_instance(actor)[0]
        handle, _fail = lib.subobjects.add_new_subobject(
            unreal.AddNewSubobjectParams(
                parent_handle=root,
                new_class=unreal.RotatingMovementComponent,
                blueprint_context=None,
            )
        )
        spin = unreal.SubobjectDataBlueprintFunctionLibrary.get_object(
            unreal.SubobjectDataBlueprintFunctionLibrary.get_data(handle)
        )
        spin.set_editor_property("rotation_rate", unreal.Rotator(rate, 0.0, 0.0))  # roll: about X
        spin.set_editor_property("rotation_in_local_space", True)
    log(f"features: {count} placed, {len(spec['spin'])} rotors")


def track():
    """The ride as the level's DA_RideTrack: up the stretch and back down, or, for the world's
    rides (info.json's "rides", the default first), once round each loop by stretches, with no
    weather rows so the level's own light stays. The default is DA_RideTrack, which lists the
    others, each DA_Route_<key>, for the start page's route choice."""
    with open(f"{SRC}/routes.json") as f:
        routes = json.load(f)
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    rides = info.get("rides") or ([info["ride"]] if info.get("ride") else [None])
    others = [
        _track(f"DA_Route_{r['key']}", routes["routes"][r["key"]], r, info) for r in rides[1:]
    ]
    _track("DA_RideTrack", routes["island"], rides[0], info, others)


def _track(asset, centre, ride, info, others=()):
    cls = unreal.load_class(None, "/Script/VeloverseUE.RideTrack")
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", cls)
    t = lib.fresh(asset, RIDE, cls, factory)
    # The rider's line 6 cm under the tarmac, as on the island.
    t.set_editor_property(
        "centre", [unreal.Vector(x * 100, y * 100, z * 100 - 6) for x, y, z in centre]
    )
    t.set_editor_property("closed", False)
    t.set_editor_property("title", "Hero stretch")
    t.set_editor_property("blurb", "A kilometre up through the beech and spruce, and back down.")
    t.set_editor_property("step", 100.0)
    # cm left of the centre line: the hero's wide climb keeps well left; the world's narrow
    # path and two-lane asphalt ride near the middle (info.json's "keep_left").
    t.set_editor_property("keep_left", float(info.get("keep_left", 160.0)))
    t.set_editor_property("sun_azimuth", -62.0)
    t.set_editor_property("routes", list(others))
    coast = unreal.load_asset("/Game/Coast/Ride/DA_RideTrack")
    for prop in ("rain_shell", "collection"):
        t.set_editor_property(prop, coast.get_editor_property(prop))
    row = unreal.RideWeather()
    row.set_editor_property("metres", 0.0)
    t.set_editor_property("weather", [row])
    stretch = unreal.RideStretch()
    stretch.set_editor_property("name", "Hero stretch")
    t.set_editor_property("stretches", [stretch])
    if ride:
        t.set_editor_property("closed", ride["closed"])
        t.set_editor_property("title", ride["title"])
        t.set_editor_property("blurb", ride["blurb"])
        t.set_editor_property("weather", [])
        # info.json gives each stretch's length; the track wants where each starts.
        runs, start = [], 0.0
        for name, metres in ride["stretches"]:
            run = unreal.RideStretch()
            run.set_editor_property("name", name)
            run.set_editor_property("metres", start)
            runs.append(run)
            start += float(metres)
        t.set_editor_property("stretches", runs)
        t.set_editor_property("start_at", ride.get("start", ""))
    assets.save_loaded_asset(t, False)
    log(f"track {asset}: {len(centre)} m, {ride['title'] + ' loop' if ride else 'out and back'}")
    return t


DRESSING = (
    "Road",
    "Rail",
    "Trees",
    "Props",
    "Sea",
    "Features",
)  # outliner folders the dress stage owns


def drop_sky_dome():
    """The OpenWorld template's SM_SkySphere: the level's SkyAtmosphere draws the sky, and the
    dome, not covering the whole view, printed the engine's "skydome mesh" warning in rides."""
    for actor in actors.get_all_level_actors():
        if not isinstance(actor, unreal.StaticMeshActor):
            continue
        mesh = actor.static_mesh_component.get_editor_property("static_mesh")
        if mesh is not None and mesh.get_name() == "SM_SkySphere":
            actors.destroy_actor(actor)
            log("dropped the template's sky sphere")


def fresh_level(ground):
    if assets.does_asset_exist(LEVEL):
        assets.delete_asset(LEVEL)
    world = unreal.EditorLoadingAndSavingUtils.new_map_from_template(
        "/Engine/Maps/Templates/OpenWorld", False
    )
    for actor in actors.get_all_level_actors():
        if isinstance(actor, unreal.LandscapeProxy):
            actors.destroy_actor(actor)
    drop_sky_dome()
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
    drop_sky_dome()  # levels built before the template's dome was dropped
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
        island.FOLDER = FOLDER
        road = road_material()  # before the level loads: no compile with its meshes in view
        path = path_material() if _has_paths() else None
        if os.environ.get("OC_PATH") == "road":  # bisecting: the path pieces in the road's look
            path = None
        trail = trail_materials() if _has_trail() else None
        water = sea_material()
        horizon = far_material()
        dressing = dressing_materials()
        paints = feature_materials()
        yield
        world = undress()
    else:
        ground, road = materials()
        path = path_material() if _has_paths() else None
        trail = trail_materials() if _has_trail() else None
        water = sea_material()
        horizon = far_material()
        dressing = dressing_materials()
        paints = feature_materials()
        yield
        world = fresh_level(ground)
    yield
    save(world)
    if STAGE == "ground":
        return
    yield
    roads(road, path, trail[0] if trail else None)
    if _has_paths():  # the world: its ground writes the RVT the road edges read
        rvt_volume()
    timber, posts, logs = dressing
    if posts is not None:
        rails(timber, posts)
    if logs is not None:
        props(timber, logs)
    if trail is not None and _listed("trail.json"):
        trail_dressing(trail[1], trail[2])
    track()
    sea(water)
    if horizon is not None:
        far(horizon)
    yield
    save(world)
    yield
    if paints:
        features(paints)
        yield
        save(world)
        yield
    # The trees last, saved on their own: thousands of actors on top of a fresh landscape
    # got the editor killed for memory as it saved them all together.
    trees()
    yield
    save(world)
    yield from stills(world)


def _views():
    """info.json's views ([name, eye, target] in m), if the generator chose them: the world's
    ride has one per stretch."""
    try:
        with open(f"{SRC}/info.json") as f:
            views = json.load(f).get("views")
    except OSError:
        return None
    views = list(views or []) + list((_features() or {}).get("views", []))
    if not views:
        return None
    return [(n, tuple(v * 100 for v in p), tuple(v * 100 for v in a)) for n, p, a in views]


def cameras():
    """(name, position, look-at) in cm: a rider's eye along the road, looking 30 m up it, and
    one from above the hairpin back down the lower traverse; or info.json's views."""
    views = _views()
    if views:
        return views
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
    # The bench from the path, 7 m in front of it and a little to one side.
    if os.path.exists(f"{SRC}/props.json"):
        with open(f"{SRC}/props.json") as f:
            bench = [p for p in json.load(f) if p[0] == "bench"]
        if bench:
            _, bx, by, bz, yaw = bench[0]
            r = math.radians(yaw + 20)
            eye = (bx / 100 + 7 * math.cos(r), by / 100 + 7 * math.sin(r), bz / 100 + 1.6)
            out.append(("bench", eye, (bx / 100, by / 100, bz / 100 + 0.5)))
    return [(n, tuple(v * 100 for v in p), tuple(v * 100 for v in a)) for n, p, a in out]


def stills(world):
    """Shot from the editor's own viewport, moved to each view: a stand-alone camera doesn't
    drive grass, texture streaming or tree LODs, so its shots had no grass and blocky trees."""
    os.makedirs(STILLS, exist_ok=True)
    editor = unreal.UnrealEditorSubsystem()
    yield 60  # let the level's streaming, Nanite and lighting settle
    for name, where, target in cameras():
        eye = unreal.Vector(*where)
        rot = unreal.MathLibrary.find_look_at_rotation(eye, unreal.Vector(*target))
        editor.set_level_viewport_camera_info(eye, rot)
        yield 120  # grass, textures and shadows stream in around the new view
        unreal.AutomationLibrary.take_high_res_screenshot(1280, 720, f"{STILLS}/{name}.png")
        yield 30  # the shot is written a few frames later
        log(f"still {STILLS}/{name}.png")


def open_level():
    """L_Hero with every actor loaded. Reloading it while it's open crashed the editor."""
    world = unreal.UnrealEditorSubsystem().get_editor_world()
    if not world or world.get_path_name().split(".")[0] != LEVEL:
        world = unreal.EditorLoadingAndSavingUtils.load_map(LEVEL)
    wp = getattr(unreal, "WorldPartitionBlueprintLibrary", None)
    if wp is not None:
        descs = wp.get_actor_descs()
        if os.environ.get("OC_NO_SKELETAL") or globals().get("NO_SKELETAL"):
            # The skeletal PVE trees overflow the GPU's skinning buffer, and the Radeon driver
            # has crashed three times with them loaded.
            keep = [d for d in descs if "SkeletalMeshActor" not in str(d.native_class)]
            log(f"open_level: leaving {len(descs) - len(keep)} skeletal trees unloaded")
            descs = keep
        wp.load_actors([d.guid for d in descs])
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
    unreal.EditorPythonScripting.set_keep_python_script_alive(True)
    steps = main()
    handle = unreal.register_slate_post_tick_callback(tick)
