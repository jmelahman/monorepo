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
  run the same chain with `OC_BIOME=alpine` (about 12 minutes).

To add a biome: a `SHAPES` row, a `PRESETS` row, one chain, then tune with `oc.py look`.

## Where each biome stands

BIOME_STATUS

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
