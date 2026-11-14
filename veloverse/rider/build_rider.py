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
    "muscle": 0.68,
    "weight": 0.30,  # lean
    "proportions": 1.0,  # MakeHuman's "ideal" proportions
    "cupsize": 0.5,
    "firmness": 0.5,
}
RACE = {"asian": 0.2, "caucasian": 0.6, "african": 0.2}
# Extra MPFB targets for a cyclist's build: (path under mpfb/data/targets, weight)
BODY_TARGETS = [
    ("torso/torso-vshape-decr.target.gz", 0.25),  # narrower upper body than a gym build
    ("arms/l-upperarm-muscle-decr.target.gz", 0.30),
    ("arms/r-upperarm-muscle-decr.target.gz", 0.30),
    ("legs/l-upperleg-muscle-incr.target.gz", 0.45),  # quads
    ("legs/r-upperleg-muscle-incr.target.gz", 0.45),
    ("legs/l-lowerleg-muscle-incr.target.gz", 0.35),  # calves
    ("legs/r-lowerleg-muscle-incr.target.gz", 0.35),
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
BACK_ANGLE = 42.0  # deg from horizontal, hip joint to shoulder joint
GAZE_DISTANCE = 10.0  # m ahead, where the head's forward axis meets the road
SADDLE_COMPRESSION = 0.003  # m of shorts and saddle-padding compression under the sit bones
KNEE_AT_BDC = 145.0  # deg interior knee angle at the bottom of the stroke
ELBOW_BEND = 17.0  # deg short of straight
HIP_ROCK_ROLL = 1.5  # deg each way
HIP_ROCK_DROP = 0.003  # m, over each downstroke
SHOULDER_COUNTER = 1.2  # deg of upper-torso counter-roll

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
        # hoods: the Fork's plastic, per side
        hood = self.material_verts(fork, "M_BlackPlastic")
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
    bpy.context.view_layer.update()
    # eyeball centres from the helper eyes, then drop the helpers except the eyeballs
    co = mesh_co(basemesh, world=False)
    eyes = [co[group_mask(basemesh, f"helper-{s}-eye")].mean(0) for s in ("l", "r")]
    keep = group_mask(basemesh, "body") | group_mask(basemesh, "helper-l-eye")
    keep |= group_mask(basemesh, "helper-r-eye")
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
FINGER_CURL = {"01": 52.0, "02": 62.0, "03": 32.0}  # deg about each bone's X, draped on hoods
THUMB_CURL = {"01": 8.0, "02": 18.0, "03": 20.0}
HAND_AXIS = (1.0, 0.0, -0.10)  # wrist to knuckles on the hood, world
HAND_PALM_IN = 1.0  # tan of the palm's turn from facing down toward the bike's centre (45 deg)
ARM_POLE = (-0.20, 0.20, -0.20)  # elbow direction from the shoulder-wrist midpoint (x, out, z)
CLAV_MAX = 20.0  # deg, the most the shoulders protract
HOOD_CONTACT_X = 0.548  # m, where the middle of the palm rests on the hood's top


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
            # foot's centre line just behind the ball joint, under sole + cleat + pedal stack
            self.cleat_rest[s] = Vector(
                (ball.x - CLEAT_BEHIND_BALL, ball.y, -(SOLE_THICKNESS + CLEAT_THICKNESS))
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
            D = (rot_z(0.6 * upper) @ rot_x(rolls[i]) @ rot_y(acc)).to_3x3()
            self.chain_X(name, D, self.X[self.parent[name]])
            self.set_basis(name)
        D3 = (rot_z(0.6 * upper) @ rot_x(upper) @ rot_y(acc)).to_3x3()
        for s, side in SIDES:
            D = (
                D3
                @ rot_z(-side * P["clav"]).to_3x3()
                @ rot_x(side * P.get("clav_drop", 0.0)).to_3x3()
            )
            self.chain_X(f"clavicle_{s}", D, self.X["spine_03"])
            self.set_basis(f"clavicle_{s}")
        # neck: between the back and the head; head: fixed in the world, looking down the road
        gaze = P["gaze"]
        neck_pitch = 0.5 * (acc + gaze) + P.get("neck_extra", 0.0)
        self.chain_X("neck_01", rot_y(neck_pitch).to_3x3(), self.X["spine_03"])
        self.set_basis("neck_01")
        self.chain_X("head", rot_y(gaze).to_3x3(), self.X["neck_01"])
        self.set_basis("head")
        # fingers
        pb = self.rig.pose.bones
        for s, _side in SIDES:
            for f in ("index", "middle", "ring", "pinky"):
                for k, a in FINGER_CURL.items():
                    pb[f"{f}_{k}_{s}"].matrix_basis = rot_x(a)
            for k, a in THUMB_CURL.items():
                pb[f"thumb_{k}_{s}"].matrix_basis = rot_x(a)
        # arms: palm on the hood, hand fixed in the world
        for s, side in SIDES:
            tgt, pole, con = self.targets[("arm", s)]
            hp = self.palm[s]
            Fr = frame_from(hp["axis"], hp["normal"])
            Fd = frame_from(HAND_AXIS, (0.0, -side * HAND_PALM_IN, -1.0))
            Dh = Fd @ Fr.transposed()
            contact = Vector(self.hood[s])
            wrist = contact - Dh @ (hp["point"] - hp["wrist"])
            rest_rot = self.rest[f"hand_{s}"].to_3x3()
            tgt.matrix_world = Matrix.Translation(wrist) @ (Dh @ rest_rot).to_4x4()
            sh = self.X[f"clavicle_{s}"] @ self.head_rest[f"upperarm_{s}"]
            mid = (sh + wrist) / 2
            pole.location = mid + Vector((ARM_POLE[0], side * ARM_POLE[1], ARM_POLE[2]))
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
def solve_fit(rider, bike, iters=8):
    hood = {}
    for s, side in SIDES:
        z, y = bike.hood_top(side, HOOD_CONTACT_X)
        hood[s] = (HOOD_CONTACT_X, y, z)
    rider.hood = hood
    P: dict[str, Any] = {
        "hip": [-0.215, 1.13],
        "torso": 48.0,
        "clav": 8.0,
        "gaze": 8.5,
        "sit_x": -0.25,
        "saddle_z": float(bike.saddle_top(-0.25)),
        "rock": 0.0,
        "pole": {k: 0.0 for k in rider.targets},
    }
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
            -5.0,
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
        P["gaze"] = math.degrees(math.atan2(eye.z, GAZE_DISTANCE))
        if abs(back - BACK_ANGLE) < 0.2 and it > 0:
            break
        P["torso"] += back - BACK_ANGLE
    rider.calibrate_poles(P)
    return P


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
    return rider, P


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


def main():
    opts = parse_args()
    os.makedirs(opts["out"], exist_ok=True)
    bpy.ops.wm.read_homefile(use_empty=True)
    if opts["stage"] == "probe":
        stage_probe(opts)
    elif opts["stage"] == "fit":
        stage_fit(opts)


if __name__ == "__main__":
    main()
