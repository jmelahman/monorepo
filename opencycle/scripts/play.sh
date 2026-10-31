#!/usr/bin/env bash
# Starts the game in the background with the agent bridge listening, and
# returns once it is ready for scripts/bridge.py. Extra arguments go to Godot
# (a scene path, --resolution 1280x720, ...). XVFB=1 keeps it off your screen.
#
#   scripts/play.sh && scripts/bridge.py tree && scripts/bridge.py quit
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"
port=${AGENT_BRIDGE_PORT:-9877}
log="$ROOT/.godot-home/play.log"

if "$ROOT/scripts/bridge.py" ping >/dev/null 2>&1; then
	echo "play: a game is already listening on 127.0.0.1:$port; quit it or set AGENT_BRIDGE_PORT" >&2
	exit 1
fi

# setsid so the game outlives this script and the shell that ran it.
with_display setsid "$GODOT" --path "$ROOT" "$@" -- --agent-bridge >"$log" 2>&1 </dev/null &
pid=$!

# Import on a fresh checkout can take a while; the bridge line is the signal.
for _ in $(seq 240); do
	if grep -q "agent_bridge: listening" "$log"; then
		echo "play: pid $pid, bridge on 127.0.0.1:$port, log at $log"
		exit 0
	fi
	if ! kill -0 "$pid" 2>/dev/null; then
		cat "$log" >&2
		echo "play: the game exited before the bridge came up" >&2
		exit 1
	fi
	sleep 0.25
done
kill "$pid" 2>/dev/null || true
cat "$log" >&2
echo "play: no bridge after 60s" >&2
exit 1
