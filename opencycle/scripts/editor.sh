#!/bin/bash
# Opens the Unreal editor on the host's Wayland display. Run as root in the devcontainer;
# Unreal refuses root, so it runs as ue4, which needs write access to the Wayland socket.
chmod o+w "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY"
cd /workspace/ue || exit 1
# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
sudo -u ue4 env HOME=/home/ue4 UE-LocalDataCachePath=/workspace/ddc XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" \
	WAYLAND_DISPLAY="$WAYLAND_DISPLAY" SDL_VIDEO_DRIVER=wayland VK_ICD_FILENAMES="$VK_ICD_FILENAMES" \
	setsid nohup /home/ue4/UnrealEngine/Engine/Binaries/Linux/UnrealEditor \
	/workspace/ue/OpenCycleUE/OpenCycleUE.uproject -log >editor.log 2>&1 </dev/null &
echo "editor starting; log in /workspace/ue/editor.log"
