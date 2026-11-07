#!/bin/bash
# Re-imports the regenerated kit pieces named in KIT (default: tufts and pines), then rebuilds the
# short ride and renders stills into OC_OUT, on the local machine at low priority.
#   OC_OUT=/workspace/ue/out/local_v3 nice -n 15 bash scripts/local_kit.sh > local_kit.log 2>&1
set -u
cd /workspace/ue || exit 1
export OC_PLAN=short
step() { echo "== $(date +%H:%M:%S) $*"; }
step import
for k in ${KIT:-V_tuft_ K_matsu_}; do rm -rf OpenCycleUE/Content/Coast/Gen/Kit/"${k}"*; done
OC_TAIL=6 bash scripts/ue_run.sh coast_assets.py coast_assets_local.log
step level
OC_STILLS=${OC_STILLS:-50,300,550,830,930,1010,1090,1300,1700,2050} bash scripts/coast_rebuild.sh
step "done"
