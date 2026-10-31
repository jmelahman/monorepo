#!/usr/bin/env bash
# Compiles every script we own with the autoloads loaded, and fails on any
# error or warning. See tools/check.gd.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
# -d: GDScript reports warnings only with a debugger attached.
"$GODOT" --headless --path "$ROOT" -d -s res://tools/check.gd </dev/null
