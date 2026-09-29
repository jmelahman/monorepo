#!/usr/bin/env bash
# Builds the QuickJS GDExtension (native/) into bin/, where
# fivewild_js.gdextension loads it. Incremental: ninja rebuilds only what
# changed, and a no-op run takes a second. The first build compiles godot-cpp,
# about a minute and a half on eight cores.
#
#   scripts/build-native.sh             bin/fivewild_js.linux.x86_64.so
#   scripts/build-native.sh windows     bin/fivewild_js.windows.x86_64.dll (mingw-w64)
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
target=${1:-linux}
args=(-S "$ROOT/native" -B "$ROOT/.native-build/$target" -G Ninja -DCMAKE_BUILD_TYPE=Release)
case $target in
linux) ;;
windows) args+=(-DCMAKE_TOOLCHAIN_FILE="$ROOT/native/mingw-w64.cmake") ;;
*)
	echo "unknown target: $target (linux or windows)" >&2
	exit 2
	;;
esac
cmake "${args[@]}" >/dev/null
cmake --build "$ROOT/.native-build/$target"
