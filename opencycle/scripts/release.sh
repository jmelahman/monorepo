#!/bin/bash
# Packs the build that package.sh staged in out/package and publishes it as a release on
# github.com/jmelahman/FreeSwift. Usage: scripts/release.sh TAG [--upload]
# Without --upload it only writes out/release/TAG: the tarball, split into parts under GitHub's
# 2 GiB asset limit when it is bigger, a SHA256SUMS and the release notes. --upload needs the
# GitHub CLI logged in (gh auth login); its token stays in gh's own config.
# Mirror the source first (scripts/freeswift_sync.sh --push) so the tag lands on current code.
set -euo pipefail
REPO=jmelahman/FreeSwift
tag=${1:?usage: release.sh TAG [--upload]}
build=/workspace/ue/out/package/Linux
out=/workspace/ue/out/release/$tag
here=$(cd "$(dirname "$0")/.." && pwd)
[ -x "$build/OpenCycleUE.sh" ] || {
	echo "no build in $build; run scripts/package.sh first" >&2
	exit 1
}
rm -rf "$out" && mkdir -p "$out"
name=FreeSwift-$tag-linux-x86_64
cp "$here/docs/release-notes.md" "$out/NOTES.md"
# The tarball unpacks to one folder holding the game and the notes.
tar -C "$build/.." --transform "s#^Linux#$name#" -cf - Linux \
	--transform "s#^.*NOTES.md#$name/README.md#" -C "$out" NOTES.md |
	xz -T0 -6 >"$out/$name.tar.xz"
size=$(stat -c %s "$out/$name.tar.xz")
if [ "$size" -gt $((1900 * 1024 * 1024)) ]; then
	split -b 1900M -d -a 2 "$out/$name.tar.xz" "$out/$name.tar.xz.part"
	rm "$out/$name.tar.xz"
	printf 'The build is split in parts: cat %s.tar.xz.part* | tar -xJ\n' "$name" >>"$out/NOTES.md"
fi
(cd "$out" && sha256sum "$name".tar.xz* >SHA256SUMS)
ls -l "$out"
if [ "${2:-}" = --upload ]; then
	command -v gh >/dev/null || {
		echo "install the GitHub CLI and run gh auth login" >&2
		exit 1
	}
	gh release create "$tag" --repo "$REPO" --title "FreeSwift $tag" --prerelease \
		--notes-file "$out/NOTES.md" "$out/$name".tar.xz* "$out/SHA256SUMS"
	gh release view "$tag" --repo "$REPO" --json url,assets --jq '.url, (.assets[] | .name)'
fi
