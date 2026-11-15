"""Build the Veloverse player's rider procedurally in Blender, fitted to the player's bike.

Run headless:

    blender -b -P build_rider.py -- [--out DIR] [--bike-blend PATH] [--no-render] [--quick]

The body comes from MPFB (the MakeHuman add-on for Blender) with its CC0 base mesh, targets
and "game_engine" skeleton; the kit, helmet, glasses and shoes are built here. The bike
(../bike/bike.blend, from build_bike.py) is loaded as reference geometry only and is never
exported. Outputs: rider.fbx, rider.blend, T_*.png, render_*.png and stats.json.

Frame: Blender metres, +X forward, +Z up, the drive side at -Y. The armature's origin and its
Root bone sit on the ground under the bottom bracket, like the bike's "Bike" empty.
"""

import copy
import itertools
import math
import os
import subprocess
import sys
import time
from typing import Any

# Blender's bundled modules (and its numpy) aren't in the monorepo's venv.
import bmesh  # ty: ignore[unresolved-import]
import bpy  # ty: ignore[unresolved-import]
import numpy as np  # ty: ignore[unresolved-import]
from mathutils import Matrix, Vector  # ty: ignore[unresolved-import]
from mathutils.bvhtree import BVHTree  # ty: ignore[unresolved-import]

HERE = os.path.dirname(os.path.abspath(__file__))

# =============================================================================================
# The rider (MPFB macros; heights in metres)
# =============================================================================================
RIDER_HEIGHT = 1.780  # crown of the head, standing, barefoot
MACROS = {
    "gender": 1.0,  # male
    "age": 0.5,  # 25 years
    "muscle": 0.55,
    "weight": 0.12,  # lean, a climber's build
    "proportions": 1.0,  # MakeHuman's "ideal" proportions
    "cupsize": 0.5,
    "firmness": 0.5,
}
RACE = {"asian": 0.2, "caucasian": 0.6, "african": 0.2}
# Extra MPFB targets for a cyclist's build: (path under mpfb/data/targets, weight)
BODY_TARGETS = [
    # a slim, narrow silhouette that sits inside the bike's width rather than over it
    ("torso/torso-vshape-decr.target.gz", 0.50),
    ("torso/measure-shoulder-dist-decr.target.gz", 0.80),
    ("torso/torso-scale-horiz-decr.target.gz", 0.40),
    ("torso/torso-scale-depth-decr.target.gz", 0.20),
    ("torso/measure-hips-circ-decr.target.gz", 0.90),
    ("buttocks/buttocks-volume-decr.target.gz", 1.00),
    ("arms/l-upperarm-muscle-decr.target.gz", 0.40),
    ("arms/r-upperarm-muscle-decr.target.gz", 0.40),
    # slim legs: smaller thigh and calf girth, little fat, lean muscle
    ("legs/measure-thigh-circ-decr.target.gz", 0.60),
    ("legs/measure-calf-circ-decr.target.gz", 0.50),
    ("legs/l-upperleg-fat-decr.target.gz", 0.50),
    ("legs/r-upperleg-fat-decr.target.gz", 0.50),
    ("legs/l-lowerleg-fat-decr.target.gz", 0.50),
    ("legs/r-lowerleg-fat-decr.target.gz", 0.50),
    ("stomach/stomach-tone-incr.target.gz", 0.5),
]
# Proportion targets: a positive weight loads the "-incr" target, a negative one the "-decr".
PROPORTION_TARGETS = {
    "leg": ("legs/upperlegs-height", "legs/lowerlegs-height"),
    "arm": ("arms/measure-upperarm-length", "arms/measure-lowerarm-length"),
}

# =============================================================================================
# The bike (mm unless noted). These mirror ../bike/build_bike.py; the meshes are measured too.
# =============================================================================================
BB_Z = 0.269  # m, bottom bracket centre above the ground
CRANK_LENGTH = 0.1725  # m
PEDAL_Y = 0.073 + 0.052  # m, pedal body centre from the bike's centre plane
PEDAL_STACK = 0.012  # m, pedal axle centre to the pedal's top face (the bike's pedal mesh)
CLEAT_THICKNESS = 0.008  # m
SOLE_THICKNESS = 0.007  # m, under the foot
CLEAT_BEHIND_BALL = 0.010  # m, cleat centre behind the ball joint

# =============================================================================================
# The pose
# =============================================================================================
FPS = 30
FRAMES = 60  # one crank revolution, frames 0..59; frame 60 would be frame 0 again
# Efforts: one looping action each, with the same crank timing, hands and feet, so a game can
# blend them by power. Per action: the back (deg from horizontal, hip joints to the base of the
# neck), the hip rock (x HIP_ROCK_*) and where the head's forward axis meets the road (m
# ahead). The hands stay on the hoods, so a lower back brings the shoulders nearer them and
# bends the elbows. "Pedal", the default, is the aero one: a low back on the hoods.
EFFORTS = {
    "Pedal_Easy": (40.0, 0.6, 12.0),
    "Pedal": (32.0, 1.0, 10.0),
    "Pedal_Hard": (28.0, 1.8, 8.0),
}
DEFAULT_EFFORT = "Pedal"
# The first-person action: Pedal's crank timing, hands and finger curls, with the upper body
# sunk down and back so that from the game's cockpit camera the forearms reach in from the
# frame's bottom corners to the hoods. The game hides the head and legs; the head folds down
# out of view anyway. Hip joints (x, z), torso pitch, clavicles, elbow pole, neck and gaze, as
# in Rider.pose. Found by a search that kept the back, shoulders, upper arms and head out of
# the camera's view.
COCKPIT_ACTION = "Cockpit"
COCKPIT = {
    "hip": (-0.42, 0.78),
    "torso": 80.0,
    "clav": 20.0,
    "arm_pole": (-0.20, 0.20, -0.20),
    "neck_extra": 60.0,
    "gaze": 90.0,
}
# The game's cameras (16:9, GAME_HFOV wide): the cockpit camera, pitched down to put the bars
# in the frame's bottom third, and the chase camera, CHASE_DISTANCE back from its target.
GAME_HFOV = 61.0
COCKPIT_CAMERA = (-0.27, 0.0, 1.12)
COCKPIT_PITCH = 5.0  # deg down
CHASE_TARGET = (0.18, 0.0, 1.62)
CHASE_DISTANCE = 5.1
CHASE_PITCH = 2.0  # deg down
# deg short of straight: the shoulders protract until the elbows bend this much, or, once they
# are fully drawn back (a low back), the elbows bend more to take up the reach
ELBOW_BEND = 17.0
SADDLE_COMPRESSION = 0.003  # m of shorts and saddle-padding compression under the sit bones
KNEE_AT_BDC = 145.0  # deg interior knee angle at the bottom of the stroke
HIP_ROCK_ROLL = 1.5  # deg each way
HIP_ROCK_DROP = 0.003  # m, over each downstroke
# deg of upper-torso counter-roll: just enough to hold the shoulders still over the rocking
# pelvis, so the arms stay quiet between the fixed hands and the torso
SHOULDER_COUNTER = 0.25

# Ankling: sole pitch, toe-down positive, against phi (deg past top dead centre, in the
# direction of travel): about +4 at the top, the heel ~10 deg lower through the downstroke,
# toe slightly down at the bottom and through the back of the stroke.
ANKLING = (5.0, -2.0, -10.0, 1.0)  # a0 + a1 cos phi + b1 sin phi + a2 cos 2 phi

# =============================================================================================
# Kit colours (linear)
# =============================================================================================
NAVY = (0.010, 0.024, 0.075)
ORANGE = (0.80, 0.22, 0.05)
BLACK = (0.012, 0.012, 0.013)
WHITE = (0.80, 0.80, 0.78)
SKIN = (0.48, 0.30, 0.22)


# =============================================================================================
# Helpers
# =============================================================================================
def log(*a):
    print("[rider]", *a, flush=True)


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    opts = {
        "out": os.path.join(HERE, "out"),
        "bike_blend": os.path.join(HERE, "..", "bike", "bike.blend"),
        "render": True,
        "quick": False,
        "stage": "all",
    }
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--out":
            opts["out"] = argv[i + 1]
            i += 1
        elif a == "--bike-blend":
            opts["bike_blend"] = argv[i + 1]
            i += 1
        elif a == "--no-render":
            opts["render"] = False
        elif a == "--quick":
            opts["quick"] = True
        elif a == "--stage":
            opts["stage"] = argv[i + 1]
            i += 1
        i += 1
    opts["out"] = os.path.abspath(opts["out"])
    opts["bike_blend"] = os.path.abspath(opts["bike_blend"])
    return opts


def mesh_co(ob, evaluated=False, world=True):
    if evaluated:
        dg = bpy.context.evaluated_depsgraph_get()
        ev = ob.evaluated_get(dg)
        me = ev.to_mesh()
        co = np.empty(len(me.vertices) * 3)
        me.vertices.foreach_get("co", co)
        ev.to_mesh_clear()
    else:
        me = ob.data
        co = np.empty(len(me.vertices) * 3)
        me.vertices.foreach_get("co", co)
    co = co.reshape(-1, 3)
    if world:
        m = np.array(ob.matrix_world)
        co = co @ m[:3, :3].T + m[:3, 3]
    return co


def group_mask(ob, name, threshold=0.5):
    vg = ob.vertex_groups.get(name)
    mask = np.zeros(len(ob.data.vertices), bool)
    if vg is None:
        return mask
    gi = vg.index
    for v in ob.data.vertices:
        for g in v.groups:
            if g.group == gi and g.weight >= threshold:
                mask[v.index] = True
    return mask


def rot_y(deg):
    return Matrix.Rotation(math.radians(deg), 4, "Y")


def rot_z(deg):
    return Matrix.Rotation(math.radians(deg), 4, "Z")


def rot_x(deg):
    return Matrix.Rotation(math.radians(deg), 4, "X")


def rot_about(m4, pivot):
    """The 4x4 that applies rotation m4 about the world point pivot."""
    p = Vector(pivot)
    return Matrix.Translation(p) @ m4 @ Matrix.Translation(-p)


# =============================================================================================
# The bike, as reference geometry
# =============================================================================================
def load_bike(path, out):
    if not os.path.isfile(path):
        ref = os.path.join(out, "bike_ref")
        log(f"no bike at {path}; rebuilding it into {ref}")
        os.makedirs(ref, exist_ok=True)
        script = os.path.join(HERE, "..", "bike", "build_bike.py")
        cmd = [bpy.app.binary_path, "-b", "-P", script, "--", "--out", ref, "--no-render"]
        subprocess.run(cmd, check=True)
        path = os.path.join(ref, "bike.blend")
    names = ["Bike", "Frame", "Fork", "WheelFront", "WheelRear", "Cranks", "PedalL", "PedalR"]
    names.append("Chain")
    with bpy.data.libraries.load(path, link=False) as (src, dst):
        dst.objects = [n for n in src.objects if n in names]
    coll = bpy.data.collections.new("BikeRef")
    bpy.context.scene.collection.children.link(coll)
    bike = {}
    for ob in dst.objects:
        coll.objects.link(ob)
        bike[ob.name] = ob
    bpy.context.view_layer.update()
    for ob in bike.values():
        if ob.type == "MESH":
            log(f"bike {ob.name}: {[m.name for m in ob.data.materials]}, parent {ob.parent}")
    return bike, coll


class BikeRef:
    """Contact points and collision trees measured from the bike's meshes."""

    def __init__(self, bike):
        self.objs = bike
        self.bb = np.array([0.0, 0.0, BB_Z])
        frame = bike["Frame"]
        fork = bike["Fork"]
        # saddle: the Frame's plastic above the top tube
        sad = self.material_verts(frame, "M_BlackPlastic")
        sad = sad[sad[:, 2] > 0.85]
        self.saddle_verts = sad
        self.saddle_xmin, self.saddle_xmax = sad[:, 0].min(), sad[:, 0].max()
        # hoods: the Fork's plastic at cockpit height, per side (the disc caliper on the
        # non-drive side of the fork is the same plastic, low down)
        hood = self.material_verts(fork, "M_BlackPlastic")
        hood = hood[hood[:, 2] > 0.7]
        self.hood_verts = {s: hood[np.sign(hood[:, 1]) == s] for s in (-1, 1)}
        self.trees = {}

    @staticmethod
    def material_verts(ob, mat_name):
        me = ob.data
        idx = [i for i, m in enumerate(me.materials) if m.name == mat_name]
        mi = np.empty(len(me.polygons), int)
        me.polygons.foreach_get("material_index", mi)
        loops = np.empty(len(me.polygons), int)
        me.polygons.foreach_get("loop_total", loops)
        starts = np.empty(len(me.polygons), int)
        me.polygons.foreach_get("loop_start", starts)
        lv = np.empty(len(me.loops), int)
        me.loops.foreach_get("vertex_index", lv)
        sel = np.isin(mi, idx)
        keep = np.zeros(len(me.vertices), bool)
        for s, n in zip(starts[sel], loops[sel], strict=True):
            keep[lv[s : s + n]] = True
        return mesh_co(ob)[keep]

    def saddle_top(self, x, y=0.0, r=0.006):
        s = self.saddle_verts
        near = s[(np.abs(s[:, 0] - x) < r) & (np.abs(s[:, 1] - y) < r)]
        return near[:, 2].max()

    def hood_top(self, side, x, r=0.004):
        h = self.hood_verts[side]
        near = h[np.abs(h[:, 0] - x) < r]
        return near[:, 2].max(), 0.5 * (near[:, 1].max() + near[:, 1].min())

    def tree(self, name):
        """A BVH of the object's mesh in its own space (query with local points)."""
        if name not in self.trees:
            dg = bpy.context.evaluated_depsgraph_get()
            self.trees[name] = BVHTree.FromObject(self.objs[name], dg)
        return self.trees[name]

    def saddle_tree(self):
        """A world-space BVH of the saddle's top (the Frame's plastic above the top tube)."""
        if "_saddle" not in self.trees:
            ob = self.objs["Frame"]
            me = ob.data
            idx = {i for i, m in enumerate(me.materials) if m.name == "M_BlackPlastic"}
            co = mesh_co(ob)
            polys = []
            for p in me.polygons:
                if p.material_index in idx and co[p.vertices[0], 2] > 0.85:
                    polys.append(tuple(p.vertices))
            used = sorted({v for p in polys for v in p})
            remap = {v: i for i, v in enumerate(used)}
            verts = [tuple(co[v]) for v in used]
            self.trees["_saddle"] = BVHTree.FromPolygons(
                verts, [tuple(remap[v] for v in p) for p in polys]
            )
        return self.trees["_saddle"]


def crank_alpha(frame):
    """The drive-side crank's angle from +X towards +Z (rad). The cranks turn so a pedal at the
    top moves forward, which is clockwise seen from the drive side: alpha falls."""
    return -2.0 * math.pi * frame / FRAMES


def pedal_axle(frame, side):
    """Pedal axle centre under the cleat, side -1 = drive (-Y), +1 = non-drive."""
    a = crank_alpha(frame) + (0.0 if side < 0 else math.pi)
    return np.array([CRANK_LENGTH * math.cos(a), side * PEDAL_Y, BB_Z + CRANK_LENGTH * math.sin(a)])


def phi_deg(frame, side):
    """Degrees past top dead centre in the direction of travel (90 = forward, 180 = bottom)."""
    return (90.0 + 360.0 * frame / FRAMES + (0.0 if side < 0 else 180.0)) % 360.0


def sole_pitch(frame, side):
    a0, a1, b1, a2 = ANKLING
    p = math.radians(phi_deg(frame, side))
    return a0 + a1 * math.cos(p) + b1 * math.sin(p) + a2 * math.cos(2 * p)


def pose_bike(bike, frame):
    """Turn the reference bike's cranks and pedals to the frame (pedals stay level)."""
    a = crank_alpha(frame)
    bike["Cranks"].matrix_world = Matrix.Translation((0.0, 0.0, BB_Z)) @ rot_y(-math.degrees(a))
    for name, s in (("PedalR", -1), ("PedalL", 1)):
        aa = a + (0.0 if s < 0 else math.pi)
        loc = (CRANK_LENGTH * math.cos(aa), s * 0.073, BB_Z + CRANK_LENGTH * math.sin(aa))
        bike[name].matrix_world = Matrix.Translation(loc)
    bpy.context.view_layer.update()


# =============================================================================================
# The body (MPFB)
# =============================================================================================
def mpfb():
    import addon_utils  # ty: ignore[unresolved-import]

    mod = "bl_ext.user_default.mpfb"
    if not addon_utils.check(mod)[1]:
        addon_utils.enable(mod, default_set=True)
    from bl_ext.user_default.mpfb.services.humanservice import (  # ty: ignore[unresolved-import]
        HumanService,
    )
    from bl_ext.user_default.mpfb.services.locationservice import (  # ty: ignore[unresolved-import]
        LocationService,
    )
    from bl_ext.user_default.mpfb.services.targetservice import (  # ty: ignore[unresolved-import]
        TargetService,
    )

    return HumanService, TargetService, LocationService


def create_body(height_macro, props):
    """An MPFB human, standing at the origin facing MPFB's -Y, with the game_engine rig."""
    human_service, target_service, location_service = mpfb()
    for ob in [o for o in bpy.data.objects if o.get("mpfb_tmp")]:
        bpy.data.objects.remove(ob, do_unlink=True)
    macro = target_service.get_default_macro_info_dict()
    macro.update(MACROS)
    macro["height"] = height_macro
    macro["race"] = dict(RACE)
    basemesh = human_service.create_human(macro_detail_dict=macro, scale=0.1)
    tdir = location_service.get_mpfb_data("targets")
    for rel, w in BODY_TARGETS:
        target_service.load_target(basemesh, os.path.join(tdir, rel), weight=w)
    for key, rels in PROPORTION_TARGETS.items():
        w = props.get(key, 0.0)
        if abs(w) < 1e-6:
            continue
        for rel in rels:
            path = os.path.join(tdir, f"{rel}-{'incr' if w > 0 else 'decr'}.target.gz")
            target_service.load_target(basemesh, path, weight=abs(w))
    bpy.context.view_layer.update()
    rig = human_service.add_builtin_rig(basemesh, "game_engine")
    basemesh["mpfb_tmp"] = 1
    rig["mpfb_tmp"] = 1
    return basemesh, rig


def body_height(basemesh):
    co = mesh_co_shape_mix(basemesh)[group_mask(basemesh, "body")]
    return co[:, 2].max() - co[:, 2].min()


def mesh_co_shape_mix(ob):
    """Vertex positions with the shape keys mixed in, without modifiers."""
    sk = ob.data.shape_keys
    co = mesh_co(ob, world=False)
    if sk is None:
        return co
    basis = np.empty(len(sk.key_blocks[0].data) * 3)
    sk.key_blocks[0].data.foreach_get("co", basis)
    basis = basis.reshape(-1, 3)
    out = basis.copy()
    for kb in sk.key_blocks[1:]:
        if kb.value == 0.0 or kb.mute:
            continue
        k = np.empty(len(kb.data) * 3)
        kb.data.foreach_get("co", k)
        out += kb.value * (k.reshape(-1, 3) - basis)
    return out


def build_body(props):
    """The MPFB human at RIDER_HEIGHT, rigged with the game_engine skeleton, turned into the bike
    frame (facing +X, its left at +Y) and with the helper geometry removed. Returns
    (mesh, rig, eyes) with eyes = the two eyeball centres, left (+Y) first."""
    human_service, target_service, _ = mpfb()
    t0 = time.time()
    basemesh, _ = create_body(MACRO_HEIGHT, props)
    # exact height by a uniform scale about the floor (the macro gets within a centimetre)
    h = body_height(basemesh)
    target_service.bake_targets(basemesh)
    s = RIDER_HEIGHT / h
    basemesh.data.transform(Matrix.Scale(s, 4))
    # the proportion targets move the feet: stand the soles back on the floor
    floor = mesh_co_shape_mix(basemesh)[group_mask(basemesh, "body")][:, 2].min()
    basemesh.data.transform(Matrix.Translation((0.0, 0.0, -floor)))
    rig = human_service.add_builtin_rig(basemesh, "game_engine")
    rig["mpfb_tmp"] = 1
    rig.name = "Armature"
    rig.data.name = "Armature"
    basemesh.name = "Body"
    basemesh.data.name = "Body"
    # MPFB's human faces -Y with its left at +X; the bike's rider faces +X with the left at +Y
    R = rot_z(90.0)
    for ob in (rig, basemesh):
        ob.matrix_parent_inverse = Matrix.Identity(4)
    rig.data.transform(R @ rig.matrix_world)
    rig.matrix_world = Matrix.Identity(4)
    basemesh.data.transform(R @ basemesh.matrix_world)
    basemesh.matrix_basis = Matrix.Identity(4)
    # MPFB's Root sits 3 mm up; put it on the ground (it deforms nothing)
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.mode_set(mode="EDIT")
    eb = rig.data.edit_bones["Root"]
    eb.head.z = 0.0
    eb.tail.z = 0.0
    bpy.ops.object.mode_set(mode="OBJECT")
    bpy.context.view_layer.update()
    # eyeball centres from the helper eyes, then drop the helpers except the eyeballs
    co = mesh_co(basemesh, world=False)
    eyes = [co[group_mask(basemesh, f"helper-{s}-eye")].mean(0) for s in ("l", "r")]
    keep = group_mask(basemesh, "body")
    for lr in ("l", "r"):
        for g in ("eye", "eyelashes-1", "eyelashes-2"):
            keep |= group_mask(basemesh, f"helper-{lr}-{g}")
    bm = bmesh.new()
    bm.from_mesh(basemesh.data)
    bm.verts.ensure_lookup_table()
    bmesh.ops.delete(bm, geom=[v for v in bm.verts if not keep[v.index]], context="VERTS")
    bm.to_mesh(basemesh.data)
    bm.free()
    for m in list(basemesh.modifiers):
        if m.type != "ARMATURE":
            basemesh.modifiers.remove(m)
    seat_weights(basemesh, rig)
    log(f"body {props}: standing height {h * s:.3f} m (scale {s:.4f}), {time.time() - t0:.1f}s")
    return basemesh, rig, eyes


def smoothstep(e0, e1, x):
    t = np.clip((x - e0) / (e1 - e0), 0.0, 1.0)
    return t * t * (3 - 2 * t)


def vertex_weights(ob, names):
    """Dense (verts x names) weight matrix of the named vertex groups."""
    idx = {ob.vertex_groups[n].index: i for i, n in enumerate(names) if n in ob.vertex_groups}
    W = np.zeros((len(ob.data.vertices), len(names)))
    for v in ob.data.vertices:
        for g in v.groups:
            j = idx.get(g.group)
            if j is not None:
                W[v.index, j] = g.weight
    return W


def set_vertex_weights(ob, names, W, rows):
    for j, n in enumerate(names):
        vg = ob.vertex_groups.get(n) or ob.vertex_groups.new(name=n)
        for i in rows:
            if W[i, j] > 1e-4:
                vg.add([int(i)], float(W[i, j]), "REPLACE")
            else:
                vg.remove([int(i)])


def seat_weights(mesh, rig):
    """Seated weights for the buttocks: MPFB weights them partly to the thighs, so a flexed
    thigh swings them down through the saddle. Behind and below the hip joints, move that
    share of the weight onto the pelvis, fading out down the back of the thigh."""
    names = ["pelvis", "thigh_l", "thigh_r"]
    W = vertex_weights(mesh, names)
    co = mesh_co(mesh, world=False)
    bones = rig.data.bones
    rows = []
    for j, s in ((1, "l"), (2, "r")):
        hip = np.array(bones[f"thigh_{s}"].head_local)
        knee = np.array(bones[f"thigh_{s}"].tail_local)
        axis = (knee - hip) / np.linalg.norm(knee - hip)
        rel = co - hip
        t = rel @ axis / np.linalg.norm(knee - hip)
        behind = smoothstep(0.01, -0.05, rel[:, 0])
        down = 1.0 - smoothstep(0.12, 0.42, t)
        f = behind * down * (W[:, j] > 0)
        moved = W[:, j] * f
        W[:, j] -= moved
        W[:, 0] += moved
        rows.extend(np.where(f > 0)[0].tolist())
    set_vertex_weights(mesh, names, W, sorted(set(rows)))


MACRO_HEIGHT = 0.517  # MPFB height macro that gives about RIDER_HEIGHT; scaled exactly after


# =============================================================================================
# Pose
# =============================================================================================
SIDES = (("l", 1), ("r", -1))  # MPFB's left is the bike's non-drive side, +Y
TORSO = ("pelvis", "spine_01", "spine_02", "spine_03", "neck_01", "head")
PELVIS_SHARE = 0.8  # of the torso's forward pitch taken by the pelvis; the spine takes the rest
SPINE_SHARE = (0.30, 0.35, 0.35)
# The grip (place_hands, then solve_grip). The palm point sits over the hood's body and lifts
# until the palm and knuckles rest on it. Then each joint, knuckle first, closes from its
# relaxed curl about its bone's X until the skin comes GRIP_GAP off the hood, lever or bar,
# within its (open, closed) range in degrees.
HOOD_CONTACT_X = 0.553  # m, the palm point along the hood: on its body, behind the horn
HOOD_OUT = 0.0  # m, the palm point outboard of the hood's middle
HOOD_LIFT = 0.009  # m, palm point above the hood's top: the thenar pad bulges below it
HOOD_LIFT_RANGE = (-0.015, 0.045)  # m, the lift searched on top of HOOD_LIFT
GRIP_RANGE = {"01": (-10.0, 85.0), "02": (0.0, 100.0), "03": (0.0, 70.0)}
THUMB_RANGE = {"01": (-10.0, 30.0), "02": (0.0, 45.0), "03": (0.0, 50.0)}
GRIP_OPEN = 15.0  # deg, the most a joint opens from its relaxed curl to clear the hood
THUMB_CURL = {"01": 8.0, "02": 18.0, "03": 20.0}  # before the grip is solved
GRIP_REST = {"01": 8.0, "02": 12.0, "03": 8.0}  # before the grip is solved
GRIP_GAP = 0.0015  # m, skin to the hood or lever
GRIP_STEP = 1.0  # deg
FINGERS = ("index", "middle", "ring", "pinky")
# The hand on the hood (deg): the wrist-to-knuckle line turned out from straight ahead and
# pitched down, and the palm rolled from facing down toward the bike's centre. A handshake
# grip: the hood's top in the web of the thumb, the palm round its outside, the fingers over
# the lever and the thumb round the inside. Found by a search that every finger touch the hood
# or lever with its tip below its knuckle, and the thumb's tip inside the hood.
HAND_YAW = 10.0
HAND_PITCH = 5.0
HAND_ROLL = 75.0
# the elbows' direction from the shoulder-wrist midpoint (x, out, z): back, down and in, so
# they tuck under the chest and the arms barely show beside the back from behind
ARM_POLE = (-0.20, -0.20, -0.20)
CLAV_MIN = 16.0  # deg, the least the shoulders protract: rounded, not braced back
CLAV_MAX = 30.0  # deg, the most the shoulders protract
HIP_X = -0.24  # m, the hip joints behind the BB
NECK_EXTRA = 30.0  # deg, the neck pitched down past halfway between back and gaze: head low
CLAV_DROP = 8.0  # deg, the shoulders' drop about the clavicles' line


def angle_at(a, b, c):
    """Interior angle at b (deg)."""
    u = np.asarray(a) - np.asarray(b)
    v = np.asarray(c) - np.asarray(b)
    cosv = np.dot(u, v) / (np.linalg.norm(u) * np.linalg.norm(v))
    return math.degrees(math.acos(max(-1.0, min(1.0, cosv))))


def frame_from(axis, normal):
    """An orthonormal 3x3 with columns (axis, normal x axis, normal), normal made orthogonal."""
    a = Vector(axis).normalized()
    n = Vector(normal)
    n = (n - n.dot(a) * a).normalized()
    m = Matrix.Identity(3)
    m.col[0] = a
    m.col[1] = n.cross(a)
    m.col[2] = n
    return m


def hand_frame(side):
    """The hand's world frame on the hood (columns: wrist to knuckles, across, palm normal)."""
    y, p, r = (math.radians(a) for a in (HAND_YAW, HAND_PITCH, HAND_ROLL))
    axis = (math.cos(p) * math.cos(y), side * math.cos(p) * math.sin(y), -math.sin(p))
    return frame_from(axis, (0.0, -side * math.sin(r), -math.cos(r)))


class Rider:
    """The MPFB body and rig posed on the bike. Torso, clavicles, neck, head and fingers are
    set directly; the arms and legs are solved with Blender IK (constraints, baked later)."""

    def __init__(self, mesh, rig, eyes, bike):
        self.mesh, self.rig, self.bike = mesh, rig, bike
        self.eyes_rest = [Vector(e) for e in eyes]
        self.rest = {b.name: b.matrix_local.copy() for b in rig.data.bones}
        self.parent = {b.name: (b.parent.name if b.parent else None) for b in rig.data.bones}
        pb = rig.pose.bones
        for b in pb:
            b.rotation_mode = "QUATERNION"
        self.head_rest = {b.name: Vector(b.bone.head_local) for b in pb}
        self.tail_rest = {b.name: Vector(b.bone.tail_local) for b in pb}
        self.hip_rest = (self.head_rest["thigh_l"] + self.head_rest["thigh_r"]) / 2
        self.X = {}
        self.hood = {}
        self.palm = {}
        self.cleat_rest = {}
        self.empties = []
        self.setup_hands()
        self.setup_feet()
        self.setup_constraints()

    # ----- rest-pose measurements -----
    def setup_hands(self):
        co = mesh_co(self.mesh, world=False)
        for s, _side in SIDES:
            wrist = self.head_rest[f"hand_{s}"]
            knuckle = self.head_rest[f"middle_01_{s}"]
            axis = (knuckle - wrist).normalized()
            normal = Vector(self.rest[f"hand_{s}"].to_3x3().col[2])  # bone +Z: the palm side
            m = group_mask(self.mesh, f"hand_{s}", 0.6)
            p = co[m]
            t = (p - np.array(wrist)) @ np.array(axis) / (knuckle - wrist).length
            sel = p[(t > 0.35) & (t < 0.75)]
            d = (sel - np.array(wrist)) @ np.array(normal)
            top = sel[np.argsort(d)[-12:]].mean(0)
            self.palm[s] = {"axis": axis, "normal": normal, "point": Vector(top), "wrist": wrist}

    def setup_feet(self):
        for s, _side in SIDES:
            ball = self.head_rest[f"ball_{s}"]
            # the sole is the floor (MPFB stands the feet on z = 0); the cleat sits on the
            # foot's centre line just behind the ball joint. The point that rides the pedal
            # axle is PEDAL_STACK under the cleat's face, so the cleat rests on the pedal's top.
            self.cleat_rest[s] = Vector(
                (
                    ball.x - CLEAT_BEHIND_BALL,
                    ball.y,
                    -(SOLE_THICKNESS + CLEAT_THICKNESS + PEDAL_STACK),
                )
            )

    def empty(self, name):
        ob = bpy.data.objects.new(name, None)
        ob.empty_display_size = 0.05
        bpy.context.scene.collection.objects.link(ob)
        self.empties.append(ob)
        return ob

    def setup_constraints(self):
        pb = self.rig.pose.bones
        self.targets = {}
        for s, _side in SIDES:
            for chain, tip, rotb in (
                ("arm", f"lowerarm_{s}", f"hand_{s}"),
                ("leg", f"calf_{s}", f"foot_{s}"),
            ):
                tgt = self.empty(f"_ik_{chain}_{s}")
                pole = self.empty(f"_pole_{chain}_{s}")
                # elbows and knees are hinges about their bone's X
                pb[tip].lock_ik_y = True
                pb[tip].lock_ik_z = True
                c = pb[tip].constraints.new("IK")
                c.target = tgt
                c.pole_target = pole
                c.chain_count = 2
                c.use_tail = True
                c.use_stretch = False
                c.iterations = 500
                r = pb[rotb].constraints.new("COPY_ROTATION")
                r.target = tgt
                r.target_space = "WORLD"
                r.owner_space = "WORLD"
                self.targets[(chain, s)] = (tgt, pole, c)

    def dispose(self):
        for ob in self.empties:
            bpy.data.objects.remove(ob, do_unlink=True)
        self.empties = []

    # ----- the transforms -----
    def chain_X(self, name, D, parent_X, pivot_rest=None, translate=None):
        """Rigid world transform of a bone: rotate by D about its rest head, which has been
        carried by the parent's transform (or move pivot_rest to translate)."""
        h = self.head_rest[name] if pivot_rest is None else pivot_rest
        to = parent_X @ h if translate is None else translate
        X = Matrix.Translation(to) @ D.to_4x4() @ Matrix.Translation(-h)
        self.X[name] = X
        return X

    def set_basis(self, name):
        par = self.parent[name]
        Xp = self.X.get(par, Matrix.Identity(4)) if par else Matrix.Identity(4)
        rest = self.rest[name]
        basis = (Xp @ rest).inverted() @ self.X[name] @ rest
        self.rig.pose.bones[name].matrix_basis = basis

    def pose(self, frame, P):
        """Pose for a frame. P holds the solved fit: hip (x, z), torso pitch, clavicle
        protraction and the pole angles."""
        phi = math.radians(phi_deg(frame, -1))  # drive side
        rock = math.sin(phi)
        roll = HIP_ROCK_ROLL * rock * P.get("rock", 1.0)
        drop = HIP_ROCK_DROP * math.sin(phi) ** 2 * P.get("rock", 1.0)
        upper = -SHOULDER_COUNTER * rock * P.get("rock", 1.0)
        T = P["torso"]
        pp = T * PELVIS_SHARE
        hip = Vector((P["hip"][0], 0.0, P["hip"][1]))
        # pelvis: pitch about the hip joints, then rock on the saddle about the sit-bone centre
        D_p = rot_y(pp).to_3x3()
        Xp = Matrix.Translation(hip) @ D_p.to_4x4() @ Matrix.Translation(-self.hip_rest)
        c = Vector((P["sit_x"], 0.0, P["saddle_z"]))
        Xp = Matrix.Translation((0.0, 0.0, -drop)) @ rot_about(rot_x(roll), c) @ Xp
        self.X["pelvis"] = Xp
        self.set_basis("pelvis")
        acc = pp
        rolls = (0.3 * roll + 0.7 * upper, upper, upper)
        for i, name in enumerate(("spine_01", "spine_02", "spine_03")):
            acc += (T - pp) * SPINE_SHARE[i]
            D = (rot_x(rolls[i]) @ rot_y(acc)).to_3x3()
            self.chain_X(name, D, self.X[self.parent[name]])
            self.set_basis(name)
        D3 = (rot_x(upper) @ rot_y(acc)).to_3x3()
        for s, side in SIDES:
            D = (
                D3
                @ rot_z(-side * P["clav"]).to_3x3()
                @ rot_x(-side * P.get("clav_drop", CLAV_DROP)).to_3x3()
            )
            self.chain_X(f"clavicle_{s}", D, self.X["spine_03"])
            self.set_basis(f"clavicle_{s}")
        # neck: between the back and the head; head: fixed in the world, looking down the road
        gaze = P["gaze"]
        neck_pitch = 0.5 * (acc + gaze) + P.get("neck_extra", NECK_EXTRA)
        self.chain_X("neck_01", rot_y(neck_pitch).to_3x3(), self.X["spine_03"])
        self.set_basis("neck_01")
        self.chain_X("head", rot_y(gaze).to_3x3(), self.X["neck_01"])
        self.set_basis("head")
        # fingers
        pb = self.rig.pose.bones
        grip = P.get("grip", {})
        for s, _side in SIDES:
            for f in FINGERS:
                for k, a in GRIP_REST.items():
                    pb[f"{f}_{k}_{s}"].matrix_basis = rot_x(grip.get(f"{f}_{k}_{s}", a))
            for k, a in THUMB_CURL.items():
                pb[f"thumb_{k}_{s}"].matrix_basis = rot_x(grip.get(f"thumb_{k}_{s}", a))
        # arms: palm on the hood, hand fixed in the world
        for s, side in SIDES:
            tgt, pole, con = self.targets[("arm", s)]
            hp = self.palm[s]
            Fr = frame_from(hp["axis"], hp["normal"])
            Fd = hand_frame(side)
            Dh = Fd @ Fr.transposed()
            contact = Vector(self.hood[s])
            wrist = contact - Dh @ (hp["point"] - hp["wrist"])
            rest_rot = self.rest[f"hand_{s}"].to_3x3()
            tgt.matrix_world = Matrix.Translation(wrist) @ (Dh @ rest_rot).to_4x4()
            sh = self.X[f"clavicle_{s}"] @ self.head_rest[f"upperarm_{s}"]
            mid = (sh + wrist) / 2
            ax, ay, az = P.get("arm_pole", ARM_POLE)
            pole.location = mid + Vector((ax, side * ay, az))
            con.pole_angle = P["pole"][("arm", s)]
        # legs: cleat on the pedal axle, sole pitched by the ankling curve
        for s, side in SIDES:
            tgt, pole, con = self.targets[("leg", s)]
            pitch = sole_pitch(frame, side)
            R = rot_y(pitch).to_3x3()
            cleat = Vector(pedal_axle(frame, side))
            ankle = cleat + R @ (self.head_rest[f"foot_{s}"] - self.cleat_rest[s])
            rest_rot = self.rest[f"foot_{s}"].to_3x3()
            tgt.matrix_world = Matrix.Translation(ankle) @ (R @ rest_rot).to_4x4()
            hipj = self.X["pelvis"] @ self.head_rest[f"thigh_{s}"]
            pole.location = Vector(
                ((hipj.x + ankle.x) / 2 + 0.8, side * PEDAL_Y, (hipj.z + ankle.z) / 2)
            )
            con.pole_angle = P["pole"][("leg", s)]
        bpy.context.view_layer.update()

    # ----- measurements of the current pose -----
    def bone_pts(self, name):
        pb = self.rig.pose.bones[name]
        return Vector(pb.head), Vector(pb.tail)

    def knee_angle(self, s):
        hip, knee = self.bone_pts(f"thigh_{s}")
        return angle_at(hip, knee, self.rig.pose.bones[f"calf_{s}"].tail)

    def elbow_bend(self, s):
        sh, el = self.bone_pts(f"upperarm_{s}")
        return 180.0 - angle_at(sh, el, self.rig.pose.bones[f"lowerarm_{s}"].tail)

    def back_angle(self):
        """Hip joints to the base of the neck, from horizontal (the clavicles' protraction
        moves the shoulder joints, so they don't measure the back)."""
        pb = self.rig.pose.bones
        hip = (Vector(pb["thigh_l"].head) + Vector(pb["thigh_r"].head)) / 2
        d = Vector(pb["neck_01"].head) - hip
        return math.degrees(math.atan2(d.z, d.x))

    def pose_mat(self, name):
        return self.rig.pose.bones[name].matrix @ self.rest[name].inverted()

    def cleat_world(self, s):
        return self.pose_mat(f"foot_{s}") @ self.cleat_rest[s]

    def palm_world(self, s):
        return self.pose_mat(f"hand_{s}") @ self.palm[s]["point"]

    def hand_skin(self, s, bones):
        """Rest positions and normalised weights of the hand vertices that follow `bones`
        (with hand_s as their root), for skinning in numpy."""
        names = [f"hand_{s}", *bones]
        W = vertex_weights(self.mesh, names)
        rows = np.where(W[:, 1:].sum(1) > 0.05)[0]
        W = W[rows] / np.maximum(W[rows].sum(1, keepdims=True), 1e-6)
        co = mesh_co(self.mesh, world=False)[rows]
        return rows, W, np.c_[co, np.ones(len(rows))]

    def skin_chain(self, s, bones, curl, W, V):
        """Linear-blend skin V by hand_s (as posed) and the chain `bones` curled about X."""
        hand = f"hand_{s}"
        M = {hand: self.rig.pose.bones[hand].matrix.copy()}
        Ms = [np.array(M[hand] @ self.rest[hand].inverted())]
        for b in bones:
            par = self.parent[b]
            M[b] = M[par] @ self.rest[par].inverted() @ self.rest[b] @ rot_x(curl[b])
            Ms.append(np.array(M[b] @ self.rest[b].inverted()))
        out = sum(W[:, j : j + 1] * (V @ Ms[j].T) for j in range(len(Ms)))
        return out[:, :3]

    def cockpit_clearance(self, pts):
        """Signed distance (m) of each point to the cockpit (the Fork object: bar, hoods,
        levers); negative inside."""
        fork = self.bike.objs["Fork"]
        tree = self.bike.tree("Fork")
        inv = fork.matrix_world.inverted()
        d = np.full(len(pts), 1.0)
        for i, q in enumerate(pts):
            loc = inv @ Vector(q)
            hit = tree.find_nearest(loc, 0.05)
            if hit[0] is not None:
                d[i] = hit[3] if (loc - hit[0]).dot(hit[1]) >= 0 else -hit[3]
        return d

    def solve_grip(self):
        """Close the fingers and thumbs on the hoods, with the hands where the current pose
        put them. Each joint, knuckle first, starts at its relaxed curl. If its skin (skinned
        here in numpy) is in the cockpit there, it opens by up to GRIP_OPEN to where it's
        clearest. Then it closes until its skin would come nearer than GRIP_GAP (or deeper in
        than it started). The hands are fixed in the world, so one solve holds for every frame.
        Returns {bone: deg}."""
        bpy.context.view_layer.update()
        grip = {}
        for s, _side in SIDES:
            for f in (*FINGERS, "thumb"):
                rng = THUMB_RANGE if f == "thumb" else GRIP_RANGE
                bones = [f"{f}_{k}_{s}" for k in rng]
                _rows, W, V = self.hand_skin(s, bones)
                relax = THUMB_CURL if f == "thumb" else GRIP_REST
                curl = {b: relax[b.split("_")[1]] for b in bones}
                for j, b in enumerate(bones):
                    moving = W[:, 1 + j :].sum(1) > 0.25
                    lo, hi = rng[b.split("_")[1]]
                    angles = np.arange(lo, hi + 1e-6, GRIP_STEP)
                    clear = []
                    for a in angles:
                        curl[b] = float(a)
                        pts = self.skin_chain(s, bones, curl, W, V)[moving]
                        clear.append(self.cockpit_clearance(pts).min())
                    clear = np.array(clear)
                    rest = relax[b.split("_")[1]]
                    i = int(np.argmin(np.abs(angles - rest)))
                    if clear[i] < GRIP_GAP:  # in the hood relaxed: open a little if that helps
                        near = np.where(angles >= rest - GRIP_OPEN)[0]
                        near = near[angles[near] <= rest]
                        i = int(near[np.argmax(clear[near] - 1e-6 * (rest - angles[near]))])
                    floor = min(GRIP_GAP, clear[i])  # never deeper in than it started
                    while i + 1 < len(angles) and clear[i + 1] >= floor:
                        i += 1  # close until the next step would touch
                    curl[b] = float(angles[i])
                grip.update(curl)
                c = self.cockpit_clearance(self.skin_chain(s, bones, curl, W, V)).min()
                log(
                    f"grip {f}_{s}: "
                    + ", ".join(f"{curl[b]:.0f}" for b in bones)
                    + f" deg, closest {1000 * c:+.1f} mm"
                )
        return grip

    def eye_point(self):
        X = self.pose_mat("head")
        return (X @ self.eyes_rest[0] + X @ self.eyes_rest[1]) / 2

    def calibrate_poles(self, P, frame=0):
        """Find each IK chain's pole angle: the one that puts its middle joint in the plane
        through its root, its target and its pole."""
        for (chain, s), (tgt, pole, _con) in self.targets.items():
            mid = f"{'lowerarm' if chain == 'arm' else 'calf'}_{s}"
            root = f"{'upperarm' if chain == 'arm' else 'thigh'}_{s}"

            def err(a, chain=chain, s=s, mid=mid, root=root, tgt=tgt, pole=pole):
                P["pole"][(chain, s)] = a
                self.pose(frame, P)
                r = Vector(self.rig.pose.bones[root].head)
                m = Vector(self.rig.pose.bones[mid].head)
                t = tgt.matrix_world.translation
                pl = pole.matrix_world.translation
                n = (t - r).cross(pl - r).normalized()
                # the middle joint must bend toward the pole: compare the parts of both that
                # are perpendicular to the root-target line
                u = (t - r).normalized()
                em = (m - r) - (m - r).dot(u) * u
                ep = (pl - r) - (pl - r).dot(u) * u
                side_ok = em.dot(ep) > 0
                return abs((m - r).dot(n)) + (0.0 if side_ok else 1.0)

            grid = np.radians(np.arange(-180, 180, 15))
            best = min(grid, key=err)
            lo, hi = best - math.radians(15), best + math.radians(15)
            for _ in range(30):
                a1 = lo + (hi - lo) * 0.382
                a2 = lo + (hi - lo) * 0.618
                if err(a1) < err(a2):
                    hi = a2
                else:
                    lo = a1
            P["pole"][(chain, s)] = (lo + hi) / 2
            err(P["pole"][(chain, s)])
            log(f"pole {chain}_{s}: {math.degrees(P['pole'][(chain, s)]):.1f} deg")

    def saddle_penetration(self):
        """Deepest body point below the saddle's top (m, positive = into the saddle) and the
        mean position of the points within 2 mm of it."""
        tree = self.bike.saddle_tree()
        co = mesh_co(self.mesh, evaluated=True)
        if not hasattr(self, "seat_mask"):
            # the sitting surface: skin that moves with the pelvis (the thighs pass beside
            # the saddle's nose and are checked with the rest of the body later)
            self.seat_mask = vertex_weights(self.mesh, ["pelvis"])[:, 0] >= 0.5
        sel = np.where(
            self.seat_mask
            & (co[:, 0] > self.bike.saddle_xmin - 0.01)
            & (co[:, 0] < self.bike.saddle_xmax + 0.01)
            & (np.abs(co[:, 1]) < 0.09)
            & (co[:, 2] < 1.1)
        )[0]
        pens = []
        for i in sel:
            p = co[i]
            hit = tree.ray_cast(Vector((p[0], p[1], 1.5)), Vector((0, 0, -1)))
            if hit[0] is None:
                continue
            pen = hit[0].z - p[2]
            if pen > -0.03 and pen < 0.25:
                pens.append((pen, p))
        if not pens:
            return -1.0, None
        depth = max(q for q, _ in pens)
        if os.environ.get("RIDER_DEBUG"):
            pb = self.rig.pose.bones
            worst = max(pens, key=lambda t: t[0])[1]
            log(
                "  worst",
                np.round(worst, 3).tolist(),
                "hip",
                np.round(pb["thigh_l"].head, 3).tolist(),
                "pelvis",
                np.round(pb["pelvis"].head, 3).tolist(),
            )
        near = np.array([p for q, p in pens if q > depth - 0.002])
        return depth, near.mean(0)


# =============================================================================================
# The fit
# =============================================================================================
def solve_fit(rider, bike, effort=DEFAULT_EFFORT, P0=None, iters=8):
    """Fit the torso, the shoulders and the seat to an effort's back angle. From
    scratch, the palms start at their first point on the hoods; from P0 (a solved fit) they stay
    where it put them, with its grip, so every effort holds the bars the same way."""
    if P0 is not None:
        return _fit_loop(rider, copy.deepcopy(P0), effort, iters)
    hood = {}
    for s, side in SIDES:
        hood[s] = hood_point(bike, HOOD_CONTACT_X, side)
    rider.hood = hood
    P: dict[str, Any] = {
        "hip": [HIP_X, 1.13],
        "torso": 48.0,
        "clav": 8.0,
        "gaze": 8.5,
        "sit_x": -0.25,
        "saddle_z": float(bike.saddle_top(-0.25)),
        "rock": 0.0,
        "pole": {k: 0.0 for k in rider.targets},
    }
    return _fit_loop(rider, P, effort, iters)


def _fit_loop(rider, P, effort, iters):
    back_target, _rock, gaze_m = EFFORTS[effort]
    rider.calibrate_poles(P)

    def bisect(key, lo, hi, f, n=14):
        """Solve f() = 0 for P[key] in [lo, hi], f rising with P[key]."""
        for _ in range(n):
            mid = 0.5 * (lo + hi)
            if isinstance(key, tuple):
                P[key[0]][key[1]] = mid
            else:
                P[key] = mid
            rider.pose(0, P)
            if f() > 0:
                hi = mid
            else:
                lo = mid
        return 0.5 * (lo + hi)

    for it in range(iters):
        # sit: the deepest point sinks SADDLE_COMPRESSION into the saddle
        P["hip"][1] = bisect(
            ("hip", 1), 0.95, 1.30, lambda: SADDLE_COMPRESSION - rider.saddle_penetration()[0]
        )
        # arms: protract the shoulders until the elbows bend ELBOW_BEND
        P["clav"] = bisect(
            "clav",
            CLAV_MIN,
            CLAV_MAX,
            lambda: 0.5 * (rider.elbow_bend("l") + rider.elbow_bend("r")) - ELBOW_BEND,
        )
        rider.pose(0, P)
        back = rider.back_angle()
        bend = 0.5 * (rider.elbow_bend("l") + rider.elbow_bend("r"))
        depth, c = rider.saddle_penetration()
        eye = rider.eye_point()
        log(
            f"fit {it}: hip {P['hip'][1]:.3f}, torso {P['torso']:.1f}, clav {P['clav']:.1f},"
            f" back {back:.1f}, elbow {bend:.1f}, saddle {depth * 1000:.1f} mm"
            f" at x {c[0] if c is not None else float('nan'):.3f}, eye {np.round(eye, 3).tolist()}"
        )
        if c is not None:
            P["sit_x"] = float(c[0])
        P["gaze"] = math.degrees(math.atan2(eye.z, gaze_m))
        if abs(back - back_target) < 0.2 and it > 0:
            break
        P["torso"] += back - back_target
    rider.calibrate_poles(P)
    P["effort"] = effort
    return P


def hood_point(bike, x, side, dz=0.0):
    """The palm point over the hood at x: HOOD_OUT outboard of the hood's middle and
    HOOD_LIFT + dz above its top."""
    z, y = bike.hood_top(side, x)
    return (x, y + side * HOOD_OUT, z + HOOD_LIFT + dz)


def place_hands(rider, P):
    """Lift each palm point off its hood until the palm, knuckles and the fingers' first
    segments rest GRIP_GAP off the cockpit (bisected to a tenth of a millimetre). The arms
    follow by IK. Then the fingers and thumbs close on the hood (solve_grip)."""
    names = [b.name for b in rider.rig.data.bones]
    W = vertex_weights(rider.mesh, names)
    dom = np.array(names)[W.argmax(1)]
    rigid = {}
    for s, _side in SIDES:
        hand = W[:, names.index(f"hand_{s}")]
        fing = np.array(
            [d.endswith(f"_{s}") and d.split("_")[0] in (*FINGERS, "thumb") for d in dom]
        )
        first = np.isin(dom, [f"{f}_01_{s}" for f in FINGERS])
        rigid[s] = np.where((dom == f"hand_{s}") | (fing & (hand > 0.3)) | first)[0]

    def clearance(dz):
        for s, side in SIDES:
            rider.hood[s] = hood_point(rider.bike, HOOD_CONTACT_X, side, dz)
        rider.pose(0, P)
        co = mesh_co(rider.mesh, evaluated=True, world=False)
        return min(rider.cockpit_clearance(co[rigid[s]]).min() for s, _ in SIDES)

    lo, hi = HOOD_LIFT_RANGE
    for _ in range(10):
        mid = 0.5 * (lo + hi)
        if clearance(mid) > GRIP_GAP:
            hi = mid
        else:
            lo = mid
    worst = clearance(hi)
    log(f"hands: palm lifted {1000 * hi:+.1f} mm; palm and knuckles {1000 * worst:+.1f} mm off")
    P["hood"] = {s: list(rider.hood[s]) for s, _ in SIDES}
    rider.calibrate_poles(P)
    rider.pose(0, P)
    P["grip"] = rider.solve_grip()
    rider.pose(0, P)


def max_knee(rider, P, step=2):
    """The most open knee over the stroke, both legs (deg)."""
    out = 0.0
    for k in range(0, FRAMES, step):
        rider.pose(k, P)
        out = max(out, rider.knee_angle("l"), rider.knee_angle("r"))
    return out


def solve_proportions(bike, iters=8):
    """Solve the leg length (MPFB's leg-height targets, then the body rescaled to RIDER_HEIGHT)
    so the knee opens to KNEE_AT_BDC at the bottom of the stroke, by the secant method."""
    hist = []
    w = 0.0
    rider = None
    for _ in range(iters):
        if rider is not None:
            rider.dispose()
        mesh, rig, eyes = build_body({"leg": w})
        rider = Rider(mesh, rig, eyes, bike)
        P = solve_fit(rider, bike)
        knee = max_knee(rider, P)
        log(f"proportions: leg {w:+.3f} -> knee at BDC {knee:.2f}")
        hist.append((w, knee))
        if abs(knee - KNEE_AT_BDC) < 0.3:
            break
        if len(hist) < 2:
            w_next = w - (knee - KNEE_AT_BDC) / 35.0  # about 35 deg per unit of leg weight
        else:
            (w0, k0), (w1, k1) = hist[-2], hist[-1]
            w_next = w1 - (k1 - KNEE_AT_BDC) * (w1 - w0) / (k1 - k0)
        w = float(np.clip(w_next, -1.0, 1.0))
    P["props"] = {"leg": hist[-1][0]}
    place_hands(rider, P)
    return rider, P


def solve_efforts(rider, bike, P):
    """Fit every effort from the default one's fit: the same body, palms and grip."""
    fits = {DEFAULT_EFFORT: P}
    for name in EFFORTS:
        if name != DEFAULT_EFFORT:
            fits[name] = solve_fit(rider, bike, name, P0=P)
    for name, Q in fits.items():
        rider.pose(0, Q)
        log(
            f"effort {name}: back {rider.back_angle():.1f}, elbows"
            f" {0.5 * (rider.elbow_bend('l') + rider.elbow_bend('r')):.1f},"
            f" shoulders {Q['clav']:.1f}, knee at BDC {max_knee(rider, Q):.1f}"
        )
    fits[COCKPIT_ACTION] = fit_cockpit(rider, P)
    rider.pose(0, P)
    return fits


def fit_cockpit(rider, P):
    """The Cockpit action's fit: Pedal's, with COCKPIT's upper body. The hands keep Pedal's
    palm points and grip, and the crank keeps its timing and hip rock."""
    Q = copy.deepcopy(P)
    Q.update({k: (list(v) if isinstance(v, tuple) else v) for k, v in COCKPIT.items()})
    Q["action"] = COCKPIT_ACTION
    rider.calibrate_poles(Q)
    views = []
    for k in range(0, FRAMES, 4):
        rider.pose(k, Q)
        views.append(cockpit_view(rider))
    worst = max(views, key=lambda v: v["seen_upper_body_verts"])
    worst = dict(worst, camera_to_arms_m=min(v["camera_to_arms_m"] for v in views))
    log(f"cockpit, the worst of every 4th frame: {worst}")
    Q["view"] = worst
    return Q


def game_camera(cam, loc, pitch_down):
    """The game's camera: at loc, looking down the road (+X), pitched down, GAME_HFOV wide."""
    cam.location = loc
    cam.rotation_euler = (math.radians(90.0 - pitch_down), 0.0, math.radians(-90.0))
    cam.data.type = "PERSP"
    cam.data.sensor_fit = "HORIZONTAL"
    cam.data.angle = math.radians(GAME_HFOV)


def chase_camera_location():
    p = math.radians(CHASE_PITCH)
    return Vector(CHASE_TARGET) - CHASE_DISTANCE * Vector((math.cos(p), 0.0, -math.sin(p)))


def cockpit_view(rider):
    """What the cockpit camera sees of the rider as posed: the vertices of the back, shoulders,
    upper arms and head inside its frustum (16:9), the forearm vertices it sees, where they
    enter the frame (the lowest and most outboard, in -1..1 of the half-frame), and the camera's
    distance to the nearest clavicle or arm vertex."""
    names = [b.name for b in rider.rig.data.bones]
    if not hasattr(rider, "cockpit_groups"):
        W = vertex_weights(rider.mesh, names)
        dom = np.array(names)[W.argmax(1)].astype(str)
        upper = np.isin(dom, ["pelvis", "spine_01", "spine_02", "spine_03", "neck_01", "head"])
        upper |= np.char.startswith(dom, "clavicle") | np.char.startswith(dom, "upperarm")
        fore = np.char.startswith(dom, "lowerarm")
        arms = np.char.startswith(dom, "clavicle") | np.char.startswith(dom, "upperarm") | fore
        rider.cockpit_groups = (upper, fore, arms)
    upper, fore, arms = rider.cockpit_groups
    co = mesh_co(rider.mesh, evaluated=True)
    c = np.array(COCKPIT_CAMERA)
    th = math.radians(COCKPIT_PITCH)
    fwd = np.array([math.cos(th), 0.0, -math.sin(th)])
    up = np.array([math.sin(th), 0.0, math.cos(th)])
    tx = math.tan(math.radians(GAME_HFOV / 2))
    ty = tx * 9 / 16
    d = co - c
    z = d @ fwd
    zs = np.maximum(z, 1e-6)
    x = -d[:, 1] / zs / tx  # +Y is the frame's left
    y = (d @ up) / zs / ty
    seen = (z > 0.02) & (np.abs(x) < 1) & (np.abs(y) < 1)
    f = seen & fore
    return {
        "seen_upper_body_verts": int((seen & upper).sum()),
        "seen_forearm_verts": int(f.sum()),
        "forearm_entry": [round(float(np.abs(x[f]).max()), 2), round(float(y[f].min()), 2)]
        if f.any()
        else None,
        "camera_to_arms_m": round(float(np.linalg.norm(d[arms], axis=1).min()), 3),
    }


def fit_report(rider, P):
    out = {}
    for k in (0, 15, 30, 45):
        rider.pose(k, P)
        out[k] = {
            "knee_l": rider.knee_angle("l"),
            "knee_r": rider.knee_angle("r"),
            "cleat_err": max(
                (rider.cleat_world(s) - Vector(pedal_axle(k, side))).length for s, side in SIDES
            ),
            "palm_err": max((rider.palm_world(s) - Vector(rider.hood[s])).length for s, _ in SIDES),
        }
        log(f"frame {k}: " + ", ".join(f"{a} {b:.4f}" for a, b in out[k].items()))
    return out


def debug_render(path, cam_loc, target, ortho=None, res=(1200, 900)):
    sc = bpy.context.scene
    sc.render.engine = "BLENDER_WORKBENCH"
    sc.display.shading.light = "STUDIO"
    sc.display.shading.color_type = "MATERIAL"
    sc.render.resolution_x, sc.render.resolution_y = res
    cam = bpy.data.objects.get("_dbgcam")
    if cam is None:
        cam = bpy.data.objects.new("_dbgcam", bpy.data.cameras.new("_dbgcam"))
        sc.collection.objects.link(cam)
    sc.camera = cam
    cam.location = cam_loc
    cam.rotation_euler = (Vector(target) - Vector(cam_loc)).to_track_quat("-Z", "Y").to_euler()
    if ortho:
        cam.data.type = "ORTHO"
        cam.data.ortho_scale = ortho
    else:
        cam.data.type = "PERSP"
        cam.data.lens = 50
    sc.render.filepath = path
    bpy.ops.render.render(write_still=True)


# =============================================================================================
# Gear: helmet, straps, glasses, brows and shoes (built in the rest pose)
# =============================================================================================
ARM_PARTS = ("upperarm", "lowerarm", "hand", "thumb", "index", "middle", "ring", "pinky")
LEG_PARTS = ("thigh", "calf", "foot", "ball")
HELMET_PAD = 0.010  # m, scalp to the helmet's inner shell (pads and hair)
HELMET_SHELL = 0.022  # m, shell thickness at the crown
HELMET_AZ, HELMET_ROWS = 144, 26
# vent slots: (centre of d.y, d.x from, d.x to, half width in d.y), d the unit direction from
# the helmet's centre; mirrored for the +Y lanes
HELMET_VENTS = [
    (0.0, 0.32, 0.80, 0.035),
    (0.0, -0.82, -0.30, 0.035),
    (0.22, 0.26, 0.82, 0.040),
    (0.22, -0.86, -0.24, 0.040),
    (0.46, 0.16, 0.62, 0.034),
    (0.46, -0.70, -0.20, 0.034),
]
LENS_R = 0.090  # m, radius of the wraparound lens's cylinder
LENS_GAP = 0.017  # m, front of the eyeball to the lens
SHOE_GAP = 0.004  # m, foot skin to the shoe's outer surface
STRAP_WIDTH, STRAP_THICK = 0.013, 0.0015


def unit(v):
    v = np.asarray(v, float)
    return v / np.linalg.norm(v)


def new_object(name, verts, faces, uvs=None, mats=(), mat_idx=None, groups=None, rig=None):
    """A mesh object from numpy verts and face tuples, with per-corner UVs (list per face),
    material slots, vertex groups {name: weights per vertex} and an armature modifier."""
    me = bpy.data.meshes.new(name)
    me.from_pydata([tuple(v) for v in verts], [], [tuple(f) for f in faces])
    uv = me.uv_layers.new(name="UVMap")
    if uvs is not None:
        flat = [c for f in uvs for c in f]
        uv.data.foreach_set("uv", np.asarray(flat, np.float32).ravel())
    for m in mats:
        me.materials.append(m)
    if mat_idx is not None:
        me.polygons.foreach_set("material_index", np.asarray(mat_idx, np.int32))
    me.validate()
    me.update()
    ob = bpy.data.objects.new(name, me)
    bpy.context.scene.collection.objects.link(ob)
    for g, w in (groups or {}).items():
        vg = ob.vertex_groups.new(name=g)
        w = np.broadcast_to(np.asarray(w, float), (len(verts),))
        for i in np.where(w > 0)[0]:
            vg.add([int(i)], float(w[i]), "REPLACE")
    if rig is not None:
        ob.parent = rig
        mod = ob.modifiers.new("Armature", "ARMATURE")
        mod.object = rig
    return ob


def apply_modifiers(ob):
    """Bake the object's non-armature modifiers into its mesh."""
    arm = [(m.name, m.object) for m in ob.modifiers if m.type == "ARMATURE"]
    for m in list(ob.modifiers):
        if m.type == "ARMATURE":
            ob.modifiers.remove(m)
    dg = bpy.context.evaluated_depsgraph_get()
    me = bpy.data.meshes.new_from_object(
        ob.evaluated_get(dg), preserve_all_data_layers=True, depsgraph=dg
    )
    old = ob.data
    ob.modifiers.clear()
    ob.data = me
    bpy.data.meshes.remove(old)
    for name, rig in arm:
        mod = ob.modifiers.new(name, "ARMATURE")
        mod.object = rig


def solidify(ob, thickness, offset=-1.0, mat_offset=0, group=None, group_factor=1.0, even=True):
    mod = ob.modifiers.new("Solidify", "SOLIDIFY")
    mod.thickness = thickness
    mod.offset = offset
    mod.use_rim = True
    mod.use_even_offset = even  # off where faces are degenerate: even offset explodes there
    mod.use_quality_normals = True
    mod.material_offset = mat_offset
    mod.material_offset_rim = mat_offset
    if group:
        mod.vertex_group = group
        mod.thickness_vertex_group = group_factor
    apply_modifiers(ob)


def tube_mesh(points, normals, width, thick):
    """A flat strap along points: a width x thick rectangle, its thickness along normals."""
    P = np.asarray(points, float)
    N = np.asarray(normals, float)
    T = np.gradient(P, axis=0)
    verts, faces = [], []
    n = len(P)
    for i in range(n):
        t = unit(T[i])
        nn = unit(N[i] - N[i].dot(t) * t)
        b = np.cross(t, nn)
        w = width[i] if np.ndim(width) else width
        for sb, sn in ((-1, -1), (1, -1), (1, 1), (-1, 1)):
            verts.append(P[i] + sb * b * w / 2 + sn * nn * thick / 2)
    for i in range(n - 1):
        for k in range(4):
            a, c = 4 * i + k, 4 * i + (k + 1) % 4
            faces.append((a + 4, c + 4, c, a))
    faces.append((0, 1, 2, 3))
    m = 4 * (n - 1)
    faces.append((m + 3, m + 2, m + 1, m))
    return np.array(verts), faces


def catmull_rom(ctrl, per_seg=10):
    C = [np.asarray(c, float) for c in ctrl]
    C = [2 * C[0] - C[1], *C, 2 * C[-1] - C[-2]]
    out = []
    for i in range(1, len(C) - 2):
        p0, p1, p2, p3 = C[i - 1], C[i], C[i + 1], C[i + 2]
        for k in range(per_seg):
            t = k / per_seg
            out.append(
                0.5
                * (
                    2 * p1
                    + (-p0 + p2) * t
                    + (2 * p0 - 5 * p1 + 4 * p2 - p3) * t * t
                    + (-p0 + 3 * p1 - 3 * p2 + p3) * t**3
                )
            )
    out.append(C[-2])
    return np.array(out)


class Gear:
    """Head landmarks and BVHs of the rest-pose body, and the gear builders."""

    def __init__(self, rider, mats):
        self.rider = rider
        self.mesh, self.rig, self.mats = rider.mesh, rider.rig, mats
        self.E = np.array((rider.eyes_rest[0] + rider.eyes_rest[1]) / 2)
        co = mesh_co(self.mesh, world=False)
        body = group_mask(self.mesh, "body")
        polys = [
            tuple(p.vertices)
            for p in self.mesh.data.polygons
            if all(body[v] for v in p.vertices) and co[p.vertices[0], 2] > self.E[2] - 0.30
        ]
        self.head = BVHTree.FromPolygons([tuple(v) for v in co], polys)
        self.C = self.E + np.array((-0.075, 0.0, 0.020))  # the helmet's centre

    def snap(self, p, off):
        loc, nrm, _, _ = self.head.find_nearest(Vector(p))
        n = np.array(nrm)
        if np.dot(np.asarray(p) - np.array(loc), n) < -0.02:
            n = -n
        return np.array(loc) + n * off, n

    def skull(self, d):
        hit = self.head.ray_cast(Vector(self.C), Vector(d))
        return hit[3] if hit[0] is not None else 0.10

    # ----- helmet -----
    @staticmethod
    def rim_el(phi):
        c = math.cos(phi)
        return math.radians(5.0 + 14.0 * c - 2.0 * c * c)

    def helmet_dir(self, phi, i):
        e0 = self.rim_el(phi)
        el = e0 + (math.radians(86.0) - e0) * (i / (HELMET_ROWS - 1)) ** 0.9
        return np.array((math.cos(el) * math.cos(phi), math.cos(el) * math.sin(phi), math.sin(el)))

    @staticmethod
    def in_vent(d):
        for yc, x0, x1, w in HELMET_VENTS:
            for s in (1, -1) if yc else (1,):
                xm, hl = 0.5 * (x0 + x1), 0.5 * (x1 - x0)
                if ((d[1] - s * yc) / w) ** 2 + ((d[0] - xm) / hl) ** 8 < 1.0:
                    return True
        return False

    def build_helmet(self):
        A, R = HELMET_AZ, HELMET_ROWS
        D = np.array(
            [[self.helmet_dir(2 * math.pi * j / A, i) for j in range(A)] for i in range(R)]
        )
        r = np.array([[self.skull(D[i, j]) for j in range(A)] for i in range(R)])
        for _ in range(4):  # smooth out the hair and the ears' edges
            up = np.vstack([r[1:], r[-1:]])
            dn = np.vstack([r[:1], r[:-1]])
            r = 0.5 * r + 0.125 * (np.roll(r, 1, 1) + np.roll(r, -1, 1) + up + dn)
        tail = 0.012 * np.clip(-D[..., 0], 0, 1) ** 1.5 * np.clip(1 - np.abs(D[..., 2]), 0, 1)
        rout = r + HELMET_PAD + HELMET_SHELL + tail
        self.helmet_r = r
        V = (self.C + D * rout[..., None]).reshape(-1, 3)
        top = np.array([self.C + np.array((0.0, 0.0, 1.0)) * (self.skull((0, 0, 1)) + 0.032)])
        V = np.vstack([V, top])
        pole = len(V) - 1
        F, UV = [], []
        for i in range(R - 1):
            for j in range(A):
                jn = (j + 1) % A
                dc = unit(D[i, j] + D[i, (j + 1) % A] + D[i + 1, j] + D[i + 1, jn])
                if i >= 3 and self.in_vent(dc):
                    continue
                F.append((i * A + j, i * A + jn, (i + 1) * A + jn, (i + 1) * A + j))
                v0, v1 = i / (R - 1) * 0.98, (i + 1) / (R - 1) * 0.98
                UV.append(((j / A, v0), ((j + 1) / A, v0), ((j + 1) / A, v1), (j / A, v1)))
        for j in range(A):
            F.append(((R - 1) * A + j, (R - 1) * A + (j + 1) % A, pole))
            UV.append(((j / A, 0.98), ((j + 1) / A, 0.98), ((j + 0.5) / A, 1.0)))
        rows = np.repeat(np.arange(R), A)
        thick = np.append(np.clip(rows / 4.0, 0, 1), 1.0)
        ob = new_object(
            "Helmet",
            V,
            F,
            UV,
            [self.mats["M_Helmet"], self.mats["M_Strap"]],
            [0] * len(F),
            {"thick": thick, "head": 1.0},
        )
        # shell inward; the inner foam and the vents' walls get M_Strap's black
        solidify(ob, HELMET_SHELL, -1.0, 1, "thick", 0.45)
        ob.vertex_groups.remove(ob.vertex_groups["thick"])
        return ob

    def helmet_inner(self, phi, el):
        d = np.array((math.cos(el) * math.cos(phi), math.cos(el) * math.sin(phi), math.sin(el)))
        return self.C + d * (self.skull(d) + HELMET_PAD + 0.002)

    # ----- straps -----
    def build_straps(self):
        E = self.E
        V, F = [], []
        for side in (1, -1):
            front = self.helmet_inner(side * math.radians(70), self.rim_el(math.radians(70)) + 0.06)
            rear = self.helmet_inner(
                side * math.radians(120), self.rim_el(math.radians(120)) + 0.06
            )
            junction = E + np.array((-0.085, side * 0.064, -0.078))
            paths = [
                [front, E + np.array((-0.055, side * 0.074, -0.030)), junction],
                [rear, E + np.array((-0.128, side * 0.064, -0.035)), junction],
                [
                    junction,
                    E + np.array((-0.055, side * 0.050, -0.108)),
                    E + np.array((-0.026, side * 0.010, -0.126)),
                ],
            ]
            for ctrl in paths:
                P = catmull_rom(ctrl, 12)
                S = [self.snap(p, 0.0035) for p in P]
                verts, faces = tube_mesh(
                    [s[0] for s in S], [s[1] for s in S], STRAP_WIDTH, STRAP_THICK
                )
                F += [tuple(len(V) + k for k in f) for f in faces]
                V += list(verts)
        # the buckle under the chin
        c, _n = self.snap(E + np.array((-0.026, 0.0, -0.126)), 0.006)
        bv, bf = box_mesh(c, (0.022, 0.030, 0.008))
        F += [tuple(len(V) + k for k in f) for f in bf]
        V += list(bv)
        return new_object(
            "Straps", np.array(V), F, None, [self.mats["M_Strap"]], None, {"head": 1.0}
        )

    # ----- glasses -----
    def build_glasses(self):
        E = self.E
        cx = E[0] + 0.012 + LENS_GAP - LENS_R
        na, nz = 44, 9
        amax = math.radians(82.0)
        A = np.linspace(-amax, amax, na)
        V, notch = [], []
        for k in range(nz):
            for a in A:
                f = (abs(a) / amax) ** 2
                zb, zt = E[2] - 0.030 + 0.016 * f, E[2] + 0.021 - 0.002 * f
                z = zb + (zt - zb) * k / (nz - 1)
                d = np.array((math.cos(a), math.sin(a), 0.0))
                o = np.array((cx, 0.0, z))
                hit = self.head.ray_cast(Vector(o), Vector(d))
                rr, bad = LENS_R, False
                if hit[0] is not None and hit[3] > LENS_R - 0.005:
                    if abs(LENS_R * math.sin(a)) < 0.026 and z < E[2] + 0.006:
                        bad = True
                    else:
                        rr = hit[3] + 0.006
                V.append(o + d * rr)
                notch.append(bad)
        F = []
        for k in range(nz - 1):
            for j in range(na - 1):
                q = (k * na + j, k * na + j + 1, (k + 1) * na + j + 1, (k + 1) * na + j)
                # wound so the normal faces out, away from the face
                if not any(notch[v] for v in q):
                    F.append(q)
        lens = new_object("Lens", np.array(V), F, None, [self.mats["M_Lens"]], None, {"head": 1.0})
        solidify(lens, 0.0015, 1.0)
        # frame: a bar along the lens's top edge and the temple arms over the ears
        top = np.array(V[(nz - 1) * na : nz * na]) + np.array((0, 0, 0.002))
        out = top - np.array((cx, 0.0, 0.0))
        out[:, 2] = 0
        out /= np.linalg.norm(out, axis=1, keepdims=True)
        FV, FF = [], []
        verts, faces = tube_mesh(top, out, 0.005, 0.004)
        FF += faces
        FV += list(verts)
        for side in (1, -1):
            end = np.array(V[(nz - 1) * na + (na - 1 if side > 0 else 0)])
            ctrl = [
                end + np.array((0, 0, -0.006)),
                E + np.array((-0.075, side * 0.077, 0.012)),
                E + np.array((-0.108, side * 0.076, 0.002)),
            ]
            P = catmull_rom(ctrl, 10)
            S = [(P[0], np.array((0, side, 0.0)))] + [self.snap(p, 0.003) for p in P[1:]]
            verts, faces = tube_mesh([s[0] for s in S], [s[1] for s in S], 0.005, 0.0022)
            FF += [tuple(len(FV) + k for k in f) for f in faces]
            FV += list(verts)
        frame = new_object(
            "GlassesFrame", np.array(FV), FF, None, [self.mats["M_Strap"]], None, {"head": 1.0}
        )
        return [lens, frame]

    # ----- brows -----
    def build_brows(self):
        E = self.E
        V, F = [], []
        n = 9
        for side in (1, -1):
            P, N = [], []
            for k in range(n):
                t = k / (n - 1)
                y = side * (0.013 + 0.040 * t)
                z = E[2] + 0.027 + 0.006 * math.sin(math.pi * t) - 0.004 * t
                hit = self.head.ray_cast(Vector((E[0] + 0.2, y, z)), Vector((-1.0, 0.0, 0.0)))
                p = np.array(hit[0]) + np.array(hit[1]) * 0.0012
                P.append(p)
                N.append(np.array(hit[1]))
            base = len(V)
            for k in range(n):
                w = 0.0075 * (1 - 0.5 * k / (n - 1))
                up = np.array((0.0, 0.0, 1.0))
                up = unit(up - up.dot(N[k]) * N[k])
                V += [P[k] - up * w / 2, P[k] + up * w / 2]
            for k in range(n - 1):
                a = base + 2 * k
                q = (a, a + 2, a + 3, a + 1)
                F.append(q if side > 0 else q[::-1])
        return new_object("Brows", np.array(V), F, None, [self.mats["M_Hair"]], None, {"head": 1.0})

    # ----- shoes -----
    def shoe_masks(self):
        """Per body vertex: the foot share (foot + ball weights), the shoe region and the part
        hidden inside the shoe."""
        co = mesh_co(self.mesh, world=False)
        W = vertex_weights(self.mesh, [f"{b}_{s}" for s in ("l", "r") for b in ("foot", "ball")])
        foot = np.maximum(W[:, 0] + W[:, 1], W[:, 2] + W[:, 3])
        collar = np.zeros(len(co))
        for s, side in SIDES:
            ank = np.array(self.rider.head_rest[f"foot_{s}"])
            m = np.sign(co[:, 1]) == side
            collar[m] = np.interp(
                co[m, 0] - ank[0], [-0.09, -0.03, 0.02, 0.07], [0.078, 0.060, 0.066, 0.098]
            )
        shoe = (foot > 0.4) & (co[:, 2] < collar)
        hidden = shoe & (co[:, 2] < collar - 0.012)
        return shoe, hidden

    def build_shoes(self):
        shoe_v, _ = self.shoe_masks()
        ob = self.mesh.copy()
        ob.data = self.mesh.data.copy()
        ob.name = ob.data.name = "Shoes"
        bpy.context.scene.collection.objects.link(ob)
        ob.modifiers.clear()
        me = ob.data
        bm = bmesh.new()
        bm.from_mesh(me)
        bm.normal_update()
        orig_z = {v: v.co.z for v in bm.verts}
        bm.faces.ensure_lookup_table()
        kill = [f for f in bm.faces if not all(shoe_v[v.index] for v in f.verts)]
        bmesh.ops.delete(bm, geom=kill, context="FACES")
        loose = [v for v in bm.verts if not v.link_faces]
        bmesh.ops.delete(bm, geom=loose, context="VERTS")
        foot = BVHTree.FromBMesh(bm)
        # a shoe is a smooth last, not a sock: round off the toes' clefts, keeping the collar
        inner = [v for v in bm.verts if not v.is_boundary]
        for _ in range(30):
            bmesh.ops.smooth_vert(
                bm, verts=inner, factor=0.5, use_axis_x=True, use_axis_y=True, use_axis_z=True
            )
        bm.normal_update()
        for v in bm.verts:
            v.co += v.normal * SHOE_GAP
            # never inside the foot: at least the gap off its nearest surface
            p, n, _i, _d = foot.find_nearest(v.co)
            if p is not None and (v.co - p).dot(n) < SHOE_GAP:
                v.co = p + n * SHOE_GAP
            if orig_z[v] < 0.008:
                v.co.z = -SOLE_THICKNESS
            v.co.z = max(v.co.z, -SOLE_THICKNESS)
        bm.to_mesh(me)
        bm.free()
        me.materials.clear()
        me.materials.append(self.mats["M_Shoe"])
        me.polygons.foreach_set("material_index", np.zeros(len(me.polygons), np.int32))
        ob.parent = self.rig
        mod = ob.modifiers.new("Armature", "ARMATURE")
        mod.object = self.rig
        solidify(ob, 0.002, -1.0, even=False)  # the flattened sole has degenerate faces
        # the cleats, rigid to the foot (the solver's cleat point rides with the foot bone)
        for s, _side in SIDES:
            c = self.rider.cleat_rest[s]
            z0, z1 = -(SOLE_THICKNESS + CLEAT_THICKNESS), -SOLE_THICKNESS + 0.001
            bv, bf = box_mesh((c.x, c.y, 0.5 * (z0 + z1)), (0.064, 0.054, z1 - z0))
            n0 = len(ob.data.vertices)
            bm = bmesh.new()
            bm.from_mesh(ob.data)
            vs = [bm.verts.new(tuple(p)) for p in bv]
            for f in bf:
                bm.faces.new([vs[k] for k in f])
            bm.to_mesh(ob.data)
            bm.free()
            idx = list(range(n0, n0 + len(bv)))
            for g in ob.vertex_groups:
                g.remove(idx)
            ob.vertex_groups[f"foot_{s}"].add(idx, 1.0, "REPLACE")
        smart_uv(ob)
        return ob

    def remove_hidden_feet(self):
        _, hidden = self.shoe_masks()
        bm = bmesh.new()
        bm.from_mesh(self.mesh.data)
        bm.faces.ensure_lookup_table()
        kill = [f for f in bm.faces if all(hidden[v.index] for v in f.verts)]
        bmesh.ops.delete(bm, geom=kill, context="FACES")
        loose = [v for v in bm.verts if not v.link_faces]
        bmesh.ops.delete(bm, geom=loose, context="VERTS")
        bm.to_mesh(self.mesh.data)
        bm.free()
        log(f"removed {len(kill)} foot faces hidden by the shoes")


def box_mesh(centre, size):
    c = np.asarray(centre, float)
    h = np.asarray(size, float) / 2
    V = np.array([c + h * np.array((x, y, z)) for z in (-1, 1) for y in (-1, 1) for x in (-1, 1)])
    F = [(0, 2, 3, 1), (4, 5, 7, 6), (0, 1, 5, 4), (2, 6, 7, 3), (0, 4, 6, 2), (1, 3, 7, 5)]
    return V, F


def smart_uv(ob):
    bpy.ops.object.select_all(action="DESELECT")
    ob.select_set(True)
    bpy.context.view_layer.objects.active = ob
    bpy.ops.object.mode_set(mode="EDIT")
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.uv.smart_project(angle_limit=math.radians(60), island_margin=0.004)
    bpy.ops.object.mode_set(mode="OBJECT")


# =============================================================================================
# Textures: rasterised in UV space from rest-pose fields
# =============================================================================================
SKIN_L, JERSEY_L, ORANGE_L, SHORTS_L, SOCK_L, HAIR_L = range(6)
KIT_LABELS = (JERSEY_L, ORANGE_L, SHORTS_L, SOCK_L)
SLEEVE_END = 0.55  # of the upper arm, shoulder joint to elbow
CUFF = 0.08
SHORTS_END = 0.82  # of the thigh, hip joint to knee
SOCK_TOP = 0.78  # of the calf, knee to ankle
HEM_ABOVE_HIP = 0.05  # m, the jersey's hem above the hip joints (standing)


def tri_arrays(me):
    me.calc_loop_triangles()
    lt = me.loop_triangles
    n = len(lt)
    vi = np.empty(n * 3, np.int64)
    lt.foreach_get("vertices", vi)
    li = np.empty(n * 3, np.int64)
    lt.foreach_get("loops", li)
    pi = np.empty(n, np.int64)
    lt.foreach_get("polygon_index", pi)
    uv = np.empty(len(me.loops) * 2)
    me.uv_layers["UVMap"].data.foreach_get("uv", uv)
    uv = uv.reshape(-1, 2)[li].reshape(n, 3, 2)
    return vi.reshape(n, 3), uv, pi


def rasterize(uv, attrs, size):
    """Interpolate per-corner attributes (T, 3, C) over UV triangles (T, 3, 2) into a
    (size, size, C) image, row 0 at v = 1. Returns the image and its coverage."""
    C = attrs.shape[2]
    out = np.zeros((size, size, C), np.float32)
    cov = np.zeros((size, size), bool)
    px = uv[..., 0] * size - 0.5
    py = (1.0 - uv[..., 1]) * size - 0.5
    for t in range(len(uv)):
        x0, x1, x2 = px[t]
        y0, y1, y2 = py[t]
        d = (y1 - y2) * (x0 - x2) + (x2 - x1) * (y0 - y2)
        if abs(d) < 1e-9:
            continue
        xa = max(math.floor(min(x0, x1, x2)), 0)
        xb = min(math.ceil(max(x0, x1, x2)), size - 1)
        ya = max(math.floor(min(y0, y1, y2)), 0)
        yb = min(math.ceil(max(y0, y1, y2)), size - 1)
        if xb < xa or yb < ya:
            continue
        gy, gx = np.mgrid[ya : yb + 1, xa : xb + 1]
        l0 = ((y1 - y2) * (gx - x2) + (x2 - x1) * (gy - y2)) / d
        l1 = ((y2 - y0) * (gx - x2) + (x0 - x2) * (gy - y2)) / d
        l2 = 1.0 - l0 - l1
        e = -1e-4
        ins = (l0 >= e) & (l1 >= e) & (l2 >= e)
        if not ins.any():
            continue
        a = attrs[t]
        val = l0[ins, None] * a[0] + l1[ins, None] * a[1] + l2[ins, None] * a[2]
        out[gy[ins], gx[ins]] = val
        cov[gy[ins], gx[ins]] = True
    return out, cov


def dilate(img, cov, iters=6):
    """Spread the edge texels into the empty space so mips and seams don't bleed black."""
    img = img.copy()
    m = cov.copy()
    for _ in range(iters):
        acc = np.zeros_like(img)
        cnt = np.zeros(m.shape, np.float32)
        for dy, dx in ((0, 1), (0, -1), (1, 0), (-1, 0)):
            acc += np.roll(np.roll(img * m[..., None], dy, 0), dx, 1)
            cnt += np.roll(np.roll(m, dy, 0), dx, 1)
        new = (~m) & (cnt > 0)
        img[new] = acc[new] / cnt[new][:, None]
        m |= new
    return img


def to_srgb(lin):
    lin = np.clip(lin, 0.0, 1.0)
    return np.where(lin <= 0.0031308, 12.92 * lin, 1.055 * np.power(lin, 1 / 2.4) - 0.055)


def blur(h, n=1):
    for _ in range(n):
        h = (h + np.roll(h, 1, 0) + np.roll(h, -1, 0) + np.roll(h, 1, 1) + np.roll(h, -1, 1)) / 5
    return h


class BodyMap:
    """Where the kit, skin and hair are on the body, from rest-pose position and weights."""

    def __init__(self, rider):
        mesh, rig = rider.mesh, rider.rig
        self.bones = {
            b.name: (np.array(b.head_local), np.array(b.tail_local)) for b in rig.data.bones
        }
        names = [b.name for b in rig.data.bones]
        W = vertex_weights(mesh, names)
        col = {n: i for i, n in enumerate(names)}

        def share(parts):
            idx = [col[n] for n in names if n.rsplit("_", 2)[0] in parts]
            return W[:, idx].sum(1)

        self.arm = share(ARM_PARTS)
        self.leg = share(LEG_PARTS)
        self.head = W[:, [col["neck_01"], col["head"]]].sum(1)
        self.co = mesh_co(mesh, world=False)
        self.E = np.array((rider.eyes_rest[0] + rider.eyes_rest[1]) / 2)
        self.hip_z = self.bones["thigh_l"][0][2]
        self.armpit_z = self.bones["upperarm_l"][0][2] - 0.07
        # torso centre line per height
        tor = (self.arm < 0.5) & (self.leg < 0.5) & (self.head < 0.5) & group_mask(mesh, "body")
        z = self.co[tor, 2]
        bins = np.arange(z.min(), z.max() + 0.01, 0.01)
        xs = [self.co[tor][(z >= b) & (z < b + 0.01), 0] for b in bins]
        ok = np.array([len(x) > 0 for x in xs])
        # the middle of the front and back, smoothed: a vertex mean is pulled about by density
        cx = np.array([(x.min() + x.max()) / 2 for x, k in zip(xs, ok, strict=True) if k])
        cx = np.convolve(np.pad(cx, 3, mode="edge"), np.ones(7) / 7, mode="valid")
        self.cz, self.cx = bins[ok] + 0.005, cx

    def t_along(self, p, bone):
        """Parameter along the bone (0 head, 1 tail), the side picked by the sign of y."""
        out = np.zeros(len(p))
        for s, side in SIDES:
            h, t = self.bones[f"{bone}_{s}"]
            m = np.sign(p[:, 1]) == side
            ax = t - h
            out[m] = (p[m] - h) @ ax / ax.dot(ax)
        return out

    def labels(self, p, arm, leg, head):
        lab = np.full(len(p), SKIN_L)
        x, y, z = p[:, 0], p[:, 1], p[:, 2]
        neck = self.bones["neck_01"][0]
        below_collar = z < neck[2] + 0.015 - 0.4 * (x - neck[0])
        is_arm, is_leg = arm > 0.5, leg > 0.5
        torso = ~is_arm & ~is_leg
        hem = self.hip_z + HEM_ABOVE_HIP
        tua = self.t_along(p, "upperarm")
        tth = self.t_along(p, "thigh")
        tca = self.t_along(p, "calf")
        jersey = (torso & (z > hem) & below_collar) | (is_arm & (tua < SLEEVE_END))
        lab[jersey] = JERSEY_L
        cuff = is_arm & (tua > SLEEVE_END - CUFF) & (tua < SLEEVE_END)
        cxz = np.interp(z, self.cz, self.cx)
        ang = np.abs(np.arctan2(np.abs(y), x - cxz) - math.pi / 2)
        panel = jersey & torso & (ang < math.radians(15)) & (z > hem + 0.02) & (z < self.armpit_z)
        lab[cuff | panel] = ORANGE_L
        shorts = (torso & (z <= hem)) | (is_leg & (tth < SHORTS_END) & (z > 0.2))
        lab[shorts] = SHORTS_L
        lab[is_leg & (tca > SOCK_TOP)] = SOCK_L
        # short hair under the helmet
        r = p - self.E
        zline = np.interp(
            r[:, 0],
            [-0.20, -0.15, -0.11, -0.07, -0.04, 0.0],
            [-0.085, -0.075, -0.03, -0.01, 0.045, 0.055],
        )
        ear = (np.abs(r[:, 1]) > 0.074) & (r[:, 2] < 0.03)
        lab[(head > 0.5) & (r[:, 2] > zline) & ~ear & ~below_collar] = HAIR_L
        return lab

    def vertex_labels(self):
        return self.labels(self.co, self.arm, self.leg, self.head)


def paint_body(rider, out, size):
    bmap = BodyMap(rider)
    me = rider.mesh.data
    vi, uv, _ = tri_arrays(me)
    body = group_mask(rider.mesh, "body")
    keep = body[vi].all(1)  # the helper eyes and lashes overlap the body's UVs
    vi, uv = vi[keep], uv[keep]
    attrs = np.concatenate(
        [bmap.co[vi], bmap.arm[vi][..., None], bmap.leg[vi][..., None], bmap.head[vi][..., None]],
        -1,
    )
    img, cov = rasterize(uv, attrs.astype(np.float32), size)
    p = img[cov][:, :3]
    lab = bmap.labels(p, img[cov][:, 3], img[cov][:, 4], img[cov][:, 5])
    rng = np.random.default_rng(3)
    lo = blur(rng.normal(0, 1, (size, size)), 6) * 6  # low-frequency mottling
    fine = rng.normal(0, 1, (size, size))
    colours = {
        SKIN_L: SKIN,
        JERSEY_L: NAVY,
        ORANGE_L: ORANGE,
        SHORTS_L: BLACK,
        SOCK_L: WHITE,
        HAIR_L: (0.030, 0.019, 0.012),
    }
    base = np.zeros((size, size, 3), np.float32)
    lab_img = np.full((size, size), -1)
    lab_img[cov] = lab
    for k, c in colours.items():
        m = lab_img == k
        base[m] = c
    skin = lab_img == SKIN_L
    base[skin] *= (1 + 0.015 * lo[skin] + 0.004 * fine[skin])[:, None]
    hair = lab_img == HAIR_L
    base[hair] = (
        base[hair] * 0.8 + 0.2 * np.array(SKIN) * (0.5 + 0.5 * np.clip(fine[hair], 0, 1))[:, None]
    )
    kit = np.isin(lab_img, KIT_LABELS)
    base[kit] *= (1 + 0.01 * lo[kit] + 0.004 * fine[kit])[:, None]
    base = dilate(base, cov, 8)
    # normals: a fine knit on the kit, pores on the skin, streaks in the hair
    Y, X = np.mgrid[0:size, 0:size].astype(np.float32)
    knit = 0.5 * np.abs(np.sin(np.pi * X / 3.0 + 0.6 * np.sin(np.pi * Y / 3.0))) + 0.15 * fine
    pores = blur(fine, 1)
    streak = blur(np.roll(fine, 1, 0) + fine + np.roll(fine, -1, 0), 1)
    h = np.where(kit, 0.15 * knit, np.where(hair, 0.4 * streak, 0.05 * pores))
    nrm = normal_from_height(h, 1.0)
    nrm[~cov] = (0.5, 0.5, 1.0)
    nrm = dilate(nrm, cov, 8)
    save_png(os.path.join(out, "T_Body_BaseColor.png"), to_srgb(base), "sRGB")
    save_png(os.path.join(out, "T_Body_Normal.png"), nrm, "Non-Color")
    return bmap


def paint_helmet(helmet, C, out, size):
    me = helmet.data
    vi, uv, pi = tri_arrays(me)
    mi = np.empty(len(me.polygons), np.int64)
    me.polygons.foreach_get("material_index", mi)
    keep = mi[pi] == 0
    co = mesh_co(helmet, world=False)
    d = co - np.asarray(C)
    d /= np.linalg.norm(d, axis=1, keepdims=True)
    img, cov = rasterize(uv[keep], d[vi[keep]].astype(np.float32), size)
    D = img[cov]
    base = np.zeros((size, size, 3), np.float32)
    col = np.tile(np.array(NAVY, np.float32), (len(D), 1))
    ay = np.abs(D[:, 1])
    stripe = (ay > 0.315) & (ay < 0.355) & (D[:, 2] > 0.25)
    tailband = (D[:, 0] < -0.80) & (D[:, 2] > -0.05) & (D[:, 2] < 0.20)
    col[stripe | tailband] = ORANGE
    el = np.degrees(np.arcsin(np.clip(D[:, 2], -1, 1)))
    c = np.cos(np.arctan2(D[:, 1], D[:, 0]))
    rim = 5.0 + 14.0 * c - 2.0 * c * c  # Gear.rim_el in degrees
    col[el < rim + 3.0] = BLACK
    base[cov] = col
    base = dilate(base, cov, 8)
    h = np.zeros((size, size), np.float32)
    hv = np.zeros(len(D), np.float32)
    hv[stripe | tailband] = 0.3  # the accent is a raised decal
    h[cov] = hv
    nrm = normal_from_height(blur(h, 1), 2.0)
    nrm[~cov] = (0.5, 0.5, 1.0)
    save_png(os.path.join(out, "T_Helmet_BaseColor.png"), to_srgb(base), "sRGB")
    save_png(os.path.join(out, "T_Helmet_Normal.png"), dilate(nrm, cov, 8), "Non-Color")


def paint_shoes(shoes, rider, out, size):
    me = shoes.data
    vi, uv, _ = tri_arrays(me)
    co = mesh_co(shoes, world=False)
    img, cov = rasterize(uv, co[vi].astype(np.float32), size)
    p = img[cov]
    ank = np.zeros((len(p), 3))
    for s, side in SIDES:
        ank[np.sign(p[:, 1]) == side] = np.array(rider.head_rest[f"foot_{s}"])
    rx, z = p[:, 0] - ank[:, 0], p[:, 2]
    col = np.tile(np.array(WHITE, np.float32), (len(p), 1))
    black = (
        (z < 0.009) | ((rx < -0.035) & (z < 0.058)) | ((np.abs(rx - 0.062) < 0.011) & (z > 0.032))
    )
    col[black] = BLACK
    col[z < -SOLE_THICKNESS - 0.0005] = (0.03, 0.03, 0.032)  # the cleat
    base = np.zeros((size, size, 3), np.float32)
    base[cov] = col
    # perforations on the toe box
    Y, X = np.mgrid[0:size, 0:size]
    holes = ((X % 7 < 2) & (Y % 7 < 2)).astype(np.float32)
    toe = np.zeros((size, size), bool)
    toe[cov] = (rx > 0.12) & (z > 0.02) & ~black
    h = np.where(toe, -0.5 * holes, 0.0)
    base[toe & (holes > 0)] *= 0.6
    nrm = normal_from_height(h, 1.5)
    nrm[~cov] = (0.5, 0.5, 1.0)
    save_png(os.path.join(out, "T_Shoe_BaseColor.png"), to_srgb(dilate(base, cov, 8)), "sRGB")
    save_png(os.path.join(out, "T_Shoe_Normal.png"), dilate(nrm, cov, 8), "Non-Color")


# =============================================================================================
# Materials
# =============================================================================================
# name: (base colour or texture prefix, roughness, extra Principled inputs)
MATERIALS = {
    # matte throughout: a simple, flat-shaded look that sits in the world
    "M_Skin": ("T_Body", 0.70, {"Subsurface Weight": 0.05}),
    "M_Kit": ("T_Body", 0.85, {}),
    "M_Helmet": ("T_Helmet", 0.55, {}),
    "M_Lens": ((0.010, 0.011, 0.014), 0.15, {"Metallic": 0.3}),
    "M_Shoe": ("T_Shoe", 0.60, {}),
    "M_Strap": (BLACK, 0.80, {}),
    "M_Hair": ((0.030, 0.019, 0.012), 0.80, {}),
}


def make_materials(out):
    mats = {}
    for name, (base, rough, extra) in MATERIALS.items():
        m = bpy.data.materials.get(name) or bpy.data.materials.new(name)
        nt = m.node_tree
        bsdf = nt.nodes.get("Principled BSDF")
        bsdf.inputs["Roughness"].default_value = rough
        for k, v in extra.items():
            if k in bsdf.inputs:
                bsdf.inputs[k].default_value = v
        if isinstance(base, str):
            m["textures"] = base
        else:
            bsdf.inputs["Base Color"].default_value = (*base, 1.0)
        mats[name] = m
    return mats


def hook_textures(mats, out):
    """Wire the saved PNGs into the textured materials (after painting)."""
    for name, m in mats.items():
        prefix = m.get("textures")
        if not prefix:
            continue
        nt = m.node_tree
        bsdf = nt.nodes.get("Principled BSDF")
        bc = nt.nodes.new("ShaderNodeTexImage")
        bc.image = bpy.data.images.load(
            os.path.join(out, f"{prefix}_BaseColor.png"), check_existing=True
        )
        nt.links.new(bc.outputs["Color"], bsdf.inputs["Base Color"])
        nm = nt.nodes.new("ShaderNodeTexImage")
        nm.image = bpy.data.images.load(
            os.path.join(out, f"{prefix}_Normal.png"), check_existing=True
        )
        nm.image.colorspace_settings.name = "Non-Color"
        nmap = nt.nodes.new("ShaderNodeNormalMap")
        nt.links.new(nm.outputs["Color"], nmap.inputs["Color"])
        nt.links.new(nmap.outputs["Normal"], bsdf.inputs["Normal"])
        if name == "M_Skin":
            bsdf.inputs["Subsurface Radius"].default_value = (0.010, 0.004, 0.002)
            bsdf.inputs["Subsurface Scale"].default_value = 0.01


def assign_body_materials(rider, bmap, mats):
    me = rider.mesh.data
    me.materials.clear()
    for n in ("M_Skin", "M_Kit", "M_Lens", "M_Hair"):
        me.materials.append(mats[n])
    lab = bmap.vertex_labels()
    kit = np.isin(lab, KIT_LABELS)
    eye = group_mask(rider.mesh, "helper-l-eye") | group_mask(rider.mesh, "helper-r-eye")
    lash = np.zeros(len(me.vertices), bool)
    for s in ("l", "r"):
        for g in ("eyelashes-1", "eyelashes-2"):
            lash |= group_mask(rider.mesh, f"helper-{s}-{g}")
    idx = np.zeros(len(me.polygons), np.int32)
    for p in me.polygons:
        v = list(p.vertices)
        if all(eye[v]):
            idx[p.index] = 2
        elif all(lash[v]):
            idx[p.index] = 3
        elif kit[v].mean() >= 0.5:
            idx[p.index] = 1
    me.polygons.foreach_set("material_index", idx)


def join_rider(rider, parts):
    body = rider.mesh
    for ob in parts:
        ob.parent = rider.rig
    with bpy.context.temp_override(
        active_object=body, object=body, selected_editable_objects=[body, *parts]
    ):
        bpy.ops.object.join()
    body.name = body.data.name = "Rider"
    bones = {b.name for b in rider.rig.data.bones}
    for g in list(body.vertex_groups):
        if g.name not in bones:
            body.vertex_groups.remove(g)
    limit_influences(body, 4)
    return body


def limit_influences(ob, limit):
    """Keep each vertex's strongest `limit` bone weights and renormalise them."""
    groups = ob.vertex_groups
    for v in ob.data.vertices:
        ws = sorted(((g.weight, g.group) for g in v.groups), reverse=True)
        for _, gi in ws[limit:]:
            groups[gi].remove([v.index])
        kept = [(w, gi) for w, gi in ws[:limit] if w > 0]
        total = sum(w for w, _ in kept)
        if total > 0:
            for w, gi in kept:
                groups[gi].add([v.index], w / total, "REPLACE")


# =============================================================================================
# Animation
# =============================================================================================
def record_pedal(rider, P):
    """Pose every frame of an effort through the IK and record the bones (armature space)."""
    P = dict(P, rock=EFFORTS[P["effort"]][1])
    rec = []
    for k in range(FRAMES):
        rider.pose(k, P)
        rec.append({pb.name: pb.matrix.copy() for pb in rider.rig.pose.bones})
    return rec


def bake_efforts(rider, fits):
    """Record every effort and the Cockpit action, drop the IK, and key one cyclic action each,
    ending with the default effort's action assigned. Returns {action: recorded frames}."""
    rig = rider.rig
    recs = {name: record_pedal(rider, fits[name]) for name in fits}
    for pb in rig.pose.bones:
        for c in list(pb.constraints):
            pb.constraints.remove(c)
        pb.lock_ik_y = pb.lock_ik_z = False
    rider.dispose()
    bpy.context.preferences.edit.keyframe_new_interpolation_type = "LINEAR"
    rig.animation_data_create()
    for name in (*[n for n in fits if n != DEFAULT_EFFORT], DEFAULT_EFFORT):
        key_action(rig, name, recs[name])
    sc = bpy.context.scene
    sc.frame_start, sc.frame_end = 0, FRAMES - 1
    sc.render.fps = FPS
    sc.frame_set(0)
    return recs


def local_poses(rig, rec):
    """Each frame's bone transforms relative to their parents' (the pose basis)."""
    rest = {b.name: b.matrix_local for b in rig.data.bones}
    par = {b.name: (b.parent.name if b.parent else None) for b in rig.data.bones}
    out = []
    for M in rec:
        f = {}
        for n in M:
            p = par[n]
            if p:
                f[n] = (rest[p].inverted() @ rest[n]).inverted() @ M[p].inverted() @ M[n]
            else:
                f[n] = rest[n].inverted() @ M[n]
        out.append(f)
    return out


def key_action(rig, name, rec):
    action = bpy.data.actions.new(name)
    use_action(rig, action)
    prev = {}
    for k, L in enumerate(local_poses(rig, rec)):
        for pb in rig.pose.bones:
            n = pb.name
            loc, q, _ = L[n].decompose()
            if n in prev and q.dot(prev[n]) < 0:
                q.negate()
            prev[n] = q
            pb.location = loc
            pb.rotation_quaternion = q
            pb.scale = (1.0, 1.0, 1.0)
            pb.keyframe_insert("location", frame=k, group=n)
            pb.keyframe_insert("rotation_quaternion", frame=k, group=n)
    action.use_frame_range = True
    action.frame_start, action.frame_end = 0, FRAMES - 1
    action.use_cyclic = True
    action.use_fake_user = True  # kept in the .blend while another action is assigned
    for fc in action_fcurves(action):
        for kp in fc.keyframe_points:
            kp.interpolation = "LINEAR"
    return action


def use_action(rig, action):
    ad = rig.animation_data or rig.animation_data_create()
    ad.action = action
    if hasattr(ad, "action_slot") and len(getattr(action, "slots", [])):
        ad.action_slot = action.slots[0]
    bpy.context.view_layer.update()


def blend_drift(rig, rec_a, rec_b, t=0.5):
    """How far the hands and feet move off the hoods and pedals when a game blends two efforts'
    local bone transforms (as an animation blend does) at weight t: the largest distance over
    the loop from where the first effort holds them (m)."""
    par = {b.name: (b.parent.name if b.parent else None) for b in rig.data.bones}
    rest = {b.name: b.matrix_local for b in rig.data.bones}
    order = [b.name for b in rig.data.bones]  # parents come first
    ends = [f"{e}_{s}" for e in ("hand", "foot") for s, _ in SIDES]
    worst = 0.0
    for ra, la, lb in zip(rec_a, local_poses(rig, rec_a), local_poses(rig, rec_b), strict=True):
        M = {}
        for n in order:
            la_l, la_q, _ = la[n].decompose()
            lb_l, lb_q, _ = lb[n].decompose()
            if la_q.dot(lb_q) < 0:
                lb_q.negate()
            B = Matrix.Translation(la_l.lerp(lb_l, t)) @ la_q.slerp(lb_q, t).to_matrix().to_4x4()
            p = par[n]
            M[n] = (M[p] @ rest[p].inverted() @ rest[n] @ B) if p else rest[n] @ B
        for e in ends:
            worst = max(worst, (M[e].translation - ra[e].translation).length)
    return worst


def action_fcurves(action):
    if hasattr(action, "layers"):
        out = []
        for layer in action.layers:
            for strip in layer.strips:
                for cb in strip.channelbags:
                    out.extend(cb.fcurves)
        return out
    return list(action.fcurves)


# =============================================================================================
# Checks
# =============================================================================================
def seam_check(rec):
    """The pose's step from frame 59 back to 0, against the largest step between frames."""

    def turn(qa, qb):
        ang = qa.rotation_difference(qb).angle
        return math.degrees(min(ang, 2 * math.pi - ang))  # q and -q are the same rotation

    def step(a, b):
        dl = max((a[n].translation - b[n].translation).length for n in a)
        da = max(turn(a[n].to_quaternion(), b[n].to_quaternion()) for n in a)
        return dl, da

    steps = [step(rec[k], rec[k + 1]) for k in range(FRAMES - 1)]
    seam = step(rec[-1], rec[0])
    return {
        "max_step_mm": round(1000 * max(s[0] for s in steps), 2),
        "max_step_deg": round(max(s[1] for s in steps), 2),
        "seam_mm": round(1000 * seam[0], 2),
        "seam_deg": round(seam[1], 2),
    }


def inside(tree, p):
    """Whether a local point is inside a mesh: an odd number of crossings along each of three
    rays. The nearest face's normal alone misreads open air under an overhang, such as a
    fingertip in the hollow between a hood and its lever blade."""
    odd = 0
    for d in (Vector((1, 0, 0)), Vector((0, 1, 0)), Vector((0, 0, -1))):
        o, n = Vector(p), 0
        while n < 64:
            hit = tree.ray_cast(o, d, 2.0)
            if hit[0] is None:
                break
            n += 1
            o = hit[0] + d * 1e-5
        odd += n % 2
    return odd >= 2


def penetration(rider_ob, bike, bike_objs, slot_of_vertex):
    """Rider vertices more than 5 mm inside a bike part, over all frames."""
    names = [
        n
        for n in ("Frame", "Fork", "WheelFront", "WheelRear", "Cranks", "PedalL", "PedalR", "Chain")
        if n in bike_objs
    ]
    worst = {}
    box = {}
    for n in names:
        v = np.array([x.co for x in bike_objs[n].data.vertices])
        box[n] = (v.min(0) - 0.01, v.max(0) + 0.01)
    sc = bpy.context.scene
    for k in range(FRAMES):
        sc.frame_set(k)
        pose_bike(bike_objs, k)
        co = mesh_co(rider_ob, evaluated=True)
        for n in names:
            ob = bike_objs[n]
            inv = np.array(ob.matrix_world.inverted())
            loc = co @ inv[:3, :3].T + inv[:3, 3]
            lo, hi = box[n]
            sel = np.where(np.all((loc > lo) & (loc < hi), 1))[0]
            tree = bike.tree(n)
            for i in sel:
                hit = tree.find_nearest(Vector(loc[i]), 0.06)
                if hit[0] is None:
                    continue
                dvec = Vector(loc[i]) - hit[0]
                if dvec.dot(hit[1]) < 0 and hit[3] > 0.005 and inside(tree, loc[i]):
                    key = (n, slot_of_vertex[i])
                    w = worst.get(key, {"verts": set(), "max_mm": 0.0, "frames": set()})
                    w["verts"].add(int(i))
                    w["frames"].add(k)
                    w["max_mm"] = max(w["max_mm"], 1000 * hit[3])
                    worst[key] = w
    pose_bike(bike_objs, 0)
    sc.frame_set(0)
    return [
        {
            "bike_part": n,
            "rider_slot": s,
            "vertices": len(w["verts"]),
            "frames": len(w["frames"]),
            "max_depth_mm": round(w["max_mm"], 1),
        }
        for (n, s), w in sorted(worst.items())
    ]


def collect_stats(rider, fits, rider_ob, bike, bike_objs, recs):
    me = rider_ob.data
    mi = np.empty(len(me.polygons), np.int64)
    me.polygons.foreach_get("material_index", mi)
    lt = np.empty(len(me.polygons), np.int64)
    me.polygons.foreach_get("loop_total", lt)
    tris = {}
    for i, m in enumerate(me.materials):
        tris[m.name] = tris.get(m.name, 0) + int((lt[mi == i] - 2).sum())
    # vertex -> slot name for the penetration report
    slot = np.empty(len(me.vertices), object)
    for p in me.polygons:
        for v in p.vertices:
            slot[v] = me.materials[p.material_index].name
    efforts, frame0 = {}, {}
    for name in fits:
        use_action(rider.rig, bpy.data.actions[name])
        efforts[name] = effort_stats(rider, fits[name], rider_ob, bike, bike_objs, recs[name], slot)
        frame0[name] = frame0_stats(rider, bike)
    efforts[COCKPIT_ACTION]["cockpit_view"] = fits[COCKPIT_ACTION]["view"]
    use_action(rider.rig, bpy.data.actions[DEFAULT_EFFORT])
    names = list(EFFORTS)
    drift = {
        f"{a}/{b}": round(1000 * blend_drift(rider.rig, recs[a], recs[b]), 1)
        for a, b in itertools.pairwise(names)
    }
    leg = rider.rig.data.bones
    P = fits[DEFAULT_EFFORT]
    return {
        "triangles": {"per_slot": tris, "total": sum(tris.values())},
        "bones": len(rider.rig.data.bones),
        "max_influences": max(len(v.groups) for v in me.vertices),
        "frames": FRAMES,
        "fps": FPS,
        "default_action": DEFAULT_EFFORT,
        "eye_point_game_assumes": [0.18, None, 1.50],
        "body": {
            "leg_target_weight": round(P["props"]["leg"], 3),
            "hip_joint_x": round(P["hip"][0], 4),
            "thigh_m": round((leg["thigh_l"].tail_local - leg["thigh_l"].head_local).length, 4),
            "calf_m": round((leg["calf_l"].tail_local - leg["calf_l"].head_local).length, 4),
            "palm_points": {s: [round(c, 4) for c in rider.hood[s]] for s, _ in SIDES},
        },
        # per action at frame 0, for the game to check its import against (m; "l" is the
        # non-drive side, +Y)
        "frame0": frame0,
        "cameras": {
            "hfov_deg": GAME_HFOV,
            "aspect": "16:9",
            "cockpit": {"location": list(COCKPIT_CAMERA), "pitch_down_deg": COCKPIT_PITCH},
            "chase": {
                "location": [round(c, 4) for c in chase_camera_location()],
                "target": list(CHASE_TARGET),
                "pitch_down_deg": CHASE_PITCH,
            },
        },
        "efforts": efforts,
        # a halfway blend of neighbouring efforts' local transforms: the hands' and feet's
        # largest move off the hoods and pedals, in mm
        "blend_drift_mm": drift,
    }


def effort_stats(rider, P, rider_ob, bike, bike_objs, rec, slot):
    """The checks for the action assigned now."""
    sc = bpy.context.scene
    cleat, hood, knee = 0.0, 0.0, {"l": [], "r": []}
    for k in range(FRAMES):
        sc.frame_set(k)
        for s, side in SIDES:
            cleat = max(cleat, (rider.cleat_world(s) - Vector(pedal_axle(k, side))).length)
            hood = max(hood, (rider.palm_world(s) - Vector(rider.hood[s])).length)
            knee[s].append(rider.knee_angle(s))
    sc.frame_set(0)
    eye = rider.eye_point()
    out = {
        "back_deg_hip_to_neck_base": round(rider.back_angle(), 1),
        "elbow_bend_deg": round(0.5 * (rider.elbow_bend("l") + rider.elbow_bend("r")), 1),
        "torso_pitch_deg": round(P["torso"], 2),
        "clavicle_protraction_deg": round(P["clav"], 2),
        "hip_joint_z": round(P["hip"][1], 4),
        "hip_rock_roll_deg": round(HIP_ROCK_ROLL * EFFORTS[P["effort"]][1], 2),
        "max_cleat_to_pedal_axle_mm": round(1000 * cleat, 2),
        "knee_deg": {
            ("drive_r" if s == "r" else "non_drive_l"): {
                "min": round(min(v), 1),
                "max": round(max(v), 1),
            }
            for s, v in knee.items()
        },
        "max_hand_to_hood_mm": round(1000 * hood, 2),
        "interpenetration_over_5mm": penetration(rider_ob, bike, bike_objs, slot),
        "eye_point_frame0": [round(c, 4) for c in eye],
        "loop_seam": seam_check(rec),
    }
    log(f"{P.get('action', P['effort'])}: " + ", ".join(f"{k} {v}" for k, v in out.items()))
    return out


def frame0_stats(rider, bike):
    """The assigned action at frame 0: the back, the eye point, the palm points, the head
    bone's head, and the hood points under the palms (the hood's top at the palm's x, across
    the hood's middle)."""
    bpy.context.scene.frame_set(0)
    hood = {}
    for s, side in SIDES:
        z, y = bike.hood_top(side, HOOD_CONTACT_X)
        hood[s] = [round(HOOD_CONTACT_X, 4), round(float(y), 4), round(float(z), 4)]
    return {
        "back_deg_hip_to_neck_base": round(rider.back_angle(), 2),
        "eye_point": [round(c, 4) for c in rider.eye_point()],
        "palm_points": {s: [round(c, 4) for c in rider.palm_world(s)] for s, _ in SIDES},
        "head_bone": [round(c, 4) for c in rider.rig.pose.bones["head"].head],
        "hood_contact": hood,
    }


# =============================================================================================
# Export and renders
# =============================================================================================
def export_fbx(rig, rider_ob, path):
    bpy.ops.object.select_all(action="DESELECT")
    rig.select_set(True)
    rider_ob.select_set(True)
    bpy.context.view_layer.objects.active = rig
    bpy.ops.export_scene.fbx(
        filepath=path,
        use_selection=True,
        object_types={"ARMATURE", "MESH"},
        apply_unit_scale=True,
        apply_scale_options="FBX_SCALE_ALL",
        bake_space_transform=False,
        axis_forward="Y",
        axis_up="Z",
        mesh_smooth_type="FACE",
        use_tspace=True,
        use_mesh_modifiers=False,
        add_leaf_bones=False,
        use_armature_deform_only=True,
        armature_nodetype="NULL",
        bake_anim=True,
        bake_anim_use_all_bones=True,
        bake_anim_use_nla_strips=False,
        bake_anim_use_all_actions=True,  # one take per effort, "<armature>|<action>"
        bake_anim_force_startend_keying=True,
        bake_anim_step=1.0,
        bake_anim_simplify_factor=0.0,
        path_mode="RELATIVE",
        embed_textures=False,
        use_custom_props=False,
    )


def check_fbx(path):
    """Re-import the FBX into an empty scene and report what came back."""
    bpy.ops.wm.read_homefile(use_empty=True)
    bpy.ops.import_scene.fbx(filepath=path, anim_offset=0.0)
    arms = [o for o in bpy.data.objects if o.type == "ARMATURE"]
    meshes = [o for o in bpy.data.objects if o.type == "MESH"]
    out = {
        "objects": sorted(o.name for o in bpy.data.objects),
        "bones": len(arms[0].data.bones) if arms else 0,
        "actions": [a.name for a in bpy.data.actions],
        "frame_range": [list(a.frame_range) for a in bpy.data.actions],  # anim_offset=0 below
        "materials": sorted({m.name for o in meshes for m in o.data.materials}),
        "max_influences": max((len(v.groups) for o in meshes for v in o.data.vertices), default=0),
        "triangles": sum(sum(len(p.vertices) - 2 for p in o.data.polygons) for o in meshes),
        "armature_scale": [round(c, 4) for c in arms[0].scale] if arms else None,
    }
    log("fbx check", out)
    return out


def bike_mod():
    path = os.path.join(HERE, "..", "bike")
    if path not in sys.path:
        sys.path.insert(0, path)
    import build_bike  # ty: ignore[unresolved-import]

    return build_bike


def save_png(path, rgb, colorspace="sRGB"):
    return bike_mod().save_png(path, rgb.astype(np.float32), colorspace)


def normal_from_height(h, strength):
    return bike_mod().normal_from_height(h, strength)


def aim(cam, loc, target, lens):
    cam.location = loc
    cam.rotation_euler = (Vector(target) - Vector(loc)).to_track_quat("-Z", "Y").to_euler()
    cam.data.type = "PERSP"
    cam.data.sensor_fit = "AUTO"
    cam.data.lens = lens


def render_all(out, quick, bike_objs, rig):
    sc = bpy.context.scene
    cam, _ = bike_mod().setup_render_scene(quick)
    sc.frame_set(0)
    pose_bike(bike_objs, 0)
    views = {
        "render_chase": ((-3.0, -0.35, 1.40), (0.35, 0.0, 0.85), 40),
        "render_side_drive": ((0.10, -4.4, 0.95), (0.10, 0.0, 0.82), 50),
        "render_side_non_drive": ((0.10, 4.4, 0.95), (0.10, 0.0, 0.82), 50),
        "render_front": ((4.6, -0.25, 1.00), (0.0, 0.0, 0.88), 45),
    }
    for name, (loc, tgt, lens) in views.items():
        aim(cam, loc, tgt, lens)
        sc.render.filepath = os.path.join(out, name + ".png")
        t0 = time.time()
        bpy.ops.render.render(write_still=True)
        log(f"{name}: {time.time() - t0:.0f}s")
    # the game's views (16:9) and the hands on the hoods from above and behind
    res = sc.render.resolution_x, sc.render.resolution_y
    sc.render.resolution_x, sc.render.resolution_y = 1280, 720
    clip = cam.data.clip_start
    cam.data.clip_start = 0.02
    shots = (
        (
            "render_game_chase",
            DEFAULT_EFFORT,
            lambda: game_camera(cam, chase_camera_location(), CHASE_PITCH),
        ),
        (
            "render_game_cockpit",
            COCKPIT_ACTION,
            lambda: game_camera(cam, COCKPIT_CAMERA, COCKPIT_PITCH),
        ),
        (
            "render_hands_top",
            DEFAULT_EFFORT,
            lambda: aim(cam, (0.26, 0.0, 1.28), (0.56, 0.0, 0.90), 32),
        ),
    )
    for name, action, place in shots:
        use_action(rig, bpy.data.actions[action])
        # the game hides the head in the cockpit view: shrink it to a point (unkeyed channel)
        head = rig.pose.bones["head"]
        head.scale = (1e-3,) * 3 if action == COCKPIT_ACTION else (1.0,) * 3
        sc.frame_set(0)
        place()
        sc.render.filepath = os.path.join(out, name + ".png")
        t0 = time.time()
        bpy.ops.render.render(write_still=True)
        log(f"{name}: {time.time() - t0:.0f}s")
    rig.pose.bones["head"].scale = (1.0, 1.0, 1.0)
    use_action(rig, bpy.data.actions[DEFAULT_EFFORT])
    sc.frame_set(0)
    cam.data.clip_start = clip
    sc.render.resolution_x, sc.render.resolution_y = res
    # contact sheet: drive side, legs and drivetrain, labelled with the crank angle
    sc.render.resolution_x, sc.render.resolution_y = 800, 520
    sc.render.resolution_percentage = 50 if quick else 100
    sc.cycles.samples = 24 if quick else 64
    aim(cam, (0.0, -2.5, 0.62), (0.0, 0.0, 0.58), 50)
    tcu = bpy.data.curves.new("_label", "FONT")
    tob = bpy.data.objects.new("_label", tcu)
    sc.collection.objects.link(tob)
    tcu.size = 0.07
    tob.location = (-0.72, -0.35, 0.98)
    tob.rotation_euler = (math.pi / 2, 0.0, 0.0)
    lm = bpy.data.materials.new("_label")
    lm.node_tree.nodes["Principled BSDF"].inputs["Emission Color"].default_value = (1, 1, 1, 1)
    lm.node_tree.nodes["Principled BSDF"].inputs["Emission Strength"].default_value = 2.0
    tcu.materials.append(lm)
    tiles = []
    for k in (0, 8, 15, 23, 30, 38, 45, 53):
        sc.frame_set(k)
        pose_bike(bike_objs, k)
        tcu.body = f"frame {k}   crank {360.0 * k / FRAMES:.0f} deg"
        path = os.path.join(out, "debug", f"contact_{k:02d}.png")
        sc.render.filepath = path
        bpy.ops.render.render(write_still=True)
        img = bpy.data.images.load(path)
        w, h = img.size
        px = np.array(img.pixels[:], np.float32).reshape(h, w, 4)[::-1, :, :3]
        tiles.append(px)
        bpy.data.images.remove(img)
    rows = [np.concatenate(tiles[i : i + 2], 1) for i in range(0, 8, 2)]
    sheet = np.concatenate(rows, 0)
    if sheet.shape[1] != 1600:  # quick renders are half size
        rep = 1600 // sheet.shape[1]
        sheet = sheet.repeat(rep, 0).repeat(rep, 1)
    save_png(os.path.join(out, "render_contact.png"), sheet, "sRGB")
    # efforts: the drive side at frame 0 in each action, easy to hard
    sc.render.resolution_x, sc.render.resolution_y = 800, 620
    sc.frame_set(0)
    pose_bike(bike_objs, 0)
    aim(cam, (0.05, -3.4, 1.0), (0.05, 0.0, 0.95), 50)
    tob.location = (-1.0, -0.4, 1.72)
    tiles = []
    for name, (back, _rock, _gaze) in EFFORTS.items():
        use_action(rig, bpy.data.actions[name])
        sc.frame_set(0)
        tcu.body = f"{name}   back {back:.0f} deg"
        path = os.path.join(out, "debug", f"effort_{name}.png")
        sc.render.filepath = path
        bpy.ops.render.render(write_still=True)
        img = bpy.data.images.load(path)
        w, h = img.size
        tiles.append(np.array(img.pixels[:], np.float32).reshape(h, w, 4)[::-1, :, :3])
        bpy.data.images.remove(img)
    use_action(rig, bpy.data.actions[DEFAULT_EFFORT])
    sheet = np.concatenate(tiles, 1)
    if quick:
        sheet = sheet.repeat(2, 0).repeat(2, 1)
    save_png(os.path.join(out, "render_efforts.png"), sheet, "sRGB")
    sc.frame_set(0)
    pose_bike(bike_objs, 0)


# =============================================================================================
# Stages
# =============================================================================================
def limb_lengths(rig):
    b = rig.data.bones
    return {
        n: round((b[n].tail_local - b[n].head_local).length, 4)
        for n in ("thigh_l", "calf_l", "upperarm_l", "lowerarm_l")
    } | {
        "hip_z": round(b["thigh_l"].head_local.z, 4),
        "sho_z": round(b["upperarm_l"].head_local.z, 4),
    }


def stage_probe(opts):
    for props in ({}, {"leg": -1.0}, {"arm": 1.0}):
        _, rig, _ = build_body(props)
        log("limbs", props, limb_lengths(rig))
    mesh, rig, eyes = build_body({})
    log("eyes", [e.round(4).tolist() for e in eyes])
    for b in rig.data.bones:
        log(
            f"{b.name:12s} par={b.parent.name if b.parent else '-':12s}",
            np.round(b.head_local, 3).tolist(),
            np.round(b.tail_local, 3).tolist(),
            "x",
            np.round(b.matrix_local.to_3x3().col[0], 2).tolist(),
            "z",
            np.round(b.matrix_local.to_3x3().col[2], 2).tolist(),
            "conn" if b.use_connect else "",
        )
    co = mesh_co(mesh)
    log("verts", len(co), "bbox", co.min(0).round(3).tolist(), co.max(0).round(3).tolist())
    bpy.ops.wm.save_as_mainfile(filepath=os.path.join(opts["out"], "probe.blend"))


def stage_fit(opts):
    bike_objs, _ = load_bike(opts["bike_blend"], opts["out"])
    bike = BikeRef(bike_objs)
    rider, P = solve_proportions(bike)
    fit_report(rider, P)
    dbg = os.path.join(opts["out"], "debug")
    os.makedirs(dbg, exist_ok=True)
    for k in (0, 15):
        rider.pose(k, P)
        pose_bike(bike_objs, k)
        debug_render(os.path.join(dbg, f"side_{k:02d}.png"), (0.15, -4, 0.8), (0.15, 0, 0.8), 2.0)
    debug_render(os.path.join(dbg, "front.png"), (4, 0, 1.0), (0, 0, 0.9), 1.4)
    debug_render(os.path.join(dbg, "hands.png"), (0.75, -0.6, 1.2), (0.52, -0.1, 0.9))
    bpy.ops.wm.save_as_mainfile(filepath=os.path.join(opts["out"], "fit.blend"))


def stage_all(opts):
    import json

    out = opts["out"]
    t0 = time.time()
    bike_objs, _ = load_bike(opts["bike_blend"], out)
    bike = BikeRef(bike_objs)
    rider, P = solve_proportions(bike)
    fit_report(rider, P)
    fits = solve_efforts(rider, bike, P)
    # gear, built on the rest pose and skinned to the rig
    mats = make_materials(out)
    gear = Gear(rider, mats)
    helmet = gear.build_helmet()
    parts = [helmet, gear.build_straps(), *gear.build_glasses(), gear.build_brows()]
    shoes = gear.build_shoes()
    parts.append(shoes)
    gear.remove_hidden_feet()
    tex = 1024 if opts["quick"] else 2048
    bmap = paint_body(rider, out, tex)
    paint_helmet(helmet, gear.C, out, min(tex, 1024))
    paint_shoes(shoes, rider, out, min(tex, 1024))
    hook_textures(mats, out)
    assign_body_materials(rider, bmap, mats)
    for ob in parts:
        if not ob.modifiers:
            mod = ob.modifiers.new("Armature", "ARMATURE")
            mod.object = rider.rig
    rider_ob = join_rider(rider, parts)
    rider.mesh = rider_ob
    log(f"gear and textures done, {time.time() - t0:.0f}s")
    recs = bake_efforts(rider, fits)
    stats = collect_stats(rider, fits, rider_ob, bike, bike_objs, recs)
    log(json.dumps(stats, indent=1, default=str))
    fbx = os.path.join(out, "rider.fbx")
    export_fbx(rider.rig, rider_ob, fbx)
    blend = os.path.join(out, "rider.blend")
    bpy.ops.wm.save_as_mainfile(filepath=blend)
    if os.path.exists(blend + "1"):
        os.remove(blend + "1")  # Blender's backup of the previous build
    if opts["render"]:
        os.makedirs(os.path.join(out, "debug"), exist_ok=True)
        render_all(out, opts["quick"], bike_objs, rider.rig)
    stats["fbx_reimport"] = check_fbx(fbx)
    stats["build_seconds"] = round(time.time() - t0)
    with open(os.path.join(out, "stats.json"), "w") as f:
        json.dump(stats, f, indent=2, default=str)
    log(f"done in {time.time() - t0:.0f}s")


def main():
    opts = parse_args()
    os.makedirs(opts["out"], exist_ok=True)
    bpy.ops.wm.read_homefile(use_empty=True)
    if opts["stage"] == "probe":
        stage_probe(opts)
    elif opts["stage"] == "fit":
        stage_fit(opts)
    else:
        stage_all(opts)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        import traceback

        traceback.print_exc()
        sys.exit(1)  # Blender would otherwise exit 0 after a script error
