#!/usr/bin/env bash
#
# Copy this file to /path/to/repo/.git/hooks/pre-push

set -euo pipefail

function match() {
  local pattern="$1"
  local string="$2"
  if [[ "${string}" =~ $pattern ]]; then
    return 0
  fi
  return 1
}

function check_match() {
   local pattern="$1"
   local file="$2"
   local outfile="$3"
   if match "${pattern}" "${file}"; then
     echo "true" > "${outfile}"
   fi
 }

function main() {
  changed_build="$(mktemp)"
  local -r changed_py="$(mktemp)"
  trap "rm -rf $changed_build $changed_py" EXIT

  git fetch origin master
  while read file; do
    (
      check_match ".*(BUILD|BUILD\.bazel|\.bzl)" "${file}" "${changed_build}"
      check_match ".*\.py" "${file}" "${changed_py}"
    ) &
  done < <(git diff --name-only $(git merge-base origin/master HEAD))
  wait

  if [ -s "$changed_build" ]; then
    bazel run //:buildifier
  fi
  if [ -s "$changed_py" ]; then
    bazel run //tools/format
  fi
}

main "$@"
