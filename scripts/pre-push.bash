#!/usr/bin/env bash
#
# Copy this file to /path/to/repo/.git/hooks/pre-push

function main() {
  set -euo pipefail

  command -v git-lfs >/dev/null 2>&1 || { echo >&2 "\nThis repository is configured for Git LFS but 'git-lfs' was not found on your path. If you no longer wish to use Git LFS, remove this hook by deleting '.git/hooks/pre-push'.\n"; exit 2; }
  git lfs pre-push "$@"

  # Patterns from https://github.com/bazelbuild/buildtools/blob/master/buildifier/runner.bash.template#L20
  # Non-relevant, non-globs are omitted because otheriwse git-diff complains.
  local merge_base
  local buildifier_patterns=(
    '*.bzl'
    '*.sky'
    'BUILD'
    '*.BUILD'
    'BUILD.*.bazel'
    'BUILD.*.oss'
    'WORKSPACE'
    'WORKSPACE.*.bazel'
    'WORKSPACE.*.oss'
  )
  git fetch origin master
  merge_base="$(git merge-base origin/master HEAD)"

  if ! git diff --quiet "${merge_base}" "${buildifier_patterns[@]}"; then
    bazel run //:buildifier.check
  fi
}

main "$@"
