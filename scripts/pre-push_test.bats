#!/usr/bin/env bats

load "external/bats_shellmock/load.bash"

__BUILDIFIER_PATTERN='*.bzl *.sky BUILD *.BUILD BUILD.*.bazel BUILD.*.oss WORKSPACE WORKSPACE.*.bazel WORKSPACE.*.oss'

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
    local fixture='foo'
    local merge_base='b501e53b27bc4649d15dc0580766599a39554951'

    shellmock_expect git --match 'lfs pre-push'
    shellmock_expect git --match 'fetch origin master'
    shellmock_expect git --match "merge-base origin/master HEAD" --output "${merge_base}"
    shellmock_expect git --match "diff --quiet ${merge_base} ${__BUILDIFIER_PATTERN}" --status 0
    shellmock_expect git --match "diff --quiet ${merge_base} *.py" --status 0

    run "${BATS_TEST_DIRNAME}/pre-push.bash"

    [ "$status" = "0" ]
    shellmock_verify
    shellmock_verify_times 5
}

@test "pre-push changed build" {
    local fixture='BUILD'
    local merge_base='b501e53b27bc4649d15dc0580766599a39554951'

    shellmock_expect git --match 'lfs pre-push'
    shellmock_expect git --match 'fetch origin master'
    shellmock_expect git --match 'merge-base origin/master HEAD' --output "${merge_base}"
    shellmock_expect git --match "diff --quiet ${merge_base} ${__BUILDIFIER_PATTERN}" --status 1
    shellmock_expect git --match "diff --quiet ${merge_base} *.py" --status 0
    shellmock_expect bazel --status 0 --match 'run //:buildifier.check'

    run "${BATS_TEST_DIRNAME}/pre-push.bash"

    [ "$status" = "0" ]
    shellmock_verify
    shellmock_verify_times 6
}

@test "pre-push changed py" {
    local fixture='snake.py'
    local merge_base="b501e53b27bc4649d15dc0580766599a39554951"

    shellmock_expect git --match 'lfs pre-push'
    shellmock_expect git --match 'fetch origin master'
    shellmock_expect git --match 'merge-base origin/master HEAD' --output "${merge_base}"
    shellmock_expect git --match "diff --quiet ${merge_base} ${__BUILDIFIER_PATTERN}" --status 0
    shellmock_expect git --match "diff --quiet ${merge_base} *.py" --status 1
    shellmock_expect bazel --status 0 --match 'run //tools/format -- --check'

    run "${BATS_TEST_DIRNAME}/pre-push.bash"

    shellmock_dump
    cat "${CAPTURE_FILE}"

    [ "$status" = "0" ]
    shellmock_verify
    shellmock_verify_times 6
}
