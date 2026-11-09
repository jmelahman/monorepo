# OpenCycle: the climb plan

This is the plan of record, written 2026-11-09 after the user's verdict on the island ("4 FPS, no
trees, every biome looks awful"). Read it at the start of every session. Update the "Where things
stand" section before you end one.

## Goal

**One realistic mountain climb that looks good and runs well.** Only after that come other places.

The climb starts in beech woods. It rises through spruce and larch to the treeline, then crosses
dwarf pine and juniper, and finishes in snow at a viewpoint. It is about 8 km at 5–8%.

What the island taught us: we built breadth first (seven routes, six biomes, a town) and none of it
reached a good look or a playable frame rate. From now on we finish one stretch before starting the
next.

## Target hardware (the user's decision)

| Tier | Machine | Target |
| --- | --- | --- |
| Floor | This laptop: Radeon 890M iGPU | **30 fps at Low** (`scalability 0`), 1080p output, upscaled from a lower internal resolution if needed |
| Full | A discrete GPU, RTX 4070 class and up | Epic: Lumen, Nanite, dense foliage, the "immersive" look |

Every milestone is measured on both tiers. A milestone isn't done until the laptop holds 30 fps at
Low on it. The discrete tier is measured on a rented Runpod GPU. Quote the price first, and ask the
user before any spend over $10.

## Quality bar

- **Reference photos.** Collect 10–15 real road-climb photos for each zone before building it (beech
  woods, spruce forest, treeline, snow). Save them in `/workspace/ue/ref/climb/` with their source
  URLs, and use only photos whose licence allows it.
- **QA.** A Sonnet subagent scores each still from 1 to 10 against the matching reference photos, and
  lists the five worst defects. Tell it that sky banding seen through Read is a downscale artifact.
- **Bar to pass a stretch.** Epic scores at least 7 with no "High" defect. Low scores at least 5,
  meaning it is clearly the same place, only simpler.
- **Show the user** before moving on whenever the look changes a lot. Put stills in the storybook
  (https://claude.ai/artifact/UuLeEWcskoabDCuZceuLqb).

## Measurements so far (2026-11-09)

All of these come from the same offscreen ride test (`scripts/ride_test.sh`) on the 890M.

| Level | Settings | fps | Notes |
| --- | --- | --- | --- |
| Scanned island (`/Game/Island`) | Epic, 720p | 3–4 | Megascans grass and props; no trees |
| Japanese town (`/Game/Coast`, 7 Nov) | Epic, 720p | 2.5–5.5 | Free Poly Haven assets |

Neither reaches 30 fps yet.

**A stylized look is out (the user's decision, 9 Nov).** The low-poly valley in `/Game/Stylized`
was only a probe. At Low on this laptop it rode at 70–95 fps (GPU 8–11 ms, `out/stylized_low.log`),
so the engine and game code leave plenty of room. The cost is in what we draw: grass cards,
scanned meshes, Lumen and VSM. Don't build on that level or its look.

## Assets we have

- **Standing trees, today.** Unreal's Procedural Vegetation plugin ships `PVE_Conifer_01` and
  `PVE_Deciduous_Tree_01` (in `/Game/Spike`, used by the 6 Nov forest spike, `out/local_v11`). They
  use Nanite foliage, so they need `r.Nanite.AllowAssemblies=1` and `r.Nanite.Foliage=1`. Never
  rebuild their meshes (that breaks them into cubes or paper cards). They aren't proven on Low yet.
- **Megascans, already downloaded** (`/workspace/ue/fab/MANIFEST.tsv`): grasses, rocks, cliffs, snow,
  ground surfaces, trunks, logs, stumps, dwarf birch. No canopy trees.
- **Fab trees, on request.** `/workspace/ue/fab/CLIMB_BRIEF.md` is the brief for a separate browser
  session that fetches spruce, larch, beech, pine, alder, birch and treeline shrubs. Prefer ones with
  LODs and billboards. They land in `fab/climb/` with a `SUMMARY.md`.

## Milestones

1. **Hero stretch, 1 km, Epic.** The beech-to-spruce transition at 6–7%:
   - a real road (asphalt, edge lines, gravel shoulder, guard rail on the drop side),
   - a cut slope above and a drop below,
   - trees from PVE now, swapped for Fab trees when they arrive,
   - grass and rocks only where the camera sees them.

   Build it as its own level (`/Game/Climb/L_Hero`). Iterate on stills until it passes the bar.
2. **Hero stretch on Low at 30 fps.** Use tree LODs and impostors, short cull distances for grass,
   no Lumen on Low (sky light plus a baked or distance-field AO fallback), virtual shadow maps off,
   and TSR from a lower internal resolution. Profile with `stat unit` / `stat gpu` / the CSV
   profiler. Add a Low-specific foliage density cvar if needed.
3. **The whole climb.** Repeat the hero recipe for the beech woods, the treeline and the snow summit,
   one zone at a time, each with its own references and QA. Then the full 8 km route, with its ride
   track, weather and start-screen entry.
4. **Packaged build.** A shipping build with cooked textures (the "Preparing Textures" stall goes
   away). Measure both tiers.
5. **Only then:** other places, reusing the recipe (coast road, valley farmland).

## Blockers and risks

- **Nanite foliage on the iGPU.** We don't know its cost at Low. Measure it in milestone 2 before
  relying on it; Fab trees with classic LODs are the fallback.
- **Memory.** An external killer acts at about 3 GB free. Run only one Unreal at a time. The 1 m
  island level didn't fit.
- **The editor sometimes hangs after logging its exit.** `ue_run.sh` and `ride_test.sh` handle it;
  never kill an Unreal yourself.

## Where things stand

- 2026-11-09, late night: the road is now a **3 m bike-only path** (no markings) with a timber
  fence. `oc.py build-level` rebuilds L_Hero in 5 minutes with no crash. `look` gained `--grass
  DENSITY CULL` (2.0 1.5 now) and `--fog DENSITY FALLOFF` (0.008 0.02 now; the template's falloff
  of 0.2 hid all fog above 100 m). A rim of ridges round the map hides its edge and the sea.
  Storybook version 7 has the stills. Open defects: smeared grass banks, pale tarmac and fence,
  the near rim reads as a bare cliff, trees only within 320 m of the path (static trees next).

- 2026-11-09, night: **the loop is `scripts/oc.py`** (run from /workspace/ue). It's one command
  that blocks until the job is done, failed or crashed, and says which on its last line.
  - Commands: `oc.py look --tint Dirt=r,g,b --grey Rock=0.75 [--shots road_72]`, then `stills`,
    `rebuild-ground`, `status`, `wait JOB`.
  - A look change with one still takes 16 s.
  - It starts its own supervised editor (`oc_supervisor.sh` runs `oc_server.py`).
  - `oc_climb.py` holds the climb's material and ops, with no island_world monkeypatching.
  - The `ue_serve` and `out/cmd` entry below is superseded.
  - Look set so far: the Dirt shoulders are tinted 0.62,0.63,0.65 with grey 0.85, Rock has
    grey 0.75, and Rock mix has grey 0.6.

- 2026-11-09, evening: the crash rules are enforced in code, not kept in memory.
  - Start the server with `nohup bash scripts/ue_serve.sh >out/serve_supervisor.log 2>&1 &`
    from /workspace/ue. When the editor dies, the supervisor marks the running command
    `.crashed` (log tail in `.crashed.txt`) and restarts on the empty map. It gives up after
    three exits in ten minutes. Each run's log is kept in `out/serve_logs/`.
  - The landscape uses `MI_ClimbGround`. Recolour it with `climb_world.tune({"Dirt": (r, g,
    b)})`: parameters only, no compile, safe with the level open.
  - `ground_material()` raises if a landscape is loaded. To rebuild, send `quit.py` (which also
    stops the supervisor), start `ue_serve.sh` again, then queue the rebuild.
  - The server holds commands while under 6 GB of memory is free.

- 2026-11-09, later: L_Hero is built (landscape, road, rails, 3,600 PVE trees). The loop is one
  warm editor (`scripts/ue_serve.py`) that runs command files dropped into `out/cmd`: a look
  change plus six stills takes about 2.5 minutes. Stills now come from the editor viewport, so
  grass and tree LODs load. The rock is now granite and the shoulders gravel.
  - **Crash rule:** never rebuild the ground material while the landscape is loaded. The editor
    recompiles every landscape piece in one frame, and the Radeon driver resets the GPU
    (DEVICE_LOST, twice). Build the material first, then open the level (`out/cmd/0005_look.py`).
  - Never reload L_Hero while it's open (segfault). `NO_SKELETAL` / `OC_NO_SKELETAL` leaves
    the skeletal trees unloaded.
  - The skeletal trees overflow the GPU skinning buffer (`r.Skinning.Buffers.*`). The PVE
    plugin can export static meshes (`EPVExportMeshType::StaticMesh`), but only from its
    editor UI, not from Python.

- 2026-11-09: Plan written. The user ruled out a stylized look; the stylized valley was only a
  frame-rate probe (see Measurements). Fab trees are
  being fetched by a separate session from `fab/CLIMB_BRIEF.md`, with results in
  `fab/climb/SUMMARY.md`. Next: milestone 1 with the PVE trees.
- 2026-11-09, later: milestone 1 started.
  - `make_climb.py` generates the hero stretch into `world_gen/climb`. It is 2 km of ground at 1 m
    and 1.2 km of road at 6% average with one hairpin, with about 20k beech and spruce stems and
    guard-rail runs.
  - `climb_world.py` builds `/Game/Climb/L_Hero` from that output. It isn't built yet.
  - Nine reference photos are in `/workspace/ue/ref/climb/`, with sources and licences in
    `SOURCES.tsv`.
  - The first Fab trees have landed in `fab/climb/`. They are single-LOD FBX files, with no spruce
    or beech yet.
  - Fixed an editor crash when a level has no ride track (it broke the `L_Forest` spike).
- **Only one Unreal at a time, across all sessions.** Two sessions rode at once on 9 Nov, and both
  wrote `/workspace/ue/ride_test.log`. Check `pgrep -a UnrealEditor` before starting one.
