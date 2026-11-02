#!/bin/sh
# Writes $VERSION (e.g. v1.2.3) everywhere the version is written down. Run by
# `git orchard release`, as this subtree's pre-release hook in the monorepo.
set -eu
cd "$(dirname "$0")/.."

ver=${VERSION#v}
sed -i.bak "s/^version = .*/version = \"$ver\"/" Cargo.toml
rm Cargo.toml.bak
cargo update --workspace --offline --quiet
for f in README.md docs/index.html; do
	sed -i.bak "s/^\( *\)rev: v.*/\1rev: v$ver/" "$f"
	rm "$f.bak"
done
