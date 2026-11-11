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

`world_gen/world/preview.png` is the map; `world.json` has the numbers below.

| Ride | km | Climb m | Hours* | Mostly |
| --- | --- | --- | --- | --- |
| island (the ring) | 134.6 | 1371 | 4.8 | coast, desert, forest, town, heath, mountain |
| forest | 61.6 | 812 | 2.3 | forest, west cliffs |
| coast | 49.3 | 225 | 1.6 | south beaches, town, farmland |
| desert | 47.7 | 299 | 1.6 | desert, mesas |
| meadow | 39.2 | 167 | 1.3 | farmland |
| dunes | 37.5 | 250 | 1.3 | desert, harbour |
| summit | 26.6 (there and back) | 714 | 1.25 | mountain, to 863 m |

*At 30 km/h on the flat, slower uphill (`hours` in make_world.py).

Regions (centres in world km, X east, Y south): mountain in the north, heath north-west,
forest west, meadow in the middle, desert east, town and harbour south-east, beaches along
the south, cliffs west and north. `REGIONS`, `ROADS` and `ROUTES` are `make_island.py`'s,
times `K = 7`; make_world adds the heath, mid-scale hills (`RELIEF`) and a wander on every
road (`WANDER`).

Known gaps in the plan:

- The ring is 135 km, a little under the 5 to 6 hours asked for. Raise `K` to 8, or add
  wander, or route the ring out to the headlands.
- The summit climb finds only one switchback and tops out at 863 m of a 1076 m mountain. The
  pathfinder's window and costs (`climb`) were scaled from the island's, not tuned.
- Rides join their legs with gaps of up to about 100 m (the "widest gap" the generator
  prints): a leg's end snaps to the nearest road point but the junction isn't shared tarmac.
- Roads are graded (under 8%) but not carved into the height map, and nothing checks a road
  against a mesa or the canyon.
- Regions are still round blobs. For Skyrim-like asymmetry, give each a ridge, valley or
  river that the road must go round; add landmarks to ride toward.

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
