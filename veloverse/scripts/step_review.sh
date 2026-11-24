#!/usr/bin/env bash
# The end of a queue step: A/B crop stacks with their numbers (step_stack.py), then Gemini's
# review of them (gemini_review.sh), so both are on disk when `oc_queue.sh wait` reports the step.
#   step_review.sh [--bg] OUT "WHAT CHANGED" "shot,shot" LABEL=DIR [LABEL=DIR ...]
# Writes OUT/<shot>.jpg, OUT/stats.json and OUT/gemini.md. A failed review doesn't fail the step:
# the stills are the step's work, the review a convenience (Gemini's advice, 23 Nov).
# --bg detaches it (log in OUT/review.log, OUT/review.done when it ends) so the step ends with its
# stills and the next one starts building: three reviews held the editor idle ~4.5 min (v99).
set -uo pipefail
if [ "${1:-}" = --bg ]; then
	shift
	mkdir -p "$1"
	rm -f "$1/review.done"
	# shellcheck disable=SC2016  # expanded by the inner bash, on purpose
	setsid nohup bash -c '"$0" "$@"; touch "$1/review.done"' "$0" "$@" >"$1/review.log" 2>&1 </dev/null &
	echo "step_review: in the background, log $1/review.log"
	exit 0
fi
[ $# -ge 4 ] || {
	sed -n 2,6p "$0"
	exit 2
}
out=$1 what=$2 shots=$3
shift 3
here=$(cd "$(dirname "$0")" && pwd)
numbers=$(timeout 600 uv run -q --no-project --with numpy --with pillow python -I \
	"$here/step_stack.py" "$out" "$@" --shots "$shots") || {
	echo "step_review: the stacks failed"
	exit 0
}
echo "$numbers"
mapfile -t imgs < <(ls "$out"/*.jpg 2>/dev/null)
[ ${#imgs[@]} -gt 0 ] || {
	echo "step_review: no stills to stack"
	exit 0
}
labels=$(printf '%s ' "${@%%=*}")
"$here/gemini_review.sh" "$out/gemini.md" \
	"Foreground crops at full resolution, stacked top to bottom: ${labels}(labelled in yellow, \
whole-frame numbers beside). What changed: $what. Numbers per crop:
$numbers" "${imgs[@]}" || echo "step_review: gemini failed"
exit 0
