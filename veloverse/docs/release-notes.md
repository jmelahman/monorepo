# Veloverse preview

A first look at Veloverse, a free, open cycling sim. This build has one ride: **The Ring**, a
closed 165.5 km loop round a generated island. It passes coast, forest, heath, mountain and
desert. It is a preview: the world is complete end to end, but most of it is a first pass.

## Running it

Linux x86-64 with a Vulkan GPU. Unpack it and run `./VeloverseUE.sh` in the unpacked folder.
Extra arguments pass through, for example `./VeloverseUE.sh -RideStart=102850` to start at the
mountain or `-RideFtp=230` to set your FTP for the power zones.

## Controls

- **W / Up** adds power, **S / Down** takes it away (there is no trainer support in this build).
- **Esc, P or Space** pauses. The start and pause menus hold the video settings: an overall
  quality, each scalability group, render scale, a frame-rate cap, VSync and the window.

## Known faults

- Grass grows on the paving of the town stretches, and on the road's edge in a few places.
- The landscape is 14 m a sample, so far slopes are coarse.
- Coast water is flat, and a precision warning about the sea plane may show in the log.
- Performance: about 20 fps at Epic on a Radeon 890M (integrated). Low is faster; there is
  no tuning yet.
