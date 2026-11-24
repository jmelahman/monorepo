#!/usr/bin/env bash
# Watches the build queue and prints one line per event, for a Monitor in the agent's session:
#   STEP   a step started or ended (with how long it took)
#   JOB    an oc.py job inside a step ended (DONE, FAILED, CRASHED, TIMEOUT, NO EDITOR); a
#          restart or stop that went fine isn't news
#   BEAT   every BEAT seconds: the step, its age, its last log line and the stills so far
#   STALL  nothing has moved for STALL seconds: no step log, editor log or new still
# A hang is noticed in minutes, and the STEP and JOB times show where iteration slowed.
# Usage: oc_watch.sh [BEAT_S=600] [STALL_S=480]
set -u
BEAT=${1:-600}
STALL=${2:-480}
Q=/workspace/ue/out/oc/queue
EDITOR_LOG=/workspace/ue/out/oc/editor.log
step="" started=0 last_beat=$(date +%s) stalled=0 seen=0 sig="" changed=$(date +%s) first=1

mtime() { stat -c %Y "$1" 2>/dev/null || echo 0; }
newest_still() { find /workspace/ue/out -maxdepth 5 -name '*.png' -mmin -$((STALL / 60)) -print -quit 2>/dev/null; }

while true; do
	now=$(date +%s)
	active=$(find "$Q" -maxdepth 1 -name '*.active' -print -quit)
	name=$(basename "${active:-}" .active)
	if [ "$name" != "$step" ]; then
		[ -n "$step" ] && echo "STEP ended $step after $(((now - started) / 60)) min"
		[ -n "$name" ] && echo "STEP started $name at $(date -d "@$(stat -c %W "$Q/$name.log" 2>/dev/null || date +%s)" +%H:%M)"
		# From the step's own start, its log's birth (the runner moves the step file in, so the
		# .active file keeps the time it was queued), so a watch armed mid-step times it right.
		step=$name started=$(stat -c %W "$Q/$name.log" 2>/dev/null || echo "$now") seen=0 changed=$now
		# Armed mid-step: only the jobs that end from now on.
		[ "$first" = 1 ] && seen=$(grep -cE '^(DONE|FAILED|CRASHED|TIMEOUT|NO EDITOR)' "$Q/$step.log" 2>/dev/null || true)
	fi
	if [ -n "$step" ]; then
		log="$Q/$step.log"
		# oc.py's last lines for each job: DONE, FAILED, CRASHED, TIMEOUT or NO EDITOR.
		total=$(grep -cE '^(DONE|FAILED|CRASHED|TIMEOUT|NO EDITOR)' "$log" 2>/dev/null || true)
		total=${total:-0}
		if [ "$total" -gt "$seen" ]; then
			grep -E '^(DONE|FAILED|CRASHED|TIMEOUT|NO EDITOR)' "$log" | tail -n $((total - seen)) |
				grep -vE '^DONE: .*(_restart|_stop|back on the empty map|have stopped)$' |
				sed "s/^/JOB $(date +%H:%M) /"
			seen=$total
		fi
		# oc.py's "... running, N s" lines go on while a job hangs: they don't count as progress.
		now_sig=$(grep -cv '^\.\.\. .* running, ' "$log" 2>/dev/null || true)
		[ "$now_sig" != "$sig" ] && sig=$now_sig changed=$now
		moved=$(printf '%s\n' "$changed" "$(mtime "$EDITOR_LOG")" | sort -n | tail -1)
		if [ $((now - moved)) -ge "$STALL" ] && [ -z "$(newest_still)" ]; then
			[ "$stalled" = 0 ] && echo "STALL $step: no log or still for $(((now - moved) / 60)) min;" \
				"last: $(tail -1 "$log" | cut -c1-160)"
			stalled=1
		else
			stalled=0
		fi
		if [ $((now - last_beat)) -ge "$BEAT" ]; then
			echo "BEAT $(date +%H:%M) $step, $(((now - started) / 60)) min in;" \
				"last: $(tail -1 "$log" | cut -c1-160)"
			last_beat=$now
		fi
	fi
	first=0
	sleep 30
done
