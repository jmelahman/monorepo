#!/bin/bash
# Rides the coast unattended and offscreen at a held power, taking screenshots with the HUD, and
# reports how far and fast it got. Usage: scripts/ride_test.sh [WATTS] [SHOTS] [START_METRES] [ARGS...]
# e.g. scripts/ride_test.sh 220 10,40,80 0 -RideTrainer=sim. Extra arguments go to the game. Shots land in OC_OUT (default out/ride_test).
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/OpenCycleUE/OpenCycleUE.uproject
OUT=${OC_OUT:-/workspace/ue/out/ride_test}
cd /workspace/ue || exit 1
# shellcheck source=/dev/null
[ -f local.env ] && . ./local.env
res=${OC_RES:-1920x1080}
rm -rf "$OUT" && mkdir -p "$OUT" && chmod a+rwx "$OUT"
cvars=${OC_CVARS:+-dpcvars=${OC_CVARS//;/,}}
# shellcheck disable=SC2024,SC2086 # the log is written as the caller; cvars is empty or one word
timeout "${OC_TIMEOUT:-900}" sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 \
	"$UE/UnrealEditor" "$PROJECT" /Game/Coast/L_Coast -game -RenderOffscreen -Unattended -NoLoadingScreen \
	-windowed -resx="${res%x*}" -resy="${res#*x}" -nosplash -log -stdout -FullStdOutLogOutput $cvars \
	-RideWatts="${1:-220}" -RideShots="${2:-10,40,80}" -RideStart="${3:-0}" -RideShotDir="$OUT" "${@:4}" \
	>ride_test.log 2>&1 </dev/null
echo "exit $?"
grep -E "RIDE |Error|Fatal" ride_test.log | grep -v "LogPython" | cut -c30- | tail -30
ls "$OUT"
