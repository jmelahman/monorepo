# Performance: where it stands

Everything here was measured on the neon street's 1 km route, at 1920x1080 on a Radeon 890M.

## TODO

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

`--off=shadows,aa,fog,ssao,glow,probes` leaves any of those out, and `--scale=0.67` renders
at a lower resolution. Both work on a ride, a still or a bench.

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
