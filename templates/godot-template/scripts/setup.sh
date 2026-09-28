#!/usr/bin/env bash
# Provisions everything the project needs that git does not carry, at the pins
# in versions.env, verified by sha256. Idempotent: each step checks before it
# downloads, so it is cheap to run on every container start or CI job.
#
#   scripts/setup.sh              godot, addons, import (the everyday set)
#   scripts/setup.sh system       apt packages from .devcontainer/packages.txt
#   scripts/setup.sh godot        the editor binary, unless a matching one is on PATH
#   scripts/setup.sh addons       gdUnit4 and GodotSteam into addons/ (gitignored)
#   scripts/setup.sh import       Godot's first import, failing on any error
#   scripts/setup.sh templates    export templates (1.3 GB; only for exporting)
#
# GODOT_PREFIX (default ~/.local) is where `godot` installs; the devcontainer
# image sets /usr/local. Downloads are cached in ~/.cache/godot-downloads, so a
# second project on the same pins downloads nothing.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/env.sh"

CACHE="${GODOT_DOWNLOADS:-$HOME/.cache/godot-downloads}"
PREFIX="${GODOT_PREFIX:-$HOME/.local}"

log() { printf '\033[1m==> %s\033[0m\n' "$*" >&2; }

# fetch URL SHA256 NAME: prints the path of a verified download of URL. A
# cached file with the wrong sum is discarded rather than trusted.
fetch() {
	local url=$1 want=$2 out="$CACHE/$3"
	mkdir -p "$CACHE"
	if [[ -f $out ]] && echo "$want  $out" | sha256sum --check --status; then
		echo "$out"
		return
	fi
	log "downloading $url"
	curl --fail --location --silent --show-error --retry 3 -o "$out.part" "$url"
	local got
	got=$(sha256sum "$out.part" | cut -d' ' -f1)
	if [[ $got != "$want" ]]; then
		rm -f "$out.part"
		echo "sha256 mismatch for $url" >&2
		echo "  want $want" >&2
		echo "  got  $got" >&2
		exit 1
	fi
	mv "$out.part" "$out"
	echo "$out"
}

godot_matches() {
	command -v "$GODOT" >/dev/null && [[ $("$GODOT" --version 2>/dev/null) == "$GODOT_VERSION.stable"* ]]
}

step_system() {
	if ! command -v apt-get >/dev/null; then
		log "no apt-get; install the equivalents of .devcontainer/packages.txt yourself"
		return
	fi
	local sudo=()
	[[ $(id -u) == 0 ]] || sudo=(sudo)
	local packages
	mapfile -t packages < <(grep -Ev '^\s*(#|$)' "$ROOT/.devcontainer/packages.txt")
	log "installing ${#packages[@]} apt packages"
	"${sudo[@]}" apt-get update -qq
	DEBIAN_FRONTEND=noninteractive "${sudo[@]}" apt-get install -y -qq --no-install-recommends "${packages[@]}"
}

step_godot() {
	if godot_matches; then
		log "godot $GODOT_VERSION already at $(command -v "$GODOT")"
		return
	fi
	local arch sum
	case $(uname -m) in
	x86_64) arch=x86_64 sum=$GODOT_SHA256_X86_64 ;;
	aarch64 | arm64) arch=arm64 sum=$GODOT_SHA256_ARM64 ;;
	*)
		echo "no Godot build pinned for $(uname -m)" >&2
		exit 1
		;;
	esac
	local name="Godot_v${GODOT_VERSION}-stable_linux.${arch}"
	local zip
	zip=$(fetch "$GODOT_RELEASE/$name.zip" "$sum" "$name.zip")
	# Versioned directory plus a symlink, so a bump never half-overwrites the
	# binary a running editor has open.
	local dir="$PREFIX/lib/godot-$GODOT_VERSION"
	mkdir -p "$dir" "$PREFIX/bin"
	unzip -oq "$zip" -d "$dir"
	ln -sfn "$dir/$name" "$PREFIX/bin/godot"
	log "installed $PREFIX/bin/godot"
	if [[ $(command -v "$GODOT") != "$PREFIX/bin/godot" ]]; then
		log "note: $(command -v "$GODOT") shadows it on PATH; set GODOT=$PREFIX/bin/godot"
	fi
}

# install_addon DIR MARKER ARCHIVE SUBPATH: replaces addons/DIR with SUBPATH
# from ARCHIVE unless addons/DIR/.version already says MARKER.
install_addon() {
	local dir=$1 marker=$2 archive=$3 sub=$4
	local dest="$ROOT/addons/$dir"
	if [[ -f $dest/.version && $(<"$dest/.version") == "$marker" ]]; then
		log "addons/$dir $marker present"
		return
	fi
	local tmp
	tmp=$(mktemp -d)
	case $archive in
	*.zip) unzip -q "$archive" -d "$tmp" ;;
	*) tar -xzf "$archive" -C "$tmp" ;;
	esac
	rm -rf "$dest"
	# The image build runs this with no addons/ to warm the download cache.
	mkdir -p "$(dirname "$dest")"
	mv "$tmp/$sub" "$dest"
	rm -rf "$tmp"
	echo "$marker" >"$dest/.version"
	log "installed addons/$dir $marker"
}

step_addons() {
	install_addon gdUnit4 "$GDUNIT4_VERSION" \
		"$(fetch "$GDUNIT4_URL" "$GDUNIT4_SHA256" "gdUnit4-$GDUNIT4_VERSION.tar.gz")" \
		"gdUnit4-$GDUNIT4_VERSION/addons/gdUnit4"
	install_addon godotsteam "$GODOTSTEAM_VERSION" \
		"$(fetch "$GODOTSTEAM_URL" "$GODOTSTEAM_SHA256" "godotsteam-$GODOTSTEAM_VERSION.zip")" \
		addons/godotsteam
}

# The first import writes .godot/ (the uid cache, global class names, imported
# assets). Nothing else works before it, and a script that fails to parse
# still exits 0 here, so the log is the verdict.
step_import() {
	log "importing"
	local out
	out=$("$GODOT" --headless --path "$ROOT" --import 2>&1) || {
		echo "$out" >&2
		exit 1
	}
	if grep -E "$GODOT_ERROR_PATTERN" <<<"$out" >&2; then
		echo "import reported errors (above)" >&2
		exit 1
	fi
}

step_templates() {
	local shared="$REAL_DATA_HOME/godot/export_templates/$GODOT_VERSION.stable"
	if [[ ! -f $shared/version.txt ]]; then
		local tpz
		tpz=$(fetch "$GODOT_RELEASE/Godot_v${GODOT_VERSION}-stable_export_templates.tpz" \
			"$GODOT_TEMPLATES_SHA256" "export_templates-$GODOT_VERSION.tpz")
		local tmp
		tmp=$(mktemp -d)
		unzip -q "$tpz" -d "$tmp"
		mkdir -p "$(dirname "$shared")"
		rm -rf "$shared"
		mv "$tmp/templates" "$shared"
		rm -rf "$tmp"
		log "installed export templates to $shared"
	else
		log "export templates $GODOT_VERSION present"
	fi
	# Scripted exports see the redirected data home, so link the shared copy in.
	mkdir -p "$XDG_DATA_HOME/godot/export_templates"
	ln -sfn "$shared" "$XDG_DATA_HOME/godot/export_templates/$GODOT_VERSION.stable"
}

steps=("$@")
[[ ${#steps[@]} -gt 0 ]] || steps=(godot addons import)
for s in "${steps[@]}"; do
	case $s in
	system | godot | addons | import | templates) "step_$s" ;;
	*)
		echo "unknown step: $s (see the header of $0)" >&2
		exit 2
		;;
	esac
done
