#!/bin/bash
# Builds the coast sequence (make_coast_ride.py; OC_* settings pass through) and renders it
# offscreen into OC_OUT. In stills mode it then keeps only the last frame of each held shot,
# as still_NN.png.
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/OpenCycleUE/OpenCycleUE.uproject
export OC_OUT=${OC_OUT:-/workspace/ue/out/coast}
cd /workspace/ue || exit 1
# Local machine only: lighter, throttled renders (see local.env).
# shellcheck source=/dev/null
[ -f local.env ] && . ./local.env
rm -rf "$OC_OUT" && mkdir -p "$OC_OUT" && chmod a+rwx "$OC_OUT"
OC_TAIL=10 bash scripts/ue_run.sh make_coast_ride.py make_coast_ride.log | grep -E "RIDE|Error"
grep -q "RIDE saved" make_coast_ride.log || exit 1
vars=()
while IFS='=' read -r name _; do vars+=("$name=${!name}"); done < <(env | grep '^OC_')
start=$(date +%s)
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 "${vars[@]}" \
	"$UE/UnrealEditor" "$PROJECT" /Game/Coast/L_Coast -game -LevelSequence=/Game/Coast/SEQ_Coast \
	-MoviePipelineConfig=/Game/Coast/MRQ_Coast -RenderOffscreen -Unattended -NoLoadingScreen -windowed \
	-resx=1920 -resy=1080 -nosplash -log -stdout -FullStdOutLogOutput >coast_render.log 2>&1 </dev/null
grep -E "Movie Pipeline completed|Fatal" coast_render.log | cut -c30-
# shellcheck disable=SC2012 # only counting entries
echo "rendered $(ls "$OC_OUT" | wc -l) frames in $(($(date +%s) - start)) s"
if [ "${OC_MODE:-ride}" = stills ]; then
	hold=${OC_HOLD:-40}
	n=$(echo "${OC_STILLS:-50,300,550,830,930,1010,1090,1300,1700,2050}" | tr ',' '\n' | wc -l)
	for ((i = 0; i < n; i++)); do
		f=$(printf "%s/frame.%04d.png" "$OC_OUT" $((i * hold + hold - 1)))
		[ -f "$f" ] && cp "$f" "$(printf "%s/still_%02d.png" "$OC_OUT" "$i")"
	done
	find "$OC_OUT" -name 'frame.*.png' -delete
	ls "$OC_OUT"
fi
if [ "${OC_MODE:-ride}" = ride ]; then
	# The first frames still stream in textures and Nanite pages (missing flowers, half-drawn
	# lattices), whatever the warm-up: drop them and renumber from 0.
	skip=${OC_SKIP:-8}
	# shellcheck disable=SC2012 # relies on sorted order
	for f in $(ls "$OC_OUT"/frame.*.png | sort | head -n "$skip"); do rm -f "$f"; done
	i=0
	# shellcheck disable=SC2012 # relies on sorted order
	for f in $(ls "$OC_OUT"/frame.*.png | sort); do
		mv "$f" "$(printf "%s/frame.%04d.png" "$OC_OUT" $i)" 2>/dev/null
		i=$((i + 1))
	done
fi
