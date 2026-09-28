#!/usr/bin/env bash
# Runs the gdUnit4 suites under tests/ and exits non-zero on any failure.
#
#   scripts/test.sh                        everything
#   scripts/test.sh res://tests/sim        one directory (or one file)
#
# Under Xvfb when there is no display: gdUnit4 refuses --headless by default,
# and rightly, since input simulation and rendering do nothing there. The
# reports land in .godot-home/reports/ (HTML and JUnit XML).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"

if [[ ! -f addons/gdUnit4/.version ]]; then
	echo "gdUnit4 is missing; run scripts/setup.sh" >&2
	exit 1
fi

paths=("$@")
[[ ${#paths[@]} -gt 0 ]] || paths=(res://tests)
add=()
for p in "${paths[@]}"; do add+=(-a "$p"); done

reports=.godot-home/reports
log="$(mktemp)"
trap 'rm -f "$log"' EXIT

# Settings and Platform read this to keep their hands off the window and Steam.
export GODOT_TEST=1
status=0
with_display "$GODOT" --path "$ROOT" --audio-driver Dummy \
	-s -d res://addons/gdUnit4/bin/GdUnitCmdTool.gd \
	"${add[@]}" -rd "$reports" -c 2>&1 | tee "$log" || status=$?
"$GODOT" --headless --path "$ROOT" --quiet -s res://addons/gdUnit4/bin/GdUnitCopyLog.gd \
	-rd "$reports" >/dev/null 2>&1 || true

# gdUnit4 exits 0 when a suite fails to parse and is silently skipped; a
# script error anywhere in the run is a failure here.
if grep -E "$GODOT_ERROR_PATTERN" "$log" >/dev/null; then
	echo "test.sh: Godot reported script errors (above)" >&2
	status=1
fi
exit "$status"
