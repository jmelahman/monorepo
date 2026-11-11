#!/bin/bash
# The 25 km loop with the latest look work: the kit regenerated (grass kinds, lobed pine pads,
# house colours), the terrain material rebuilt (patches, litter, surf) and the level and
# stills redone into OC_OUT (default out/loop25_v3). Terrain and paths are the ones pod_long25.sh put in place.
set -u
cd /workspace/ue || exit 1
export OC_PLAN=long OC_KM=25
step() { echo "== $(date +%H:%M:%S) $*"; }
step kit
python3 -I scripts/make_village.py | tail -1
python3 -I scripts/make_sugi.py | tail -1
python3 -I scripts/make_houses.py | tail -1
rm -rf VeloverseUE/Content/Coast/Gen/Kit/V_tuft_* VeloverseUE/Content/Coast/Gen/Kit/K_matsu_* VeloverseUE/Content/Coast/Gen/Kit/H_*
step level
OC_OUT=${OC_OUT:-/workspace/ue/out/loop25_v3} OC_RETEX=1 \
	OC_STILLS=$(seq -s, 50 1500 24000) bash scripts/coast_rebuild.sh
step "done"
