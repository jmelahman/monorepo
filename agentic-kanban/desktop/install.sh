#!/usr/bin/env bash
# Builds the desktop app from this checkout and installs it for the current
# user. Rerun it to upgrade; it never changes the checkout, so pull first to
# pick up new commits. `./install.sh --uninstall` removes it again. Settings in
# ~/.config/kanban-desktop are kept either way.
set -euo pipefail

app_dir="$HOME/.local/opt/agentic-kanban"
new_dir="$app_dir.new"
bin_link="$HOME/.local/bin/agentic-kanban"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
icon="$data_home/icons/hicolor/512x512/apps/agentic-kanban.png"
# Named after desktopName in package.json so windows match this launcher entry.
desktop_entry="$data_home/applications/agentic-kanban.desktop"

usage() {
	echo "Usage: $0 [--uninstall]" >&2
	exit 2
}

# Swapping files under a running Electron breaks any process it spawns later.
require_not_running() {
	if pgrep -f "^$app_dir/" >/dev/null; then
		echo "Agentic Kanban is running. Quit it, then run this again." >&2
		exit 1
	fi
}

refresh_launcher() {
	if command -v update-desktop-database >/dev/null; then
		update-desktop-database "$(dirname "$desktop_entry")" || true
	fi
}

uninstall() {
	require_not_running
	rm -rf "$app_dir"
	rm -f "$bin_link" "$icon" "$desktop_entry"
	refresh_launcher
	echo "Uninstalled Agentic Kanban."
}

install_app() {
	command -v bun >/dev/null || {
		echo "This needs Bun: https://bun.sh/" >&2
		exit 1
	}
	require_not_running

	cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"
	bun install --frozen-lockfile
	bun x electron-builder --linux dir

	# Copy first and swap last, so a failed copy leaves the old version working.
	trap 'rm -rf "$new_dir"' EXIT
	rm -rf "$new_dir"
	mkdir -p "$(dirname "$app_dir")"
	cp -r dist/linux-unpacked "$new_dir"
	rm -rf "$app_dir"
	mv "$new_dir" "$app_dir"

	install -Dm644 assets/icon.png "$icon"
	mkdir -p "$(dirname "$bin_link")" "$(dirname "$desktop_entry")"
	# Also replaces an AppImage installed at this path by hand.
	ln -sfn "$app_dir/agentic-kanban" "$bin_link"
	cat >"$desktop_entry" <<EOF
[Desktop Entry]
Name=Agentic Kanban
Comment=Desktop client for a running kanban server.
Exec="$app_dir/agentic-kanban"
Icon=agentic-kanban
Type=Application
Categories=Development;
StartupWMClass=agentic-kanban
EOF
	refresh_launcher

	echo "Installed Agentic Kanban from commit $(git rev-parse --short HEAD)."
	case ":$PATH:" in
	*":$(dirname "$bin_link"):"*) ;;
	*) echo "Add $(dirname "$bin_link") to your PATH to run agentic-kanban from a shell." ;;
	esac
}

case "${1:-}" in
"") install_app ;;
--uninstall) uninstall ;;
*) usage ;;
esac
