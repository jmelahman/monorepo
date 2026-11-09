"""Imports the Megascans assets prep_megascans.py prepared into /Game/Megascans/<biome>/<slug>:
each asset's textures, its meshes, and a material instance of one master per kind on every slot.

Two stages, because Interchange finishes an import after the call returns and then rewrites the
meshes' material slots (see the README's pitfalls):
    OC_STAGE=import bash scripts/ue_run.sh megascans.py megascans_import.log
    OC_STAGE=assign bash scripts/ue_run.sh megascans.py megascans_assign.log
OC_ONLY narrows either stage to a comma list of biomes or biome/slug pairs, and OC_SKIP leaves out
such a list (for a raw scan that crashes the commandlet). Assets already imported are skipped unless
OC_FORCE=1.
"""

import importlib.util
import json
import os

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/megascans.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = sibling("ue_lib")
assets, tools, Graph = lib.assets, lib.tools, lib.Graph
PREP = "/workspace/ue/fab_prep"
ROOT = "/Game/Megascans"
STAGE = os.environ.get("OC_STAGE", "import")
ONLY = set(filter(None, (os.environ.get("OC_ONLY") or "").split(",")))
SKIP = set(filter(None, (os.environ.get("OC_SKIP") or "").split(",")))
FORCE = os.environ.get("OC_FORCE") == "1"
TEXTURES = {
    # file: (suffix, sRGB, compression)
    "D": ("D", True, unreal.TextureCompressionSettings.TC_DEFAULT),
    "N": ("N", False, unreal.TextureCompressionSettings.TC_NORMALMAP),
    "M": ("M", False, unreal.TextureCompressionSettings.TC_MASKS),
    "T": ("T", True, unreal.TextureCompressionSettings.TC_DEFAULT),
}


def log(message):
    unreal.log(f"MEGASCANS {message}")


def wanted(record):
    keys = {record["biome"], f"{record['biome']}/{record['slug']}"}
    return (not ONLY or bool(keys & ONLY)) and not keys & SKIP


def folder_of(record):
    return f"{ROOT}/{record['biome']}/{record['slug']}"


def import_one(path, folder, name):
    task = unreal.AssetImportTask()
    task.filename = path
    task.destination_path = folder
    task.destination_name = name
    task.replace_existing = True
    task.automated = True
    task.save = False
    tools.import_asset_tasks([task])
    return [unreal.load_asset(p) for p in (task.get_editor_property("imported_object_paths") or [])]


def mesh_name(record, fbx):
    """SM_<slug>_<variation>: VarA.. for plants, the file's last part otherwise."""
    stem = fbx.rsplit(".", 1)[0]
    var = next((p for p in stem.split("_") if p.startswith("Var")), None)
    return f"SM_{record['slug']}" + (f"_{var}" if var else "")


def import_asset(record):
    folder = folder_of(record)
    src = f"{PREP}/{record['biome']}/{record['slug']}"
    for key, (suffix, srgb, compression) in TEXTURES.items():
        name = f"T_{record['slug']}_{suffix}"
        if not os.path.exists(f"{src}/{key}.png"):
            continue
        if assets.does_asset_exist(f"{folder}/{name}") and not FORCE:
            continue
        tex = next(
            (
                a
                for a in import_one(f"{src}/{key}.png", folder, name)
                if isinstance(a, unreal.Texture)
            ),
            None,
        )
        if tex is None:
            raise RuntimeError(f"could not import {src}/{key}.png")
        tex.set_editor_property("srgb", srgb)
        tex.set_editor_property("compression_settings", compression)
        assets.save_loaded_asset(tex, False)
    for mesh in record["meshes"]:
        name = mesh_name(record, mesh["file"])
        if assets.does_asset_exist(f"{folder}/{name}") and not FORCE:
            continue
        import_one(f"{src}/mesh/{mesh['file']}", folder, name)
    assets.save_directory(folder, only_if_is_dirty=True, recursive=True)
    log(f"imported {folder}")


def texture_param(g, name, kind):
    node = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900)
    node.set_editor_property("parameter_name", name)
    node.set_editor_property(
        "sampler_type",
        {
            "colour": unreal.MaterialSamplerType.SAMPLERTYPE_COLOR,
            "normal": unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL,
            "masks": unreal.MaterialSamplerType.SAMPLERTYPE_MASKS,
        }[kind],
    )
    node.set_editor_property(
        "texture",
        unreal.load_asset(
            {
                "colour": "/Engine/EngineResources/DefaultTexture",
                "normal": "/Engine/EngineMaterials/DefaultNormal",
                "masks": "/Engine/EngineResources/WhiteSquareTexture",
            }[kind]
        ),
    )
    return node


def vector_param(g, name, value):
    return g.node(
        unreal.MaterialExpressionVectorParameter,
        parameter_name=name,
        default_value=unreal.LinearColor(*value, 1.0),
    )


def scalar_param(g, name, value):
    return g.node(
        unreal.MaterialExpressionScalarParameter, parameter_name=name, default_value=value
    )


def master(kind):
    """M_MS_Plant: masked two-sided foliage lit through by its translucency map. M_MS_Prop: opaque.
    Both take D (colour, alpha the cut-out), N and M (AO, roughness, height), and a Tint."""
    name = {"plant": "M_MS_Plant", "prop": "M_MS_Prop"}[kind]
    g = Graph(name, f"{ROOT}/Materials")
    d = texture_param(g, "D", "colour")
    n = texture_param(g, "N", "normal")
    m = texture_param(g, "M", "masks")
    colour = g.mul(g.mask(d, "rgb"), g.mask(vector_param(g, "Tint", (1.0, 1.0, 1.0)), "rgb"))
    rough = g.saturate(g.mul(g.mask(m, "g"), scalar_param(g, "Roughness", 1.0)))
    material = g.material
    if kind == "plant":
        t = texture_param(g, "T", "colour")
        sss = g.mul(
            g.mask(t, "rgb"), g.mask(vector_param(g, "TranslucencyTint", (0.5, 0.6, 0.4)), "rgb")
        )
        g.output(
            colour,
            g.mask(n, "rgb"),
            rough,
            specular=0.4,
            opacity_mask=g.mask(d, "a"),
            subsurface=sss,
            ao=g.mask(m, "r"),
        )
        material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_MASKED)
        material.set_editor_property(
            "shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE
        )
        material.set_editor_property("two_sided", True)
        material.set_editor_property("opacity_mask_clip_value", 0.33)
    else:
        g.output(colour, g.mask(n, "rgb"), rough, ao=g.mask(m, "r"))
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    return g.finish()


def set_nanite(mesh, foliage):
    settings = mesh.get_editor_property("nanite_settings")
    settings.set_editor_property("enabled", True)
    if foliage:
        settings.set_editor_property(
            "shape_preservation", unreal.NaniteShapePreservation.PRESERVE_AREA
        )
    mesh.set_editor_property("nanite_settings", settings)


def assign_asset(record, masters):
    folder = folder_of(record)
    slug = record["slug"]
    kind = "plant" if record["kind"] == "plant" and record["opacity"] else "prop"
    textures = {
        key: unreal.load_asset(f"{folder}/T_{slug}_{key}")
        for key in ("D", "N", "M", "T")
        if assets.does_asset_exist(f"{folder}/T_{slug}_{key}")
    }
    if kind == "prop":
        textures.pop("T", None)
    vectors = {}
    if kind == "plant" and record.get("translucency_tint"):
        vectors["TranslucencyTint"] = (*record["translucency_tint"][:3], 1.0)
    mi = lib.instance(f"MI_{slug}", masters[kind], folder, textures, vectors=vectors)
    stray = []
    for path in assets.list_assets(folder, recursive=True):
        asset = unreal.load_asset(path)
        if isinstance(asset, unreal.StaticMesh):
            # Each slot anew: the array's items are copies, so setting one in place changes nothing.
            slots = [
                unreal.StaticMaterial(mi, slot.material_slot_name, slot.uv_channel_data)
                for slot in asset.get_editor_property("static_materials")
            ]
            asset.set_editor_property("static_materials", slots)
            kept = asset.get_editor_property("static_materials")
            if any(s.material_interface != mi for s in kept):
                raise RuntimeError(f"{asset.get_name()} kept its old materials")
            set_nanite(asset, kind == "plant")
            assets.save_loaded_asset(asset, False)
            box = asset.get_bounding_box()
            size = box.max - box.min
            log(f"mesh {asset.get_name()} {size.x:.0f}x{size.y:.0f}x{size.z:.0f} cm")
        elif isinstance(asset, unreal.MaterialInterface) and asset != mi:
            stray.append(path)
    for path in stray:  # the empty materials Interchange made from the FBX
        assets.delete_asset(path)
    log(f"assigned {folder} ({kind}, {len(stray)} stray materials removed)")


def main():
    with open(f"{PREP}/index.json") as f:
        records = [r for r in json.load(f) if wanted(r)]
    log(f"{STAGE}: {len(records)} assets")
    if STAGE == "import":
        for record in records:
            import_asset(record)
    elif STAGE == "assign":
        masters = {kind: master(kind) for kind in ("plant", "prop")}
        for record in records:
            if record["kind"] != "surface":
                assign_asset(record, masters)
    else:
        raise RuntimeError(f"unknown OC_STAGE {STAGE}")


main()
