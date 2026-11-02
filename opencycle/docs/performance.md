# Performance: where it stands

Everything here was measured on the neon street's 1 km route, at 1920x1080 on a Radeon 890M.

## Video presets

The inner loop at rate 1, in a private compositor with little else running (`--video=NAME`). High
is from a rate 4 run, so it's a rough read; the other two moved only a little between the rates.

| Preset | GPU ms, median | GPU ms, 99th | Average fps | 1% low | Video memory |
| --- | --- | --- | --- | --- | --- |
| Low | 19.7 | 59 | 38 | 10 | 1099 MB |
| Medium | 37.2 | 74 | 22 | 8 | 1140 MB |
| High, rate 4 | 73 | 163 | 11 | 7 | 1400 MB |

Low is the default, because it's the only one that holds the 30 fps cap on most of the ride. Even
it dips: its slowest 100 m takes 54 ms, around 400 m along. The 1% low and a worst frame of 150 ms
hardly change with the preset, so those aren't the GPU's: the main thread is stalling, probably as
each section arrives, about every 11.5 s. That isn't confirmed yet, and it's the next thing to fix.

Turning shadows off also hides the sky's light, because without its shadow it comes through the
deck. The figures below are older, from the 1 km route.

## TODO

- Find what stalls the main thread for 100 ms and more during a ride, on every preset.
- Reach 33 ms a frame on the 890M. The last market stretch is about 58 ms now, around 16 fps.
- Whole-ride figures at full resolution, on a quiet machine: about 65 to 80 ms over the first
  300 m and 50 to 60 ms over the last 300 m. The stretch between has no clean full-resolution
  run yet. At `--scale=0.75` it was 61 to 68 ms.
- About 31 ms of the frame doesn't change with resolution, so resolution alone can't reach the
  target. Lamp shadows are about half of that.
- Check the ride in motion for shimmer on shutters and balusters.

## Measured

The last market stretch (`--at=112`), one run each:

| Change | GPU ms, median | Saved |
| --- | --- | --- |
| nothing | 57.7 | |
| `--off=shadows` | 42.7 | 15 |
| `--off=ssao` | 51.6 | 6 |
| `--off=aa` | 51.9 | 6 |
| `--off=fog` | 56.8 | 1 |
| `--off=glow` | 58.7 | none |
| `--scale=0.75` | 47.2 | 10.5 |
| `--scale=0.5` | 37.7 | 20 |

Before `LampReach` and the thinner fog grid, the same stretch took 101 ms, with lamp shadows
about 35 ms and fog about 40 ms. `LampReach` keeps shadows on lamps within 35 m of the rider
and turns lamps off beyond 170 m. Widening it to 50 m cost 5 ms; narrowing the lit range to
110 m saved nothing. TAA makes lamp shadows redraw every frame; the reason isn't understood.

Scanned props are capped at a set detail (`Kit.lean`). That took the scene from 10.3M to 2.7M
triangles and barely moved the frame rate, so triangles aren't what's limiting it.

## Running faster

The video options on the menu and the pause screen are what a player uses. For tooling,
`--video=Low|Medium|High` picks a preset, `--off=shadows,aa,fog,ssao,glow,probes,rain` leaves any of
those out, and `--scale=0.67` renders at a lower resolution. All work on a ride, a still or a bench.

## Measuring

A window on a workspace nobody is looking at gets throttled to about 7.5 fps, whatever the
GPU could do. Bench with the window on screen, or inside a private compositor, which shows
nothing and isn't throttled. Sway needs a short `XDG_RUNTIME_DIR` for its socket:

```sh
mkdir -p -m 700 /run/user/$UID/och
env -u SWAYSOCK -u WAYLAND_DISPLAY -u DISPLAY XDG_RUNTIME_DIR=/run/user/$UID/och \
	WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1 sway -c /dev/null &
env -u DISPLAY XDG_RUNTIME_DIR=/run/user/$UID/och WAYLAND_DISPLAY=wayland-1 \
	godot --path . --audio-driver Dummy --display-driver wayland -- \
	--biome=neon --quality=ship --bench=captures/bench.json
```

The report's `gpu_ms_median` is what the GPU spent, whatever the window was doing, and
`gpu_ms_median_by_stretch` splits it by 100 m of road.

Other work on the machine distorts this badly. The CPU and GPU share a power budget, so a
process using ten cores took the GPU from 2400 to 1400 MHz, and a stretch from about 65 to
130 ms. It looks like a slow part of the route, but it moves between runs. Check the machine
is quiet, and repeat any number that matters.
