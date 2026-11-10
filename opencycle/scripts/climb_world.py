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
LEVEL = f"{FOLDER}/L_Hero"
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
    """Whether a loaded landscape draws with M_ClimbGround, directly or through an instance. (The
    empty startup map has a landscape of its own, which a rebuild doesn't touch.)"""
    for actor in unreal.EditorActorSubsystem().get_all_level_actors():
        if not isinstance(actor, unreal.LandscapeProxy):
            continue
        material = actor.get_editor_property("landscape_material")
        while material is not None:
            if material.get_name() == "M_ClimbGround":
                return True
            if not isinstance(material, unreal.MaterialInstance):
                break
            material = material.get_editor_property("parent")
    return False


def ground_material():
    """M_ClimbGround, rebuilt with this climb's surfaces; the saved grass types are reused
    (building them takes over ten minutes, on one core) unless OC_REBUILD is set.

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
    return island.Ground("M_ClimbGround").build(grass)


def ground_instance(parent):
    """MI_ClimbGround, the landscape's material: M_ClimbGround with its tints set by tune()."""
    path = f"{FOLDER}/MI_ClimbGround"
    mi = unreal.load_asset(path)
    if mi is None:
        mi = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
            "MI_ClimbGround",
            FOLDER,
            unreal.MaterialInstanceConstant,
            unreal.MaterialInstanceConstantFactoryNew(),
        )
    if mi.get_editor_property("parent") != parent:
        unreal.MaterialEditingLibrary.set_material_instance_parent(mi, parent)
    unreal.EditorAssetLibrary.save_loaded_asset(mi, False)
    return mi


def tune(tints):
    """Recolours the ground's surfaces on MI_ClimbGround, e.g. tune({"Dirt": (0.6, 0.62, 0.65)}),
    keys being SURFACES' layers (or "<layer> mix" for a MIXES surface). Parameters only: nothing
    compiles, so it's safe, and quick, with the level open."""
    mi = unreal.load_asset(f"{FOLDER}/MI_ClimbGround")
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
    ground = unreal.load_asset(f"{FOLDER}/M_ClimbGround")
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


ROAD_DARKEN = 0.3  # the island's asphalt read washed-out grey on the climb (0.4 still pale, v13)
# The scan's bright aggregate read as a regular white speckle (QA, v12): clamped before darkening.
ROAD_SPECK_CLAMP = 0.3
# The edge's soil_mud, darkened and browned to the ground's damp litter.
ROAD_EDGE_SOIL = (0.45, 0.38, 0.32)
# Old country tarmac is warm grey-brown, not the scan's blue-grey (QA, v11).
ROAD_WARM = (1.1, 1.0, 0.8)


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
    # Wear: stretches of cracked old tarmac, darker patched repairs, soil and litter washed onto
    # the edges, and fallen leaves, most of them near the edges.
    ctile = island.tile_cm("shared", "cracked_asphalt")
    cd, cn = (island.ms_texture("shared", "cracked_asphalt", k) for k in ("D", "N"))
    cuv = g.world_uv(ctile, (0.21, 0.47))
    # Partly cracked, in patches a few metres across with soft edges: a hard mask at 8 m drew big
    # dark ellipses on the path.
    cracked = g.mul(g.saturate(g.mul(g.add(g.world_noise(0.004, -1.0, 2.0, 3), -0.4), 1.5)), 0.55)
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
    patch = g.saturate(g.mul(g.add(noise, -0.8), 40.0))
    colour = g.lerp(colour, g.mul(colour, 0.72), patch)
    colour = g.mul(colour, g.world_noise(0.001, 0.94, 1.04, 3))
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
    edge = g.saturate(g.mul(g.add(g.add(across, g.mul(ragged, 0.6)), -1.0), 3.0))
    stile = island.tile_cm("forest", "soil_mud")
    soil = g.mask(g.sample(island.ms_texture("forest", "soil_mud", "D"), g.world_uv(stile)), "rgb")
    soil = g.mul(soil, g.const(ROAD_EDGE_SOIL))
    colour = g.lerp(colour, soil, g.mul(edge, 0.85))
    leaves = g.saturate(g.mul(g.add(g.world_noise(0.07, 0.0, 1.0, 2), -0.72), 7.0))
    leaves = g.mul(leaves, g.lerp(g.const(0.12), g.const(1.0), edge))
    leaf_colour = g.mul(g.const((0.11, 0.07, 0.04)), g.world_noise(0.03, 0.6, 1.4, 2))
    colour = g.lerp(colour, leaf_colour, leaves)
    rough = g.lerp(rough, g.const(0.92), g.saturate(g.add(edge, leaves)))
    g.output(colour, normal, rough, specular=0.3, ao=g.mask(masks, "r"))
    g.material.set_editor_property("used_with_nanite", True)
    return g.finish()


CUBE = "/Engine/BasicShapes/Cube"  # 100 cm, centred
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


DRESSING = ("Road", "Rail", "Trees", "Props")  # outliner folders the dress stage owns


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
        island.FOLDER = FOLDER
        road = road_material()  # before the level loads: no compile with its meshes in view
        yield
        world = undress()
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
    timber = rail_material()
    rails(timber, post_material())
    props(timber, log_material())
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
