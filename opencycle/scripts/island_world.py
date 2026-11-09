"""Builds /Game/Island/L_Island from make_island.py's full output: a World Partition level with
the island's landscape (8 km at a metre a sample), painted from Megascans surfaces, growing its
grass per region, with the roads, the sea, and a ride track for each of make_island.py's routes.

Run after megascans.py (both stages) and OC_RES=8129 python3 -I scripts/make_island.py:
    OC_EDITOR=1 bash scripts/ue_run.sh island_world.py island_world.log
It needs the whole editor, as trial_world.py does. OC_SRC picks another make_island.py output, e.g.
/workspace/ue/world_gen/island_1025 for a quick, coarse build.

OC_STAGE=materials builds only the materials and grass types, and OC_STAGE=level only the level,
from the saved materials. Run them in that order, as two editors, for the full island: building a
material leaves every texture it samples resident on the GPU, about 8 GB for the ground's, which
with the landscape's import is more than an integrated GPU's share of memory. A material that is
only loaded keeps its textures out until something draws them.

OC_STAGE=landmarks then stands make_island.py's landmarks in the saved level, in a third editor:
with the landscape's import still in memory, the level's editor has no room left for them. Run it
once after each level build; the sea's tag marks a level that has them.
"""

import importlib.util
import json
import math
import os
import re
import traceback

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/island_world.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
Graph = lib.Graph
assets, tools, actors = lib.assets, lib.tools, lib.actors

SRC = os.environ.get("OC_SRC", "/workspace/ue/world_gen/island")
STAGE = os.environ.get("OC_STAGE", "all")  # "materials", "level", "landmarks", or "all"
PREP = "/workspace/ue/fab_prep"
FOLDER = "/Game/Island"
LEVEL = f"{FOLDER}/L_Island"
MS = "/Game/Megascans"
QUADS = 127  # per landscape component
GRID = 4  # components a side per streaming proxy: about 500 m

# Each paint layer's surfaces: (biome, slug, tint). A second surface is mixed in by `MIXES`.
SURFACES = {
    "Grass": ("shared", "uncut_grass", (0.85, 0.9, 0.7)),
    "Forest": ("forest", "forest_floor", (1.0, 1.0, 1.0)),
    "Rock": ("shared", "stony_soil", (0.72, 0.72, 0.74)),
    "Snow": ("coast", "thai_wet_sand", (1.0, 1.0, 1.0)),
    "Desert": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
    "Sand": ("coast", "trampled_beach_sand", (1.0, 1.0, 1.0)),
    "Dirt": ("shared", "gravel_on_soil", (0.78, 0.76, 0.72)),
}
# A second surface per layer and what mixes it in: "patches" (noise, metres across) or "wet"
# (below the given height in cm, the tideline).
MIXES = {
    "Forest": ("forest", "mossy_forest_floor", "patches", 18.0),
    "Desert": ("desert", "dry_mud_ground", "patches", 60.0),
    "Sand": ("coast", "thai_wet_sand", "wet", 120.0),
    "Grass": ("shared", "lawn_grass", "patches", 30.0),
    "Dirt": ("shared", "dirt_road", "patches", 8.0),
}
FAR = 6.0  # far off, each surface is also sampled this many times larger, against tiling
ROUGHNESS = {"Snow": 0.55, "Sand": 0.9}
# Layers that keep a surface's relief but not its colour: shaded from the first colour in its
# hollows to the second on its crests. There's no snow scan, so snow is wet sand's smooth relief
# in white: rippled sand's read as dunes.
FLAT = {"Snow": ((0.62, 0.68, 0.78), (0.86, 0.89, 0.93))}

# Grass per layer: (biome, slug, instances per 100 m^2 (Unreal's 10 m square) of each of the scan's
# meshes, cull start, cull end in cm, and for rocks and logs the scale range). A meadow wants
# several clumps a square metre. Scans whose meshes take 5 GB or more to build are left out:
# with the rest of the editor they run the machine out of memory.
GRASS = {
    "Grass": (
        ("shared", "wild_grass", 40.0, 3500, 5000),
        ("shared", "grass_clumps", 24.0, 3500, 5000),
        ("shared", "wheat_grass", 10.0, 3500, 5000),
        ("shared", "tundra_grass", 6.0, 3500, 5000),
        ("shared", "kikuyu_grass", 8.0, 3500, 5000),
        ("shared", "tall_grass", 2.4, 4000, 6000),
        ("shared", "ribbon_grass", 1.6, 3000, 4500),
        ("shared", "clovers", 12.0, 2000, 3000),
        ("shared", "clover_varieties", 6.0, 2000, 3000),
        ("shared", "leafy_dandelions", 4.0, 2000, 3000),
    ),
    "Forest": (
        ("shared", "fern", 3.6, 4000, 6000),
        ("shared", "lady_fern", 2.4, 4000, 6000),
        ("shared", "common_fern", 2.4, 4000, 6000),
        ("shared", "leather_fern", 1.2, 4000, 6000),
        ("shared", "grass_clumps", 6.0, 3000, 4500),
        ("forest", "mossy_rocks", 0.25, 8000, 11000, (0.8, 2.2)),
        ("forest", "nordic_forest_rock_small", 0.3, 6000, 9000, (0.8, 2.0)),
        ("forest", "nordic_forest_cluster_rock_medium", 0.08, 9000, 12000, (0.8, 1.8)),
        ("forest", "nordic_forest_tree_trunk_spruce_large", 0.08, 7000, 10000, (0.8, 1.3)),
        ("forest", "oak_tree_trunk", 0.05, 7000, 10000, (0.8, 1.2)),
    ),
    "Desert": (
        ("desert", "dry_grass", 1.2, 4000, 6000),
        ("desert", "dead_shrubs", 0.15, 6000, 9000),
        ("desert", "desert_american_plant_cactus_medium_11", 0.01, 12000, 16000),
        ("desert", "desert_american_plant_branch_cactus_large_03", 0.005, 15000, 20000),
        ("desert", "canyon_sandstone_boulder", 0.5, 7000, 10000, (0.8, 3.0)),
        ("desert", "desert_western_cluster_rock_medium_05", 0.04, 10000, 14000, (0.8, 1.6)),
        ("desert", "desert_western_ground_patch_rocky_02", 0.05, 6000, 9000, (0.8, 1.4)),
    ),
    "Sand": (
        ("coast", "marram_grass", 4.5, 4000, 6000),
        ("coast", "thai_beach_log", 0.03, 6000, 9000, (1.0, 1.6)),
        ("coast", "nordic_beach_rocks", 0.03, 8000, 12000, (0.8, 1.6)),
    ),
    "Rock": (
        ("mountain", "alpine_bearberry", 0.8, 3000, 4500),
        ("mountain", "limestone_rocks", 0.15, 7000, 10000, (0.8, 1.8)),
        ("mountain", "tundra_boulder", 0.05, 9000, 13000, (0.6, 1.6)),
        ("mountain", "icelandic_rock_plates", 0.03, 8000, 12000, (0.7, 1.3)),
    ),
}
SEA_LEVEL = 0.0


def log(message):
    unreal.log(f"ISLAND {message}")


def prepared(biome, slug):
    with open(f"{PREP}/{biome}/{slug}/asset.json") as f:
        return json.load(f)


def tile_cm(biome, slug):
    """A surface's tile from its scan's size, e.g. "2x2" metres; 2 m when it doesn't say."""
    found = re.findall(r"[\d.]+", str(prepared(biome, slug).get("scan_area") or ""))
    return float(found[0]) * 100 if found else 200.0


def ms_texture(biome, slug, key):
    path = f"{MS}/{biome}/{slug}/T_{slug}_{key}"
    if not assets.does_asset_exist(path):
        raise RuntimeError(f"no {path}: run megascans.py first")
    return unreal.load_asset(path)


# --- the ground ----------------------------------------------------------------------------


class Ground:
    """The landscape material: each layer's surfaces height-blended, sampled twice, near and far,
    and varied in brightness over tens of metres, so 8 km of ground doesn't show its tiles."""

    def __init__(self, name="M_IslandGround"):
        self.g = Graph(name, FOLDER)
        g = self.g
        world = g.node(unreal.MaterialExpressionWorldPosition)
        camera = g.node(unreal.MaterialExpressionCameraPositionWS)
        distance = g.node(unreal.MaterialExpressionDistance)
        g.link(world, distance, "A")
        g.link(camera, distance, "B")
        # 0 within 15 m, 1 past 60 m.
        self.far = g.saturate(g.mul(g.add(distance, -1500.0), 1 / 4500))
        self.height = g.mask(world, "b")
        self.macro = g.world_noise(0.0004, 0.82, 1.12, 4)
        # Steep ground is sampled from the side as well, projected on whichever of the XZ and YZ
        # planes it faces more, or a top-down sample streaks down a cliff. 0 up to 37 degrees, 1
        # past 53.
        up = g.node(unreal.MaterialExpressionVertexNormalWS)
        self.steep = g.saturate(g.mul(g.add(g.mask(up, "b"), -0.8), -5.0))
        across = g.add(
            g.link(g.mask(up, "r"), g.node(unreal.MaterialExpressionAbs)),
            g.mul(g.link(g.mask(up, "g"), g.node(unreal.MaterialExpressionAbs)), -1.0),
        )
        faces_x = g.saturate(g.mul(across, 1000.0))
        xz = g.op(unreal.MaterialExpressionAppendVector, g.mask(world, "r"), self.height)
        yz = g.op(unreal.MaterialExpressionAppendVector, g.mask(world, "g"), self.height)
        self.side = g.lerp(xz, yz, faces_x)

    def surface(self, biome, slug, tint):
        """A surface's colour, normal, roughness and height, near blended into far."""
        g = self.g
        tile = tile_cm(biome, slug)
        d, n, m = (ms_texture(biome, slug, k) for k in ("D", "N", "M"))
        near_uv, far_uv = g.world_uv(tile), g.world_uv(tile * FAR, (0.37, 0.61))
        near_d = g.mask(g.sample(d, near_uv), "rgb")
        far_d = g.mask(g.sample(d, far_uv), "rgb")
        near_n = g.mask(g.sample(n, near_uv, "normal"), "rgb")
        far_n = g.mask(g.sample(n, far_uv, "normal"), "rgb")
        side_d = g.mask(g.sample(d, g.mul(self.side, 1.0 / tile)), "rgb")
        masks = g.sample(m, near_uv, "masks")
        top = g.lerp(near_d, far_d, g.mul(self.far, 0.7))
        colour = g.mul(g.lerp(top, side_d, self.steep), g.const(tint))
        # The top-down normal map streaks on a cliff too; there it fades to the ground's own.
        normal = g.lerp(g.lerp(near_n, far_n, self.far), g.const((0.0, 0.0, 1.0)), self.steep)
        return {
            "colour": colour,
            "normal": normal,
            "rough": g.mask(masks, "g"),
            "height": g.mask(masks, "b"),
        }

    def layer(self, name):
        g = self.g
        biome, slug, tint = SURFACES[name]
        base = self.surface(biome, slug, tint)
        if name in MIXES:
            biome2, slug2, how, size = MIXES[name]
            other = self.surface(biome2, slug2, (1.0, 1.0, 1.0))
            if how == "patches":
                alpha = g.world_noise(1.0 / (size * 100), -1.5, 2.0, 3)
            else:  # wet below a height
                alpha = g.mul(g.add(g.mul(self.height, -1.0), size), 1 / 60)
            # The mix follows the surfaces' own heights at its edge, not a soft fade.
            alpha = g.saturate(g.add(alpha, g.add(other["height"], g.mul(base["height"], -1.0))))
            base = {k: g.lerp(base[k], other[k], alpha) for k in base}
        if name in FLAT:
            low, high = FLAT[name]
            base["colour"] = g.lerp(g.const(low), g.const(high), base["height"])
        if name in ROUGHNESS:
            base["rough"] = g.const(ROUGHNESS[name])
        return base

    def build(self, grass_types):
        g = self.g
        layers = {name: self.layer(name) for name in SURFACES}
        outputs = {}
        for prop in ("colour", "normal", "rough"):
            blend = g.node(unreal.MaterialExpressionLandscapeLayerBlend)
            rows = []
            for name in layers:
                row = unreal.LayerBlendInput()
                row.set_editor_property("layer_name", name)
                row.set_editor_property(
                    "blend_type", unreal.LandscapeLayerBlendType.LB_HEIGHT_BLEND
                )
                rows.append(row)
            blend.set_editor_property("layers", rows)
            for name, parts in layers.items():
                g.link(parts[prop], blend, f"Layer {name}")
                g.link(parts["height"], blend, f"Height {name}")
            outputs[prop] = blend
        colour = g.mul(outputs["colour"], self.macro)
        g.output(colour, outputs["normal"], outputs["rough"], specular=0.35)
        out = g.node(unreal.MaterialExpressionLandscapeGrassOutput)
        inputs = []
        for name, gt in grass_types.items():
            gi = unreal.GrassInput()
            gi.set_editor_property("name", name)
            gi.set_editor_property("grass_type", gt)
            inputs.append(gi)
        out.set_editor_property("grass_types", inputs)
        for name in grass_types:
            sample = g.node(unreal.MaterialExpressionLandscapeLayerSample, parameter_name=name)
            weight = sample
            if name == "Sand":  # marram on the dunes behind the beach, not at the water
                above = g.saturate(g.mul(g.add(self.height, -350.0), 1 / 250))
                weight = g.mul(sample, above)
            g.link(weight, out, name)
        return g.finish()


def grass_type(layer):
    gt = lib.fresh(
        f"GT_{layer}",
        f"{FOLDER}/Grass",
        unreal.LandscapeGrassType,
        unreal.LandscapeGrassTypeFactory(),
    )
    varieties = []
    for biome, slug, density, cull_start, cull_end, *scale in GRASS[layer]:
        meshes = [
            a
            for a in map(unreal.load_asset, assets.list_assets(f"{MS}/{biome}/{slug}", False))
            if isinstance(a, unreal.StaticMesh)
        ]
        if not meshes:
            raise RuntimeError(f"no meshes for {biome}/{slug}: run megascans.py first")
        for mesh in meshes:
            v = unreal.GrassVariety()
            v.set_editor_property("grass_mesh", mesh)
            v.set_editor_property("grass_density", unreal.PerPlatformFloat(default=density))
            v.set_editor_property("start_cull_distance", unreal.PerPlatformInt(default=cull_start))
            v.set_editor_property("end_cull_distance", unreal.PerPlatformInt(default=cull_end))
            v.set_editor_property("scaling", unreal.GrassScaling.UNIFORM)
            v.set_editor_property("scale_x", unreal.FloatInterval(*(scale or [(0.85, 1.3)])[0]))
            v.set_editor_property("random_rotation", True)
            # Lying things follow the slope; cacti and shrubs stand up straight.
            v.set_editor_property("align_to_surface", density > 0.5 or bool(scale))
            v.set_editor_property("cast_dynamic_shadow", True)
            varieties.append(v)
    gt.set_editor_property("grass_varieties", varieties)
    assets.save_loaded_asset(gt, False)
    log(f"grass {layer}: {len(varieties)} varieties")
    return gt


# --- the rest ------------------------------------------------------------------------------


def road_material():
    """Asphalt from the Megascans set, world-mapped, with the same near and far trick."""
    g = Graph("M_IslandRoad", f"{FOLDER}/Road")
    tile = tile_cm("urban", "asphalt_road")
    d, n, m = (ms_texture("urban", "asphalt_road", k) for k in ("D", "N", "M"))
    uv = g.world_uv(tile)
    masks = g.sample(m, uv, "masks")
    colour = g.mul(g.mask(g.sample(d, uv), "rgb"), g.world_noise(0.001, 0.85, 1.08, 3))
    g.output(
        colour,
        g.mask(g.sample(n, uv, "normal"), "rgb"),
        g.mask(masks, "g"),
        specular=0.4,
        ao=g.mask(masks, "r"),
    )
    g.material.set_editor_property("used_with_nanite", True)
    return g.finish()


def roads(material):
    count = 0
    for name in sorted(os.listdir(SRC)):
        if not (name.startswith("road_") and name.endswith(".gltf")):
            continue
        stem = name[:-5]
        task = unreal.AssetImportTask()
        task.filename = f"{SRC}/{name}"
        task.destination_path = f"{FOLDER}/Road"
        task.destination_name = f"SM_{stem}"
        task.replace_existing = True
        task.automated = True
        task.save = False
        tools.import_asset_tasks([task])
        mesh = next(
            a
            for a in map(unreal.load_asset, task.get_editor_property("imported_object_paths"))
            if isinstance(a, unreal.StaticMesh)
        )
        nanite = mesh.get_editor_property("nanite_settings")
        nanite.set_editor_property("enabled", True)
        mesh.set_editor_property("nanite_settings", nanite)
        assets.save_loaded_asset(mesh, False)
        lib.mesh_actor(mesh, label=stem, folder="Road", materials={0: material})
        count += 1
    log(f"roads: {count} pieces")


def landmarks():
    """The rocks and the summit viewpoint make_island placed, from {SRC}/landmarks.json."""
    path = f"{SRC}/landmarks.json"
    if not os.path.exists(path):
        log("no landmarks.json: none placed")
        return
    with open(path) as f:
        placed = json.load(f)
    meshes = {}
    for k, item in enumerate(placed):
        slug = item["mesh"]
        if slug not in meshes:
            # Only the mesh: loading the folder's every asset brings in its textures, gigabytes.
            meshes[slug] = next(
                unreal.load_asset(path)
                for path in assets.list_assets(f"{MS}/{slug}", False)
                if str(assets.find_asset_data(path).asset_class_path.asset_name) == "StaticMesh"
            )
        lib.mesh_actor(
            meshes[slug],
            (item["x"] * 100, item["y"] * 100, item["z"] * 100),
            yaw=item["yaw"],
            scale=item["scale"],
            label=f"{slug.split('/')[-1]}_{k}",
            folder="Landmarks",
        )
    log(f"landmarks: {len(placed)}")


def sea(extent_cm):
    """A plane of the coast's sea out past the map's edge, loaded always."""
    plane = unreal.load_asset("/Engine/BasicShapes/Plane")
    material = unreal.load_asset(f"{lib.FOLDER}/Materials/M_Sea")
    scale = extent_cm * 3 / 100
    actor = lib.mesh_actor(
        plane, (0, 0, SEA_LEVEL), scale=(scale, scale, 1), label="Sea", folder="Sea"
    )
    if material:
        actor.static_mesh_component.set_material(0, material)
    actor.set_editor_property("is_spatially_loaded", False)
    log("sea")


# The start screen's routes from make_island.py's routes.json, in its order: title and a line.
ROUTES = {
    "island": ("Island loop", "The whole coast road round the island, through every region."),
    "coast": ("Coast", "Along the south shore and back over the low road behind the harbour."),
    "forest": (
        "Forest",
        "Under the trees on the west side and home by the coast road.",
    ),
    "meadow": ("Meadows", "The green valley in the middle of the island and the lanes round it."),
    "desert": ("Desert", "Past the mesas in the dry north-east and back across the island."),
    "dunes": ("Dunes", "The sand country in the south-east, between the dunes and the sea."),
    "summit": ("Summit", "Up the hairpins to the snow line and the viewpoint, and back down."),
}


def retraced(centre):
    """Whether a route rides out and back: its second half lies on its first."""
    n = len(centre)
    return all(
        math.dist(centre[k][:2], centre[n - 1 - k][:2]) < 5.0 for k in (n // 8, n // 4, 3 * n // 8)
    )


def track(name, centre, others=()):
    """A route as a ride track in {FOLDER}/Ride: the island loop as DA_RideTrack, which the level
    rides by default and which lists the others, each as DA_Route_<name>."""
    title, blurb = ROUTES.get(name, (name.title(), ""))
    cls = unreal.load_class(None, "/Script/OpenCycleUE.RideTrack")
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", cls)
    asset = "DA_RideTrack" if name == "island" else f"DA_Route_{name}"
    t = lib.fresh(asset, f"{FOLDER}/Ride", cls, factory)
    # The rider's line 6 cm under the tarmac, as in the trial world.
    t.set_editor_property(
        "centre", [unreal.Vector(x * 100, y * 100, z * 100 - 6) for x, y, z in centre]
    )
    # A loop joins up (its ends within a few metres); out and back finishes where it started.
    closed = math.dist(centre[0][:2], centre[-1][:2]) < 5.0 and not retraced(centre)
    t.set_editor_property("closed", closed)
    t.set_editor_property("title", title)
    t.set_editor_property("blurb", blurb)
    t.set_editor_property("routes", list(others))
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
    stretch.set_editor_property("name", title)
    t.set_editor_property("stretches", [stretch])
    assets.save_loaded_asset(t, False)
    log(f"route {name}: {len(centre)} m, {'loop' if closed else 'out and back'}")
    return t


def tracks():
    """Every route in routes.json, the island loop last so its list can name the others."""
    with open(f"{SRC}/routes.json") as f:
        routes = json.load(f)
    others = [track(name, centre) for name, centre in routes.items() if name != "island"]
    track("island", routes["island"], others)


def fit(size, names):
    """The heights and paint layers at a size the landscape takes: whole components of QUADS
    quads, or of 63 for a preview whose size isn't. A preview's 1025 samples take 16 of 63 quads,
    1009: the rest is open sea at the edges, cropped evenly off each side into {SRC}/fit."""
    quads = QUADS if (size - 1) % QUADS == 0 else 63
    comps = (size - 1) // quads
    fitted = comps * quads + 1
    files = {n: f"{SRC}/{n}.r8" for n in names} | {"height": f"{SRC}/height.r16"}
    if fitted == size:
        return quads, comps, size, files
    os.makedirs(f"{SRC}/fit", exist_ok=True)
    cut = (size - fitted) // 2
    for key, path in list(files.items()):
        width = 2 if path.endswith(".r16") else 1
        with open(path, "rb") as f:
            data = f.read()
        rows = (
            data[(r * size + cut) * width : (r * size + cut + fitted) * width]
            for r in range(cut, cut + fitted)
        )
        files[key] = f"{SRC}/fit/{os.path.basename(path)}"
        with open(files[key], "wb") as f:
            f.write(b"".join(rows))
    log(f"cropped {size} samples a side to {fitted} for {comps} components of {quads} quads")
    return quads, comps, fitted, files


def materials():
    """The ground and road materials, the ground's grass types with them."""
    grass = {layer: grass_type(layer) for layer in GRASS}
    return Ground().build(grass), road_material()


def saved_material(path):
    material = unreal.load_asset(path)
    if material is None:
        raise RuntimeError(f"no {path}: run OC_STAGE=materials first")
    return material


def main():
    if STAGE not in ("materials", "level", "landmarks", "all"):
        raise RuntimeError(f"OC_STAGE={STAGE}: expected materials, level, landmarks or all")
    if STAGE == "landmarks":
        stand_landmarks(unreal.EditorLoadingAndSavingUtils.load_map(LEVEL))
        return
    if STAGE == "level":
        ground = saved_material(f"{FOLDER}/M_IslandGround")
        road = saved_material(f"{FOLDER}/Road/M_IslandRoad")
    else:
        ground, road = materials()
        if STAGE == "materials":
            return
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    size, spacing, z_scale = info["size"], info["spacing"], info["z_scale"]
    names = info["layers"]
    quads, comps, size, files = fit(size, names)
    layers = [
        unreal.WorldBuilderLibrary.make_layer_info(f"{FOLDER}/Layers/LI_{n}", n) for n in names
    ]
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
    half = (size - 1) * spacing / 2
    land = unreal.WorldBuilderLibrary.make_landscape(
        world,
        files["height"],
        comps,
        comps,
        quads,
        unreal.Vector(-half, -half, 0),
        unreal.Vector(spacing, spacing, z_scale),
        ground,
        layers,
        [files[n] for n in names],
        GRID,
    )
    if not land:
        raise RuntimeError("no landscape")
    log(f"landscape: {comps}x{comps} components of {quads} quads at {spacing:.0f} cm")
    roads(road)
    sea(half * 2)
    tracks()
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
    log(f"saved {LEVEL}")
    if STAGE == "all":
        stand_landmarks(world)


def stand_landmarks(world):
    """The landmarks in the saved level, once: the always-loaded sea carries a tag once it has
    them, as its other actors may not be loaded to look for."""
    sea_actor = next(
        (a for a in actors.get_all_level_actors() if a.get_actor_label() == "Sea"), None
    )
    if sea_actor is None:
        raise RuntimeError(f"no Sea in {LEVEL}: run OC_STAGE=level first")
    tags = list(sea_actor.tags)
    if "landmarks" in tags:
        raise RuntimeError(f"{LEVEL} has its landmarks: run OC_STAGE=level to build it afresh")
    landmarks()
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    # Tagged only once they are saved, so a run stopped part way leaves the level to run again.
    sea_actor.set_editor_property("tags", [*tags, "landmarks"])
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
    log(f"saved {LEVEL} with its landmarks")


# climb_world.py loads this file for its ground material and sets OC_LIBRARY first: then only
# the definitions above run.
if not globals().get("OC_LIBRARY"):
    try:
        main()
    except Exception:
        # Logged here: quitting first leaves the editor no time to log the error itself.
        unreal.log_error(f"ISLAND failed: {traceback.format_exc()}")
        raise
    finally:
        unreal.SystemLibrary.quit_editor()
