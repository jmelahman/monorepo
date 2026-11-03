#!/usr/bin/env bash
# Builds the desktop app from this checkout and installs it for the current
# user. Rerun it to upgrade; it builds whatever is checked out, so pull first to
# pick up new commits. `./install.sh --uninstall` removes it again. Settings in
# ~/.config/kanban-desktop are kept either way.
set -euo pipefail

app_dir="$HOME/.local/opt/agentic-kanban"
new_dir="$app_dir.new"
old_dir="$app_dir.old"
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
# Matches on the executable, not the command line, so a launch through the
# $bin_link symlink counts too.
require_not_running() {
	local exe
	for exe in /proc/[0-9]*/exe; do
		case "$(readlink "$exe" 2>/dev/null)" in
		"$app_dir"/*)
			echo "Agentic Kanban is running. Quit it, then run this again." >&2
			exit 1
			;;
		esac
	done
}

# Escapes a path for a quoted argument in a .desktop Exec line: the quoting
# rule's backslash, then the string rule's, then field codes.
desktop_exec_arg() {
	printf '%s' "$1" | sed -e 's/[\\"`$]/\\&/g' -e 's/\\/\\\\/g' -e 's/%/%%/g'
}

refresh_launcher() {
	if command -v update-desktop-database >/dev/null; then
		update-desktop-database "$(dirname "$desktop_entry")" || true
	fi
}

uninstall() {
	require_not_running
	rm -rf "$app_dir"
	# Leave a command at this path alone unless it's this script's symlink.
	if [ "$(readlink "$bin_link" 2>/dev/null)" = "$app_dir/agentic-kanban" ]; then
		rm -f "$bin_link"
	fi
	rm -f "$icon" "$desktop_entry"
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
	bun x electron-builder --linux dir --publish never

	# Copy first and swap last, so a failed copy leaves the old version working.
	trap 'rm -rf "$new_dir"' EXIT
	# An interrupted swap leaves the previous install aside; put it back.
	if [ ! -e "$app_dir" ] && [ -e "$old_dir" ]; then
		mv "$old_dir" "$app_dir"
	fi
	rm -rf "$new_dir" "$old_dir"
	mkdir -p "$(dirname "$app_dir")"
	cp -r dist/linux-unpacked "$new_dir"
	# The build takes a while, so check again before touching the install.
	require_not_running
	if [ -e "$app_dir" ]; then
		mv "$app_dir" "$old_dir"
	fi
	if ! mv "$new_dir" "$app_dir"; then
		[ -e "$old_dir" ] && mv "$old_dir" "$app_dir"
		exit 1
	fi
	rm -rf "$old_dir"

	install -Dm644 assets/icon.png "$icon"
	mkdir -p "$(dirname "$bin_link")" "$(dirname "$desktop_entry")"
	# Also replaces an AppImage installed at this path by hand.
	ln -sfn "$app_dir/agentic-kanban" "$bin_link"
	cat >"$desktop_entry" <<EOF
[Desktop Entry]
Name=Agentic Kanban
Comment=Desktop client for a running kanban server.
Exec="$(desktop_exec_arg "$app_dir/agentic-kanban")"
Icon=agentic-kanban
Type=Application
Categories=Development;
StartupWMClass=agentic-kanban
EOF
	refresh_launcher

	echo "Installed Agentic Kanban from commit $(git describe --always --dirty --exclude="*" 2>/dev/null || echo "a checkout without git history")."
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
