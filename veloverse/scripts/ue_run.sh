#!/bin/bash
# Runs one of our Python scripts in the editor commandlet as ue4, passing every OC_* variable.
# Usage: scripts/ue_run.sh SCRIPT.py LOG
set -u
UE=/home/ue4/UnrealEngine/Engine/Binaries/Linux
PROJECT=/workspace/ue/VeloverseUE/VeloverseUE.uproject
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
# OC_UE_ARGS adds Unreal options, split on spaces, e.g. a cap on shader compilers where memory is
# short: -ini:Engine:[DevOptions.Shaders]:PercentageUnusedShaderCompilingThreads=83
read -r -a extra <<<"${OC_UE_ARGS:-}"
run+=("${extra[@]}")
# OC_START_MAP opens that level instead of the project's startup map (the coast town, which
# builds its meshes and textures for minutes on every start).
if [ -n "${OC_START_MAP:-}" ]; then
	run+=("-ini:Engine:[/Script/EngineSettings.GameMapsSettings]:EditorStartupMap=$OC_START_MAP")
fi
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 "${vars[@]}" \
	"${run[@]}" -unattended -nosplash -stdout -FullStdOutLogOutput >"$2" 2>&1 </dev/null &
editor=$!
# The editor can crash tearing down ("double free or corruption") and then hang for good, holding
# its memory: a minute after its exit or teardown line, stop it. A hung editor catches TERM and
# never finishes, so a minute after that it is killed outright. A TERM to sudo
# reaches the editor, as does one to this script, e.g. from timeout.
trap 'kill "$editor" 2>/dev/null' TERM INT
exited=0
while kill -0 "$editor" 2>/dev/null; do
	# An ensure starts CrashReportClient, which logs to the same file and ends with its own
	# "LogExit: Exiting." (11 Nov: that killed a healthy build two minutes later). Count only
	# exits it doesn't account for.
	exits=$(grep -a -c "LogExit: Exiting\." "$2" 2>/dev/null)
	reports=$(grep -a -c "reason: CrashReportClientApp RequestExit" "$2" 2>/dev/null)
	if [ "${exits:-0}" -gt "${reports:-0}" ] || grep -a -q -E "Tearing down SDL|double free or corruption" "$2" 2>/dev/null; then
		exited=$((exited + 1))
		[ "$exited" -eq 13 ] && echo "ue_run: editor hung after exiting; stopping it" && kill "$editor"
		[ "$exited" -eq 25 ] && echo "ue_run: editor ignored TERM; killing it" && pkill -KILL -P "$editor"
	fi
	sleep 5 &
	wait $!
done
wait "$editor"
grep -E "LogPython|Error" "$2" | grep -v -E "Deprecation|LogPython: (Warning|Display)" | cut -c30- | tail -"${OC_TAIL:-40}"
