# Veloverse preview

A first look at Veloverse, a free, open cycling sim. This build has one ride: **The Ring**, a
closed 165.5 km loop round a generated island. It passes coast, forest, heath, mountain and
desert. It is a preview: the world is complete end to end, but most of it is a first pass.

## Running it

Linux x86-64 with a Vulkan GPU. Unpack it and run `./VeloverseUE.sh` in the unpacked folder.
Extra arguments pass through, for example `./VeloverseUE.sh -RideFrom=Mountain` to start at the
mountain or `-RideFtp=230` to set your FTP for the power zones.

## Controls

- **W / Up** adds power, **S / Down** takes it away (there is no trainer support in this build).
- **Esc, P or Space** pauses. The start and pause menus hold the video settings: an overall
  quality, each scalability group, render scale, a frame-rate cap, VSync and the window.

## Changes since v0.1.0

- No more grass tufts through the tarmac: the grass-free verge now reaches the road everywhere.
- Desert and beach plants no longer grow on the road either; in dry country its verge is sand,
  not dirt.
- Pick where to start on the start screen ("Start in": any stretch of the ring), or with
  `-RideFrom=Desert`. The ride now starts on the coast past the harbour by default.
- The HUD names the stretch you're in correctly; it was off by one stretch all the way round.
- Under the mountain, the lighting was clipped and a red engine warning showed across the screen.

## Known faults

- The landscape is 14 m a sample, so far slopes are coarse.
- Coast water is flat, and a precision warning about the sea plane may show in the log.
- Performance: 32–44 fps at 1080p on a Radeon 890M (integrated) at the default settings; Low is
  faster. There is no tuning yet.
