#!/bin/bash
# Keeps exactly one warm editor (oc_server.py) up. Started by oc.py, never by hand. One supervisor
# at a time: it holds a lock on OC_DIR/supervisor.lock for as long as it runs.
#
# When the editor exits it starts a fresh one on the empty OpenWorld map (so nothing from a
# crashed or restarted session is loaded), and renames a job that was running <id>.crashed with
# the log tail inside. It stops when OC_DIR/stop exists, or after three exits in ten minutes
# (something a restart won't fix).
set -u
cd /workspace/ue || exit 1
DIR=${OC_DIR:-/workspace/ue/out/oc}
LOG=$DIR/editor.log
mkdir -p "$DIR/jobs" "$DIR/logs"
chmod 777 "$DIR" "$DIR/jobs" "$DIR/logs" 2>/dev/null
exec 9>"$DIR/supervisor.lock"
flock -n 9 || exit 0
rm -f "$DIR/stop"
crashes=()
while [ ! -e "$DIR/stop" ]; do
	# One Unreal at a time on this machine, across every session.
	while pgrep -x UnrealEditor >/dev/null || pgrep -x UnrealEditor-Cmd >/dev/null; do
		[ -e "$DIR/stop" ] && break 2
		echo "$(date +%T) another Unreal is running; waiting"
		sleep 10
	done
	echo "$(date +%T) starting the editor"
	# The level to work on (OC_SRC, OC_LEVEL, OC_STILLS), written by oc.py at each restart.
	unset OC_SRC OC_LEVEL OC_STILLS
	if [ -e "$DIR/level.env" ]; then
		set -a
		# shellcheck disable=SC1091
		. "$DIR/level.env"
		set +a
	fi
	# fd 9 (the lock) is closed for the editor, or a hung editor would hold the lock.
	OC_EDITOR=1 OC_START_MAP=/Engine/Maps/Templates/OpenWorld OC_TAIL=0 \
		bash scripts/ue_run.sh oc_server.py "$LOG" 9>&-
	stamp=$(date +%Y%m%d-%H%M%S)
	cp "$LOG" "$DIR/logs/editor-$stamp.log"
	# serve logs hold NUL bytes, and r.GPUCrashDebugging is a config line, not a crash.
	tail=$(tr -d '\000' <"$LOG" | grep -a -v "r.GPUCrash" | tail -80)
	for running in "$DIR"/jobs/*.running; do
		[ -e "$running" ] || continue
		printf '%s\n' "$tail" >"${running%.running}.crashed"
		chmod 666 "${running%.running}.crashed"
		rm -f "$running"
		echo "$(date +%T) the editor died running $(basename "$running"); marked .crashed"
	done
	if [ -e "$DIR/quitting" ]; then # asked to by oc.py restart/stop: not a crash
		rm -f "$DIR/quitting"
		continue
	fi
	now=$(date +%s)
	recent=()
	for t in "${crashes[@]}" "$now"; do [ $((now - t)) -lt 600 ] && recent+=("$t"); done
	crashes=("${recent[@]}")
	if [ "${#crashes[@]}" -ge 3 ]; then
		echo "$(date +%T) three exits in ten minutes; stopping (log: $DIR/logs/editor-$stamp.log)"
		break
	fi
	echo "$(date +%T) the editor exited; restarting"
done
rm -f "$DIR/stop"
