#!/bin/bash
# Runs one of our Python scripts in the editor commandlet as ue4, passing every OC_* variable.
# Usage: scripts/ue_run.sh SCRIPT.py LOG
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/OpenCycleUE/OpenCycleUE.uproject
vars=()
while IFS='=' read -r name _; do vars+=("$name=${!name}"); done < <(env | grep '^OC_')
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 "${vars[@]}" \
	"$UE/UnrealEditor-Cmd" "$PROJECT" -run=pythonscript -script="/workspace/ue/scripts/$1" \
	-unattended -nosplash -stdout -FullStdOutLogOutput >"$2" 2>&1 </dev/null
grep -E "LogPython|Error" "$2" | grep -v -E "Deprecation|LogPython: (Warning|Display)" | cut -c30- | tail -"${OC_TAIL:-40}"
