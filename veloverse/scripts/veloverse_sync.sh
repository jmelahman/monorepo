#!/bin/bash
# Mirrors veloverse/ to github.com/jmelahman/Veloverse main. It clones the monorepo's current
# branch (sharing its objects, no copy) and rewrites it with git filter-repo, keeping only
# veloverse/ and its history under its old name opencycle/ (renamed 11 Nov 2026), moved to the
# root, and dropping the old Godot files that were stored in Git LFS (their objects aren't kept
# locally).
# The rewrite is deterministic, so a sync normally fast-forwards; --force is there for when the
# history is rewritten (the user allows force pushes to Veloverse).
# Usage: scripts/veloverse_sync.sh [--push]. Without --push it only builds the history and
# prints its head.
set -euo pipefail
REMOTE=git@github.com:jmelahman/Veloverse.git
MONO=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
cd "$MONO"
[ -d veloverse ] || {
	echo "run from the monorepo checkout" >&2
	exit 1
}
branch=$(git rev-parse --abbrev-ref HEAD)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git lfs ls-files --all -n 2>/dev/null | sed -n 's#^\(opencycle\|veloverse\)/##p' | sort -u >"$work/lfs"
GIT_LFS_SKIP_SMUDGE=1 git clone -q --shared --bare --single-branch --branch "$branch" \
	"$(git rev-parse --git-common-dir)" "$work/repo"
cd "$work/repo"
uvx -q git-filter-repo --force --quiet --refs "$branch" \
	--path opencycle/ --path veloverse/ --path-rename opencycle/: --path-rename veloverse/:
uvx -q git-filter-repo --force --quiet --refs "$branch" --invert-paths --paths-from-file "$work/lfs"
head=$(git rev-parse "$branch")
left=$(git rev-list --objects "$branch" | git cat-file --batch | grep -ac '^version https://git-lfs' || true)
echo "rewritten: $head ($(git rev-list --count "$branch") commits, $left LFS pointers)"
[ "$left" = 0 ] || {
	echo "LFS pointers remain; not pushing" >&2
	exit 1
}
if [ "${1:-}" = --push ]; then
	# The monorepo's pre-push hooks check its other projects, not this history.
	git -c core.hooksPath=/dev/null push --force "$REMOTE" "$branch:main"
	git ls-remote "$REMOTE" refs/heads/main
fi
