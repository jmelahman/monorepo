#!/usr/bin/env bash
# Runs the tests with the race detector and enforces the coverage floor.
set -euo pipefail

# A ratchet, not a target: raise it as gaps close, never lower it.
floor=91.0

out="$(mktemp)"
trap 'rm -f "$out"' EXIT
# -coverpkg=./... credits cross-package hits (e.g. the golden tests
# exercising the internal parsers), which is the honest total.
go test -race -coverpkg=./... -coverprofile="$out" ./...
total=$(go tool cover -func="$out" | awk '/^total:/ {gsub(/%/,""); print $3}')
echo "total statement coverage: ${total}% (floor: ${floor}%)"
if ! awk -v t="$total" -v f="$floor" 'BEGIN { exit (t+0 >= f+0) ? 0 : 1 }'; then
	echo "coverage ${total}% fell below the ${floor}% floor" >&2
	exit 1
fi
