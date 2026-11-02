# OpenCycle

A free, open cycling sim, starting with what makes one worth riding: the world.
Worlds are generated from a seed, and for now they are ridden by deterministic
replays, with no trainer and no avatars. The renderer reads only a position and a
camera pose, so a trainer or physics can drive it later.

So far it's a futuristic city at night, with three nested loops of 2.5, 6.3 and 14.5 km, and a
1 km conifer forest left over from the first experiments.

## Run it

Needs Godot 4.7, Git LFS and a Vulkan GPU.

```sh
scripts/setup.sh              # addons, CC0 assets, first import
godot --path .                # the menu: pick a loop, then ride
```

`Esc` pauses, and `H` hides the HUD. Video options are on the menu and the pause screen, and are
kept in `user://video.cfg`.

Anything after `--` skips the menu and goes straight to the ride, for tooling: `--biome=forest|neon`,
`--course=inner|medium|full`, `--seed=N`, `--quality=ship|max`, `--replay=res://replays/x.json`,
`--still=out.png --at=SECONDS`, `--video=Low|Medium|High`, `--off=shadows,aa,fog,ssao,glow,probes,rain`,
`--scale=0.5`.

- `scripts/bench.sh [biome] [quality]`: frame times to `captures/`, on your real GPU
- `scripts/capture.sh [biome] [quality] [fps]`: an mp4 at a fixed frame rate, 60 unless given,
  through Movie Maker and ffmpeg
- `scripts/test.sh`, `scripts/lint.sh`: tests and static checks

## Frame rate

Radeon 890M at 1080p, over the inner loop, with a ride capped at 30 fps by default. Low, the default,
averages 38 fps uncapped, Medium 22 and High about 11. It all still stutters now and then;
`docs/performance.md` has the detail.

## Art

Everything from [Poly Haven](https://polyhaven.com) is CC0, fetched by
`scripts/fetch_assets.py` at pinned hashes and left out of git. The tree cards in
`assets/baked/` are rendered from its models by `scripts/bake.sh` and kept in Git LFS.
