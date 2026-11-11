#!/bin/bash
# Rides the coast in the editor's -game mode, windowed on the host's Wayland display (as editor.sh).
# Extra arguments pass through, e.g. -RideStart=1200 -RideWatts=220. OC_MAP picks the level, e.g.
# OC_MAP=/Game/World/L_World for the ride round the whole world (make_world_level.py).
MAP=${OC_MAP:-/Game/Coast/L_Coast}
chmod o+w "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY"
cd /workspace/ue || exit 1
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env HOME=/home/ue4 UE-LocalDataCachePath=/workspace/ddc XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" \
	WAYLAND_DISPLAY="$WAYLAND_DISPLAY" SDL_VIDEO_DRIVER=wayland VK_ICD_FILENAMES="$VK_ICD_FILENAMES" \
	setsid nohup /home/ue4/UnrealEngine/Engine/Binaries/Linux/UnrealEditor \
	/workspace/ue/OpenCycleUE/OpenCycleUE.uproject "$MAP" -game -windowed -resx=1280 -resy=720 \
	-log "$@" >play.log 2>&1 </dev/null &
echo "ride starting; log in /workspace/ue/play.log"
