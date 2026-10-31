#!/usr/bin/env bash
# Run by .github/workflows/pre-commit.yml, and by the monorepo's, before the
# hooks: the Godot hooks need the apt packages (Xvfb, Mesa), Godot itself and
# the addons, all at the pins in scripts/versions.env.
set -euo pipefail
"$(dirname "${BASH_SOURCE[0]}")/../scripts/setup.sh" system godot addons import
# setup.sh installs godot to ~/.local/bin; later steps must find it on PATH.
if [[ -n ${GITHUB_PATH:-} ]]; then
	echo "$HOME/.local/bin" >>"$GITHUB_PATH"
fi
