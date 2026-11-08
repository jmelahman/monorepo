#!/bin/bash
# Builds and runs the engine-free tests of the ride's C++ (physics) with the system compiler.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
for test in "$here"/*_test.cpp; do
	echo "== $(basename "$test")"
	${CXX:-g++} -std=c++20 -Wall -Wextra -Werror -I"$here/../OpenCycleUE/Source/OpenCycleUE/Public" \
		"$test" -o "$out/test"
	"$out/test"
done
