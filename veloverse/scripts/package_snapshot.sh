#!/bin/bash
# Packages a frozen copy of the project, so a release cooks while the editor keeps building and
# saving the live one: copies VeloverseUE (less Saved and the DDC) to /workspace/ue/snap, packages
# that with package.sh into out/package_snap, and tars it as out/veloverse-linux-<time>.tar.gz.
# Take the snapshot when no build is saving (the queue between steps, or during rides).
set -eu
snap=/workspace/ue/snap/VeloverseUE
rm -rf "$snap" && install -d -o ue4 -g ue4 "$snap"
tar -C /workspace/ue/VeloverseUE --exclude ./Saved --exclude ./DerivedDataCache -cf - . |
	sudo -u ue4 tar -C "$snap" -xf -
echo "snapshot $(du -sh "$snap" | cut -f1) at $(date +%H:%M)"
OC_PKG_PROJECT=$snap OC_PKG_OUT=/workspace/ue/out/package_snap OC_PKG_LOG=/workspace/ue/package_snap.log \
	bash "$(dirname "$0")/package.sh"
name=/workspace/ue/out/veloverse-linux-$(date +%Y%m%d-%H%M).tar.gz
tar -C /workspace/ue/out -czf "$name" package_snap
echo "artifact $name ($(du -sh "$name" | cut -f1))"
