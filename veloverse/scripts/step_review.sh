#!/usr/bin/env bash
# The end of a queue step: A/B crop stacks with their numbers (step_stack.py), then Gemini's
# review of them (gemini_review.sh), so both are on disk when `oc_queue.sh wait` reports the step.
#   step_review.sh OUT "WHAT CHANGED" "shot,shot" LABEL=DIR [LABEL=DIR ...]
# Writes OUT/<shot>.jpg, OUT/stats.json and OUT/gemini.md. A failed review doesn't fail the step:
# the stills are the step's work, the review a convenience (Gemini's advice, 23 Nov).
set -uo pipefail
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
