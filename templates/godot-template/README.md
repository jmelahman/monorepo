# godot-template

[![Tests](https://github.com/jmelahman/godot-template/actions/workflows/pre-commit.yml/badge.svg)](https://github.com/jmelahman/godot-template/actions/workflows/pre-commit.yml)

> [!NOTE]
> I am not a game developer. This template was generated entirely by AI and is
> mostly for demonstration and personal experimentation. Treat it as a starting
> point to learn from, not a vetted recommendation for how Godot games should
> be built.

A template repository for a Godot 4 desktop game headed for Steam. The
tooling is the point. The sample game is push-your-luck dice (roll to fill a
pot, a 1 loses it, bank to keep it), and it is deliberately tiny. That still
gives each layer one worked example to copy from: pure rules, presentation,
saves, settings, localization, an achievement, tests and an export.

Everything here can be driven without opening the editor, which is what lets
an agent build, check, play and look at the game on its own.

## What's included

**A deterministic core.** `sim/` holds the rules as a pure reducer,
`Rules.reduce(state, action) -> {state, events}`. All randomness is derived
from the run's seed, so a run is its seed plus its actions: it replays
exactly, it saves as plain JSON, and a balance question is answered by
simulating a few thousand seeds in a test. A test enforces the purity, so it
doesn't depend on anyone remembering it. The sim emits codes, never
sentences; `game/` turns them into translated text (`locale/*.po`).

**The services every game needs**, as autoloads in `core/`:

| Autoload   | Does                                                                                         |
| ---------- | -------------------------------------------------------------------------------------------- |
| `Settings` | Volume, fullscreen and language in a `ConfigFile`, applied on change and on launch           |
| `Saves`    | Versioned JSON slots, atomic writes, a `.bak` fallback and a migration hook                  |
| `Platform` | [GodotSteam](https://godotsteam.com) (init, callbacks, achievements), inert when Steam isn't |
| `Dev`      | Starts the agent bridge in debug builds that ask for it                                      |

**Checks, everywhere the same**

| Tool                                                                       | Local (prek) | CI               |
| -------------------------------------------------------------------------- | ------------ | ---------------- |
| [gdlint, gdformat](https://github.com/Scony/godot-gdscript-toolkit)        | pre-commit   | `pre-commit` job |
| `scripts/check.sh`: compile every script, warnings as errors               | pre-commit   | `pre-commit` job |
| `scripts/resave.sh --check`: scenes are what the editor would write        | pre-commit   | `pre-commit` job |
| [gdUnit4](https://github.com/godot-gdunit-labs/gdUnit4) tests              | pre-commit   | `pre-commit` job |
| [ruff](https://docs.astral.sh/ruff), shellcheck, shfmt, actionlint, zizmor | pre-commit   | `pre-commit` job |

The compile check exists because Godot is forgiving in the wrong places. It
prints warnings only with a debugger attached, and it exits 0 after a parse
error. `check.sh` runs with one attached, collects every warning through a
`Logger` and fails on any of them. Every script greps Godot's output for
errors rather than trusting the exit code.

**An agent bridge.** `addons/agent_bridge` is a small JSON-over-TCP server
started by `scripts/play.sh`. Through it, `scripts/bridge.py` can read and set
properties, call methods, evaluate expressions, press input actions, click,
wait for frames and take screenshots of the running game. It exists only in
debug builds that ask for it, and every export preset excludes it.

**Release.** Pushing a `v*.*.*` tag runs the tests, exports Linux and Windows
builds (Steam libraries included), and attaches the zips to a GitHub release.

**A devcontainer** on the
[monorepo's](https://github.com/jmelahman/monorepo) base image, with Godot,
Xvfb, Mesa and gdtoolkit baked in at the pins in `scripts/versions.env`.

## Using this template

1. Create a repo from this template (GitHub → "Use this template").
2. Name the game: `config/name` in `project.godot`, and the export paths
   (`game.x86_64`, `game.exe`) in `export_presets.cfg`.
3. Steam: set `APP_ID` in `core/platform.gd` (480 is Valve's Spacewar test
   app) and replace `ACH_FIRST_WIN` in `game/main.gd` with your achievement
   API names.
4. Replace the dice: `sim/rules.gd` and `sim/run_state.gd`, their tests, and
   `game/main.gd` and `game/main.tscn`. Keep `sim/rng.gd` and the purity
   test.
5. Replace this README, and update `LICENSE`.
6. `scripts/setup.sh && prek install && prek run --all-files`.

## Develop

```sh
scripts/setup.sh                       # Godot (pinned), gdUnit4, GodotSteam, first import
scripts/test.sh                        # gdUnit4; or scripts/test.sh res://tests/sim
scripts/lint.sh                        # every static check; scripts/format.sh fixes style
godot --path . -e                      # the editor, as usual

XVFB=1 scripts/play.sh                 # run it off-screen with the bridge on
scripts/bridge.py action roll          # drive it
scripts/bridge.py eval 'scene.state.score'
scripts/screenshot.sh shot.png         # look at it
scripts/bridge.py quit

scripts/setup.sh templates             # once, 1.3 GB
scripts/export.sh                      # build/*.zip, as the release does
```

The scripts keep Godot's user data in `.godot-home/`, so nothing they run
touches your real saves or editor settings.

Every download is pinned in `scripts/versions.env`: the archives by version
and sha256, gdtoolkit by version. To bump an archive, edit it and run
`scripts/setup.sh`, which prints the new sum on a mismatch. The devcontainer image and CI read the same file.
