#!/bin/sh
# Regenerates .config/git-orchard/shared/pybuild/build-constraints.txt: the
# locked, hash-pinned transitive closure of every build toolchain pinned by
# the subtrees sharing the pybuild profile. `git orchard sync` copies it into
# each of them, and their CI passes it to `uv build --build-constraints`.
#
# The direct pins stay single-sourced: [build-system].requires in each
# subtree's pyproject.toml (or the shared manygo block, for subtrees that
# carry it) and any `*_PIN` constants in hatch_build.py. A constraints file
# never installs anything itself, so one superset lock serves every subtree,
# as long as no two of them pin a package differently; uv fails to resolve
# if they do. Existing transitive pins are kept; pass --upgrade to move them.
#
# Usage: tools/scripts/gen-build-constraints.sh [--upgrade]
set -eu
cd "$(dirname "$0")/../.."

shared=.config/git-orchard/shared

requires() {
	sed -n 's/^requires = \[\(.*\)\]$/\1/p' "$1" | tr ',' '\n' | tr -d ' "'
}

pins() {
	requires "$shared/manygo/pyproject.toml"
	git config -f .config/git-orchard/subtrees --get-regexp '^subtree\..*\.shared$' '^pybuild$' |
		sed 's/^subtree\.\(.*\)\.shared pybuild$/\1/' |
		while read -r dir; do
			# The shared block is read from its source above, which is
			# newer than the subtree's copy until the next sync.
			if ! grep -q "BEGIN orchard:manygo" "$dir/pyproject.toml"; then
				requires "$dir/pyproject.toml"
			fi
			if [ -f "$dir/hatch_build.py" ]; then
				sed -n 's/^[A-Z_]*_PIN = "\(.*\)"$/\1/p' "$dir/hatch_build.py"
			fi
		done
}

pins | sort -u | uv pip compile - -o "$shared/pybuild/build-constraints.txt" \
	--universal --generate-hashes --quiet "$@" \
	--custom-compile-command "tools/scripts/gen-build-constraints.sh (in https://github.com/jmelahman/monorepo)"
