#!/usr/bin/env bash
# A second opinion on stills from Gemini (the `agy` CLI), for the look review: it reads only the
# images given, in plan mode (no edits, no shell), and writes its answer to OUT.
#   gemini_review.sh OUT "WHAT THE IMAGES ARE" IMAGE...
# The question list is AGENTS.md's "Judging the look". Its verdict is a second opinion beside our
# own crop review, never the decision. GEMINI_MODEL picks a model (`agy models`), GEMINI_TIMEOUT
# the limit in seconds (default 300). About 40 s for one stack (23 Nov).
set -euo pipefail
[ $# -ge 3 ] || {
	sed -n 2,7p "$0"
	exit 2
}
out=$1 what=$2
shift 2
AGY=${AGY:-/root/.local/bin/agy}
limit=${GEMINI_TIMEOUT:-300}
# Only copies of the images are in the agent's workspace.
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
names=()
for img in "$@"; do
	name=$(basename "$img" | tr ' ' '_')
	cp "$img" "$dir/$name"
	names+=("$dir/$name")
done
prompt="You are reviewing renders from a lifelike open-world cycling game in Unreal Engine 5.
The bar is a AAA environment: dense, lived-in, lit like the best open-world games. The target
look is vibrant yet realistic: rich green grass with small flowers, warm brown dirt, deep shadows,
knee-high dense grass that shows no ground.

The images: ${names[*]}
What they are: $what

For each image or labelled crop, answer briefly:
- Does ground show through the grass, and is it a different colour?
- Do blades sparkle or go black?
- Is the cover patchy?
- Is anything neon or lime?
- Is anything growing on the road or path?
- Anything else that breaks the illusion of a real place.
Then: which is best, and the single biggest remaining fault, located precisely in the frame.
Be concrete and terse. Do not edit or create any files."
model=()
[ -n "${GEMINI_MODEL:-}" ] && model=(--model "$GEMINI_MODEL")
mkdir -p "$(dirname "$out")"
timeout $((limit + 30)) "$AGY" --mode plan --print-timeout "${limit}s" --add-dir "$dir" \
	"${model[@]}" --output-format json --print="$prompt" >"$dir/reply.json"
cp "$dir/reply.json" "$out.json"
python3 -I - "$out.json" "$out" <<'PY'
import json
import sys

r = json.load(open(sys.argv[1]))
status = r.get("status")
if status != "SUCCESS":
    sys.exit("gemini: " + str(status))
open(sys.argv[2], "w").write(r["response"])
print("gemini review in %s (%.0f s)" % (sys.argv[2], r.get("duration_seconds", 0)))
PY
