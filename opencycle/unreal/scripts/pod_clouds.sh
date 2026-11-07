#!/bin/bash
# Tries cloud materials on the built level: two sky-facing stills per OC_CLOUD variant, into out/clouds_N.
cd /uew || exit 1
n=0
for v in "0.008,0.08,0.03,-0.2" "0.03,0.25,0.12,-0.2" "0.02,0.4,0.2,-0.1" "0.05,0.3,0.15,-0.3"; do
	echo "== $(date +%T) variant $n $v"
	OC_CLOUD=$v OC_TAIL=4 bash scripts/ue_run.sh cloud_swap.py cloud_swap.log | grep CLOUDS
	OC_MODE=stills OC_STILLS=50,1700 OC_OUT=/uew/out/clouds_$n bash scripts/coast_ride.sh | tail -2
	n=$((n + 1))
done
echo "== $(date +%T) done"
