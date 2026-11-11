#!/bin/bash
# Runs the packaged game that package.sh built, on the host or in the devcontainer.
# The Unreal checkout is ~/.cache/opencycle-neural/ue on the host and /workspace/ue in the container;
# OC_UE points somewhere else. Extra arguments pass through, e.g. -RideTrainer=sim -RideFtp=230.
set -u
for ue in "${OC_UE:-}" "$HOME/.cache/opencycle-neural/ue" /workspace/ue; do
	game="$ue/out/package/Linux/VeloverseUE.sh"
	if [ -n "$ue" ] && [ -x "$game" ]; then
		echo "playing the build of $(date -r "$ue/out/package/Linux/VeloverseUE/Binaries/Linux/VeloverseUE" '+%a %d %b %H:%M')"
		exec "$game" "$@"
	fi
done
echo "no packaged game found; build one with scripts/package.sh" >&2
exit 1
