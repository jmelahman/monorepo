#!/bin/bash
# Mirrors opencycle/ to github.com/jmelahman/FreeSwift main: splits the subtree out of the
# monorepo, drops the old Godot files that were stored in Git LFS (their objects aren't kept
# locally), and pushes the result. The split and the rewrite are deterministic, so a sync
# normally fast-forwards; --force is there for when the history is rewritten (the user allows
# force pushes to FreeSwift). Usage: scripts/freeswift_sync.sh [--push]
# Without --push it only builds the rewritten history and prints its head.
set -euo pipefail
REMOTE=git@github.com:jmelahman/FreeSwift.git
MONO=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
cd "$MONO"
[ -d opencycle ] || {
	echo "run from the monorepo checkout" >&2
	exit 1
}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sha=$(git subtree split --prefix=opencycle 2>/dev/null)
echo "split: $sha"
git lfs ls-files --all -n 2>/dev/null | sed -n 's#^opencycle/##p' | sort -u >"$work/lfs"
git init -q --bare "$work/repo"
GIT_LFS_SKIP_PUSH=1 git push -q --no-verify "$work/repo" "$sha:refs/heads/main"
(cd "$work/repo" && uvx -q git-filter-repo --force --quiet --invert-paths \
	--paths-from-file "$work/lfs")
head=$(git -C "$work/repo" rev-parse main)
left=$(git -C "$work/repo" rev-list --objects main | git -C "$work/repo" cat-file --batch |
	grep -ac '^version https://git-lfs' || true)
echo "rewritten: $head ($(git -C "$work/repo" rev-list --count main) commits, $left LFS pointers)"
[ "$left" = 0 ] || {
	echo "LFS pointers remain; not pushing" >&2
	exit 1
}
if [ "${1:-}" = --push ]; then
	# The monorepo's pre-push hooks check its other projects, not this history.
	git -C "$work/repo" -c core.hooksPath=/dev/null push --force "$REMOTE" main:main
	git ls-remote "$REMOTE" refs/heads/main
fi
