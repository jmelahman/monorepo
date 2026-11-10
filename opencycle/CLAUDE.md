# OpenCycle: working notes for Claude

Read `docs/climb-plan.md` first. It is the plan of record, and its "Where things stand" section
says what was done last. Update both files before a session ends or when a rule changes.

## The goal (the user's decisions)

- **The goal:** an original cycling world (Zwift-like, not a copy) that looks **realistic**.
  "No stylized."
- **The look (10 Nov):** "vibrant yet realistic", like the modded Skyrim screenshots in
  `ref/climb/style_*.jpg`. That means rich green grass carpets with small flowers, warm brown dirt,
  big rounded boulders, red-brown pine trunks, deep shadows and moody skies with a snowy peak. Use
  them for mood and colour only; build nothing from that game.
- **The first target:** one realistic mountain climb. Build a 1 km hero stretch first, then the
  full climb, then other biomes.
- **Performance:** this laptop (Radeon 890M) must hold 30 fps at Low *eventually*. For now the
  user says **visuals first, perf later**: keep trees at full quality (2 km cull, shadows, wind).
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
  - Grass is `--grass 1.2 1.5`. The plants' look is `oc_climb.PLANT_LOOK`, applied by every ground
    build (`--plants`/`--plant-param` only try values). Fog is `--fog 0.004 0.02` (the template's falloff of 0.2 hides the fog above about 100 m; 0.03
    washes everything out). The map has a rim of ridges at its edge.
  - Stills wait at least 240 frames per view, then until the landscape's grass instance count
    holds steady (`_settle`; the log prints `OC settled <shot>: N frames`). A fixed 480 shot whole
    banks and views before their grass was built: v21-v23's "bare banks" were never a placement
    problem. Suspect timing before density when grass looks missing.
  - Every look re-applies LOOK_TINTS/GREYS, PLANT_LOOK and PROP_LOOK first, so false colour
    (`--tint`, `--prop-param`) lasts only for that look. `look` takes only `bench` unless you pass `--shots`.
  - More post keys: aerial (SkyAtmosphere aerial scale), cloud_bottom (km), fog_inscatter, bloom,
    flare, sun_pitch, sun_yaw. They persist in the level: put a test value back explicitly
    (sun_pitch=-16.3 sun_yaw=43.7, bloom=0.675, aerial=1, cloud_bottom=5).
  - `look --post KEY=F` (repeatable) sets the grade on an "OC Post" volume: exposure, contrast,
    saturation, vignette, fog_start (m), volumetric (0/1), shafts, sky, sun, and ev (locks
    exposure at that EV100; 99 frees it). build-level loses fog and post settings, so re-run look
    after it. Last used: ev=-4.5 exposure=0.5 sky=1.8 temp=6700 sky_warm=0.35 contrast=1.1 saturation=1.05 vignette=0.3
    fog_start=150 volumetric=0 contact=0.05 clouds=-0.2 (the default coverage). sky=1.8 (v21) lifts the dark spruce more evenly than exposure did. Keep exposure locked: auto exposure undoes every
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
  one `&&` command so it finishes with one notification. Don't poll it.
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
  `--ground-param NAME=...` (MI_ClimbGround), `--prop-param SLUG:NAME=...` (one prop's MI;
  false-colour props this way too). Bake the winners into `PLANT_LOOK`, `STEEP`,
  `LOOK_TINTS` or `LOOK_GREYS`.
- **Restart between looks.** A second `look` on the same editor hit a radv GPUVM fault
  (DEVICE_LOST) on 9 Nov; put `oc.py restart` before each look in a chain.
- **Keep grass density in budget.** GRASS densities are per mesh variant (fern's six variants at
  2.0 draw 12 per 100 m^2), and `look --grass` multiplies them again. Instances go as density
  times cull squared: v18's Forest table (about 250 drawn per 100 m^2) is safe, while v13's
  extra ferns hit the GPUVM fault on the first look. For more cover, draw grass bigger (a
  row's optional (min, max) scale) rather than denser.
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
  - Use Conventional Commits with scope `opencycle`, and end with
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
- **QA:** a Sonnet subagent scores stills against the reference photos in `/workspace/ue/ref/climb/`.
  Tell it that sky banding seen through Read is a downscale artifact.
- **Storybook:** https://claude.ai/artifact/UuLeEWcskoabDCuZceuLqb
  - Update it whenever the look changes.
  - Read it with the Artifact tool first, then republish to the same URL.
  - Its images live in `img*/` folders, which are published files.
  - Keep it condensed (about 8 KB): the current stills and changes at the top, then one line per
    version in History, with each old image set opened from a link (`SETS` in its script). Add a
    new `imgN/` for each version's stills and use `edits` for small changes.
