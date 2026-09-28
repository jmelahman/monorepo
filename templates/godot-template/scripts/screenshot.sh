#!/usr/bin/env bash
# Writes a PNG of the game as it stands, for a person or an agent to look at.
# Uses the running game if scripts/play.sh started one, and leaves it running;
# otherwise starts one off-screen, lets it settle, and quits it afterwards.
#
#   scripts/screenshot.sh [out.png]     (default .godot-home/screenshot.png)
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
out=$(realpath -m "${1:-$ROOT/.godot-home/screenshot.png}")
bridge="$ROOT/scripts/bridge.py"

started=0
if ! "$bridge" ping >/dev/null 2>&1; then
	XVFB=1 "$ROOT/scripts/play.sh" >/dev/null
	started=1
	# The first frames are layout settling and the window being mapped.
	"$bridge" wait 30 >/dev/null
fi
"$bridge" screenshot "$out" >/dev/null
if ((started)); then
	"$bridge" quit >/dev/null
fi
echo "screenshot: $out"
