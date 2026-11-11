# OpenCycle: the world plan and the biome foundation

Written 10 Nov for whoever continues (the user plans to hand this to Opus). Read `CLAUDE.md`
first for the loop, the crash rules and the vegetation rules; this file is the map of what
exists beyond the alpine climb and what to do with it.

## What the user asked for (10 Nov, evening)

- "A whole world like Skyrim or Watopia. The asymmetry adds to the immersion." Creative
  liberty on the island.
- "Each biome had a ~1-2 hour ride. Maybe a whole loop around the world being ~5-6 hours."
- "A foundation for every biome that I could eventually pass off to Opus", to be
  extrapolated to the rest of the world. Foundation, not polish.

## The two halves

1. **The world plan** (`scripts/make_world.py` → `/workspace/ue/world_gen/world`). A map,
   not a landscape: the old island's layout at seven times the size (57 km across, 28 m a
   sample), with regions, roads every 10 m at graded heights, and rides. Unreal never loads
   it. It takes about 80 s to run and needs no editor.
2. **Biome stretches** (`scripts/biomes.py`, `OC_BIOME` in `scripts/make_climb.py`). Each
   biome is proven as a 2 km stretch in the one level slot (L_Hero), on the same pipeline
   and commands as the alpine climb, so every lesson in CLAUDE.md applies to all of them.

Why not the old 8 km island level (`make_island.py`, `island_world.py`, L_Island)? It ran at
3 to 4 fps and predates everything that made the climb look real (blade grass, the ground
rules, daylight, static trees). Its *layout* is reused; its level is not.

## The world as planned

`world_gen/world/preview.png` is the map (red dots are landmarks); `world.json` has the numbers
below, the landmarks, each road's cut and fill (`roads`) and the windows.

| Ride | km | Climb m | Hours* | Mostly |
| --- | --- | --- | --- | --- |
| island (the ring) | 165.5 | 1752 | 5.8 | coast, desert, forest, heath, town, mountain |
| forest | 49.6 | 543 | 1.7 | forest, the fjord's south shore, west cliffs |
| coast | 49.7 | 244 | 1.7 | south beaches, Gull Point, town, farmland |
| desert | 48.5 | 361 | 1.65 | desert, mesas |
| dunes | 43.9 | 382 | 1.5 | desert, Beacon Head, harbour |
| meadow | 39.6 | 171 | 1.3 | farmland |
| summit | 21.1 up, 42.2 there and back | 891 | 1.75 | mountain, 7 hairpins, to 1031 m of 1076 |

*At 30 km/h on the flat, slower uphill (`hours` in make_world.py). No ride is steeper than
7.8% anywhere, and every ride's widest gap between points is one 10 m step.

The broad shape (coast, region blend and heights, mesas, canyon) is still `make_island.py`'s
times `K = 7`. Everything else is make_world's own, in world km (X east, Y south):

- **Landforms:** `FJORD` (Skarfjord, cut into the west coast between forest and heath; the
  ring runs in along its south shore, round Fjordhead and out along its north shore),
  `SPINE` (a spur ridge running south off the mountain between forest and farmland, with
  two passes in `SPINE_PASSES`: Heath Gap for the ring, Spine Gap for the cross road),
  `HORN` (a second peak in the north-east, 786 m) and `HEADLANDS` (Beacon Head in the east).
  The ring also loops round Gull Point, the south-west spit.
- **Roads** (`ROADS`): drawn control points. An open road's ends snap onto the nearest
  earlier road and take its height; those shared points are junctions, `<road>.a` / `.b`.
- **Rides** (`ROUTES`): legs between junctions, so legs share a point.
- **The summit climb** (`climb`, settings in `CLIMB`): an A* search over 60 m cells and 16
  headings on the mountain smoothed to 200 m, with a penalty above 6.6%, a corridor around
  the start–summit line, and hairpins as an explicit move (a fixed cost, at least 6 cells
  apart). Ends 220 m from the summit.
- **Landmarks** (`LANDMARKS`): name, position, one line each, in world.json.
- **Audit:** each run prints, per road, max and mean |road z − ground z| with the worst
  place, and any point over the sea (none now). Worst: the summit road (67 m fill near the
  top, mean 9.5 m), the ring (31 m cut on the mountain's foot), the forest road (9 m).
- **Windows:** one 2 km stretch per biome, at least 3 km inland except the coast's.
  `scripts/make_window.py <biome>` cuts one from the plan (see its docstring).

Known gaps in the plan:

- Roads are graded (under 8%) but not carved into the height map; the summit road's top
  kilometre fills up to 67 m over crags the plan can't resolve at 28 m.
- Roads cross each other only at junctions by design; no check finds an accidental
  crossing at different heights.
- The canyon and mesas are the island's at seven times the size; the canyon reads as three
  parallel grooves on the map. No river yet.
- Beacon Head is narrow, and the Horn is a blob of ridged noise; both could use a shaped
  profile. A faint streak shows on the Spine's north end where it fades in.
- Heath and town have no ride of their own (the ring covers them).

## Biome presets: how they work

- `OC_BIOME=<name>` on the generator picks `SHAPES[name]` in `make_climb.py`: treeline,
  snowline, how open the woods are, how many trees, whether there is a sea. The flank, the
  path and its hairpin are the same in every biome for now. The name goes into
  `world_gen/climb/info.json`.
- `oc_climb.py` reads that name when it loads (so `oc.py restart` after a generator run) and
  lays `biomes.PRESETS[name]` over SURFACES, MIXES, LOOK_TINTS, LOOK_GREYS, STEEP,
  BLADE_LOOK, PROP_LOOK, FLAT and GRASS. The table at the top of `biomes.py` says what each
  of the seven paint layers *is* in each biome.
- Grass types are saved per biome (`GT_<biome>_<layer>`; alpine keeps `GT_<layer>`).
- A biome with a sea gets a water plane (`climb_world.sea`, M_ClimbSea) at `sea_cm` and its
  "Snow" layer painted as wet beach at the waterline.
- One slot: building a biome replaces the last one in L_Hero. To get the alpine climb back,
  run `scripts/oc_biome.sh alpine` (about 12 minutes).
- **The chain is `scripts/oc_biome.sh <biome>`**: stop, generate, rebuild the ground, build
  the level, then the standard daylight look with the preset's `"POST"` keys. Run it as one
  background job.

To add a biome: a `SHAPES` row, a `PRESETS` row, one chain, then tune with `oc.py look`.

## Where each biome stands

All four use the same flank and path. Stills: the storybook, or re-run the chain.

- **alpine** (the hero climb): the reference build; see `docs/climb-plan.md`. Every
  overnight chain ends by rebuilding it, so L_Hero should hold the alpine flank (check
  `world_gen/climb/info.json`: `"biome": "alpine"`, `"window": null`).
- **desert** (built; storybook img29): reads as desert at `ev=-0.4`. Pale sand with mud
  pans, straw tufts, red sandstone, a few cacti. Wrong: the far slopes show one blotchy
  patch pattern (the 18 m mud patches tile visibly from far off: fade the mix with distance
  or make it larger); cacti are few and small; the rock is purple-grey, not red; no mesas
  or canyon walls (the flank is still the alpine one); the fence and bench are alpine props.
- **coast** (built twice; storybook img30): a turquoise bay below the path, sand with grass
  patches, marram and palms. Wrong: the sea (`climb_world.sea_material`) is a flat tinted
  sheet with no waves, foam or depth fade, and reads brighter than its dark constants
  because it mirrors the sky; the grass is lush lawn (the kept "Grass" layer and its blade
  rows: use drier, sparser rows); the trees are beech (palms are only GRASS rows, so they
  have no far LOD and cull at about 100 m); the hillside behind is bare sand-and-rock.
- **meadow** (built; storybook img31): reads as a meadow at the standard look. A green
  grass carpet to the path's edge, beech groves, the bench under a lone tree. Wrong: the
  steep flank above (the "Rock" and bank layers) is bare, pale grey-pink and washes out to
  white in the sun, so the valley looks like scree, not pasture: paint grass up the steep
  ground in this preset, or darken those layers; the grass is a little lime in full sun;
  no flowers (the user's flower scans are still to fetch); the alpine fence and snowy
  peaks are the flank's.

**Windows: the stretch on the plan's own ground (10 Nov, night)**

`OC_WINDOW=<window> scripts/oc_biome.sh <biome>` builds L_Hero from a window cut by
`make_window.py` instead of the hand-made flank: its ground at world heights (metres above
sea level) and its graded road. `make_climb.WINDOW_SHAPES` overrides a biome's shape for a
window (the mountain's treeline at 820 m, snowline 900 m, a fifth of the trees; the coast's
sea at 0 m). `OC_CLIMB_OUT=<dir>` writes a trial elsewhere, safe while Unreal builds. Cut
so far: mountain, meadow, desert, forest, coast.

- **meadow window** (built; storybook img32): open rolling grassland with beech copses,
  the path straight across it. Reads right. Wrong: the horizon is the window's edge (flat
  ground meets the sky); from above the far ground is washed out; the bench sits on a bare
  bank.
- **mountain window** (built as alpine; storybook img33): 6.6 km of road, 704 to 972 m,
  13 fence runs, 10,015 spruce. Larch-like spruce on green slopes, heath and rock higher,
  snow at the top. Wrong: the top kilometre is the plan's 67 m fill, a causeway along a
  snowfield (carve the road into the plan instead); the snow is blown white; the fence
  zig-zagged across the road there (fixed in `rails`, not yet rebuilt); the horizon is the
  window's edge.
- **coast window** (built: see below; storybook img34): a real shoreline, the ring road
  crossing a sandy point by a bay; 20% of the window is sea.
- **Not windowed yet:** desert (cut, not built), forest (cut; no forest preset: try alpine
  with `"treeline": NONE` or a new preset), heath and town (not cut).
- **What all windows need next:** a far horizon (a low-resolution ring of the plan's
  heights around the window, or a distant-terrain mesh, in place of the flank's rim of
  ridges); the stills' `hairpin_above` view is placed by the road's midpoint and points
  at a fifth of the way, which on a long window is often a dull view.

**Lessons from the first builds**

- **Exposure is per biome.** The standard look is exposed for dark greens; sand blew out to
  white whatever its tint. A preset's `"POST"` keys are laid over the look by
  `scripts/oc_biome.sh`. Passing the standard look by hand resets it to alpine's.
- **False colour first.** The white desert looked like snow; `look --tint` per layer showed
  it was the sand layer, and the weights (`world_gen/climb/*.r8`) showed no snow at all.
- **The engine's plane is 1 m.** The first sea was 60 m across.

## What to do next (suggested order)

1. Tune each built biome with `look` (tints first, then densities), judging full-size crops
   against a reference photo per biome. There are no reference photos for desert or coast
   yet: ask the user for two or three each, as with the Skyrim shots.
2. Give each biome its own landform instead of the alpine flank: cut the 2 km window named in
   `world.json` → `windows` from the world plan's heights (upsample, add `make_climb`'s
   detail noise, carve that ride's road through it). This joins the two halves.
3. Trees per biome. Only spruce and beech are baked (`oc.py static-trees`). Palms and cacti
   are small scans placed as grass rows for now. Other PVE species, or new scans the user
   fetches (never buy on Fab), need the same bake.
4. Town: paving, kerbs, barriers and a sign exist (`fab_prep` urban); there are no
   buildings. It needs assets before a preset is worth building.
5. PCG. 708 PCG classes are exposed to Python (`out/qa/probe_pcg.py`): graphs can be built
   with `PCGGraph.add_node_of_type/add_edge`, with SurfaceSampler, StaticMeshSpawner,
   GetLandscape, SelfPruning, Difference and SpatialNoise. No PCGBiomeCore classes are
   exposed. So far the ecology rules (altitude, slope, noise, distance from the road) live in
   the numpy generator, which is fast and can't crash the editor; PCG is the route to
   placing at world scale at runtime, once the rules are settled on two or three biomes.
6. Streaming the ride: a corridor of landscape along a road, tiles loaded ahead of the
   rider. Nothing exists for this yet; the island level's World Partition setup in
   `island_world.py` is the nearest thing.
7. Performance (30 fps at Low on the Radeon 890M) is still deferred by the user.
