"""Upgrades the existing rider mesh and textures directly from rider.blend without needing MPFB
installed:
- Refines the overly bulky/angular calf silhouette on the Rider mesh using strictly radial
  (axis-perpendicular) slimming and Taubin smoothing so knees, ankles, sock hems, skinning,
  and baked actions stay 100% crisp and intact.
- Smooths the pelvis/thigh skinning transition and outer-lower glute mesh ridge symmetrically
  so both glutes deform smoothly across all pedal phases without vertical creases or pinches.
- Paints antialiased, tailored three rear jersey pockets, waist hem elastic, center-back seam,
  bib-short leg gripper cuffs, and ribbed cycling sock cuffs into T_Body_BaseColor,
  T_Body_Shade, and T_Body_Normal so every custom Kit colour inherits crisp garment detail.

Run headless with Blender 5.x:
    blender -b /path/to/src/rider.blend -P rider/polish_rider_textures.py -- \
        --src /path/to/src --dst /path/to/dst [--preview /path/to/preview_dir]
"""

import argparse
import contextlib
import os
import shutil
import sys

import bmesh  # ty: ignore[unresolved-import]
import bpy  # ty: ignore[unresolved-import]
import numpy as np  # ty: ignore[unresolved-import]
from mathutils import Vector  # ty: ignore[unresolved-import]

BODY_MATS = {"M_Skin", "M_Kit"}


def _blur(a, r):
    out = a.copy()
    for _ in range(r):
        out = (
            out
            + np.roll(out, 1, 0)
            + np.roll(out, -1, 0)
            + np.roll(out, 1, 1)
            + np.roll(out, -1, 1)
        ) / 5.0
    return out


def _smoothstep(e0, e1, x):
    t = np.clip((x - e0) / np.maximum(e1 - e0, 1e-8), 0.0, 1.0)
    return t * t * (3.0 - 2.0 * t)


def _bilinear_upsample(img, cov, scale):
    """Bilinearly upsamples (h, w, C) float array by integer `scale`, using valid coverage only."""
    h, w, _c = img.shape
    H, W = h * scale, w * scale
    # Dilate valid pixels by 2 steps so UV island boundaries interpolate cleanly
    p = img.copy()
    m = cov.copy()
    for _ in range(2):
        inv = ~m
        acc = np.zeros_like(p)
        cnt = np.zeros((h, w), np.float32)
        for dy, dx in ((-1, 0), (1, 0), (0, -1), (0, 1)):
            sm = np.roll(np.roll(m, dy, 0), dx, 1)
            sp = np.roll(np.roll(p, dy, 0), dx, 1)
            acc += sp * sm[..., None]
            cnt += sm.astype(np.float32)
        fill = inv & (cnt > 0)
        p[fill] = acc[fill] / cnt[fill, None]
        m = m | fill

    gy = (np.arange(H, dtype=np.float32) + 0.5) / scale - 0.5
    gx = (np.arange(W, dtype=np.float32) + 0.5) / scale - 0.5
    y0 = np.clip(np.floor(gy).astype(np.int32), 0, h - 1)
    y1 = np.clip(y0 + 1, 0, h - 1)
    wy = np.clip(gy - y0, 0.0, 1.0)[:, None, None]
    x0 = np.clip(np.floor(gx).astype(np.int32), 0, w - 1)
    x1 = np.clip(x0 + 1, 0, w - 1)
    wx = np.clip(gx - x0, 0.0, 1.0)[None, :, None]

    # Separable bilinear (row then col) to keep memory small
    tmp = p[y0] * (1.0 - wy) + p[y1] * wy
    out = tmp[:, x0] * (1.0 - wx) + tmp[:, x1] * wx
    cov_up = np.repeat(np.repeat(cov, scale, axis=0), scale, axis=1)
    return out, cov_up


def _load_rgba(path):
    img = bpy.data.images.load(path, check_existing=False)
    w, h = img.size
    arr = np.empty(w * h * 4, np.float32)
    img.pixels.foreach_get(arr)
    bpy.data.images.remove(img)
    return arr.reshape(h, w, 4)


def _save_rgba(path, arr, colorspace="sRGB"):
    h, w, _ = arr.shape
    img = bpy.data.images.new("_tmp_out", width=w, height=h, alpha=False)
    img.colorspace_settings.name = colorspace
    img.pixels.foreach_set(np.clip(arr, 0.0, 1.0).ravel())
    img.filepath_raw = path
    img.file_format = "PNG"
    img.save()
    bpy.data.images.remove(img)


def _body_mesh_graph(mesh_ob):
    """Returns rest-pose coordinates, body-material vertex mask, and 1-ring neighbor lists."""
    me = mesh_ob.data
    n_v = len(me.vertices)
    co = np.empty(n_v * 3, np.float32)
    me.vertices.foreach_get("co", co)
    co = co.reshape(n_v, 3)

    body_mat_ids = {i for i, m in enumerate(me.materials) if m and m.name in BODY_MATS}
    is_body = np.zeros(n_v, dtype=bool)
    for p in me.polygons:
        if p.material_index in body_mat_ids:
            for vi in p.vertices:
                is_body[vi] = True

    bm = bmesh.new()
    bm.from_mesh(me)
    bm.verts.ensure_lookup_table()
    neighbors = [[e.other_vert(v).index for e in v.link_edges] for v in bm.verts]
    bm.free()
    return co, is_body, neighbors


def _taubin_smooth(co, neighbors, idx, strength, iters, lam_pos=0.45, lam_neg=-0.40, u_axis=None):
    """Volume-preserving Taubin smoothing over `idx`, optionally projected off `u_axis`."""
    for _ in range(iters):
        for lam in (lam_pos, lam_neg):
            prev = co.copy()
            for vi in idx:
                nbrs = neighbors[vi]
                if not nbrs:
                    continue
                delta = prev[nbrs].mean(axis=0) - prev[vi]
                if u_axis is not None:
                    delta = delta - float(delta @ u_axis) * u_axis
                co[vi] = prev[vi] + (lam * float(strength[vi])) * delta


def _smooth_calves(mesh_ob, rig):
    """Refines the overly sharp/bulky calf silhouette on both legs strictly perpendicular to the
    calf bone axis so vertices never slide vertically along the leg (keeping the sock cuff line
    razor-straight and knees/ankles untouched)."""
    me = mesh_ob.data
    n_v = len(me.vertices)
    co, is_body, neighbors = _body_mesh_graph(mesh_ob)

    new_co = co.copy()
    affected = 0
    for side in ("l", "r"):
        b = rig.data.bones[f"calf_{side}"]
        g_calf = mesh_ob.vertex_groups.get(f"calf_{side}")
        g_thigh = mesh_ob.vertex_groups.get(f"thigh_{side}")
        if g_calf is None:
            continue
        w_leg = np.zeros(n_v, np.float32)
        c_idx = g_calf.index
        t_idx = g_thigh.index if g_thigh else -1
        for v in me.vertices:
            wc = 0.0
            wt = 0.0
            for grp in v.groups:
                if grp.group == c_idx:
                    wc = grp.weight
                elif grp.group == t_idx:
                    wt = grp.weight
            # Include knee-to-calf transition smoothly
            w_leg[v.index] = wc + 0.5 * min(wc, wt)

        h = np.array(b.head_local, np.float32)
        t_pos = np.array(b.tail_local, np.float32)
        axis = t_pos - h
        length = float(np.linalg.norm(axis))
        u_axis = axis / max(length, 1e-6)

        rel = new_co - h
        t = (rel @ u_axis) / length
        proj = h + np.outer(t * length, u_axis)
        rad_vec = new_co - proj
        rad = np.linalg.norm(rad_vec, axis=1)

        # Smooth bell spanning from just below knee (t=0.0) down to lower calf (t=0.72)
        bell = _smoothstep(-0.02, 0.16, t) * (1.0 - _smoothstep(0.46, 0.72, t))
        mask = is_body & (w_leg > 0.20) & (bell > 0.01) & (rad > 0.025)
        strength = bell * np.clip((w_leg - 0.20) / 0.65, 0.0, 1.0)

        # 1) Gentle radial slimming focused on the exaggerated lateral/posterior gastrocnemius flare
        lateral = np.abs(rad_vec[:, 1]) / np.maximum(rad, 1e-6)
        back = np.clip(-rad_vec[:, 0] / np.maximum(rad, 1e-6), 0.0, 1.0)
        slim = 0.13 * (0.45 + 0.65 * lateral + 0.20 * back) * strength
        idx = np.where(mask)[0]
        affected += len(idx)
        new_co[idx] = proj[idx] + rad_vec[idx] * (1.0 - slim[idx, None])

        # 2) Strictly axis-perpendicular Taubin smoothing (6 iterations of +0.42 / -0.38)
        # Removes sharp polygonal corners without collapsing volume or sliding UV bands along leg
        _taubin_smooth(new_co, neighbors, idx, strength, 6, 0.42, -0.38, u_axis=u_axis)

    me.vertices.foreach_set("co", new_co.ravel())
    me.update()
    return affected


def _smooth_weight_ratios(ratio, neighbors, idx_w, w_hip, y, iters, mix):
    """Smooths normalized hip weight ratios while keeping thighs strictly on their own side."""
    for _ in range(iters):
        prev = ratio.copy()
        for vi in idx_w:
            nbrs = [ni for ni in neighbors[vi] if w_hip[ni] > 0.15]
            if not nbrs:
                continue
            s = float(mix[vi])
            nr = (1.0 - s) * prev[vi] + s * prev[nbrs].mean(axis=0)
            if y[vi] < 0.005:
                nr[1] = 0.0
            if y[vi] > -0.005:
                nr[2] = 0.0
            ns = float(nr.sum())
            if ns > 1e-6:
                ratio[vi] = nr / ns


def _smooth_glutes(mesh_ob, weight_iters=12, taubin_iters=6):
    """Smooths the pelvis/thigh skinning transition, outer-lower glute ridge, and central
    saddle-contact cleft symmetrically so both glutes deform cleanly and the bib shorts sit
    flush on top of the saddle across all pedal phases.

    Preserves spine, knee/calf, and midline weights cleanly, keeping total weights normalized.
    """
    me = mesh_ob.data
    n_v = len(me.vertices)
    co, is_body, neighbors = _body_mesh_graph(mesh_ob)
    x, y, z = co[:, 0], co[:, 1], co[:, 2]
    ay = np.abs(y)

    # 1) Smooth the (pelvis, thigh_l, thigh_r) ratio within each vertex's hip weight budget
    hip_groups = ("pelvis", "thigh_l", "thigh_r")
    g_ids = [mesh_ob.vertex_groups[n].index for n in hip_groups]
    w = np.zeros((n_v, 3), np.float32)
    for v in me.vertices:
        for grp in v.groups:
            if grp.group in g_ids:
                w[v.index, g_ids.index(grp.group)] = grp.weight

    w_hip = w.sum(axis=1)
    bell_w = _smoothstep(0.72, 0.80, z) * (1.0 - _smoothstep(0.98, 1.05, z))
    idx_w = np.where(is_body & (w_hip > 0.15) & (bell_w > 0.01))[0]

    ratio = np.zeros_like(w)
    pos = w_hip > 1e-6
    ratio[pos] = w[pos] / w_hip[pos, None]

    _smooth_weight_ratios(ratio, neighbors, idx_w, w_hip, y, weight_iters, 0.5 * bell_w)

    # Gently anchor the narrow central chamois strip toward pelvis so downstrokes don't pull
    # the midline bib crotch down into the saddle, then blend the boundary smoothly
    crotch_pelvis = (
        (1.0 - _smoothstep(0.016, 0.052, ay))
        * _smoothstep(0.815, 0.850, z)
        * (1.0 - _smoothstep(0.910, 0.945, z))
        * _smoothstep(-0.055, -0.020, x)
        * (1.0 - _smoothstep(0.060, 0.095, x))
    )
    for vi in idx_w:
        cp = float(crotch_pelvis[vi])
        if cp > 0.01:
            ratio[vi] = (1.0 - 0.55 * cp) * ratio[vi] + (0.55 * cp) * np.array(
                [1.0, 0.0, 0.0], np.float32
            )
    _smooth_weight_ratios(ratio, neighbors, idx_w, w_hip, y, 4, np.full(n_v, 0.35, np.float32))

    new_w = ratio * w_hip[:, None]
    for vi in idx_w:
        for k, gi in enumerate(g_ids):
            val = float(new_w[vi, k])
            vg = mesh_ob.vertex_groups[gi]
            if val > 1e-4:
                vg.add([int(vi)], val, "REPLACE")
            else:
                with contextlib.suppress(RuntimeError):
                    vg.remove([int(vi)])
        mv = me.vertices[vi]
        tot = sum(grp.weight for grp in mv.groups)
        if tot > 1e-6 and abs(tot - 1.0) > 1e-6:
            for grp in mv.groups:
                grp.weight /= tot

    # 2) Subtle volume-preserving Taubin smoothing on the rest-pose outer-lower glute ridge
    strength_g = (
        _smoothstep(0.055, 0.095, ay)
        * (1.0 - _smoothstep(0.175, 0.205, ay))
        * _smoothstep(0.835, 0.870, z)
        * (1.0 - _smoothstep(0.975, 1.015, z))
        * (1.0 - _smoothstep(0.005, 0.045, x))
    )
    idx_g = np.where(is_body & (strength_g > 0.01))[0]
    new_co = co.copy()
    _taubin_smooth(new_co, neighbors, idx_g, strength_g, taubin_iters, 0.45, -0.40)

    # 3) Unfold the deep midline gluteal/perineal cleft pocket and lift it gently along world +Z
    # (pelvis rest direction [-0.685, 0, 0.728] under ~43 deg cycling forward lean) so the
    # center bib shorts sit cleanly on top of the saddle instead of dipping into the cutout
    cleft_w = (
        (1.0 - _smoothstep(0.020, 0.068, ay))
        * _smoothstep(0.820, 0.855, z)
        * (1.0 - _smoothstep(0.910, 0.945, z))
        * _smoothstep(-0.060, -0.025, x)
        * (1.0 - _smoothstep(0.065, 0.100, x))
    )
    idx_c = np.where(is_body & (cleft_w > 0.01))[0]
    _taubin_smooth(new_co, neighbors, idx_c, cleft_w, 18, 0.45, -0.38)

    x2, y2, z2 = new_co[:, 0], new_co[:, 1], new_co[:, 2]
    lift_w = (
        (1.0 - _smoothstep(0.020, 0.064, np.abs(y2)))
        * _smoothstep(0.820, 0.855, z2)
        * (1.0 - _smoothstep(0.905, 0.945, z2))
        * _smoothstep(-0.055, -0.018, x2)
        * (1.0 - _smoothstep(0.065, 0.100, x2))
    )
    lift_dir = np.array([-0.685, 0.0, 0.728], np.float32)
    lift_dir /= np.linalg.norm(lift_dir)
    idx_l = np.where(is_body & (lift_w > 0.01))[0]
    new_co[idx_l] += (0.013 * lift_w[idx_l, None]) * lift_dir
    _taubin_smooth(new_co, neighbors, idx_c, cleft_w, 4, 0.40, -0.36)

    # 4) Bridge the visible rear intergluteal cleft (facing the Chase camera at x < -0.03,
    # z in 0.87..1.04, |y| < 0.06) so tight Lycra bib shorts stretch smoothly across the seat
    # pad instead of tucking into a deep anatomical wedgie crease.
    x3, y3, z3 = new_co[:, 0], new_co[:, 1], new_co[:, 2]
    ay3 = np.abs(y3)
    rear_w = (
        (1.0 - _smoothstep(0.012, 0.062, ay3))
        * _smoothstep(0.865, 0.905, z3)
        * (1.0 - _smoothstep(1.005, 1.045, z3))
        * (1.0 - _smoothstep(-0.045, -0.010, x3))
    )
    idx_r = np.where(is_body & (rear_w > 0.01))[0]
    cheek_mask = is_body & (ay3 >= 0.052) & (ay3 <= 0.088) & (x3 < -0.04)
    cheek_z = z3[cheek_mask]
    cheek_x = x3[cheek_mask]
    if len(cheek_z) > 10 and len(idx_r) > 0:
        for vi in idx_r:
            near_z = np.abs(cheek_z - z3[vi]) < 0.018
            if np.any(near_z):
                # Rear surface has negative X; pulling toward cheek min X bridges the cleft.
                target_x = float(np.percentile(cheek_x[near_z], 25))
                if new_co[vi, 0] > target_x:
                    w_pull = 0.82 * float(rear_w[vi])
                    new_co[vi, 0] = (1.0 - w_pull) * new_co[vi, 0] + w_pull * target_x
        _taubin_smooth(new_co, neighbors, idx_r, rear_w, 12, 0.45, -0.38)

    me.vertices.foreach_set("co", new_co.ravel())
    me.update()
    return len(idx_w), len(idx_g), len(idx_l)


def _rasterize_body_xyz(mesh_ob, size):
    """Rasterizes rest-pose (x, y, z) body positions onto UV space (h, w, 3), strictly filtering
    to body atlas materials so helmet/shoe/hair UVs never overwrite body UVs."""
    me = mesh_ob.data
    me.calc_loop_triangles()
    uv_layer = me.uv_layers.active.data
    body_mat_ids = {i for i, m in enumerate(me.materials) if m and m.name in BODY_MATS}

    n_v = len(me.vertices)
    co = np.empty(n_v * 3, np.float32)
    me.vertices.foreach_get("co", co)
    co = co.reshape(n_v, 3)

    n_tri = len(me.loop_triangles)
    vi = np.empty(n_tri * 3, np.int32)
    li = np.empty(n_tri * 3, np.int32)
    mi = np.empty(n_tri, np.int32)
    me.loop_triangles.foreach_get("vertices", vi)
    me.loop_triangles.foreach_get("loops", li)
    me.loop_triangles.foreach_get("material_index", mi)
    vi = vi.reshape(n_tri, 3)
    li = li.reshape(n_tri, 3)
    keep = np.isin(mi, list(body_mat_ids))
    vi, li = vi[keep], li[keep]

    uvs = np.empty(len(uv_layer) * 2, np.float32)
    uv_layer.foreach_get("uv", uvs)
    uvs = uvs.reshape(-1, 2)
    tri_uv = uvs[li] * size
    tri_co = co[vi]

    img = np.zeros((size, size, 3), np.float32)
    cov = np.zeros((size, size), dtype=bool)
    for t in range(len(vi)):
        p0, p1, p2 = tri_uv[t]
        x0 = max(0, int(np.floor(min(p0[0], p1[0], p2[0]))))
        x1 = min(size - 1, int(np.ceil(max(p0[0], p1[0], p2[0]))))
        y0 = max(0, int(np.floor(min(p0[1], p1[1], p2[1]))))
        y1 = min(size - 1, int(np.ceil(max(p0[1], p1[1], p2[1]))))
        if x1 < x0 or y1 < y0:
            continue
        xs = np.arange(x0, x1 + 1, dtype=np.float32) + 0.5
        ys = np.arange(y0, y1 + 1, dtype=np.float32) + 0.5
        gx, gy = np.meshgrid(xs, ys)
        denom = (p1[1] - p2[1]) * (p0[0] - p2[0]) + (p2[0] - p1[0]) * (p0[1] - p2[1])
        if abs(denom) < 1e-8:
            continue
        w0 = ((p1[1] - p2[1]) * (gx - p2[0]) + (p2[0] - p1[0]) * (gy - p2[1])) / denom
        w1 = ((p2[1] - p0[1]) * (gx - p2[0]) + (p0[0] - p2[0]) * (gy - p2[1])) / denom
        w2 = 1.0 - w0 - w1
        m = (w0 >= -1e-4) & (w1 >= -1e-4) & (w2 >= -1e-4)
        if not m.any():
            continue
        vals = w0[m, None] * tri_co[t, 0] + w1[m, None] * tri_co[t, 1] + w2[m, None] * tri_co[t, 2]
        sub_y, sub_x = np.nonzero(m)
        img[y0 + sub_y, x0 + sub_x] = vals
        cov[y0 + sub_y, x0 + sub_x] = True
    return img, cov


def _export_fbx(rig, rider_ob, path):
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
        bake_anim_use_all_actions=True,
        bake_anim_force_startend_keying=True,
        bake_anim_step=1.0,
        bake_anim_simplify_factor=0.0,
        path_mode="RELATIVE",
        embed_textures=False,
        use_custom_props=False,
    )


def _render_previews(rig, dst, preview_dir):
    os.makedirs(preview_dir, exist_ok=True)
    sc = bpy.context.scene
    for img in bpy.data.images:
        fp = bpy.path.abspath(img.filepath)
        cand = os.path.join(dst, os.path.basename(fp))
        if os.path.exists(cand):
            img.filepath = cand
            img.reload()
    if sc.world is None:
        sc.world = bpy.data.worlds.new("_prev_world")
    if sc.world.node_tree:
        bg = sc.world.node_tree.nodes.get("Background")
        if bg:
            bg.inputs[0].default_value = (0.32, 0.35, 0.40, 1.0)
            bg.inputs[1].default_value = 0.85
    for lname, loc, tgt, energy in (
        ("_key", (-3.5, -2.8, 3.2), (0.0, 0.0, 0.9), 4.0),
        ("_rim", (2.5, 2.2, 2.4), (0.0, 0.0, 0.9), 2.5),
    ):
        ld = bpy.data.lights.new(lname, "SUN")
        ld.energy = energy
        ld.angle = 0.25
        lo = bpy.data.objects.new(lname, ld)
        lo.location = loc
        lo.rotation_euler = (Vector(tgt) - Vector(loc)).to_track_quat("-Z", "Y").to_euler()
        sc.collection.objects.link(lo)

    sc.render.engine = "CYCLES"
    sc.cycles.device = "CPU"
    sc.cycles.samples = 24
    sc.cycles.use_denoising = True
    sc.render.resolution_x = 1280
    sc.render.resolution_y = 720
    sc.render.resolution_percentage = 100
    cam = sc.camera
    if cam is None:
        cd = bpy.data.cameras.new("_prev_cam")
        cam = bpy.data.objects.new("_prev_cam", cd)
        sc.collection.objects.link(cam)
        sc.camera = cam
    views = {
        "preview_chase.png": (45, (-2.3, -0.25, 1.32), (0.25, 0.0, 0.85), 42),
        "preview_chase_flex.png": (15, (-2.3, -0.25, 1.32), (0.25, 0.0, 0.85), 42),
        "preview_side.png": (45, (0.10, -3.8, 0.95), (0.10, 0.0, 0.82), 50),
    }
    for name, (frame, loc, tgt, lens) in views.items():
        sc.frame_set(frame)
        cam.location = loc
        cam.rotation_euler = (Vector(tgt) - Vector(loc)).to_track_quat("-Z", "Y").to_euler()
        cam.data.lens = lens
        sc.render.filepath = os.path.join(preview_dir, name)
        bpy.ops.render.render(write_still=True)

    # Cockpit over-the-bars close-up using A_Rider_Cockpit (if present)
    cockpit_act = next((a for a in bpy.data.actions if "Cockpit" in a.name), None)
    if cockpit_act is not None and rig.animation_data is not None:
        orig_act = rig.animation_data.action
        rig.animation_data.action = cockpit_act
        sc.frame_set(15)
        cam.location = (0.12, 0.0, 1.04)
        cam.rotation_euler = (
            (Vector((0.68, 0.0, 0.82)) - Vector((0.12, 0.0, 1.04)))
            .to_track_quat("-Z", "Y")
            .to_euler()
        )
        cam.data.lens = 30
        sc.render.filepath = os.path.join(preview_dir, "preview_cockpit.png")
        bpy.ops.render.render(write_still=True)
        rig.animation_data.action = orig_act


def _sculpt_forearms_and_hands(mesh_ob, rig):
    """Beef up forearms & wrists, smooth the wrist crease, and add glove/finger volume."""
    me = mesh_ob.data
    n = len(me.vertices)
    coords = np.empty(n * 3, dtype=np.float32)
    me.vertices.foreach_get("co", coords)
    coords = coords.reshape(n, 3)

    # Build vertex adjacency for Taubin wrist smoothing
    adj = [[] for _ in range(n)]
    for e in me.edges:
        a, b = e.vertices[0], e.vertices[1]
        adj[a].append(b)
        adj[b].append(a)

    bones = rig.data.bones
    touched = 0
    for s in ("l", "r"):
        la = bones.get(f"lowerarm_{s}")
        ha = bones.get(f"hand_{s}")
        kn = bones.get(f"middle_01_{s}")
        if la is None or ha is None or kn is None:
            continue
        e_pt = np.array(la.head_local, dtype=np.float32)
        w_pt = np.array(la.tail_local, dtype=np.float32)
        k_pt = np.array(kn.head_local, dtype=np.float32)

        arm_vec = w_pt - e_pt
        arm_len = float(np.linalg.norm(arm_vec))
        arm_dir = arm_vec / max(arm_len, 1e-6)

        hand_vec = k_pt - w_pt
        hand_len = float(np.linalg.norm(hand_vec))
        hand_dir = hand_vec / max(hand_len, 1e-6)

        # Gather vertex weights for forearm, hand, and fingers on this side
        arm_vgs = {vg.index for vg in mesh_ob.vertex_groups if vg.name == f"lowerarm_{s}"}
        hand_vgs = {
            vg.index
            for vg in mesh_ob.vertex_groups
            if vg.name.endswith(f"_{s}")
            and (
                vg.name.startswith("hand_")
                or vg.name.startswith("thumb_")
                or vg.name.startswith("index_")
                or vg.name.startswith("middle_")
                or vg.name.startswith("ring_")
                or vg.name.startswith("pinky_")
            )
        }
        wrist_smooth_idx = []
        for v in me.vertices:
            w_arm = sum(g.weight for g in v.groups if g.group in arm_vgs)
            w_hand = sum(g.weight for g in v.groups if g.group in hand_vgs)
            if w_arm + w_hand < 0.05:
                continue
            p = coords[v.index]

            # 1) Forearm & wrist radial volume boost (+26% belly -> +20% wrist)
            rel_a = p - e_pt
            t_a = float(np.dot(rel_a, arm_dir)) / max(arm_len, 1e-6)
            if -0.05 <= t_a <= 1.08 and w_arm > 0.05:
                proj_a = e_pt + (t_a * arm_len) * arm_dir
                rad_a = p - proj_a
                if float(np.linalg.norm(rad_a)) < 0.075:
                    # Smooth envelope: 0 at elbow, +26% in upper/mid muscle belly, +20% at wrist
                    belly = float(np.exp(-(((t_a - 0.34) / 0.30) ** 2)))
                    wrist_fill = float(_smoothstep(0.05, 0.25, t_a)) * (
                        1.0 - float(_smoothstep(0.98, 1.08, t_a))
                    )
                    scale = 1.0 + w_arm * wrist_fill * (0.19 + 0.07 * belly)
                    coords[v.index] = proj_a + rad_a * scale
                    touched += 1

            # 2) Hand & finger fullness (+8% radial + 1.4mm fingerless glove cuff/back thickness)
            p = coords[v.index]
            rel_h = p - w_pt
            t_h = float(np.dot(rel_h, hand_dir)) / max(hand_len, 1e-6)
            if w_hand > 0.15 and -0.20 <= t_h <= 2.40:
                proj_h = w_pt + (t_h * hand_len) * hand_dir
                rad_h = p - proj_h
                r_len = float(np.linalg.norm(rad_h))
                if 0.004 < r_len < 0.068:
                    # Fingerless glove thickness from -0.25*hand_len (cuff) to 1.08 (knuckles)
                    glove_env = float(_smoothstep(-0.32, -0.18, t_h)) * (
                        1.0 - float(_smoothstep(1.02, 1.16, t_h))
                    )
                    extra = 0.0014 * glove_env
                    coords[v.index] = (
                        proj_h + rad_h * (1.0 + 0.07 * w_hand) + (rad_h / r_len) * extra
                    )
                    touched += 1

            if float(np.linalg.norm(p - w_pt)) < 0.028 and (w_arm > 0.05 or w_hand > 0.05):
                wrist_smooth_idx.append(v.index)

        # 3) 6-pass Taubin smoothing across the wrist seam to eliminate pinching
        for _ in range(6):
            for lam in (0.42, -0.44):
                nxt = coords.copy()
                for vi in wrist_smooth_idx:
                    nb = adj[vi]
                    if not nb:
                        continue
                    avg = coords[nb].mean(axis=0)
                    nxt[vi] = coords[vi] + lam * (avg - coords[vi])
                coords = nxt

    me.vertices.foreach_set("co", coords.ravel())
    me.update()
    return touched


def main():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", required=True)
    ap.add_argument("--dst", required=True)
    ap.add_argument("--preview", default="")
    args = ap.parse_args(argv)
    src, dst = args.src, args.dst
    os.makedirs(dst, exist_ok=True)

    for fn in os.listdir(src):
        sp = os.path.join(src, fn)
        dp = os.path.join(dst, fn)
        if os.path.isfile(sp) and fn.endswith((".png", ".json")):
            shutil.copy2(sp, dp)

    rig = next(o for o in bpy.data.objects if o.type == "ARMATURE")
    mesh_ob = next(o for o in bpy.data.objects if o.type == "MESH" and o.find_armature() == rig)

    # 1) Smooth overly angular calves, outer-lower glute crease, and saddle-contact cleft,
    #    and sculpt athletic forearms/wrists + fingerless glove volume.
    calf_v = _smooth_calves(mesh_ob, rig)
    glute_w_v, glute_g_v, saddle_v = _smooth_glutes(mesh_ob)
    arm_v = _sculpt_forearms_and_hands(mesh_ob, rig)

    # 2) Rasterize body-only xyz at 512x512 and bilinearly upsample to texture resolution
    base = _load_rgba(os.path.join(src, "T_Body_BaseColor.png"))
    shade = _load_rgba(os.path.join(src, "T_Body_Shade.png"))
    norm = _load_rgba(os.path.join(src, "T_Body_Normal.png"))
    mask1 = _load_rgba(os.path.join(src, "T_Body_Mask.png"))
    mask2 = _load_rgba(os.path.join(src, "T_Body_Mask2.png"))
    size = base.shape[0]

    rsize = 512
    p_small, cov_small = _rasterize_body_xyz(mesh_ob, rsize)
    pimg, cov = _bilinear_upsample(p_small, cov_small, size // rsize)

    is_jersey_main = (mask1[..., 0] > 0.5) & cov
    is_jersey = (mask1[..., 0] + mask1[..., 1] > 0.5) & cov
    is_shorts = (mask1[..., 2] > 0.5) & cov
    is_socks = (mask2[..., 0] > 0.5) & cov
    x, y, z = pimg[..., 0], pimg[..., 1], pimg[..., 2]
    ay = np.abs(y)

    # Torso jersey bounds (rider faces +X, left/right is Y, up is Z)
    torso_j = is_jersey & (ay < 0.175)
    jx_mid = float(np.median(x[torso_j]))
    jz_min = float(np.percentile(z[torso_j], 1.5))
    jz_max = float(np.percentile(z[torso_j], 98.5))
    print(
        f"body jersey torso x=[{x[torso_j].min():.3f},{x[torso_j].max():.3f}] mid={jx_mid:.3f} "
        f"z=[{jz_min:.3f},{jz_max:.3f}]",
        flush=True,
    )

    # Three structured rear cycling pockets on the lower back (-X side, strictly inside main jersey)
    back_w = (
        is_jersey_main.astype(np.float32)
        * (1.0 - _smoothstep(jx_mid - 0.032, jx_mid - 0.018, x))
        * (1.0 - _smoothstep(0.115, 0.130, ay))
    )
    pk_bot = jz_min + 0.028
    # Gentle aerodynamic droop at the outer edges of the outer two pockets
    pk_w = 0.106
    pk_top = (jz_min + 0.178) - 0.012 * np.clip(ay / pk_w, 0.0, 1.0) ** 2

    in_w = 1.0 - _smoothstep(pk_w - 0.003, pk_w + 0.003, ay)
    in_z = _smoothstep(pk_bot - 0.003, pk_bot + 0.003, z) * (
        1.0 - _smoothstep(pk_top - 0.003, pk_top + 0.003, z)
    )
    in_pocket = back_w * in_w * in_z

    # Antialiased pocket top elastic lip, recessed shadow above lip, and subtle divider stitch seams
    pk_lip = back_w * in_w * np.exp(-(((z - pk_top) / 0.0065) ** 2))
    pk_shadow = back_w * in_w * np.exp(-(((z - (pk_top + 0.009)) / 0.006) ** 2))
    pk_piping = (
        back_w
        * (1.0 - _smoothstep(pk_w - 0.005, pk_w, ay))
        * np.exp(-(((z - (pk_top - 0.002)) / 0.0022) ** 2))
    )
    div_seam = np.exp(-(((ay - 0.037) / 0.0026) ** 2))
    side_seam = np.exp(-(((ay - (pk_w - 0.002)) / 0.0026) ** 2))
    bot_seam = np.exp(-(((z - (pk_bot + 0.002)) / 0.0028) ** 2))
    pk_seams = in_pocket * np.clip(div_seam + side_seam + bot_seam, 0.0, 1.0)

    # Tailored jersey waist gripper hem & subtle center-back spine seam
    waist_hem = (
        is_jersey.astype(np.float32)
        * _smoothstep(jz_min - 0.004, jz_min + 0.002, z)
        * (1.0 - _smoothstep(jz_min + 0.016, jz_min + 0.022, z))
    )
    spine_seam = (
        back_w
        * _smoothstep(pk_top + 0.015, pk_top + 0.030, z)
        * (1.0 - _smoothstep(jz_max - 0.05, jz_max - 0.02, z))
        * np.exp(-((ay / 0.0022) ** 2))
    )

    # Bib-short thigh gripper cuff (bottom 3.8 cm of shorts)
    shorts_z_min = float(np.percentile(z[is_shorts], 1.5))
    bib_gripper = (
        is_shorts.astype(np.float32)
        * _smoothstep(shorts_z_min - 0.004, shorts_z_min + 0.002, z)
        * (1.0 - _smoothstep(shorts_z_min + 0.034, shorts_z_min + 0.040, z))
    )
    bib_seam = is_shorts.astype(np.float32) * np.exp(
        -(((z - (shorts_z_min + 0.037)) / 0.0024) ** 2)
    )

    # Ribbed cycling sock cuff (top 4.0 cm of socks)
    sock_z_max = float(np.percentile(z[is_socks], 99.0))
    sock_cuff = (
        is_socks.astype(np.float32)
        * (1.0 - _smoothstep(sock_z_max - 0.002, sock_z_max + 0.004, z))
        * _smoothstep(sock_z_max - 0.042, sock_z_max - 0.036, z)
    )

    # Fingerless road cycling gloves (wrist cuff to first finger joints on both hands)
    is_skin = (mask2[..., 2] > 0.5) & cov
    glove_body = np.zeros((size, size), np.float32)
    glove_cuff_accent = np.zeros((size, size), np.float32)
    glove_seams = np.zeros((size, size), np.float32)
    bones = rig.data.bones
    for s in ("l", "r"):
        la = bones.get(f"lowerarm_{s}")
        kn = bones.get(f"middle_01_{s}")
        if la is None or kn is None:
            continue
        w_pt = np.array(la.tail_local, dtype=np.float32)
        k_pt = np.array(kn.head_local, dtype=np.float32)
        h_vec = k_pt - w_pt
        h_len = float(np.linalg.norm(h_vec))
        h_dir = h_vec / max(h_len, 1e-6)

        dx = x - w_pt[0]
        dy = y - w_pt[1]
        dz = z - w_pt[2]
        t_h = dx * h_dir[0] + dy * h_dir[1] + dz * h_dir[2]
        rad2 = np.maximum(dx * dx + dy * dy + dz * dz - t_h * t_h, 0.0)
        near_hand = is_skin.astype(np.float32) * (1.0 - _smoothstep(0.062**2, 0.072**2, rad2))

        # Glove spans from -0.018 m (clean wrist cuff) to +h_len + 0.004 m (right at knuckles)
        g_start = -0.018
        g_end = h_len + 0.004
        in_g = (
            near_hand
            * _smoothstep(g_start - 0.0012, g_start + 0.0012, t_h)
            * (1.0 - _smoothstep(g_end - 0.0012, g_end + 0.0012, t_h))
        )
        cuff_band = (
            near_hand
            * _smoothstep(g_start - 0.0012, g_start + 0.0012, t_h)
            * (1.0 - _smoothstep(g_start + 0.0055, g_start + 0.0075, t_h))
        )
        cuff_seam = near_hand * np.exp(-(((t_h - (g_start + 0.007)) / 0.0014) ** 2))
        end_seam = near_hand * np.exp(-(((t_h - (g_end - 0.0015)) / 0.0014) ** 2))
        glove_body = np.maximum(glove_body, in_g)
        glove_cuff_accent = np.maximum(glove_cuff_accent, cuff_band)
        glove_seams = np.maximum(glove_seams, np.clip(cuff_seam + end_seam, 0.0, 1.0))

    # Paint fingerless gloves into tint masks: dark shorts channel + accent cuff stripe
    g_mask = glove_body > 0.5
    c_mask = glove_cuff_accent > 0.5
    mask2[..., 2] = np.where(g_mask, 0.0, mask2[..., 2])  # clear skin under glove
    mask1[..., 2] = np.where(g_mask & ~c_mask, 1.0, mask1[..., 2])  # charcoal glove body
    mask1[..., 1] = np.where(c_mask, 1.0, mask1[..., 1])  # kit accent wrist trim

    # Rich dark charcoal fabric base/shade on the glove + subtle technical weave
    mesh_pat = 0.5 + 0.5 * np.sin((x + y) * 1200.0) * np.sin((y - z) * 1200.0)
    mul = np.ones((size, size), np.float32)
    mul *= 1.0 - 0.34 * pk_shadow
    mul *= 1.0 - 0.24 * pk_seams
    mul *= 1.0 - 0.10 * pk_lip
    mul *= 1.0 - 0.14 * waist_hem
    mul *= 1.0 - 0.07 * spine_seam
    mul *= 1.0 - 0.16 * bib_gripper
    mul *= 1.0 - 0.24 * bib_seam
    mul *= 1.0 - 0.06 * sock_cuff
    mul *= 1.0 - 0.14 * glove_seams
    mul *= 1.0 - 0.05 * glove_body * mesh_pat

    base[..., :3] *= mul[..., None]
    shade[..., :3] *= mul[..., None]
    # Dark charcoal Lycra on glove body + kit orange accent stripe on cuff (for Blender & UE5)
    g_w = glove_body[..., None]
    c_w = glove_cuff_accent[..., None]
    charcoal = np.array([0.055, 0.058, 0.064], dtype=np.float32)
    accent_col = np.array([0.85, 0.34, 0.08], dtype=np.float32)
    base[..., :3] = base[..., :3] * (1.0 - g_w) + charcoal * g_w * mul[..., None]
    base[..., :3] = base[..., :3] * (1.0 - c_w) + accent_col * c_w * mul[..., None]
    # Subtle reflective satin piping along the top pocket lip
    base[..., :3] = (
        base[..., :3] * (1.0 - 0.28 * pk_piping[..., None]) + 0.22 * pk_piping[..., None]
    )
    shade[..., :3] = np.clip(shade[..., :3] + 0.10 * pk_piping[..., None], 0.0, 1.0)

    # Smooth normal-map relief (OpenGL Y+ tangent space)
    h = np.zeros((size, size), np.float32)
    h += 1.6 * _blur(in_pocket, 5)
    h += 1.1 * pk_lip - 1.3 * pk_seams
    h += 0.85 * waist_hem - 0.40 * spine_seam
    h += 0.65 * bib_gripper - 0.70 * bib_seam
    sock_rib = 0.5 * np.sin(y * 420.0 + x * 420.0)
    h += sock_cuff * (0.50 + 0.30 * sock_rib)
    h += glove_body * (0.22 + 0.10 * mesh_pat) - 0.32 * glove_seams

    dh_dx = (np.roll(h, -1, 1) - np.roll(h, 1, 1)) * 0.28
    dh_dy = (np.roll(h, -1, 0) - np.roll(h, 1, 0)) * 0.28
    nx = (norm[..., 0] * 2.0 - 1.0) - dh_dx
    ny = (norm[..., 1] * 2.0 - 1.0) - dh_dy
    nz = norm[..., 2] * 2.0 - 1.0
    inv = 1.0 / np.maximum(np.sqrt(nx * nx + ny * ny + nz * nz), 1e-6)
    norm[..., 0] = 0.5 + 0.5 * nx * inv
    norm[..., 1] = 0.5 + 0.5 * ny * inv
    norm[..., 2] = 0.5 + 0.5 * nz * inv

    _save_rgba(os.path.join(dst, "T_Body_BaseColor.png"), base, "sRGB")
    _save_rgba(os.path.join(dst, "T_Body_Shade.png"), shade, "Non-Color")
    _save_rgba(os.path.join(dst, "T_Body_Normal.png"), norm, "Non-Color")
    _save_rgba(os.path.join(dst, "T_Body_Mask.png"), mask1, "Non-Color")
    _save_rgba(os.path.join(dst, "T_Body_Mask2.png"), mask2, "Non-Color")

    # 3) Export polished rider.fbx and save rider.blend
    _export_fbx(rig, mesh_ob, os.path.join(dst, "rider.fbx"))
    bpy.ops.wm.save_as_mainfile(filepath=os.path.join(dst, "rider.blend"))
    print(
        f"Polished rider in {dst}: calf_verts={calf_v}, glute_w_verts={glute_w_v}, "
        f"glute_geom_verts={glute_g_v}, saddle_verts={saddle_v}, arm_verts={arm_v}, "
        f"glove_px={int(g_mask.sum())}, pocket_px={int((in_pocket > 0.5).sum())}, "
        f"gripper_px={int((bib_gripper > 0.5).sum())}, sock_cuff_px={int((sock_cuff > 0.5).sum())}",
        flush=True,
    )

    if args.preview:
        _render_previews(rig, dst, args.preview)
        print(f"Rendered previews to {args.preview}", flush=True)


if __name__ == "__main__":
    main()
