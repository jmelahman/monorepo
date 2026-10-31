#!/usr/bin/env bash
# Rebuilds assets/baked/ from Poly Haven's source trees. Downloads 1.4 GB the
# first time and needs a real GPU, so it opens a window on your desktop. Rarely
# run: its output is committed.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"

python3 -I scripts/fetch_assets.py --bake-sources
python3 -I scripts/fetch_people.py
log="$(mktemp)"
trap 'rm -f "$log"' EXIT
"$GODOT" --path "$ROOT" --audio-driver Dummy --resolution 640x360 \
	-s res://tools/bake_foliage.gd 2>&1 | tee "$log"
"$GODOT" --path "$ROOT" --audio-driver Dummy --resolution 640x360 \
	-s res://tools/bake_signs.gd 2>&1 | tee -a "$log"
"$GODOT" --path "$ROOT" --audio-driver Dummy --resolution 640x360 \
	-s res://tools/bake_people.gd 2>&1 | tee -a "$log"
if grep -E "$GODOT_ERROR_PATTERN" "$log" >/dev/null; then
	echo "bake.sh: Godot reported errors (above)" >&2
	exit 1
fi
"$GODOT" --headless --path "$ROOT" --import >/dev/null 2>&1
