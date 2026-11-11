#!/bin/bash
# Rebuilds the coast level and renders stills: terrain material (OC_RETEX=1), level, slot fixup, stills.
set -u
cd /workspace/ue || exit 1
# Local machine only: thinner scatter and lighter renders (see local.env).
# shellcheck source=/dev/null
[ -f local.env ] && . ./local.env
if [ "${OC_RETEX:-0}" = 1 ]; then
	OC_STAGE=terrain_material OC_TAIL=2 bash scripts/ue_run.sh coast_assets.py coast_assets_retex.log | grep -E "COAST|Error"
fi
OC_TAIL=40 bash scripts/ue_run.sh make_coast.py make_coast.log | grep -E "COAST (instances|town|paddies|hills|jungle|village|coast|saved|WARN|no mesh)|no material|weather|rain|Error|Traceback"
OC_TAIL=6 bash scripts/ue_run.sh coast_fixup.py coast_fixup.log | grep -E "FIX|Error"
# The playable ride's track, once the module that defines it is built.
if ls VeloverseUE/Binaries/Linux/libUnrealEditor-VeloverseUE*.so >/dev/null 2>&1; then
	OC_TAIL=4 bash scripts/ue_run.sh ride_track.py ride_track.log | grep -E "TRACK|Error"
fi
OC_MODE=stills bash scripts/coast_ride.sh 2>&1 | tail -9
echo "default-material warnings: $(grep -c 'Default Material will be used' coast_render.log)"
grep 'Default Material will be used' coast_render.log | cut -c30-200 | sort -u | head -5
