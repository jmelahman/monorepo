"""Biome presets for the climb pipeline: what each landscape layer is made of and what grows on
it, per biome. No Unreal here, so the generator and the tests can read it too.

One level slot (L_Hero) holds one biome at a time. To switch:

    OC_BIOME=desert <generator> scripts/make_climb.py     (writes "biome" into info.json)
    oc.py restart && oc.py rebuild-ground && oc.py restart && oc.py build-level && oc.py look ...

`oc_climb.py` reads the biome from info.json when it loads and lays the preset over its own
tables with `apply`. The alpine forest is the base: its preset is empty. Grass types are saved
per biome (GT_<biome>_<layer>), so switching back is quick.

The seven paint layers are fixed names; a preset says what each one *is* in that biome:

| layer  | alpine              | desert                  | coast                     |
| Forest | litter + grass      | sand + dry mud patches  | dry sand + grass patches  |
| Grass  | meadow              | dry mud flats           | meadow                    |
| Desert | treeline heath      | (unused)                | (unused)                  |
| Sand   | cut banks' soil     | cracked mud banks       | rippled sand banks        |
| Dirt   | path edge           | path edge, dusty        | path edge, sandy          |
| Rock   | granite             | granite tinted red      | granite                   |
| Snow   | snow                | (unused)                | wet beach at the waterline|

A preset's keys are oc_climb table names. SURFACES, MIXES, LOOK_TINTS, LOOK_GREYS, STEEP,
BLADE_LOOK and PROP_LOOK are laid over the base key by key; FLAT and GRASS replace the base
(GRASS keeps the base's layers named in KEEP_GRASS). Rows are oc_climb's: see GRASS there.

Every value below is a first guess that has been built once at most. Tune with `oc.py look`
(--tint, --grey, --blade-param) before baking, as CLAUDE.md says.
"""

DRY_BLADES = {
    "Root colour": (0.12, 0.1, 0.04),
    "Tip colour": (0.3, 0.25, 0.1),
    "Drift tint": (1.0, 0.9, 0.7),
    "Dry share": 0.8,
    "Translucency": (0.2, 0.17, 0.06),
}
SAND = (0.62, 0.47, 0.29)
MUD = (0.42, 0.31, 0.19)

PRESETS = {
    "alpine": {},
    # Farmland and lanes: the alpine tables on a generator shape with open fields (make_climb).
    "meadow": {},
    "desert": {
        # Sand is several times brighter than the alpine greens the standard look is exposed
        # for: at ev=-2.6 it blew out to white whatever its tint (10 Nov).
        "POST": {"ev": -0.4},
        "SURFACES": {
            "Forest": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
            "Grass": ("desert", "dry_mud_ground", (1.0, 1.0, 1.0)),
            "Sand": ("desert", "dry_cracked_mud", (1.0, 1.0, 1.0)),
            "Dirt": ("desert", "dry_mud_ground", (1.0, 1.0, 1.0)),
        },
        "MIXES": {
            # Dry mud pans in the sand, tens of metres across: the scrub grows on them.
            "Forest": ("desert", "dry_mud_ground", "patches", 18.0, (-1.0, 2.0)),
            "Grass": ("desert", "dry_cracked_mud", "patches", 20.0),
            "Sand": ("desert", "bright_desert_sand", "patches", 4.0),
            "Dirt": ("desert", "bright_desert_sand", "patches", 4.0),
            "Rock": ("desert", "dry_cracked_mud", "patches", 12.0),
        },
        "LOOK_TINTS": {
            "Forest": SAND,
            "Forest mix": MUD,
            "Grass": MUD,
            "Grass mix": (0.5, 0.38, 0.24),
            "Sand": (0.45, 0.3, 0.18),
            "Sand mix": SAND,
            "Dirt": MUD,
            "Dirt mix": SAND,
            "Rock": (0.5, 0.29, 0.17),  # red sandstone from grey granite
            "Rock mix": (0.5, 0.33, 0.2),
        },
        "LOOK_GREYS": {
            "Forest": 0.5,
            "Forest mix": 0.5,
            "Grass": 0.5,
            "Grass mix": 0.5,
            "Sand": 0.5,
            "Sand mix": 0.5,
            "Dirt": 0.5,
            "Dirt mix": 0.5,
            "Rock": 0.8,
            "Rock mix": 0.6,
        },
        "STEEP": {k: (0.85, 0.7, 0.55) for k in ("Forest", "Sand", "Dirt")},
        "BLADE_LOOK": DRY_BLADES,
        "PROP_LOOK": {
            "canyon_sandstone_boulder": {"Tint": (1.0, 1.0, 1.0)},
            "rock_sandstone": {"Tint": (1.0, 1.0, 1.0)},
        },
        "GRASS": {
            # The mud pans: straw tufts, dry grass and dead scrub.
            "Forest": (
                ("climb", "blade_short", 30.0, 2500, 4000, (0.6, 1.0)),
                ("climb", "blade_mid", 25.0, 3000, 5000, (0.6, 1.0)),
                ("desert", "dry_grass", 2.0, 4000, 6000, (0.8, 1.5)),
                ("desert", "dead_shrubs", 0.6, 6000, 9000, (0.8, 1.6)),
            ),
            # Open sand: cacti and sandstone, far apart.
            "Forest floor": (
                ("desert", "dead_shrubs", 0.25, 6000, 9000, (0.8, 1.6)),
                ("desert", "desert_american_plant_cactus_medium_11", 0.12, 9000, 12000, (1.0, 2.2)),
                (
                    "desert",
                    "desert_american_plant_branch_cactus_large_03",
                    0.08,
                    9000,
                    12000,
                    (3.0, 6.0),
                ),
                ("desert", "canyon_sandstone_boulder", 0.15, 9000, 12000, (0.8, 2.5)),
                ("desert", "rock_sandstone", 0.6, 6000, 8000, (0.8, 2.0)),
                ("desert", "desert_western_cluster_rock_medium_05", 0.04, 12000, 15000),
            ),
            "Grass": (
                ("climb", "blade_short", 60.0, 2500, 4000, (0.6, 1.1)),
                ("climb", "blade_mid", 50.0, 3000, 5000, (0.6, 1.1)),
                ("desert", "dry_grass", 3.0, 4000, 6000, (0.8, 1.5)),
                ("desert", "dead_shrubs", 0.3, 6000, 9000),
            ),
            "Sand": (
                ("desert", "rock_sandstone", 1.0, 6000, 8000, (0.6, 1.6)),
                ("desert", "dry_grass", 1.0, 4000, 6000),
            ),
        },
    },
    "coast": {
        "POST": {"ev": -1.6},  # between the greens and the sand: -0.8 left the grass dark
        "SURFACES": {
            "Forest": ("coast", "trampled_beach_sand", (1.0, 1.0, 1.0)),
            "Sand": ("coast", "rippled_sand", (1.0, 1.0, 1.0)),
            "Dirt": ("coast", "trampled_beach_sand", (1.0, 1.0, 1.0)),
        },
        "MIXES": {
            "Forest": ("shared", "uncut_grass", "patches", 14.0, (-1.8, 1.4)),
            "Sand": ("coast", "beach_pebbles", "patches", 5.0),
            "Dirt": ("coast", "beach_pebbles", "patches", 4.0),
        },
        "LOOK_TINTS": {
            "Forest": (0.55, 0.47, 0.33),
            "Sand": (0.55, 0.47, 0.33),
            "Sand mix": (0.4, 0.37, 0.32),
            "Dirt": (0.5, 0.43, 0.3),
            "Dirt mix": (0.4, 0.37, 0.32),
            "Snow": (0.42, 0.36, 0.26),  # wet sand, darker than the dry sand above it
        },
        "LOOK_GREYS": {"Forest": 0.3, "Sand": 0.3, "Dirt": 0.3, "Sand mix": 0.3, "Dirt mix": 0.3},
        "STEEP": {k: (0.9, 0.85, 0.75) for k in ("Forest", "Sand", "Dirt")},
        "FLAT": {},  # the Snow layer is wet sand here, in its own colour
        "KEEP_GRASS": ("Grass",),
        "GRASS": {
            # Grass patches behind the beach, with fan palms.
            "Forest": (
                ("climb", "blade_short", 120.0, 2500, 4000, (0.8, 1.3)),
                ("climb", "blade_mid", 100.0, 3000, 5000, (0.8, 1.3)),
                ("coast", "marram_grass", 3.0, 3000, 5000, (1.0, 1.8)),
                ("urban", "chinese_fan_palm", 0.1, 9000, 12000, (1.5, 3.0)),
            ),
            # Dry sand: marram, palms, driftwood.
            "Forest floor": (
                ("coast", "marram_grass", 4.0, 3000, 5000, (1.0, 1.8)),
                ("coast", "alexandra_palm", 0.15, 9000, 12000, (2.0, 4.0)),
                ("coast", "tropical_palm", 0.15, 9000, 12000, (2.0, 4.0)),
                ("coast", "nordic_beach_rocks", 0.05, 12000, 15000),
                ("coast", "eroded_pine_log", 0.03, 9000, 12000),
            ),
            "Sand": (("coast", "marram_grass", 3.0, 3000, 5000, (1.0, 1.8)),),
            # The waterline.
            "Snow": (
                ("coast", "nordic_beach_rocks", 0.08, 12000, 15000),
                ("coast", "thai_beach_log", 0.03, 9000, 12000),
            ),
        },
    },
}
# "POST" is not laid over a table: `scripts/oc_biome.sh` adds it to the look's `--post` keys.
# The whole world as one level (make_world_level.py, L_World): the alpine climb's layers for the
# green ground, with the desert's sand on the Desert layer and the beaches on Sand. The look is
# the alpine daylight (ev -2.6), so the sand is tinted darker than in the desert's own preset
# (ev -0.4). Grass keeps the alpine rows bar the heath's and the cut banks'.
PRESETS["world"] = {
    # Dirt is the verge, the road's own shoulder: short turf, the Grass layer's look, rather than
    # a 35 m strip of mud.
    "SURFACES": {
        "Desert": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
        "Sand": ("coast", "trampled_beach_sand", (1.0, 1.0, 1.0)),
        "Dirt": ("shared", "uncut_grass", (0.55, 0.58, 0.42)),
        # Verge is the bare sand beside the road in the desert and on the beaches
        # (make_world_level.py): Desert's sand with no mix and no GRASS row, so nothing grows.
        "Verge": ("desert", "bright_desert_sand", (1.0, 1.0, 1.0)),
    },
    "MIXES": {
        "Desert": ("desert", "dry_mud_ground", "patches", 30.0),
        "Sand": ("coast", "beach_pebbles", "patches", 5.0),
        "Dirt": ("shared", "lawn_grass", "patches", 30.0),
    },
    "LOOK_TINTS": {
        "Dirt": (0.14, 0.23, 0.04),
        "Dirt mix": (0.14, 0.22, 0.04),
        "Desert": (0.2, 0.15, 0.09),
        "Desert mix": (0.14, 0.1, 0.06),
        "Verge": (0.2, 0.15, 0.09),
        "Sand": (0.28, 0.24, 0.17),
        "Sand mix": (0.2, 0.18, 0.16),
    },
    "LOOK_GREYS": {
        "Desert": 0.4,
        "Desert mix": 0.4,
        "Verge": 0.4,
        "Sand": 0.3,
        "Sand mix": 0.3,
        "Dirt": 0.0,
        "Dirt mix": 0.0,
    },
    "STEEP": {"Desert": (0.85, 0.7, 0.55), "Sand": (0.9, 0.85, 0.75)},
    "KEEP_GRASS": ("Grass", "Forest", "Forest floor", "Rock"),
    "GRASS": {
        # The verge (12 Nov): short turf to the tarmac's edge, now that the road fold works. Bare,
        # it read as a lime strip in the sun beside the darker blades.
        "Dirt": (
            ("climb", "blade_short", 140.0, 2500, 4000, (0.6, 1.0)),
            ("climb", "blade_mid", 40.0, 3000, 4500, (0.6, 1.0)),
            ("shared", "clovers", 12.0, 1500, 2500),
            ("shared", "ribwort_plantain", 0.4, 2000, 3000),
        ),
        "Desert": (
            ("desert", "dry_grass", 2.0, 4000, 6000, (0.8, 1.5)),
            ("desert", "dead_shrubs", 0.4, 6000, 9000, (0.8, 1.6)),
            ("desert", "desert_american_plant_cactus_medium_11", 0.08, 9000, 12000, (1.0, 2.2)),
            ("desert", "canyon_sandstone_boulder", 0.1, 9000, 12000, (0.8, 2.5)),
            ("desert", "rock_sandstone", 0.4, 6000, 8000, (0.8, 2.0)),
        ),
        "Sand": (
            ("coast", "marram_grass", 3.0, 3000, 5000, (1.0, 1.8)),
            ("coast", "nordic_beach_rocks", 0.05, 12000, 15000),
            ("coast", "eroded_pine_log", 0.02, 9000, 12000),
        ),
    },
}
LAID_OVER = ("SURFACES", "MIXES", "LOOK_TINTS", "LOOK_GREYS", "STEEP", "BLADE_LOOK", "PROP_LOOK")


def apply(name, tables):
    """Lays preset `name` over `tables` (oc_climb's globals), in place."""
    preset = PRESETS[name]
    for key in LAID_OVER:
        tables[key].update(preset.get(key, {}))
    if "FLAT" in preset:
        tables["FLAT"].clear()
        tables["FLAT"].update(preset["FLAT"])
    if "GRASS" in preset:
        kept = {k: tables["GRASS"][k] for k in preset.get("KEEP_GRASS", ())}
        tables["GRASS"].clear()
        tables["GRASS"].update({**kept, **preset["GRASS"]})
