#!/bin/bash
# The pod's own build queue, so its GPU always has work: the same runner as here (oc_queue.sh), on
# the pod, over its own out/oc/queue. Jobs are written as for this machine's queue. A job's
# `git ... merge --ff-only SHA` line isn't run there; the source at SHA goes up with the job and is
# unpacked over the pod's project when the job starts, so queued jobs don't change each other's.
#
#   scripts/pod_queue.sh add NAME JOB.sh   # queue a job on the pod (and start its runner)
#   scripts/pod_queue.sh list              # waiting, done and failed there; what's running
#   scripts/pod_queue.sh wait NAME         # block till NAME ends there, then bring its OC_STILLS back
#   scripts/pod_queue.sh sync              # push here to R2 and pull on the pod; only while it's idle
#
# The pod is $POD_HOST:$POD_PORT, or out/lane/pod.env's POD_HOST= and POD_PORT= lines.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd -P)
[ -n "${POD_HOST:-}" ] || . /workspace/ue/out/lane/pod.env
ssh_cmd=(ssh -p "$POD_PORT" -o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30 -o BatchMode=yes)
on_pod() { "${ssh_cmd[@]}" "root@$POD_HOST" "$@"; }
Q=/workspace/ue/out/oc/queue

start_runner() {
	on_pod "cat > /root/oc_queue.sh" <"$here/oc_queue.sh"
	# A second runner sees the first's pid file and exits.
	on_pod "mkdir -p $Q && cd /workspace/ue && { nohup bash /root/oc_queue.sh run >>$Q/runner.log 2>&1 </dev/null & }"
}

case "${1:-}" in
add)
	name=${2:?job name} job=${3:?job script}
	sha=$(grep -oP 'merge -q --ff-only \K[0-9a-f]{7,40}' "$job" || true)
	body=$(grep -v 'git -C /workspace/veloverse merge' "$job")
	# The pod's editor can hang for minutes quitting before its watchdog kills it, so a restart
	# there outlasts oc.py's 300 s wait for the editor to come back.
	body=$(sed -E 's#(scripts/oc\.py [a-z-]+)( |$)#\1 --boot-timeout 1200\2#' <<<"$body")
	if [ -n "$sha" ]; then
		sha=$(git -C /workspace rev-parse "$sha")
		tar=/workspace/ue/out/lane/src_${sha:0:12}.tar
		if ! on_pod "test -s $tar"; then
			on_pod "mkdir -p /workspace/ue/out/lane && cat > $tar.tmp && mv $tar.tmp $tar" < <(git -C /workspace archive \
				"$sha:veloverse" -- scripts README.md VeloverseUE/Source VeloverseUE/Config VeloverseUE/VeloverseUE.uproject)
		fi
		# After the header comments, so a "# needs:" line still reads first.
		body=$(awk -v t="$tar" '!done && !/^#/ {print "tar -xf " t " -C /workspace/ue --no-same-owner"; done=1} {print}' <<<"$body")
	fi
	on_pod "mkdir -p $Q && cat > $Q/$name.sh.tmp && bash -n $Q/$name.sh.tmp && mv $Q/$name.sh.tmp $Q/$name.sh" <<<"$body"
	start_runner
	echo "queued $name on the pod${sha:+ (source ${sha:0:8})}"
	;;
list)
	on_pod "bash /root/oc_queue.sh list; cd $Q && for a in *.active; do [ -e \"\$a\" ] && echo \"running: \${a%.active}\"; done; true"
	;;
wait)
	name=${2:?job name}
	on_pod "bash /root/oc_queue.sh wait $name"
	stills=$(on_pod "cat $Q/done/$name.sh $Q/failed/$name.sh 2>/dev/null" | grep -oP '^export OC_STILLS=\K\S+' | head -1 || true)
	if [ -n "$stills" ]; then
		bash "$here/pod_sync.sh" "$POD_HOST" "$POD_PORT" pull "${stills#/workspace/ue/}"
		echo "stills in $stills"
	fi
	;;
sync)
	if on_pod "ls $Q/*.active" >/dev/null 2>&1; then
		echo "a job is running on the pod; sync between jobs" >&2
		exit 1
	fi
	bash "$here/r2_sync.sh" push
	r2_env=${R2_POD_ENV:-$HOME/.config/opencycle/r2-read.env}
	[ -r "$r2_env" ] || r2_env=$HOME/.config/opencycle/r2.env
	on_pod "umask 077 && mkdir -p /root/.config/opencycle && cat > /root/.config/opencycle/r2.env" <"$r2_env"
	on_pod "cat > /root/r2_sync.sh" <"$here/r2_sync.sh"
	# shellcheck disable=SC2016 # expands on the pod
	on_pod 'bash /root/r2_sync.sh pull /workspace/ue; rc=$?; rm -f /root/.config/opencycle/r2.env; exit $rc'
	;;
*)
	sed -n '2,12p' "$0" >&2
	exit 2
	;;
esac
