#!/bin/bash
# Builds, cooks and stages a standalone Linux build of the ride into out/package.
# OC_PKG_PROJECT packages another copy of the project (package_snapshot.sh's frozen one, so a
# build can cook while the editor keeps saving the live project); OC_PKG_OUT archives elsewhere.
set -u
cd /workspace/ue || exit 1
proj=${OC_PKG_PROJECT:-/workspace/ue/VeloverseUE}
out=${OC_PKG_OUT:-/workspace/ue/out/package}
log=${OC_PKG_LOG:-/workspace/ue/package.log}
# Archive into a fresh folder that ue4 owns: RunUAT runs as ue4 and can't replace files root
# left behind, and an old build's files (such as its launcher) would otherwise ship too.
rm -rf "$out" && install -d -o ue4 -g ue4 "$out" || exit 1
# The project's configs are symlinks into the repo. Staging copies each ini, stamps the copy with
# the link's own time and then checks it against the file's, failing ten times over when they
# differ (11 Nov), so the links take their files' times.
for link in "$proj"/Config/*.ini "$proj"/VeloverseUE.uproject; do
	[ -L "$link" ] && touch -h -r "$(readlink -f "$link")" "$link"
done
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env HOME=/home/ue4 UE-LocalDataCachePath=/workspace/ddc \
	/home/ue4/UnrealEngine/Engine/Build/BatchFiles/RunUAT.sh BuildCookRun \
	-project="$proj"/VeloverseUE.uproject -platform=Linux -clientconfig=Development \
	-build -cook -stage -pak -archive -archivedirectory="$out" -nop4 -utf8output \
	-unattended -UbtArgs=-NoUBA >"$log" 2>&1 </dev/null
status=$?
grep -a -E "BUILD SUCCESSFUL|BUILD FAILED|Error:|error:" "$log" | head -20
echo "package exit $status; $(du -sh "$out" 2>/dev/null | cut -f1) in $out"
exit $status
