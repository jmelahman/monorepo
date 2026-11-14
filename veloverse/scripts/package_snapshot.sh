#!/bin/bash
# Packages a frozen copy of the project, so a release cooks while the editor keeps building and
# saving the live one: copies VeloverseUE (less Saved and the DDC) to /workspace/ue/snap, packages
# that with package.sh into out/package_snap, and tars it as out/veloverse-linux-<time>.tar.gz.
# Take the snapshot when no build is saving (the queue between steps, or during rides).
set -eu
snap=/workspace/ue/snap/VeloverseUE
rm -rf "$snap" && install -d -o ue4 -g ue4 "$snap"
# -h: Source, Config and the .uproject are links into the repo, and a copied link would build
# whatever the source is when the compile reaches it, edits in progress and all.
tar -h -C /workspace/ue/VeloverseUE --exclude ./Saved --exclude ./DerivedDataCache -cf - . |
	sudo -u ue4 tar -C "$snap" -xf -
# Straight from the repo: config files the live project doesn't link (DefaultScalability.ini went
# unpackaged that way), and the cursors, read as loose files from Content/Menu.
repo=$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd -P) # scripts/ is linked from /workspace/ue
for f in "$repo"/VeloverseUE/Config/*.ini; do
	[ -e "$snap/Config/${f##*/}" ] || install -o ue4 -g ue4 -m 644 "$f" "$snap/Config/"
done
install -d -o ue4 -g ue4 "$snap/Content/Menu/Cursors"
install -o ue4 -g ue4 -m 644 "$repo"/VeloverseUE/Cursors/*.png "$snap/Content/Menu/Cursors/"
echo "snapshot $(du -sh "$snap" | cut -f1) at $(date +%H:%M)"
OC_PKG_PROJECT=$snap OC_PKG_OUT=/workspace/ue/out/package_snap OC_PKG_LOG=/workspace/ue/package_snap.log \
	bash "$(dirname "$0")/package.sh"
# The notes ride along: how to install and run it, and what's new.
cp "$(dirname "$0")/../docs/release-notes.md" /workspace/ue/out/package_snap/README.md
name=/workspace/ue/out/veloverse-linux-$(date +%Y%m%d-%H%M).tar.gz
tar -C /workspace/ue/out -czf "$name" package_snap
echo "artifact $name ($(du -sh "$name" | cut -f1))"
