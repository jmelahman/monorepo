#!/bin/zsh

function array_contains() {
  needle=$1
  haystack=$2

  if (( ${haystack[(I)$needle]} )); then
     return 0
  fi
  return 1
}

function run_tests() {
  failures=0

  expected="foo"
  unexpected="bar"
  my_array=(
    $expected
  )

  echo "### Testing array_contains ###"
  if array_contains $expected $my_array; then
    echo "Passed"
  else
    echo "Failed"
    failures=$((failures + 1))
  fi

  if ! array_contains $unexpected $my_array; then
    echo "Passed"
  else
    echo "Failed"
    failures=$((failures + 1))
  fi

  echo "There were $failures failures"
  exit $failures
}

run_tests
