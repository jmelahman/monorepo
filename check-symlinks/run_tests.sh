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
  ./check-symlinks "${files[@]}"
  if [ $? != 1 ]; then
    >&2 echo "Test failed with case: ${files[*]}"
    exit 1
  fi
}

function expect_error() {
  local args=("$@")
  ./check-symlinks "${args[@]}"
  if [ $? != 2 ]; then
    >&2 echo "Test failed with case: ${args[*]}"
    exit 1
  fi
}

setUp
expect_success ""
expect_success testdata/doesnt_exist
expect_success testdata/root_owned_file
expect_success testdata/some_file
expect_success testdata/valid_directory_link
expect_success testdata/valid_link
expect_success testdata/.hidden_dir
expect_success testdata/.hidden_dir/hidden_file
expect_success --hidden testdata/.hidden_dir/hidden_file

expect_failure testdata/broken_link
expect_failure testdata/recursive_broken_link
expect_failure --hidden testdata/.hidden_broken_link
expect_failure --hidden testdata/.hidden_dir/hidden_broken_link
expect_failure testdata/broken_link testdata/some_file testdata/valid_link "" doesnt_exist

expect_error --foo
exit 0
