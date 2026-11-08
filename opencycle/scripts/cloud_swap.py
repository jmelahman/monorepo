"""Gives the built level's clouds a material instance from
OC_CLOUD="density,erode1,erode2,coverage", and saves.
The engine's default instance is thin (density 0.008) with little detail erosion, which draws
clouds as
flat, hard-edged cut-outs; denser ones shade themselves and eroded ones get soft edges."""

import os

import unreal

LEVEL = "/Game/Coast/L_Coast"
density, e1, e2, coverage = (
    float(x) for x in os.environ.get("OC_CLOUD", "0.03,0.25,0.12,-0.2").split(",")
)
base = unreal.load_asset("/Engine/EngineSky/VolumetricClouds/m_SimpleVolumetricCloud_Inst")
path = "/Game/Coast/MI_Clouds"
if unreal.EditorAssetLibrary.does_asset_exist(path):
    unreal.EditorAssetLibrary.delete_asset(path)
mi = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
    "MI_Clouds",
    "/Game/Coast",
    unreal.MaterialInstanceConstant,
    unreal.MaterialInstanceConstantFactoryNew(),
)
lib = unreal.MaterialEditingLibrary
lib.set_material_instance_parent(mi, base)
lib.set_material_instance_scalar_parameter_value(mi, "Cloud_GlobalDensity", density)
lib.set_material_instance_scalar_parameter_value(mi, "Cloud_GlobalCoverage", coverage)
lib.set_material_instance_vector_parameter_value(
    mi, "Noise_Strength", unreal.LinearColor(0.8, e1, e2, 2.5)
)
lib.update_material_instance(mi)
unreal.EditorAssetLibrary.save_loaded_asset(mi)
unreal.get_editor_subsystem(unreal.LevelEditorSubsystem).load_level(LEVEL)
for actor in unreal.get_editor_subsystem(unreal.EditorActorSubsystem).get_all_level_actors():
    if isinstance(actor, unreal.VolumetricCloud):
        actor.get_component_by_class(unreal.VolumetricCloudComponent).set_editor_property(
            "material", mi
        )
        unreal.log(f"CLOUDS {actor.get_actor_label()} <- {density} {e1} {e2} {coverage}")
unreal.get_editor_subsystem(unreal.LevelEditorSubsystem).save_current_level()
unreal.log("CLOUDS saved")
