#!/bin/bash
# Rides L_Bench in real time at each scalability preset and writes the CSV profiler's frame
# times to out/bench/<preset>.csv. Build the bench level first with make_bench.py.
# OC_FRAMES is frames captured per preset (default 1500); OC_PRESETS the presets (default 0 1 2 3).
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/VeloverseUE/VeloverseUE.uproject
OUT=/workspace/ue/out/bench
SAVED=/workspace/ue/VeloverseUE/Saved/Profiling/CSV
mkdir -p "$OUT" && chmod a+rwx "$OUT"
for preset in ${OC_PRESETS:-0 1 2 3}; do
	rm -rf "$SAVED"
	# shellcheck disable=SC2024,SC2086 # log is written as the caller; OC_DISPLAY_ENV and OC_BENCH_ARGS are word lists
	sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 ${OC_DISPLAY_ENV:-} \
		"$UE/UnrealEditor" "$PROJECT" /Game/Spike/L_Bench -game -RenderOffscreen -windowed -resx=1920 -resy=1080 \
		-nosplash -NoLoadingScreen -Unattended -csvCaptureFrames="${OC_FRAMES:-1500}" \
		-ExitAfterCsvProfiling -csvGpuStats -ExecCmds="t.MaxFPS 0, r.VSync 0, scalability $preset" \
		-log -stdout -FullStdOutLogOutput ${OC_BENCH_ARGS:-} >"$OUT/$preset.log" 2>&1 </dev/null
	# shellcheck disable=SC2012 # newest by mtime, which find cannot sort
	csv=$(ls -t "$SAVED"/*.csv 2>/dev/null | head -1)
	[ -n "$csv" ] && cp "$csv" "$OUT/$preset.csv" && echo "preset $preset: $OUT/$preset.csv" || echo "preset $preset: no CSV"
done
