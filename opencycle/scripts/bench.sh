#!/usr/bin/env bash
# Rides a biome's replay on the real GPU, as fast as it will go, and reports frame
# times to captures/bench-<biome>-<quality>.json. Opens a window on your desktop.
# Usage: bench.sh [forest|neon] [ship|max] [--rate=4 ...]
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"

biome="${1:-forest}"
quality="${2:-ship}"
mkdir -p captures
"$GODOT" --path "$ROOT" --audio-driver Dummy --resolution 1280x720 -- \
	--biome="$biome" --quality="$quality" \
	--bench="$ROOT/captures/bench-$biome-$quality.json" "${@:3}"
