#!/bin/bash
# Builds the long ride (OC_PLAN=long: a loop of about OC_KM kilometres) from scratch on the pod and renders
# stills along it: terrain and paths for the long plan, their import, the level, the slot fixup,
# then one still per OC_STILLS entry into OC_OUT.
#   OC_KM=10 OC_OUT=/workspace/ue/out/long_v1 bash scripts/pod_long.sh
# The short ride's generated terrain is replaced, so run it on the pod's copy, not the local one.
set -u
cd /workspace/ue || exit 1
export OC_PLAN=${OC_PLAN:-long} OC_KM=${OC_KM:-10}
export OC_OUT=${OC_OUT:-/workspace/ue/out/long}
step() { echo "== $(date +%H:%M:%S) $*"; }

step terrain
rm -f assets_gen/terrain/T_*.gltf assets_gen/terrain/T_*.bin
python3 -I scripts/make_terrain.py | tail -1
step paths
rm -f assets_gen/paths/P_*
python3 -I scripts/make_paths.py | tail -1
mkdir -p out && python3 -I scripts/plan_page.py
# Stale tiles and chunks from the short ride would still be spawned by name: clear them first.
rm -rf VeloverseUE/Content/Coast/Terrain VeloverseUE/Content/Coast/Gen/Paths
step import
OC_STAGE=terrain,sea OC_TAIL=4 bash scripts/ue_run.sh coast_assets.py coast_assets_long.log
step level
OC_STILLS=${OC_STILLS:-50,550,930,1700,2600,3200,3800,4400,5000,5450,6200,6900,7500,8100,8700,9400,9800} \
	OC_RETEX=${OC_RETEX:-0} bash scripts/coast_rebuild.sh
step "done"
