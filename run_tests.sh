#!/usr/bin/env bash

set -u

function setUp() {
  trap 'rm testdata/valid_absolute_link' EXIT
  ln -s /etc/hosts testdata/valid_absolute_link
  go build .
}

function expect_success() {
  local files=("$@")
  ./check-symlinks "${files[@]}" || {
    >&2 echo "Test failed with case: ${files[*]}"
    exit 1
  }
}

function expect_failure() {
  local files=("$@")
  ./check-symlinks "${files[@]}" && {
    >&2 echo "Test failed with case: ${files[*]}"
    exit 1
  }
}

setUp
expect_success ""
expect_success testdata/doesnt_exist
expect_success testdata/root_owned_file
expect_success testdata/some_file
expect_success testdata/valid_directory_link
expect_success testdata/valid_link

expect_failure testdata/broken_link
expect_failure testdata/recursive_broken_link
expect_failure testdata/broken_link testdata/some_file testdata/valid_link "" doesnt_exist
exit 0
