"""The climb's world, inside the editor: its ground material, the look (tints on the level's
ground instance, MI_ClimbGround on L_Hero), and stills from the editor viewport. Loaded afresh
for each job by oc_server.py; oc.py is the way to run any of it. Self-contained apart from
ue_lib's Graph helper.

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

# OC_SRC and OC_LEVEL point the tools at another build: the world's ride is
# OC_SRC=/workspace/ue/world_gen/ride OC_LEVEL=/Game/World/L_World (make_world_level.py).
SRC = os.environ.get("OC_SRC") or "/workspace/ue/world_gen/climb"
PREP = "/workspace/ue/fab_prep"
MS = "/Game/Megascans"
FOLDER = "/Game/Climb"
LEVEL = os.environ.get("OC_LEVEL") or f"{FOLDER}/L_Hero"


def _ground_names(level):
    """The level's own ground material: (folder, material, instance). L_Hero keeps
    /Game/Climb/M_ClimbGround; any other level gets M_<name>Ground beside it, so building or
    retinting one level's ground never touches another's (L_World shared L_Hero's until 11 Nov,
    and each one's rebuild undid the other's look). climb_world.py names it the same way."""
    folder, name = level.rsplit("/", 1)
    if level.lower() == f"{FOLDER}/L_Hero".lower():
        return FOLDER, "M_ClimbGround", "MI_ClimbGround"
    stem = name.removeprefix("L_")
    return folder, f"M_{stem}Ground", f"MI_{stem}Ground"


GROUND, MATERIAL, INSTANCE = _ground_names(LEVEL)
ROAD_INSTANCE = INSTANCE.removesuffix("Ground") + "Road"  # climb_world.road_instance's
STILLS = os.environ.get("OC_STILLS") or "/workspace/ue/out/climb_stills"
SHOTS = (0.08, 0.3, 0.5, 0.72, 0.95)  # fractions of the way up, for the road-level stills
STILL_SIZE = (1920, 1080)
# Frames to wait at each view: grass builds around the camera within a time budget, and at 120
# frames whole wooded stretches were still bare.
STILL_SETTLE = 480
# Stills wait this many frames at least, then until the landscape's grass instance count holds
# still for GRASS_STEADY checks 30 frames apart (at most STILL_SETTLE_MAX frames): a fixed 480
# often shot the banks, or a whole view, before their grass was built (v21-v23's "bare banks").
STILL_SETTLE_MIN = 240
STILL_SETTLE_MAX = 1500
GRASS_STEADY = 4
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
    # v20: forest_floor (litter with small plants), greyed toward brown by LOOK_GREYS. The
    # alpine_needles scan used here before is an opacity scan: 8% needles, the rest dilated
    # padding, which drew every bank, verge and the woods' floor as blurred streaks (v19 false
    # colour; its D's alpha averages 0.08).
    "Forest": ("forest", "forest_floor", (0.5, 0.46, 0.44)),
    "Rock": ("mountain", "granite_rocky_ground", (0.85, 0.85, 0.85)),
    "Snow": ("coast", "thai_wet_sand", (1.0, 1.0, 1.0)),
    # The climb has no desert: its "Desert" layer is the heath above the treeline (v35), a dull
    # olive moss floor that the heath's cushions (GRASS) grow out of.
    "Desert": ("forest", "mossy_forest_floor", (1.0, 1.0, 1.0)),
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
    # v30: a fifth item, the noise's (low, high) before it is clamped to 0..1: here about a
    # third of the floor is litter, a third grass, and the rest the fade between.
    "Forest": ("shared", "uncut_grass", "patches", 12.0, (-1.2, 2.4)),
    # Gravel and bare soil show through the heath in patches a few metres across.
    "Desert": ("shared", "gravel_on_soil", "patches", 5.0, (-2.6, 1.6)),
    # Litter and small plants over the banks' soil (alpine_needles was padding: see "Forest").
    "Sand": ("forest", "forest_floor", "patches", 4.0),
    "Grass": ("shared", "lawn_grass", "patches", 30.0),
    "Rock": ("forest", "rocky_forest_path", "patches", 12.0),
    "Dirt": ("forest", "forest_floor", "patches", 4.0),
}
# The tuned look (see `oc.py look`), set on every rebuilt ground instance.
# forest_floor (the Forest base, and the Sand and Dirt mixes) is olive as scanned: greyed and
# warmed toward brown litter, keeping a little of its green.
# Halved from v13 once exposure was locked (`--post ev=-4.5`): under auto exposure darker albedo
# came out just as bright.
LITTER = (0.38, 0.27, 0.13)
LOOK_TINTS = {
    "Rock": (0.45, 0.42, 0.37),
    # Warmer than neutral litter: under the blue sky the floor read blue-grey (v15 false colour).
    "Forest": LITTER,
    # Moss and grass: a deep green, not v13's pale lime. Grass mix (lawn_grass patches) had no
    # tint at all: it made the pale beech bank and the far hillside's leopard spots (v15).
    # v18: a little yellower, as the moss and lawn read teal felt under the sky (v17 crops).
    # v20: (0.55, 0.75, 0.3) read lime on the hairpin's slopes; this, greyed a little, didn't.
    # v24: the bench's and 72%'s mint carpet was this mix (false colour): deeper, yellower, and
    # no longer greyed, as grey turned it mint under the sky.
    # v30: a little greener, with the blades' "Flat colour" (PLANT_LOOK): move the two together.
    "Forest mix": (0.2, 0.34, 0.05),
    "Grass": (0.14, 0.23, 0.04),
    "Grass mix": (0.14, 0.22, 0.04),
    "Sand mix": (0.22, 0.2, 0.08),
    "Dirt mix": LITTER,
    # soil_mud as scanned read pale and dusty on the banks and the path's edge (QA, v12): dark,
    # damp earth as in the reference photos.
    # v20: warmer, as the style shots' brown dirt; the cooler soil read mauve under the sky.
    # v21: browner, with some blue: at R 1.7x G and no blue the banks read maroon under the sky.
    # v23: warmer again, less blue: v22's banks still read purple-brown in the shade (QA).
    # v24: more green in the brown, as red-brown soil reads plum under the blue sky light.
    # v28: darker again, so the gaps between the bank's tufts read as shadow, not bare mud (QA,
    # v27; tried with look first). Its litter mix too.
    "Sand": (0.17, 0.15, 0.06),
    "Dirt": (0.2, 0.16, 0.07),
    # v35: the heath's floor, near HEATH_LOOK's root colour, and its gravel.
    "Desert": (0.26, 0.25, 0.1),
    "Desert mix": (0.3, 0.27, 0.2),
}
LOOK_GREYS = {
    "Grass": 0.0,
    "Grass mix": 0.0,
    "Rock": 0.5,
    "Rock mix": 0.3,
    "Forest mix": 0.0,
    "Forest": 0.25,
    "Dirt mix": 0.25,
    "Sand mix": 0.25,
    "Desert": 0.2,
}
ROUGHNESS = {"Snow": 0.55}
# Every other layer is at least this rough: the scans' low roughness made the soil glitter.
ROUGH_FLOOR = 0.75
# Steep ground on these layers is shaded toward darker, browner soil ("<layer> steep" parameter):
# the cut banks read pale beige against the reference photos' dark earth.
# v21: (0.7, 0.6, 0.48), a darker, more neutral shade: (0.8, 0.6, 0.38) turned every bank maroon.
STEEP = {
    "Forest": (0.7, 0.6, 0.48),
    "Sand": (0.7, 0.6, 0.48),
    "Dirt": (0.7, 0.6, 0.48),
}
# Snow keeps wet sand's relief but not its colour: shaded from the first colour in its hollows to
# the second on its crests.
FLAT = {"Snow": ((0.62, 0.68, 0.78), (0.86, 0.89, 0.93))}
# The colour a layer's grass reads as from past its blades' cull (Ground.layer), faded in from
# "<layer> canopy start" cm over "<layer> canopy span": about M_Blade's root and tip colours,
# averaged as a far meadow shows them. Try values with `look --ground-param`.
CANOPY = {"Grass": (0.085, 0.16, 0.03)}
# Grass per layer: (biome, slug, instances per 100 m^2, cull start, cull end in cm[, (min, max)
# scale, 0.6 to 1.4 if left out]). The density is per mesh: a scan with six variants draws six
# times it (fern at 2.0 was 12 per 100 m^2), and `look --grass` multiplies it again. Scans whose
# meshes take 5 GB or more to build are left out: with the rest of the editor they run the machine
# out of memory. The rows under 0.5 are props (logs, stumps, rocks), drawn farther.
GRASS = {
    "Grass": (
        # v32: the meadow is the climb's own blade grass (make_grass.py), knee high with seed
        # stalks; the scans' cards read thin at any density (v12-v31).
        ("climb", "blade_short", 120.0, 3000, 5000, (0.8, 1.3)),
        ("climb", "blade_mid", 160.0, 4000, 7000, (0.8, 1.4)),
        ("climb", "blade_tall", 80.0, 4000, 7000, (0.7, 1.2)),
        ("shared", "wheat_grass", 6.0, 3500, 5000),
        ("shared", "tall_grass", 2.4, 4000, 6000),
        ("shared", "ribbon_grass", 1.6, 3000, 4500),
        ("shared", "clovers", 12.0, 2000, 3000),
        ("shared", "clover_varieties", 6.0, 2000, 3000),
        ("shared", "leafy_dandelions", 4.0, 2000, 3000),
        # 12 Nov: the Fab meadow flowers, sparse, for the colour a meadow has (densities are per
        # variant, 5 to 9 each): white, yellow, purple thyme, a few poppies and goldenrod.
        ("shared", "yellow_flower", 0.5, 2500, 4000),
        ("shared", "bermuda_buttercup", 0.4, 2500, 4000),
        ("shared", "white_everlasting", 0.3, 2500, 4000),
        ("shared", "white_windflower", 0.4, 2500, 4000),
        ("shared", "thyme", 0.8, 2000, 3000),
        ("shared", "ribwort_plantain", 0.6, 2500, 4000),
        ("shared", "field_poppy", 0.15, 3000, 4500),
        ("shared", "common_hemp_nettle", 0.15, 3000, 4500),
        ("shared", "goldenrods", 0.08, 3500, 5000),
        # 12 Nov Fab wave 2: low tufts that break up the blade carpet's even top.
        ("shared", "tufted_grass", 1.0, 3000, 4500),
        ("mountain", "tundra_boulder", 0.03, 12000, 15000),
    ),
    # The woods' floor is two tables (v30). "Forest" grows only on the layer's grass patches (its
    # MIXES surface), "Forest floor" on the litter between them: see Ground.build. Until v29 one
    # table covered both evenly, so blades stood on brown litter, nothing was bare, and ferns
    # were everywhere. Densities are per mesh variant, and the patches are under half the floor,
    # so this draws fewer instances than v29 did. Scans near their own size (clumps are 60 cm
    # across and 25 cm tall; v19-v29 drew them up to 2.8 times that, as sparse reeds).
    "Forest": (
        # v32: blade grass (make_grass.py) for volume, after the user's Folkvangr shot: no ground
        # showing, shin high in the woods. v31's three-times-denser scans closed the cover but
        # stayed short, flat cards. Low graphics may thin it (grass.DensityScale); tune for Epic.
        ("climb", "blade_short", 160.0, 2500, 4000, (0.8, 1.3)),
        ("climb", "blade_mid", 120.0, 3000, 5000, (0.8, 1.3)),
        ("climb", "blade_tall", 16.0, 3000, 5000, (0.7, 1.1)),
        ("shared", "grass_clumps", 20.0, 2000, 3000, (1.2, 2.0)),
        ("shared", "clovers", 26.0, 1500, 2500, (1.0, 1.6)),
        ("shared", "clover_varieties", 14.0, 1500, 2500, (1.0, 1.6)),
        ("shared", "leafy_dandelions", 15.0, 1500, 2500),
        # 12 Nov Fab: woodland herbs in the patches.
        ("forest", "wood_sorrel", 3.0, 2000, 3500),
        ("shared", "herb_robert", 1.0, 2000, 3500),
        ("forest", "lily_of_the_valley", 1.0, 2000, 3500),
        ("forest", "yellow_archangel", 0.3, 2000, 3500),
        ("mountain", "tundra_boulder", 0.03, 12000, 15000),
    ),
    "Forest floor": (
        # 12 Nov Fab: the low cover a broadleaf floor has, so the litter isn't bare: ivy, sorrel,
        # lingonberry, periwinkle and brambles, with moss mounds, mushrooms and cones.
        ("forest", "english_ivy", 1.5, 3000, 4500),
        ("forest", "wood_sorrel", 2.0, 2000, 3500),
        ("forest", "lingonberry", 1.5, 2500, 4000),
        ("shared", "lesser_periwinkle", 1.0, 2500, 4000),
        ("shared", "blackberry", 0.25, 4000, 6000),
        ("forest", "icelandic_moss_clusters", 0.3, 6000, 8000),
        ("forest", "nordic_forest_ledge_moss_small_02", 0.1, 6000, 8000),
        ("forest", "bolete_mushrooms", 0.4, 1500, 2500),
        ("forest", "puffball_mushrooms", 0.4, 1500, 2500),
        ("forest", "small_pine_cones", 1.0, 1500, 2500),
        ("shared", "fern", 1.6, 4000, 6000),
        ("shared", "lady_fern", 1.0, 4000, 6000),
        ("shared", "common_fern", 1.0, 4000, 6000),
        ("mountain", "alpine_bearberry", 2.0, 3000, 4500),
        ("desert", "dry_grass", 1.0, 3000, 4500),
        ("desert", "dead_shrubs", 0.3, 4000, 6000),
        # A few tufts through the litter, so the patches' edges aren't a hard line.
        ("shared", "grass_clumps", 5.0, 2000, 3000, (1.0, 1.6)),
        # Big rounded boulders, as in the style shots, and the floor's debris.
        ("mountain", "tundra_boulder", 0.05, 12000, 15000),
        ("forest", "nordic_forest_ground_patch_root", 0.2, 9000, 12000),
        ("forest", "mossy_rocks", 0.15, 9000, 12000),
        ("forest", "nordic_forest_rock_small", 0.5, 6000, 8000),
        ("forest", "mossy_wood_stump", 0.05, 9000, 12000),
        ("forest", "fallen_log", 0.05, 12000, 15000),
        ("forest", "fallen_pine_tree", 0.025, 12000, 15000),
        # 12 Nov Fab wave 2: leaf litter, bracken, twigs, stumps and a dead birch now and then.
        ("forest", "dead_leaves", 1.0, 2000, 3500),
        ("shared", "bracken_fern", 0.5, 4000, 6000),
        ("forest", "wooden_sticks_and_twigs", 0.3, 2000, 3500),
        ("forest", "scatter_mushroom_set", 0.2, 1500, 2500),
        ("forest", "lichen_stump", 0.03, 9000, 12000),
        ("forest", "pine_tree_stump", 0.03, 9000, 12000),
        ("forest", "mossy_rock_formation", 0.015, 12000, 15000),
        ("climb", "dead_birch_snag_01", 0.004, 15000, 20000),
    ),
    # The soil of the cut banks: roots, stones, ferns and tufts, so a bank is never bare plaster.
    "Sand": (
        # v20: more roots and stones, and bigger tufts: the banks read bare once their smeared
        # litter was fixed. v21: still bare (about 16 tufts on a 30 m^2 bank, QA): grass near the
        # forest floor's density, fewer yellow dry tussocks. v23: landscape grass is placed per
        # metre of map, not of slope, so a 50 degree cut 3 m tall gets the grass of 2 m of flat
        # ground: twice the grass again, drawn bigger, now that it grows upright (v22). Fewer root
        # patches: with their material filled (v21) they read as flat pink slabs on the banks.
        ("forest", "nordic_forest_ground_patch_root", 1.5, 6000, 8000),
        ("forest", "nordic_forest_rock_small", 3.0, 5000, 7000),
        ("forest", "mossy_rocks", 0.6, 8000, 11000),
        ("shared", "fern", 2.0, 4000, 6000),
        ("shared", "lady_fern", 1.0, 4000, 6000),
        # v33: nearly twice the blades: v32's banks carried separate tufts with soil between.
        ("climb", "blade_short", 220.0, 2500, 4000, (0.8, 1.3)),
        ("climb", "blade_mid", 140.0, 3000, 4500, (0.7, 1.2)),
        ("shared", "grass_clumps", 30.0, 3000, 4500, (1.2, 2.4)),
        ("desert", "dry_grass", 1.5, 3000, 4500),
        # v27: twice the clover, bigger, over the bare soil between the tufts (QA, v26).
        ("shared", "clovers", 10.0, 1500, 2500, (1.2, 2.0)),
        # v26: the spruce banks read as flat brown mud (QA, v25): the floor's low carpet here too.
    ),
    # The heath above the treeline (v35), after ref/climb/alpine_meadow_trail.jpg: a low mottled
    # cushion with wiry tussocks, a little green turf, bearberry, and slabs and boulders.
    "Desert": (
        ("climb", "heath_low", 240.0, 3000, 5000, (0.8, 1.4)),
        ("climb", "heath_tuft", 50.0, 3000, 5000, (0.8, 1.3)),
        ("climb", "blade_short", 40.0, 2500, 4000, (0.7, 1.1)),
        ("mountain", "alpine_bearberry", 3.0, 3000, 4500),
        ("shared", "tundra_grass", 3.0, 3000, 4500),
        ("mountain", "icelandic_rock_plates", 0.08, 12000, 15000),
        ("mountain", "tundra_boulder", 0.06, 12000, 15000),
        ("mountain", "limestone_rocks", 0.15, 9000, 12000),
        ("forest", "nordic_forest_rock_small", 0.8, 6000, 8000),
    ),
    # No "Dirt" row: the landscape under the road mesh is painted Dirt too, so v28's low verge
    # cover grew straight through the whole tarmac.
    # Talus under the crags.
    "Rock": (
        ("forest", "nordic_forest_rock_small", 0.6, 8000, 11000),
        ("forest", "nordic_forest_cluster_rock_medium", 0.15, 12000, 15000),
        ("mountain", "tundra_boulder", 0.08, 12000, 15000),
        ("forest", "mossy_rocks", 0.15, 9000, 12000),
        # 12 Nov Fab wave 2: the tundra's own shrubs and sedge, after the treeline reference
        # (here, not "Desert": the world's Desert table is Red Sands').
        ("mountain", "crowberry", 1.2, 3000, 4500),
        ("mountain", "dwarf_willow", 0.6, 3000, 4500),
        ("mountain", "downy_willow", 0.2, 4000, 6000),
        ("mountain", "cotton_grass", 0.6, 3000, 4500),
        ("mountain", "limestone_rubble", 0.1, 9000, 12000),
        ("mountain", "tundra_rocky_ground", 0.02, 12000, 15000),
        # 12 Nov Fab wave 2: scree and old snow below the crags.
        ("mountain", "volcanic_scree", 0.2, 9000, 12000),
        ("mountain", "rocky_snow_pile", 0.02, 12000, 15000),
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
        self.g = g = _graph(MATERIAL, GROUND)
        world = g.node(unreal.MaterialExpressionWorldPosition)
        camera = g.node(unreal.MaterialExpressionCameraPositionWS)
        distance = g.node(unreal.MaterialExpressionDistance)
        g.link(world, distance, "A")
        g.link(camera, distance, "B")
        # 0 within 15 m, 1 past 60 m.
        self.far = g.saturate(g.mul(g.add(distance, -1500.0), 1 / 4500))
        self.distance = distance
        self.height = g.mask(world, "b")
        self.macro = g.world_noise(0.0004, 0.82, 1.12, 4)
        # Over a few hundred metres the ground drifts between drier, warmer and lusher, cooler
        # ("macro hue" 0 turns it off without a rebuild), and darker and lighter, so a long
        # view isn't one even colour.
        drift = g.world_noise(0.000025, 0.0, 1.0, 3)
        hue = g.lerp(g.const((1.07, 1.0, 0.86)), g.const((0.9, 1.0, 1.0)), drift)
        hue = g.lerp(g.const((1.0, 1.0, 1.0)), hue, g.param("macro hue", 1.0))
        shade = g.world_noise(0.00006, 0.88, 1.08, 3)
        self.macro = g.mul(g.mul(self.macro, shade), hue)
        # Each layer's "patches" mix, 0 to 1, before the surfaces' heights break up its edge:
        # the layer's grass grows by it too (see build), so blades stand on the grass surface.
        self.patch = {}
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
            biome2, slug2, how, size, *span = MIXES[name]
            other = self.surface(biome2, slug2, (1.0, 1.0, 1.0), f"{name} mix")
            if how == "patches":
                low, high = span[0] if span else (-1.5, 2.0)
                alpha = g.world_noise(1.0 / (size * 100), low, high, 3)
                self.patch[name] = g.saturate(alpha)
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
        if name in CANOPY:
            # Past the blades' cull the layer takes the blades' own colour, mottled over a few
            # metres as a meadow is, so the far ground reads as grass and not as paint.
            mottle = g.world_noise(1.0 / 300, 0.7, 1.25, 2)
            canopy = g.mul(g.mask(g.param(f"{name} canopy", CANOPY[name]), "rgb"), mottle)
            start = g.param(f"{name} canopy start", 4000.0)
            fade = g.op(
                unreal.MaterialExpressionDivide,
                g.op(unreal.MaterialExpressionSubtract, self.distance, start),
                g.param(f"{name} canopy span", 3000.0),
            )
            fade = g.mul(g.saturate(fade), g.param(f"{name} canopy share", 1.0))
            base["colour"] = g.lerp(base["colour"], canopy, fade)
            base["rough"] = g.lerp(base["rough"], g.const(0.95), fade)
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
        g.output(colour, outputs["normal"], outputs["rough"], specular=0.3)
        # Its colour into the ground RVT too (written only where a level's volume covers it: the
        # world's), for the road's and path's edges to fade into.
        rvt = g.node(unreal.MaterialExpressionRuntimeVirtualTextureOutput)
        g.link(colour, rvt, "BaseColor")
        out = g.node(unreal.MaterialExpressionLandscapeGrassOutput)
        inputs = []
        for name, gt in grass_types.items():
            gi = unreal.GrassInput()
            gi.set_editor_property("name", name)
            gi.set_editor_property("grass_type", gt)
            inputs.append(gi)
        out.set_editor_property("grass_types", inputs)
        for name in grass_types:
            # "<layer> floor" grows where the layer's patches mix is absent, and then "<layer>"
            # itself only on the mix. v29 and before spread one table evenly over both, so
            # green blades stood on brown litter and nothing was ever bare.
            layer = name.removesuffix(" floor")
            weight = g.node(unreal.MaterialExpressionLandscapeLayerSample, parameter_name=layer)
            if name != layer:
                gap = g.link(self.patch[layer], g.node(unreal.MaterialExpressionOneMinus))
                weight = g.mul(weight, gap)
            elif f"{layer} floor" in grass_types:
                weight = g.mul(weight, self.patch[layer])
            g.link(weight, out, name)
        return g.finish()


LUMA = (0.3, 0.59, 0.11)
PLANT_MASTER = f"{MS}/Materials/M_MS_Plant"
PROP_MASTER = f"{MS}/Materials/M_MS_Prop"
# Set on every plant MI by each ground build (try values with `look --plant-param`). The scans'
# own translucency tints made the ferns glow lime where the sun came through them.
# Specular 0.4 and the scans' low roughness made fern fronds read blue-white and metallic against
# the sky (QA, v12).
PLANT_LOOK = {
    # v34: brighter for daylight: at (0.6, 0.62, 0.42) the ferns were near black in the shade.
    "Tint": (1.0, 1.1, 0.6),
    # v27: halved: the grass glowed lime, back-lit, under the dark spruce (QA, v26). v28: less
    # again, as the blade tips still glowed in the shade.
    # v34: up again with the high sun, from (0.06, 0.08, 0.05).
    "TranslucencyTint": (0.14, 0.2, 0.07),
    "Grey": 0.12,
    # v30: less per-plant brightness and fewer dry plants (0.7 and 0.3 made the carpet blotchy).
    # v30: the first render's blades were grey and dark against the ground (gain 4, shade 0.35).
    "Flat gain": 12.0,
    "Flat colour": (0.1, 0.2, 0.03),
    "Root shade": 0.5,
    "Variation": 0.3,
    "Dry share": 0.1,
    "Specular": 0.15,
    "Rough floor": 0.6,
}
# Props' MIs (M_MS_Prop). The tundra boulder as scanned is pale tundra stone and read near white
# in the woods (v16), and still pale at 0.42 (v17): darker, with a moss cap (see _prop_master).
# Per plant, after PLANT_LOOK: the grass scans are near black as scanned (grass_clumps averages
# 0.14 sRGB, wild_grass 0.2) and read as dark tufts on a paler floor (v18). Brighter, yellow-green.
PLANT_SLUG_LOOK = {
    # v21: less red and blue: at R/G 0.8 the tufts read yellow-olive against the style shots' green.
    # v27: darker and bluer: v26's carpet read lime and self-lit in the shade (QA).
    # v30: "Flat" takes most of each grass's colour from one green, the ground's (_plant_master).
    "grass_clumps": {"Tint": (1.5, 2.8, 1.1), "Flat": 0.7},
    "wild_grass": {"Tint": (1.0, 1.8, 0.75), "Flat": 0.7},
    # v26: untinted it read as a pale grey-green wash on the shaded floor (QA, v25).
    "kikuyu_grass": {"Tint": (1.0, 1.8, 0.75), "Flat": 0.7},
}
PROP_LOOK = {
    "tundra_boulder": {"Tint": (0.3, 0.29, 0.26), "Moss amount": 1.0},
    # v19: the pointed grey rocks by the bench and at 95% are these scans (0.34 sRGB, untinted).
    "nordic_forest_rock_small": {"Tint": (0.4, 0.38, 0.34), "Moss amount": 1.0},
    "nordic_forest_cluster_rock_medium": {"Tint": (0.4, 0.38, 0.34), "Moss amount": 1.0},
    # Named so a false-colour look (`--prop-param`) is undone by the next look or build.
    "mossy_rocks": {"Tint": (1.0, 1.0, 1.0)},
    "nordic_forest_ground_patch_root": {"Tint": (1.0, 1.0, 1.0)},
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


# The world's road distance map (make_world_level.road_distance): a plant whose base is within
# "Road clear" m of a road's centre is folded into its base in the vertex shader, so the grass
# grows to the tarmac's edge and never through it. The grass map has a sample (7 m) a texel, so
# the paint alone left 14..21 m of bare verge (12 Nov). Only for a build whose source has the
# map; elsewhere nothing is hidden. "Road clear" is the road's half width plus the plant's reach.
ROAD_DIST = f"{SRC}/road_dist.png"
ROAD_DIST_NEAR, ROAD_DIST_SIZE = 16.0, 4096  # m, texels: as make_world_level.py writes it
ROAD_REACH = {"blade": 0.35, "plant": 0.6, "prop": 1.5}


def _road_texture():
    """The source's road distance map, imported afresh (None without one): linear 8-bit RGB, no
    mips and never streamed, as the vertex shader reads its top mip."""
    if not os.path.exists(ROAD_DIST):
        return None
    folder = f"{FOLDER}/Road"
    tex = unreal.load_asset(f"{folder}/road_dist")
    if tex is None or not getattr(unreal, "_oc_road_dist", False):
        tex = next(a for a in lib.import_file(ROAD_DIST, folder) if isinstance(a, unreal.Texture))
        tex.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_VECTOR_DISPLACEMENTMAP
        )
        tex.set_editor_property("srgb", False)
        tex.set_editor_property("mip_gen_settings", unreal.TextureMipGenSettings.TMGS_NO_MIPMAPS)
        tex.set_editor_property("never_stream", True)
        tex.set_editor_property("filter", unreal.TextureFilter.TF_BILINEAR)
        assets.save_loaded_asset(tex, False)
        unreal._oc_road_dist = True  # once per editor session: each master reads the same one
    return tex


def _road_look(kind):
    """The road parameters' built values for `kind`, which every look sets again first, so a
    trial "Road clear" or "Road debug" lasts one look ({} without a road map)."""
    if not os.path.exists(ROAD_DIST):
        return {}
    with open(f"{SRC}/info.json") as f:
        clear = json.load(f)["road_half"] + ROAD_REACH[kind]
    debug = {"Road debug": 0.0, "Road debug sample": 0.0, "Road debug uv": 0.0}
    debug |= {"Road debug uv scale": 8.0}
    clear = {"Road clear": clear}
    return {**clear, **debug} if kind == "blade" else clear


def _road_fold(g, kind, offset=None):
    """The world position offset for `kind`'s master ("blade", "plant" or "prop"): `offset` (or
    none), turned into a fold to the plant's base where its base is on a road."""
    tex = _road_texture()
    if tex is None:
        return offset
    with open(f"{SRC}/info.json") as f:
        info = json.load(f)
    spacing, half = info["spacing"], (info["size"] - 1) / 2
    # The instance's origin. "Local" space is the primitive's here, not the instance's (the
    # engine's own TODO: inconsistent with instancing), so every blade of a grass component read
    # one texel (12 Nov).
    base = g.node(
        unreal.MaterialExpressionTransformPosition,
        transform_source_type=unreal.MaterialPositionTransformSource.TRANSFORMPOSSOURCE_INSTANCE,
        transform_type=unreal.MaterialPositionTransformSource.TRANSFORMPOSSOURCE_WORLD,
    )
    g.link(g.const((0.0, 0.0, 0.0)), base)
    texel = (half + 0.5) / ROAD_DIST_SIZE
    shift = g.node(unreal.MaterialExpressionConstant2Vector, r=texel, g=texel)
    # Divide, not multiply by the reciprocal: HLSL constants are written with 8 decimals, so
    # 1 / (700 * 4096) became 3.5e-7, 0.35% out, which moved the road 17 m at x = 4.7 km.
    size = g.const(float(spacing * ROAD_DIST_SIZE))
    uv = g.add(g.op(unreal.MaterialExpressionDivide, g.mask(base, "rg"), size), shift)
    sample = g.sample(tex, uv, "linear")
    sample.set_editor_property(
        "sampler_source", unreal.SamplerSourceMode.SSM_CLAMP_WORLD_GROUP_SETTINGS
    )
    sample.set_editor_property("mip_value_mode", unreal.TextureMipValueMode.TMVM_MIP_LEVEL)
    sample.set_editor_property("const_mip_value", 0)
    near = g.mask(sample, "b")
    pre = g.add(g.mul(g.mask(sample, "rg"), 2.0), -1.0)
    v = g.op(
        unreal.MaterialExpressionDivide,
        pre,
        g.op(unreal.MaterialExpressionMax, near, g.const(0.02)),
    )
    length = g.link(
        g.op(unreal.MaterialExpressionDotProduct, v, v),
        g.node(unreal.MaterialExpressionSquareRoot),
    )
    # Past the map's reach (B 0) a plant is far from any road.
    far = g.mul(
        g.link(g.saturate(g.mul(near, 4.0)), g.node(unreal.MaterialExpressionOneMinus)), 100.0
    )
    metres = g.add(g.mul(length, ROAD_DIST_NEAR), far)
    clear = g.param("Road clear", info["road_half"] + ROAD_REACH[kind])
    hide = g.saturate(g.mul(g.add(clear, g.mul(metres, -1.0)), 4.0))
    g.road_metres, g.road_sample, g.road_uv = metres, sample, uv  # for the "Road debug" colours
    fold = g.op(
        unreal.MaterialExpressionSubtract, base, g.node(unreal.MaterialExpressionWorldPosition)
    )
    return g.mul(fold, hide) if offset is None else g.lerp(offset, fold, hide)


def _wire_wpo(name, offset):
    if offset is not None and not lib.edit.connect_material_property(
        offset, "", unreal.MaterialProperty.MP_WORLD_POSITION_OFFSET
    ):
        raise RuntimeError(f"{name}: could not wire the world position offset")


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
    _wire_wpo(name, _road_fold(g, "prop"))
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
    # "Flat" pulls a plant toward one colour, the ground's own grass, keeping the scan's light
    # and dark: tinted scans alone never matched the ground they stood on (v29).
    scan_luma = g.op(unreal.MaterialExpressionDotProduct, g.mask(d, "rgb"), g.const(LUMA))
    shade = g.saturate(g.mul(scan_luma, g.param("Flat gain", 4.0)))
    flat = g.mul(g.mul(g.mask(g.param("Flat colour", (0.09, 0.16, 0.03)), "rgb"), shade), bright)
    colour = g.lerp(colour, flat, g.param("Flat", 0.0))
    # Darker toward the root, by height above the plant's own base (cm before its scale): the
    # blades' feet sink into shadow as real turf does, and hide where they meet the ground.
    local = g.mask(g.node(unreal.MaterialExpressionLocalPosition), "b")
    rise = g.saturate(g.op(unreal.MaterialExpressionDivide, local, g.param("Root height", 14.0)))
    root = g.lerp(g.param("Root shade", 0.35), g.const(1.0), rise)
    colour = g.mul(colour, root)
    rough = g.saturate(g.mul(g.mask(m, "g"), g.param("Roughness", 1.0)))
    rough = g.op(unreal.MaterialExpressionMax, rough, g.param("Rough floor", 0.6))
    sss = g.mul(g.mask(t, "rgb"), g.mask(g.param("TranslucencyTint", (0.5, 0.6, 0.4)), "rgb"))
    # Lit mostly by the ground's normal (straight up), not each card's own: per-card normals
    # made the carpet sparkle, lime where a blade faced the sun and black where it faced away
    # (the ferns went fully black). In world space, so a two-sided card's back is lit the same.
    face = g.node(unreal.MaterialExpressionVertexNormalWS)
    bent = g.lerp(g.mask(face, "rgb"), g.const((0.0, 0.0, 1.0)), g.param("Up normal", 0.85))
    normal = g.link(bent, g.node(unreal.MaterialExpressionNormalize))
    g.output(
        colour,
        normal,
        rough,
        specular=g.param("Specular", 0.15),
        opacity_mask=g.mask(d, "a"),
        subsurface=g.mul(sss, root),
        ao=g.mul(g.mask(m, "r"), root),
    )
    _wire_wpo(name, _road_fold(g, "plant"))
    material = g.material
    material.set_editor_property("tangent_space_normal", False)
    material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_MASKED)
    material.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE)
    material.set_editor_property("two_sided", True)
    material.set_editor_property("opacity_mask_clip_value", 0.33)
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    g.finish()
    log("rebuilt M_MS_Plant with per-instance variation")


# The climb's own grass (make_grass.py): clumps of real blades, shaded by M_Blade. Set on MI_Blade
# by every ground build and look; try values with `look --blade-param`.
BLADES = f"{FOLDER}/Blades"
BLADE_SRC = "/workspace/ue/world_gen/grass"
BLADE_LOOK = {
    # Root to tip. The root sits near the lit colour of the ground's own grass, so the blades
    # rise out of it; move them with LOOK_TINTS' "Forest mix".
    "Root colour": (0.03, 0.07, 0.012),
    # v32: (0.16, 0.3, 0.05) bleached to white in full sun past 10 m.
    # v34: a little darker and less yellow: v33 was one lime in full sun.
    "Tip colour": (0.075, 0.125, 0.03),
    "Gradient": 0.5,
    # Over tens of metres the field drifts toward this multiplier. v34: toward a duller, cooler
    # green (was a yellower (1.15, 1.05, 0.7)), so the meadow mixes two greens.
    "Drift tint": (0.7, 0.78, 0.8),
    # v33: more spread between blades and clumps: v32 was one even lime.
    "Blade variation": 0.8,
    "Variation": 0.4,
    "Dry share": 0.25,
    "Dry colour": (0.3, 0.26, 0.1),
    # v32: (0.35, 0.28, 0.14) glowed like cotton.
    "Seed colour": (0.14, 0.11, 0.06),
    "Translucency": (0.15, 0.25, 0.04),
    "Root shade": 0.35,
    "Up normal": 0.7,
    "Roughness": 0.85,
    "Specular": 0.05,
    # v34: sway at the tip in cm, and gusts per second (they roll across the slope every 13 m).
    "Wind": 5.0,
    "Wind speed": 0.35,
}


# The heath's cushions and tussocks are the same blade meshes under a second instance, MI_Heath:
# these over BLADE_LOOK. Olive leaves drifting to rust, straw tussocks, and heather-pink spikes.
HEATH_LOOK = {
    "Root colour": (0.035, 0.045, 0.015),
    "Tip colour": (0.11, 0.11, 0.035),
    "Drift tint": (1.25, 0.75, 0.5),
    "Blade variation": 1.0,
    "Variation": 0.6,
    "Dry share": 0.35,
    "Dry colour": (0.32, 0.25, 0.11),
    # (0.3, 0.09, 0.22) bleached to pale magenta in full sun.
    "Seed colour": (0.15, 0.04, 0.12),
    "Translucency": (0.1, 0.1, 0.03),
    "Wind": 1.5,
}
# The instance for a blade mesh, by the first word of its slug.
BLADE_INSTANCES = {"blade": ("MI_Blade", {}), "heath": ("MI_Heath", HEATH_LOOK)}


def _biome():
    """The biome the generator last wrote (make_climb.py's OC_BIOME), and its preset laid over
    the tables above: see biomes.py."""
    here = os.path.dirname(os.path.abspath(__file__))
    spec = importlib.util.spec_from_file_location("biomes", os.path.join(here, "biomes.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    try:
        with open(f"{SRC}/info.json") as f:
            name = json.load(f).get("biome", "alpine")
    except OSError:
        name = "alpine"
    module.apply(name, globals())
    log(f"biome {name}")
    return name


BIOME = _biome()


def _blade_instances(material=None, params=None):
    """MI_Blade and MI_Heath by slug prefix, made if `material` is given, with their looks set.
    `params` are trial values: for MI_Blade, or for MI_Heath when the name starts "heath:"."""
    out = {}
    for prefix, (name, look) in BLADE_INSTANCES.items():
        path = f"{BLADES}/{name}"
        mi = unreal.load_asset(path) if assets.does_asset_exist(path) else None
        if mi is None and material is not None:
            mi = lib.fresh(
                name,
                BLADES,
                unreal.MaterialInstanceConstant,
                unreal.MaterialInstanceConstantFactoryNew(),
            )
            lib.edit.set_material_instance_parent(mi, material)
        if mi is None:
            continue
        trial = {}
        for key, value in (params or {}).items():
            if key.startswith("heath:") and prefix == "heath":
                trial[key[6:]] = value
            elif not key.startswith("heath:") and prefix == "blade":
                trial[key] = value
        _set_params(mi, {**_road_look("blade"), **BLADE_LOOK, **look, **trial})
        out[prefix] = mi
    return out


def _mesh_folder(biome, slug):
    # glTF imports land in <slug>/<file>/StaticMeshes, so callers list "climb" folders recursively.
    return f"{BLADES}/{slug}" if biome == "climb" else f"{MS}/{biome}/{slug}"


def _blade_master():
    """M_Blade: opaque two-sided foliage coloured by the blade mesh's UVs alone (u: the blade's
    own random number, 1.0 a seed head; v: 0 at the root to 1 at the tip). No textures and no
    cut-out, so nothing thins or shimmers with distance."""
    g = _graph("M_Blade", BLADES)
    uv = g.node(unreal.MaterialExpressionTextureCoordinate)
    own, rise = g.mask(uv, "r"), g.mask(uv, "g")
    curve = g.lerp(rise, g.mul(rise, rise), g.param("Gradient", 0.5))
    colour = g.lerp(
        g.mask(g.param("Root colour", (0.03, 0.07, 0.012)), "rgb"),
        g.mask(g.param("Tip colour", (0.16, 0.3, 0.05)), "rgb"),
        curve,
    )
    drift = g.saturate(g.world_noise(1.0 / 2500, -0.6, 1.6, 2))
    tint = g.lerp(
        g.const((1.0, 1.0, 1.0)), g.mask(g.param("Drift tint", (1.3, 1.1, 0.6)), "rgb"), drift
    )
    colour = g.mul(colour, tint)
    rnd = g.node(unreal.MaterialExpressionPerInstanceRandom)
    blade = g.link(g.add(g.mul(own, 7.31), rnd), g.node(unreal.MaterialExpressionFrac))
    bright = g.add(
        g.add(g.const(1.0), g.mul(g.add(blade, -0.5), g.param("Blade variation", 0.5))),
        g.mul(g.add(rnd, -0.5), g.param("Variation", 0.25)),
    )
    colour = g.mul(colour, bright)
    # A share of the blades dries toward the tip.
    dry = g.saturate(g.mul(g.add(blade, g.add(g.param("Dry share", 0.08), -1.0)), 20.0))
    colour = g.lerp(
        colour, g.mask(g.param("Dry colour", (0.3, 0.26, 0.1)), "rgb"), g.mul(dry, rise)
    )
    seed = g.saturate(g.mul(g.add(own, -0.95), 40.0))
    colour = g.lerp(colour, g.mask(g.param("Seed colour", (0.35, 0.28, 0.14)), "rgb"), seed)
    root = g.lerp(g.param("Root shade", 0.35), g.const(1.0), g.saturate(g.mul(rise, 3.0)))
    face = g.node(unreal.MaterialExpressionVertexNormalWS)
    bent = g.lerp(g.mask(face, "rgb"), g.const((0.0, 0.0, 1.0)), g.param("Up normal", 0.7))
    normal = g.link(bent, g.node(unreal.MaterialExpressionNormalize))
    sss = g.mul(g.mul(colour, g.mask(g.param("Translucency", (0.5, 0.6, 0.2)), "rgb")), rise)
    # Wind: gusts roll across the slope and each blade bends from its root (v squared), a
    # little out of step with its neighbours.
    place = g.node(unreal.MaterialExpressionWorldPosition)
    along = g.add(g.mul(g.mask(place, "r"), 1.0 / 1300), g.mul(g.mask(place, "g"), 1.0 / 2100))
    beat = g.mul(g.node(unreal.MaterialExpressionTime), g.param("Wind speed", 0.35))
    phase = g.add(beat, along)
    gust = g.link(phase, g.node(unreal.MaterialExpressionSine, period=1.0))
    own_phase = g.add(g.mul(phase, 2.3), g.add(rnd, g.mul(own, 0.3)))
    flutter = g.link(own_phase, g.node(unreal.MaterialExpressionSine, period=1.0))
    push = g.add(g.add(g.mul(gust, 0.6), g.mul(flutter, 0.4)), 0.4)
    sway = g.mul(g.mul(push, g.mul(rise, rise)), g.param("Wind", 5.0))
    offset = g.mul(g.const((0.8, 0.6, -0.12)), sway)
    _wire_wpo("M_Blade", _road_fold(g, "blade", offset))
    metres = getattr(g, "road_metres", None)
    if metres is not None:
        # "Road debug" 1 colours each blade by the road distance it decodes: blue within 3 m of a
        # road centre, red rising to 50 m, and a green band every 5 m.
        band = g.link(g.mul(metres, 0.2), g.node(unreal.MaterialExpressionFrac))
        near_road = g.saturate(g.add(g.const(3.0), g.mul(metres, -1.0)))
        debug = g.op(
            unreal.MaterialExpressionAppendVector,
            g.op(
                unreal.MaterialExpressionAppendVector,
                g.saturate(g.mul(metres, 1.0 / 50)),
                g.mul(band, 0.5),
            ),
            near_road,
        )
        colour = g.lerp(colour, debug, g.param("Road debug", 0.0))
        # "Road debug sample" shows the map's texel as read; "Road debug uv" the UV, banded every
        # 1/8 of the map (red u, green v).
        colour = g.lerp(colour, g.mask(g.road_sample, "rgb"), g.param("Road debug sample", 0.0))
        scale = g.param("Road debug uv scale", 8.0)
        tiles = g.link(g.mul(g.road_uv, scale), g.node(unreal.MaterialExpressionFrac))
        uv_colour = g.op(unreal.MaterialExpressionAppendVector, tiles, g.const(0.0))
        colour = g.lerp(colour, uv_colour, g.param("Road debug uv", 0.0))
    g.output(
        colour,
        normal,
        g.param("Roughness", 0.55),
        specular=g.param("Specular", 0.2),
        subsurface=sss,
        ao=root,
    )
    material = g.material
    material.set_editor_property("tangent_space_normal", False)
    material.set_editor_property("blend_mode", unreal.BlendMode.BLEND_OPAQUE)
    material.set_editor_property("shading_model", unreal.MaterialShadingModel.MSM_TWO_SIDED_FOLIAGE)
    material.set_editor_property("two_sided", True)
    material.set_editor_property("used_with_instanced_static_meshes", True)
    material.set_editor_property("used_with_nanite", True)
    g.finish()
    log("built M_Blade")
    return _blade_instances(material)


def _blades(instances):
    """Imports make_grass.py's clumps as Nanite meshes under /Game/Climb/Blades/<slug>."""
    count = 0
    for name in sorted(os.listdir(BLADE_SRC)):
        if not name.endswith(".gltf"):
            continue
        task = unreal.AssetImportTask()
        task.filename = f"{BLADE_SRC}/{name}"
        task.destination_path = f"{BLADES}/{name[:-7]}"
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
        # Thin blades vanish as Nanite simplifies them unless it keeps their area.
        shape = getattr(unreal, "NaniteShapePreservation", None)
        for prop, value in (
            ("shape_preservation", shape and shape.PRESERVE_AREA),
            ("preserve_area", True),
        ):
            try:
                nanite.set_editor_property(prop, value)
                log(f"{name}: nanite {prop}")
                break
            except Exception:
                continue
        mesh.set_editor_property("nanite_settings", nanite)
        mesh.set_material(0, instances[name.split("_")[0]])
        assets.save_loaded_asset(mesh, False)
        b = mesh.get_bounds().box_extent
        log(f"blade {name}: ext {b.x:.0f} {b.y:.0f} {b.z:.0f}")
        count += 1
        yield 10
    log(f"blades: {count} meshes")


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


def _fill_empty_slots(biome, slug, meshes):
    """Gives a scan's meshes its own MI where a slot is empty. The root patch, fallen log and
    stump imported with none, so they drew in the engine's default grey: v16's "mystery boulder"
    that ignored every prop tint (v21 probe, out/qa/probe_nomat.py)."""
    path = f"{MS}/{biome}/{slug}/MI_{slug}"
    if not assets.does_asset_exist(path):
        return
    mi = unreal.load_asset(path)
    for mesh in meshes:
        slots = mesh.get_editor_property("static_materials")
        empty = [i for i, s in enumerate(slots) if not s.get_editor_property("material_interface")]
        for i in empty:
            mesh.set_material(i, mi)
        if empty:
            assets.save_loaded_asset(mesh)
            log(f"{mesh.get_name()}: MI_{slug} in {len(empty)} empty slot(s)")


def _gt_name(layer):
    """Grass types are saved per biome, so a switch of biome doesn't reuse the last one's."""
    prefix = "GT_" if BIOME == "alpine" else f"GT_{BIOME}_"
    return prefix + layer.replace(" ", "_")


def _grass_type(layer):
    gt = lib.fresh(
        _gt_name(layer),
        f"{FOLDER}/Grass",
        unreal.LandscapeGrassType,
        unreal.LandscapeGrassTypeFactory(),
    )
    with open(f"{PREP}/index.json") as f:
        plants = {r["slug"] for r in json.load(f) if r["kind"] == "plant"}
    varieties = []
    for biome, slug, density, cull_start, cull_end, *scale in GRASS[layer]:
        paths = assets.list_assets(_mesh_folder(biome, slug), biome == "climb")
        meshes = [a for a in map(unreal.load_asset, paths) if isinstance(a, unreal.StaticMesh)]
        if not meshes:
            raise RuntimeError(f"no meshes for {biome}/{slug}: run megascans.py first")
        if biome != "climb":
            _keep_alpha_coverage(biome, slug)
            _fill_empty_slots(biome, slug, meshes)
        for mesh in meshes:
            v = unreal.GrassVariety()
            v.set_editor_property("grass_mesh", mesh)
            v.set_editor_property("grass_density", unreal.PerPlatformFloat(default=density))
            v.set_editor_property("start_cull_distance", unreal.PerPlatformInt(default=cull_start))
            v.set_editor_property("end_cull_distance", unreal.PerPlatformInt(default=cull_end))
            v.set_editor_property("scaling", unreal.GrassScaling.UNIFORM)
            v.set_editor_property("scale_x", unreal.FloatInterval(*(scale or [(0.6, 1.4)])[0]))
            v.set_editor_property("random_rotation", True)
            # Plants grow upright: aligned to a 50 degree cut bank, tufts jutted sideways out of it
            # and read as almost nothing, whatever the density (v21: 3x density, no change).
            # Rocks, roots and logs still lie along the ground.
            v.set_editor_property("align_to_surface", slug not in plants and biome != "climb")
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
    """Whether a loaded landscape draws with this level's ground material, directly or through an
    instance. (The startup map's own landscape doesn't, and is harmless.)"""
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
    the skeletal trees loaded, and points its landscape at its own ground instance (INSTANCE) and
    its road at its road instance (ROAD_INSTANCE).
    Returns the world."""
    mi = unreal.load_asset(f"{GROUND}/{INSTANCE}")
    if mi is None:
        raise RuntimeError(f"no {GROUND}/{INSTANCE}: run `oc.py rebuild-ground` first")
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
    road = unreal.load_asset(f"{GROUND}/{ROAD_INSTANCE}")  # None until a rebuild-ground makes it
    changed = roads = 0
    for actor in unreal.EditorActorSubsystem().get_all_level_actors():
        if (
            isinstance(actor, unreal.LandscapeProxy)
            and actor.get_editor_property("landscape_material") != mi
        ):
            actor.set_editor_property("landscape_material", mi)
            changed += 1
        elif (
            road is not None
            and isinstance(actor, unreal.StaticMeshActor)
            and str(actor.get_folder_path()) == "Road"
        ):
            mesh = actor.static_mesh_component
            if mesh.get_material(0) != road:
                # Through the property, which marks the actor's package dirty: set_material
                # changed it in memory only, and the save skipped all 166 pieces (11 Nov).
                mesh.set_editor_property("override_materials", [road])
                roads += 1
    if changed or roads:
        yield 120
        unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
        unreal.EditorLoadingAndSavingUtils.save_map(world, LEVEL)
        log(
            f"landscape pointed at {INSTANCE} ({changed} pieces), road at {ROAD_INSTANCE} "
            f"({roads}); saved"
        )
    return world


# --- ops (run by oc_server.py; each a generator returning a result dict) -------------------------


def op_ride_track():
    """Rewrites the level's DA_RideTrack from its source (climb_world.track): the centreline,
    stretches and where rides start, without opening the level or rebuilding anything else."""
    yield
    cw = _old_build()
    cw.track()
    return {"notes": [f"rewrote {cw.RIDE}/DA_RideTrack from {cw.SRC}"]}


def op_road_look():
    """Rebuilds only the road's looks: M_ClimbRoad, M_ClimbPath (when the source has dirt-path
    pieces) and the level's road instance with info.json's road_look. Run on the empty map: the
    level's road meshes keep these asset paths, so its next load shows the new look."""
    yield
    cw = _old_build()
    road = cw.road_material()
    notes = [f"rebuilt {road.get_path_name()}"]
    if cw._has_paths():
        notes.append(f"rebuilt {cw.path_material().get_path_name()}")
    notes.append(f"rebuilt {cw.road_instance(road).get_path_name()}")
    return {"notes": notes}


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
            for path in assets.list_assets(_mesh_folder(biome, slug), biome == "climb"):
                mesh = unreal.load_asset(path)
                if isinstance(mesh, unreal.StaticMesh):
                    unreal.EditorStaticMeshLibrary.get_number_verts(mesh, 0)  # waits for it
                    yield 20
            yield 20


def op_rebuild_ground(rebuild_grass=False):
    """The level's ground material and instance (MATERIAL, INSTANCE), rebuilt. Only on the empty
    map (see _guard_rebuild). Saved grass types are reused: building them takes over ten
    minutes, on one core."""
    _guard_rebuild()
    _plant_master()
    _prop_master()
    _plant_params(PLANT_LOOK)
    _plant_params(PLANT_SLUG_LOOK)
    _prop_params(PROP_LOOK)
    _tree_look()
    yield 20
    blade = _blade_master()
    if rebuild_grass or not assets.does_directory_exist(f"{BLADES}/heath_low"):
        yield from _blades(blade)
    yield from _warm_grass_meshes()
    grass = {}
    for layer in GRASS:
        saved = unreal.load_asset(f"{FOLDER}/Grass/{_gt_name(layer)}")
        grass[layer] = saved if saved and not rebuild_grass else _grass_type(layer)
        yield 10
    # The instance goes first: while it refers to the material, the material can't be deleted,
    # and a reused material keeps its old Landscape Grass node, which fails the compile.
    # The material goes too: a reused one can keep a stale Landscape Grass node that
    # delete_all_material_expressions misses (after the crash of 9 Nov), and the compile fails.
    for name in (INSTANCE, MATERIAL):
        if assets.does_asset_exist(f"{GROUND}/{name}"):
            assets.delete_asset(f"{GROUND}/{name}")
    yield 5
    material = Ground().build(grass)
    yield 30
    mi = lib.fresh(
        INSTANCE,
        GROUND,
        unreal.MaterialInstanceConstant,
        unreal.MaterialInstanceConstantFactoryNew(),
    )
    lib.edit.set_material_instance_parent(mi, material)
    lib.edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)
    _tune(LOOK_TINTS, LOOK_GREYS)
    yield 10
    # The road's material too, and the level's instance of it at its road's width.
    build = _old_build()
    build.island.FOLDER = FOLDER
    build.road_instance(build.road_material())
    yield 20
    return {
        "notes": [
            f"rebuilt {MATERIAL} and {INSTANCE}, with the look {LOOK_TINTS} {LOOK_GREYS}; "
            f"rebuilt M_ClimbRoad and {ROAD_INSTANCE}"
        ]
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
        gt = unreal.load_asset(f"{FOLDER}/Grass/{_gt_name(layer)}")
        if gt is None:
            raise RuntimeError(f"no {_gt_name(layer)}: run `oc.py rebuild-ground --rebuild-grass`")
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
    "sun_pitch": "sun: elevation in degrees, negative (the level's is -16)",
    "sun_yaw": "sun: compass yaw in degrees (the level's is 44)",
    "contact": "sun: contact shadow length, screen fraction (0.05 grounds posts)",
    "clouds": "volumetric clouds: Cloud_GlobalCoverage on MI_ClimbClouds",
    "cloud_density": "volumetric clouds: Cloud_GlobalDensity on MI_ClimbClouds",
    "sky_lum": "sky atmosphere: sky luminance factor, 1 neutral; lower is a deeper, moodier sky",
    "bloom": "post volume: bloom intensity (0.675 default)",
    "flare": "post volume: lens flare intensity (1 default)",
    "aerial": "sky atmosphere: aerial perspective distance scale, 1 neutral; lower is less haze",
    "cloud_bottom": "volumetric clouds: layer bottom altitude in km",
    "cloud_wind": "volumetric clouds: wind scale, 1 the template's; 0 freezes them",
    "fog_inscatter": "height fog: directional inscattering (sun glow) intensity scale, 0 off",
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
    keys = ("exposure", "ev", "contrast", "saturation", "vignette", "temp", "bloom", "flare")
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
        for key, prop in (("bloom", "bloom_intensity"), ("flare", "lens_flare_intensity")):
            if key in post:
                st.set_editor_property(f"override_{prop}", True)
                st.set_editor_property(prop, float(post[key]))
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
    if {"sun_pitch", "sun_yaw"} & set(post):
        for sun in _level_actors(unreal.DirectionalLight):
            rot = sun.get_actor_rotation()
            log(f"sun was pitch {rot.pitch:.1f} yaw {rot.yaw:.1f}")
            sun.set_actor_rotation(
                unreal.Rotator(
                    roll=rot.roll,
                    pitch=float(post.get("sun_pitch", rot.pitch)),
                    yaw=float(post.get("sun_yaw", rot.yaw)),
                ),
                False,
            )
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
    if "cloud_bottom" in post:
        for cloud in _level_actors(unreal.VolumetricCloud):
            comp = cloud.get_component_by_class(unreal.VolumetricCloudComponent)
            log(f"cloud layer bottom was {comp.get_editor_property('layer_bottom_altitude')} km")
            comp.set_editor_property("layer_bottom_altitude", float(post["cloud_bottom"]))
    if "fog_inscatter" in post:
        for fog in _level_actors(unreal.ExponentialHeightFog):
            comp = fog.get_component_by_class(unreal.ExponentialHeightFogComponent)
            v = float(post["fog_inscatter"])
            comp.set_editor_property(
                "directional_inscattering_luminance", unreal.LinearColor(v, v, v, 1.0)
            )
    if "clouds" in post or "cloud_density" in post or "cloud_wind" in post:
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
            if "cloud_wind" in post:
                # The clouds drift with the editor's clock and the sky light captures them in real
                # time, so an unfrozen still depends on how long after start it was taken.
                w = float(post["cloud_wind"])
                lib.edit.set_material_instance_vector_parameter_value(
                    mi, "Layout_WindControls", unreal.LinearColor(w, w, 0.5 * w, w / 3)
                )
            assets.save_loaded_asset(mi, False)
            comp.set_editor_property("material", mi)
    if "sky_lum" in post:
        for atmo in _level_actors(unreal.SkyAtmosphere):
            comp = atmo.get_component_by_class(unreal.SkyAtmosphereComponent)
            v = float(post["sky_lum"])
            comp.set_editor_property("sky_luminance_factor", unreal.LinearColor(v, v, v, 1.0))
    if "aerial" in post:
        for atmo in _level_actors(unreal.SkyAtmosphere):
            comp = atmo.get_component_by_class(unreal.SkyAtmosphereComponent)
            prop = "aerial_pespective_view_distance_scale"  # sic: the engine's spelling
            log(f"aerial perspective scale was {comp.get_editor_property(prop)}")
            comp.set_editor_property(prop, float(post["aerial"]))
    unreal.EditorLoadingAndSavingUtils.save_dirty_packages(True, True)
    log(f"post {post}")


def _tune(tints, greys=None):
    greys = greys or {}
    names = tint_names()
    bad = [k for k in [*tints, *greys] if k not in names]
    if bad:
        raise RuntimeError(f"unknown tint {bad}; use one of {names}")
    mi = unreal.load_asset(f"{GROUND}/{INSTANCE}")
    if mi is None:
        raise RuntimeError(f"no {GROUND}/{INSTANCE}: run `oc.py rebuild-ground` first")
    for name, rgb in tints.items():
        lib.edit.set_material_instance_vector_parameter_value(
            mi, f"{name} tint", unreal.LinearColor(*rgb, 1.0)
        )
    for name, amount in greys.items():
        lib.edit.set_material_instance_scalar_parameter_value(mi, f"{name} grey", float(amount))
    lib.edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)
    log(f"tuned {tints} grey {greys}")


def _views():
    """info.json's views ([name, eye, target] in m), if the generator chose them: the world's
    ride has one per stretch."""
    try:
        with open(f"{SRC}/info.json") as f:
            views = json.load(f).get("views")
    except OSError:
        return None
    if not views:
        return None
    return [(n, tuple(v * 100 for v in p), tuple(v * 100 for v in a)) for n, p, a in views]


def cameras(only=None):
    """(name, position, look-at) in cm: a rider's eye along the road, looking 30 m up it, and one
    from above the hairpin back down the lower traverse; or info.json's views."""
    views = _views()
    if views:
        return [v for v in views if not only or v[0] in only]
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


def _grass_instances():
    """Landscape grass built so far: GrassInstancedStaticMeshComponents on the streaming proxies
    (not found as HierarchicalInstancedStaticMeshComponent), plus their CPU-side instances. A
    component appears as each piece of grass finishes building."""
    n = 0
    for actor in unreal.get_editor_subsystem(unreal.EditorActorSubsystem).get_all_level_actors():
        if isinstance(actor, unreal.LandscapeProxy):
            for c in actor.get_components_by_class(unreal.GrassInstancedStaticMeshComponent):
                n += 1 + c.get_instance_count()
    return n


def _settle(name, before=None):
    """Waits for a new view's grass: STILL_SETTLE_MIN frames, then until the instance count is
    steady. Falls back to STILL_SETTLE if no grass components are found.

    before: the count at the last view. A count still equal to it is the last view's grass, not
    yet rebuilt (a look shot road_72 bare at "settled 360 frames, 1199" after road_50's 1227), so
    steady only counts once the count has moved, or after half of STILL_SETTLE_MAX."""
    yield STILL_SETTLE_MIN
    frames, last, steady, moved = STILL_SETTLE_MIN, -1, 0, before is None
    while frames < STILL_SETTLE_MAX:
        n = _grass_instances()
        if n == 0 and frames >= STILL_SETTLE:
            break
        moved = moved or n != before or frames >= STILL_SETTLE_MAX // 2
        steady = steady + 1 if n == last and n > 0 and moved else 0
        if steady >= GRASS_STEADY:
            break
        last = n
        yield 30
        frames += 30
    log(f"settled {name}: {frames} frames, {last} grass instances")


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
    before = None
    for name, where, target in plan:
        path = f"{out}/{name}.png"
        if os.path.exists(path):
            os.remove(path)
        eye = unreal.Vector(*where)
        editor.set_level_viewport_camera_info(
            eye, unreal.MathLibrary.find_look_at_rotation(eye, unreal.Vector(*target))
        )
        yield from _settle(name, before)  # grass, textures and shadows stream in around the view
        before = _grass_instances()
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


def op_fps(
    quality=0,
    frames=240,
    shots=None,
    tree_cull=None,
    tree_shadows=None,
    tree_wpo=None,
    tries=None,
):
    """Frame times at each still's view, at a scalability level (0 Low .. 3 Epic): the editor's
    whole frame, so a pessimistic stand-in for the game's. Vsync and the frame cap off.

    tries: ["label=cvar value;cvar value", ...]. Each is measured at every view after the base
    run, alone, and its cvars are put back after it: one editor session ablates several costs."""
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
    system = unreal.SystemLibrary
    for name, where, target in cameras(shots):
        eye = unreal.Vector(*where)
        editor.set_level_viewport_camera_info(
            eye, unreal.MathLibrary.find_look_at_rotation(eye, unreal.Vector(*target))
        )
        yield 150  # streaming settles
        for label, commands in [("base", [])] + [_parse_try(s) for s in tries or []]:
            saved_cvars = []
            for cvar, value in commands:
                saved_cvars.append((cvar, system.get_console_variable_float_value(cvar)))
                system.execute_console_command(None, f"{cvar} {value}")
            yield 60 if commands else 0
            times = []
            last = time.perf_counter()
            for _ in range(int(frames)):
                yield 1
                now = time.perf_counter()
                times.append((now - last) * 1000.0)
                last = now
            for cvar, value in reversed(saved_cvars):
                system.execute_console_command(None, f"{cvar} {value:g}")
            times.sort()
            mean = sum(times) / len(times)
            p95 = times[int(len(times) * 0.95)]
            tag = name if label == "base" else f"{name} [{label}]"
            notes.append(f"{tag}: {1000.0 / mean:.1f} fps (mean {mean:.1f} ms, p95 {p95:.1f} ms)")
            log(notes[-1])
    unreal.SystemLibrary.execute_console_command(None, "scalability 3")
    for c, (cull, shadow) in zip(trees, saved, strict=True):
        c.set_visibility(True)
        c.set_cull_distances(0, cull)
        c.set_cast_shadow(shadow)
        if tree_wpo is not None:
            c.set_evaluate_world_position_offset(True)
    return {"notes": [f"scalability {quality}", *notes]}


def _parse_try(spec):
    """ "label=cvar value;cvar value" -> (label, [(cvar, value), ...])."""
    label, _, body = spec.partition("=")
    commands = [c.split(None, 1) for c in body.split(";") if c.strip()]
    if not commands or any(len(c) != 2 for c in commands):
        raise ValueError(f"--try wants LABEL=CVAR VALUE[;CVAR VALUE]: {spec!r}")
    return label, [(c, v.strip()) for c, v in commands]


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
    """Sets parameters on named scans' MIs, e.g. PROP_LOOK (M_MS_Prop: Tint, Roughness). Plants
    work too (`look --prop-param grass_clumps:Tint=...`), to try one plant's PLANT_SLUG_LOOK."""
    with open(f"{PREP}/index.json") as f:
        props = {r["slug"]: r for r in json.load(f) if r["kind"] in ("prop", "plant")}
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
    prop_params=None,
    blade_params=None,
    stills=True,
    shots=None,
    out=None,
):
    """Recolour through the ground instance's parameters (tint: {"Dirt": [r, g, b]}, grey: {"Dirt":
    0.8}), grass ([density, cull] factors on GRASS), fog (density), then stills. Nothing
    compiles. PLANT_LOOK and PROP_LOOK are applied too, so a false-colour look is undone by the
    next look, and trying a prop's tint needs only a restart."""
    _plant_params({**_road_look("plant"), **PLANT_LOOK})
    _plant_params(PLANT_SLUG_LOOK)
    _prop_params(PROP_LOOK)
    _blade_instances(params=blade_params)
    if prop_params:
        _prop_params(prop_params)
    # The baked tints first, so a false-colour look's tints don't outlive it.
    _tune(
        {**LOOK_TINTS, **{k: tuple(v) for k, v in (tint or {}).items()}},
        {**LOOK_GREYS, **(grey or {})},
    )
    if grass:
        _grass(*grass)
    if plants:
        _plant_tint(plants)
    if plant_params:
        _plant_params(plant_params)
    if ground_params:
        _set_params(unreal.load_asset(f"{GROUND}/{INSTANCE}"), ground_params)
    yield
    notes = [f"tuned {tint} grey {grey}"] if tint or grey else []
    notes += [f"plants tinted {plants}"] if plants else []
    notes += [f"plant params {plant_params}"] if plant_params else []
    notes += [f"ground params {ground_params}"] if ground_params else []
    notes += [f"prop params {prop_params}"] if prop_params else []
    notes += [f"blade params {blade_params}"] if blade_params else []
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
