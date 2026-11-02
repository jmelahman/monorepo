#!/bin/sh
# Guards that the version agrees everywhere it is written down and, on tag
# pushes in CI, with the pushed tag. Cargo.toml is the source of truth:
# the binary, the wheels, and the rev `undot migrate` writes all come from it.
set -eu
cd "$(dirname "$0")/.."

ver=$(sed -n 's/^version = "\(.*\)"$/\1/p' Cargo.toml | head -n 1)

if [ -z "$ver" ]; then
	echo "check-version: cannot find version in Cargo.toml" >&2
	exit 1
fi
if ! grep -A 1 '^name = "undot"$' Cargo.lock | grep -q "^version = \"$ver\"$"; then
	echo "check-version: Cargo.lock does not pin undot $ver; run cargo check" >&2
	exit 1
fi
for f in README.md docs/index.html; do
	if ! grep -q "^ *rev: v$ver$" "$f"; then
		echo "check-version: $f does not show rev: v$ver" >&2
		exit 1
	fi
done
if [ "${GITHUB_REF_TYPE:-}" = "tag" ] && [ "${GITHUB_REF_NAME:-}" != "v$ver" ]; then
	echo "check-version: tag ${GITHUB_REF_NAME:-?} != v$ver" >&2
	exit 1
fi

# The release workflow runs maturin itself; it must build with the same one.
maturin=$(sed -n 's/^requires = \["maturin==\([^"]*\)"\].*$/\1/p' pyproject.toml)
if [ -z "$maturin" ]; then
	echo "check-version: cannot find the maturin pin in pyproject.toml" >&2
	exit 1
fi
if ! grep -q "^  MATURIN_VERSION: \"$maturin\"" .github/workflows/release.yml; then
	echo "check-version: release.yml does not pin maturin $maturin" >&2
	exit 1
fi

echo "check-version: OK ($ver, maturin $maturin)"
