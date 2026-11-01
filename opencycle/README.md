# OpenCycle

A free, open cycling sim, starting with what makes one worth riding: the world.
Worlds are generated from a seed, and for now they are ridden by deterministic
replays, with no trainer and no avatars. The renderer reads only a position and a
camera pose, so a trainer or physics can drive it later.

Two biomes so far: a 1 km conifer forest and a 200 m cyberpunk street.

## Run it

Needs Godot 4.7, Git LFS and a Vulkan GPU.

```sh
scripts/setup.sh              # addons, CC0 assets, first import
godot --path .                # ride the forest
godot --path . -- --biome=neon
```

Options after `--`: `--biome=forest|neon`, `--seed=N`, `--quality=ship|max`,
`--replay=res://replays/x.json`, `--still=out.png --at=SECONDS`, `--scale=0.5`.

- `scripts/bench.sh [biome] [quality]`: frame times to `captures/`, on your real GPU
- `scripts/capture.sh [biome] [quality] [fps]`: an mp4 at a fixed frame rate, 60 unless given,
  through Movie Maker and ffmpeg
- `scripts/test.sh`, `scripts/lint.sh`: tests and static checks

## Frame rate

Radeon 890M, 1080p output, `ship` quality, over a whole replay:

| Biome  | Render scale | Average | 1% low | Worst frame | Video memory |
| ------ | ------------ | ------- | ------ | ----------- | ------------ |
| Forest | 0.67         | 53 fps  | 32 fps | 38 ms       | 480 MB       |
| Forest | 0.5          | 57 fps  | 36 fps | 34 ms       | 455 MB       |
| Street | 0.67         | 67 fps  | 43 fps | 26 ms       | 212 MB       |
| Street | 0.5          | 78 fps  | 49 fps | 24 ms       | 171 MB       |

Neither holds a steady 60, so a ride is capped at 30 fps.

## Art

Everything from [Poly Haven](https://polyhaven.com) is CC0, fetched by
`scripts/fetch_assets.py` at pinned hashes and left out of git. The tree cards in
`assets/baked/` are rendered from its models by `scripts/bake.sh` and kept in Git LFS.
