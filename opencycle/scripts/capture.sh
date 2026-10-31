#!/usr/bin/env bash
# Renders a biome's replay to captures/<biome>-<quality>.avi at a fixed 60 fps,
# however long each frame takes, so max quality shows what the world can look like
# on any GPU. Opens a window on your desktop.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"

biome="${1:-forest}"
quality="${2:-max}"
mkdir -p captures
"$GODOT" --path "$ROOT" --audio-driver Dummy --resolution 1920x1080 --fixed-fps 60 \
	--write-movie "$ROOT/captures/$biome-$quality.avi" -- --biome="$biome" --quality="$quality"
