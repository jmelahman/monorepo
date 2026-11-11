#!/bin/bash
# The short ride rebuilt from scratch on the local machine: terrain, paths and kit regenerated,
# everything re-imported, the level built and stills rendered into OC_OUT. Runs at low CPU priority.
#   OC_OUT=/workspace/ue/out/local_v1 nice -n 15 bash scripts/local_short.sh > local_short.log 2>&1
set -u
cd /workspace/ue || exit 1
export OC_PLAN=short
export OC_OUT=${OC_OUT:-/workspace/ue/out/local_v1}
step() { echo "== $(date +%H:%M:%S) $*"; }
step generate
rm -f assets_gen/terrain/T_*.gltf assets_gen/terrain/T_*.bin assets_gen/paths/P_*
python3 -I scripts/make_terrain.py | tail -1
python3 -I scripts/make_paths.py | tail -1
python3 -I scripts/make_village.py | tail -1
python3 -I scripts/make_sugi.py | tail -1
python3 -I scripts/make_houses.py | tail -1
rm -rf VeloverseUE/Content/Coast/Terrain VeloverseUE/Content/Coast/Gen/Paths VeloverseUE/Content/Coast/Gen/Kit/V_* VeloverseUE/Content/Coast/Gen/Kit/K_matsu_* VeloverseUE/Content/Coast/Gen/Kit/K_sugi_* VeloverseUE/Content/Coast/Gen/Kit/H_*
step import
OC_TAIL=6 bash scripts/ue_run.sh coast_assets.py coast_assets_local.log
step level
OC_RETEX=1 OC_STILLS=${OC_STILLS:-50,300,550,830,930,1010,1090,1300,1700,2050} bash scripts/coast_rebuild.sh
step "done"
