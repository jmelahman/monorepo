"""Logs the rider's pose in Unreal at frame 0 of each take, to check it against rider/README.md.

    bash scripts/ue_run.sh probe_rider.py rider_probe.log

For the reference pose and each A_Rider_Pedal* take: component-space positions (cm, the bike's
frame, origin on the ground under the BB) of the hands, the palms (middle_01) and the head, and
the back angle from the pelvis to neck_01 against the horizontal (the README's aero Pedal: palms
51.3 cm forward of the BB, back 34 degrees).
"""

import math

import unreal

FOLDER = "/Game/Rider/Body"
BONES = ("pelvis", "neck_01", "head", "upperarm_l", "lowerarm_l", "hand_l", "middle_01_l", "hand_r")


def log(message):
    unreal.log(f"PROBE {message}")


def report(label, pose):
    world = unreal.AnimPoseSpaces.WORLD
    at = {b: unreal.AnimPoseExtensions.get_bone_pose(pose, b, world).translation for b in BONES}
    for b in BONES:
        v = at[b]
        log(f"{label} {b} ({v.x:.1f}, {v.y:.1f}, {v.z:.1f})")
    p, n = at["pelvis"], at["neck_01"]
    back = math.degrees(math.atan2(n.z - p.z, math.hypot(n.x - p.x, n.y - p.y)))
    log(f"{label} back {back:.1f} deg, pelvis-to-neck dx {n.x - p.x:.1f}")


mesh = unreal.load_asset(f"{FOLDER}/SK_Rider")
skeleton = mesh.get_editor_property("skeleton")
report("ref", unreal.AnimPoseExtensions.get_reference_pose(skeleton))
opts = unreal.AnimPoseEvaluationOptions()
for name in ("A_Rider_Pedal_Easy", "A_Rider_Pedal", "A_Rider_Pedal_Hard"):
    anim = unreal.load_asset(f"{FOLDER}/{name}")
    log(
        f"{name} frames {anim.get_editor_property('number_of_sampled_frames')} rate "
        f"{anim.get_editor_property('target_frame_rate')}"
    )
    report(name, unreal.AnimPoseExtensions.get_anim_pose_at_time(anim, 0.0, opts))
