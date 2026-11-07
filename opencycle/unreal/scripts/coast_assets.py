"""Brings the coast ride's assets into the project: Poly Haven models with proper foliage materials,
the generated terrain tiles and sea, and the materials for both.

OC_STAGE picks what runs, a comma list of: models, terrain, sea (default: all three). Run with
    bash scripts/ue_run.sh coast_assets.py coast_assets.log
"""

import glob
import importlib.util
import os

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/coast_assets.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
log, Graph, texture, instance, assets = lib.log, lib.Graph, lib.texture, lib.instance, lib.assets
MODELS = "/workspace/ue/assets_ph/models"
TERRAIN = "/workspace/ue/assets_gen/terrain"
STAGES = set((os.environ.get("OC_STAGE") or "models,terrain,sea").split(","))
ONLY = set(filter(None, (os.environ.get("OC_ONLY") or "").split(",")))


def meshes_in(folder):
    found = []
    for path in assets.list_assets(folder, recursive=True):
        asset = unreal.load_asset(path)
        if isinstance(asset, unreal.StaticMesh):
            found.append(asset)
    return found


def set_nanite(mesh, on=True, foliage=False):
    settings = mesh.get_editor_property("nanite_settings")
    settings.set_editor_property("enabled", on)
    if foliage:
        try:
            settings.set_editor_property(
                "shape_preservation", unreal.NaniteShapePreservation.PRESERVE_AREA
            )
        except Exception as exc:
            log(f"no shape preservation: {exc}")
    mesh.set_editor_property("nanite_settings", settings)


# --- foliage ---------------------------------------------------------------------------


def foliage_master():
    """Masked, two-sided foliage that takes its colour, cut-out, GL normal and ARM as parameters."""
    path = f"{lib.FOLDER}/Materials/M_PH_Foliage"
    if assets.does_asset_exist(path) and "rebuild" not in STAGES:
        return unreal.load_asset(path)
    g = Graph("M_PH_Foliage", f"{lib.FOLDER}/Materials")

    def param(name, kind, default):
        node = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900)
        node.set_editor_property("parameter_name", name)
        node.set_editor_property("texture", default)
        node.set_editor_property(
            "sampler_type",
            {
                "colour": unreal.MaterialSamplerType.SAMPLERTYPE_COLOR,
                "mask": unreal.MaterialSamplerType.SAMPLERTYPE_LINEAR_GRAYSCALE,
                "normal": unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL,
                "linear": unreal.MaterialSamplerType.SAMPLERTYPE_LINEAR_COLOR,
            }[kind],
        )
        node.set_editor_property("sampler_source", lib.SHARED)
        return node

    base = f"{MODELS}/fern_02/textures/fern_02"
    colour = param(
        "Colour",
        "colour",
        texture(
            "fern_02_diff", source=f"{base}_diff_1k.jpg", folder=f"{lib.FOLDER}/PH/fern_02/Foliage"
        ),
    )
    alpha = param(
        "Alpha",
        "mask",
        texture(
            "fern_02_alpha", "mask", f"{base}_alpha_1k.png", f"{lib.FOLDER}/PH/fern_02/Foliage"
        ),
    )
    normal = param(
        "Normal",
        "normal",
        texture(
            "fern_02_nor", "normal", f"{base}_nor_gl_1k.jpg", f"{lib.FOLDER}/PH/fern_02/Foliage"
        ),
    )
    arm = param(
        "ARM",
        "linear",
        texture("fern_02_arm", "linear", f"{base}_arm_1k.jpg", f"{lib.FOLDER}/PH/fern_02/Foliage"),
    )
    tint = g.node(
        unreal.MaterialExpressionVectorParameter,
        parameter_name="Tint",
        default_value=unreal.LinearColor(1, 1, 1, 1),
    )
    rgb = g.mul(g.mask(colour, "rgb"), g.mask(tint, "rgb"))
    # Poly Haven ships OpenGL normals; Unreal wants DirectX, so green flips.
    flip = g.node(
        unreal.MaterialExpressionConstant3Vector, constant=unreal.LinearColor(1, -1, 1, 1)
    )
    nor = g.mul(g.mask(normal, "rgb"), flip)
    rough = g.mask(arm, "g")
    ao = g.mask(arm, "r")
    translucency = g.node(
        unreal.MaterialExpressionScalarParameter, parameter_name="Translucency", default_value=0.55
    )
    sss = g.mul(g.mul(rgb, translucency), g.const((1.0, 1.05, 0.6)))
    g.output(rgb, nor, rough, specular=0.35, opacity_mask=g.mask(alpha, "r"), subsurface=sss, ao=ao)
    material = g.material
    material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_MASKED)
    material.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE)
    material.set_editor_property("two_sided", True)
    material.set_editor_property("opacity_mask_clip_value", 0.4)
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    return g.finish()


def foliage_sets(model_dir):
    """{slot name: {kind: file}} for every textured slot with an alpha map."""
    found = {}
    for alpha in glob.glob(f"{model_dir}/textures/*_alpha_*.png"):
        stem = os.path.basename(alpha).split("_alpha_")[0]
        res = os.path.basename(alpha).split("_alpha_")[1].split(".")[0]
        kinds = {"alpha": alpha}
        for kind, name in (("diff", "diff"), ("nor", "nor_gl"), ("arm", "arm"), ("arm", "rough")):
            path = f"{model_dir}/textures/{stem}_{name}_{res}.jpg"
            if os.path.exists(path) and kind not in kinds:
                kinds[kind] = path
        found[stem] = kinds
    return found


TINTS = {
    # Poly Haven's leaves are shot in soft light and read pale under a hard sun; push them greener.
    "island_tree": (0.85, 0.95, 0.75),
    "jacaranda_tree": (1.0, 1.0, 1.0),
}


def model_folder(model_id):
    return f"{lib.FOLDER}/PH/{model_id}"


def import_model(model_id, master):
    model_dir = f"{MODELS}/{model_id}"
    files = glob.glob(f"{model_dir}/*.gltf")
    if not files:
        log(f"no glTF for {model_id}")
        return
    folder = model_folder(model_id)
    lib.import_file(files[0], folder)
    sets = foliage_sets(model_dir)
    instances = {}
    for stem, kinds in sets.items():
        tex_folder = f"{folder}/Foliage"
        params = {
            "Colour": texture(f"{stem}_diff", "colour", kinds["diff"], tex_folder),
            "Alpha": texture(f"{stem}_alpha", "mask", kinds["alpha"], tex_folder),
            "Normal": texture(f"{stem}_nor", "normal", kinds["nor"], tex_folder),
        }
        if "arm" in kinds:
            params["ARM"] = texture(f"{stem}_arm", "linear", kinds["arm"], tex_folder)
        tint = next((v for k, v in TINTS.items() if stem.startswith(k)), (1, 1, 1))
        instances[stem] = instance(
            f"MI_{stem}", master, tex_folder, params, vectors={"Tint": (*tint, 1)}
        )
    foliage = bool(instances)
    for mesh in meshes_in(folder):
        slots = mesh.get_editor_property("static_materials")
        changed = []
        for _index, slot in enumerate(slots):
            names = [str(slot.get_editor_property("material_slot_name"))]
            if slot.material_interface:
                names.append(slot.material_interface.get_name())
            stem = next(
                (
                    s
                    for s in instances
                    if any(n == s or n.startswith(s) or n.endswith(s) for n in names)
                ),
                None,
            )
            if stem:
                slot.set_editor_property("material_interface", instances[stem])
                changed.append(stem)
        if changed:
            mesh.set_editor_property("static_materials", slots)
        set_nanite(mesh, True, foliage)
        box = mesh.get_bounding_box()
        log(
            f"model {mesh.get_path_name()} {box.min.z:.0f}..{box.max.z:.0f} "
            f"w {box.max.x - box.min.x:.0f} foliage {changed}"
        )
    assets.save_directory(folder, only_if_is_dirty=False, recursive=True)


# --- ground and sea ----------------------------------------------------------------------


def green(g, colour, tint, gain):
    """The texture's brightness times a colour: its detail without its own hue."""
    grey = g.node(unreal.MaterialExpressionDesaturation)
    g.link(colour, grey, "")
    return g.mul(g.mul(grey, g.const(tint)), gain)


def retexture_terrain():
    """Rebuilds M_Terrain and puts it back on the imported tiles, without reimporting them."""
    material = terrain_material()
    tiles = 0
    for folder in assets.list_assets(f"{lib.FOLDER}/Terrain", recursive=False, include_folder=True):
        name = folder.rstrip("/").split("/")[-1]
        if not name.startswith("T_") or name == "T_sea":
            continue
        for mesh in meshes_in(folder.rstrip("/")):
            mesh.set_material(0, material)
            assets.save_loaded_asset(mesh, False)
            tiles += 1
    log(f"terrain material on {tiles} tiles")


def terrain_material():
    g = Graph("M_Terrain", f"{lib.FOLDER}/Materials")
    vc = g.node(unreal.MaterialExpressionVertexColor)

    def channel(pin):
        node = g.node(unreal.MaterialExpressionMultiply, const_b=1.0)
        g.link(vc, node, "A", pin)
        return node

    rock_w, sand_w, mud_w, forest_w = channel("R"), channel("G"), channel("B"), channel("A")
    grass = g.texture_set("leafy_grass", 260)
    meadow = g.texture_set("sparse_grass", 420, (0.37, 0.11))
    forest = g.texture_set("forest_ground_04", 320, (0.21, 0.53))
    mud = g.texture_set("brown_mud", 300, (0.7, 0.2))
    sand = g.texture_set("coast_sand_01", 350, (0.1, 0.9))
    rock = g.texture_set("rock_face", 900, (0.5, 0.5))
    # Big soft patches of meadow break the grass's repeat.
    patches = g.saturate(g.world_noise(0.0007, -0.6, 1.6, 4))
    # Poly Haven's grasses are mostly dry litter. Take only brightness from them and give the
    # grass a summer green, darker and yellower in the meadow patches (as the Godot ground does).
    grass["diff"] = green(g, grass["diff"], (0.15, 0.20, 0.07), 2.0)
    meadow["diff"] = green(g, meadow["diff"], (0.23, 0.24, 0.10), 1.9)
    # Hill and jungle floors: half litter, half moss, so the wooded hills read green, not as dunes.
    # Foliage cards aren't in Lumen's scene, so nothing shades the floor from the sky: keep it dark.
    # The set's own colour is a cold grey that the sky's blue fill turns to ash, so both halves take
    # only its brightness: warm brown earth and olive moss.
    forest["diff"] = g.mul(
        g.lerp(
            green(g, forest["diff"], (0.13, 0.09, 0.05), 2.0),
            green(g, forest["diff"], (0.07, 0.09, 0.03), 2.0),
            g.saturate(g.world_noise(0.0012, 0.35, 1.05, 3)),
        ),
        0.7,
    )
    # Drifts of brown fallen leaves over the moss, so a jungle floor isn't one even sheet.
    litter = g.texture_set("forest_leaves_04", 280, (0.63, 0.17))
    # Darkened and pulled toward olive: bright, these drifts read as sand dunes on the open hills.
    litter["diff"] = g.mul(
        g.lerp(litter["diff"], green(g, litter["diff"], (0.16, 0.12, 0.06), 2.0), g.const(0.5)), 0.5
    )
    forest = g.blend_sets(
        forest,
        litter,
        g.mul(g.saturate(g.mul(g.add(g.world_noise(0.0019, 0.0, 1.0, 4), -0.4), 3.0)), 0.85),
    )
    # Rock face is red-brown, which reads as desert: keep its detail, make it grey-green.
    rock["diff"] = g.lerp(
        rock["diff"], green(g, rock["diff"], (0.30, 0.31, 0.27), 1.5), g.const(0.85)
    )
    # Nobody mows a verge to a lawn: drifts of yellowed, dry grass, darker clover, and bare earth
    # showing through where the grass has thinned, each at its own scale so they never line up.
    dry = g.saturate(g.mul(g.add(g.world_noise(0.0021, 0.0, 1.0, 4), -0.55), 3.0))
    clover = g.saturate(g.mul(g.add(g.world_noise(0.006, 0.0, 1.0, 3), -0.6), 3.5))
    bare = g.saturate(g.mul(g.add(g.world_noise(0.011, 0.0, 1.0, 3), -0.74), 4.0))
    straw = green(g, grass["diff"], (0.34, 0.29, 0.13), 2.0)
    grass["diff"] = g.lerp(grass["diff"], straw, g.mul(dry, 0.6))
    grass["diff"] = g.lerp(
        grass["diff"], green(g, grass["diff"], (0.07, 0.12, 0.035), 2.0), g.mul(clover, 0.7)
    )
    grass = g.blend_sets(grass, forest, g.mul(bare, 0.55))
    ground = g.blend_sets(grass, meadow, patches)
    ground = g.blend_sets(ground, forest, forest_w)
    ground = g.blend_sets(ground, mud, mud_w)
    ground = g.blend_sets(ground, sand, sand_w)
    # Japanese hills are wooded to the top. Far off, a forested face stays forest however steep: on
    # the far tiles' 20 m triangles, rock there shows as pale sawtooth wedges up the mountain.
    distance = g.node(unreal.MaterialExpressionPixelDepth)
    wooded_far = g.mul(forest_w, g.saturate(g.mul(g.add(distance, -30000.0), 1.0 / 40000.0)))
    bare_rock = g.add(g.mul(wooded_far, -0.9), 1.0)
    ground = g.blend_sets(ground, rock, g.mul(rock_w, bare_rock))
    # And from afar it is the canopy's green, not the floor's brown, that a wooded hill shows.
    ground["diff"] = g.lerp(
        ground["diff"],
        green(g, ground["diff"], (0.05, 0.075, 0.03), 2.0),
        g.mul(wooded_far, 0.75),
    )
    # Steep faces: a top-down mapping smears any texture into vertical stripes there, so cliffs take
    # rock projected from the side (biplanar, X and Y), with a flat normal since the set's normals
    # are for the top projection.
    world = g.node(unreal.MaterialExpressionWorldPosition)
    normal_ws = g.node(unreal.MaterialExpressionVertexNormalWS)
    along_x = g.texture_set("rock_face", 700, uv=g.mul(g.mask(world, "gb"), 1.0 / 700))
    along_y = g.texture_set("rock_face", 700, uv=g.mul(g.mask(world, "rb"), 1.0 / 700))
    nx = g.link(g.mask(normal_ws, "r"), g.node(unreal.MaterialExpressionAbs))
    ny = g.link(g.mask(normal_ws, "g"), g.node(unreal.MaterialExpressionAbs))
    facing_x = g.saturate(g.add(g.mul(g.add(nx, g.mul(ny, -1.0)), 4.0), 0.5))
    cliff = g.blend_sets(along_y, along_x, facing_x)
    cliff["diff"] = g.mul(
        g.lerp(cliff["diff"], green(g, cliff["diff"], (0.30, 0.31, 0.27), 1.5), g.const(0.85)),
        g.world_noise(0.002, 0.75, 1.1, 3),
    )
    cliff["nor"] = g.const((0.0, 0.0, 1.0))
    steep = g.saturate(g.mul(g.add(g.mask(normal_ws, "b"), -0.8), -4.0))
    ground = g.blend_sets(ground, cliff, g.mul(steep, bare_rock))
    variation = g.world_noise(0.00025, 0.82, 1.12, 4)
    colour = g.mul(ground["diff"], variation)
    # Wet mud in the paddies shines.
    rough = g.lerp(ground["rough"], g.const(0.18), g.mul(mud_w, 0.8))
    # A forest floor is matt: any sheen there mirrors the bright sky and greys it to blue concrete.
    rough = g.lerp(rough, g.const(0.97), forest_w)
    # The shore: a band of dark wet ground above the sea, and broken surf right at the waterline
    # that
    # drifts with time, so the sea doesn't just stop against dry land.
    z = g.mask(world, "b")
    above = g.add(z, 800.0)  # coast_layout.SEA_LEVEL is -8 m
    wet = g.saturate(g.add(g.mul(above, -1.0 / 120.0), 1.0))
    colour = g.lerp(colour, g.mul(colour, 0.45), wet)
    rough = g.lerp(rough, g.const(0.12), wet)
    drift = g.mul(g.node(unreal.MaterialExpressionTime), g.const((35.0, 22.0, 0.0)))
    surf = g.node(
        unreal.MaterialExpressionNoise,
        scale=0.006,
        quality=2,
        levels=3,
        output_min=0.0,
        output_max=1.0,
    )
    g.link(g.add(world, drift), surf, "World Position")
    band = g.saturate(
        g.add(
            g.mul(g.link(g.add(above, -12.0), g.node(unreal.MaterialExpressionAbs)), -1.0 / 22.0),
            1.0,
        )
    )
    # Noise overshoots its output range, so the band gates the foam outright rather than biasing it.
    foam = g.mul(band, g.saturate(g.mul(g.add(surf, -0.45), 4.0)))
    colour = g.lerp(colour, g.const((0.82, 0.84, 0.83)), foam)
    rough = g.lerp(rough, g.const(0.7), foam)
    g.output(
        colour,
        ground["nor"],
        g.saturate(rough),
        specular=g.lerp(g.const(0.4), g.const(0.15), forest_w),
    )
    g.material.set_editor_property("used_with_nanite", True)
    return g.finish()


def sea_material():
    g = Graph("M_Sea", f"{lib.FOLDER}/Materials")
    waves = texture("sea_waves_nor_dx", "normal", "/workspace/ue/textures/sea_waves_nor_dx.png")
    time = g.node(unreal.MaterialExpressionTime)

    def layer(tile, speed, direction):
        uv = g.world_uv(tile)
        drift = g.node(
            unreal.MaterialExpressionConstant2Vector, r=direction[0] * speed, g=direction[1] * speed
        )
        moved = g.add(uv, g.mul(drift, time))
        return g.mask(g.sample(waves, moved, "normal"), "rgb")

    a = layer(2400, 0.012, (0.8, 0.6))
    b = layer(900, 0.03, (-0.5, 0.85))
    c = layer(15000, 0.002, (1.0, 0.1))
    normal = g.add(g.add(a, b), c)
    # Flatten toward the horizon so distant water doesn't fizz.
    distance = g.node(unreal.MaterialExpressionPixelDepth)
    far = g.saturate(g.mul(distance, 1.0 / 250000.0))
    flat = g.const((0.0, 0.0, 1.0))
    normal = g.lerp(normal, flat, g.mul(far, 0.85))
    normal = g.link(normal, g.node(unreal.MaterialExpressionNormalize))
    colour = g.const((0.015, 0.05, 0.065))
    g.output(colour, normal, 0.06, specular=0.5)
    material = g.material
    try:
        water = g.node(unreal.MaterialExpressionSingleLayerWaterMaterialOutput)
        for pin, value in (
            ("ScatteringCoefficients", (0.25, 0.12, 0.06)),
            ("AbsorptionCoefficients", (0.35, 0.09, 0.05)),
        ):
            g.link(g.const(value), water, pin)
        g.link(g.const(0.3), water, "PhaseG")
        material.set_editor_property(
            "shading_model", unreal.MaterialShadingModel.MSM_SINGLE_LAYER_WATER
        )
        log("sea: single layer water")
    except Exception as exc:
        # Default lit: a half-built water output fails to compile and the sea falls back to the
        # checker.
        material.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_DEFAULT_LIT)
        log(f"single layer water unavailable, default lit: {exc}")
    return g.finish()


def import_terrain():
    material = terrain_material()
    files = sorted(glob.glob(f"{TERRAIN}/T_near_*.gltf") + glob.glob(f"{TERRAIN}/T_far_*.gltf"))
    log(f"terrain: {len(files)} tiles")
    for i, path in enumerate(files):
        name = os.path.basename(path)[:-5]
        folder = f"{lib.FOLDER}/Terrain/{name}"
        lib.import_file(path, folder)
        for mesh in meshes_in(folder):
            mesh.set_material(0, material)
            set_nanite(mesh, True)
        assets.save_directory(folder, only_if_is_dirty=False, recursive=True)
        if i % 10 == 0:
            log(f"terrain {i}/{len(files)} {name}")


def import_sea():
    material = sea_material()
    folder = f"{lib.FOLDER}/Terrain/T_sea"
    lib.import_file(f"{TERRAIN}/T_sea.gltf", folder)
    for mesh in meshes_in(folder):
        mesh.set_material(0, material)
        set_nanite(mesh, False)
    assets.save_directory(folder, only_if_is_dirty=False, recursive=True)
    log("sea imported")


if "models" in STAGES:
    master = foliage_master()
    for model_id in sorted(os.listdir(MODELS)):
        if ONLY and model_id not in ONLY:
            continue
        try:
            import_model(model_id, master)
        except Exception as exc:
            unreal.log_error(f"COAST model {model_id} failed: {exc}")
if "sea" in STAGES:
    import_sea()
if "terrain" in STAGES:
    import_terrain()
if "terrain_material" in STAGES:
    retexture_terrain()
log("done")
