# OpenCycle: working notes for Claude

Read `docs/climb-plan.md` first. It is the plan of record, and its "Where things stand" section
says what was done last. Update both files before a session ends or when a rule changes.

## The goal (the user's decisions)

- **The goal:** an original cycling world (Zwift-like, not a copy) that looks **realistic**.
  "No stylized."
- **The first target:** one realistic mountain climb. Build a 1 km hero stretch first, then the
  full climb, then other biomes.
- **Performance:** this laptop (Radeon 890M) must hold 30 fps at Low.
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
| `oc.py look --tint Dirt=r,g,b --grey Rock=0.75 --grass DENSITY CULL --fog DENSITY [FALLOFF] --shots NAME` | Parameters only, then stills. `--shots` takes one name. | 16 s for one still, ~95 s for six |
| `oc.py stills` | Stills from the editor viewport into `out/climb_stills/` | about 1 min |
| `oc.py build-level` | Rebuilds L_Hero from `world_gen/climb`: the ground stage, then the dress stage, each on a fresh editor | about 5 min |
| `oc.py rebuild-ground` | Rebuilds the ground material on the empty map | about 1 min |
| `oc.py restart` / `stop` / `status` / `log` / `wait JOB` | | |

- **Reload:** `oc_server.py` does not reload `oc_climb.py`. Run `oc.py restart` after you edit it.
- **The generator:** `uv run -q --no-project --with numpy --with scipy --with pillow python -I
  scripts/make_climb.py`. It takes 20 s and writes `world_gen/climb`. Run it only while the editor
  is not importing, and never run a generator as a "check".
- **Current look:**
  - Dirt is tinted 0.62,0.63,0.65 with grey 0.85.
  - Rock has grey 0.75, and Rock mix has grey 0.6.
  - Grass is `--grass 2.0 1.5`.
  - Fog is `--fog 0.008 0.02`. The template's falloff of 0.2 hides the fog above about 100 m. Use falloff 0.02.
    Density 0.03 washes everything out, and 0.012 doesn't hide the map's edge. The map has a rim
    of ridges at its edge (`make_climb.terrain`) so the fog can stay light.

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
