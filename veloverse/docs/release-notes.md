# Veloverse 0.2.0 preview

Veloverse is a free, open cycling sim. This build has one ride, **The Ring**: a closed 165.5 km
loop round a generated island, through coast, forest, heath, mountain and desert. It is still a
preview. The world is complete end to end, but much of it is a first pass.

## Installing

You need Linux x86-64 with a Vulkan GPU. The build is a tarball, `veloverse-linux-<date>.tar.gz`.

```sh
tar -xzf veloverse-linux-<date>.tar.gz          # unpacks to package_snap/Linux
cd package_snap/Linux && ./VeloverseUE.sh
```

The game folder can sit anywhere, and you can delete it to uninstall. Your profile, rides and
segment bests are kept in `~/.config/Veloverse`, so they survive a new build. Delete that folder
to start over.

Extra arguments pass through to the game, for example:

- `./VeloverseUE.sh -RideFrom=Mountain` starts at the mountain.
- `-RideFtp=230` sets your FTP for one ride.

## Riding

- **W / Up** adds power and **S / Down** takes it away.
- **Esc, P or Space** pauses.
- In a workout, **[** and **]** make it easier or harder by 5 %, **N** skips a step, and **E**
  turns ERG on and off.

The start page sets up a ride:

- **Power** comes from the keyboard, or from a simulated smart trainer that behaves like a real
  one, ERG included. **Bluetooth** (experimental) connects a smart trainer or power meter, and a
  heart-rate strap, through BlueZ; pick the trainer under "Trainer", or start with
  `-RideTrainer=ble`. It needs BlueZ and libdbus, and hasn't been tried on many trainers yet.
- **Route preview:** the route's profile, its timed climbs, and where you start ("Start in": any
  stretch of the ring).
- **Workouts:** pick one, or just ride.
- **Your level** and XP are shown.

Video settings are on the start and pause menus: an overall quality, each scalability group,
render scale, a frame-rate cap, VSync and the window.

## New in 0.2.0

- **Profile and ride log.** Set your FTP, weight and bike weight; the first run asks for them.
  - Every ride is saved with its summary: time, distance, climbing, average and normalised power,
    TSS, and best efforts.
  - The profile page lists recent rides and the past week's load.
  - Each ride is also written as a `.fit` file in `~/.config/Veloverse/Rides`, ready to upload to
    Strava or Garmin Connect. It is placed at Point Nemo, since The Ring isn't on a real map.
- **Workouts with ERG.**
  - There are eight built in: sweet spot, VO2 max, threshold, over-unders, an endurance hour,
    sprints, a ramp test and a recovery spin.
  - Your own `.zwo`, `.erg` or `.mrc` files can go in `~/.config/Veloverse/Workouts`.
  - A HUD bar shows the blocks, the target, the time left in the step and what comes next. ERG
    holds the target on a trainer that supports it.
- **Segments.** The Ring's seven climbs are timed segments.
  - A banner shows the next one coming up.
  - On a climb, the banner shows your time and your gap to your best (the ghost).
  - At the top, it shows your result: a new best, or where the effort ranks.
  - The ride summary lists every segment you rode.
- **Points and levels.** Rides earn XP:
  - for distance and climbing;
  - for each segment, with more for a new best;
  - for a finished workout.

  XP adds up to levels, and levels unlock HUD colours, which you pick on the profile page.
- The start page shows the version, so you can quote it in bug reports.

## Known faults

- Bluetooth is experimental. It has been tested against a simulated trainer, not yet a real one.
- The `.fit` file leaves out paused time. A ride paused halfway shows as one continuous effort.
- Segment bests are tied to the route as generated. A regenerated route starts them over.
- The landscape is 14 m a sample, so far slopes are coarse.
- Coast water is flat, and a precision warning about the sea plane may show in the log.
- Performance is 32–44 fps at 1080p on a Radeon 890M (integrated) at the default settings. Low is
  faster. There is no tuning yet.
