#!/bin/bash
# Packs the build that package.sh staged in out/package as a release tarball to copy by hand:
# out/release/TAG holds Veloverse-TAG-linux-x86_64.tar.xz (the game and the notes as README.md)
# and its SHA256SUMS. Nothing is uploaded; there are no GitHub releases.
# Usage: scripts/release.sh TAG. For a QA build, scripts/package_snapshot.sh is quicker.
set -euo pipefail
tag=${1:?usage: release.sh TAG}
build=/workspace/ue/out/package/Linux
out=/workspace/ue/out/release/$tag
here=$(cd "$(dirname "$0")/.." && pwd)
[ -x "$build/VeloverseUE.sh" ] || {
	echo "no build in $build; run scripts/package.sh first" >&2
	exit 1
}
rm -rf "$out" && mkdir -p "$out"
name=Veloverse-$tag-linux-x86_64
cp "$here/docs/release-notes.md" "$out/NOTES.md"
# The tarball unpacks to one folder holding the game and the notes.
tar -C "$build/.." --transform "s#^Linux#$name#" -cf - Linux \
	--transform "s#^.*NOTES.md#$name/README.md#" -C "$out" NOTES.md |
	xz -T0 -6 >"$out/$name.tar.xz"
(cd "$out" && sha256sum "$name".tar.xz >SHA256SUMS)
ls -l "$out"
