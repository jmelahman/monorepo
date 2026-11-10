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
import math
import os
import re
import time

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
STILL_SIZE = (1920, 1080)
# Frames to wait at each view: grass builds around the camera within a time budget, and at 120
# frames whole wooded stretches were still bare.
STILL_SETTLE = 480
STILL_CVARS = ("r.Streaming.PoolSize 3000", "r.Streaming.FullyLoadUsedTextures 1")
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
    "Grass": ("shared", "uncut_grass", (0.55, 0.58, 0.42)),
    # Brown needle and leaf litter: the scanned forest floor alone is olive green and read as lawn.
    "Forest": ("mountain", "alpine_needles", (0.5, 0.46, 0.44)),
    "Rock": ("mountain", "granite_rocky_ground", (0.85, 0.85, 0.85)),
    "Snow": ("coast", "thai_wet_sand", (1.0, 1.0, 1.0)),
    "Desert": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
    # The climb has no beach: its "Sand" layer is the cut banks' soil, apart from the path's edge
    # so that the banks' stones and ferns (GRASS) don't grow through the tarmac.
    # v19: stony_soil, not soil_mud: the smooth mud scan made the banks read as smeared putty.
    "Sand": ("shared", "stony_soil", (0.62, 0.53, 0.46)),
    # The path's edge: dark soil with leaf litter over it, as in the reference photos.
    "Dirt": ("forest", "soil_mud", (0.62, 0.53, 0.46)),
}
# A second surface per layer and how it mixes in: "patches" (noise, metres across) or "wet"
# (below that height in cm).
MIXES = {
    # Moss in patches through the litter: QA found the floor between the ferns bare soil.
    # v19: grass patches, not moss: mossy_forest_floor is a dull olive (0.19 sRGB) that read as
    # grey-teal felt under the sky, never as the style shots' green carpet.
    "Forest": ("shared", "uncut_grass", "patches", 10.0),
    "Desert": ("desert", "dry_mud_ground", "patches", 60.0),
    # The soil's litter is brown needles: olive forest_floor here made green splotches on brown.
    "Sand": ("mountain", "alpine_needles", "patches", 4.0),
    "Grass": ("shared", "lawn_grass", "patches", 30.0),
    "Rock": ("forest", "rocky_forest_path", "patches", 12.0),
    "Dirt": ("mountain", "alpine_needles", "patches", 4.0),
}
# The tuned look (see `oc.py look`), set on every rebuilt MI_ClimbGround.
# alpine_needles (the Forest base, and the Sand and Dirt mixes) is pale beige as scanned: untinted
# it made every bank and the woods' floor read as sand (found by false colour, v12).
# Halved from v13 once exposure was locked (`--post ev=-4.5`): under auto exposure darker albedo
# came out just as bright.
LITTER = (0.22, 0.17, 0.12)
LOOK_TINTS = {
    "Rock": (0.45, 0.42, 0.37),
    # Warmer than neutral litter: under the blue sky the floor read blue-grey (v15 false colour).
    "Forest": (0.26, 0.17, 0.09),
    # Moss and grass: a deep green, not v13's pale lime. Grass mix (lawn_grass patches) had no
    # tint at all: it made the pale beech bank and the far hillside's leopard spots (v15).
    # v18: a little yellower, as the moss and lawn read teal felt under the sky (v17 crops).
    "Forest mix": (0.55, 0.75, 0.3),
    "Grass": (0.14, 0.23, 0.04),
    "Grass mix": (0.14, 0.22, 0.04),
    "Sand mix": LITTER,
    "Dirt mix": LITTER,
    # soil_mud as scanned read pale and dusty on the banks and the path's edge (QA, v12): dark,
    # damp earth as in the reference photos.
    "Sand": (0.24, 0.18, 0.14),
    "Dirt": (0.22, 0.17, 0.13),
}
LOOK_GREYS = {
    "Grass": 0.0,
    "Grass mix": 0.0,
    "Rock": 0.5,
    "Rock mix": 0.3,
    "Forest mix": 0.0,
    "Dirt mix": 0.3,
    "Sand mix": 0.3,
}
ROUGHNESS = {"Snow": 0.55}
# Every other layer is at least this rough: the scans' low roughness made the soil glitter.
ROUGH_FLOOR = 0.75
# Steep ground on these layers is shaded toward darker, browner soil ("<layer> steep" parameter):
# the cut banks read pale beige against the reference photos' dark earth.
STEEP = {
    "Forest": (0.6, 0.5, 0.42),
    "Sand": (0.6, 0.5, 0.42),
    "Dirt": (0.6, 0.5, 0.42),
}
# Snow keeps wet sand's relief but not its colour: shaded from the first colour in its hollows to
# the second on its crests.
FLAT = {"Snow": ((0.62, 0.68, 0.78), (0.86, 0.89, 0.93))}
# Grass per layer: (biome, slug, instances per 100 m^2, cull start, cull end in cm[, (min, max)
# scale, 0.6 to 1.4 if left out]). The density is per mesh: a scan with six variants draws six
# times it (fern at 2.0 was 12 per 100 m^2), and `look --grass` multiplies it again. Scans whose
# meshes take 5 GB or more to build are left out: with the rest of the editor they run the machine
# out of memory. The rows under 0.5 are props (logs, stumps, rocks), drawn farther.
GRASS = {
    "Grass": (
        ("shared", "wild_grass", 40.0, 3500, 5000, (1.2, 2.0)),
        ("shared", "grass_clumps", 24.0, 3500, 5000, (1.2, 2.0)),
        ("shared", "wheat_grass", 10.0, 3500, 5000),
        ("shared", "tundra_grass", 6.0, 3500, 5000),
        ("shared", "kikuyu_grass", 8.0, 3500, 5000),
        ("shared", "tall_grass", 2.4, 4000, 6000),
        ("shared", "ribbon_grass", 1.6, 3000, 4500),
        ("shared", "clovers", 12.0, 2000, 3000),
        ("shared", "clover_varieties", 6.0, 2000, 3000),
        ("shared", "leafy_dandelions", 4.0, 2000, 3000),
        ("mountain", "tundra_boulder", 0.03, 12000, 15000),
    ),
    "Forest": (
        # Grass and dry tussocks first, ferns only here and there: the spruce photos show grassy
        # floors, and the user found v12's ferns too many (v13's 40 per 100 m^2 also crashed the GPU
        # on its first look; this stays near 31). leather_fern, the glossiest, is out. v16: fewer
        # yellow dry tussocks, more green clumps, and clover and dandelions for small yellow flowers
        # (the style shots' flowering carpet). v18: the floor read as moss felt with sparse tufts;
        # twice the grass and more flowers near the road, culled sooner. Instances go as density
        # times cull squared, so the layer draws a little fewer than v17's (about 670 vs 700).
        # v19: the ferns (1.5 to 2 m across, six to nine variants each) covered twice the floor
        # all the grass did, whose clumps are 20 to 60 cm: fewer ferns, and grass drawn 1.4 to
        # 2.4 times its size, so it reads as a carpet with the same instance count.
        ("shared", "fern", 1.2, 4000, 6000),
        ("shared", "lady_fern", 0.8, 4000, 6000),
        ("shared", "common_fern", 0.8, 4000, 6000),
        ("mountain", "alpine_bearberry", 2.0, 3000, 4500),
        ("desert", "dry_grass", 1.5, 3000, 4500),
        ("desert", "dead_shrubs", 0.3, 4000, 6000),
        ("shared", "grass_clumps", 20.0, 2000, 3000, (1.4, 2.4)),
        ("shared", "wild_grass", 14.0, 2000, 3000, (1.4, 2.4)),
        ("shared", "tall_grass", 2.0, 4000, 6000),
        ("shared", "clovers", 5.0, 1500, 2500, (1.0, 1.6)),
        ("shared", "leafy_dandelions", 2.5, 1500, 2500),
        # Big rounded boulders, as in the style shots.
        ("mountain", "tundra_boulder", 0.02, 12000, 15000),
        # The floor's debris: the woods in the reference photos are never bare between trunks.
        ("forest", "nordic_forest_ground_patch_root", 0.12, 9000, 12000),
        ("forest", "mossy_rocks", 0.1, 9000, 12000),
        ("forest", "nordic_forest_rock_small", 0.3, 6000, 8000),
        ("forest", "mossy_wood_stump", 0.05, 9000, 12000),
        ("forest", "fallen_log", 0.03, 12000, 15000),
        ("forest", "fallen_pine_tree", 0.015, 12000, 15000),
    ),
    # The soil of the cut banks: roots, stones, ferns and tufts, so a bank is never bare plaster.
    "Sand": (
        ("forest", "nordic_forest_ground_patch_root", 1.5, 6000, 8000),
        ("forest", "nordic_forest_rock_small", 2.0, 5000, 7000),
        ("forest", "mossy_rocks", 0.3, 8000, 11000),
        ("shared", "fern", 2.0, 4000, 6000),
        ("shared", "lady_fern", 1.0, 4000, 6000),
        ("shared", "grass_clumps", 12.0, 3000, 4500),
        ("shared", "wild_grass", 8.0, 3000, 4500),
        ("desert", "dry_grass", 4.0, 3000, 4500),
    ),
    # Talus under the crags.
    "Rock": (
        ("forest", "nordic_forest_rock_small", 0.6, 8000, 11000),
        ("forest", "nordic_forest_cluster_rock_medium", 0.15, 12000, 15000),
        ("mountain", "tundra_boulder", 0.08, 12000, 15000),
        ("forest", "mossy_rocks", 0.15, 9000, 12000),
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
        # faces more, or a top-down sample streaks down a cut bank. 0 up to 32 degrees, 1 past 50.
        up = g.node(unreal.MaterialExpressionVertexNormalWS)
        self.steep = g.saturate(g.mul(g.add(g.mask(up, "b"), -0.85), -4.5))

        # Triplanar: each surface is sampled on the XZ and YZ planes too, weighted by the normal's
        # components to the fourth. One side plane picked by a hard switch, faded in from 32
        # degrees, stretched the soil into smears on diagonal and middling slopes (v16, QA).
        def w(channel):
            a = g.link(g.mask(up, channel), g.node(unreal.MaterialExpressionAbs))
            a = g.mul(a, a)
            return g.mul(a, a)

        wx, wy, wz = w("r"), w("g"), w("b")
        total = g.add(g.add(wx, wy), wz)
        self.w_yz = g.op(unreal.MaterialExpressionDivide, wx, total)
        self.w_xz = g.op(unreal.MaterialExpressionDivide, wy, total)
        self.w_top = g.op(unreal.MaterialExpressionDivide, wz, total)
        self.xz = g.op(unreal.MaterialExpressionAppendVector, g.mask(world, "r"), self.height)
        self.yz = g.op(unreal.MaterialExpressionAppendVector, g.mask(world, "g"), self.height)

    def surface(self, biome, slug, tint, label):
        g = self.g
        tile = _tile_cm(biome, slug)
        d, n, m = (_texture(biome, slug, k) for k in ("D", "N", "M"))
        near_uv, far_uv = g.world_uv(tile), g.world_uv(tile * FAR, (0.37, 0.61))
        near_d = g.mask(g.sample(d, near_uv), "rgb")
        far_d = g.mask(g.sample(d, far_uv), "rgb")
        near_n = g.mask(g.sample(n, near_uv, "normal"), "rgb")
        far_n = g.mask(g.sample(n, far_uv, "normal"), "rgb")
        xz_uv, yz_uv = g.mul(self.xz, 1.0 / tile), g.mul(self.yz, 1.0 / tile)

        def blend(top, xz, yz):
            return g.add(g.add(g.mul(top, self.w_top), g.mul(xz, self.w_xz)), g.mul(yz, self.w_yz))

        masks = g.sample(m, near_uv, "masks")
        top = g.lerp(near_d, far_d, g.mul(self.far, 0.7))
        side_d = blend(top, g.mask(g.sample(d, xz_uv), "rgb"), g.mask(g.sample(d, yz_uv), "rgb"))
        colour = g.mul(side_d, g.param(f"{label} tint", tint))
        # "<label> grey", 0 to 1, takes the scan's own hue out: a tint can darken warm gravel but
        # not make it grey.
        luma = g.op(unreal.MaterialExpressionDotProduct, colour, g.const((0.3, 0.59, 0.11)))
        colour = g.lerp(colour, luma, g.param(f"{label} grey", 0.0))
        # Steep ground takes its relief from the side projection, as its colour does: the top
        # projection's normal, stretched there, made the banks read as smooth putty.
        normal = blend(
            g.lerp(near_n, far_n, self.far),
            g.mask(g.sample(n, xz_uv, "normal"), "rgb"),
            g.mask(g.sample(n, yz_uv, "normal"), "rgb"),
        )
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
        else:
            base["rough"] = g.op(unreal.MaterialExpressionMax, base["rough"], g.const(ROUGH_FLOOR))
        if name in STEEP:
            shade = g.lerp(
                g.const((1.0, 1.0, 1.0)), g.param(f"{name} steep", STEEP[name]), self.steep
            )
            base["colour"] = g.mul(base["colour"], shade)
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
            g.mul(outputs["colour"], self.macro), outputs["normal"], outputs["rough"], specular=0.3
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


PLANT_MASTER = f"{MS}/Materials/M_MS_Plant"
PROP_MASTER = f"{MS}/Materials/M_MS_Prop"
# Set on every plant MI by each ground build (try values with `look --plant-param`). The scans'
# own translucency tints made the ferns glow lime where the sun came through them.
# Specular 0.4 and the scans' low roughness made fern fronds read blue-white and metallic against
# the sky (QA, v12).
PLANT_LOOK = {
    "Tint": (0.6, 0.62, 0.42),
    "TranslucencyTint": (0.2, 0.24, 0.12),
    "Grey": 0.12,
    "Specular": 0.15,
    "Rough floor": 0.6,
}
# Props' MIs (M_MS_Prop). The tundra boulder as scanned is pale tundra stone and read near white
# in the woods (v16), and still pale at 0.42 (v17): darker, with a moss cap (see _prop_master).
# Per plant, after PLANT_LOOK: the grass scans are near black as scanned (grass_clumps averages
# 0.14 sRGB, wild_grass 0.2) and read as dark tufts on a paler floor (v18). Brighter, yellow-green.
PLANT_SLUG_LOOK = {
    "grass_clumps": {"Tint": (3.2, 4.0, 1.6)},
    "wild_grass": {"Tint": (2.0, 2.5, 1.2)},
}
PROP_LOOK = {
    "tundra_boulder": {"Tint": (0.3, 0.29, 0.26), "Moss amount": 1.0},
}


def _master_texture(g, param, kind, default):
    """A texture parameter node for a Megascans master (the MIs set D, N, M and T by name)."""
    node = g.node(unreal.MaterialExpressionTextureSampleParameter2D, x=-900)
    node.set_editor_property("parameter_name", param)
    node.set_editor_property(
        "sampler_type",
        {
            "colour": unreal.MaterialSamplerType.SAMPLERTYPE_COLOR,
            "normal": unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL,
            "masks": unreal.MaterialSamplerType.SAMPLERTYPE_MASKS,
        }[kind],
    )
    node.set_editor_property("texture", unreal.load_asset(default))
    return node


def _prop_master():
    """M_MS_Prop rebuilt in place, as megascans.py builds it plus a moss cap: faces turned up
    are blended toward "Moss" (times the scan's own brightness, so it keeps the stone's grain),
    by "Moss amount" (0 by default, so only the props PROP_LOOK names grow it). The tundra
    boulder read pale and bare among the woods (v16, v17); the style shots' boulders are capped
    with moss."""
    folder, name = PROP_MASTER.rsplit("/", 1)
    g = _graph(name, folder)
    d = _master_texture(g, "D", "colour", "/Engine/EngineResources/DefaultTexture")
    n = _master_texture(g, "N", "normal", "/Engine/EngineMaterials/DefaultNormal")
    m = _master_texture(g, "M", "masks", f"{MS}/shared/fern/T_fern_M")
    raw = g.mask(d, "rgb")
    colour = g.mul(raw, g.mask(g.param("Tint", (1.0, 1.0, 1.0)), "rgb"))
    luma = g.op(unreal.MaterialExpressionDotProduct, raw, g.const((0.3, 0.59, 0.11)))
    up = g.mask(g.node(unreal.MaterialExpressionVertexNormalWS), "b")
    # Up from about 40 degrees off vertical, its edge broken up by the stone's own light and
    # dark (luma about 0.1 to 0.2 linear on these scans).
    edge = g.add(g.add(up, g.mul(luma, 2.0)), g.mul(g.param("Moss start", 0.75), -1.0))
    moss = g.mul(g.saturate(g.mul(edge, 5.0)), g.param("Moss amount", 0.0))
    moss_colour = g.mul(g.mask(g.param("Moss", (0.12, 0.2, 0.04)), "rgb"), g.mul(luma, 4.0))
    colour = g.lerp(colour, moss_colour, moss)
    rough = g.saturate(g.mul(g.mask(m, "g"), g.param("Roughness", 1.0)))
    rough = g.lerp(rough, g.const(1.0), moss)
    g.output(colour, g.mask(n, "rgb"), rough, ao=g.mask(m, "r"))
    material = g.material
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    g.finish()
    log("rebuilt M_MS_Prop with a moss cap")


def _plant_master():
    """M_MS_Plant rebuilt in place (its instances keep their parent), as megascans.py builds it
    but varied per instance: brightness by "Variation", and about one plant in "Dry share" turned
    to "Dry tint". QA found the ferns one uniform saturated green. Rerunning megascans.py's assign
    stage puts back the plain master; run rebuild-ground after it."""
    folder, name = PLANT_MASTER.rsplit("/", 1)
    g = _graph(name, folder)
    d = _master_texture(g, "D", "colour", "/Engine/EngineResources/DefaultTexture")
    n = _master_texture(g, "N", "normal", "/Engine/EngineMaterials/DefaultNormal")
    # A real masks texture as the default: the engine's white square is sRGB, which a Masks
    # sampler refuses at compile.
    m = _master_texture(g, "M", "masks", f"{MS}/shared/fern/T_fern_M")
    t = _master_texture(g, "T", "colour", "/Engine/EngineResources/DefaultTexture")
    colour = g.mul(g.mask(d, "rgb"), g.mask(g.param("Tint", (1.0, 1.0, 1.0)), "rgb"))
    luma = g.op(unreal.MaterialExpressionDotProduct, colour, g.const((0.3, 0.59, 0.11)))
    colour = g.lerp(colour, luma, g.param("Grey", 0.12))
    rnd = g.node(unreal.MaterialExpressionPerInstanceRandom)
    bright = g.add(g.const(1.0), g.mul(g.add(rnd, -0.5), g.param("Variation", 0.7)))
    colour = g.mul(colour, bright)
    second = g.link(g.mul(rnd, 7.13), g.node(unreal.MaterialExpressionFrac))
    dry = g.saturate(g.mul(g.add(second, g.add(g.param("Dry share", 0.3), -1.0)), 20.0))
    colour = g.lerp(colour, g.mul(colour, g.param("Dry tint", (1.5, 1.1, 0.55))), dry)
    rough = g.saturate(g.mul(g.mask(m, "g"), g.param("Roughness", 1.0)))
    rough = g.op(unreal.MaterialExpressionMax, rough, g.param("Rough floor", 0.6))
    sss = g.mul(g.mask(t, "rgb"), g.mask(g.param("TranslucencyTint", (0.5, 0.6, 0.4)), "rgb"))
    g.output(
        colour,
        g.mask(n, "rgb"),
        rough,
        specular=g.param("Specular", 0.15),
        opacity_mask=g.mask(d, "a"),
        subsurface=sss,
        ao=g.mask(m, "r"),
    )
    material = g.material
    material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_MASKED)
    material.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE)
    material.set_editor_property("two_sided", True)
    material.set_editor_property("opacity_mask_clip_value", 0.33)
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    g.finish()
    log("rebuilt M_MS_Plant with per-instance variation")


def _keep_alpha_coverage(biome, slug):
    """Keeps a plant's cut-out (D's alpha) as full in its small mips as at full size. Averaged,
    the alpha falls under the material's 0.33 clip a few metres off, and ferns thin to slivers."""
    path = f"{MS}/{biome}/{slug}/T_{slug}_D"
    if not assets.does_asset_exist(path):
        return
    texture = unreal.load_asset(path)
    if texture.get_editor_property("do_scale_mips_for_alpha_coverage"):
        return
    texture.set_editor_property("alpha_coverage_thresholds", unreal.Vector4(0.0, 0.0, 0.0, 0.33))
    texture.set_editor_property("do_scale_mips_for_alpha_coverage", True)
    assets.save_loaded_asset(texture, False)
    log(f"alpha coverage kept in {slug}'s mips")


def op_reimport_plants():
    """Re-imports every plant's D texture (colour, cut-out in alpha) from fab_prep in place,
    after `OC_FORCE=1 OC_ONLY=... prep_megascans.py` has redone it. Meshes and materials keep
    their references, so nothing else needs rebuilding."""
    with open(f"{PREP}/index.json") as f:
        plants = [r for r in json.load(f) if r["kind"] == "plant" and r["opacity"]]
    for record in plants:
        biome, slug = record["biome"], record["slug"]
        task = unreal.AssetImportTask()
        task.filename = f"{PREP}/{biome}/{slug}/D.png"
        task.destination_path = f"{MS}/{biome}/{slug}"
        task.destination_name = f"T_{slug}_D"
        task.replace_existing = True
        task.automated = True
        task.save = False
        unreal.AssetToolsHelpers.get_asset_tools().import_asset_tasks([task])
        texture = unreal.load_asset(f"{MS}/{biome}/{slug}/T_{slug}_D")
        texture.set_editor_property("srgb", True)
        texture.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_DEFAULT
        )
        texture.set_editor_property("do_scale_mips_for_alpha_coverage", False)
        assets.save_loaded_asset(texture, False)
        _keep_alpha_coverage(biome, slug)
        log(f"reimported {slug}'s colour and cut-out")
        yield 5
    return {"notes": [f"{len(plants)} plants reimported"]}


def _grass_type(layer):
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
        _keep_alpha_coverage(biome, slug)
        for mesh in meshes:
            v = unreal.GrassVariety()
            v.set_editor_property("grass_mesh", mesh)
            v.set_editor_property("grass_density", unreal.PerPlatformFloat(default=density))
            v.set_editor_property("start_cull_distance", unreal.PerPlatformInt(default=cull_start))
            v.set_editor_property("end_cull_distance", unreal.PerPlatformInt(default=cull_end))
            v.set_editor_property("scaling", unreal.GrassScaling.UNIFORM)
            v.set_editor_property("scale_x", unreal.FloatInterval(*(scale or [(0.6, 1.4)])[0]))
            v.set_editor_property("random_rotation", True)
            v.set_editor_property("align_to_surface", True)
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


STATIC_TREES = f"{FOLDER}/Trees"


def op_probe(path):
    """Runs a read-only inspection script (in out/qa) with `unreal`, `log`, `lib` and a `notes`
    list; returns the notes. For looking at assets, never for changing them."""
    scope = {"unreal": unreal, "log": log, "lib": lib, "notes": []}
    with open(path) as f:
        exec(compile(f.read(), path, "exec"), scope)
    yield
    return {"notes": scope["notes"]}


PART_TRIANGLES = 2500  # cap per assembly part (twig or branch) when baking static trees
_NUM = r"(-?[0-9.]+(?:e-?[0-9]+)?)"


def _xyz(text, key, default, keys="XYZ"):
    """A struct field of an export_text, as floats; fields at their default are left out."""
    m = re.search(key + r"=\(([^()]*)\)", text)
    found = dict(re.findall(r"([XYZW])=" + _NUM, m.group(1))) if m else {}
    return [float(found.get(k, d)) for k, d in zip(keys, default, strict=True)]


def _assembly(sk):
    """A Nanite-assembly skeletal mesh's parts [(mesh path, material remap)] and nodes [(part,
    transform)]. The nodes aren't exposed to Python, so they are read from the export text."""
    data = sk.get_editor_property("nanite_settings").get_editor_property("nanite_assembly_data")
    text = data.export_text()
    head, _, body = text.partition("Nodes=(")
    parts = [
        (path, [int(i) for i in remap.split(",") if i])
        for path, remap in re.findall(
            r'MeshObjectPath="([^"]+)"(?:,MaterialRemap=\(([0-9,]*)\))?', head
        )
    ]
    nodes = []
    for chunk in body.split("(PartIndex=")[1:]:
        if "TransformSpace=BoneRelative" in chunk:
            raise RuntimeError("bone-relative assembly nodes aren't handled")
        part = int(re.match(r"[0-9]+", chunk).group(0))
        t = chunk.split("BoneInfluences=")[0]
        x, y, z, w = _xyz(t, "Rotation", (0, 0, 0, 1), "XYZW")
        transform = unreal.Transform()
        transform.set_editor_property("rotation", unreal.Quat(x, y, z, w))
        transform.set_editor_property(
            "translation", unreal.Vector(*_xyz(t, "Translation", (0, 0, 0)))
        )
        transform.set_editor_property("scale3d", unreal.Vector(*_xyz(t, "Scale3D", (1, 1, 1))))
        nodes.append((part, transform))
    return parts, nodes


def _bake_assembly(sk, dm):
    """Appends every assembly node's part mesh (the twigs and their leaves) to dm, in mesh space,
    with its material slots remapped onto the tree's. Returns the node count."""
    au = unreal.GeometryScript_AssetUtils
    edit = unreal.GeometryScript_MeshEdits
    mat = unreal.GeometryScript_Materials
    parts, nodes = _assembly(sk)
    meshes = []
    for path, remap in parts:
        part_sk = unreal.load_asset(path)
        if part_sk is None:
            raise RuntimeError(f"no assembly part {path}")
        pm = unreal.DynamicMesh()
        if isinstance(part_sk, unreal.SkeletalMesh):
            pm, outcome = au.copy_mesh_from_skeletal_mesh(
                part_sk,
                pm,
                unreal.GeometryScriptCopyMeshFromAssetOptions(),
                unreal.GeometryScriptMeshReadLOD(),
            )
        else:
            pm, outcome = au.copy_mesh_from_static_mesh(
                part_sk,
                pm,
                unreal.GeometryScriptCopyMeshFromAssetOptions(),
                unreal.GeometryScriptMeshReadLOD(),
            )
        if outcome != unreal.GeometryScriptOutcomePins.SUCCESS:
            raise RuntimeError(f"copying assembly part {path} failed")
        # Each part is placed hundreds of times: uncapped, the spruce's dead branch alone (219k
        # triangles, 208 times) makes 45M and the Nanite build gets the editor killed for memory.
        if pm.get_triangle_count() > PART_TRIANGLES:
            unreal.GeometryScript_MeshSimplification.apply_simplify_to_triangle_count(
                pm, PART_TRIANGLES, unreal.GeometryScriptSimplifyMeshOptions()
            )
        # Through a high offset, so a slot isn't remapped twice.
        for slot in range(len(remap)):
            mat.remap_material_i_ds(pm, slot, 1000 + slot)
        for slot, target in enumerate(remap):
            mat.remap_material_i_ds(pm, 1000 + slot, target)
        meshes.append(pm)
    for index, pm in enumerate(meshes):
        placed = [t for part, t in nodes if part == index]
        if placed:
            edit.append_mesh_transformed(dm, pm, placed, unreal.Transform())
    return len(nodes)


def op_static_trees():
    """Static copies of the PVE trees (SM_spruce, SM_beech in /Game/Climb/Trees), Nanite with
    area preservation, so the climb can instance tens of thousands of them: the skeletal ones
    overflow the GPU's skinning buffer. Run on a fresh editor (oc.py static-trees restarts)."""
    cw = _old_build()

    au, na = unreal.GeometryScript_AssetUtils, unreal.GeometryScript_NewAssetUtils
    notes = []
    for kind, path in cw.PVE_TREES.items():
        sk = unreal.load_asset(path)
        if not isinstance(sk, unreal.SkeletalMesh):
            raise RuntimeError(f"{kind}: {path} is not a skeletal mesh")
        yield
        lod = unreal.GeometryScriptMeshReadLOD()
        dm = unreal.DynamicMesh()
        dm, outcome = au.copy_mesh_from_skeletal_mesh(
            sk, dm, unreal.GeometryScriptCopyMeshFromAssetOptions(), lod
        )
        if outcome != unreal.GeometryScriptOutcomePins.SUCCESS:
            raise RuntimeError(f"{kind}: copying {path} failed")
        yield
        twigs = _bake_assembly(sk, dm)
        log(f"{kind}: {twigs} twigs and branches baked in")
        mats, _index, slots, outcome = au.get_lod_material_list_from_skeletal_mesh(sk, lod)
        yield
        target = f"{STATIC_TREES}/SM_{kind}"
        if assets.does_asset_exist(target):
            assets.delete_asset(target)
        opts = unreal.GeometryScriptCreateNewStaticMeshAssetOptions()
        opts.set_editor_property("enable_nanite", True)
        opts.set_editor_property("enable_collision", False)
        nanite = opts.get_editor_property("nanite_settings")
        nanite.set_editor_property("enabled", True)
        try:
            nanite.set_editor_property(
                "shape_preservation", unreal.NaniteShapePreservation.PRESERVE_AREA
            )
        except Exception as e:  # older engines: no area preservation
            log(f"no shape_preservation: {e}")
        opts.set_editor_property("nanite_settings", nanite)
        sm, outcome = na.create_new_static_mesh_asset_from_mesh(dm, target, opts)
        if outcome != unreal.GeometryScriptOutcomePins.SUCCESS or sm is None:
            raise RuntimeError(f"{kind}: creating {target} failed")
        yield
        sm.set_editor_property(
            "static_materials",
            [unreal.StaticMaterial(m, slot) for m, slot in zip(mats, slots, strict=True)],
        )
        assets.save_loaded_asset(sm, False)
        tris = dm.get_triangle_count() if hasattr(dm, "get_triangle_count") else "?"
        notes.append(f"{kind}: {target}, {len(mats)} materials, {twigs} twigs, {tris} triangles")
        log(notes[-1])
        yield
    return {"notes": notes}


def _warm_grass_meshes():
    """Loads and builds GRASS's meshes one at a time, with frames between. Six million-triangle
    prop scans building their Nanite data and textures at once, inside one frame, crashed the
    editor in Vulkan (9 Nov). Once built they come from the DDC and this is quick."""
    seen = set()
    for table in GRASS.values():
        for biome, slug, *_ in table:
            if slug in seen:
                continue
            seen.add(slug)
            for path in assets.list_assets(f"{MS}/{biome}/{slug}", False):
                mesh = unreal.load_asset(path)
                if isinstance(mesh, unreal.StaticMesh):
                    unreal.EditorStaticMeshLibrary.get_number_verts(mesh, 0)  # waits for it
                    yield 20
            yield 20


def op_rebuild_ground(rebuild_grass=False):
    """M_ClimbGround and MI_ClimbGround, rebuilt. Only on the empty map (see _guard_rebuild).
    Saved grass types are reused: building them takes over ten minutes, on one core."""
    _guard_rebuild()
    _plant_master()
    _prop_master()
    _plant_params(PLANT_LOOK)
    _plant_params(PLANT_SLUG_LOOK)
    _prop_params(PROP_LOOK)
    _tree_look()
    yield 20
    yield from _warm_grass_meshes()
    grass = {}
    for layer in GRASS:
        saved = unreal.load_asset(f"{FOLDER}/Grass/GT_{layer}")
        grass[layer] = saved if saved and not rebuild_grass else _grass_type(layer)
        yield 10
    # The instance goes first: while it refers to the material, the material can't be deleted,
    # and a reused material keeps its old Landscape Grass node, which fails the compile.
    # The material goes too: a reused one can keep a stale Landscape Grass node that
    # delete_all_material_expressions misses (after the crash of 9 Nov), and the compile fails.
    for name in (INSTANCE, MATERIAL):
        if assets.does_asset_exist(f"{FOLDER}/{name}"):
            assets.delete_asset(f"{FOLDER}/{name}")
    yield 5
    material = Ground().build(grass)
    yield 30
    mi = lib.fresh(
        INSTANCE,
        FOLDER,
        unreal.MaterialInstanceConstant,
        unreal.MaterialInstanceConstantFactoryNew(),
    )
    lib.edit.set_material_instance_parent(mi, material)
    lib.edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)
    _tune(LOOK_TINTS, LOOK_GREYS)
    return {
        "notes": [f"rebuilt {MATERIAL} and {INSTANCE}, with the look {LOOK_TINTS} {LOOK_GREYS}"]
    }


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
        for _biome, slug, base, start, end, *_ in table:
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


# The keys `look --post` takes, and what each sets on the open level.
POST_KEYS = {
    "exposure": "post volume: auto-exposure bias in EV",
    "ev": "post volume: exposure locked at this EV100 (about -4.5 for the 6 lux sun); 99 frees it",
    "contrast": "post volume: colour contrast, 1 neutral",
    "saturation": "post volume: colour saturation, 1 neutral",
    "vignette": "post volume: vignette intensity",
    "fog_start": "height fog: metres from the camera before it starts",
    "volumetric": "height fog: volumetric fog (light shafts through the canopy), 0 or 1",
    "shafts": "sun: volumetric scattering intensity, 1 neutral",
    "sky": "sky light: intensity, 1 neutral",
    "sky_warm": "sky light: warmth 0..1; 0 is white, 1 is a warm (1, 0.8, 0.6) fill",
    "temp": "post volume: white balance in kelvin, 6500 neutral; higher reads warmer",
    "sun": "sun: intensity in lux",
    "contact": "sun: contact shadow length, screen fraction (0.05 grounds posts)",
    "clouds": "volumetric clouds: Cloud_GlobalCoverage on MI_ClimbClouds",
    "cloud_density": "volumetric clouds: Cloud_GlobalDensity on MI_ClimbClouds",
    "sky_lum": "sky atmosphere: sky luminance factor, 1 neutral; lower is a deeper, moodier sky",
}
CLOUDS = f"{FOLDER}/MI_ClimbClouds"
POST_LABEL = "OC Post"


def _level_actors(cls):
    return [a for a in unreal.EditorActorSubsystem().get_all_level_actors() if isinstance(a, cls)]


def _post(post):
    """The light and grade of the open level: an unbound post-process volume (made once, labelled
    POST_LABEL), the height fog's start and volumetric fog, and the sky light's and sun's
    intensity. Nothing compiles."""
    bad = [k for k in post if k not in POST_KEYS]
    if bad:
        raise RuntimeError(f"unknown --post {bad}; use {sorted(POST_KEYS)}")
    keys = ("exposure", "ev", "contrast", "saturation", "vignette", "temp")
    grade = {k: post[k] for k in keys if k in post}
    if grade:
        found = [
            a for a in _level_actors(unreal.PostProcessVolume) if a.get_actor_label() == POST_LABEL
        ]
        if found:
            ppv = found[0]
        else:
            ppv = unreal.EditorActorSubsystem().spawn_actor_from_class(
                unreal.PostProcessVolume, unreal.Vector(0, 0, 0)
            )
            ppv.set_actor_label(POST_LABEL)
        ppv.set_editor_property("unbound", True)
        st = ppv.get_editor_property("settings")
        if "exposure" in grade:
            st.set_editor_property("override_auto_exposure_bias", True)
            st.set_editor_property("auto_exposure_bias", float(grade["exposure"]))
        # Auto exposure undid every albedo change: darker soil came out as bright (v13).
        if "ev" in grade:
            locked = float(grade["ev"]) != 99
            for prop, free in (
                ("auto_exposure_min_brightness", -10.0),
                ("auto_exposure_max_brightness", 20.0),
            ):
                st.set_editor_property(f"override_{prop}", locked)
                st.set_editor_property(prop, float(grade["ev"]) if locked else free)
        for key, prop in (("contrast", "color_contrast"), ("saturation", "color_saturation")):
            if key in grade:
                v = float(grade[key])
                st.set_editor_property(f"override_{prop}", True)
                st.set_editor_property(prop, unreal.Vector4(v, v, v, 1.0))
        if "vignette" in grade:
            st.set_editor_property("override_vignette_intensity", True)
            st.set_editor_property("vignette_intensity", float(grade["vignette"]))
        # The shaded floor read cold blue-grey under the sky (v15): a warmer balance.
        if "temp" in grade:
            st.set_editor_property("override_white_temp", True)
            st.set_editor_property("white_temp", float(grade["temp"]))
        ppv.set_editor_property("settings", st)
    if "fog_start" in post or "volumetric" in post:
        for fog in _level_actors(unreal.ExponentialHeightFog):
            comp = fog.get_component_by_class(unreal.ExponentialHeightFogComponent)
            if "fog_start" in post:
                comp.set_editor_property("start_distance", float(post["fog_start"]) * 100)
            if "volumetric" in post:
                comp.set_editor_property("enable_volumetric_fog", bool(post["volumetric"]))
    if "sky" in post or "sky_warm" in post:
        for sky in _level_actors(unreal.SkyLight):
            if "sky" in post:
                sky.light_component.set_editor_property("intensity", float(post["sky"]))
            if "sky_warm" in post:
                w = min(max(float(post["sky_warm"]), 0.0), 1.0)
                # Keywords: positional unreal.Color is BGRA, which turned this fill blue.
                colour = unreal.Color(
                    r=255, g=int(255 * (1 - 0.2 * w)), b=int(255 * (1 - 0.4 * w)), a=255
                )
                sky.light_component.set_editor_property("light_color", colour)
    if {"sun", "shafts", "contact"} & set(post):
        for sun in _level_actors(unreal.DirectionalLight):
            comp = sun.light_component
            if "sun" in post:
                comp.set_editor_property("intensity", float(post["sun"]))
            if "shafts" in post:
                comp.set_editor_property("volumetric_scattering_intensity", float(post["shafts"]))
            if "contact" in post:
                comp.set_editor_property("contact_shadow_length", float(post["contact"]))
    # The template's sky was clear and pale white (QA, v17): clouds and a deeper sky. The clouds'
    # material is the engine's; ours is an instance of it, made once.
    if "clouds" in post or "cloud_density" in post:
        for cloud in _level_actors(unreal.VolumetricCloud):
            comp = cloud.get_component_by_class(unreal.VolumetricCloudComponent)
            mi = unreal.load_asset(CLOUDS) if assets.does_asset_exist(CLOUDS) else None
            if mi is None:
                folder, name = CLOUDS.rsplit("/", 1)
                mi = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
                    name,
                    folder,
                    unreal.MaterialInstanceConstant,
                    unreal.MaterialInstanceConstantFactoryNew(),
                )
                lib.edit.set_material_instance_parent(mi, comp.get_editor_property("material"))
            for key, name in (
                ("clouds", "Cloud_GlobalCoverage"),
                ("cloud_density", "Cloud_GlobalDensity"),
            ):
                if key in post:
                    lib.edit.set_material_instance_scalar_parameter_value(
                        mi, name, float(post[key])
                    )
            assets.save_loaded_asset(mi, False)
            comp.set_editor_property("material", mi)
    if "sky_lum" in post:
        for atmo in _level_actors(unreal.SkyAtmosphere):
            comp = atmo.get_component_by_class(unreal.SkyAtmosphereComponent)
            v = float(post["sky_lum"])
            comp.set_editor_property("sky_luminance_factor", unreal.LinearColor(v, v, v, 1.0))
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    log(f"post {post}")


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
    # The bench from the path, 7 m in front of it and a little to one side.
    if os.path.exists(f"{SRC}/props.json"):
        with open(f"{SRC}/props.json") as f:
            bench = [p for p in json.load(f) if p[0] == "bench"]
        if bench:
            _, bx, by, bz, yaw = bench[0]
            r = math.radians(yaw + 20)
            eye = (bx / 100 + 7 * math.cos(r), by / 100 + 7 * math.sin(r), bz / 100 + 1.6)
            out.append(("bench", eye, (bx / 100, by / 100, bz / 100 + 0.5)))
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
    # The default 1000 MB pool can't hold the scanned ground and plant textures at full mips: the
    # ground smeared, and the plants' opacity masks, averaged at low mips, cut the fern fronds down
    # to sprigs. Stills get a bigger pool and every visible texture fully loaded.
    for command in STILL_CVARS:
        unreal.SystemLibrary.execute_console_command(None, command)
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
        yield STILL_SETTLE  # grass, textures and shadows stream in around the new view
        unreal.AutomationLibrary.take_high_res_screenshot(*STILL_SIZE, path)
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


def _tree_components():
    found = []
    for actor in unreal.get_editor_subsystem(unreal.EditorActorSubsystem).get_all_level_actors():
        for c in actor.get_components_by_class(unreal.HierarchicalInstancedStaticMeshComponent):
            mesh = c.get_editor_property("static_mesh")
            if mesh and mesh.get_path_name().startswith(STATIC_TREES):
                found.append(c)
    return found


def op_fps(quality=0, frames=240, shots=None, tree_cull=None, tree_shadows=None, tree_wpo=None):
    """Frame times at each still's view, at a scalability level (0 Low .. 3 Epic): the editor's
    whole frame, so a pessimistic stand-in for the game's. Vsync and the frame cap off."""
    yield from _ensure_level()
    for command in (f"scalability {int(quality)}", "r.VSync 0", "t.MaxFPS 0"):
        unreal.SystemLibrary.execute_console_command(None, command)
    # Trees for this measurement only (restored after): a draw distance in m (0 hides them),
    # and whether they cast shadows.
    trees = _tree_components()
    saved = [(c.instance_end_cull_distance, c.cast_shadow) for c in trees]
    for c in trees:
        if tree_cull is not None:
            c.set_visibility(tree_cull > 0)
            c.set_cull_distances(0, max(1, int(tree_cull * 100)))
        if tree_shadows is not None:
            c.set_cast_shadow(bool(tree_shadows))
        if tree_wpo is not None:
            c.set_evaluate_world_position_offset(bool(tree_wpo))
    editor = unreal.UnrealEditorSubsystem()
    yield 60
    notes = [f"trees: {len(trees)}, cull {tree_cull} m, shadows {tree_shadows}, wpo {tree_wpo}"]
    for name, where, target in cameras(shots):
        eye = unreal.Vector(*where)
        editor.set_level_viewport_camera_info(
            eye, unreal.MathLibrary.find_look_at_rotation(eye, unreal.Vector(*target))
        )
        yield 150  # streaming settles
        times = []
        last = time.perf_counter()
        for _ in range(int(frames)):
            yield 1
            now = time.perf_counter()
            times.append((now - last) * 1000.0)
            last = now
        times.sort()
        mean = sum(times) / len(times)
        p95 = times[int(len(times) * 0.95)]
        notes.append(f"{name}: {1000.0 / mean:.1f} fps (mean {mean:.1f} ms, p95 {p95:.1f} ms)")
        log(notes[-1])
    unreal.SystemLibrary.execute_console_command(None, "scalability 3")
    for c, (cull, shadow) in zip(trees, saved, strict=True):
        c.set_visibility(True)
        c.set_cull_distances(0, cull)
        c.set_cast_shadow(shadow)
        if tree_wpo is not None:
            c.set_evaluate_world_position_offset(True)
    return {"notes": [f"scalability {quality}", *notes]}


def op_stills(shots=None, out=None):
    yield from _ensure_level()
    files = yield from _stills(shots, out or STILLS)
    return {"files": files}


def _set_params(mi, params):
    """Sets {name: value} on a material instance: a number is a scalar, a list a colour."""
    edit = unreal.MaterialEditingLibrary
    for name, value in params.items():
        if isinstance(value, (list, tuple)) and len(value) > 1:
            colour = unreal.LinearColor(*value, *([1.0] * (4 - len(value))))
            edit.set_material_instance_vector_parameter_value(mi, name, colour)
        else:
            value = value[0] if isinstance(value, (list, tuple)) else value
            edit.set_material_instance_scalar_parameter_value(mi, name, float(value))
    edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)


# Bark per static tree: {kind: {material slot: params}}, set on child instances of the PVE sample
# materials (/Game/Climb/Trees/MI_<kind>_<slot>) that the tree's slots then use, so the plugin's
# own assets stay untouched. The spruce bark read pale grey; the look reference has red-brown pines.
TREE_LOOK = {
    "spruce": {
        0: {"BaseColor Tint": (0.6, 0.4, 0.28)},
        1: {"BaseColor Tint": (0.6, 0.4, 0.28)},
    },
}


def _tree_look(look=None):
    """Applies TREE_LOOK (or `look`) to the static trees. Run with L_Hero closed."""
    look = TREE_LOOK if look is None else look
    for kind, slots in look.items():
        sm = unreal.load_asset(f"{STATIC_TREES}/SM_{kind}")
        if sm is None:
            log(f"no SM_{kind}: run `oc.py static-trees` first")
            continue
        mats = sm.get_editor_property("static_materials")
        changed = False
        for slot, params in slots.items():
            current = mats[slot].get_editor_property("material_interface")
            name = f"MI_{kind}_{slot}"
            path = f"{STATIC_TREES}/{name}"
            if assets.does_asset_exist(path):
                mi = unreal.load_asset(path)
            else:
                mi = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
                    name,
                    STATIC_TREES,
                    unreal.MaterialInstanceConstant,
                    unreal.MaterialInstanceConstantFactoryNew(),
                )
                lib.edit.set_material_instance_parent(mi, current)
            _set_params(mi, params)
            if current.get_path_name() != mi.get_path_name():
                # Setting the static_materials array did not stick (v14); set_material does.
                sm.set_material(slot, mi)
                changed = True
        if changed:
            assets.save_loaded_asset(sm, False)
        log(f"tree look {kind}: {slots}")


def _plant_params(params):
    """Sets the same parameters on every plant's MI (see _plant_master for the names), or, given
    {slug: params} (PLANT_SLUG_LOOK), each named plant's own."""
    with open(f"{PREP}/index.json") as f:
        plants = [r for r in json.load(f) if r["kind"] == "plant" and r["opacity"]]
    by_slug = all(isinstance(v, dict) for v in params.values())
    for r in plants:
        own = params.get(r["slug"]) if by_slug else params
        path = f"{MS}/{r['biome']}/{r['slug']}/MI_{r['slug']}"
        if own and assets.does_asset_exist(path):
            _set_params(unreal.load_asset(path), own)


def _prop_params(look):
    """Sets parameters on named props' MIs (M_MS_Prop: Tint, Roughness), e.g. PROP_LOOK."""
    with open(f"{PREP}/index.json") as f:
        props = {r["slug"]: r for r in json.load(f) if r["kind"] == "prop"}
    for slug, params in look.items():
        r = props[slug]
        path = f"{MS}/{r['biome']}/{slug}/MI_{slug}"
        if assets.does_asset_exist(path):
            _set_params(unreal.load_asset(path), params)


def _plant_tint(rgb):
    """Sets every plant's MI "Tint" (their colour's multiplier, 1 neutral) and saves them: the
    scanned ferns read lime green beside the reference woods."""
    with open(f"{PREP}/index.json") as f:
        plants = [r for r in json.load(f) if r["kind"] == "plant" and r["opacity"]]
    edit = unreal.MaterialEditingLibrary
    for r in plants:
        path = f"{MS}/{r['biome']}/{r['slug']}/MI_{r['slug']}"
        mi = unreal.load_asset(path) if assets.does_asset_exist(path) else None
        if mi:
            edit.set_material_instance_vector_parameter_value(
                mi, "Tint", unreal.LinearColor(*rgb, 1.0)
            )
            assets.save_loaded_asset(mi, False)
    return len(plants)


def op_look(
    tint=None,
    grey=None,
    grass=None,
    fog=None,
    post=None,
    plants=None,
    plant_params=None,
    ground_params=None,
    stills=True,
    shots=None,
    out=None,
):
    """Recolour through MI_ClimbGround's parameters (tint: {"Dirt": [r, g, b]}, grey: {"Dirt":
    0.8}), grass ([density, cull] factors on GRASS), fog (density), then stills. Nothing
    compiles."""
    if tint or grey:
        _tune({k: tuple(v) for k, v in (tint or {}).items()}, grey)
    if grass:
        _grass(*grass)
    if plants:
        _plant_tint(plants)
    if plant_params:
        _plant_params(plant_params)
    if ground_params:
        _set_params(unreal.load_asset(f"{FOLDER}/{INSTANCE}"), ground_params)
    yield
    notes = [f"tuned {tint} grey {grey}"] if tint or grey else []
    notes += [f"plants tinted {plants}"] if plants else []
    notes += [f"plant params {plant_params}"] if plant_params else []
    notes += [f"ground params {ground_params}"] if ground_params else []
    notes += [f"grass {grass}"] if grass else []
    if fog is not None:
        yield from _ensure_level()
        _fog(fog)
        notes.append(f"fog {fog}")
        yield 30
    if post:
        yield from _ensure_level()
        _post(post)
        notes.append(f"post {post}")
        yield 30
    result = {"notes": notes}
    if stills:
        yield from _ensure_level()
        result["files"] = yield from _stills(shots, out or STILLS)
    return result
