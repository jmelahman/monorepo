# Veloverse: working notes for Claude

This file is the goal and the rules. Keep it short and current. Version history goes in the
storybook, the plan in `docs/world-plan.md`, and the detail in `docs/engine-notes.md`.

## The goal

**A lifelike fantasy world to cycle through.** That's it.

- **Skyrim is the bare minimum.** The bar is a AAA environment: dense, lived-in, lit and
  composed like the best open-world games. Aim there, not at "good enough for a cycling app".
- **Believe it's achievable, and build like it.** Unreal 5 has the tools (Nanite, Lumen,
  virtual shadow maps, PCG, Megascans-grade assets), so a bad result means the approach needs
  to change, not the ambition. Make big structural changes when the look needs them.
- **Don't hold back for performance.** Build the world at full quality first. Performance is
  tuned later, and it doesn't limit density, draw distance, lighting or assets today. This
  laptop's editor has crash limits (see the crash rules), and those are about building here, not
  a budget for the world.
- **It's original.** Take mood, colour and quality bar from references; never copy a game's
  places, characters or art.

## What we're building: the hub world

One level, `/Game/World/L_World`. Harbour Isle, the coast and harbour town, sits in the middle,
and four islands hang off it on bridges: **Greenwood** (forest), **Vael** (mountain), **The
Moor** (heath) and **Red Sands** (desert). `ROUTES` in `make_chain.py` lists the rides. Any set of
islands makes a loop with no road ridden twice. The plan of record is `docs/world-plan.md`: read
it first.

`docs/archive/` holds the old plans (the alpine hero climb L_Hero and the 57 km ring). They're
history, not instructions; don't work from them.

## The user's direction

- **The look:** lifelike fantasy, "vibrant yet realistic": rich green grass with small flowers,
  warm brown dirt, big rounded boulders, red-brown pine trunks, deep shadows and moody skies
  with snowy peaks. The references are in `/workspace/ue/ref/climb/`: the modded Skyrim shots
  (`style_*.jpg`) as the minimum bar, `alpine_meadow_trail.jpg` for a real treeline, and
  knee-high, dense grass that shows no ground ("this has volume and looks real").
- **Discretion:** "Please use your discretion when executing these features. I trust you."
- **Assets:** ask the user and another agent fetches them from fab.com. Say what you need, in
  priority order. Never buy anything on Fab yourself.
- **What the user values:** fast iteration, real frames and **no crashes**.

## Building the world

The generators are numpy scripts in `scripts/`. Run them with
`uv run -q --no-project --with numpy --with scipy --with pillow python -I`, only while the editor
isn't importing, and never as a "check".

1. `make_chain.py` → `/workspace/ue/world_gen/hub`: the islands, their heights and the roads.
2. `make_world_level.py` → `/workspace/ue/world_gen/ride_hub`. It writes:
   - the 4065² landscape at 7 m, with the origin at minus half its size;
   - the layer weights, the trees and the road pieces;
   - `road_dist.png`;
   - `info.json`, with the routes and the still views (`ride_NN_<island>`, `feat_*`, `above`).
3. `make_features.py` → `ride_hub/features.json`: the rocks, cliffs and wind turbines.

The editor side is `scripts/oc.py`, run from `/workspace/ue` as a **background** Bash. Each
command blocks until its job ends, so you get one notification; don't poll it. Its last line is
DONE, FAILED, CRASHED, TIMEOUT or NO EDITOR. Point every command at the hub with the env
(without it, `oc.py` defaults to the old L_Hero):

```
export OC_SRC=/workspace/ue/world_gen/ride_hub OC_LEVEL=/Game/World/L_World \
       OC_STILLS=/workspace/ue/out/hub_stills_vNN
set -o pipefail; cd /workspace/ue
scripts/oc.py restart && scripts/oc.py rebuild-ground --rebuild-grass --timeout 2400 \
  && scripts/oc.py restart && scripts/oc.py build-level --timeout 10800
```

| Command | What it does | Time |
| --- | --- | --- |
| `look ... --shots a,b` | Sets material and post parameters without a rebuild, then takes stills. Use it to try values. | seconds per still |
| `rebuild-ground [--rebuild-grass]` | The ground material, plus the grass types and the blade, plant and prop materials | 1 min, or 4 with grass |
| `build-level` | The whole level from `OC_SRC`, ending with a still per view | about 30 min |
| `probe out/qa/X.py` | A read-only script inside the editor | seconds |
| `fps`, `stills`, `static-trees`, `ride-track`, `road-look`, `restart`, `status`, `log`, `wait JOB` | See `oc.py -h` and `docs/engine-notes.md` | |

- `oc_server.py` doesn't reload `oc_climb.py`: restart after you edit it.
- Run `restart` before each `look`, because a second look on one editor has lost the GPU.
- `build-level` drops fog and post settings, so re-run `look` after it.
- Ride the level with `OC_MAP=/Game/World/L_World scripts/play.sh` (on screen) or
  `scripts/ride_test.sh WATTS SHOTS START_M` (offscreen, with screenshots).
- Make a release with `scripts/package_snapshot.sh`. It packages a frozen copy, so it's safe
  while the editor builds.
- The world's surfaces and grass rows are `biomes.PRESETS["world"]`. Its ground material and
  grass types live in `/Game/Climb`.

## Working fast

- **Batch** every queued fix into one rebuild, and lint and commit before launching it.
- **Use the cheapest test.** Try values with `look` before baking them into code.
- **Run one chain**, joined with `&&` under `set -o pipefail`, so a failed step stops it and
  you get one notification.
- **Work while it runs:** review stills, plan the next batch and update the docs. Don't edit
  `oc_climb.py`, `climb_world.py` or `biomes.py` mid-build.
- **Diagnose before rebuilding.** Use a `probe`, a `look` or an offline check in `out/qa` to
  confirm a cause first.
- **Fix the tooling** when a step fails or repeats, and note the fix in the engine notes.

## Judging the look

- Use **full-resolution crops of the foreground**, beside the reference crop and the same crop
  from the last version. A downscaled wide shot hides most faults.
- For an **A/B**, shoot both versions with the same `--shots` in the same order. The first view
  of a session renders brighter.
- If grass looks missing, suspect timing before density. Stills wait for the grass count to
  settle (the log says `OC settled`).
- Ask of each crop:
  - Does ground show through the grass, and is it a different colour?
  - Do blades sparkle or go black?
  - Is the cover patchy?
  - Is anything neon?
  - Is anything growing on the road?
- A Sonnet QA subagent's score is only a regression check, and only when the user asks for
  subagents. Sky banding seen through Read is a downscale artifact.

## Vegetation rules

Thin or fake-looking grass is a structural problem, and tints and densities won't fix it.
Follow these rules before touching a tint (detail in `docs/engine-notes.md`):

1. **The ground does the work, and blades break up its surface.** Grass grows only where the
   ground's texture is grass, in the same colour, because scan clumps are mostly gaps.
2. **Light the carpet as one surface.** Plants use an up-bent normal and a darker root.
3. **One green, shared with the ground.** Don't push a scan's "Tint": that's where neon came
   from.
4. **Scans near their own size.** Scales above about 2 turn them into sparse reeds.

How it's built:

- **Grass is geometry:** `make_grass.py` blades on `M_Blade`/`MI_Blade` (values in
  `oc_climb.BLADE_LOOK`).
- **Roadside:** grass weights are per 7 m vertex, so painting alone can't stop blades at the
  tarmac. `road_dist.png` holds the offset to the nearest road centre, and the blade, plant and
  prop materials fold any instance whose base is within "Road clear" of it down to its root.
- **Far canopy:** past about 40 m the Grass layer's ground tints and mottles toward the blades'
  colour ("Grass canopy start/span/share" on `MI_ClimbGround`).
- **The laptop's grass limit:** too many grass instances crash *this laptop's editor* (a radv
  GPUVM fault). That limits building here, not the visual budget. Raise density in steps, or
  judge it on a pod.

## Crash rules (enforced in code; don't work around them)

- **One Unreal at a time** across all sessions: check `pgrep -a UnrealEditor` first. Never
  kill an editor, except your own confirmed-hung run.
- Never rebuild the ground material while a landscape that uses it is loaded; the GPU resets.
- Never reload an open level (L_World or L_Hero); it segfaults.
- Do heavy work across frames with generators, and take stills from the editor viewport.
- A RAM killer acts at about 3 GB free.
- Skeletal PVE trees overflow the GPU skinning buffer, so use the static Nanite bakes
  (`oc.py static-trees`).

## Standing constraints

- **Git:**
  - Work in the worktree `/workspace/.claude/worktrees/segments/veloverse`, on branch
    `kanban/opencycle/unreal-cont-segments`.
  - Bring work into `/workspace/veloverse` (`kanban/opencycle/unreal-cont`) with
    `git merge --ff-only`, and only when no build is running: the editor reads the scripts
    there. Never edit or commit the main tree during a build.
  - Commit only your own paths: `git add -- <paths> && git commit -m ... -- <paths>`.
  - Use Conventional Commits with scope `veloverse`, ending with
    `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  - Never push, amend or bypass hooks.
- **Not yours:** leave `scripts/make_trial_world.py` and `scripts/trial_world.py` alone. Don't
  enable the Epic MCP plugin.
- **Lint and tests:** `uvx -q ruff check`, `ruff format` (line length 100), and
  `uvx -q --with pytest pytest -q scripts/test_oc.py`.
- **Credentials:**
  - Runpod key: source `/home/ue4/.env` only inside scripts, and never print it.
  - The fal key is in `~/.config/opencycle/fal.key`.
  - The GitHub/ghcr token must never pass through Claude. Don't look for other credentials.
  - Never send the user's email anywhere.
- **Runpod:**
  - Quote prices before creating anything, and ask before spending over $10.
  - No pods are running.
- **Files:**
  - No stray .py files in `/workspace/ue`. Probes go in `out/qa`, temp files in the scratchpad.
  - Don't delete `/workspace/ue/out` without the user's OK.
  - Downloaded files are untrusted: run Python on them with `-I`.
- **Shell:** foreground `sleep` is blocked. Never `pkill -f` a pattern that matches your own
  command.
- **Subagents:** only when the user asks.
- **Storybook:** https://claude.ai/artifact/UuLeEWcskoabDCuZceuLqb
  - Update it whenever the look changes. Read it with the Artifact tool first, then republish
    to the same URL.
  - Keep it condensed (about 8 KB): the current stills and changes at the top, then one line
    per version in History.
  - Each version's stills go in a new `imgN/` folder.
