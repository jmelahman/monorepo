#!/bin/sh
# Regenerates build-constraints.txt: the locked, hash-pinned transitive
# closure of the build toolchain. The direct pins stay single-sourced in
# pyproject.toml ([build-system].requires) and hatch_build.py (GO_BIN_PIN,
# ZIGLANG_PIN); this script resolves them with uv into an exact lock that
# CI passes to `uv build --build-constraints`.
set -eu
cd "$(dirname "$0")/.."

pins() {
    sed -n 's/^requires = \[\(.*\)\]$/\1/p' pyproject.toml | tr ',' '\n' | tr -d ' "'
    sed -n 's/^GO_BIN_PIN = "\(.*\)"$/\1/p' hatch_build.py
    sed -n 's/^ZIGLANG_PIN = "\(.*\)"$/\1/p' hatch_build.py
}

pins | uv pip compile - -o build-constraints.txt --universal --generate-hashes
