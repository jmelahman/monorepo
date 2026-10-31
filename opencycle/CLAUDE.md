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
- Thick air is the neon street's easiest way to look worse: it lifts the blacks and smears
  the road's mirror, which reads the screen after the fog is drawn. Keep fog thin.

## The neon street

The bar is `docs/realism-standard.md`: it should feel real, with art-directed lighting and
interesting buildings, not match a photo. Performance is deliberately deferred while the
look is worked out, and `NeonBiome.CROWD` keeps the pedestrians off to speed up iteration.

`Street`, `Skyline` and `Undercity` in `sim/` decide what stands where; `game/city/` builds
it, with `Facades` for the brutalist buildings and the close lights that rake up them
(`game/brutal.gdshader` paints the formwork and the stains), `Landmarks` for bridges and rail, `Overhead` for the deck, its pylons and
the traffic under it, `StreetWall` for the shop level (lintels, piers, shutters, boarded
fronts, doors), `StreetDressing` for what collects on it (canopies, air conditioners, cable,
bills, spray, rubbish), and `Weather` for mist, steam and rain. Stills take
`--look=YAW,PITCH,SIDEWAYS` to inspect a wall up close. Anything animated
runs off the ride's clock: the `ride_time` shader global, or `set_ride_time` on the
`ride_clock` group. `game/wet_sheen.gdshader` is an additive second pass over the road,
which mirrors the street by reading the screen.
