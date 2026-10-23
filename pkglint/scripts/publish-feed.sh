#!/usr/bin/env bash
# publish-feed.sh REMOTE EXPECTED_SHA DIR
#
# Replace the three files the half-hourly AUR poll writes — data/state.jsonl,
# data/feed.jsonl and docs/feed.xml, read from DIR — in the site branch at
# REMOTE, and force-push the result as a new single root commit, the shape
# site.yml keeps that branch in.
#
# The rest of the branch is ~350MB of docs/ that the poll did not touch, so
# this never checks it out: a blobless, depth-one fetch brings the commit and
# its trees, and the new tree is assembled in the index from those plus the
# three new blobs. Unchanged blobs are referenced by ID and never downloaded.
#
# The three files' old blobs are the exception, fetched on purpose. push
# --thin looks up the previous blob at each path it sends as a delta base, and
# fails outright when one is missing; and state.jsonl is ~40MB and changes by
# a few records a poll, so with its predecessor on hand it goes out as a delta
# of a few kilobytes. Without that each poll would send, and the server keep
# as an unreachable object until it is collected, another 40MB.
#
# EXPECTED_SHA is the site commit the poll read its state from. If the branch
# has moved since — a nightly run published, say — the poll's state is a fork
# of an older one, and pushing it would discard the newer run's work, so the
# script stops and the next poll starts over from the new commit. That is a
# race the workflow expects, so it exits 0, but it says so as a workflow
# warning: one that kept happening would leave feed.xml stale.
set -euo pipefail

if [ $# -ne 3 ]; then
	echo "usage: $0 REMOTE EXPECTED_SHA DIR" >&2
	exit 2
fi
remote=$1 expected=$2 dir=$(cd "$3" && pwd)
if ! [[ $expected =~ ^[0-9a-f]{40}$ ]]; then
	echo "publish-feed: $expected is not a commit ID" >&2
	exit 2
fi
files=(data/state.jsonl data/feed.jsonl docs/feed.xml)
for f in "${files[@]}"; do
	if [ ! -f "$dir/$f" ]; then
		echo "publish-feed: $dir/$f is missing" >&2
		exit 1
	fi
done

repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
git() { command git -C "$repo" "$@"; }

git init -q
git remote add origin "$remote"
git fetch -q --depth=1 --filter=blob:none origin "+refs/heads/site:refs/remotes/origin/site"
base=$(git rev-parse refs/remotes/origin/site)
if [ "$base" != "$expected" ]; then
	echo "::warning title=publish-feed::site moved from $expected to $base since the poll read it; not publishing"
	exit 0
fi

# The delta bases, fetched lazily from the promisor remote. A file the branch
# does not have yet simply has no base.
for f in "${files[@]}"; do
	git cat-file -e "$base:$f" 2>/dev/null || true
done

# Everything from here on must work from what is already local: a lazy fetch
# past this point would be a docs/ blob, and there are thousands of them.
export GIT_NO_LAZY_FETCH=1

git read-tree "$base"
for f in "${files[@]}"; do
	blob=$(git hash-object -w "$dir/$f")
	git update-index --add --cacheinfo "100644,$blob,$f"
done
tree=$(git write-tree --missing-ok)
if [ "$tree" = "$(git rev-parse "$base^{tree}")" ]; then
	echo "publish-feed: nothing changed"
	exit 0
fi

commit=$(
	GIT_AUTHOR_NAME="github-actions[bot]" \
		GIT_AUTHOR_EMAIL="41898282+github-actions[bot]@users.noreply.github.com" \
		GIT_COMMITTER_NAME="github-actions[bot]" \
		GIT_COMMITTER_EMAIL="41898282+github-actions[bot]@users.noreply.github.com" \
		git commit-tree "$tree" -m "Report card feed $(date -u +%Y-%m-%dT%H:%MZ)"
)
git push -q --thin --force-with-lease="site:$base" origin "$commit:refs/heads/site"
echo "publish-feed: site $base -> $commit"
