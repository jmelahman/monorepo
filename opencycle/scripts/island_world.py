"""Builds /Game/Island/L_Island from make_island.py's full output: a World Partition level with
the island's landscape (8 km at a metre a sample), painted from Megascans surfaces, growing its
grass per region, with the roads, the sea, and the ring as the ride's track.

Run after megascans.py (both stages) and OC_RES=8129 python3 -I scripts/make_island.py:
    OC_EDITOR=1 bash scripts/ue_run.sh island_world.py island_world.log
It needs the whole editor, as trial_world.py does. OC_SRC picks another make_island.py output, e.g.
/workspace/ue/world_gen/island_1025 for a quick, coarse build.
"""

import importlib.util
import json
import os
import re

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
PREP = "/workspace/ue/fab_prep"
FOLDER = "/Game/Island"
LEVEL = f"{FOLDER}/L_Island"
MS = "/Game/Megascans"
QUADS = 127  # per landscape component
GRID = 4  # components a side per streaming proxy: about 500 m

# Each paint layer's surfaces: (biome, slug, tint). A second surface is mixed in by `MIXES`.
SURFACES = {
    "Grass": ("shared", "lawn_grass", (0.8, 0.88, 0.62)),
    "Forest": ("forest", "forest_floor", (1.0, 1.0, 1.0)),
    "Rock": ("shared", "stony_soil", (0.72, 0.72, 0.74)),
    "Snow": ("coast", "rippled_sand", (1.75, 1.8, 1.9)),
    "Desert": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
    "Sand": ("coast", "trampled_beach_sand", (1.0, 1.0, 1.0)),
    "Dirt": ("shared", "dirt_road", (1.0, 1.0, 1.0)),
}
# A second surface per layer and what mixes it in: "patches" (noise, metres across) or "wet"
# (below the given height in cm, the tideline).
MIXES = {
    "Forest": ("forest", "mossy_forest_floor", "patches", 18.0),
    "Desert": ("desert", "dry_cracked_mud", "patches", 60.0),
    "Sand": ("coast", "thai_wet_sand", "wet", 120.0),
    "Dirt": ("shared", "gravel", "patches", 8.0),
}
FAR = 6.0  # far off, each surface is also sampled this many times larger, against tiling
ROUGHNESS = {"Snow": 0.55, "Sand": 0.9}

# Grass per layer: (biome, slug, instances per 10 m^2 per variation, cull start, cull end in cm).
GRASS = {
    "Grass": (
        ("shared", "wild_grass", 10.0, 3500, 5000),
        ("shared", "grass_clumps", 6.0, 3500, 5000),
        ("shared", "wheat_grass", 2.5, 3500, 5000),
        ("shared", "tundra_grass", 1.5, 3500, 5000),
        ("shared", "clovers", 3.0, 2000, 3000),
        ("shared", "leafy_dandelions", 1.0, 2000, 3000),
    ),
    "Forest": (
        ("shared", "fern", 1.2, 4000, 6000),
        ("shared", "lady_fern", 0.8, 4000, 6000),
        ("shared", "common_fern", 0.8, 4000, 6000),
        ("shared", "leather_fern", 0.4, 4000, 6000),
        ("shared", "grass_clumps", 2.0, 3000, 4500),
    ),
    "Desert": (
        ("desert", "dry_grass", 1.2, 4000, 6000),
        ("desert", "dead_shrubs", 0.15, 6000, 9000),
        ("desert", "desert_american_plant_cactus_medium_11", 0.01, 12000, 16000),
        ("desert", "desert_american_plant_branch_cactus_large_03", 0.005, 15000, 20000),
    ),
    "Sand": (("coast", "marram_grass", 1.5, 4000, 6000),),
    "Rock": (
        ("mountain", "alpine_bearberry", 0.4, 3000, 4500),
        ("mountain", "dwarf_birch", 0.08, 5000, 7000),
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

    def __init__(self):
        self.g = Graph("M_IslandGround", FOLDER)
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
        masks = g.sample(m, near_uv, "masks")
        colour = g.mul(g.lerp(near_d, far_d, g.mul(self.far, 0.7)), g.const(tint))
        normal = g.lerp(near_n, far_n, self.far)
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
    for biome, slug, density, cull_start, cull_end in GRASS[layer]:
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
            v.set_editor_property("scale_x", unreal.FloatInterval(0.85, 1.3))
            v.set_editor_property("random_rotation", True)
            v.set_editor_property("align_to_surface", density > 0.5)  # small plants only
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


def roads():
    material = road_material()
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


def track(ring):
    """The ring as the ride's track, from the harbour clockwise."""
    cls = unreal.load_class(None, "/Script/OpenCycleUE.RideTrack")
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", cls)
    t = lib.fresh("DA_RideTrack", f"{FOLDER}/Ride", cls, factory)
    # The rider's line 6 cm under the tarmac, as in the trial world.
    t.set_editor_property(
        "centre", [unreal.Vector(x * 100, y * 100, z * 100 - 6) for x, y, z in ring]
    )
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
    stretch.set_editor_property("name", "Island loop")
    t.set_editor_property("stretches", [stretch])
    assets.save_loaded_asset(t, False)
    log(f"track: {len(ring)} m")


def main():
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    with open(f"{SRC}/roads.json") as f:
        ring = json.load(f)["ring"]["centre"]
    size, spacing, z_scale = info["size"], info["spacing"], info["z_scale"]
    quads = QUADS if (size - 1) % QUADS == 0 else 63
    comps = (size - 1) // quads
    grass = {layer: grass_type(layer) for layer in GRASS}
    ground = Ground().build(grass)
    names = info["layers"]
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
        f"{SRC}/height.r16",
        comps,
        comps,
        quads,
        unreal.Vector(-half, -half, 0),
        unreal.Vector(spacing, spacing, z_scale),
        ground,
        layers,
        [f"{SRC}/{n}.r8" for n in names],
        GRID,
    )
    if not land:
        raise RuntimeError("no landscape")
    log(f"landscape: {comps}x{comps} components of {quads} quads at {spacing:.0f} cm")
    roads()
    sea(half * 2)
    track(ring)
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
    log(f"saved {LEVEL}")


try:
    main()
finally:
    unreal.SystemLibrary.quit_editor()
