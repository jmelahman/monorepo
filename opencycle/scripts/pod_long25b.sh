#!/bin/bash
# The 25 km loop again, once the first build is done, with the later look work: the kit regenerated
# (twelve grass tufts, needle pines, varied house finishes), the terrain material rebuilt (biplanar cliffs) and the level and
# stills redone into out/loop25_v2. Terrain and paths are the ones pod_long25.sh put in place.
set -u
cd /workspace/ue || exit 1
until grep -q "== .* done" pod_long25.log; do sleep 30; done
export OC_PLAN=long OC_KM=25
step() { echo "== $(date +%H:%M:%S) $*"; }
step kit
python3 -I scripts/make_village.py | tail -1
python3 -I scripts/make_sugi.py | tail -1
python3 -I scripts/make_houses.py | tail -1
rm -rf OpenCycleUE/Content/Coast/Gen/Kit/V_tuft_* OpenCycleUE/Content/Coast/Gen/Kit/K_matsu_* OpenCycleUE/Content/Coast/Gen/Kit/H_*
step level
OC_OUT=/workspace/ue/out/loop25_v2 OC_RETEX=1 \
	OC_STILLS=$(seq -s, 50 1500 24000) bash scripts/coast_rebuild.sh
step "done"
