#!/usr/bin/env bash
# Locate this repository's Go modules, and optionally run a command in each.
#
#   scripts/modules.sh                     # list them, one path per line
#   scripts/modules.sh go test ./...       # run a command in each, in turn
#   scripts/modules.sh golangci-lint run
#
# Paths are relative to the repo root ("." for a module rooted there). With a
# go.work, the `use` directives are authoritative, so a module deliberately
# left out of the workspace is left out here too; without one, every go.mod
# in the tree is a module (testdata/ and vendor/ hold fixtures and copies,
# not modules to build).
#
# This exists because nothing else gets a multi-module repo right: from a
# workspace root `go vet ./...` fails outright ("directory prefix . does not
# contain modules listed in go.work"), and from a module root `./...` skips
# any nested module. Both CI and the lint hooks go through here instead.
#
# Every module runs even if an earlier one fails; the exit status is the
# first failure.
set -uo pipefail

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

list_modules() {
	cd "$root" || exit 1

	if [ -f go.work ]; then
		go list -m -f '{{.Dir}}' | while IFS= read -r dir; do
			rel="${dir#"$root"}"
			rel="${rel#/}"
			printf '%s\n' "${rel:-.}"
		done
		return
	fi

	# POSIX primaries only: `-printf` and `-not` are GNU extensions, and this
	# has to work with the BSD find macOS ships.
	find . -name go.mod \
		! -path './.git/*' \
		! -path '*/testdata/*' \
		! -path '*/vendor/*' \
		-print |
		sed -e 's|/go\.mod$||' -e 's|^\./||' -e 's|^$|.|' |
		sort
}

# Not `mapfile`: it is bash 4, and macOS ships bash 3.2.
modules=()
while IFS= read -r module; do
	modules+=("$module")
done < <(list_modules)

if [ "${#modules[@]}" -eq 0 ]; then
	echo "$0: no Go modules found under $root" >&2
	exit 1
fi

if [ "$#" -eq 0 ]; then
	printf '%s\n' "${modules[@]}"
	exit 0
fi

status=0
for module in "${modules[@]}"; do
	# One module is the common case; a header there is just noise.
	if [ "${#modules[@]}" -gt 1 ]; then
		printf '==> %s: %s\n' "$module" "$*" >&2
	fi
	(cd "$root/$module" && "$@")
	rc=$?
	if [ "$rc" -ne 0 ] && [ "$status" -eq 0 ]; then
		status="$rc"
	fi
done

exit "$status"
