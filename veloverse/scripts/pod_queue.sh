#!/bin/bash
# Keeps the GPU busy: worn short-ride clips on /uew now, and loop clips on /workspace/ue once loop_v2 is done.
set -u
(
	cd /uew || exit 1
	declare -A FROM=([town]=20 [paddies]=400 [jungle]=750 [rain]=790 [village]=960 [hills]=1250 [coast]=1700)
	for name in town paddies jungle rain village hills coast; do
		echo "== $(date +%H:%M:%S) wear $name"
		OC_MODE=ride OC_SECONDS=8 OC_FROM=${FROM[$name]} OC_OUT=/uew/out/wear_clip_$name bash scripts/coast_ride.sh | tail -2
	done
	echo "== $(date +%H:%M:%S) wear clips done"
) >/uew/pod_wear_clips.log 2>&1 &
(
	until grep -q "== .* done" /workspace/ue/pod_loop2.log; do sleep 30; done
	cd /workspace/ue && bash scripts/pod_loop_clips.sh
) >/workspace/ue/pod_loop_clips.log 2>&1 &
wait
