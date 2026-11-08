#!/bin/bash
# Runs one of our Python scripts in the editor commandlet as ue4, passing every OC_* variable.
# Usage: scripts/ue_run.sh SCRIPT.py LOG
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/OpenCycleUE/OpenCycleUE.uproject
vars=()
while IFS='=' read -r name _; do vars+=("$name=${!name}"); done < <(env | grep '^OC_')
# OC_EDITOR=1 runs the script in the whole editor, rendering offscreen, rather than the commandlet:
# a landscape's material permutations and grass maps are only made where there is a renderer. The
# script should end with unreal.SystemLibrary.quit_editor().
if [ -n "${OC_EDITOR:-}" ]; then
	run=("$UE/UnrealEditor" "$PROJECT" -RenderOffscreen -ExecutePythonScript="/workspace/ue/scripts/$1")
else
	run=("$UE/UnrealEditor-Cmd" "$PROJECT" -run=pythonscript -script="/workspace/ue/scripts/$1")
fi
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 "${vars[@]}" \
	"${run[@]}" -unattended -nosplash -stdout -FullStdOutLogOutput >"$2" 2>&1 </dev/null
grep -E "LogPython|Error" "$2" | grep -v -E "Deprecation|LogPython: (Warning|Display)" | cut -c30- | tail -"${OC_TAIL:-40}"
