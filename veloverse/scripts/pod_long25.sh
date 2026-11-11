#!/bin/bash
# Builds the 25 km loop from terrain and paths generated off the pod (uploaded to gen25/), on the first pod tree
# import, level, stills every 1.5 km into out/loop25_v1.
set -u
cd /workspace/ue || exit 1
until [ -f gen25/READY ]; do sleep 20; done
export OC_PLAN=long OC_KM=25
step() { echo "== $(date +%H:%M:%S) $*"; }
step swap
rm -rf assets_gen/terrain assets_gen/paths
cp -r gen25/terrain gen25/paths assets_gen/
rm -rf VeloverseUE/Content/Coast/Terrain VeloverseUE/Content/Coast/Gen/Paths
step import
OC_STAGE=terrain,sea OC_TAIL=4 bash scripts/ue_run.sh coast_assets.py coast_assets_long.log
step level
OC_OUT=/workspace/ue/out/loop25_v1 OC_RETEX=0 \
	OC_STILLS=$(seq -s, 50 1500 24000) bash scripts/coast_rebuild.sh
step "done"
