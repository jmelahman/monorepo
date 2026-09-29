#!/usr/bin/env bash
# Sets up a bare machine (a CI runner, a fresh container) for the Godot hooks,
# which are `manual` for now and so run by nothing automatically. They need the apt packages (Xvfb,
# Mesa, the C++ toolchain), Godot and the addons at the pins in
# scripts/versions.env, the native library, and the bundle, which is built
# from 5-wild's own node_modules.
set -euo pipefail
here=$(dirname "${BASH_SOURCE[0]}")
"$here/setup.sh" system godot addons
if [[ ! -x $here/../../node_modules/.bin/rolldown ]]; then
	npm ci --prefix "$here/../.." --no-audit --no-fund
fi
"$here/setup.sh" native bundle import
# setup.sh installs godot to ~/.local/bin; later steps must find it on PATH.
if [[ -n ${GITHUB_PATH:-} ]]; then
	echo "$HOME/.local/bin" >>"$GITHUB_PATH"
fi
