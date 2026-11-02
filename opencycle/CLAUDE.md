# OpenCycle — Claude Notes

A free, open cycling sim in Godot 4.7. Forward+. `sim/` decides what is where, `game/`
draws it, and `replays/` ride it. Built from `templates/godot-template`, with the same
tooling; the dice game and Steam are gone.

## Commands

- First run: `scripts/setup.sh`.
- Tests: `scripts/test.sh` (gdUnit4 under Xvfb, on GL because GPU drivers won't present
  to Xvfb). Static: `scripts/lint.sh`. After hand-editing a `.tscn`: `scripts/resave.sh`.
- Look at it: `godot --path . --audio-driver Dummy -- --biome=neon --still=out.png --at=8`,
  then read the PNG. It opens a window on the desktop; use a real display, since Xvfb
  cannot run Vulkan.
- Keep a record of it: `scripts/storybook.py add "Title" --note "..."` renders the usual shots
  into `captures/storybook`; `scripts/storybook.py serve` shows them at localhost:8000.
- Measure it: `scripts/bench.sh forest ship [--rate=4 --scale=0.5]`. `--rate=4` rides four
  times as fast for a quick read; only trust a rate 1 run, the default.
- Rebake cards after changing their source or lighting: `scripts/bake.sh`.

## Keeping the expensive work cheap

Use a Sonnet subagent (`Agent` with `model: sonnet`) for work that eats tokens but needs little
context: QA of stills and clips, comparing batches of renders, and long log or transcript searches.
Tell it the shots, what to look for and what to report back, and keep design decisions here.

## The shape of the thing

`sim/` is pure and `tests/sim/test_purity.gd` enforces it. All randomness is
`Rng.derive`/`Rng.unit`, so one seed always gives the same road, ground, plants and street.
`Route` is the road, `Terrain` the ground around it, `Planting` and `Street` what stands
on it, and `Replay` turns time into a distance and a camera pose.

A biome is a `build(parent, run_seed, route, quality)` in `game/biomes/`, and a line in
`game/ride.gd`. It owns its sky and lights. The rider never leaves the road, so level of
detail is fixed at build time by distance from the road wherever that is possible.

## Gotchas, each of which cost time

- FSR2 posterises the HDR sky into bands. `ship` uses FSR1 with TAA.
- Trees are cards with normals leaning out from the trunk, drawn by
  `game/foliage.gdshader`. `StandardMaterial3D` flips a card's normal on its far side,
  which lights trees in hard two-tone stripes.
- Godot's front faces wind clockwise. A ribbon wound the other way just isn't there.
- Don't multiply a coloured ground texture by a colour: brown times brown reads as clay.
  `game/ground.gdshader` takes only brightness from the texture.
- A class named `Noise` hides the native one, hence `ValueNoise`. `seed` shadows a global,
  hence `run_seed`.
- `assets/baked/*.webp` are in LFS, because the slices pass the 500 KB commit hook limit.
- Godot exits 0 on parse errors. `scripts/check.sh` is the verdict.
- Only source `scripts/env.sh` in a subshell. In the shell that runs `git commit`, it
  redirects the hook cache and the git-orchard-sync hook fails on `go install`.
- `shader_type fog` goes on a `ShaderMaterial`. `FogMaterial` has no `shader`.
- A quad whose vertex shader pins it to the screen needs `cull_disabled`, or it can vanish.
- The gdformat pre-commit hook fails a commit if it reformats anything. Format first, and
  check `git log` afterwards.
- A bench in a window on a hidden workspace is throttled, and the numbers mean nothing.
  `docs/performance.md` says how to bench without that.
- Thick air is the neon street's easiest way to look worse: it lifts the blacks and smears
  the road's mirror, which reads the screen after the fog is drawn. Keep fog thin.

- After rebuilding a `.glb`, `godot --headless --import` can keep the old import. Delete its
  files in `.godot/imported/` first, then check the `.scn` is newer than the `.glb`.
- A Godot still can hang on quit after writing its PNG; wrap it in `timeout`. Rain hides most
  of a surface, so inspect with `--off=rain`.
- pre-commit stashes unstaged changes while a commit runs anywhere in this monorepo, so a
  still taken during one shows the last commit. Commit before a batch of stills.
- A new `class_name` isn't known until `godot --headless --path . --import`. Until then a
  windowed run stalls on the parse error instead of exiting, so a bench just hangs.
- A function in a `.gdshaderinc` can't name a parameter after a uniform or varying in a shader
  that includes it ("Redefinition of"). `game/road_wear.gdshaderinc` is shared this way.
- Other load on the machine slows the GPU, because they share a power budget. A bench slowdown
  that moves between runs is probably that, not the route.

## Modelled assets

`scripts/blender/` builds meshes headless: `blender -b --python scripts/blender/housing_bay.py`
writes `assets/modelled/housing_bay.glb`, in LFS. `shapes.py` bakes occlusion and room light into
vertex colours, and `Kit.restyle` swaps in the game's materials by name. Modelled concrete
uses `modelled` in `game/brutal.gdshader`, because the world-space mapping that suits flat
walls turns undulating surfaces into crumpled foil.

Blender suits repeating city pieces; one-off landmarks come from AI services. `assets/generated/`
holds those, brought to scale by `scripts/blender/generated.py`, and its README records each
one's source, prompt and terms. Add to it for every new model.

## The neon street

The bar is `docs/realism-standard.md`: it should feel real, with art-directed lighting and
interesting buildings, not match a photo. `docs/performance.md` has the numbers and the TODO, `game/city/lamp_reach.gd` limits which lamps are lit and which cast shadows as the rider moves. Pedestrians aren't built on the courses yet: `Pedestrians` still takes one whole street.

`Street`, `Skyline` and `Undercity` in `sim/` decide what stands where; `game/city/` builds
it, with `Facades` for the brutalist buildings and the close lights that rake up them
(`game/brutal.gdshader` paints the formwork and the stains), `Landmarks` for bridges and rail, `Overhead` for the deck, its pylons and
the traffic under it, `StreetWall` for the shop level (lintels, piers, shutters, boarded
fronts, doors), `Shopfronts` and `ShopFittings` for the rooms behind open fronts, `StreetDressing` for what collects on it (canopies, air conditioners, cable,
bills, spray, rubbish), and `Weather` for mist, steam and rain. `Districts` in `sim/` splits the ride into market, underpass, works, plaza, cistern, retail, lanterns, yard and residential, and `Quarters` builds what's particular to each; `Districts.sky_open` marks where the deck stops and `game/night_sky.gdshader` shows; a ride too short for a stretch stays market. Stills take
`--look=YAW,PITCH,SIDEWAYS` to inspect a wall up close, and `--at` is seconds along the replay,
which on a course carry on round the loop, not metres. Anything animated
runs off the ride's clock: the `ride_time` shader global, or `set_ride_time` on the
`ride_clock` group. `game/wet_sheen.gdshader` is an additive second pass over the road,
which mirrors the street by reading the screen.

`CityPlan` lays out the city as two long parallel streets joined by hairpins, and a `Course` (`--course=inner|medium|full`) is a loop of those roads, one `CitySegment` after another. A road is seeded by its own name, so it's the same street on every course; each one has its own `Route`, `Street`, `Skyline` and `Undercity`, in its own distances from 0 to its length, and its own order of districts. A road only knows about itself, apart from the `RoadMap` that keeps towers off the others. Roads are market where they meet, and level, so a seam has nothing much to give it away. `--at` is seconds round the course, so anything aimed at one spot depends on the course.

Four junctions join them: forks at the starts of N2 and N3, merges at the ends of S2 and S3. Each road there holds a `Fork`, keeps its ground and deck to its own side of the bisector, and keeps its lots out of it, so nothing overlaps. `Junctions` builds the wedge building between them, from the one road marked `draws`, and `CityStream` builds `NOT_TAKEN` sections of a road the course doesn't ride, ending in a wall in the fog. Roads run dead level and sway-free for `Route.SWAY_FLAT` at each end, so they part cleanly.

`CityStream` builds the street a section at a time as the rider comes to it, on a worker thread
during a ride and synchronously for stills and movies. The sim still lays out the whole route, and
each builder takes a `StreetSection`: it walks everything, keeps its dice in step, and makes only
what `section.holds(s)`. A long piece goes through `section.takes(from, to)`, which keeps its section
loaded until the rider is past the far end. Anything shared between sections (materials, kits) goes
through `Once`, and a `Kit` clears itself on commit so it can serve the next.
