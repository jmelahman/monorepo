#!/bin/bash
# Rides L_Coast at each quality preset from a few places along the loop, uncapped, and keeps the CSV
# profiler's frame and GPU times: out/ride_bench/q<preset>_<metres>.csv. Run it on a real GPU (the
# pod); the local iGPU shares the desktop and its numbers mean nothing.
#   scripts/ride_bench.sh                     presets 0-3 from 1200, 6000, 12000 and 18000 m
#   OC_PRESETS="0 3" OC_STARTS="6000" OC_FRAMES=900 scripts/ride_bench.sh
# Then: python3 scripts/bench_summary.py out/ride_bench/*.csv --skip=600
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/VeloverseUE/VeloverseUE.uproject
OUT=/workspace/ue/out/ride_bench
SAVED=/workspace/ue/VeloverseUE/Saved/Profiling/CSV
mkdir -p "$OUT" && chmod a+rwx "$OUT"
for preset in ${OC_PRESETS:-0 1 2 3}; do
	for start in ${OC_STARTS:-1200 6000 12000 18000}; do
		rm -rf "$SAVED"
		# The first frames compile shaders and stream the level in: bench_summary.py skips them.
		# shellcheck disable=SC2024 # the log is written as the caller
		timeout "${OC_TIMEOUT:-900}" sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 \
			"$UE/UnrealEditor" "$PROJECT" /Game/Coast/L_Coast -game -RenderOffscreen -Unattended \
			-NoLoadingScreen -windowed -resx=1920 -resy=1080 -nosplash \
			-RideWatts=220 -RideStart="$start" -RideQuality="$preset" \
			-csvCaptureFrames="${OC_FRAMES:-2100}" -ExitAfterCsvProfiling -csvGpuStats \
			-ExecCmds="t.MaxFPS 0, r.VSync 0" -log -stdout -FullStdOutLogOutput \
			>"$OUT/q${preset}_$start.log" 2>&1 </dev/null
		# shellcheck disable=SC2012 # newest by mtime, which find cannot sort
		csv=$(ls -t "$SAVED"/*.csv 2>/dev/null | head -1)
		if [ -n "$csv" ]; then
			cp "$csv" "$OUT/q${preset}_$start.csv" && echo "q$preset from $start m: $OUT/q${preset}_$start.csv"
		else
			echo "q$preset from $start m: no CSV"
		fi
	done
done
