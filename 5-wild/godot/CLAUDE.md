# 5-wild/godot — Claude Notes

The desktop build of 5-wild, headed for Steam. It does not reimplement the
game: `src/engine`, the same TypeScript the web build runs, is bundled into
one script and run inside Godot by QuickJS, through a small GDExtension of our
own in `native/`. Godot draws, takes input, saves and talks to Steam; every
rule is the web build's. Read 5-wild's `CLAUDE.md` first: engine purity,
`RunState` as plain JSON and "the engine authors no prose" hold here exactly as
they do there, because it is the same engine.

The project began as `templates/godot-template` in the monorepo, and the
tooling (scripts, agent bridge, autoloads, gotchas) is the template's.

## Commands

Every script sources `scripts/env.sh`, which points Godot's user dirs into
`.godot-home/`, so nothing scripted touches a real save or editor setting.

- First run, and after a pin bump: `npm ci` in 5-wild, then
  `scripts/setup.sh` (Godot at the pinned version if yours differs, the
  addons, the native library, the bundle, the first import).
  `scripts/setup.sh system` for the apt packages; `templates` for exporting.
- After changing anything in `src/engine` or `js/`: `scripts/bundle.sh`.
  After changing `native/`: `scripts/build-native.sh` (`windows` for the
  `.dll`, which needs mingw-w64). Godot must be restarted to load a rebuilt
  library; `reloadable = false`.
- Tests: `scripts/test.sh [res://tests/engine ...]` (gdUnit4, always under Xvfb
  so no window opens; `XVFB=0` to watch it).
- Everything static: `scripts/lint.sh` (gdlint, gdformat, `check.sh`,
  `resave.sh --check`). `scripts/format.sh` fixes the formatting. The
  TypeScript in `js/` is checked by 5-wild's own `tsc` and biome.
- All of it: `prek run --all-files --hook-stage manual` on top of
  `prek run --all-files`. The three Godot hooks (check, resave, gdUnit4) are
  `manual` for now, so neither a commit nor the monorepo's CI runs them; CI
  would have to build Godot, the native library and the bundle first.
- After editing a `.tscn`/`.tres` by hand: `scripts/resave.sh`, then read
  the diff.
- Run and drive the game: `XVFB=1 scripts/play.sh`, then `scripts/bridge.py
  <cmd>` (`tree`, `get`, `set`, `call`, `eval`, `action`, `key`, `click`,
  `wait`, `screenshot`, `quit`; `scripts/bridge.py --help`). Type a guess
  with `key C`, `key R`, … and `action submit`.
- Look at it: `scripts/screenshot.sh out.png`, then read the PNG. Uses a
  running `play.sh` game if there is one, else starts and stops its own.
- Release builds: `scripts/export.sh [preset]` into `build/`. CI does the same
  on every push and attaches the zips to a tag's release; see
  `../.github/workflows/desktop.yml`.

## The shape of the thing

Four layers, and the seam between each pair is a string.

`native/` is `JsContext`, a `RefCounted` wrapping one QuickJS runtime:
`load(source)`, `invoke("dotted.path", [args])`, `get_error()`. Strings in,
a string out, and deliberately nothing richer: `RunState` is plain JSON by
contract, so JSON text crosses losslessly, and a converter between JS values
and Variants would be the one piece of this that could get a number wrong.
godot-cpp and quickjs-ng are fetched by CMake at pinned hashes
(`native/CMakeLists.txt`).

`js/engine.ts` is the engine alone, a global `fivewild` with `setWords`,
`start`, `dispatch`, `state` and `resume`, and `core/wild.gd` (`Wild`) is its
GDScript face. Only the golden tests drive it now; it is not exported.

`js/shell.ts` is the game. It is `src/ui/app.ts`'s controller half (which
screen a phase gets, what a tap dispatches, when the intro card shows, what
the record counts), running the web build's own `src/ui/views.ts` against
`js/dom-shim.ts`, a fake DOM that serializes what the views build into a JSON
tree. Its header says what was kept out and why (telemetry, `seal`, and every
workaround `app.ts` carries for a browser). Two consequences:

- **The prose is the web's.** The four `src/ui/lang` catalogs are bundled
  with the views, so there is no `.po` and nothing to translate here.
  `dom-shim.ts` supplies the `Intl.PluralRules` and `ListFormat` they build at
  import (QuickJS has no `Intl`), for all four locales. `js/intl-shim.ts` is
  the English-only one the golden bundle still uses.
- **The shell holds the run**, as `app.ts` does, because Godot's JSON writer
  rounds floats and a state that went out and back on every action would
  come back changed. Godot holds only the *store*: the web build's
  `localStorage` keys (`5wild:run:v2`, `5wild:meta:v2`, the settings; see
  5-wild's CLAUDE.md), as text, which `flush` hands out after anything writes
  and `main.gd` saves as one `Saves` slot, `store`. The run save is unsealed
  JSON here.

`core/shell.gd` (`Shell`) is the only file that calls it. Every input
returns *effects*, `{render, cues, toast, shake, bump, animate, open}`, and
`game/main.gd` does what they say, in the web's order: draw, then play over
the drawing.

`game/` draws the trees:

- `render.gd` builds a tree into `Box`es, whole, on every render, as the web
  does. It keeps what the browser would have: which `<details>` are open,
  links, tooltips. `focus.gd` keeps the rest of that: Tab, the ring, and
  which box the keyboard is on across a rebuild.
- `rules.gd` is `src/style.css` as data, one `[selector, props]` pair per
  rule in the sheet's order. `style.gd` is the matcher, the prop legend and
  the palettes. Porting a CSS change means editing the matching rule here.
- `box.gd` is a CSS-ish box: layout (col, row, grid, and the game's own
  board, tiles, keys, meter), background, border, text. `flex.gd` holds its
  width algorithms, pure, so `box.gd` stays under gdlint's file length.
- `animator.gd` performs the scoring cascade from a script the shell wrote
  (`script` in `shell.ts`), restyling boxes by class; it never reads an event.
  `sound.gd` plays the cues with the web's recordings and levels.

`words/`, `audio/` and `js/build/` are copied and built by `scripts/bundle.sh`
from 5-wild's `public/words`, `src/ui/sounds`, `src/ui/tracks` and the
TypeScript, and are gitignored. The bundle's banner sets `__BUILD_VERSION__`
and `__BUILD_COMMIT__` from 5-wild's `package.json` and HEAD, as Vite's
defines do. The export presets include `js/build/shell.js` and `words/*` by
name, since neither is a resource Godot finds by itself, and exclude
`engine.js` and `golden.js`.

The layout unit is the CSS pixel: `content_scale_factor` is viewport height
/ 800, so a rule's `16` is the web's `1rem` on an 800px-tall phone.

## The engine is tested by the web build's vectors

`tests/engine/test_golden.gd` replays 5-wild's golden vectors
(`../test/golden/vectors.json`) two ways. Parity: `js/golden.ts` runs the
recorder's own `replay` inside QuickJS and must match what Node recorded, to
the chip. That is the portability contract `test/golden.test.ts` states, and
it is what makes "the same game" a claim rather than a hope. The seam: the same
actions driven one `Wild.dispatch` at a time from GDScript, with a save and a
resume in a fresh engine halfway, must end at exactly the state the engine
reaches on its own. A balance change re-records the vectors on the web side,
and these follow with nothing to update here; a `contentVersion` mismatch
means `scripts/bundle.sh` has not been run since.

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
  input there sees each event twice. Use `_input`. It also runs before any
  control sees a key, which is why Tab and Enter-on-a-focused-box are routed
  there (`_navigate`) rather than left to Godot's own focus handling.
- **GodotSteam registers an engine singleton named `Steam`**, so an
  autoload of that name collides. Ours is `Platform`, and it reaches Steam
  through `Engine.get_singleton("Steam").call(...)` so nothing fails to
  parse where the extension is missing.
- **`seed` is a global function**, and a variable named `seed` shadows it
  with a warning. It is `run_seed` throughout.
- **Reloading the running script with `CACHE_MODE_IGNORE` crashes the VM**
  (`tools/check.gd` skips itself).
- **Synthetic key events carry no unicode.** gdUnit's `simulate_key_pressed`
  and the bridge's `key` set a keycode only, so `_letter` in `game/main.gd`
  falls back to it; a player's keypress is read by its printed character,
  which is what makes AZERTY type an A where the A is.
- **The engine keeps words lowercase** (the draft, the answer, every tile's
  letter); the board uppercases what it draws.
- **`z_index` is sorted across the whole canvas**, not within a parent.
  A key's count badge at `z_index = 2` drew through the pause sheet until
  `main.gd` put the sheet layer at 10 and the effects layer at 20.
- **The content scale leaves hairlines fractional.** At 720px tall a 1px
  border is 0.9 of a screen pixel, and a `StyleBoxFlat` with square corners
  drew some of them and not others, depending on scroll offset. Snapping
  transforms to pixels only moved which. `Box._hairlines` draws square
  borders itself, never thinner than a screen pixel; the scale it measures
  against is `get_viewport().get_final_transform()`, not the canvas's.
- **`Box.set_class` restyles**, which re-matches the rules and discards
  anything written to `own` by hand. Change a box by class, as the animator
  does, or the next restyle undoes it.
- **A key during the cascade skips it and is dropped**, as the web drops
  keys while `busy`; a key on the intro card dismisses it. A script typing
  guesses must wait for `_animator.running` to go false, not a fixed time.
- **Keyboard focus is `game/focus.gd`, on the web's rules, not Godot's.**
  Tab visits what a browser's would (a handler, a link, `tabindex >= 0`);
  an open `.sheet` traps it by taking `focus_mode` off everything outside;
  the ring shows only after a key moved focus (`:focus-visible`); and a
  rebuild keeps focus only on a box wearing the same `data-focus` name
  (`holdFocus`). Enter over the board's own keyboard still submits. Arrows
  and a pad's D-pad use Godot's neighbour search inside the same fence, and
  a focused box does not show its tip, which the web's does.
- **The tests erase the `store` save**, and `play.sh` shares `.godot-home`
  with them, so a test run ends whatever you were playing.
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
controller would. `main.gd` has four methods for it: `state()` (the run),
`showing(kind)` (is that screen or sheet up), `press(name)` (a key by the
shell's name) and `start_seeded(n)` (a run on a known seed, so
`state()["round"]["answer"]` can be typed in).

## Conventions

- Typed GDScript throughout: `untyped_declaration` is an error in
  `project.godot`. Doc comments (`##`) on anything a reader would ask about.
- Scenes are written as the editor would write them; `resave --check` in
  pre-commit holds that. Use unique names (`%Score`) for nodes a script
  touches, so moving them in the tree doesn't break the script.
- Pins live in `scripts/versions.env`, each archive with its sha256.
  Bumping one means editing it and running `scripts/setup.sh`, which prints
  the new sum on mismatch. `GDTOOLKIT_VERSION` moves with the gdtoolkit rev
  in `.pre-commit-config.yaml`.
- Third-party addons (`gdUnit4`, `godotsteam`) are gitignored and installed
  by `setup.sh`. The `.gd.uid` and `.import` files are committed.
- The native pins (godot-cpp, quickjs-ng) live in `native/CMakeLists.txt`
  with their sha256, not in `versions.env`: CMake fetches them, and nothing
  else reads them.
