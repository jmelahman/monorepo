#!/bin/bash
# Sends the project to the Runpod pod (same paths there: /workspace/ue on its volume), or with
# `pull DIR` brings a render folder back. The pod's SSH address changes on every start:
#   scripts/pod_sync.sh HOST PORT            push sources, generated assets and Content
#   scripts/pod_sync.sh HOST PORT pull out/coast_v11
# Auth is the host's agent through $SSH_AUTH_SOCK; no key lives in the container.
set -euo pipefail
host=$1 port=$2
ssh_cmd="ssh -p $port -o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30"
cd /workspace/ue
if [ "${3:-}" = pull ]; then
	mkdir -p "$4"
	rsync -a --info=stats1 -e "$ssh_cmd" "root@$host:/workspace/ue/$4/" "$4/"
	exit 0
fi
$ssh_cmd "root@$host" "command -v rsync >/dev/null || (apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq rsync >/dev/null); mkdir -p /workspace/ue /workspace/ddc && chown -R ue4 /workspace/ue /workspace/ddc 2>/dev/null || true"
rsync -a --info=stats1 --delete -e "$ssh_cmd" \
	--exclude 'OpenCycleUE/Intermediate/' --exclude 'OpenCycleUE/Saved/' --exclude 'OpenCycleUE/DerivedDataCache/' \
	--exclude 'out/' --exclude '*.log' --exclude '__pycache__/' \
	scripts assets_gen textures assets_ph OpenCycleUE "root@$host:/workspace/ue/"
$ssh_cmd "root@$host" "chown -R ue4 /workspace/ue"
