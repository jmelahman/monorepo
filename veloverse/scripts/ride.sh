#!/bin/bash
# Rebuilds the ride's sequence and render config, then renders it offscreen into OC_OUT.
# Runs on the pod as root; Unreal itself runs as ue4. Settings pass through to make_ride.py.
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/VeloverseUE/VeloverseUE.uproject
export OC_OUT=${OC_OUT:-/workspace/ue/out/ride}
LEVEL=${OC_LEVEL:-/PCGBiomeSample/Maps/BiomeSampleLevel}
EXEC=()
[ -n "${OC_EXEC:-}" ] && EXEC=(-ExecCmds="$OC_EXEC")
as_ue() { sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 OC_OUT="$OC_OUT" \
	OC_LEVEL="$LEVEL" OC_SECONDS="${OC_SECONDS:-}" OC_SPEED="${OC_SPEED:-}" OC_EYE="${OC_EYE:-}" OC_RES="${OC_RES:-}" OC_AT="${OC_AT:-}" OC_YAW="${OC_YAW:-}" OC_PITCH="${OC_PITCH:-}" OC_EV="${OC_EV:-}" \
	OC_CURVE="${OC_CURVE:-}" OC_WAVELENGTH="${OC_WAVELENGTH:-}" "$@"; }

rm -rf "$OC_OUT" && mkdir -p "$OC_OUT" && chmod a+rwx "$OC_OUT"
cd /workspace/ue || exit 1
as_ue "$UE/UnrealEditor-Cmd" "$PROJECT" -run=pythonscript -script=/workspace/ue/scripts/make_ride.py \
	-unattended -nosplash -stdout -FullStdOutLogOutput >make_ride.log 2>&1 </dev/null
grep -E "RIDE|Error" make_ride.log | grep -v Deprecation | cut -c30-
grep -q "RIDE saved" make_ride.log || exit 1
start=$(date +%s)
as_ue "$UE/UnrealEditor" "$PROJECT" "$LEVEL" -game -LevelSequence=/Game/Spike/SEQ_Ride \
	-MoviePipelineConfig=/Game/Spike/MRQ_Ride -RenderOffscreen -Unattended -NoLoadingScreen -windowed \
	-resx=1920 -resy=1080 -nosplash -log -stdout -FullStdOutLogOutput "${EXEC[@]}" >render.log 2>&1 </dev/null
grep -E "Movie Pipeline completed|Fatal" render.log | cut -c30-
# shellcheck disable=SC2012 # only counting entries
echo "rendered $(ls "$OC_OUT" | wc -l) frames in $(($(date +%s) - start)) s"
