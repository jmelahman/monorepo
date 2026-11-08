#!/bin/bash
# Renders 8 s ride clips round the island loop into out/loop_clip_NAME, from the level pod_long.sh built.
#   bash scripts/pod_loop_clips.sh            (all)   or   bash scripts/pod_loop_clips.sh seam jungle
set -u
cd /workspace/ue || exit 1
export OC_PLAN=long OC_KM=${OC_KM:-10} OC_MODE=ride OC_SECONDS=${OC_SECONDS:-8}
declare -A FROM=([seam]=9790 [paddies]=4300 [village]=5380 [north]=6150 [bamboo]=8650 [jungle]=3050)
names=("$@")
[ ${#names[@]} -eq 0 ] && names=(seam paddies village north bamboo jungle)
for name in "${names[@]}"; do
	echo "== $(date +%H:%M:%S) $name from ${FROM[$name]} m"
	OC_FROM=${FROM[$name]} OC_OUT=/workspace/ue/out/loop_clip_$name bash scripts/coast_ride.sh | tail -2
done
echo "== $(date +%H:%M:%S) done"
