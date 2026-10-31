#!/usr/bin/env bash
# Rewrites every .tscn/.tres we own the way the editor would. Run it after
# editing a scene by hand, then read the diff. See tools/resave.gd.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
# --import refreshes the uid cache that resave.gd reads ext_resource uids from.
"$GODOT" --headless --path "$ROOT" --import >/dev/null 2>&1 || true
"$GODOT" --headless --path "$ROOT" -s res://tools/resave.gd -- "$@"
