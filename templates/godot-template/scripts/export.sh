#!/usr/bin/env bash
# Exports release builds and zips each one into build/, ready to upload:
#
#   scripts/export.sh              every preset in export_presets.cfg
#   scripts/export.sh Linux        just those named
#
# Needs the export templates: `scripts/setup.sh templates` (1.3 GB, once per
# Godot version). The presets exclude the agent bridge, gdUnit4, tests/ and
# tools/, so none of it reaches a player. VERSION names the zips; it defaults
# to `git describe`.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"

presets=("$@")
if [[ ${#presets[@]} == 0 ]]; then
	mapfile -t presets < <(sed -n 's/^name="\(.*\)"$/\1/p' export_presets.cfg)
fi
version=${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}
game=$(basename "$ROOT")

for preset in "${presets[@]}"; do
	path=$(awk -F'"' -v want="$preset" '/^name=/ { n = $2 } /^export_path=/ && n == want { print $2 }' export_presets.cfg)
	if [[ -z $path ]]; then
		echo "export: no preset named $preset in export_presets.cfg" >&2
		exit 2
	fi
	dir=$(dirname "$path")
	rm -rf "$dir"
	mkdir -p "$dir"
	echo "== $preset -> $path"
	log=$("$GODOT" --headless --path "$ROOT" --export-release "$preset" "$ROOT/$path" 2>&1) || {
		echo "$log" >&2
		exit 1
	}
	# A missing template or a failed resource still exits 0 on some paths.
	if grep -E "$GODOT_ERROR_PATTERN" <<<"$log" >&2 || [[ ! -s $path ]]; then
		echo "export: $preset failed (above)" >&2
		exit 1
	fi
	slug=$(tr '[:upper:] ' '[:lower:]-' <<<"$preset")
	zip=build/$game-$version-$slug.zip
	rm -f "$zip"
	# Python's zipfile rather than zip(1), which not every machine has; it
	# keeps the executable bit, which the Linux binary needs.
	(cd "$dir" && python3 -m zipfile -c "$ROOT/$zip" ./*)
	echo "   $zip"
done
