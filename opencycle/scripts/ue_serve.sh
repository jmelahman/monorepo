#!/bin/bash
# Keeps the editor server (ue_serve.py) up: when the editor dies, the command it was running is
# marked .crashed (its log tail in .crashed.txt) and a fresh editor starts on the light map, so a
# crash costs a minute's restart instead of a stopped loop. It stops after quit.py, or after three
# crashes within ten minutes, which means something is wrong that a restart won't fix.
# Usage, from /workspace/ue: nohup bash scripts/ue_serve.sh >out/serve_supervisor.log 2>&1 &
set -u
cd /workspace/ue || exit 1
CMD=${OC_CMD:-/workspace/ue/out/cmd}
LOG=serve.log
mkdir -p "$CMD" out/serve_logs
crashes=()
while true; do
	# One Unreal at a time on this machine, across every session.
	while pgrep -x UnrealEditor >/dev/null || pgrep -x UnrealEditor-Cmd >/dev/null; do
		echo "$(date +%T) another Unreal is running; waiting"
		sleep 30 &
		wait $!
	done
	echo "$(date +%T) starting the editor server"
	OC_EDITOR=1 OC_START_MAP=/Engine/Maps/Templates/OpenWorld OC_TAIL=0 \
		bash scripts/ue_run.sh ue_serve.py "$LOG"
	stamp=$(date +%Y%m%d-%H%M%S)
	cp "$LOG" "out/serve_logs/serve-$stamp.log"
	if tr -d '\000' <"$LOG" | grep -a -q "SERVE run quit.py"; then
		echo "$(date +%T) quit by command"
		break
	fi
	for running in "$CMD"/*.running; do
		[ -e "$running" ] || continue
		mv "$running" "${running%.running}.crashed"
		tr -d '\000' <"$LOG" | grep -a -v "r.GPUCrash" | tail -60 >"${running%.running}.crashed.txt"
		echo "$(date +%T) the editor died running $(basename "$running"); marked .crashed"
	done
	now=$(date +%s)
	recent=()
	for t in "${crashes[@]}" "$now"; do [ $((now - t)) -lt 600 ] && recent+=("$t"); done
	crashes=("${recent[@]}")
	if [ "${#crashes[@]}" -ge 3 ]; then
		echo "$(date +%T) three exits in ten minutes; stopping (log: out/serve_logs/serve-$stamp.log)"
		break
	fi
	echo "$(date +%T) restarting"
done
