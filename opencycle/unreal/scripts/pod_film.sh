#!/bin/bash
# Films the whole loop at riding speed once loop_v2 and its clips are done: twelve parts of 820 m,
# each started 3 s early so the exposure has settled, encoded to out/loop_film/part_NN.mp4, then
# joined into out/loop_film/loop.mp4. Frames go to local disk and are deleted after each part.
set -u
cd /workspace/ue || exit 1
FF=$(python3 -c "import imageio_ffmpeg;print(imageio_ffmpeg.get_ffmpeg_exe())")
until grep -q "== .* done" /workspace/ue/pod_loop_clips.log 2>/dev/null; do sleep 30; done
mkdir -p /film out/loop_film && chmod a+rwx /film
export OC_PLAN=long OC_KM=10 OC_MODE=ride
: >out/loop_film/list.txt
for k in $(seq 0 11); do
	part=$(printf "%02d" "$k")
	echo "== $(date +%H:%M:%S) part $part from $((k * 820)) m"
	OC_FROM=$(((k * 820 + 9845 - 21) % 9845)) OC_SECONDS=120.2 OC_OUT=/film/p$part bash scripts/coast_ride.sh | tail -1
	"$FF" -loglevel error -y -framerate 30 -start_number 90 -i "/film/p${part}/frame.%04d.png" \
		-c:v libx264 -crf 18 -preset slow -pix_fmt yuv420p "out/loop_film/part_${part}.mp4"
	rm -rf "/film/p${part}"
	echo "file 'part_$part.mp4'" >>out/loop_film/list.txt
done
"$FF" -loglevel error -y -f concat -safe 0 -i out/loop_film/list.txt -c copy out/loop_film/loop.mp4
echo "== $(date +%H:%M:%S) done"
