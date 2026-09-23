#!/bin/sh
# Guards that the version constants agree with each other and, on tag
# pushes in CI, with the pushed tag. Solod has no ldflags-style version
# stamping, so the source constant is the single source of truth and
# releases must be tagged to match.
set -eu
cd "$(dirname "$0")/.."

ver=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' cli/cli.go)

if [ -z "$ver" ]; then
    echo "check-version: cannot find Version in cli/cli.go" >&2
    exit 1
fi
# The pre-commit snippet in the README pins a rev; users copy it verbatim.
if ! grep -q "^    rev: v$ver\$" README.md; then
    echo "check-version: README pre-commit rev is not v$ver" >&2
    exit 1
fi
if [ "${GITHUB_REF_TYPE:-}" = "tag" ] && [ "${GITHUB_REF_NAME:-}" != "v$ver" ]; then
    echo "check-version: tag ${GITHUB_REF_NAME:-?} != v$ver" >&2
    exit 1
fi

# The solod pin is centralized in hatch_build.py; the module files must
# agree with it.
solod=$(sed -n 's/^SOLOD_VERSION = "\(.*\)"$/\1/p' hatch_build.py)
for f in so/go.mod so/gotest.mod; do
    if ! grep -q "solod.dev $solod" "$f"; then
        echo "check-version: $f does not pin solod.dev $solod" >&2
        exit 1
    fi
done
# build-constraints.txt is generated from the direct pins
# (scripts/gen-build-constraints.sh); fail if it went stale.
for pin in $(sed -n 's/^requires = \[\(.*\)\]$/\1/p' pyproject.toml | tr ',' '\n' | tr -d ' "') \
    "$(sed -n 's/^GO_BIN_PIN = "\(.*\)"$/\1/p' hatch_build.py)" \
    "$(sed -n 's/^ZIGLANG_PIN = "\(.*\)"$/\1/p' hatch_build.py)"; do
    if ! grep -q "^$pin " build-constraints.txt && ! grep -q "^$pin\$" build-constraints.txt; then
        echo "check-version: build-constraints.txt is stale (missing $pin); run scripts/gen-build-constraints.sh" >&2
        exit 1
    fi
done

echo "check-version: OK ($ver, solod $solod)"
