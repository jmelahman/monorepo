#!/bin/bash
# Runs a queue job on the Runpod pod instead of here: a second Unreal, so two builds run at once.
#   scripts/pod_lane.sh HOST PORT JOB.sh
# The job is written as for out/oc/queue. Its `git ... merge --ff-only SHA` line is not run (that
# would move this machine's live checkout under its own build); the source at SHA goes to the pod
# instead, over the synced project. The job's other lines run there in /workspace/ue, under nohup
# so a dropped connection doesn't stop them, and its $OC_STILLS folder comes back here when done.
set -euo pipefail
host=$1 port=$2 job=$3
ssh_cmd=(ssh -p "$port" -o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30 -o BatchMode=yes)
on_pod() { "${ssh_cmd[@]}" "root@$host" "$@"; }
name=$(basename "$job" .sh)
sha=$(grep -oP 'merge -q --ff-only \K[0-9a-f]{7,40}' "$job" || true)
stills=$(grep -oP '^export OC_STILLS=\K\S+' "$job" || true)
here=$(cd "$(dirname "$0")" && pwd -P)

echo "== $(date +%T) the pod pulls the project from R2"
# Push this machine's changes to R2 first (only what changed goes up), then the pod pulls at
# datacenter speed: SSH to a pod is slow from here. The pod gets the key on its local disk for the
# pull and loses it after; a read-only key ($R2_POD_ENV) is preferred when there is one.
bash "$here/r2_sync.sh" push
r2_env=${R2_POD_ENV:-$HOME/.config/opencycle/r2-read.env}
[ -r "$r2_env" ] || r2_env=$HOME/.config/opencycle/r2.env
on_pod "umask 077 && mkdir -p /root/.config/opencycle && cat > /root/.config/opencycle/r2.env" <"$r2_env"
on_pod "cat > /root/r2_sync.sh" <"$here/r2_sync.sh"
on_pod "bash /root/r2_sync.sh pull /workspace/ue; rc=\$?; rm -f /root/.config/opencycle/r2.env; \
mkdir -p /workspace/ddc; chown -R ue4 /workspace/ue /workspace/ddc 2>/dev/null; exit \$rc"

if [ -n "$sha" ]; then
	echo "== $(date +%T) source at ${sha:0:8}"
	src=$(mktemp -d)
	trap 'rm -rf "$src"' EXIT
	git -C /workspace archive "$sha:veloverse" -- scripts README.md VeloverseUE/Source VeloverseUE/Config \
		VeloverseUE/VeloverseUE.uproject | tar -x -C "$src"
	rsync -a --no-owner --no-group -e "${ssh_cmd[*]}" "$src/" "root@$host:/workspace/ue/"
fi

echo "== $(date +%T) run $name on the pod"
grep -v 'git -C /workspace/veloverse merge' "$job" | on_pod "mkdir -p /workspace/ue/out/lane && cat > /workspace/ue/out/lane/$name.sh"
on_pod "cd /workspace/ue && rm -f out/lane/$name.rc && nohup bash -c 'bash out/lane/$name.sh > out/lane/$name.log 2>&1; echo \$? > out/lane/$name.rc' >/dev/null 2>&1 &"
until
	rc=$(on_pod "cat /workspace/ue/out/lane/$name.rc 2>/dev/null")
	[ -n "$rc" ]
do
	sleep 60
done
on_pod "tail -5 /workspace/ue/out/lane/$name.log" || true
mkdir -p /workspace/ue/out/lane
rsync -a -e "${ssh_cmd[*]}" "root@$host:/workspace/ue/out/lane/$name.log" /workspace/ue/out/lane/ || true
if [ -n "$stills" ]; then
	echo "== $(date +%T) bring back ${stills#/workspace/ue/}"
	bash "$here/pod_sync.sh" "$host" "$port" pull "${stills#/workspace/ue/}"
fi
echo "== $(date +%T) $name exited $rc"
exit "$rc"
