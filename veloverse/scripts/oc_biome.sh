#!/bin/bash
# oc_biome.sh BIOME: builds that biome's stretch in L_Hero and shoots five stills.
# Stops the editor, runs the generator, rebuilds the ground and the level, then takes the
# standard daylight look with the preset's own "POST" keys (biomes.py) laid over it.
# About 12 minutes. Run it as one background job; its last line is the look's DONE or FAILED.
set -o pipefail
O=$(dirname "$(readlink -f "$0")")
cd /workspace/ue || exit 1
BIOME=${1:?usage: oc_biome.sh alpine|meadow|desert|coast}
read -r -a P <<<"--grass 1.2 1.5 --fog 0.004 0.02 --post ev=-2.6 --post exposure=0.5 \
--post sky=1.5 --post temp=6500 --post sky_warm=0 --post sun_pitch=-50 --post sun_yaw=43.7 \
--post contrast=1.1 --post saturation=1.05 --post vignette=0.3 --post fog_start=150 \
--post volumetric=0 --post contact=0.05 --post clouds=-0.2 --post cloud_wind=0 --timeout 2400"
read -r -a POST <<<"$(python3 -c "
import sys; sys.path.insert(0, '$O'); import biomes
print(' '.join(f'--post {k}={v}' for k, v in biomes.PRESETS['$BIOME'].get('POST', {}).items()))
")"
python3 "$O/oc.py" stop | tail -1
OC_BIOME=$BIOME uv run -q --no-project --with numpy --with scipy --with pillow python -I \
	"$O/make_climb.py" | tail -1 &&
	python3 "$O/oc.py" restart | tail -1 &&
	python3 "$O/oc.py" rebuild-ground --no-stills --timeout 2400 | tail -1 &&
	python3 "$O/oc.py" restart | tail -1 &&
	python3 "$O/oc.py" build-level --timeout 1800 | tail -1 &&
	python3 "$O/oc.py" restart | tail -1 &&
	python3 "$O/oc.py" look "${P[@]}" "${POST[@]}" --shots road_50,bench,road_08,road_95,hairpin_above | tail -1
