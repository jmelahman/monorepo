#!/bin/bash
# After the loop clips: re-render the loop's stills with settled exposure (loop_v2b), then film the loop.
set -u
cd /workspace/ue || exit 1
until grep -q "== .* done" /workspace/ue/pod_loop_clips.log 2>/dev/null; do sleep 20; done
echo "== $(date +%H:%M:%S) stills"
OC_PLAN=long OC_KM=10 OC_MODE=stills OC_OUT=/workspace/ue/out/loop_v2b \
	OC_STILLS=50,550,930,1700,2600,3200,3800,4400,5000,5450,6200,6900,7500,8100,8700,9400,9800 \
	bash scripts/coast_ride.sh | tail -3
echo "== $(date +%H:%M:%S) film"
bash scripts/pod_film.sh
