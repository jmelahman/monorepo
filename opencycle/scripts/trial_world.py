"""Builds /Game/WorldTest/L_WorldTest from make_trial_world.py's output: a World Partition level
with a streamed landscape painted grass and dirt, the engine's grass growing from the grass layer,
and the ride's track along the road. A test of the pipeline the open world will use.

Run: OC_EDITOR=1 bash scripts/ue_run.sh trial_world.py trial_world.log (after building the
OpenCycleEditor module). It needs the whole editor: the commandlet has no renderer, so it can't
compile the landscape's material permutations or bake its grass, and the game then draws the ground
with the default checker.
"""

import json

import unreal

SRC = "/workspace/ue/world_gen/test"
FOLDER = "/Game/WorldTest"
LEVEL = f"{FOLDER}/L_WorldTest"
TEX = "/Game/Coast/Textures"
edit = unreal.MaterialEditingLibrary
assets = unreal.EditorAssetLibrary
tools = unreal.AssetToolsHelpers.get_asset_tools()
actors = unreal.get_editor_subsystem(unreal.EditorActorSubsystem)


def log(message):
    unreal.log(f"WORLD {message}")


def fresh(name, folder, cls, factory):
    path = f"{folder}/{name}"
    if assets.does_asset_exist(path):
        assets.delete_asset(path)
    return tools.create_asset(name, folder, cls, factory)


def node(material, cls, x, y):
    return edit.create_material_expression(material, cls, x, y)


def wire(a, out, b, into):
    if not edit.connect_material_expressions(a, out, b, into):
        log(f"could not wire {a.get_name()}.{out} -> {b.get_name()}.{into}")


def turf_material():
    """Two-sided foliage, coloured by the blades' vertex colour."""
    m = fresh("M_Turf", f"{FOLDER}/Grass", unreal.Material, unreal.MaterialFactoryNew())
    m.set_editor_property("two_sided", True)
    m.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE)
    m.set_editor_property("used_with_instanced_static_meshes", True)
    m.set_editor_property("used_with_nanite", True)
    vc = node(m, unreal.MaterialExpressionVertexColor, -600, 0)
    edit.connect_material_property(vc, "", unreal.MaterialProperty.MP_BASE_COLOR)
    sss = node(m, unreal.MaterialExpressionMultiply, -300, 200)
    wire(vc, "", sss, "A")
    sss.set_editor_property("const_b", 0.6)
    edit.connect_material_property(sss, "", unreal.MaterialProperty.MP_SUBSURFACE_COLOR)
    rough = node(m, unreal.MaterialExpressionConstant, -300, 100)
    rough.set_editor_property("r", 0.65)
    edit.connect_material_property(rough, "", unreal.MaterialProperty.MP_ROUGHNESS)
    edit.recompile_material(m)
    return m


def turf_meshes(material):
    meshes = []
    for k in range(4):
        task = unreal.AssetImportTask()
        task.filename = f"{SRC}/turf_{k}.gltf"
        task.destination_path = f"{FOLDER}/Grass"
        task.destination_name = f"SM_Turf_{k}"
        task.replace_existing = True
        task.automated = True
        task.save = False
        tools.import_asset_tasks([task])
        for path in task.get_editor_property("imported_object_paths") or []:
            mesh = unreal.load_asset(path)
            if isinstance(mesh, unreal.StaticMesh):
                mesh.set_material(0, material)
                nanite = mesh.get_editor_property("nanite_settings")
                nanite.set_editor_property("enabled", True)
                mesh.set_editor_property("nanite_settings", nanite)
                meshes.append(mesh)
    log(f"turf meshes: {[m.get_name() for m in meshes]}")
    return meshes


def grass_type(meshes):
    gt = fresh(
        "GT_Turf", f"{FOLDER}/Grass", unreal.LandscapeGrassType, unreal.LandscapeGrassTypeFactory()
    )
    varieties = []
    for mesh in meshes:
        v = unreal.GrassVariety()
        v.set_editor_property("grass_mesh", mesh)
        # Instances per 10 m^2, over the four meshes: about three 60 cm patches a square metre.
        v.set_editor_property("grass_density", unreal.PerPlatformFloat(default=8.0))
        v.set_editor_property("start_cull_distance", unreal.PerPlatformInt(default=2500))
        v.set_editor_property("end_cull_distance", unreal.PerPlatformInt(default=4000))
        v.set_editor_property("scaling", unreal.GrassScaling.UNIFORM)
        v.set_editor_property("scale_x", unreal.FloatInterval(0.8, 1.4))
        v.set_editor_property("random_rotation", True)
        v.set_editor_property("align_to_surface", True)
        v.set_editor_property("cast_dynamic_shadow", True)
        varieties.append(v)
    gt.set_editor_property("grass_varieties", varieties)
    return gt


def layer(m, name, texture, tint, x, y):
    """A layer's colour and normal: its texture set tiled 2 m in world space, tinted."""
    coords = node(m, unreal.MaterialExpressionLandscapeLayerCoords, x - 700, y)
    coords.set_editor_property("mapping_scale", 2.0)
    out = []
    for k, (suffix, kind) in enumerate(
        (
            ("diff", unreal.MaterialSamplerType.SAMPLERTYPE_COLOR),
            ("nor_dx", unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL),
        )
    ):
        s = node(m, unreal.MaterialExpressionTextureSample, x - 450, y + k * 250)
        s.set_editor_property("texture", unreal.load_asset(f"{TEX}/{texture}_{suffix}_2k"))
        s.set_editor_property("sampler_type", kind)
        wire(coords, "", s, "UVs")
        out.append(s)
    tinted = node(m, unreal.MaterialExpressionMultiply, x - 200, y)
    wire(out[0], "RGB", tinted, "A")
    c = node(m, unreal.MaterialExpressionConstant3Vector, x - 450, y - 150)
    c.set_editor_property("constant", unreal.LinearColor(*tint, 1))
    wire(c, "", tinted, "B")
    return tinted, out[1]


def landscape_material(gt):
    m = fresh("M_WorldGround", FOLDER, unreal.Material, unreal.MaterialFactoryNew())
    names = ("Grass", "Dirt")
    blends = []
    for k, prop in enumerate(
        (unreal.MaterialProperty.MP_BASE_COLOR, unreal.MaterialProperty.MP_NORMAL)
    ):
        b = node(m, unreal.MaterialExpressionLandscapeLayerBlend, 0, k * 400)
        rows = []
        for n in names:
            row = unreal.LayerBlendInput()
            row.set_editor_property("layer_name", n)
            row.set_editor_property("blend_type", unreal.LandscapeLayerBlendType.LB_WEIGHT_BLEND)
            rows.append(row)
        b.set_editor_property("layers", rows)
        edit.connect_material_property(b, "", prop)
        blends.append(b)
    grass = layer(m, "Grass", "leafy_grass", (0.55, 0.75, 0.40), -300, 0)
    dirt = layer(m, "Dirt", "gravel_ground_01", (1.0, 0.95, 0.85), -300, 700)
    for (colour, normal), n in ((grass, "Grass"), (dirt, "Dirt")):
        wire(colour, "", blends[0], f"Layer {n}")
        wire(normal, "RGB", blends[1], f"Layer {n}")
    rough = node(m, unreal.MaterialExpressionConstant, -300, 1300)
    rough.set_editor_property("r", 0.9)
    edit.connect_material_property(rough, "", unreal.MaterialProperty.MP_ROUGHNESS)
    sample = node(m, unreal.MaterialExpressionLandscapeLayerSample, 0, 900)
    sample.set_editor_property("parameter_name", "Grass")
    out = node(m, unreal.MaterialExpressionLandscapeGrassOutput, 300, 900)
    gi = unreal.GrassInput()
    gi.set_editor_property("name", "Turf")
    gi.set_editor_property("grass_type", gt)
    out.set_editor_property("grass_types", [gi])
    wire(sample, "", out, "Turf")
    edit.recompile_material(m)
    return m


def track(centre):
    cls = unreal.load_class(None, "/Script/OpenCycleUE.RideTrack")
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", cls)
    t = fresh("DA_RideTrack", f"{FOLDER}/Ride", cls, factory)
    t.set_editor_property("centre", [unreal.Vector(x, y, z - 8) for x, y, z in centre])
    t.set_editor_property("step", 100.0)
    t.set_editor_property("keep_left", 45.0)
    t.set_editor_property("sun_azimuth", -62.0)
    coast = unreal.load_asset("/Game/Coast/Ride/DA_RideTrack")
    for prop in ("rain_shell", "collection"):
        t.set_editor_property(prop, coast.get_editor_property(prop))
    row = unreal.RideWeather()
    row.set_editor_property("metres", 0.0)
    t.set_editor_property("weather", [row])
    stretch = unreal.RideStretch()
    stretch.set_editor_property("name", "Meadows")
    t.set_editor_property("stretches", [stretch])
    assets.save_loaded_asset(t, False)


def main():
    with open(f"{SRC}/track.json") as f:
        meta = json.load(f)
    size, spacing, z_scale = meta["size"], meta["spacing"], meta["z_scale"]
    gt = grass_type(turf_meshes(turf_material()))
    ground = landscape_material(gt)
    layers = [
        unreal.WorldBuilderLibrary.make_layer_info(f"{FOLDER}/Layers/LI_{n}", n)
        for n in ("Grass", "Dirt")
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
    comps = (size - 1) // 63
    land = unreal.WorldBuilderLibrary.make_landscape(
        world,
        f"{SRC}/height.r16",
        comps,
        comps,
        63,
        unreal.Vector(-half, -half, 0),
        unreal.Vector(spacing, spacing, z_scale),
        ground,
        layers,
        [f"{SRC}/grass.r8", f"{SRC}/dirt.r8"],
        4,
    )
    if not land:
        raise RuntimeError("no landscape")
    track(meta["centre"])
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
    log(f"saved {LEVEL}")


try:
    main()
finally:
    unreal.SystemLibrary.quit_editor()
