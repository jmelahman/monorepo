"""Builds L_Bench: L_Forest with SEQ_Ride playing on a loop as the game starts, so a plain
`UnrealEditor PROJECT /Game/Spike/L_Bench -game` rides the road in real time for the CSV profiler.

Run it as a commandlet after make_forest.py and make_ride.py. L_Forest is a plain (not World
Partition) map, so saving the loaded world under a new name copies it whole.
"""

import unreal

SOURCE = "/Game/Spike/L_Forest"
BENCH = "/Game/Spike/L_Bench"

world = unreal.EditorLoadingAndSavingUtils.load_map(SOURCE)
actors = unreal.get_editor_subsystem(unreal.EditorActorSubsystem)
sequence = unreal.load_asset("/Game/Spike/SEQ_Ride")

player = actors.spawn_actor_from_class(unreal.LevelSequenceActor, unreal.Vector(0, 0, 0))
player.set_actor_label("BenchRide")
player.set_sequence(sequence)
settings = player.get_editor_property("playback_settings")
settings.set_editor_property("auto_play", True)
loop = unreal.MovieSceneSequenceLoopCount()
loop.set_editor_property("value", -1)
settings.set_editor_property("loop_count", loop)
player.set_editor_property("playback_settings", settings)

if not unreal.EditorLoadingAndSavingUtils.save_map(world, BENCH):
    raise RuntimeError(f"could not save {BENCH}")
unreal.log(f"BENCH saved {BENCH}")
