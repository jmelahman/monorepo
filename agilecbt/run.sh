#!/usr/bin/env bash
# Run the backend (:8080) and frontend (:5173) dev servers together inside
# the ghcr.io/jmelahman/devcontainer image, so the host only needs Docker.
# Extra args are passed to `serve`, e.g. `./run.sh --in-memory`.
# Ctrl-C stops both.
set -euo pipefail

cd "$(dirname "$0")"

IMAGE="${AGILECBT_DEV_IMAGE:-ghcr.io/jmelahman/devcontainer:latest}"

if [[ -z ${AGILECBT_IN_DEVCONTAINER:-} ]]; then
	# Host network keeps the ports on the host's localhost and lets the repo
	# config reach the host's Ollama at localhost:11434 unchanged.
	data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/agilecbt"
	gomod_dir="${GOMODCACHE:-$HOME/go/pkg/mod}"
	cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}"
	mkdir -p "$data_dir" "$gomod_dir" "$cache_dir"
	# Root, not the image's dev user: rootless Docker maps container root to
	# the host user, so files written to the checkout (web/node_modules) stay
	# theirs.
	args=(
		--rm --init
		--name agilecbt-dev
		--user root
		--network host
		-e GOPATH=/root/go
		-e BUN_INSTALL_CACHE_DIR=/root/.cache/bun/install
		-e AGILECBT_IN_DEVCONTAINER=1
		-v "$PWD:/workspace"
		-w /workspace
		-v "$gomod_dir:/root/go/pkg/mod"
		-v "$cache_dir:/root/.cache"
		# Keep the on-disk DB where a host-run server would put it.
		-v "$data_dir:/root/.local/share/agilecbt"
		# Claude Code's login and state, for `docker exec -it agilecbt-dev claude`.
		-v "$HOME/.claude:/root/.claude"
		-v "$HOME/.claude.json:/root/.claude.json"
	)
	if [[ -d "$HOME/.config/agilecbt" ]]; then
		args+=(-v "$HOME/.config/agilecbt:/root/.config/agilecbt:ro")
	fi
	for var in AGILECBT_LLM_BASE_URL AGILECBT_MODEL AGILECBT_LLM_API_KEY; do
		if [[ -n ${!var:-} ]]; then
			args+=(-e "$var")
		fi
	done
	if [[ -t 0 && -t 1 ]]; then
		args+=(-it)
	fi
	exec docker run "${args[@]}" "$IMAGE" /workspace/run.sh "$@"
fi

pids=()
cleanup() {
	trap - EXIT INT TERM
	for pid in "${pids[@]}"; do
		kill -- "-$pid" 2>/dev/null || kill "$pid" 2>/dev/null || true
	done
	wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Build web/dist so the backend can embed it and serve the UI on :8080 too.
# Plain `vite build`, not `bun run build`, so type errors don't block startup.
(cd web && bun install && bun x vite build)

# setsid gives each server its own process group so cleanup reaches
# children (wgo's rebuilt binary, vite under bun).
setsid wgo run -tags embed . serve "$@" &
pids+=($!)

setsid bash -c 'cd web && exec bun run dev --host 0.0.0.0' &
pids+=($!)

# Exit (and tear down the other) as soon as either server dies.
wait -n
