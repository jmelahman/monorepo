#!/usr/bin/env bash
# Sourced by every script that runs Godot. Sets $ROOT, $GODOT and the pins in
# versions.env, and points Godot's XDG dirs into .godot-home/ so a scripted run
# (tests, lint, the bridge) never reads or writes the player's real user://,
# editor settings or save files. The editor you launch yourself is unaffected.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=versions.env
source "$ROOT/scripts/versions.env"

# setup.sh installs to ~/.local/bin when no matching Godot is on PATH, so put
# it first; an explicit $GODOT still wins.
export PATH="$HOME/.local/bin:$PATH"
export GODOT="${GODOT:-godot}"

# The real data home, before the redirect: export templates are 1.3 GB and
# belong to the Godot version, not the project, so they stay shared there and
# are linked in (see setup.sh templates).
export REAL_DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
export XDG_DATA_HOME="$ROOT/.godot-home/data"
export XDG_CONFIG_HOME="$ROOT/.godot-home/config"
export XDG_CACHE_HOME="$ROOT/.godot-home/cache"
mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME"
# Inside the project, so keep Godot from importing it as assets.
touch "$ROOT/.godot-home/.gdignore"

# Runs its arguments under a virtual X server when there is no display. Godot's
# --headless drops the renderer, so screenshots come back empty and input
# events never reach the viewport; Xvfb with Mesa's llvmpipe is a real (slow)
# GPU, and CI and the devcontainer both have one. XVFB=1 uses it on a desktop
# too, for a game an agent drives that should not open a window on yours.
with_display() {
	if [[ ${XVFB:-0} != 1 && (-n ${DISPLAY:-} || -n ${WAYLAND_DISPLAY:-}) ]]; then
		"$@"
	else
		env -u WAYLAND_DISPLAY xvfb-run --auto-servernum \
			--server-args="-screen 0 1920x1080x24" "$@"
	fi
}

# Runs gdlint or gdformat: the installed one if there is one (the image has
# it), else the pinned version through uvx, so neither needs installing first.
gdtool() {
	if command -v "$1" >/dev/null; then
		"$@"
	else
		uvx --from "gdtoolkit==$GDTOOLKIT_VERSION" "$@"
	fi
}

# Every Godot run is checked for this, because Godot exits 0 from a script
# that hit a parse error in something it loaded, and says so only on stderr.
export GODOT_ERROR_PATTERN='^(SCRIPT )?ERROR:|Parse Error|Failed to load script'
