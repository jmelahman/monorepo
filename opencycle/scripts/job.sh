#!/bin/bash
# Rebuilds the forest level and renders the ride, detached from ssh. Writes job.log, then job.done.
cd /workspace/ue || exit 1
echo "JOB OC_OUT=${OC_OUT:-unset} OC_LEVEL=${OC_LEVEL:-unset} OC_TREE_SHAPE=${OC_TREE_SHAPE:-unset} OC_CURVE=${OC_CURVE:-1200}"
# Both make_forest.py and make_ride.py must use the same centreline; defaulting here keeps
# them in step when the caller does not pass the bend explicitly.
export OC_CURVE="${OC_CURVE:-1200}"
export OC_WAVELENGTH="${OC_WAVELENGTH:-45000}"
rm -f job.done
{
	# shellcheck disable=SC2024 # the log redirect is meant to run as the calling user
	sudo -u ue4 env UE-LocalDataCachePath=/workspace/ddc HOME=/home/ue4 OC_TREES="${OC_TREES:-}" OC_LENGTH="${OC_LENGTH:-}" OC_FLOOR="${OC_FLOOR:-}" \
		OC_DEPTH="${OC_DEPTH:-}" OC_UNDERGROWTH="${OC_UNDERGROWTH:-}" OC_VERGE_UNDERGROWTH="${OC_VERGE_UNDERGROWTH:-}" OC_FOG="${OC_FOG:-}" \
		OC_CURVE="${OC_CURVE}" OC_WAVELENGTH="${OC_WAVELENGTH}" \
		OC_TREE="${OC_TREE:-}" OC_LEAF_OVERRIDE="${OC_LEAF_OVERRIDE:-}" OC_LEAF_DEBUG="${OC_LEAF_DEBUG:-}" \
		OC_TRIANGLE_ASSEMBLY="${OC_TRIANGLE_ASSEMBLY:-}" OC_TREE_SHAPE="${OC_TREE_SHAPE:-}" \
		OC_VOXEL_OPACITY="${OC_VOXEL_OPACITY:-}" OC_LEAF_UV_SCALE="${OC_LEAF_UV_SCALE:-}" \
		OC_LEAF_TINT="${OC_LEAF_TINT:-}" OC_SUN="${OC_SUN:-}" OC_SUN_LUX="${OC_SUN_LUX:-}" \
		OC_SUN_SOFT="${OC_SUN_SOFT:-}" OC_SKY_LIGHT="${OC_SKY_LIGHT:-}" OC_LEAF_REBUILD="${OC_LEAF_REBUILD:-}" \
		/home/ue4/UnrealEngine/Engine/Binaries/Linux/UnrealEditor-Cmd /workspace/ue/OpenCycleUE/OpenCycleUE.uproject \
		-run=pythonscript -script=/workspace/ue/scripts/make_forest.py -unattended -nosplash -stdout -FullStdOutLogOutput \
		>make_forest.log 2>&1 </dev/null
	grep -E "FOREST|Error" make_forest.log | grep -v Deprecation | cut -c30- | head
	grep -q "FOREST saved" make_forest.log && bash /workspace/ue/scripts/ride.sh
} >job.log 2>&1
touch job.done
