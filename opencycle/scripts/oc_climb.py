"""The climb's world, inside the editor: its ground material, the look (tints on MI_ClimbGround),
and stills from the editor viewport. Loaded afresh for each job by oc_server.py; oc.py is the way
to run any of it. Self-contained apart from ue_lib's Graph helper.

The crash rules, enforced here rather than remembered:
- The ground material is rebuilt only by op_rebuild_ground, which refuses if L_Hero was opened in
  this editor session or any loaded landscape uses the material (oc.py restarts the editor onto
  the empty map first). The look changes are material-instance parameters: nothing compiles.
- L_Hero is loaded only when it isn't already the open map (reloading it open segfaults), and
  never with its skeletal trees, which overflow the GPU skinning buffer.
- Every op is a generator; stills are taken from the editor viewport so grass and LODs stream in.
"""

import importlib.util
import json
import os
import re

import unreal

SRC = "/workspace/ue/world_gen/climb"
PREP = "/workspace/ue/fab_prep"
MS = "/Game/Megascans"
FOLDER = "/Game/Climb"
LEVEL = f"{FOLDER}/L_Hero"
MATERIAL = "M_ClimbGround"
INSTANCE = "MI_ClimbGround"
STILLS = "/workspace/ue/out/climb_stills"
SHOTS = (0.08, 0.3, 0.5, 0.72, 0.95)  # fractions of the way up, for the road-level stills
FAR = 6.0  # far off, each surface is also sampled this many times larger, against tiling


def log(message):
    unreal.log(f"OC {message}")


def _load_lib():
    here = os.path.dirname(os.path.abspath(__file__))
    spec = importlib.util.spec_from_file_location("ue_lib", os.path.join(here, "ue_lib.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


lib = _load_lib()
assets = lib.assets

# --- what the ground is made of ----------------------------------------------------------------

# Each paint layer's surface: (biome, slug, default tint). Rock and Dirt are the climb's own: grey
# granite and gravel rather than the island's red-brown stony soil and yellow sand.
SURFACES = {
    "Grass": ("shared", "uncut_grass", (0.85, 0.9, 0.7)),
    "Forest": ("forest", "forest_floor", (1.0, 1.0, 1.0)),
    "Rock": ("mountain", "granite_rocky_ground", (0.85, 0.85, 0.85)),
    "Snow": ("coast", "thai_wet_sand", (1.0, 1.0, 1.0)),
    "Desert": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
    "Sand": ("coast", "trampled_beach_sand", (1.0, 1.0, 1.0)),
    "Dirt": ("shared", "gravel", (0.8, 0.8, 0.8)),
}
# A second surface per layer and how it mixes in: "patches" (noise, metres across) or "wet"
# (below that height in cm).
MIXES = {
    "Forest": ("forest", "mossy_forest_floor", "patches", 18.0),
    "Desert": ("desert", "dry_mud_ground", "patches", 60.0),
    "Sand": ("coast", "thai_wet_sand", "wet", 120.0),
    "Grass": ("shared", "lawn_grass", "patches", 30.0),
    "Rock": ("forest", "rocky_forest_path", "patches", 12.0),
}
ROUGHNESS = {"Snow": 0.55, "Sand": 0.9}
# Snow keeps wet sand's relief but not its colour: shaded from the first colour in its hollows to
# the second on its crests.
FLAT = {"Snow": ((0.62, 0.68, 0.78), (0.86, 0.89, 0.93))}
# Grass per layer: (biome, slug, instances per 100 m^2, cull start, cull end in cm). Scans whose
# meshes take 5 GB or more to build are left out: with the rest of the editor they run the machine
# out of memory.
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
        # The woods' floor reads as bare lawn texture without grass: beech woods by a path have
        # tufts and clumps between the ferns, thinner than the open verges.
        ("shared", "grass_clumps", 16.0, 3000, 4500),
        ("shared", "wild_grass", 16.0, 3000, 4500),
        ("shared", "tall_grass", 1.6, 4000, 6000),
    ),
}


def tint_names():
    """The keys a look may set: a layer ("Dirt"), or a layer's second surface ("Rock mix")."""
    return sorted(list(SURFACES) + [f"{k} mix" for k in MIXES])


def _tile_cm(biome, slug):
    """A surface's tile from its scan's size, e.g. "2x2" metres; 2 m when it doesn't say."""
    with open(f"{PREP}/{biome}/{slug}/asset.json") as f:
        found = re.findall(r"[\d.]+", str(json.load(f).get("scan_area") or ""))
    return float(found[0]) * 100 if found else 200.0


def _texture(biome, slug, key):
    path = f"{MS}/{biome}/{slug}/T_{slug}_{key}"
    if not assets.does_asset_exist(path):
        raise RuntimeError(f"no {path}: run megascans.py first")
    return unreal.load_asset(path)


def _graph(name, folder):
    """A Graph on the existing material emptied, so the instance that points at it keeps its
    parent; a new one only if there is none."""
    existing = unreal.load_asset(f"{folder}/{name}")
    if existing is None:
        return lib.Graph(name, folder)
    g = lib.Graph.__new__(lib.Graph)
    g.material, g.name, g.y = existing, name, 0
    lib.edit.delete_all_material_expressions(existing)
    return g


class Ground:
    """The landscape material: each layer's surfaces height-blended, sampled near and far, and
    varied in brightness over tens of metres, so the ground doesn't show its tiles. Every
    surface's colour is multiplied by the vector parameter "<name> tint"."""

    def __init__(self):
        self.g = g = _graph(MATERIAL, FOLDER)
        world = g.node(unreal.MaterialExpressionWorldPosition)
        camera = g.node(unreal.MaterialExpressionCameraPositionWS)
        distance = g.node(unreal.MaterialExpressionDistance)
        g.link(world, distance, "A")
        g.link(camera, distance, "B")
        # 0 within 15 m, 1 past 60 m.
        self.far = g.saturate(g.mul(g.add(distance, -1500.0), 1 / 4500))
        self.height = g.mask(world, "b")
        self.macro = g.world_noise(0.0004, 0.82, 1.12, 4)
        # Steep ground is sampled from the side as well, on whichever of the XZ and YZ planes it
        # faces more, or a top-down sample streaks down a cliff. 0 up to 37 degrees, 1 past 53.
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

    def surface(self, biome, slug, tint, label):
        g = self.g
        tile = _tile_cm(biome, slug)
        d, n, m = (_texture(biome, slug, k) for k in ("D", "N", "M"))
        near_uv, far_uv = g.world_uv(tile), g.world_uv(tile * FAR, (0.37, 0.61))
        near_d = g.mask(g.sample(d, near_uv), "rgb")
        far_d = g.mask(g.sample(d, far_uv), "rgb")
        near_n = g.mask(g.sample(n, near_uv, "normal"), "rgb")
        far_n = g.mask(g.sample(n, far_uv, "normal"), "rgb")
        side_d = g.mask(g.sample(d, g.mul(self.side, 1.0 / tile)), "rgb")
        masks = g.sample(m, near_uv, "masks")
        top = g.lerp(near_d, far_d, g.mul(self.far, 0.7))
        colour = g.mul(g.lerp(top, side_d, self.steep), g.param(f"{label} tint", tint))
        # "<label> grey", 0 to 1, takes the scan's own hue out: a tint can darken warm gravel but
        # not make it grey.
        luma = g.op(unreal.MaterialExpressionDotProduct, colour, g.const((0.3, 0.59, 0.11)))
        colour = g.lerp(colour, luma, g.param(f"{label} grey", 0.0))
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
        base = self.surface(biome, slug, tint, name)
        if name in MIXES:
            biome2, slug2, how, size = MIXES[name]
            other = self.surface(biome2, slug2, (1.0, 1.0, 1.0), f"{name} mix")
            if how == "patches":
                alpha = g.world_noise(1.0 / (size * 100), -1.5, 2.0, 3)
            else:  # wet below a height
                alpha = g.mul(g.add(g.mul(self.height, -1.0), size), 1 / 60)
            # The mix follows the surfaces' own heights at its edge, not a soft fade.
            alpha = g.saturate(g.add(alpha, g.add(other["height"], g.mul(base["height"], -1.0))))
            base = {k: g.lerp(base[k], other[k], alpha) for k in base}
        if name in FLAT:
            low, high = FLAT[name]
            shade = g.lerp(g.const(low), g.const(high), base["height"])
            base["colour"] = g.mul(shade, g.param(f"{name} tint", tint))
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
        g.output(
            g.mul(outputs["colour"], self.macro), outputs["normal"], outputs["rough"], specular=0.35
        )
        out = g.node(unreal.MaterialExpressionLandscapeGrassOutput)
        inputs = []
        for name, gt in grass_types.items():
            gi = unreal.GrassInput()
            gi.set_editor_property("name", name)
            gi.set_editor_property("grass_type", gt)
            inputs.append(gi)
        out.set_editor_property("grass_types", inputs)
        for name in grass_types:
            g.link(
                g.node(unreal.MaterialExpressionLandscapeLayerSample, parameter_name=name),
                out,
                name,
            )
        return g.finish()


def _grass_type(layer):
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
            v.set_editor_property("align_to_surface", density > 0.5)
            v.set_editor_property("cast_dynamic_shadow", True)
            varieties.append(v)
    gt.set_editor_property("grass_varieties", varieties)
    assets.save_loaded_asset(gt, False)
    log(f"grass {layer}: {len(varieties)} varieties")
    return gt


# --- the crash guards ---------------------------------------------------------------------------


def _current_map():
    world = unreal.UnrealEditorSubsystem().get_editor_world()
    return world.get_path_name().split(".")[0] if world else ""


def _landscape_uses_ground():
    """Whether a loaded landscape draws with M_ClimbGround, directly or through an instance. (The
    startup map's own landscape doesn't, and is harmless.)"""
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


def _guard_rebuild():
    if getattr(unreal, "_oc_opened_hero", False) or _current_map() == LEVEL:
        raise RuntimeError(
            f"refusing to rebuild {MATERIAL}: {LEVEL} was opened in this editor. Rebuilding it "
            "under a loaded landscape recompiles every section in one frame and the GPU is lost. "
            "Use `oc.py rebuild-ground`, which restarts the editor onto the empty map first."
        )
    if _landscape_uses_ground():
        raise RuntimeError(f"refusing to rebuild {MATERIAL}: a loaded landscape uses it")


def _ensure_level():
    """Opens L_Hero (unless it is the open map: reloading it open segfaults) with every actor but
    the skeletal trees loaded, and points its landscape at MI_ClimbGround. Returns the world."""
    mi = unreal.load_asset(f"{FOLDER}/{INSTANCE}")
    if mi is None:
        raise RuntimeError(f"no {FOLDER}/{INSTANCE}: run `oc.py rebuild-ground` first")
    world = unreal.UnrealEditorSubsystem().get_editor_world()
    if _current_map() != LEVEL:
        unreal._oc_opened_hero = True  # set before the load: from here on the ground is off limits
        world = unreal.EditorLoadingAndSavingUtils.load_map(LEVEL)
        wp = getattr(unreal, "WorldPartitionBlueprintLibrary", None)
        if wp is None:
            raise RuntimeError(
                "no WorldPartitionBlueprintLibrary: cannot keep the skeletal trees out"
            )
        descs = wp.get_actor_descs()
        # The skeletal PVE trees overflow the GPU's skinning buffer, and the Radeon driver has
        # crashed with them loaded. They are never loaded.
        keep = [d for d in descs if "SkeletalMeshActor" not in str(d.native_class)]
        log(f"opened {LEVEL}; leaving {len(descs) - len(keep)} skeletal trees unloaded")
        wp.load_actors([d.guid for d in keep])
        yield 60
    unreal._oc_opened_hero = True
    changed = 0
    for actor in unreal.EditorActorSubsystem().get_all_level_actors():
        if (
            isinstance(actor, unreal.LandscapeProxy)
            and actor.get_editor_property("landscape_material") != mi
        ):
            actor.set_editor_property("landscape_material", mi)
            changed += 1
    if changed:
        yield 120
        unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
        unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
        log(f"landscape pointed at {INSTANCE} ({changed} pieces); saved")
    return world


# --- ops (run by oc_server.py; each a generator returning a result dict) -------------------------


def op_ping():
    yield
    return {"notes": [f"map {_current_map()}"]}


def op_rebuild_ground(rebuild_grass=False):
    """M_ClimbGround and MI_ClimbGround, rebuilt. Only on the empty map (see _guard_rebuild).
    Saved grass types are reused: building them takes over ten minutes, on one core."""
    _guard_rebuild()
    grass = {}
    for layer in GRASS:
        saved = unreal.load_asset(f"{FOLDER}/Grass/GT_{layer}")
        grass[layer] = saved if saved and not rebuild_grass else _grass_type(layer)
        yield 10
    material = Ground().build(grass)
    yield 30
    mi = unreal.load_asset(f"{FOLDER}/{INSTANCE}")
    if mi is None:
        mi = lib.tools.create_asset(
            INSTANCE,
            FOLDER,
            unreal.MaterialInstanceConstant,
            unreal.MaterialInstanceConstantFactoryNew(),
        )
    lib.edit.set_material_instance_parent(mi, material)
    lib.edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)
    return {"notes": [f"rebuilt {MATERIAL}; {INSTANCE} points at it (existing tints are kept)"]}


def _old_build():
    """climb_world.py, the level build that has run without crashing (as separate ground and dress
    runs), loaded as a library."""
    spec = importlib.util.spec_from_file_location(
        "climb_world", os.path.join(os.path.dirname(os.path.abspath(__file__)), "climb_world.py")
    )
    module = importlib.util.module_from_spec(spec)
    module.OC_LIBRARY = True
    spec.loader.exec_module(module)
    return module


def op_build_level(stage):
    """One stage of the level build from world_gen/climb: "ground" (a fresh L_Hero with the
    landscape, deleting the old one) or "dress" (the path, fence, ride track and trees, then
    stills). Each only on a fresh editor: oc.py build-level restarts before each, because the
    ground stage deletes L_Hero and the dress stage loads it, and neither may happen with it
    open."""
    if stage not in ("ground", "dress"):
        raise RuntimeError(f"stage {stage!r}: ground or dress")
    if getattr(unreal, "_oc_opened_hero", False) or _current_map() == LEVEL:
        raise RuntimeError("build-level needs a fresh editor; run it through `oc.py build-level`")
    unreal._oc_opened_hero = True
    cw = _old_build()
    cw.STAGE = stage
    yield from cw.main()
    files = [f"{STILLS}/{name}.png" for name, _, _ in cw.cameras()] if stage == "dress" else []
    return {"notes": [f"built stage {stage}"], "files": files}


def _grass(density, cull):
    """Sets the saved grass types' densities and cull distances to GRASS's times these factors.
    Grass is regenerated, nothing compiles."""
    rows = {}
    for layer, table in GRASS.items():
        for _biome, slug, base, start, end in table:
            rows.setdefault(layer, []).append((f"/{slug}/", base, start, end))
    changed = 0
    for layer, table in rows.items():
        gt = unreal.load_asset(f"{FOLDER}/Grass/GT_{layer}")
        if gt is None:
            raise RuntimeError(f"no GT_{layer}: run `oc.py rebuild-ground --rebuild-grass`")
        varieties = list(gt.get_editor_property("grass_varieties"))
        for v in varieties:
            path = v.get_editor_property("grass_mesh").get_path_name()
            row = next((r for r in table if r[0] in path), None)
            if row is None:
                continue
            _, base, start, end = row
            v.set_editor_property("grass_density", unreal.PerPlatformFloat(default=base * density))
            v.set_editor_property(
                "start_cull_distance", unreal.PerPlatformInt(default=int(start * cull))
            )
            v.set_editor_property(
                "end_cull_distance", unreal.PerPlatformInt(default=int(end * cull))
            )
            changed += 1
        gt.set_editor_property("grass_varieties", varieties)
        assets.save_loaded_asset(gt, False)
    unreal.SystemLibrary.execute_console_command(None, "grass.FlushCache")
    log(f"grass x{density} density, x{cull} cull: {changed} varieties")


def _fog(fog):
    """The haze: [density] or [density, falloff] for the open level's ExponentialHeightFog (the
    template's are 0.02 and 0.2). Falloff 0.2 thins the fog e^2 per 100 m of height, so on a climb
    66-576 m up it barely shows: a low falloff spreads the haze up the mountain and over the map's
    edge."""
    density, *rest = fog if isinstance(fog, (list, tuple)) else [fog]
    fogs = [
        a
        for a in unreal.EditorActorSubsystem().get_all_level_actors()
        if isinstance(a, unreal.ExponentialHeightFog)
    ]
    if not fogs:
        raise RuntimeError("no ExponentialHeightFog in the level")
    for fog in fogs:
        comp = fog.get_component_by_class(unreal.ExponentialHeightFogComponent)
        comp.set_editor_property("fog_density", float(density))
        if rest:
            comp.set_editor_property("fog_height_falloff", float(rest[0]))
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    log(f"fog {fog}")


def _tune(tints, greys=None):
    greys = greys or {}
    names = tint_names()
    bad = [k for k in [*tints, *greys] if k not in names]
    if bad:
        raise RuntimeError(f"unknown tint {bad}; use one of {names}")
    mi = unreal.load_asset(f"{FOLDER}/{INSTANCE}")
    if mi is None:
        raise RuntimeError(f"no {FOLDER}/{INSTANCE}: run `oc.py rebuild-ground` first")
    for name, rgb in tints.items():
        lib.edit.set_material_instance_vector_parameter_value(
            mi, f"{name} tint", unreal.LinearColor(*rgb, 1.0)
        )
    for name, amount in greys.items():
        lib.edit.set_material_instance_scalar_parameter_value(mi, f"{name} grey", float(amount))
    lib.edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)
    log(f"tuned {tints} grey {greys}")


def cameras(only=None):
    """(name, position, look-at) in cm: a rider's eye along the road, looking 30 m up it, and one
    from above the hairpin back down the lower traverse."""
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
    if only:
        known = [n for n, _, _ in out]
        unknown = [n for n in only if n not in known]
        if unknown:
            raise RuntimeError(f"unknown shots {unknown}; use {known}")
        out = [c for c in out if c[0] in only]
    return [(n, tuple(v * 100 for v in p), tuple(v * 100 for v in a)) for n, p, a in out]


def _stills(shots, out):
    """Shot from the editor's own viewport, moved to each view: a stand-alone camera doesn't drive
    grass, texture streaming or tree LODs. A still counts only once its file is written."""
    plan = cameras(shots)
    os.makedirs(out, exist_ok=True)
    editor = unreal.UnrealEditorSubsystem()
    yield 60  # let the level's streaming, Nanite and lighting settle
    files = []
    for name, where, target in plan:
        path = f"{out}/{name}.png"
        if os.path.exists(path):
            os.remove(path)
        eye = unreal.Vector(*where)
        editor.set_level_viewport_camera_info(
            eye, unreal.MathLibrary.find_look_at_rotation(eye, unreal.Vector(*target))
        )
        yield 120  # grass, textures and shadows stream in around the new view
        unreal.AutomationLibrary.take_high_res_screenshot(1280, 720, path)
        for _ in range(30):
            yield 30  # the shot is written a few frames later
            if os.path.exists(path) and os.path.getsize(path) > 0:
                break
        else:
            raise RuntimeError(f"the still {path} was never written")
        yield 10
        files.append(path)
        log(f"still {path}")
    return files


def op_stills(shots=None, out=None):
    yield from _ensure_level()
    files = yield from _stills(shots, out or STILLS)
    return {"files": files}


def op_look(tint=None, grey=None, grass=None, fog=None, stills=True, shots=None, out=None):
    """Recolour through MI_ClimbGround's parameters (tint: {"Dirt": [r, g, b]}, grey: {"Dirt":
    0.8}), grass ([density, cull] factors on GRASS), fog (density), then stills. Nothing
    compiles."""
    if tint or grey:
        _tune({k: tuple(v) for k, v in (tint or {}).items()}, grey)
    if grass:
        _grass(*grass)
    yield
    notes = [f"tuned {tint} grey {grey}"] if tint or grey else []
    notes += [f"grass {grass}"] if grass else []
    if fog is not None:
        yield from _ensure_level()
        _fog(fog)
        notes.append(f"fog {fog}")
        yield 30
    result = {"notes": notes}
    if stills:
        yield from _ensure_level()
        result["files"] = yield from _stills(shots, out or STILLS)
    return result
