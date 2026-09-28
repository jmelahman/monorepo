#!/usr/bin/env bash
# Every static check, in the order that fails fastest: style, then whether
# the scripts compile with their autoloads and without a single warning, then
# whether the scenes are what the editor would write. pre-commit runs the same
# steps as separate hooks; this is for running them all at once.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
cd "$ROOT"
dirs=(addons/agent_bridge core game sim tests tools)
status=0
echo "== gdlint"
gdtool gdlint "${dirs[@]}" || status=1
echo "== gdformat --check"
gdtool gdformat --check "${dirs[@]}" || status=1
echo "== check (compile, warnings as errors)"
"$ROOT/scripts/check.sh" || status=1
echo "== resave --check"
"$ROOT/scripts/resave.sh" --check || status=1
exit $status
