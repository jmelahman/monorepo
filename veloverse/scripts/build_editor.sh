#!/bin/bash
# Compiles the editor's C++ (VeloverseUEEditor) as ue4, under a hard time limit, and prints the
# errors and the result. Usage: scripts/build_editor.sh [LIMIT_SECONDS] (default 900).
# The log is /workspace/ue/build_editor.log. Exit 124 means it hit the limit and was stopped.
#
# -NoUBA: the build accelerator's local executor crashed clang (SIGSEGV in opendir), then spent
# 46 minutes symbolizing the crashes into a 270 MB log without failing (23 Nov). Without it, an
# overlay-only change compiles in about 10 s.
set -u
limit=${1:-900}
log=/workspace/ue/build_editor.log
if pgrep -x UnrealEditor >/dev/null; then
	echo "An editor is running; let it finish before compiling." >&2
	exit 1
fi
start=$SECONDS
# timeout runs inside su so it signals the build's own process group (dotnet and the compilers).
su ue4 -c "cd /home/ue4/UnrealEngine && timeout -k 30 $limit Engine/Build/BatchFiles/Linux/Build.sh \
	VeloverseUEEditor Linux Development -project=/workspace/ue/VeloverseUE/VeloverseUE.uproject -NoUBA" \
	>"$log" 2>&1
code=$?
if [ "$code" = 124 ] || [ "$code" = 137 ]; then
	pkill -u ue4 -x clang++
	echo "TIMEOUT after ${limit}s; the log's tail:"
	tail -c 1500 "$log"
	exit 124
fi
grep -a -E ' error|Result:' "$log" | head -20
echo "exit $code after $((SECONDS - start))s"
exit "$code"
