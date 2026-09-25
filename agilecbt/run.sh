#!/usr/bin/env bash
# Run the backend (:8080) and frontend (:5173) dev servers together.
# Extra args are passed to `serve`, e.g. `./run.sh --in-memory`.
# Ctrl-C stops both.
set -euo pipefail

cd "$(dirname "$0")"

pids=()
cleanup() {
	trap - EXIT INT TERM
	for pid in "${pids[@]}"; do
		kill -- "-$pid" 2>/dev/null || kill "$pid" 2>/dev/null || true
	done
	wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# setsid gives each server its own process group so cleanup reaches
# children (wgo's rebuilt binary, vite under bun).
setsid wgo run . serve "$@" &
pids+=($!)

setsid bash -c 'cd web && bun install && exec bun run dev --host 0.0.0.0' &
pids+=($!)

# Exit (and tear down the other) as soon as either server dies.
wait -n
