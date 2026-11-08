#!/bin/bash
# Builds, cooks and stages a standalone Linux build of the ride into out/package.
set -u
cd /workspace/ue || exit 1
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env HOME=/home/ue4 UE-LocalDataCachePath=/workspace/ddc \
	/home/ue4/UnrealEngine/Engine/Build/BatchFiles/RunUAT.sh BuildCookRun \
	-project=/workspace/ue/OpenCycleUE/OpenCycleUE.uproject -platform=Linux -clientconfig=Development \
	-build -cook -stage -pak -archive -archivedirectory=/workspace/ue/out/package -nop4 -utf8output \
	-unattended -UbtArgs=-NoUBA >package.log 2>&1 </dev/null
status=$?
grep -E "BUILD SUCCESSFUL|BUILD FAILED|Error:|error:" package.log | head -20
echo "package exit $status; $(du -sh out/package 2>/dev/null | cut -f1) in out/package"
exit $status
