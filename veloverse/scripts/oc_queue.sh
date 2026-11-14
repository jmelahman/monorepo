#!/usr/bin/env bash
# The build queue: runs out/oc/queue/*.sh one at a time, in name order, until none are left, so the
# one Unreal on this machine is always busy with queued work. Each step is a bash script (set -e is
# on) that sets its own level env and calls oc.py, ride_test.sh, package.sh and so on; add steps
# while it runs (`oc_queue.sh add NAME <<'EOF' ... EOF`). A finished step moves to queue/done, a
# failed one to queue/failed, each with its log beside it; a failure doesn't stop the steps after it,
# unless a later step says "# needs: NAME" (it's then skipped). Exits when the queue is empty, printing a
# summary of what ran.
#
#     scripts/oc_queue.sh add 20_world_rides < steps.sh   # queue a step (name sets the order)
#     scripts/oc_queue.sh run                              # work through the queue (background it)
#     scripts/oc_queue.sh list                             # waiting, done and failed steps
#     scripts/oc_queue.sh wait NAME                        # block till step NAME ends (background it)
#     scripts/oc_queue.sh idle [SECONDS]                   # a line when idle, repeated every SECONDS (300)
#
# Keep the next step queued before the running one ends, so the editor never idles: review a
# step's stills while the next one builds, with `wait NAME` as the per-step notification. Before
# each step the runner waits out any oc.py or ue_run.sh run started outside the queue
# (the oc server's own editor doesn't count).
set -u
Q=/workspace/ue/out/oc/queue
# Steps start in the live checkout here; on the pod (scripts/pod_queue.sh), which has none, in /workspace/ue.
home=/workspace/veloverse
[ -d "$home" ] || home=/workspace/ue
mkdir -p "$Q/done" "$Q/failed"
shopt -s nullglob

# The step scripts in a directory, by name; newest first with -t.
steps() {
	local files=("$1"/*.sh)
	[ ${#files[@]} -eq 0 ] && return
	if [ "${2:-}" = -t ]; then ls -t "${files[@]}"; else printf '%s\n' "${files[@]}" | sort; fi | xargs -n1 basename
}

case "${1:-run}" in
add)
	name="${2:?step name}"
	cat >"$Q/$name.sh"
	echo "queued $name"
	;;
idle)
	# A line whenever the machine sits idle, and again every EVERY seconds (default 300) while it
	# stays that way: the queue empty, or steps waiting with no runner to start them (it exits when
	# the queue empties, so steps added after that wait for nothing). For an agent's monitor.
	every="${2:-300}"
	since=0 last=0
	while true; do
		waiting=$(steps "$Q" | wc -l)
		active=("$Q"/*.active)
		state=""
		if [ ${#active[@]} -eq 0 ] && [ "$waiting" -eq 0 ]; then
			state="queue empty"
		elif ! pgrep -f 'oc_queue.sh run' >/dev/null; then
			state="$waiting steps waiting and no runner"
		fi
		now=$(date +%s)
		if [ -z "$state" ]; then
			since=0 last=0
		else
			[ "$since" -eq 0 ] && since=$now
			if [ "$last" -eq 0 ] || [ $((now - last)) -ge "$every" ]; then
				echo "$(date +%H:%M) $state, idle $(((now - since) / 60)) min"
				last=$now
			fi
		fi
		sleep 20
	done
	;;
list)
	echo "waiting:"
	steps "$Q"
	echo "done:"
	steps "$Q/done" -t | head -10
	echo "failed:"
	steps "$Q/failed" -t | head -10
	;;
run)
	if [ -e "$Q/.running" ] && kill -0 "$(cat "$Q/.running")" 2>/dev/null; then
		echo "a runner is already working (pid $(cat "$Q/.running"))"
		exit 0
	fi
	echo $$ >"$Q/.running"
	trap 'rm -f "$Q/.running"' EXIT
	summary=()
	while step=$(steps "$Q" | head -1) && [ -n "$step" ]; do
		# A build started by hand finishes first: only one Unreal at a time.
		# The oc server's own editor (ue_run.sh oc_server.py) stays up between jobs: not a build.
		while pgrep -af "scripts/oc.py |scripts/ue_run.sh|UnrealEditor-Cmd" | grep -v oc_server.py |
			grep -q .; do
			sleep 20 &
			wait $!
		done
		step=$(steps "$Q" | head -1)
		name="${step%.sh}"
		# "# needs: NAME" in a step: skip it when that step failed (or was skipped).
		need=$(sed -n 's/^# needs: *//p' "$Q/$step" | head -1)
		if [ -n "$need" ] && [ ! -e "$Q/done/$need.sh" ]; then
			mkdir -p "$Q/skipped"
			mv "$Q/$step" "$Q/skipped/$step"
			summary+=("skipped $name: $need didn't finish")
			echo "${summary[-1]}"
			continue
		fi
		mv "$Q/$step" "$Q/$name.active"
		start=$(date +%s)
		echo "== $name ($(date +%H:%M))"
		(cd "$home" && bash -e "$Q/$name.active") >"$Q/$name.log" 2>&1
		code=$?
		mins=$((($(date +%s) - start + 59) / 60))
		where="done"
		[ $code -eq 0 ] || where="failed"
		mv "$Q/$name.active" "$Q/$where/$step"
		mv "$Q/$name.log" "$Q/$where/$name.log"
		summary+=("$where $name (${mins} min, exit $code): $(tail -1 "$Q/$where/$name.log" | cut -c1-140)")
		echo "${summary[-1]}"
	done
	echo "== queue empty ($(date +%H:%M))"
	printf '%s\n' "${summary[@]}"
	;;
wait)
	name="${2:?step name}"
	until [ -e "$Q/done/$name.sh" ] || [ -e "$Q/failed/$name.sh" ] || [ -e "$Q/skipped/$name.sh" ]; do
		sleep 20 &
		wait $!
	done
	for where in "done" failed skipped; do
		[ -e "$Q/$where/$name.sh" ] && echo "$where $name" && tail -"${OC_TAIL:-5}" "$Q/$where/$name.log" 2>/dev/null
	done
	echo "next: $(steps "$Q" | head -1 || true)"
	;;
*)
	echo "usage: oc_queue.sh add NAME | run | list | wait NAME" >&2
	exit 2
	;;
esac
