#!/usr/bin/env bats

load "external/bats_shellmock/load.bash"

setup() {
  skipIfNot "${BATS_TEST_DESCRIPTION:-}"
  shellmock_clean
}

teardown() {
  if [ -z "${TEST_FUNCTION:-}" ]; then
    shellmock_clean
  else
    echo Single Test Keeping stubs: "${BATS_TEST_DESCRIPTION}/${TEMP_STUBS}"
  fi
}

@test "pre-push no-op" {
    set -eo pipefail

    local fixtures=('' 'foo')
    local merge_base="b501e53b27bc4649d15dc0580766599a39554951"

    for fixture in "${fixtures[@]}"; do
      shellmock_clean
      # See https://github.com/jmelahman/bats-shellmock/issues/1
      unset capture

      shellmock_expect git --match "fetch origin master"
      shellmock_expect git --match "merge-base origin/master HEAD" --output "${merge_base}"
      shellmock_expect git --match "diff --name-only ${merge_base}" --output "${fixture}"

      run "${BATS_TEST_DIRNAME}/pre-push.bash"

      [ "$status" = "0" ]
      shellmock_verify
      shellmock_verify_times 3
    done
}

@test "pre-push changed build" {
    set -eo pipefail

    local fixtures=('BUILD' 'foo/BUILD' 'build.bzl' 'BUILD.bazel' 'bar/BUILD.bzl' 'foo BUILD bar')
    local merge_base="b501e53b27bc4649d15dc0580766599a39554951"

    for fixture in "${fixtures[@]}"; do
      shellmock_clean
      # See https://github.com/jmelahman/bats-shellmock/issues/1
      unset capture

      shellmock_expect git --match "fetch origin master"
      shellmock_expect git --match "merge-base origin/master HEAD" --output "${merge_base}"
      shellmock_expect git --match "diff --name-only ${merge_base}" --output "${fixture}"
      shellmock_expect bazel --status 0 --match "run //:buildifier"

      run "${BATS_TEST_DIRNAME}/pre-push.bash"

      #TEST_FUNCTION="true"
      #shellmock_debug "Testing fixture: $fixture"
      #shellmock_dump
      #cat "${SHELLMOCK_CAPTURE_DEBUG}" "${CAPTURE_FILE}"
      #unset TEST_FUNCTION

      [ "$status" = "0" ]
      shellmock_verify
      shellmock_verify_times 4
    done
}

@test "pre-push changed py" {
    set -eo pipefail

    local fixtures=('snake.py' 'foo/snake.py')
    local merge_base="b501e53b27bc4649d15dc0580766599a39554951"

    for fixture in "${fixtures[@]}"; do
      shellmock_clean
      # See https://github.com/jmelahman/bats-shellmock/issues/1
      unset capture

      shellmock_expect git --match "fetch origin master"
      shellmock_expect git --match "merge-base origin/master HEAD" --output "${merge_base}"
      shellmock_expect git --match "diff --name-only ${merge_base}" --output "${fixture}"
      shellmock_expect bazel --status 0 --match "run //tools/format"

      run "${BATS_TEST_DIRNAME}/pre-push.bash"

      #TEST_FUNCTION="true"
      #shellmock_debug "Testing fixture: $fixture"
      #shellmock_dump
      #cat "${SHELLMOCK_CAPTURE_DEBUG}" "${CAPTURE_FILE}"
      #unset TEST_FUNCTION

      [ "$status" = "0" ]
      shellmock_verify
      shellmock_verify_times 4
    done
}
