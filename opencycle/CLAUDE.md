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
- Measure it: `scripts/bench.sh forest ship [--rate=4 --scale=0.5]`. `--rate=4` rides four
  times as fast for a quick read; only trust a rate 1 run, the default.
- Rebake tree cards after changing their source or lighting: `scripts/bake.sh`.

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
