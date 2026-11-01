# Performance: where it stands

Deferred on purpose while the look and the content are worked out. Everything here was
measured on the neon street, at 1920x1080 on a Radeon 890M, before the route got longer.

## TODO

- Reach 33 ms a frame on the 890M. It is about 100 to 125 ms now, around 9 fps.
- Volumetric fog is the first thing to look at: about half the frame.
- Measure what the 51 shadowed spotlights cost. A run with their shadows off was the
  cheapest seen, but in a throttled window, so that number can't be trusted.
- Check the ride in motion for shimmer on shutters and balusters.

## Measured

| Left out (`--off=`) | GPU ms, median | Average fps |
| --- | --- | --- |
| nothing | 102, 125 | 9.1, 8.3 |
| `fog` | 50 | 16.6 |
| `ssao` | 87 | 10.9 |
| `glow` | 91 | 10.2 |

The two baselines are separate runs, so differences under about 20 ms mean little.

Scanned props are now capped at a set detail (`Kit.lean`). That took the scene from 10.3M
to 2.7M triangles and barely moved the frame rate, so triangles aren't what's limiting it.

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

The report's `gpu_ms_median` is what the GPU spent, whatever the window was doing.
