#!/usr/bin/env bash
# Bundles the TypeScript engine for QuickJS and copies in the word lists it
# deals from. Both are generated, gitignored, and rebuilt by `scripts/setup.sh`.
#
#   js/build/engine.js   src/engine behind the `fivewild` global (js/engine.ts)
#   js/build/golden.js   that plus the golden replay, for tests/engine only
#   js/build/shell.js    src/ui's controller and views over a fake DOM (js/shell.ts)
#   words/<lang>/        ../public/words, which a Godot export cannot reach
#   audio/sounds/        ../src/ui/sounds, the web build's recordings
#   audio/tracks/        ../src/ui/tracks, its music
#
# rolldown is Vite's own bundler, already in 5-wild's node_modules, so this
# adds no dependency; it needs `npm ci` in 5-wild first. The output is a classic
# script (iife), since the engine is run with JS_EVAL_TYPE_GLOBAL, and it is
# not minified, so a stack from QuickJS names the functions in src/engine.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
web="$ROOT/.."
if [[ ! -x $web/node_modules/.bin/rolldown ]]; then
	echo "no rolldown in $web/node_modules; run npm ci there" >&2
	exit 1
fi
# Vite's two `define`s, as vite.config.ts computes them: the version and the
# commit the about sheet and the title's corner print. A banner rather than a
# define, because it sets them on the global before anything reads them, which
# is what js/dom-shim.ts falls back from.
version=$(sed -n 's/^  "version": "\(.*\)",$/\1/p' "$web/package.json")
commit=$(git -C "$web" rev-parse --short=7 HEAD 2>/dev/null || true)
banner="globalThis.__BUILD_VERSION__ = \"$version\"; globalThis.__BUILD_COMMIT__ = \"$commit\";"
for entry in engine golden shell; do
	"$web/node_modules/.bin/rolldown" "$ROOT/js/$entry.ts" --format iife \
		--platform neutral --banner "$banner" --file "$ROOT/js/build/$entry.js" >/dev/null
done
for dir in "$web"/public/words/*/; do
	lang=$(basename "$dir")
	mkdir -p "$ROOT/words/$lang"
	# No .gdignore: Godot imports nothing from .txt, and one would also keep
	# the lists out of an export.
	cp "$dir"/answers.txt "$dir"/allowed.txt "$ROOT/words/$lang/"
done
# The recordings are imported like any resource (the next import picks them
# up), so unlike the word lists they need no mention in the export presets.
mkdir -p "$ROOT/audio/sounds" "$ROOT/audio/tracks"
cp "$web"/src/ui/sounds/*.ogg "$ROOT/audio/sounds/"
cp "$web"/src/ui/tracks/*.ogg "$ROOT/audio/tracks/"
