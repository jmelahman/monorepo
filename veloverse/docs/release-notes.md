# Veloverse 0.2.0 preview

Veloverse is a free, open cycling sim. Its world is five islands: **Harbour Isle** in the middle,
and four more round it, one for each biome, each joined to Harbour Isle by two bridges. It is
still a preview. The world is complete end to end, but much of it is a first pass.

- **Harbour Isle:** the coast, and a harbour town with boats, piers and a lighthouse. A coast
  road rings it, and every route starts on the quay.
- **Greenwood** (southwest): forest.
- **Vael** (northwest): the mountain, topping out at about 450 m.
- **The Moor** (northeast): open heath, with tors and a wind farm.
- **Red Sands** (southeast): desert, with mesas and hoodoos.

## Routes

Because every island hangs off Harbour Isle, a route can visit any of them without the others,
and none rides a road twice. Pick one under "Route" on the start page.

| Route | km | Climb m | Islands |
| --- | --- | --- | --- |
| The Grand Tour | 84.1 | 1495 | all four (the default) |
| Harbour Isle | 27.1 | 174 | the coast road alone |
| Greenwood Loop | 40.8 | 459 | Greenwood |
| Vael Loop | 39.5 | 612 | Vael |
| Moor Loop | 41.1 | 532 | The Moor |
| Red Sands Loop | 44.0 | 414 | Red Sands |
| Woods and Peaks | 53.2 | 897 | Greenwood, Vael |
| The Highlands | 53.5 | 970 | Vael, The Moor |
| Heath and Sand | 57.9 | 772 | The Moor, Red Sands |

Each route's climbs are timed separately, so a best on one doesn't count on another. Bests from
0.2.0's Island Loop stay in your history but aren't on any of these routes.

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

- `./VeloverseUE.sh -RideRoute="Woods and Peaks"` picks a route by its name.
- `-RideFrom=Vael` starts on Vael. Any island on the route works, or "The Coast".
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
  island on the route).
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
    Strava or Garmin Connect. It is placed at Point Nemo, since the islands aren't on a real map.
- **Workouts with ERG.**
  - There are eight built in: sweet spot, VO2 max, threshold, over-unders, an endurance hour,
    sprints, a ramp test and a recovery spin.
  - Your own `.zwo`, `.erg` or `.mrc` files can go in `~/.config/Veloverse/Workouts`.
  - A HUD bar shows the blocks, the target, the time left in the step and what comes next. ERG
    holds the target on a trainer that supports it.
- **Segments.** Each route's climbs are timed segments.
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
- Segment bests are tied to the route as generated. The new world started them over, so bests
  from The Ring don't carry across.
- The landscape is 7 m a sample, so far slopes are still a little coarse.
- Coast water is flat, and a precision warning about the sea plane may show in the log.
- Performance is 32–44 fps at 1080p on a Radeon 890M (integrated) at the default settings. Low is
  faster. There is no tuning yet.
