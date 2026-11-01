#!/usr/bin/env bash
# Renders a biome's replay to captures/<biome>-<quality>.mp4 at a fixed frame rate (60 unless
# given), however long each frame takes, so max quality shows what the world can look like
# on any GPU. Opens a window on your desktop.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"

biome="${1:-forest}"
quality="${2:-max}"
fps="${3:-60}"
# A render can take half an hour: find out now, not at the end of it.
command -v ffmpeg >/dev/null || {
	echo "capture.sh needs ffmpeg" >&2
	exit 1
}
mkdir -p captures
"$GODOT" --path "$ROOT" --audio-driver Dummy --resolution 1920x1080 --fixed-fps "$fps" \
	--write-movie "$ROOT/captures/$biome-$quality.avi" -- --biome="$biome" --quality="$quality"
ffmpeg -y -loglevel error -i "$ROOT/captures/$biome-$quality.avi" -c:v libx264 -crf 20 \
	-preset slow -pix_fmt yuv420p -movflags +faststart "$ROOT/captures/$biome-$quality.mp4"
rm "$ROOT/captures/$biome-$quality.avi"
