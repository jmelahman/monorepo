# godot-template — Claude Notes

A Godot 4.7 template: a deterministic rules core in `sim/`, a thin scene in
`game/` that presents it, the services every game needs in `core/`, and the
tooling that lets an agent build, check, run and look at the game without an
editor. The sample game is push-your-luck dice, small enough to delete.

## Commands

Every script sources `scripts/env.sh`, which points Godot's user dirs into
`.godot-home/`, so nothing scripted touches a real save or editor setting.

- First run, and after a pin bump: `scripts/setup.sh` (Godot at the pinned
  version if yours differs, the addons, the first import). `scripts/setup.sh
  system` for the apt packages; `templates` for exporting.
- Tests: `scripts/test.sh [res://tests/sim ...]` (gdUnit4, under Xvfb when
  there is no display).
- Everything static: `scripts/lint.sh` (gdlint, gdformat, `check.sh`,
  `resave.sh --check`). `scripts/format.sh` fixes the formatting.
- All of it, as CI runs it: `prek run --all-files`.
- After editing a `.tscn`/`.tres` by hand: `scripts/resave.sh`, then read
  the diff.
- Run and drive the game: `XVFB=1 scripts/play.sh`, then `scripts/bridge.py
  <cmd>` (`tree`, `get`, `set`, `call`, `eval`, `action`, `key`, `click`,
  `wait`, `screenshot`, `quit`; `scripts/bridge.py --help`).
- Look at it: `scripts/screenshot.sh out.png`, then read the PNG. Uses a
  running `play.sh` game if there is one, else starts and stops its own.
- Release builds: `scripts/export.sh [preset]` into `build/`.

## The shape of the thing

`sim/` is the game's rules, and nothing else: `Rules.reduce(state, action)`
returns a new `RunState` and a list of events, and never mutates its input.
No nodes, no `Engine`/`OS`/`Time`/`Input`, no global RNG, no loads from
outside `sim/`; `tests/sim/test_purity.gd` enforces that rather than trusting
it. All randomness is `Rng.derive(run_seed, [coords])`, a pure hash, so a run
is its seed plus its actions and `Rules.replay` reproduces it exactly. That is
what makes balance testable (see the many-seeds test in `test_rules.gd`) and
saves trivially correct.

`RunState` is plain data that round-trips through JSON, and a test asserts
it. JSON has one number type, so `from_dict` narrows every int back through
`type_convert`.

The sim authors no prose. Events and refusals are codes with operands
(`{"type": &"refused", "code": &"empty_pot"}`), and `game/main.gd` turns them
into `tr()` sentences. Strings live in `locale/*.po`; a new language is one
file plus a line in `project.godot`.

`game/main.gd` is one-way: input → `dispatch` → reduce → save → `_render`.
`dispatch` is public so tests and the bridge drive the game the way a player
does.

`core/` is the autoloads: `Settings` (a `ConfigFile`, typed getters, applies
itself), `Saves` (versioned, atomic, with a `.bak` fallback and a migration
hook), `Platform` (GodotSteam, inert without it) and `Dev` (starts the bridge
in debug builds that ask for it).

## Gotchas, each of which cost time

- **Warnings print only with a debugger attached.** Without `-d`, Godot
  compiles silently. `scripts/check.sh` passes it; `tools/check.gd` collects
  warnings through a `Logger` and fails on any. The project keeps
  `unsafe_*` at warn so the editor stays usable; `check.sh` is where they
  become errors. Read a `Variant` into a typed `var` (or `type_convert`)
  instead of `as`-casting it or calling `int()` on it.
- **`godot --check-only` doesn't load autoloads**, so any script naming
  `Settings` fails there. That is why `tools/check.gd` exists.
- **Godot exits 0 after a parse error** in something it loaded. Every
  script greps the log for `GODOT_ERROR_PATTERN` (`env.sh`).
- **`exclude_addons` is gone in 4.7**; warnings are scoped by
  `debug/gdscript/warnings/directory_rules`, read once at startup, so
  setting it from a running script does nothing. `project.godot` excludes
  `res://addons` and re-includes `res://addons/agent_bridge`, which is ours.
- **`--headless` has no renderer and drops input.** Screenshots come back
  blank and simulated input never reaches the viewport, so tests, play and
  screenshots run under Xvfb with Mesa's llvmpipe (`with_display`).
- **gdUnit4's scene runner calls the root's `_unhandled_input` directly**
  as well as feeding the event through `Input`, so a root that handles
  input there sees each event twice. Use `_input`. Separately, Space is
  both `roll` and `ui_accept`, so a focused button made one keypress two
  rolls: the board's buttons are `focus_mode = 0`.
- **GodotSteam registers an engine singleton named `Steam`**, so an
  autoload of that name collides. Ours is `Platform`, and it reaches Steam
  through `Engine.get_singleton("Steam").call(...)` so nothing fails to
  parse where the extension is missing.
- **`seed` is a global function**, and a variable named `seed` shadows it
  with a warning. It is `run_seed` throughout.
- **Reloading the running script with `CACHE_MODE_IGNORE` crashes the VM**
  (`tools/check.gd` skips itself).
- **`GODOT_TEST=1`** (set by `test.sh`) keeps `Settings` off the window and
  `Platform` off Steam, so a test run never resizes your screen or talks to
  the Steam client.

## The agent bridge

`addons/agent_bridge/bridge.gd` is a newline-delimited JSON server on
127.0.0.1 (port 9877, or `AGENT_BRIDGE_PORT`). It starts only in a debug
build, only when asked (`-- --agent-bridge` or the env var), and every
export preset excludes it; `eval` is arbitrary code, which is why. It keeps
answering while the tree is paused. Values JSON can't carry come back as
`var_to_str` text, and `set` takes `var:Vector2(1, 2)` for the same reason.
Prefer `action` over `key`: it goes through the input map, as a player's
controller would.

## Conventions

- Typed GDScript throughout: `untyped_declaration` is an error in
  `project.godot`. Doc comments (`##`) on anything a reader would ask about.
- Scenes are written as the editor would write them; `resave --check` in
  pre-commit holds that. Use unique names (`%Score`) for nodes a script
  touches, so moving them in the tree doesn't break the script.
- Pins live in `scripts/versions.env`, each archive with its sha256.
  Bumping one means editing it and running `scripts/setup.sh`, which prints
  the new sum on mismatch. `GDTOOLKIT_VERSION` moves with the gdtoolkit rev
  in `.pre-commit-config.yaml`. The devcontainer image bakes the same pins, so a bump
  rebuilds it (the monorepo's `devcontainer.yml` watches this file).
- Third-party addons (`gdUnit4`, `godotsteam`) are gitignored and installed
  by `setup.sh`. The `.gd.uid` and `.import` files are committed.
