"""Builds /Game/Spike/L_Forest: a sky with a physical sun, flat ground, a gently bending road from
the
origin, and conifers either side of it, scattered with a fixed seed. Run as a commandlet.

    OC_TREE    the tree asset (default the vegetation plugin's sample conifer)
    OC_LENGTH  metres of road (default 300)
    OC_TREES   trees per 100 m of road, both sides together (default 60)
    OC_CURVE   the road's bend amplitude in cm (default 1200, 0 for the old straight road)
    OC_WAVELENGTH  the bend's wavelength in cm (default 45000)

The road and both shoulders are laid out from forest_layout.RoadLayout as a chain of plane
segments; every tree and undergrowth plant is moved onto the same centreline with
RoadLayout.point, so the verge follows the bend instead of the road wandering away from it.

The PVE trees store their needles as Nanite assembly parts. We duplicate the parent into
/Game/Spike, replace its foliage slots with a generated non-virtual masked material, and rebuild
the assembly with ShapePreservation=PreserveArea so the parts keep real leaf-card triangles.
DefaultEngine.ini sets r.Nanite.ProjectEnabled=False: the renderer then draws the assembly's
generated fallback LOD as ordinary triangles. That fallback contains every assembly part, which
is why it keeps the full canopy instead of the runtime assembly resource dropping the cards.
"""

import importlib.util
import math
import os
import random

import unreal


def sibling(name):
    """Imports one of our own helper modules by file path.

    A plain import would need the scripts folder on sys.path, where its inspect.py
    shadows the stdlib inspect module that Unreal's own tooling may still want untouched.
    """
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/make_forest.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


TREE = (
    os.environ.get("OC_TREE")
    or "/ProceduralVegetationEditor/SampleAssets/StarterContent/ConiferTree_01/PVE_Conifer_01"
)
LENGTH = float(os.environ.get("OC_LENGTH") or "300") * 100.0
DENSITY = float(os.environ.get("OC_TREES") or "60")
DEPTH = float(os.environ.get("OC_DEPTH") or "1000") * 100.0
UNDERGROWTH = int(float(os.environ.get("OC_UNDERGROWTH") or "0"))
VERGE_UNDERGROWTH = int(float(os.environ.get("OC_VERGE_UNDERGROWTH") or "650"))
FOG_DENSITY = float(os.environ.get("OC_FOG") or "0.02")
CURVE = float(os.environ.get("OC_CURVE") or "1200")
WAVELENGTH = float(os.environ.get("OC_WAVELENGTH") or "45000")
ROAD = 600.0  # cm wide
SHOULDER = 90.0  # cm of grassy verge either side
SEGMENT = 2000.0  # cm of road per plane tile
LEVEL = "/Game/Spike/L_Forest"
FOLDER = "/Game/Spike"
LAYOUT = sibling("forest_layout").RoadLayout(amplitude=CURVE, wavelength=WAVELENGTH)
UNDERGROWTH_MESHES = (
    (
        "/ProceduralVegetationEditor/SampleAssets/StarterContent/"
        "Deciduous_Shrub_01/PVE_Deciduous_Shrub_01",
        0.45,
        1.0,
    ),
    (
        "/ProceduralVegetationEditor/SampleAssets/StarterContent/"
        "Deciduous_Shrub_01/PVE_Deciduous_Shrub_01",
        0.45,
        1.0,
    ),
    ("/ProceduralVegetationEditor/SampleAssets/StarterContent/Plant_01/PVE_Plant_01", 0.8, 1.8),
    ("/ProceduralVegetationEditor/SampleAssets/StarterContent/Plant_01/PVE_Plant_01", 0.8, 1.8),
    ("/ProceduralVegetationEditor/SampleAssets/StarterContent/Plant_01/PVE_Plant_01", 0.8, 1.8),
)


def log(message):
    unreal.log(f"FOREST {message}")


TEXTURES = "/workspace/ue/textures"


def import_texture(name, kind, extension="jpg"):
    """Imports one texture, set up as colour, a DirectX normal map or a linear mask."""
    task = unreal.AssetImportTask()
    task.filename = f"{TEXTURES}/{name}.{extension}"
    task.destination_path = f"{FOLDER}/Textures"
    task.replace_existing = True
    task.automated = True
    task.save = False
    unreal.AssetToolsHelpers.get_asset_tools().import_asset_tasks([task])
    paths = list(task.get_editor_property("imported_object_paths") or [])
    texture = unreal.load_asset(paths[0] if paths else f"{FOLDER}/Textures/{name}")
    log(f"texture {name}: {paths} -> {texture.get_name() if texture else None}")
    if texture is None:
        raise RuntimeError(f"could not import {task.filename}")
    if kind == "normal":
        texture.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_NORMALMAP
        )
        texture.set_editor_property("srgb", False)
    elif kind == "mask":
        texture.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_GRAYSCALE
        )
        texture.set_editor_property("srgb", False)
    unreal.EditorAssetLibrary.save_loaded_asset(texture, False)
    return texture


def world_uv(material, edit, tile):
    """World-position UVs, one tile every `tile` cm on any horizontal plane."""
    world = edit.create_material_expression(
        material, unreal.MaterialExpressionWorldPosition, -1100, 0
    )
    mask = edit.create_material_expression(
        material, unreal.MaterialExpressionComponentMask, -950, 0
    )
    mask.set_editor_property("r", True)
    mask.set_editor_property("g", True)
    edit.connect_material_expressions(world, "", mask, "")
    scale = edit.create_material_expression(material, unreal.MaterialExpressionMultiply, -800, 0)
    scale.set_editor_property("const_b", 1.0 / tile)
    edit.connect_material_expressions(mask, "", scale, "A")
    return world, scale


def texture_set(material, edit, source, uv, row):
    """Imports one Poly Haven colour/normal/roughness set and wires it to `uv`."""
    samples = {}
    for column, (suffix, kind, sampler) in enumerate(
        (
            ("diff", "colour", unreal.MaterialSamplerType.SAMPLERTYPE_COLOR),
            ("nor_dx", "normal", unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL),
            ("rough", "mask", unreal.MaterialSamplerType.SAMPLERTYPE_LINEAR_GRAYSCALE),
        )
    ):
        sample = edit.create_material_expression(
            material, unreal.MaterialExpressionTextureSample, -600 + column * 260, row
        )
        sample.texture = import_texture(f"{source}_{suffix}_2k", kind)
        sample.sampler_type = sampler
        edit.connect_material_expressions(uv, "", sample, "UVs")
        samples[suffix] = sample
    return samples


def textured_material(name, source, tile, roughness_scale=1.0, darken=1.0):
    """A material from a Poly Haven set, mapped in world space so one tile spans `tile` cm on any
    plane."""
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)
    material = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
        name, FOLDER, unreal.Material, unreal.MaterialFactoryNew()
    )
    edit = unreal.MaterialEditingLibrary
    _, uv = world_uv(material, edit, tile)
    samples = texture_set(material, edit, source, uv, 0)
    base = samples["diff"]
    if darken != 1.0:
        darker = edit.create_material_expression(
            material, unreal.MaterialExpressionMultiply, -100, 0
        )
        darker.set_editor_property("const_b", darken)
        edit.connect_material_expressions(base, "RGB", darker, "A")
        base = darker
    wired = [
        edit.connect_material_property(base, "", unreal.MaterialProperty.MP_BASE_COLOR),
        edit.connect_material_property(samples["nor_dx"], "RGB", unreal.MaterialProperty.MP_NORMAL),
    ]
    rough = edit.create_material_expression(material, unreal.MaterialExpressionMultiply, -150, 500)
    rough.set_editor_property("const_b", roughness_scale)
    edit.connect_material_expressions(samples["rough"], "R", rough, "A")
    wired.append(edit.connect_material_property(rough, "", unreal.MaterialProperty.MP_ROUGHNESS))
    errors = edit.recompile_material(material)
    if errors:
        raise RuntimeError(f"{name} did not compile: {list(errors)}")
    log(f"material {name}: colour, normal, roughness wired {wired}")
    unreal.EditorAssetLibrary.save_loaded_asset(material, False)
    return material


FOLIAGE_HINTS = ("foliage", "leaf", "needle", "leaves")
TRUNK_HINTS = ("bark", "trunk", "wood")


def plugin_texture(source, kind):
    """Exports a plugin texture to PNG and imports it as a plain project texture.

    The PVE leaves ship as streaming virtual textures, which a regular TextureSample node
    refuses to compile against ("Sampler requires non-VirtualTexture"). A PNG round-trip is
    the reliable way to get a non-virtual copy that keeps the alpha channel we need.
    """
    name = source.get_name()
    png = f"{TEXTURES}/{name}.png"
    if not os.path.exists(png):
        task = unreal.AssetExportTask()
        task.object = source
        task.filename = png
        task.exporter = unreal.TextureExporterPNG()
        task.automated = True
        task.replace_identical = True
        task.prompt = False
        if not unreal.Exporter.run_asset_export_task(task):
            raise RuntimeError(f"could not export {source.get_path_name()} to {png}")
    return import_texture(name, kind, "png")


def masked_foliage_material(instance):
    """Builds a two-sided masked material from a PVE foliage instance's own textures.

    The PVE master material renders each foliage card as a solid square here, which turns a
    conifer into a pile of green boxes. The instance's base-colour atlas has a real alpha
    channel (36% of the conifer sheet is fully transparent), so wiring that alpha to the
    opacity mask restores the needle silhouettes.
    """
    source = instance.get_path_name()
    name = f"M_Mask_{instance.get_name()}"
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)

    edit = unreal.MaterialEditingLibrary
    material = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
        name, FOLDER, unreal.Material, unreal.MaterialFactoryNew()
    )
    colour_texture = edit.get_material_instance_texture_parameter_value(instance, "Base Color")
    normal_texture = edit.get_material_instance_texture_parameter_value(instance, "Normal")
    if colour_texture is None:
        raise RuntimeError(f"{source} has no Base Color texture")
    colour_texture = plugin_texture(colour_texture, "colour")

    # The PVE sample twigs map a single large leaf region onto each card. Tiling the atlas a few
    # times per card keeps the same real leaf alpha but turns each giant leaf into a dense cluster
    # of small leaves, which is what makes the tree read as foliage rather than confetti.
    uv_scale = float(os.environ.get("OC_LEAF_UV_SCALE") or "4.0")
    uv = edit.create_material_expression(
        material, unreal.MaterialExpressionTextureCoordinate, -950, -160
    )
    uv.set_editor_property("coordinate_index", 0)
    uv_scale_node = edit.create_material_expression(
        material, unreal.MaterialExpressionConstant2Vector, -950, -40
    )
    uv_scale_node.set_editor_property("r", uv_scale)
    uv_scale_node.set_editor_property("g", uv_scale)
    uv_mul = edit.create_material_expression(
        material, unreal.MaterialExpressionMultiply, -800, -100
    )
    edit.connect_material_expressions(uv, "", uv_mul, "A")
    edit.connect_material_expressions(uv_scale_node, "", uv_mul, "B")

    colour = edit.create_material_expression(
        material, unreal.MaterialExpressionTextureSampleParameter2D, -600, 0
    )
    colour.set_editor_property("parameter_name", "Base Color")
    colour.set_editor_property("texture", colour_texture)
    colour.set_editor_property("sampler_type", unreal.MaterialSamplerType.SAMPLERTYPE_COLOR)
    edit.connect_material_expressions(uv_mul, "", colour, "UVs")
    # Pull the sheet's olive/brown mix towards living green without losing its leaf detail.
    tint_rgb = [float(x) for x in (os.environ.get("OC_LEAF_TINT") or "0.65,0.95,0.55").split(",")]
    tint = edit.create_material_expression(
        material, unreal.MaterialExpressionConstant3Vector, -450, -100
    )
    tint.set_editor_property(
        "constant", unreal.LinearColor(tint_rgb[0], tint_rgb[1], tint_rgb[2], 1.0)
    )
    tint_mul = edit.create_material_expression(
        material, unreal.MaterialExpressionMultiply, -300, -200
    )
    edit.connect_material_expressions(colour, "RGB", tint_mul, "A")
    edit.connect_material_expressions(tint, "", tint_mul, "B")
    wired = [edit.connect_material_property(tint_mul, "", unreal.MaterialProperty.MP_BASE_COLOR)]
    if os.environ.get("OC_LEAF_DEBUG") != "1":
        wired.append(
            edit.connect_material_property(colour, "A", unreal.MaterialProperty.MP_OPACITY_MASK)
        )
    if normal_texture:
        normal_texture = plugin_texture(normal_texture, "normal")
        normal = edit.create_material_expression(
            material, unreal.MaterialExpressionTextureSampleParameter2D, -600, 300
        )
        normal.set_editor_property("parameter_name", "Normal")
        normal.set_editor_property("texture", normal_texture)
        normal.set_editor_property("sampler_type", unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL)
        edit.connect_material_expressions(uv_mul, "", normal, "UVs")
        wired.append(
            edit.connect_material_property(normal, "RGB", unreal.MaterialProperty.MP_NORMAL)
        )
    rough = edit.create_material_expression(material, unreal.MaterialExpressionConstant, -300, 520)
    rough.set_editor_property("r", 0.8)
    wired.append(edit.connect_material_property(rough, "", unreal.MaterialProperty.MP_ROUGHNESS))

    debug = os.environ.get("OC_LEAF_DEBUG") == "1"
    material.set_editor_property(
        "blend_mode", unreal.BlendMode.BLEND_OPAQUE if debug else unreal.BlendMode.BLEND_MASKED
    )
    # Two-sided voxel sections cannot compile a Nanite voxel shading bin (NaniteCullRaster.cpp
    # rejects bVoxel for two-sided material permutations) and fall back to the triangle shader,
    # which renders voxel bricks as hard cubes. A one-sided leaf material unlocks the proper
    # voxel shading pipeline, so it is the default here; OC_LEAF_TWOSIDED=1 restores two-sided.
    material.set_editor_property("two_sided", (os.environ.get("OC_LEAF_TWOSIDED") or "1") == "1")
    material.set_editor_property("opacity_mask_clip_value", 0.3333)
    material.set_editor_property("used_with_skeletal_mesh", True)
    material.set_editor_property("used_with_instanced_static_meshes", True)
    for flag in ("used_with_nanite", "used_with_voxels"):
        try:
            material.set_editor_property(flag, True)
        except Exception as exc:
            log(f"could not set {flag}: {exc}")
    errors = edit.recompile_material(material)
    if errors:
        raise RuntimeError(f"{name} did not compile: {list(errors)}")
    log(f"foliage material {name} from {source}: wired {wired}")
    unreal.EditorAssetLibrary.save_loaded_asset(material, False)
    return material


leaf_materials = {}
foliage_meshes = {}


def leaf_replacement(slot):
    """Returns the masked replacement for a foliage slot, or None for bark/wood."""
    interface = slot.material_interface
    if interface is None:
        return None
    label = f"{slot.material_slot_name} {interface.get_name()}".lower()
    if not any(hint in label for hint in FOLIAGE_HINTS) or any(
        hint in label for hint in TRUNK_HINTS
    ):
        return None
    if interface.get_path_name() not in leaf_materials:
        leaf_materials[interface.get_path_name()] = masked_foliage_material(interface)
    return leaf_materials[interface.get_path_name()]


def foliage_mesh_copy(mesh, shape_env=None):
    """Duplicates a skeletal tree and rebuilds its Nanite data in one of the available shapes.

    Nanite assembly parts remap their materials into the parent mesh's base material list.
    USkinnedMeshComponent::SetMaterial only overrides the component's own sections, so it does
    not reach those parts. A project-owned duplicate leaves the plugin assets untouched while
    giving the assembly the masked material it needs. Rebuilding is essential: it refreshes the
    assembly part resources and its fallback LOD; merely changing the parent's settings leaves the
    cached part/voxel resources in place.

    shape: "preserve_area" (the default) keeps real leaf-card triangles and is the only shape used
    for the final look. "voxelize" bakes voxel leaf bricks that rasterize as hard cubes; it is kept
    only for A/B diagnosis. OC_TREE_HYBRID=1 can optionally switch far trees to voxels, off by
    default.
    """
    shape_env = (
        shape_env or os.environ.get("OC_TREE_SHAPE") or "preserve_area"
    ).lower() or "preserve_area"
    cache = foliage_meshes.setdefault(mesh.get_path_name(), {})
    if shape_env in cache:
        return cache[shape_env]
    shape_name = {"none": "None", "voxelize": "Voxelize", "preserve_area": "Area"}[shape_env]
    name = f"{mesh.get_name()}_{shape_name}"
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)
    copy = unreal.EditorAssetLibrary.duplicate_asset(mesh.get_path_name(), path)
    if copy is None:
        raise RuntimeError(f"could not duplicate {mesh.get_path_name()} to {path}")
    slots = list(copy.get_editor_property("materials"))
    replacements = []
    # job.sh forwards known switches as empty strings when unset; treat that as the default.
    leaf_override = (os.environ.get("OC_LEAF_OVERRIDE") or "1") == "1"
    for index, slot in enumerate(slots):
        replacement = leaf_replacement(slot) if leaf_override else None
        if replacement is None:
            continue
        slot.set_editor_property("material_interface", replacement)
        replacements.append(f"{index}:{slot.material_slot_name}")
    copy.set_editor_property("materials", slots)
    # Always force a Nanite rebuild of the duplicate: the plugin mesh may carry voxelized foliage
    # that the builder dropped when voxels were unsupported at runtime (SkeletalMeshBuilder.cpp does
    # exactly that). Now r.Nanite.AllowVoxels=1 makes NaniteVoxelsSupported() true, so a fresh build
    # with ShapePreservation=Voxelize bakes voxel foliage; PreserveArea bakes real leaf-card
    # triangles instead (the legacy foliage technique that kept needles visible).
    settings = copy.get_editor_property("nanite_settings")
    shape = {
        "none": unreal.NaniteShapePreservation.NONE,
        "voxelize": unreal.NaniteShapePreservation.VOXELIZE,
        "preserve_area": unreal.NaniteShapePreservation.PRESERVE_AREA,
    }[shape_env]
    settings.set_editor_property("shape_preservation", shape)
    if (
        shape == unreal.NaniteShapePreservation.VOXELIZE
        and (os.environ.get("OC_VOXEL_OPACITY") or "0") == "1"
    ):
        # The plugin mesh ships voxel_opacity=False, which culls low-coverage voxels; setting True
        # bakes per-voxel opacity from the material's alpha but scattered disconnected cubes made
        # step4k worse than the default, so it stays off unless OC_VOXEL_OPACITY=1.
        settings.set_editor_property("voxel_opacity", True)
    copy.set_editor_property("nanite_settings", settings)
    copy.modify()
    log(f"rebuilt {name} with shape_preservation={shape}")
    unreal.EditorAssetLibrary.save_loaded_asset(copy, False)
    log(f"foliage mesh {name}: base slots {replacements}")
    cache[shape_env] = copy
    return copy


# Trees closer than this read their leaf geometry per-card; past it the voxel representation
# carries the silhouette. Renders showed card-tree confetti at a few metres and voxel cubes
# there too, so the split keeps each representation where it looks best (step4p).
TREE_CARD_METRES = float(os.environ.get("OC_TREE_CARD_METRES") or 25)


def override_foliage(actor, mesh):
    """Replaces only the leaf slots of a static mesh component."""
    component = actor.static_mesh_component
    for index, slot in enumerate(mesh.get_editor_property("static_materials")):
        replacement = leaf_replacement(slot)
        if replacement is not None:
            component.set_material(index, replacement)


def matte(material, edit, rough, x=-200, y=900):
    """Dry ground scatters light, so roughness goes most of the way to 1 and specular to 0.25. The
    textures' own roughness is for wet scans; left as is, a low sun ahead turns the floor a sandy
    grey."""
    lerp = edit.create_material_expression(
        material, unreal.MaterialExpressionLinearInterpolate, x, y
    )
    lerp.set_editor_property("const_b", 1.0)
    lerp.set_editor_property("const_alpha", 0.6)
    edit.connect_material_expressions(rough, "", lerp, "A")
    specular = edit.create_material_expression(
        material, unreal.MaterialExpressionConstant, x, y + 150
    )
    specular.set_editor_property("r", 0.25)
    return [
        edit.connect_material_property(lerp, "", unreal.MaterialProperty.MP_ROUGHNESS),
        edit.connect_material_property(specular, "", unreal.MaterialProperty.MP_SPECULAR),
    ]


def forest_floor_material(name, sources, tiles):
    """Poly Haven sets blended by large-scale noise, then darkened and desaturated towards brown.

    Each set gets its own tile size, so their repeats do not line up. The blend is what stops
    either one reading as a repeating carpet: the noise spans tens of metres, so one set gives
    way to the other over a whole clearing rather than per tile.
    """
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)
    material = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
        name, FOLDER, unreal.Material, unreal.MaterialFactoryNew()
    )
    edit = unreal.MaterialEditingLibrary
    world, _ = world_uv(material, edit, tiles[0])
    sets = []
    for row, (source, tile) in enumerate(zip(sources, tiles, strict=False)):
        _, uv = world_uv(material, edit, tile)
        sets.append(texture_set(material, edit, source, uv, row * 850))

    # A noise over world position, broad patches hundreds of metres across. The ground set is grey
    # stony track dirt that reads as sand from the road, so the noise keeps it to a trace
    # (0.05..0.4)
    # under the needle and leaf litter a conifer wood actually has.
    noise = edit.create_material_expression(material, unreal.MaterialExpressionNoise, -950, 700)
    noise.set_editor_property("scale", 0.000033)
    noise.set_editor_property("quality", 2)
    noise.set_editor_property("levels", 2)
    noise.set_editor_property("output_min", 0.05)
    noise.set_editor_property("output_max", 0.4)
    # UE names this pin "World Position"; "Position" silently fails to connect.
    edit.connect_material_expressions(world, "", noise, "World Position")

    blended = {}
    for row, (suffix, channel) in enumerate((("diff", "RGB"), ("nor_dx", "RGB"), ("rough", "R"))):
        blend = edit.create_material_expression(
            material, unreal.MaterialExpressionLinearInterpolate, -250, row * 260
        )
        edit.connect_material_expressions(sets[0][suffix], channel, blend, "A")
        edit.connect_material_expressions(sets[1][suffix], channel, blend, "B")
        edit.connect_material_expressions(noise, "", blend, "Alpha")
        blended[suffix] = blend

    # Moss and grass in patches a few metres across, where the canopy opens up.
    if len(sets) > 2:
        moss = edit.create_material_expression(material, unreal.MaterialExpressionNoise, -950, 950)
        moss.set_editor_property("scale", 0.00025)
        moss.set_editor_property("quality", 2)
        moss.set_editor_property("levels", 3)
        moss.set_editor_property("output_min", -0.6)
        moss.set_editor_property("output_max", 0.9)
        edit.connect_material_expressions(world, "", moss, "World Position")
        clamp = edit.create_material_expression(
            material, unreal.MaterialExpressionSaturate, -700, 950
        )
        edit.connect_material_expressions(moss, "", clamp, "")
        for row, (suffix, channel) in enumerate(
            (("diff", "RGB"), ("nor_dx", "RGB"), ("rough", "R"))
        ):
            blend = edit.create_material_expression(
                material, unreal.MaterialExpressionLinearInterpolate, -120, row * 260 + 100
            )
            edit.connect_material_expressions(blended[suffix], "", blend, "A")
            edit.connect_material_expressions(sets[2][suffix], channel, blend, "B")
            edit.connect_material_expressions(clamp, "", blend, "Alpha")
            blended[suffix] = blend

    # The pod's best frame has a dark, brown floor; straight leaf litter is too orange and bright.
    desaturate = edit.create_material_expression(
        material, unreal.MaterialExpressionDesaturation, -30, 0
    )
    fraction = edit.create_material_expression(
        material, unreal.MaterialExpressionConstant, -180, 120
    )
    fraction.set_editor_property("r", 0.2)
    # The Desaturation input pin is unnamed, so it is addressed as "".
    edit.connect_material_expressions(blended["diff"], "", desaturate, "")
    edit.connect_material_expressions(fraction, "", desaturate, "Fraction")
    darken = edit.create_material_expression(material, unreal.MaterialExpressionMultiply, 130, 0)
    darken.set_editor_property("const_b", 0.65)
    edit.connect_material_expressions(desaturate, "", darken, "A")
    wired = [
        edit.connect_material_property(darken, "", unreal.MaterialProperty.MP_BASE_COLOR),
        edit.connect_material_property(blended["nor_dx"], "", unreal.MaterialProperty.MP_NORMAL),
        *matte(material, edit, blended["rough"]),
    ]
    errors = edit.recompile_material(material)
    if errors:
        raise RuntimeError(f"{name} did not compile: {list(errors)}")
    log(
        f"material {name}: floor from {' + '.join(sources)}; "
        f"colour, normal, roughness wired {wired}"
    )
    unreal.EditorAssetLibrary.save_loaded_asset(material, False)
    return material


def verge_material(name):
    """A dark gravel/soil/leaf-litter blend for the shoulder.

    The Poly Haven gravel set is a pale tan; as a uniform 55 cm strip it reads as a painted
    concrete kerb. Two world-space noise layers mix it with dark forest soil and leaf litter,
    then the result is desaturated and darkened. That keeps the compacted gravel texture while
    breaking the outer edge into irregular patches instead of a straight bright line.
    """
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)
    material = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
        name, FOLDER, unreal.Material, unreal.MaterialFactoryNew()
    )
    edit = unreal.MaterialEditingLibrary
    world, _ = world_uv(material, edit, 150)
    sets = []
    for row, (source, tile) in enumerate(
        (
            ("leafy_grass", 180),
            ("forest_ground_04", 320),
            ("forest_leaves_04", 220),
        )
    ):
        _, uv = world_uv(material, edit, tile)
        sets.append(texture_set(material, edit, source, uv, row * 850))

    def noise(row, scale, low, high):
        node = edit.create_material_expression(
            material, unreal.MaterialExpressionNoise, -950, 700 + row * 220
        )
        node.set_editor_property("scale", scale)
        node.set_editor_property("quality", 2)
        node.set_editor_property("levels", 2)
        node.set_editor_property("output_min", low)
        node.set_editor_property("output_max", high)
        edit.connect_material_expressions(world, "", node, "World Position")
        return node

    # Broad patches of soil in the gravel, then smaller drifts of leaves spilling over it.
    # Both stay below full coverage so the compacted gravel still reads as the verge's base.
    coarse = noise(0, 0.00008, 0.15, 0.65)
    fine = noise(1, 0.004, 0.1, 0.55)

    mixed = {}
    for row, (suffix, channel) in enumerate((("diff", "RGB"), ("nor_dx", "RGB"), ("rough", "R"))):
        soil = edit.create_material_expression(
            material, unreal.MaterialExpressionLinearInterpolate, -400, row * 260
        )
        edit.connect_material_expressions(sets[0][suffix], channel, soil, "A")
        edit.connect_material_expressions(sets[1][suffix], channel, soil, "B")
        edit.connect_material_expressions(coarse, "", soil, "Alpha")
        spill = edit.create_material_expression(
            material, unreal.MaterialExpressionLinearInterpolate, -150, row * 260
        )
        edit.connect_material_expressions(soil, "", spill, "A")
        edit.connect_material_expressions(sets[2][suffix], channel, spill, "B")
        edit.connect_material_expressions(fine, "", spill, "Alpha")
        mixed[suffix] = spill

    desaturate = edit.create_material_expression(
        material, unreal.MaterialExpressionDesaturation, 60, 0
    )
    fraction = edit.create_material_expression(
        material, unreal.MaterialExpressionConstant, -90, 120
    )
    fraction.set_editor_property("r", 0.25)
    edit.connect_material_expressions(mixed["diff"], "", desaturate, "")
    edit.connect_material_expressions(fraction, "", desaturate, "Fraction")
    darken = edit.create_material_expression(material, unreal.MaterialExpressionMultiply, 220, 0)
    darken.set_editor_property("const_b", 0.7)
    edit.connect_material_expressions(desaturate, "", darken, "A")
    wired = [
        edit.connect_material_property(darken, "", unreal.MaterialProperty.MP_BASE_COLOR),
        edit.connect_material_property(mixed["nor_dx"], "", unreal.MaterialProperty.MP_NORMAL),
        *matte(material, edit, mixed["rough"]),
    ]
    errors = edit.recompile_material(material)
    if errors:
        raise RuntimeError(f"{name} did not compile: {list(errors)}")
    log(f"material {name}: gravel/soil/litter verge; colour, normal, roughness wired {wired}")
    unreal.EditorAssetLibrary.save_loaded_asset(material, False)
    return material


levels = unreal.get_editor_subsystem(unreal.LevelEditorSubsystem)
# A blank map, saved over the old one at the end; new_level refuses to replace an existing map.
world = unreal.EditorLoadingAndSavingUtils.new_blank_map(False)
actors = unreal.get_editor_subsystem(unreal.EditorActorSubsystem)


def spawn(cls, where=(0, 0, 0), rotation=(0, 0, 0), label=None):
    actor = actors.spawn_actor_from_class(cls, unreal.Vector(*where), unreal.Rotator(*rotation))
    if label:
        actor.set_actor_label(label)
    return actor


# Sky: a physical sun low enough to rake through the trunks, the atmosphere it lights, and a sky
# light
# that recaptures it in real time so Lumen has the right blue fill.
# OC_SUN is "elevation,azimuth" in degrees, OC_SUN_LUX its illuminance, OC_SUN_SOFT its angular
# size in
# degrees (0.5 is the real sun; tens of degrees stand in for an overcast sky's soft shadows), and
# OC_SKY_LIGHT scales the sky light's fill.
elevation, azimuth = (float(v) for v in (os.environ.get("OC_SUN") or "35,40").split(","))
sun = spawn(unreal.DirectionalLight, (0, 0, 1000), (0, -elevation, azimuth), "Sun")
light = sun.get_component_by_class(unreal.DirectionalLightComponent)
light.set_intensity(float(os.environ.get("OC_SUN_LUX") or "75000"))
light.set_editor_property("light_source_angle", float(os.environ.get("OC_SUN_SOFT") or "0.5357"))
light.set_editor_property("atmosphere_sun_light", True)
light.set_editor_property("mobility", unreal.ComponentMobility.MOVABLE)
spawn(unreal.SkyAtmosphere, label="Atmosphere")
sky = spawn(unreal.SkyLight, (0, 0, 500), label="SkyLight").get_component_by_class(
    unreal.SkyLightComponent
)
sky.set_editor_property("mobility", unreal.ComponentMobility.MOVABLE)
sky.set_editor_property("real_time_capture", True)
sky.set_editor_property("intensity", float(os.environ.get("OC_SKY_LIGHT") or "1"))
fog = spawn(unreal.ExponentialHeightFog, label="Fog").get_component_by_class(
    unreal.ExponentialHeightFogComponent
)
fog.set_editor_property("fog_density", FOG_DENSITY)
fog.set_volumetric_fog(True)
spawn(unreal.VolumetricCloud, label="Clouds")

# The vegetation plugin's foliage materials read wind and tree data from this actor. Without it the
# sample trees fall back to their proxy cards, which look like green cubes rather than needles.
global_foliage_asset = unreal.load_asset(
    "/ProceduralVegetationEditor/SampleAssets/Materials/GlobalFoliageActor/"
    "BP_GlobalFoliageActor_UE5"
)
global_foliage = global_foliage_asset.generated_class() if global_foliage_asset else None
if global_foliage:
    spawn(global_foliage, label="Global Foliage")
    log("spawned BP_GlobalFoliageActor_UE5")
else:
    log("WARNING could not load BP_GlobalFoliageActor_UE5")

plane = unreal.load_asset("/Engine/BasicShapes/Plane")
ground = spawn(unreal.StaticMeshActor, (LENGTH / 2, 0, 0), label="Ground")
ground.static_mesh_component.set_static_mesh(plane)
ground.set_actor_scale3d(unreal.Vector(LENGTH / 100 + 6000, 6000, 1))
FLOOR = (os.environ.get("OC_FLOOR") or "blend").lower()
if FLOOR == "leaves":
    floor = textured_material("M_Litter", "forest_leaves_04", 250)
else:
    floor = forest_floor_material(
        "M_Litter", ("forest_leaves_04", "forest_ground_04", "leafy_grass"), (250, 600, 330)
    )
ground.static_mesh_component.set_material(0, floor)
# The road and shoulders are chained plane tiles from the shared centreline. Each tile is
# stretched 2 cm along its own X so the chord joints overlap; alternating the height by a
# fraction of a millimetre stops those coplanar overlaps from z-fighting.
# Darker and rougher than the scan, so a low sun ahead glints off it instead of turning it tan.
asphalt = textured_material("M_Asphalt", "asphalt_02", 300, 1.35, 0.42)
for index, (centre, yaw, span) in enumerate(LAYOUT.segments(0, LENGTH, SEGMENT)):
    tile = spawn(
        unreal.StaticMeshActor,
        (centre[0], centre[1], 1.0 + (index % 2) * 0.03),
        (0, 0, yaw),
        label="Road",
    )
    tile.static_mesh_component.set_static_mesh(plane)
    tile.set_actor_scale3d(unreal.Vector(span / 100 + 0.02, ROAD / 100, 1))
    tile.static_mesh_component.set_material(0, asphalt)

# A narrow dark verge hides the road's edge and gives the undergrowth somewhere to begin.
# It overlaps the asphalt by 2 cm under it, so no hairline gap can open at the joint.
verge_surface = verge_material("M_Verge")
for side in (-1, 1):
    lateral = side * (ROAD / 2 + SHOULDER / 2 - 2.0)
    for index, (centre, yaw, span) in enumerate(LAYOUT.segments(0, LENGTH, SEGMENT, lateral)):
        shoulder = spawn(
            unreal.StaticMeshActor,
            (centre[0], centre[1], 0.7 + (index % 2) * 0.03),
            (0, 0, yaw),
            label="Shoulder",
        )
        shoulder.static_mesh_component.set_static_mesh(plane)
        shoulder.set_actor_scale3d(unreal.Vector(span / 100 + 0.02, SHOULDER / 100, 1))
        shoulder.static_mesh_component.set_material(0, verge_surface)

# Trees: a fixed seed, in clumps rather than an even scatter, so the edge of the wood comes and goes
# along the road. The verge widens and narrows by stretch, and the wood runs 300 m back to fill the
# horizon.
tree = unreal.load_asset(TREE)
log(f"tree {TREE}: {type(tree).__name__}")
rng = random.Random(7)
count = int(DENSITY * (LENGTH + 20000) / 10000)


def verge(x, side):
    """How far the wood stands back from the road's edge, varying slowly along it."""
    phase = 1.7 if side > 0 else 4.1
    return 200 + 600 * (0.5 + 0.5 * math.sin(x / 4300 + phase)) * (
        0.5 + 0.5 * math.sin(x / 1700 + 2 * phase)
    )


def place(mesh, x, y, yaw, scale, folder, label=None, shape=None):
    """Spawns one mesh actor, working for both skeletal and static vegetation.

    `shape` pins the Nanite rebuild variant for this actor; when unset skeletal vegetation picks
    PreserveArea cards inside TREE_CARD_METRES and voxel bricks beyond.
    """
    # Rebuilding every skeletal tree is the default: it is the only way to get real leaf geometry
    # onto the plugin's assemblies. Set OC_NO_LEAF_REBUILD=1 to fall back to the shipped assets
    # (which render voxel cubes when r.Nanite.AllowVoxels=1) for A/B comparisons.
    # The shipped meshes are the default: their Nanite foliage renders needles (out/nanite_local).
    # OC_LEAF_REBUILD=1 brings back the rebuilt leaf-card variant, kept only for comparison.
    leaf_override = os.environ.get("OC_LEAF_REBUILD") == "1"
    if isinstance(mesh, unreal.SkeletalMesh):
        if leaf_override:
            # PreserveArea card triangles are the best overall look (step4m/step4n): real leaf
            # geometry close up and a soft leafy canopy at distance. OC_TREE_HYBRID=1 optionally
            # switches far trees to the voxel representation, whose silhouettes are exact but
            # which rasterizes as hard cubes.
            if os.environ.get("OC_TREE_HYBRID") == "1":
                near = abs(y) < TREE_CARD_METRES * 100.0
                shape = shape or ("preserve_area" if near else "voxelize")
            shape = shape or "preserve_area"
            mesh = foliage_mesh_copy(mesh, shape)
        actor = spawn(unreal.SkeletalMeshActor, (x, y, 0), (0, 0, yaw), label)
        actor.skeletal_mesh_component.set_skeletal_mesh_asset(mesh)
    else:
        actor = spawn(unreal.StaticMeshActor, (x, y, 0), (0, 0, yaw), label)
        actor.static_mesh_component.set_static_mesh(mesh)
        if os.environ.get("OC_LEAF_OVERRIDE") == "1":
            override_foliage(actor, mesh)
    if isinstance(scale, (tuple, list)):
        actor.set_actor_scale3d(unreal.Vector(*scale))
    else:
        actor.set_actor_scale3d(unreal.Vector(scale, scale, scale))
    actor.set_folder_path(folder)
    return actor


def conifer_scale(rng):
    """A deterministic age mix: a few veterans, a young understorey and the mature middle.

    A uniform 0.8..1.25 scale makes the stand read as one cloned cohort. Height and width are
    jittered separately so even same-age trees differ in build as well as size.
    """
    roll = rng.random()
    if roll < 0.07:
        height = rng.uniform(1.35, 1.6)  # old, dominant trees
    elif roll < 0.4:
        height = rng.uniform(0.55, 0.82)  # young regrowth
    else:
        height = rng.uniform(0.9, 1.3)
    return (
        height * rng.uniform(0.9, 1.06),
        height * rng.uniform(0.9, 1.06),
        height * rng.uniform(0.95, 1.08),
    )


def on_road(x, lateral):
    """World position of a point `lateral` cm off the centreline at road parameter x."""
    return LAYOUT.point(x, lateral)


# Clumps keep the edge of the wood irregular, but a third of the near trees are lone stems so
# the stand does not resolve into evenly spaced groups. Each clump has its own tightness.
clumps = [
    (
        rng.uniform(-10000, LENGTH + 10000),
        rng.choice((-1, 1)),
        rng.expovariate(1 / 6000),
        rng.uniform(350, 1800),
    )
    for _ in range(max(8, count // 5))
]
for index in range(count):
    if rng.random() < 0.33:
        x = rng.uniform(-10000, LENGTH + 10000)
        side = rng.choice((-1, 1))
        back = rng.expovariate(1 / 3500)
    else:
        cx, side, back, spread = rng.choice(clumps)
        x = cx + rng.gauss(0, spread)
        back = abs(back + rng.gauss(0, 600))
    px, py = on_road(x, side * (ROAD / 2 + verge(x, side) + back))
    scale = conifer_scale(rng)
    actor = place(tree, px, py, rng.uniform(0, 360), scale, "Trees")
    if index == 0:
        _, extent = actor.get_actor_bounds(False)
        log(
            f"first tree spans {extent.x * 2 / scale[2]:.0f} x "
            f"{extent.z * 2 / scale[2]:.0f} cm at scale 1"
        )

# A second, deeper band carries the wood to the horizon. It is deliberately sparser per square
# metre; the fog does the rest, and a solid wall of near trees would look like a fence.
deep_count = int(count * 1.5)
for _ in range(deep_count):
    x = rng.uniform(-10000, LENGTH + 10000)
    side = rng.choice((-1, 1))
    if rng.random() < 0.55:
        back = rng.uniform(2500, 25000)
    else:
        back = 25000 + max(0.0, DEPTH - 25000) * rng.random() ** 0.7
    px, py = on_road(x, side * (ROAD / 2 + verge(x, side) + back + rng.gauss(0, 800)))
    place(tree, px, py, rng.uniform(0, 360), conifer_scale(rng), "Trees/Deep")
log(f"trees: {count} near + {deep_count} deep to {DEPTH / 100:.0f} m")

if UNDERGROWTH or VERGE_UNDERGROWTH:
    under = [(unreal.load_asset(path), low, high) for path, low, high in UNDERGROWTH_MESHES]
    # The deciduous shrub is a multi-metre bush at scale 1; the low plant is a seedling.
    # Near the road we want the seedling, or the shrub only at a fraction of its size, so the
    # verge gains a low layer instead of a hedge that swallows the asphalt edge.
    shrub, plant = under[0], under[2]
    rng_under = random.Random(19)
    for _ in range(UNDERGROWTH):
        x = rng_under.uniform(-5000, LENGTH + 5000)
        side = rng_under.choice((-1, 1))
        u = rng_under.random()
        if u < 0.32:
            distance = rng_under.uniform(25, 300)
        elif u < 0.68:
            distance = rng_under.uniform(300, 900)
        else:
            distance = 900 + 3500 * rng_under.random() ** 0.7
        if distance < 300:
            if rng_under.random() < 0.3:  # seedlings read as cloned when they line the road
                mesh, _, _ = plant
                scale = rng_under.uniform(0.5, 1.1)
            else:
                mesh, _, _ = shrub
                scale = rng_under.uniform(0.2, 0.38)
        else:
            mesh, low, high = rng_under.choice(under)
            scale = rng_under.uniform(low, high)
        px, py = on_road(x, side * (ROAD / 2 + distance))
        place(mesh, px, py, rng_under.uniform(0, 360), scale, "Undergrowth", shape="preserve_area")

    # A low, dense band right at the verge. The 18 cm minimum keeps every stem off the
    # asphalt; the 1.6 power crowds them towards the edge. Mostly seedlings, with a few
    # small shrubs set back far enough that their branches only lean over the gravel.
    rng_verge = random.Random(23)
    for _ in range(VERGE_UNDERGROWTH):
        x = rng_verge.uniform(-5000, LENGTH + 5000)
        side = rng_verge.choice((-1, 1))
        if rng_verge.random() < 0.3:  # was 0.72: a row of identical lime seedlings
            mesh, _, _ = plant
            distance = 18 + 220 * rng_verge.random() ** 1.6
            scale = rng_verge.uniform(0.45, 1.05)
        else:
            mesh, _, _ = shrub
            distance = 140 + 260 * rng_verge.random()
            scale = rng_verge.uniform(0.18, 0.34)
        px, py = on_road(x, side * (ROAD / 2 + distance))
        place(
            mesh,
            px,
            py,
            rng_verge.uniform(0, 360),
            scale,
            "Undergrowth/Verge",
            shape="preserve_area",
        )
    log(f"undergrowth: {UNDERGROWTH} scattered + {VERGE_UNDERGROWTH} low at the verge")

if not unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL):
    raise RuntimeError(f"could not save {LEVEL}")
log(f"saved {LEVEL} with {count} trees along {LENGTH / 100:.0f} m")
