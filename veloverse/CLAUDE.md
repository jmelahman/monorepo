# Veloverse: working notes for Claude

Read `docs/climb-plan.md` first. It is the plan of record, and its "Where things stand" section
says what was done last, what is open and what comes next. This file holds the rules and how
things work, not a version log: put each version's changes and status in the plan, and
update this file only when a rule or a mechanism changes.

## THE PRIMARY GOAL (12 Nov, the user: "make sure it is abundantly clear")

**An immersive world on the Epic settings that plays at 30 fps on mid- to high-end consumer
hardware.** Every decision serves that, in that order:

1. **Immersive at Epic first.** Judge the look at Epic, and judge the frame rate on a mid- to
   high-end consumer GPU, not on this laptop's 890M.
2. **30 fps at Epic is the only performance target.** Don't trade Epic visuals for speed beyond
   what that needs.
3. **Low-settings visuals and Low performance come later.** They are not current goals: don't
   cut density, draw distance or quality to help Low or the 890M, and don't plan work around
   "30 fps on Low".

Where anything below or in the docs says otherwise, this section wins.

## The goal (the user's decisions)

- **The goal:** an original cycling world (Zwift-like, not a copy) that looks **realistic**.
  "No stylized."
- **The look (10 Nov):** "vibrant yet realistic", like the modded Skyrim screenshots in
  `ref/climb/style_*.jpg`. That means rich green grass carpets with small flowers, warm brown dirt,
  big rounded boulders, red-brown pine trunks, deep shadows and moody skies with a snowy peak. Use
  them for mood and colour only; build nothing from that game.
- **Two more references (10 Nov):** `ref/climb/alpine_meadow_trail.jpg` (a real treeline trail:
  low mottled heath, rock slabs, scattered conifers, snow-streaked cliffs, soft daylight) and
  the Folkvangr tundra grass shot (knee-high, dense, no ground showing, in daylight). The user
  on our grass: "very patch and thin and this has volume and looks real."
- **Think big (10 Nov):** "If we need major changes to get the desired environment, I am for
  it. I am not attached to anything at present." The agreed plan has three parts: daylight
  (done, v32), grass made of real blades (done, v32), then placement by ecology with PCG.
- **High end first (10 Nov):** "optimize for the high-end visual cases -- low graphics can
  degrade the quality for the sake of frames."
- **The first target:** one realistic mountain climb. Build a 1 km hero stretch first, then the
  full climb, then other biomes.
- **Performance:** superseded by THE PRIMARY GOAL above (30 fps at Epic on mid- to high-end
  consumer hardware; Low and the 890M later). The history: the 890M at Low was the old target,
  with **visuals first, perf later**: keep trees at full quality (2 km cull, shadows, wind).
  Measured on 9 Nov at Low: about 16–19 fps with all trees and about 29 with none (v20 on
  10 Nov: 20–23 with all trees). Shadows
  off or a 300 m cull each give about 20–24 (`oc.py fps`). The v24 `fps --try` ablation: grass,
  Lumen, reflections or 67% screen each gain only 1–2 fps, and VSM off halves it. The cost is the
  trees' draw/CPU side, not pixels.
- **Bike path:** the climb's road is a **bike-only path**, about 3 m of tarmac with no markings,
  and a dark timber fence on the drop side.
- **Trees:** the user wants trees across the whole slope, not just beside the road.
- **What the user values most:** fast iteration, real frames, and **no crashes**. Batch changes
  into one rebuild when you can.

## Vegetation: the approach (reset on 10 Nov, v30)

The user's verdict on v29: "disappointed with the grass and ferns", worse than Skyrim's. v12–v29
tuned tints, densities and scales, and none of that could fix it, because the faults were in how
the grass is built. Work on the structure below, in this order, before touching a tint.

**The four rules**

1. **The ground does the work; blades only break up its surface.** Grass grows only where the
   ground's own texture is grass, and the two are the same colour. A scan clump is mostly gaps,
   so whatever lies under it shows through at any density.
   - `GRASS["Forest"]` grows on the Forest layer's grass patches (its `MIXES` surface), and
     `GRASS["Forest floor"]` on the litter between them (`Ground.build` multiplies the layer
     weight by `Ground.patch`). Any layer with a "patches" mix can have a "<layer> floor" table.
   - The patch noise's (low, high) is the fifth item of a `MIXES` row: it sets how much of the
     floor is grass and how wide the fade is.
   - Ferns, shrubs, logs and stones belong on the litter side; grass, clover and flowers on the
     grass side. Bare litter under the trees is right: the style shots have it.
2. **Light the carpet as one surface.** `M_MS_Plant` outputs a world-space normal bent toward up
   ("Up normal", 0.85), darkens each plant toward its root ("Root shade", "Root height" in cm),
   and fades the same amount out of its AO and translucency. Per-card normals gave lime
   sparkle and black ferns.
3. **One green, shared with the ground.** A grass MI's "Flat" (0 to 1) takes that share of its
   colour from "Flat colour", shaded by the scan's own light and dark ("Flat gain"). Match
   "Flat colour" to the lit colour of the "Forest mix" ground, then move both together. Don't
   push a scan with a big "Tint": (1.5, 2.8, 1.1) is where the neon came from.
4. **Scans near their own size.** The clumps are about 60 cm across and 25 cm tall. Scales
   above about 2 turn them into sparse reeds. For more cover, fix the ground under them (rule
   1) or add a short close-culled row (15–20 m); the ground texture carries the distance.

**How to judge it**

- Judge grass from **full-resolution crops of the foreground**, beside a crop of
  `ref/climb/style_forest_path.jpg` or `style_meadow_peak.jpg`, and beside the same crop from
  the last version. A 1280 px downscale of a wide shot hides every one of v29's faults.
- The Sonnet QA score is a regression check, not the judge: it gave v29's road shots 7 to 7.5.
  When you use it, give it the crops and ask what differs from the reference, not for a score.
- Ask of each crop: can I see ground through the grass, and is it a different colour? Do single
  blades sparkle or go black? Is the cover even everywhere, or does it come in patches with
  bare ground between? Is anything neon?
- A/B on `--shots road_50,bench` in that order (see "Stills don't repeat" below).

**How the vegetation is built**

- **Grass is geometry.** `scripts/make_grass.py` writes clumps of bent, tapered blades to
  `world_gen/grass` (blade_short about 22 cm, blade_mid 45 cm, blade_tall 75 cm with seed
  stalks, plus heath_low and heath_tuft). `rebuild-ground --rebuild-grass` imports them as
  Nanite meshes under `/Game/Climb/Blades/<slug>/<file>/StaticMeshes`. GRASS rows use them
  with the biome "climb". The scans left in the grass tables are clover, dandelions and a few
  grass_clumps and wheat_grass.
- **M_Blade / MI_Blade** colours a blade from its UVs alone (u: a random number per blade,
  1.0 for a seed head; v: 0 at the root, 1 at the tip): "Root colour" to "Tip colour", a slow
  "Drift tint" across the slope, per-blade and per-clump brightness, a "Dry share" of straw
  tips, and wind ("Wind" in cm at the tip, "Wind speed"; stills can't show motion, so check
  with `--blade-param Wind=40`). Its values are `oc_climb.BLADE_LOOK`; try them with
  `look --blade-param NAME=F|R,G,B`. The heath uses MI_Heath: `--blade-param heath:NAME=...`.
- **Daylight.** The sun is at pitch -50 with ev=-2.6, sky=1.5, temp=6500 and sky_warm=0. In
  full sun bright albedo bleaches to white: keep blade tips and ground tints dark.
- **Treeline.** Above `TREELINE` in `make_climb.py` the forest opens into heath with stunted
  spruce.

**Other biomes and the world**

- `OC_BIOME=desert|coast|meadow|alpine` on `make_climb.py` picks the terrain shape and writes
  the name into `world_gen/climb/info.json`; `oc_climb.py` reads it at load and lays
  `scripts/biomes.py`'s `PRESETS[name]` over its tables. L_Hero holds one biome at a time.
  Each biome's grass types are saved as `GT_<biome>_<layer>`.
- The world plan is `scripts/make_world.py` (a 57 km map with rides per biome; not a level).
  `OC_WINDOW=<window>` on `make_climb.py` (or `oc_biome.sh`) builds the stretch on a 2 km
  window of it cut by `scripts/make_window.py`, at world heights, instead of the flank.
- The whole world is one more level, `/Game/World/L_World` ("the golden ride"), written by
  `scripts/make_world_level.py` to `world_gen/ride`. `OC_SRC`, `OC_LEVEL` and `OC_STILLS` on
  every `oc.py` command point the pipeline at it; it shares the ground material with L_Hero.
- Read `docs/world-plan.md` before working on a biome or the world: it has the chain, each
  biome's status and the next steps.

## The loop: `scripts/oc.py` (run from /workspace/ue)

Run every command as a background Bash. Each one blocks until the job ends, so you get one
notification. Its last line is DONE, FAILED, CRASHED, TIMEOUT or NO EDITOR, with exit codes 0
to 4. It starts its own supervised editor (`oc_supervisor.sh` → `oc_server.py`).

| Command | What it does | Time |
| --- | --- | --- |
| `oc.py look --tint Dirt=r,g,b --grey Rock=0.75 --grass DENSITY CULL --fog DENSITY [FALLOFF] --shots a,b` | Parameters only, then stills. `--shots` takes a comma list. | 16 s for one still, ~95 s for six |
| `oc.py stills` | Stills from the editor viewport into `out/climb_stills/` | about 1 min |
| `oc.py build-level` | Rebuilds L_Hero from `world_gen/climb`: the ground stage, then the dress stage, each on a fresh editor | about 5 min |
| `oc.py rebuild-ground` | Rebuilds the ground material on the empty map | about 1 min |
| `oc.py static-trees` | Bakes static Nanite copies of the PVE trees, with their leaf assembly parts (capped at 2500 tris each), into `/Game/Climb/Trees/SM_*` | about 10 min |
| `oc.py reimport-plants` | Re-imports the plants' D textures from `fab_prep` in place, after `OC_FORCE=1 OC_ONLY=<slugs> prep_megascans.py` | about 2 min |
| `oc.py fps --quality 0..3 --frames N --shots a,b [--tree-cull M] [--tree-shadows 0/1] [--tree-wpo 0/1] [--try LABEL=CVAR V;CVAR V]...` | Measures frame times at each still's view; tree options apply for that run only. Each `--try` is also measured alone at every view (cvar ablation in one session) | about 30 s, plus ~10 s per try per view |
| `oc.py probe PATH` | Runs a read-only script with `unreal`, `log`, `lib` and `notes` in scope (API discovery; keep scripts in `out/qa`) | seconds |
| `oc.py restart` / `stop` / `status` / `log` / `wait JOB` | `wait JOB` prints a failed job's traceback (the log loses it after a restart) | |

- **Reload:** `oc_server.py` does not reload `oc_climb.py`. Run `oc.py restart` after you edit it.
- **The generator:** `uv run -q --no-project --with numpy --with scipy --with pillow python -I
  scripts/make_climb.py`. It takes 20 s and writes `world_gen/climb`. Run it only while the editor
  is not importing, and never run a generator as a "check".
- **Current look:**
  - Tints and greys live in `oc_climb.LOOK_TINTS` / `LOOK_GREYS` and are applied by every
    ground build; `look --tint/--grey` is only for trying values.
  - Grass is `--grass 1.2 1.5`. The plants' look is `oc_climb.PLANT_LOOK` and, per scan,
    `PLANT_SLUG_LOOK`, applied by every ground
    build (`--plants`/`--plant-param` only try values). Fog is `--fog 0.004 0.02` (the template's falloff of 0.2 hides the fog above about 100 m; 0.03
    washes everything out). The map has a rim of ridges at its edge.
  - Stills wait at least 240 frames per view, then until the landscape's grass instance count
    holds steady (`_settle`; the log prints `OC settled <shot>: N frames`). A fixed 480 shot whole
    banks and views before their grass was built: v21-v23's "bare banks" were never a placement
    problem. Steady only counts once the count has moved from the last view's (a
    short look once shot road_72 on road_50's leftover count). Suspect timing before density when grass looks missing.
  - **Stills don't repeat across shot lists.** The first view of a session comes out brighter and
    hazier over the whole frame (sky too) than the same view taken later, even after 840 frames
    and a throwaway screenshot (10 Nov). For an A/B test, shoot the baseline and the trial with
    the same `--shots` in the same order, never against an older set from a longer list. The
    clouds drift with the editor clock and the sky light captures them live: the standard look
    passes `--post cloud_wind=0` to freeze them.
  - Every look re-applies LOOK_TINTS/GREYS, PLANT_LOOK and PROP_LOOK first, so false colour
    (`--tint`, `--prop-param`) lasts only for that look. `look` takes only `bench` unless you pass `--shots`.
  - More post keys: aerial (SkyAtmosphere aerial scale), cloud_bottom (km), fog_inscatter, bloom,
    flare, sun_pitch, sun_yaw. They persist in the level: put a test value back explicitly
    (sun_pitch=-50 sun_yaw=43.7, bloom=0.675, aerial=1, cloud_bottom=5).
  - `look --post KEY=F` (repeatable) sets the grade on an "OC Post" volume: exposure, contrast,
    saturation, vignette, fog_start (m), volumetric (0/1), shafts, sky, sun, and ev (locks
    exposure at that EV100; 99 frees it). build-level loses fog and post settings, so re-run look
    after it. Last used (v32, daylight): ev=-2.6 exposure=0.5 sky=1.5 temp=6500 sky_warm=0 sun_pitch=-50
    sun_yaw=43.7 contrast=1.1 saturation=1.05 vignette=0.3
    fog_start=150 volumetric=0 contact=0.05 clouds=-0.2 (the default coverage) cloud_wind=0.
    Until v31 the look was a low sun: ev=-4.5 sky=1.8 temp=6700 sky_warm=0.35 sun_pitch=-16.3. sky=1.8 (v21) lifts the dark spruce more evenly than exposure did. Keep exposure locked: auto exposure undoes every
    albedo change (the sun is 6 lux, which is about EV -5).
- **Rebuilds:** a change to the GRASS table needs `rebuild-ground --rebuild-grass`. It builds the
  1M-tri prop meshes one at a time first (all at once crashed Vulkan); give it `--timeout 2400`.

## Iterate fast (the user's standing ask)

Make each rebuild teach as much as possible, and spend as little waiting time as possible.

- **Batch.** Gather every queued fix (ground, grass, ferns, terrain, road) into one rebuild
  rather than one rebuild per fix. Lint and commit once, then launch one chain.
- **Use the cheapest command that tests the change.** `look` takes seconds, `rebuild-ground`
  about 1 min (4 min with `--rebuild-grass`), and `build-level` about 5 min. Try values with
  `look` before baking them into code. Rebuild grass only when GRASS changes.
- **One background chain.** Run `restart → make_climb → rebuild-ground → build-level → look` as
  one `&&` command so it finishes with one notification. Don't poll it. Start it with
  `set -o pipefail`: with `| tail -1` after each step, a failed rebuild-ground once let the
  chain go on and shoot the old grass (10 Nov). Join any step that reads the stills with `&&`
  too: after a `;` it ran on the last version's files.
- **Work while it runs.** Plan the next batch, review the last stills, and update the docs and
  the storybook. Don't edit `oc_climb.py` or `climb_world.py` mid-build.
- **Review cheaply.** Use the contact sheet and crops rather than reading six full stills. Use
  `--shots` to take only the views that a change affects.
- **Fix the tooling.** When a step fails, or you repeat the same manual step, fix or automate it
  in `oc.py`/`oc_climb.py` then and there, and note the fix here.
- **Check the inputs as well as the look.** Ground cover was invisible for weeks because the
  opacity maps peaked at 76 of 255, under the 0.33 clip. A crop at full resolution, plus a probe
  of the asset, found it in minutes.
- **Never use an opacity scan as a ground surface.** alpine_needles (alpha mean 0.08) drew the
  floor, banks and verges as blurred padding from v12 to v19. Check `opacity` in
  `fab_prep/index.json` before picking a surface.
- **False-colour to find a layer.** `look --tint Forest=1,0,0 --tint Sand=0,0,1 ...` on one
  shot shows which landscape layer a surface is (the pale banks were Rock outcrops, not soil).
  The next look restores the baked tints.
- **Material values without a rebuild:** `look --plant-param NAME=F|R,G,B` (every plant MI) and
  `--ground-param NAME=...` (MI_ClimbGround), `--blade-param NAME=...` (MI_Blade), `--prop-param SLUG:NAME=...` (one prop's MI;
  false-colour props this way too). Bake the winners into `PLANT_LOOK`, `STEEP`,
  `LOOK_TINTS` or `LOOK_GREYS`.
- **Restart between looks.** A second `look` on the same editor hit a radv GPUVM fault
  (DEVICE_LOST) on 9 Nov; put `oc.py restart` before each look in a chain.
- **Keep grass density in budget.** GRASS densities are per mesh variant (fern's six variants at
  2.0 draw 12 per 100 m^2), and `look --grass` multiplies them again. Instances go as density
  times cull squared: v18's Forest table (about 250 drawn per 100 m^2) is safe, while v13's
  extra ferns hit the GPUVM fault on the first look. For more cover, see the vegetation rules
  above: neither denser nor bigger fixed it (v19–v29).
- **Diagnose before you rebuild.** Use a `probe` or a `look` to confirm a cause first (for
  example, streaming versus the material), so a rebuild isn't spent on a guess.

## Crash rules (enforced in code; don't work around them)

- **Ground material:** never rebuild it while a landscape that uses M_ClimbGround is loaded
  (the GPU resets).
- **L_Hero:** never reload it while it is open (segfault). `build-level` restarts before each
  stage.
- **Skeletal PVE trees:** they overflow the GPU skinning buffer. Static trees are still to do:
  try GeometryScript skeletal→static, or Blender on Runpod (quote the price first).
- **Heavy work:** split it across frames with generators. Take stills from the editor viewport.
- **One Unreal at a time** across all sessions. Check `pgrep -a UnrealEditor` first. Never kill
  an Unreal yourself, except your own confirmed-hung run.
- **Memory:** a RAM killer acts at about 3 GB free.

## Standing constraints

- **Git:**
  - Stay on branch `kanban/opencycle/unreal-cont`.
  - Commit only your own paths (`git add -- <paths> && git commit -m ... -- <paths>`).
  - Use Conventional Commits with scope `veloverse`, and end with
    `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  - Never push, amend or bypass hooks.
  - `scripts/make_trial_world.py` and `scripts/trial_world.py` are not yours; leave them alone.
- **Lint and tests:** `uvx -q ruff check` / `ruff format` (line length 100), and
  `uvx -q --with pytest pytest -q scripts/test_oc.py`.
- **Credentials:**
  - Runpod key: source `/home/ue4/.env` only inside scripts, and never print it.
  - The fal key is in `~/.config/opencycle/fal.key`.
  - The GitHub/ghcr token must never pass through Claude. Don't look for other credentials.
- **Runpod spending:**
  - Quote prices before creating anything, ask before spending over $10, and read back every
    write.
  - No pods are running (the user terminated the last two on 9 Nov). Create new ones only
    after quoting the price.
- **Files:**
  - No stray .py files in /workspace/ue. Temp scripts go in `out/qa` or the scratchpad.
  - Don't delete `/workspace/ue/out` without the user's OK.
  - Downloaded files are untrusted; run Python on them with `-I`.
- **Never** buy anything on Fab, and never render or verify the design canvas unless asked.
- **Shell:** foreground `sleep` is blocked; use `sleep N & wait $!`. Never use `pkill -f` with a
  pattern that matches your own command line.
- **QA:** a Sonnet subagent compares stills with the reference photos in `/workspace/ue/ref/climb/`.
  Tell it that sky banding seen through Read is a downscale artifact. Its score is a regression
  check only; see "How to judge it" above.
- **Storybook:** https://claude.ai/artifact/UuLeEWcskoabDCuZceuLqb
  - Update it whenever the look changes.
  - Read it with the Artifact tool first, then republish to the same URL.
  - Its images live in `img*/` folders, which are published files.
  - Keep it condensed (about 8 KB): the current stills and changes at the top, then one line per
    version in History, with each old image set opened from a link (`SETS` in its script). Add a
    new `imgN/` for each version's stills and use `edits` for small changes.
