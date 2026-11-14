"""Build a 2024-style aero road bike in Blender from numbers alone, and export it for UE5.

Everything is generated here: no imported models or textures, so the result is ours outright.

Run headless (writes everything next to this script, or to --out):

    blender -b --factory-startup -P build_bike.py
    blender -b --factory-startup -P build_bike.py -- --out DIR --quick     # fast, low-res renders
    blender -b --factory-startup -P build_bike.py -- --no-render

Outputs: bike.fbx, bike.blend, T_*.png (generated textures), render_*.png, stats.json.

Conventions: everything is built in millimetres in the bike's frame (+X forward, +Y left, +Z up,
the ground at Z=0, the origin on the ground under the bottom bracket) and stored in metres.
The drive side is the right side, -Y. The geometry constants below are the only numbers a new size
or a new bike should need.
"""

from __future__ import annotations

import contextlib
import json
import math
import os
import sys
import time
from typing import Any

# Blender's bundled modules (and its numpy) aren't in the monorepo's venv.
import bmesh  # ty: ignore[unresolved-import]
import bpy  # ty: ignore[unresolved-import]
import numpy as np  # ty: ignore[unresolved-import]
from mathutils import Matrix  # ty: ignore[unresolved-import]

# =============================================================================================
# Geometry (mm and degrees). A 56 cm modern aero race bike.
# =============================================================================================
STACK = 565.0
REACH = 390.0
HEAD_ANGLE = 73.0
SEAT_ANGLE = 73.5
HEAD_TUBE = 150.0
CHAINSTAY = 408.0
BB_DROP = 70.0
FORK_OFFSET = 43.0
SEAT_TUBE = 560.0  # BB centre to the top of the seat tube, along it
TOP_TUBE_JOIN = 520.0  # where the top tube's centreline meets the seat tube, from the BB
TOP_TUBE_DROP = 24.0  # the top tube's centreline meets the head tube this far below its top
SEAT_STAY_JOIN = 330.0  # the dropped seat stays meet the seat tube this far up from the BB
SADDLE_HEIGHT = 740.0  # BB centre to the saddle's top, along the seat tube

# Wheels
BSD = 622.0  # rim bead seat diameter
RIM_DEPTH = 50.0
RIM_INNER_WIDTH = 21.0
RIM_OUTER_WIDTH = 30.0
TYRE_WIDTH = 28.0
WHEEL_DIAMETER = 678.0
SPOKES_FRONT = 24
SPOKES_REAR = 24
SPOKE_DIAMETER = 2.0
CROSS_FRONT = 0  # radial
CROSS_REAR_DRIVE = 2
CROSS_REAR_NON_DRIVE = 2
OLD_FRONT = 100.0  # hub over-locknut widths (12 mm thru-axles)
OLD_REAR = 142.0
AXLE_DIAMETER = 12.0
ROTOR_DIAMETER = 160.0
FRONT_ROTOR_Y = 44.0  # rotor planes, left of the wheel's centre plane
REAR_ROTOR_Y = 56.0

# Drivetrain
CRANK_LENGTH = 172.5
CHAINRINGS = (52, 36)
CHAINRING_Y = (-45.5, -38.5)
CASSETTE = (11, 12, 13, 14, 15, 16, 17, 19, 21, 24, 27, 30)
SMALL_COG_Y = -64.0  # the 11's plane; the others step inboard
COG_STEP = 3.65
GEAR = (52, 17)  # the gear the chain is modelled in
CHAIN_PITCH = 12.7
PULLEY_TEETH = 12
CAGE_LENGTH = 78.0  # rear derailleur pulley centres
UPPER_PULLEY = (-30.0, -80.0)  # from the rear axle (x, z)
CRANK_FACE_Y = 73.0  # outer face of the crank arm's pedal eye (Q factor 146)
CRANK_ANGLE = 0.0  # the drive arm's rest angle, degrees from forward, up positive
PEDAL_BODY_CENTRE = 52.0  # from the crank face

# Cockpit
BAR_WIDTH = 400.0  # centre to centre at the hoods
BAR_REACH = 75.0
BAR_DROP = 120.0
BAR_FLARE = 14.0  # extra half-width at the drop ends
STEM_LENGTH = 110.0
STEM_ANGLE = -12.0  # from perpendicular to the steerer
SPACERS = 10.0
HEADSET_COVER = 8.0

# Saddle
SADDLE_LENGTH = 270.0
SADDLE_WIDTH = 140.0

# Mesh density
SEG = 48  # around tubes (>= 32)
WHEEL_SEG = 512  # around the rims and tyres
VOXEL_FRAME = 0.9  # mm, the frame's union remesh
VOXEL_FORK = 0.7
VOXEL_COCKPIT = 0.6

# Paint
PAINT_COLOR = (0.010, 0.024, 0.075)  # deep metallic midnight blue (linear)
DECAL_COLOR = (0.80, 0.22, 0.05)  # signal orange stripe

# =============================================================================================
# Small vector helpers
# =============================================================================================
X = np.array([1.0, 0.0, 0.0])
Y = np.array([0.0, 1.0, 0.0])
Z = np.array([0.0, 0.0, 1.0])


def unit(v):
    v = np.asarray(v, float)
    n = np.linalg.norm(v, axis=-1, keepdims=True)
    return v / np.where(n == 0, 1.0, n)


def xz(x, z, y=0.0):
    return np.array([x, y, z], float)


def rad(d):
    return math.radians(d)


def polar(r, a):
    return np.array([r * math.cos(a), r * math.sin(a)])


def lerp(a, b, t):
    return a + (b - a) * t


def smoothstep(e0, e1, x):
    t = np.clip((x - e0) / (e1 - e0), 0.0, 1.0)
    return t * t * (3 - 2 * t)


# =============================================================================================
# 2D profiles. Each is (n, 2) in (a, b): a is the chord axis (positive toward the leading edge),
# b the width axis. All start at the trailing centre and go round the +b side first, so profiles
# of different shapes line up point for point and can be blended.
# =============================================================================================


def resample_closed(P, n):
    Pc = np.vstack([P, P[:1]])
    seg = np.linalg.norm(np.diff(Pc, axis=0), axis=1)
    s = np.concatenate([[0], np.cumsum(seg)])
    t = np.linspace(0, s[-1], n, endpoint=False)
    return np.stack([np.interp(t, s, Pc[:, 0]), np.interp(t, s, Pc[:, 1])], 1)


def resample_open(P, n):
    seg = np.linalg.norm(np.diff(P, axis=0), axis=1)
    s = np.concatenate([[0], np.cumsum(seg)])
    t = np.linspace(0, s[-1], n)
    return np.stack([np.interp(t, s, P[:, k]) for k in range(P.shape[1])], 1)


def naca_half(x):
    return 5 * (0.2969 * np.sqrt(x) - 0.1260 * x - 0.3516 * x**2 + 0.2843 * x**3 - 0.1015 * x**4)


def kamm(chord, width, n=SEG, trunc=0.78, corner=0.4, lead=0.5):
    """A truncated aerofoil (Kamm tail): a NACA nose cut off square, with rounded corners."""
    cf = chord / trunc
    xs = chord * (1 - np.cos(np.linspace(0, np.pi / 2, 90)))
    yh = naca_half(xs / cf)
    yh *= (width / 2) / yh.max()
    r = corner * yh[-1]
    x0 = chord - r
    y0 = float(np.interp(x0, xs, yh))
    keep = xs < x0
    ang = np.linspace(np.pi / 2, 0, 10)
    ux = np.concatenate([xs[keep], x0 + r * np.cos(ang)])
    uy = np.concatenate([yh[keep], (y0 - r) + r * np.sin(ang)])
    up = np.stack([ux, uy], 1)[::-1]
    lo = np.stack([ux, -uy], 1)[1:]
    P = np.vstack([[chord, 0.0], up, lo])
    prof = np.stack([lead * chord - P[:, 0], P[:, 1]], 1)
    return resample_closed(prof, n)


def superellipse(a_len, b_len, p=2.0, n=SEG):
    t = np.pi - np.linspace(0, 2 * np.pi, 720, endpoint=False)
    c, s = np.cos(t), np.sin(t)
    a = a_len / 2 * np.sign(c) * np.abs(c) ** (2 / p)
    b = b_len / 2 * np.sign(s) * np.abs(s) ** (2 / p)
    return resample_closed(np.stack([a, b], 1), n)


def circle(d, n=SEG):
    return superellipse(d, d, 2.0, n)


def chaikin(P, it=2, closed=True):
    P = np.asarray(P, float)
    for _ in range(it):
        Q = np.roll(P, -1, 0) if closed else P[1:]
        A = P if closed else P[:-1]
        new = np.empty((len(A) * 2, P.shape[1]))
        new[0::2] = 0.75 * A + 0.25 * Q
        new[1::2] = 0.25 * A + 0.75 * Q
        P = new if closed else np.vstack([P[:1], new, P[-1:]])
    return P


# =============================================================================================
# Paths and frames
# =============================================================================================


def spline(points, n):
    """A Catmull-Rom curve through the points, resampled to n stations evenly by arc length."""
    P = np.asarray(points, float)
    if len(P) == 2:
        return resample_open(np.vstack([P[0], P[1]]), n)
    Q = np.vstack([2 * P[0] - P[1], P, 2 * P[-1] - P[-2]])
    out = []
    for i in range(1, len(Q) - 2):
        p0, p1, p2, p3 = Q[i - 1], Q[i], Q[i + 1], Q[i + 2]
        t = np.linspace(0, 1, 60, endpoint=False)[:, None]
        out.append(
            0.5
            * (
                2 * p1
                + (-p0 + p2) * t
                + (2 * p0 - 5 * p1 + 4 * p2 - p3) * t**2
                + (-p0 + 3 * p1 - 3 * p2 + p3) * t**3
            )
        )
    out.append(P[-1:])
    return resample_open(np.vstack(out), n)


def tangents(C):
    T = np.gradient(C, axis=0)
    return unit(T)


def frames_hint(T, hint):
    hint = np.broadcast_to(np.asarray(hint, float), T.shape)
    A = hint - (hint * T).sum(1, keepdims=True) * T
    return unit(A)


def frames_rmf(C, T, A0):
    """Rotation-minimising frames (double reflection), starting from chord axis A0."""
    A = np.zeros_like(C)
    a = unit(A0 - np.dot(A0, T[0]) * T[0])
    A[0] = a
    for i in range(len(C) - 1):
        v1 = C[i + 1] - C[i]
        c1 = np.dot(v1, v1)
        if c1 < 1e-12:
            A[i + 1] = a
            continue
        aL = a - (2 / c1) * np.dot(v1, a) * v1
        tL = T[i] - (2 / c1) * np.dot(v1, T[i]) * v1
        v2 = T[i + 1] - tL
        c2 = np.dot(v2, v2)
        a = aL - (2 / c2) * np.dot(v2, aL) * v2 if c2 > 1e-12 else aL
        a = unit(a - np.dot(a, T[i + 1]) * T[i + 1])
        A[i + 1] = a
    return A


# =============================================================================================
# Mesh accumulation. Pieces are numpy arrays in mm, world space. Each piece is oriented so its
# normals point out (by signed volume) when it's closed.
# =============================================================================================


def face_normals(V, F):
    a, b, c = V[F[:, 0]], V[F[:, 1]], V[F[:, 2]]
    n = np.cross(b - a, c - a)
    if F.shape[1] == 4:
        d = V[F[:, 3]]
        n = n + np.cross(c - a, d - a)
    return unit(n)


def box_uv(V, F, scale=32.0):
    """Per-loop UVs by projecting each face on its dominant axis (scale mm per UV tile)."""
    n = np.abs(face_normals(V, F))
    ax = n.argmax(1)
    P = V[F] / scale  # (k, s, 3)
    uv = np.empty((*P.shape[:2], 2))
    for a, (i, j) in enumerate(((1, 2), (0, 2), (0, 1))):
        m = ax == a
        uv[m, :, 0] = P[m][:, :, i]
        uv[m, :, 1] = P[m][:, :, j]
    return uv


def signed_volume(V, groups):
    vol = 0.0
    for F in groups:
        a, b, c = V[F[:, 0]], V[F[:, 1]], V[F[:, 2]]
        vol += np.einsum("ij,ij->i", a, np.cross(b, c)).sum()
        if F.shape[1] == 4:
            d = V[F[:, 3]]
            vol += np.einsum("ij,ij->i", a, np.cross(c, d)).sum()
    return vol / 6.0


class Piece:
    """One vertex array and several face groups: (faces (k,3|4), uv (k,s,2) or None, material)."""

    def __init__(self, V):
        self.V = np.asarray(V, float)
        self.groups = []

    def add(self, F, mat, uv=None, offset=0):
        F = np.asarray(F, np.int64) + offset
        self.groups.append([F, uv, mat])
        return self

    def merge(self, other):
        off = len(self.V)
        self.V = np.vstack([self.V, other.V])
        for F, uv, mat in other.groups:
            self.groups.append([F + off, uv, mat])
        return self

    def orient(self):
        if signed_volume(self.V - self.V.mean(0), [g[0] for g in self.groups]) < 0:
            for g in self.groups:
                g[0] = g[0][:, ::-1].copy()
                if g[1] is not None:
                    g[1] = g[1][:, ::-1].copy()
        return self

    def transform(self, M):
        M = np.asarray(M, float)
        self.V = self.V @ M[:3, :3].T + M[:3, 3]
        if np.linalg.det(M[:3, :3]) < 0:
            for g in self.groups:
                g[0] = g[0][:, ::-1].copy()
                if g[1] is not None:
                    g[1] = g[1][:, ::-1].copy()
        return self


class Part:
    """A list of pieces that becomes one Blender object."""

    def __init__(self, name):
        self.name = name
        self.pieces = []

    def add(self, piece, orient=True):
        if orient:
            piece.orient()
        self.pieces.append(piece)
        return piece

    def extend(self, other):
        self.pieces.extend(other.pieces)


def grid(P, wrap_i=False, wrap_j=True, U=None, W=None):
    """Quads over an (m, n, 3) grid of points. U, W are per-grid-point UVs including seams."""
    m, n = P.shape[:2]
    mi = m if wrap_i else m - 1
    nj = n if wrap_j else n - 1
    I, J = np.meshgrid(np.arange(mi), np.arange(nj), indexing="ij")
    I1, J1 = (I + 1) % m, (J + 1) % n
    q = np.stack([I * n + J, I * n + J1, I1 * n + J1, I1 * n + J], -1).reshape(-1, 4)
    uv = None
    if U is not None:
        assert W is not None
        Iu, Ju = I + 1, J + 1
        uv = np.stack(
            [
                np.stack([U[I, J], W[I, J]], -1),
                np.stack([U[I, Ju], W[I, Ju]], -1),
                np.stack([U[Iu, Ju], W[Iu, Ju]], -1),
                np.stack([U[Iu, J], W[Iu, J]], -1),
            ],
            -2,
        ).reshape(-1, 4, 2)
    return q, uv


def loft(C, A, profiles, mat, caps=True, uv_scale=32.0, T=None):
    """Sweep profiles (m, n, 2) along centres C (m, 3) with chord axes A (m, 3)."""
    C = np.asarray(C, float)
    if T is None:
        T = tangents(C)
    B = unit(np.cross(T, A))
    prof = np.asarray(profiles, float)
    if prof.ndim == 2:
        prof = np.broadcast_to(prof, (len(C), *prof.shape))
    P = C[:, None, :] + prof[:, :, 0:1] * A[:, None, :] + prof[:, :, 1:2] * B[:, None, :]
    m, n = P.shape[:2]
    # UVs: u along the path, v around the profile, in uv_scale mm tiles
    s = np.concatenate([[0], np.cumsum(np.linalg.norm(np.diff(C, axis=0), axis=1))])
    per = np.concatenate([prof, prof[:, :1]], 1)
    pv = np.concatenate(
        [np.zeros((m, 1)), np.cumsum(np.linalg.norm(np.diff(per, axis=1), axis=2), 1)], 1
    )
    U = np.broadcast_to(s[:, None], (m, n + 1)) / uv_scale
    W = pv / uv_scale
    q, uv = grid(P, False, True, U, W)
    V = P.reshape(-1, 3)
    piece = Piece(V).add(q, mat, uv)
    if caps:
        tris = []
        extra = []
        for end, i in ((0, 0), (1, m - 1)):
            ci = len(V) + len(extra)
            extra.append(P[i].mean(0))
            ring = i * n + np.arange(n)
            nxt = i * n + (np.arange(n) + 1) % n
            c = np.full(n, ci)
            tris.append(np.stack([c, nxt, ring] if end == 0 else [c, ring, nxt], 1))
        piece.V = np.vstack([V, np.array(extra)])
        piece.add(np.vstack(tris), mat, None)
    return piece


def tube(points, profile_fn, mat, stations=60, hint=None, A0=None, caps=True, uv_scale=32.0):
    """A loft along a spline through points.

    profile_fn(s, i) gives the (n, 2) profile at s in [0, 1]."""
    C = spline(points, stations)
    T = tangents(C)
    A = frames_rmf(C, T, np.asarray(A0, float)) if hint is None else frames_hint(T, hint)
    s = np.linspace(0, 1, len(C))
    prof = np.array([profile_fn(si, i) for i, si in enumerate(s)])
    return loft(C, A, prof, mat, caps, uv_scale, T)


def cylinder(p0, p1, r, mat, n=SEG, caps=True, r1=None):
    p0, p1 = np.asarray(p0, float), np.asarray(p1, float)
    T = unit(p1 - p0)
    hint = X if abs(T[0]) < 0.9 else Z
    C = np.stack([p0, p1])
    A = frames_hint(np.stack([T, T]), hint)
    prof = np.stack([circle(2 * r, n), circle(2 * (r if r1 is None else r1), n)])
    return loft(C, A, prof, mat, caps, 32.0, np.stack([T, T]))


def revolve(
    profile, mat, centre, seg=WHEEL_SEG, closed=False, u_tiles=None, uv_scale=32.0, phase=0.0
):
    """Revolve (r, y) profile points about the Y axis through centre. Angles are in the XZ plane."""
    prof = np.asarray(profile, float)
    th = phase + np.linspace(0, 2 * np.pi, seg, endpoint=False)
    r, y = prof[:, 0], prof[:, 1]
    P = np.empty((seg, len(prof), 3))
    P[:, :, 0] = np.cos(th)[:, None] * r[None, :]
    P[:, :, 1] = y[None, :]
    P[:, :, 2] = np.sin(th)[:, None] * r[None, :]
    P += np.asarray(centre, float)
    n = len(prof)
    nn = n + 1 if closed else n
    pp = np.vstack([prof, prof[:1]]) if closed else prof
    sv = np.concatenate([[0], np.cumsum(np.linalg.norm(np.diff(pp, axis=0), axis=1))])
    thu = np.linspace(0, 2 * np.pi, seg + 1)
    if u_tiles is None:
        U = np.outer(thu, np.full(nn, r.mean())) / uv_scale
        W = np.broadcast_to(sv / uv_scale, (seg + 1, nn))
    else:
        U = np.broadcast_to((thu / (2 * np.pi) * u_tiles)[:, None], (seg + 1, nn))
        W = np.broadcast_to((sv / sv[-1])[None, :], (seg + 1, nn))
    q, uv = grid(P, True, closed, U, W)
    return Piece(P.reshape(-1, 3)).add(q, mat, uv)


def prism(outline, y0, y1, mat, M=None):
    """Extrude a star-shaped 2D outline (x, z) between planes y0 and y1 (in local space)."""
    o = np.asarray(outline, float)
    n = len(o)
    P = np.empty((2, n, 3))
    P[:, :, 0] = o[:, 0]
    P[:, :, 2] = o[:, 1]
    P[0, :, 1] = y0
    P[1, :, 1] = y1
    q, _ = grid(P, False, True)
    V = np.vstack(
        [
            P.reshape(-1, 3),
            [[o[:, 0].mean(), y0, o[:, 1].mean()], [o[:, 0].mean(), y1, o[:, 1].mean()]],
        ]
    )
    c0, c1 = 2 * n, 2 * n + 1
    j = np.arange(n)
    jn = (j + 1) % n
    t0 = np.stack([np.full(n, c0), jn, j], 1)
    t1 = np.stack([np.full(n, c1), n + j, n + jn], 1)
    pc = Piece(V).add(q, mat).add(np.vstack([t0, t1]), mat)
    if M is not None:
        pc.transform(M)
    return pc


def mat4(R=None, t=(0, 0, 0)):
    M = np.eye(4)
    if R is not None:
        M[:3, :3] = R
    M[:3, 3] = t
    return M


# Curve plane (x, y, 0) -> the bike's XZ plane at Y=y, angles kept: (x, y, z) -> (x, -z, y)
R_XZ = np.array([[1.0, 0, 0], [0, 0, -1.0], [0, 1.0, 0]])


def filled_shape(outlines, thickness, mat, M, bevel=0.3):
    """A flat part with holes: outlines[0] is the outside, the rest are holes, in curve XY (mm).
    Built through a 2D Blender curve (even-odd fill), extruded +-thickness/2 with a rounded edge."""
    cu = bpy.data.curves.new("tmp_shape", "CURVE")
    cu.dimensions = "2D"
    cu.fill_mode = "BOTH"
    for o in outlines:
        o = np.asarray(o, float)
        sp = cu.splines.new("POLY")
        sp.points.add(len(o) - 1)
        co = np.zeros((len(o), 4))
        co[:, :2] = o
        co[:, 3] = 1
        sp.points.foreach_set("co", co.ravel())
        sp.use_cyclic_u = True
    b = min(bevel, thickness * 0.45)
    cu.extrude = max(thickness / 2 - b, 1e-4)
    cu.bevel_depth = b
    cu.bevel_resolution = 2
    ob = bpy.data.objects.new("tmp_shape", cu)
    bpy.context.scene.collection.objects.link(ob)
    dg = bpy.context.evaluated_depsgraph_get()
    ev = ob.evaluated_get(dg)
    me = ev.to_mesh()
    V, groups = mesh_arrays(me)
    ev.to_mesh_clear()
    bpy.data.objects.remove(ob)
    bpy.data.curves.remove(cu)
    pc = Piece(V)
    for F in groups:
        pc.add(F, mat)
    pc.transform(M)
    return pc


def mesh_arrays(me):
    """Vertices and face groups (by corner count; n-gons fanned) of a Blender mesh."""
    nv = len(me.vertices)
    V = np.empty(nv * 3)
    me.vertices.foreach_get("co", V)
    V = V.reshape(-1, 3)
    npoly = len(me.polygons)
    ls = np.empty(npoly, np.int64)
    lt = np.empty(npoly, np.int64)
    me.polygons.foreach_get("loop_start", ls)
    me.polygons.foreach_get("loop_total", lt)
    lv = np.empty(len(me.loops), np.int64)
    me.loops.foreach_get("vertex_index", lv)
    groups = []
    for k in (3, 4):
        sel = lt == k
        if sel.any():
            idx = ls[sel][:, None] + np.arange(k)[None, :]
            groups.append(lv[idx])
    big = np.nonzero(lt > 4)[0]
    if len(big):
        tri = []
        for p in big:
            vs = lv[ls[p] : ls[p] + lt[p]]
            for k in range(1, len(vs) - 1):
                tri.append((vs[0], vs[k], vs[k + 1]))
        groups.append(np.array(tri, np.int64))
    return V, groups


# =============================================================================================
# Remeshing (voxel union of overlapping closed lofts, then smoothing and decimation)
# =============================================================================================


def temp_object(V, groups, name="tmp"):
    me = bpy.data.meshes.new(name)
    faces = []
    for F in groups:
        faces.extend(F.tolist())
    me.from_pydata((V * 0.001).tolist(), [], faces)
    me.update()
    ob = bpy.data.objects.new(name, me)
    bpy.context.scene.collection.objects.link(ob)
    return ob


def evaluated_arrays(ob):
    dg = bpy.context.evaluated_depsgraph_get()
    ev = ob.evaluated_get(dg)
    me = ev.to_mesh()
    V, groups = mesh_arrays(me)
    ev.to_mesh_clear()
    return V * 1000.0, groups


def remove_object(ob):
    me = ob.data
    bpy.data.objects.remove(ob)
    bpy.data.meshes.remove(me)


def union_remesh(pieces, voxel, mat, fillets=(), fillet_radius=30.0, fillet_iters=12, ratio=0.35):
    """Fuse closed pieces into one watertight skin: voxel remesh, round the joints, decimate."""
    V = np.vstack([p.V for p in pieces])
    groups = []
    off = 0
    for p in pieces:
        for F, _uv, _m in p.groups:
            groups.append(F + off)
        off += len(p.V)
    ob = temp_object(V, groups, "tmp_union")
    rm = ob.modifiers.new("remesh", "REMESH")
    rm.mode = "VOXEL"
    rm.voxel_size = voxel * 0.001
    rm.adaptivity = 0.0
    V2, g2 = evaluated_arrays(ob)
    remove_object(ob)
    ob = temp_object(V2, g2, "tmp_smooth")
    # light overall smoothing (voxel stair-steps), then heavier smoothing at the tube joints
    sm = ob.modifiers.new("smooth_all", "SMOOTH")
    sm.factor = 0.5
    sm.iterations = 2
    if fillets:
        vg = ob.vertex_groups.new(name="fillet")
        d = np.full(len(V2), 1e9)
        for c in fillets:
            d = np.minimum(d, np.linalg.norm(V2 - np.asarray(c), axis=1))
        w = 1.0 - smoothstep(0.4 * fillet_radius, fillet_radius, d)
        for val in np.unique(np.round(w, 2)):
            if val <= 0:
                continue
            ids = np.nonzero(np.round(w, 2) == val)[0].tolist()
            vg.add(ids, float(val), "REPLACE")
        sf = ob.modifiers.new("smooth_joints", "SMOOTH")
        sf.factor = 0.8
        sf.iterations = fillet_iters
        sf.vertex_group = "fillet"
    dec = ob.modifiers.new("decimate", "DECIMATE")
    dec.ratio = ratio
    dec.use_collapse_triangulate = True
    V3, g3 = evaluated_arrays(ob)
    remove_object(ob)
    pc = Piece(V3)
    for F in g3:
        pc.add(F, mat)
    return pc


# =============================================================================================
# Bike geometry: the key points
# =============================================================================================


class Geo:
    pass


def solve_geometry():
    g: Any = Geo()  # attributes are filled in below
    g.R = WHEEL_DIAMETER / 2
    g.BB = xz(0.0, g.R - BB_DROP)
    g.RA = xz(-math.sqrt(CHAINSTAY**2 - BB_DROP**2), g.R)
    ha = rad(HEAD_ANGLE)
    g.up = np.array([-math.cos(ha), 0, math.sin(ha)])  # up the steering axis
    g.fwd = np.array([math.sin(ha), 0, math.cos(ha)])  # forward, perpendicular to the axis
    g.HTT = xz(REACH, g.BB[2] + STACK)
    g.HTB = g.HTT - HEAD_TUBE * g.up
    g.HTC = 0.5 * (g.HTT + g.HTB)
    # the front axle: on the axis' offset line, at wheel-radius height
    zfoot = g.R - FORK_OFFSET * g.fwd[2]
    t = (g.HTB[2] - zfoot) / g.up[2]
    g.foot = g.HTB - t * g.up
    g.FA = g.foot + FORK_OFFSET * g.fwd
    g.axle_to_crown = t
    sa = rad(SEAT_ANGLE)
    g.st = np.array([-math.cos(sa), 0, math.sin(sa)])
    g.st_fwd = np.array([math.sin(sa), 0, math.cos(sa)])
    g.ST_TOP = g.BB + SEAT_TUBE * g.st
    g.wheelbase = g.FA[0] - g.RA[0]
    g.saddle = g.BB + SADDLE_HEIGHT * g.st
    # cockpit
    g.stem_base = g.HTT + (HEADSET_COVER + SPACERS + 20.0) * g.up
    a = rad(STEM_ANGLE)
    g.stem_dir = unit(math.cos(a) * g.fwd + math.sin(a) * g.up)
    g.BC = g.stem_base + STEM_LENGTH * g.stem_dir
    return g


G = solve_geometry()


# =============================================================================================
# Sprockets and the chain
# =============================================================================================


def sprocket_radius(n, pitch=CHAIN_PITCH):
    return pitch / (2 * math.sin(math.pi / n))


def sprocket_outline(n, phase=0.0, pitch=CHAIN_PITCH, roller=7.75):
    """Toothed outline (x, y) of an n-tooth sprocket; a roller seat sits at angle phase."""
    R = sprocket_radius(n, pitch)
    rs = roller / 2 * 1.01 + 0.05
    rtip = pitch * (0.6 + 1 / math.tan(math.pi / n)) / 2 - 0.4
    beta = rad(62 if n > 20 else 56)
    pts = []
    half = math.pi / n
    for k in range(n):
        th = phase + 2 * math.pi * k / n
        C = R * np.array([math.cos(th), math.sin(th)])
        seat = [
            C + rs * np.array([math.cos(p), math.sin(p)])
            for p in np.linspace(th + math.pi + beta, th + math.pi - beta, 9)
        ]
        pts.extend(seat)
        a_end = math.atan2(seat[-1][1], seat[-1][0])
        a_end += (th - a_end + math.pi) % (2 * math.pi) - math.pi + (a_end - th)  # unwrap near th
        a_end = th + ((a_end - th + math.pi) % (2 * math.pi) - math.pi)
        tip0, tip1 = th + half - 0.2 * half, th + half + 0.2 * half
        fl = a_end + 0.45 * (tip0 - a_end)
        rf = 0.5 * (np.linalg.norm(seat[-1]) + rtip) + 0.25 * (rtip - R)
        pts.append(polar(rf, fl))
        pts.append(polar(rtip, tip0))
        pts.append(polar(rtip, tip1))
        fl2 = th + 2 * half - (fl - th)
        pts.append(polar(rf, fl2))
    return np.array(pts)


def circle_pts(r, n=180, phase=0.0, cx=0.0, cy=0.0):
    a = phase + np.linspace(0, 2 * np.pi, n, endpoint=False)
    return np.stack([cx + r * np.cos(a), cy + r * np.sin(a)], 1)


def ring_band(r_out, r_in, a0, a1, n=24):
    """A rounded annular sector outline (for cut-outs and plates)."""
    a = np.linspace(a0, a1, n)
    outer = np.stack([r_out * np.cos(a), r_out * np.sin(a)], 1)
    inner = np.stack([r_in * np.cos(a[::-1]), r_in * np.sin(a[::-1])], 1)
    return chaikin(np.vstack([outer, inner]), 3)


def chain_path(cage_angle):
    """The chain's centreline (pin path) as a dense closed 3D polyline, in travel order.

    Circles with a signed radius: positive wraps clockwise seen from the drive side (+X right,
    +Z up), negative anticlockwise. Order: chainring, lower pulley, upper pulley, cog."""
    ring_n, cog_n = GEAR
    ring_y = CHAINRING_Y[CHAINRINGS.index(ring_n)]
    cog_y = SMALL_COG_Y + COG_STEP * CASSETTE.index(cog_n)
    up = np.array([G.RA[0] + UPPER_PULLEY[0], G.RA[2] + UPPER_PULLEY[1]])
    a = rad(cage_angle)
    lo = up + CAGE_LENGTH * np.array([math.sin(a), -math.cos(a)])
    rp = sprocket_radius(PULLEY_TEETH)
    circles = [
        (np.array([G.BB[0], G.BB[2]]), sprocket_radius(ring_n), ring_y),
        (lo, rp, cog_y),
        (up, -rp, cog_y),
        (np.array([G.RA[0], G.RA[2]]), sprocket_radius(cog_n), cog_y),
    ]

    def L(d):
        return np.array([-d[1], d[0]])

    tang = []
    for i in range(4):
        c1, s1, _ = circles[i]
        c2, s2, _ = circles[(i + 1) % 4]
        D = c2 - c1
        alpha = math.atan2(D[1], D[0])
        phi = alpha - math.asin((s1 - s2) / np.linalg.norm(D))
        d = np.array([math.cos(phi), math.sin(phi)])
        tang.append((c1 + s1 * L(d), c2 + s2 * L(d), d))
    pts = []
    step = 0.2
    for i in range(4):
        c, s, y = circles[i]
        arrive = tang[i - 1][1]
        leave = tang[i][0]
        a0 = math.atan2(*(arrive - c)[::-1])
        a1 = math.atan2(*(leave - c)[::-1])
        r = abs(s)
        if s > 0:  # clockwise: decreasing angle
            while a1 > a0:
                a1 -= 2 * math.pi
        else:
            while a1 < a0:
                a1 += 2 * math.pi
        k = max(2, int(abs(a1 - a0) * r / step))
        aa = np.linspace(a0, a1, k, endpoint=False)
        pts.append(np.stack([c[0] + r * np.cos(aa), np.full(k, y), c[1] + r * np.sin(aa)], 1))
        p0, p1 = tang[i][0], tang[i][1]
        y1 = circles[(i + 1) % 4][2]
        k = max(2, int(np.linalg.norm(p1 - p0) / step))
        t = np.linspace(0, 1, k, endpoint=False)[:, None]
        seg = np.hstack(
            [p0[0] + (p1[0] - p0[0]) * t, y + (y1 - y) * t, p0[1] + (p1[1] - p0[1]) * t]
        )
        pts.append(seg)
    return np.vstack(pts), circles, (up, lo)


def walk_chain(path, pitch=CHAIN_PITCH):
    pins = [path[0]]
    i = 0
    cur = path[0]
    N = len(path)
    win = int(pitch / 0.15) + 40
    while True:
        seg = path[i : min(N, i + win)]
        d = np.linalg.norm(seg - cur, axis=1)
        hit = np.nonzero(d >= pitch)[0]
        if len(hit) == 0:
            break
        j = i + hit[0]
        a, b = path[j - 1], path[j]
        # solve |a + t(b-a) - cur| = pitch
        ab, ac = b - a, a - cur
        qa, qb, qc = ab @ ab, 2 * ab @ ac, ac @ ac - pitch**2
        t = (-qb + math.sqrt(max(qb * qb - 4 * qa * qc, 0))) / (2 * qa)
        cur = a + t * ab
        pins.append(cur)
        i = j - 1
    res = np.linalg.norm(path[0] - cur)
    return np.array(pins), res


def solve_chain():
    def count(angle):
        path, _, _ = chain_path(angle)
        pins, res = walk_chain(path)
        return len(pins) - 1 + res / CHAIN_PITCH

    angles = np.linspace(5, 40, 36)
    counts = np.array([count(a) for a in angles])
    mid = count(20.0)
    target = 2 * round(mid / 2)
    # find the bracket nearest 20 degrees
    best = None
    for k in range(len(angles) - 1):
        c0, c1 = counts[k] - target, counts[k + 1] - target
        if (c0 == 0 or c0 * c1 < 0) and (best is None or abs(angles[k] - 20) < abs(best[0] - 20)):
            best = (angles[k], angles[k + 1])
    if best is None:
        target += 2 if counts.max() > target else -2
        for k in range(len(angles) - 1):
            if (counts[k] - target) * (counts[k + 1] - target) <= 0:
                best = (angles[k], angles[k + 1])
    assert best is not None
    lo, hi = best
    flo = count(lo) - target
    for _ in range(40):
        m = 0.5 * (lo + hi)
        fm = count(m) - target
        if (fm < 0) == (flo < 0):
            lo, flo = m, fm
        else:
            hi = m
    angle = 0.5 * (lo + hi)
    path, circles, pulleys = chain_path(angle)
    pins, res = walk_chain(path)
    if res < CHAIN_PITCH / 2:
        pins = pins[:-1]
    return angle, pins, circles, pulleys


def phase_on(pins, centre2, radius, y=None):
    """Angle of the first pin sitting on a sprocket's pitch circle."""
    d = np.hypot(pins[:, 0] - centre2[0], pins[:, 2] - centre2[1])
    k = np.argmin(np.abs(d - radius))
    return math.atan2(pins[k, 2] - centre2[1], pins[k, 0] - centre2[0])


# =============================================================================================
# Parts
# =============================================================================================
PAINT, DECAL, CARBON, RUBBER, ALLOY, PLASTIC, TAPE = (
    "M_FramePaint",
    "M_Decal",
    "M_Carbon",
    "M_Rubber",
    "M_Alloy",
    "M_BlackPlastic",
    "M_BarTape",
)


def lead_perp(d, toward):
    """Unit vector perpendicular to d in the plane of d and toward, on toward's side."""
    d = unit(d)
    t = np.asarray(toward, float)
    return unit(t - np.dot(t, d) * d)


def build_frame_body():
    """Main triangle, stays, BB shell and dropouts as overlapping closed lofts, then fused."""
    pcs = []
    joints = []
    # head tube: tapered truncated aerofoil around the steering axis
    ht_lo = G.HTB - 4 * G.up
    ht_hi = G.HTT
    C = spline([ht_lo, ht_hi], 30)
    T = np.broadcast_to(G.up, C.shape)
    A = np.broadcast_to(G.fwd, C.shape)
    s = np.linspace(0, 1, len(C))
    prof = [kamm(lerp(74, 60, si), lerp(54, 46, si), lead=0.40, trunc=0.80) for si in s]
    pcs.append(loft(C, A, prof, PAINT, T=T))
    # down tube
    dt_a = G.HTB + 45 * G.up - 4 * G.fwd
    dt_b = G.BB + xz(6, 4)
    dt_dir = unit(dt_b - dt_a)
    dt_lead = lead_perp(dt_dir, X - 0.3 * Z)

    def dt_prof(si, i):
        k = kamm(lerp(66, 56, si), lerp(44, 50, si), trunc=0.80)
        box = superellipse(56, 64, 3.2)
        return lerp(k, box, smoothstep(0.72, 1.0, si))

    pcs.append(tube([dt_a, dt_b], dt_prof, PAINT, 70, hint=dt_lead))
    # seat tube: truncated aerofoil, pushed forward where it would touch the rear tyre
    st_a = G.BB - 8 * G.st
    st_b = G.ST_TOP
    C = spline([st_a, st_b], 70)
    T = np.broadcast_to(G.st, C.shape)
    A = np.broadcast_to(G.st_fwd, C.shape)
    s = np.linspace(0, 1, len(C))
    profs = []
    Cs = C.copy()
    for i, si in enumerate(s):
        chord, width = lerp(60, 48, si), lerp(46, 32, si)
        k = kamm(chord, width, trunc=0.80, lead=0.5)
        if si < 0.08:
            k = lerp(superellipse(52, 56, 3.0), k, smoothstep(0.0, 0.08, si))
        # rear tyre clearance: the trailing face must stay 7 mm off the tyre
        tail = Cs[i] - 0.5 * chord * A[i]
        gap = np.linalg.norm(tail - G.RA) - (G.R + 7)
        if gap < 0:
            Cs[i] = Cs[i] - gap * unit(Cs[i] - G.RA)
        profs.append(k)
    # smooth the pushed centreline
    for _ in range(20):
        Cs[1:-1] = 0.25 * Cs[:-2] + 0.5 * Cs[1:-1] + 0.25 * Cs[2:]
    pcs.append(loft(Cs, A, profs, PAINT, T=tangents(Cs)))
    G.st_line = Cs
    # top tube
    tt_a = G.HTT - TOP_TUBE_DROP * G.up - 6 * G.fwd
    tt_b = G.BB + TOP_TUBE_JOIN * G.st - 6 * G.st_fwd
    tt_dir = unit(tt_b - tt_a)
    pcs.append(
        tube(
            [tt_a, tt_b],
            lambda si, i: superellipse(lerp(44, 34, si), lerp(36, 28, si), 2.6),
            PAINT,
            60,
            hint=lead_perp(tt_dir, Z),
        )
    )
    # bottom bracket shell (press fit, 86.5 mm)
    pcs.append(cylinder(G.BB + xz(0, 0, -43.0), G.BB + xz(0, 0, 43.0), 25.5, PAINT))
    pcs.append(cylinder(G.BB + xz(-12, -8, -38.0), G.BB + xz(-12, -8, 38.0), 22, PAINT))
    joints += [G.BB + xz(-30, 10, 0), G.BB + xz(10, 20, 0)]
    # chainstays: tall near the BB, tucked in past the chainrings, out to the dropouts
    for side in (-1, 1):
        yy = lambda v, side=side: side * v  # noqa: E731
        pts = [
            G.BB + xz(-8, 4, yy(24)),
            G.BB + xz(-80, 12, yy(26)),
            G.BB + xz(-150, 26, yy(32)),
            G.RA + xz(140, -16, yy(56)),
            G.RA + xz(8, -2, yy(73)),
        ]

        def cs_prof(si, i):
            return superellipse(lerp(34, 17, si), lerp(20, 12, si), 3.0)

        pcs.append(tube(pts, cs_prof, PAINT, 70, hint=Z))
        # dropped seat stays
        join = G.BB + SEAT_STAY_JOIN * G.st
        ss_pts = [G.RA + xz(4, 10, yy(73)), G.RA + xz(150, 120, yy(45)), join + xz(-4, -2, yy(9))]
        ss_dir = unit(ss_pts[-1] - ss_pts[0])

        def ss_prof(si, i):
            return kamm(lerp(20, 26, si), lerp(12, 15, si), trunc=0.82, corner=0.5)

        pcs.append(tube(ss_pts, ss_prof, PAINT, 60, hint=lead_perp(ss_dir, X - 0.4 * Z)))
        # dropout around the 12 mm thru-axle
        pcs.append(cylinder(G.RA + xz(0, 0, yy(70.5)), G.RA + xz(0, 0, yy(79.5)), 13.0, PAINT))
        pcs.append(cylinder(G.RA + xz(5, 5, yy(70.5)), G.RA + xz(5, 5, yy(79.5)), 11.0, PAINT))
        joints.append(G.RA + xz(4, 6, yy(74)))
        joints.append(join + xz(0, 0, yy(10)))
    joints += [G.HTB + 45 * G.up, G.HTT - TOP_TUBE_DROP * G.up, G.BB + TOP_TUBE_JOIN * G.st]
    body = union_remesh(
        pcs, VOXEL_FRAME, PAINT, joints, fillet_radius=34, fillet_iters=14, ratio=0.30
    )
    G.dt_a, G.dt_b = dt_a, dt_b
    return body


def apply_decal(piece):
    """Cut a two-band stripe across the top of the down tube and mark it M_Decal."""
    ob = temp_object(piece.V, [g[0] for g in piece.groups], "tmp_decal")
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    dt = unit(G.dt_b - G.dt_a)
    tilt = unit(dt + 0.35 * Z)  # band planes lean back for a raked look
    bands = [(150.0, 176.0), (184.0, 192.0)]  # mm down the tube from its top joint
    axis0 = G.dt_a * 0.001
    near = lambda co: np.linalg.norm(np.cross(np.array(co) - axis0, dt)) < 0.045  # noqa: E731
    for b0, b1 in bands:
        for d in (b0, b1):
            pc = (G.dt_a + d * dt) * 0.001
            geom = [
                v for v in bm.verts if near(v.co) and abs(np.dot(np.array(v.co) - pc, tilt)) < 0.012
            ]
            fs = set()
            for v in geom:
                fs.update(v.link_faces)
            es = set()
            for f in fs:
                es.update(f.edges)
            vs = set()
            for f in fs:
                vs.update(f.verts)
            if fs:
                bmesh.ops.bisect_plane(
                    bm, geom=list(vs) + list(es) + list(fs), plane_co=pc, plane_no=tilt
                )
    bmesh.ops.triangulate(bm, faces=[f for f in bm.faces if len(f.verts) > 4])
    bm.to_mesh(ob.data)
    bm.free()
    V, groups = mesh_arrays(ob.data)
    remove_object(ob)
    V = V * 1000.0
    out = Piece(V)
    for F in groups:
        c = V[F].mean(1)
        rel = c - G.dt_a
        along = rel @ tilt
        radial = np.linalg.norm(np.cross(rel, dt), axis=1)
        dec = np.zeros(len(F), bool)
        for b0, b1 in bands:
            p0 = np.dot(b0 * dt, tilt)
            p1 = np.dot(b1 * dt, tilt)
            dec |= (along > p0) & (along < p1)
        dec &= radial < 45
        if (~dec).any():
            out.add(F[~dec], PAINT)
        if dec.any():
            out.add(F[dec], DECAL)
    return out


def caliper(centre, axis_angle, rotor_y, rotor_r, mat=PLASTIC, M=None):
    """Flat-mount disc caliper: a C-section swept round the rotor's edge."""
    a = rad(axis_angle)
    span = 46.0 / rotor_r  # radians covered
    th = a + np.linspace(-span / 2, span / 2, 28)
    sec = np.array(
        [
            (rotor_r + 13, -13),
            (rotor_r + 13, 13),
            (rotor_r - 18, 13),
            (rotor_r - 19, 8),
            (rotor_r - 17, 2.2),
            (rotor_r + 2, 2.2),
            (rotor_r + 2, -2.2),
            (rotor_r - 17, -2.2),
            (rotor_r - 19, -8),
            (rotor_r - 18, -13),
        ]
    )
    sec = resample_closed(chaikin(sec, 3), 64)
    t = np.linspace(-1, 1, len(th))
    shrink = np.sqrt(np.clip(1 - np.abs(t) ** 8, 0.05, 1))
    P = np.empty((len(th), len(sec), 3))
    rc = rotor_r - 3
    for i, ang in enumerate(th):
        r = rc + (sec[:, 0] - rc) * shrink[i]
        yy = sec[:, 1] * (0.6 + 0.4 * shrink[i])
        P[i, :, 0] = centre[0] + r * math.cos(ang)
        P[i, :, 1] = rotor_y + yy
        P[i, :, 2] = centre[2] + r * math.sin(ang)
    q, _ = grid(P, False, True)
    V = P.reshape(-1, 3)
    m, n = P.shape[:2]
    caps = [V]
    extra = [P[0].mean(0), P[-1].mean(0)]
    j = np.arange(n)
    jn = (j + 1) % n
    t0 = np.stack([np.full(n, m * n), jn, j], 1)
    t1 = np.stack([np.full(n, m * n + 1), (m - 1) * n + j, (m - 1) * n + jn], 1)
    pc = Piece(np.vstack([*caps, np.array(extra)])).add(q, mat).add(np.vstack([t0, t1]), mat)
    return pc


def circle_hull(circles, n=48):
    """Convex outline (anticlockwise) round a set of (centre (x, y), radius) circles."""
    pts = []
    for c, r in circles:
        pts.append(circle_pts(r, n, 0.0, c[0], c[1]))
    P = np.vstack(pts)
    P = P[np.lexsort((P[:, 1], P[:, 0]))]

    def half(points):
        h = []
        for p in points:
            while (
                len(h) >= 2
                and (
                    (h[-1][0] - h[-2][0]) * (p[1] - h[-2][1])
                    - (h[-1][1] - h[-2][1]) * (p[0] - h[-2][0])
                )
                <= 0
            ):
                h.pop()
            h.append(p)
        return h

    lower = half(P)
    upper = half(P[::-1])
    return resample_closed(np.array(lower[:-1] + upper[:-1]), 160)


def build_rear_derailleur(part, upper, lower, cog_y):
    """Rear derailleur (in Frame): direct-mount B-knuckle, parallelogram links, P-knuckle,
    a slim cage with the toothed pulleys showing past its plates."""
    up2, lo2 = np.array(upper, float), np.array(lower, float)
    ra = np.array([G.RA[0], G.RA[2]])
    for c in (up2, lo2):
        out = sprocket_outline(PULLEY_TEETH, 0.0, roller=7.6)
        part.add(
            filled_shape(
                [out, circle_pts(5.2, 32)], 2.6, PLASTIC, mat4(R_XZ, (c[0], cog_y, c[1])), bevel=0.4
            )
        )
        part.add(
            cylinder(xz(c[0], c[1], cog_y - 4.4), xz(c[0], c[1], cog_y + 4.2), 5.6, ALLOY, n=32)
        )
        part.add(
            cylinder(xz(c[0], c[1], cog_y - 8.6), xz(c[0], c[1], cog_y - 7.0), 6.2, ALLOY, n=32)
        )
    # cage: outer plate (carbon) runs up past the upper pulley to the cage pivot
    pivot = up2 + np.array([5.0, 19.0])
    outer = circle_hull([(up2, 13.5), (lo2, 15.0), (pivot, 10.0)])
    part.add(filled_shape([outer], 2.2, CARBON, mat4(R_XZ, (0, cog_y - 6.0, 0)), bevel=0.7))
    inner = circle_hull([(up2, 11.0), (lo2, 12.5)])
    part.add(filled_shape([inner], 1.8, PLASTIC, mat4(R_XZ, (0, cog_y + 5.6, 0)), bevel=0.6))
    # P-knuckle over the cage pivot
    pk = xz(pivot[0], pivot[1], 0)
    part.add(cylinder(pk + cog_y * Y - 6.5 * Y, pk + cog_y * Y - 11.0 * Y, 6.5, ALLOY, n=32))
    part.add(
        tube(
            [pk + (cog_y - 10) * Y, pk + (cog_y - 30) * Y],
            lambda si, i: superellipse(lerp(26, 30, si), lerp(22, 24, si), 3.0),
            PLASTIC,
            16,
            hint=Z,
        )
    )
    # B-knuckle on the direct-mount hanger, below and behind the axle
    bk = xz(ra[0] - 14, ra[1] - 28, 0)
    hanger = [G.RA + xz(2, -4, -80.5), bk + xz(4, 6, -83.0)]
    part.add(
        tube(hanger, lambda si, i: superellipse(lerp(26, 22, si), 7.0, 3.0), PLASTIC, 16, hint=X)
    )
    part.add(
        tube(
            [bk - 82 * Y, bk - 100 * Y],
            lambda si, i: superellipse(30, 27, 3.0),
            PLASTIC,
            16,
            hint=Z,
        )
    )
    # parallelogram: a wide outer link and a narrower inner one, B-knuckle to P-knuckle
    b_out, p_out = bk + xz(-3, -2, -97), pk + xz(2, 2, cog_y - 27)
    b_in, p_in = bk + xz(5, -6, -86), pk + xz(6, -2, cog_y - 14)
    d = unit(p_out - b_out)
    part.add(
        tube(
            [b_out, p_out],
            lambda si, i: superellipse(lerp(28, 24, si), 7.5, 3.4),
            PLASTIC,
            20,
            hint=lead_perp(d, Z),
        )
    )
    d = unit(p_in - b_in)
    part.add(
        tube(
            [b_in, p_in], lambda si, i: superellipse(16, 7.0, 3.4), ALLOY, 20, hint=lead_perp(d, Z)
        )
    )
    for p, r in ((b_out, 4.5), (p_out, 4.5)):
        part.add(cylinder(p - 4.5 * Y, p + 4.5 * Y, r, ALLOY, n=24))


def build_front_derailleur(part):
    ring_r = sprocket_radius(CHAINRINGS[0]) + 6.0
    a0, a1 = rad(102), rad(134)
    # outer plate (taller) and inner plate, around the big ring's top-rear
    outer = ring_band(ring_r + 20, ring_r + 2, a0, a1)
    inner = ring_band(ring_r + 12, ring_r + 2, a0 + 0.06, a1)
    part.add(filled_shape([outer], 2.0, CARBON, mat4(R_XZ, (G.BB[0], -53.0, G.BB[2])), bevel=0.6))
    part.add(filled_shape([inner], 1.6, ALLOY, mat4(R_XZ, (G.BB[0], -35.5, G.BB[2])), bevel=0.5))
    # rear bridge between the plates
    pb = G.BB + xz(*(polar(ring_r + 10, a1 - 0.03)))
    part.add(cylinder(pb + xz(0, 0, -53), pb + xz(0, 0, -35.5), 4.0, ALLOY, n=32))
    # motor body on the seat tube's drive side, and its link to the outer plate
    st_pt = G.BB + 150 * G.st + 22 * G.st_fwd
    body = [st_pt + xz(2, -18, -20), st_pt + xz(6, 6, -30)]
    part.add(tube(body, lambda si, i: superellipse(30, 26, 3.5), PLASTIC, 16, hint=X))
    link = [st_pt + xz(8, -10, -34), G.BB + xz(*(polar(ring_r + 16, rad(112)))) + xz(0, 0, -50)]
    part.add(tube(link, lambda si, i: superellipse(14, 8, 3), PLASTIC, 16, hint=X))


def build_saddle(part):
    """Saddle shell with a relief channel, padded edges and twin rails; seatpost and head."""
    L, _Wd = SADDLE_LENGTH, SADDLE_WIDTH
    xs = np.linspace(-L / 2, L / 2, 90)
    wx = np.interp(
        xs,
        [-135, -131, -122, -100, -75, -40, -5, 30, 70, 110, 128, 135],
        [70, 112, 132, 140, 136, 112, 78, 52, 42, 38, 28, 4],
    )
    zc = np.interp(xs, [-135, -110, -60, 0, 60, 135], [7, 4, 0, -1.5, -2, -5])
    th = np.interp(xs, [-135, -100, 0, 100, 135], [20, 22, 18, 14, 10])
    n = 64
    t = np.pi - np.linspace(0, 2 * np.pi, n, endpoint=False)  # start at -y?  (profile a=chord axis)
    profs = []
    for i, x in enumerate(xs):
        w = max(wx[i], 2.0)
        c, s = np.cos(t), np.sin(t)
        top = s >= 0
        yy = np.where(
            top,
            w / 2 * np.sign(c) * np.abs(c) ** (2 / 4.5),
            w / 2 * np.sign(c) * np.abs(c) ** (2 / 3.0) * (1 - 0.08 * np.abs(s)),
        )
        drop = 9.0 * min(1.0, w / 60)
        zt = zc[i] - drop * (1 - np.abs(s) ** (2 / 4.5))
        zb = (
            zc[i]
            - drop
            - (th[i] - drop) * np.abs(s) ** (2 / 3.0)
            + 5.0 * (1 - (yy / (w / 2)) ** 2) * np.abs(s)
        )
        zz = np.where(top, zt, zb)
        # pressure relief channel down the middle
        chan = (
            4.0
            * np.exp(-((yy / 8.0) ** 2))
            * smoothstep(-110, -80, x)
            * (1 - smoothstep(30, 70, x))
        )
        zz = zz - np.where(top, chan, 0.0)
        profs.append(np.stack([zz, yy], 1))
    C = np.stack([xs, np.zeros_like(xs), np.zeros_like(xs)], 1)
    T = np.broadcast_to(X, C.shape)
    A = np.broadcast_to(Z, C.shape)
    loc = G.saddle
    pc = loft(C, A, np.array(profs), PLASTIC, T=T)
    pc.transform(mat4(None, loc))
    part.add(pc)
    # rails
    for side in (-1, 1):
        rail = [
            xz(112, -8, side * 12),
            xz(80, -26, side * 19),
            xz(40, -33, side * 22),
            xz(-55, -33, side * 22),
            xz(-92, -24, side * 24),
            xz(-112, -10, side * 28),
        ]
        rp = tube(rail, lambda si, i: circle(7.0, 32), CARBON, 60, hint=Z)
        rp.transform(mat4(None, loc))
        part.add(rp)
    # seatpost: aero D section from the seat tube to the head under the rails
    top_len = SADDLE_HEIGHT - 47.0
    sp_a = G.BB + (SEAT_TUBE + 0.5) * G.st
    sp_b = G.BB + top_len * G.st
    C = spline([sp_a, sp_b], 20)
    A = np.broadcast_to(G.st_fwd, C.shape)
    T = np.broadcast_to(G.st, C.shape)
    part.add(loft(C, A, kamm(44, 24, trunc=0.84, corner=0.45, lead=0.55), PAINT, T=T))
    head = [loc + xz(-30, -42, 0), loc + xz(30, -42, 0)]
    part.add(tube(head, lambda si, i: superellipse(18, 52, 3.5), PAINT, 20, hint=Z))


def build_frame(chain_info):
    part = Part("Frame")
    body = build_frame_body()
    part.add(apply_decal(body), orient=True)
    # BB cups
    for side in (-1, 1):
        part.add(
            cylinder(
                G.BB + xz(0, 0, side * 43.0), G.BB + xz(0, 0, side * 46.5), 21.0, PLASTIC, n=64
            )
        )
    # rear thru-axle ends
    part.add(cylinder(G.RA + xz(0, 0, 79.5), G.RA + xz(0, 0, 84.0), 9.5, ALLOY, n=48))
    part.add(cylinder(G.RA + xz(0, 0, -79.5), G.RA + xz(0, 0, -82.0), 8.0, ALLOY, n=48))
    # rear caliper on the non-drive chainstay
    part.add(caliper(G.RA, 18.0, REAR_ROTOR_Y, ROTOR_DIAMETER / 2))
    build_front_derailleur(part)
    _angle, _pins, circles, (up, lo) = chain_info
    build_rear_derailleur(part, up, lo, circles[1][2])
    build_saddle(part)
    return part


# ---------------------------------------------------------------------------------------------
# Fork and cockpit
# ---------------------------------------------------------------------------------------------


def bar_path(n=260):
    """The handlebar's centreline from the left drop end to the right drop end (bike frame)."""
    BR, BD, HW = BAR_REACH, BAR_DROP, BAR_WIDTH / 2
    half = [
        (0, 0, 0),
        (2, 60, 0),
        (6, 120, 0.5),
        (20, 165, 0),
        (50, HW - 9, -4),
        (BR - 9, HW, -15),
        (BR, HW + 3, -45),
        (BR - 12, HW + 7, -BD + 14),
        (BR - 52, HW + 10, -BD),
        (BR - 132, HW + BAR_FLARE, -BD + 4),
    ]
    half = np.array(half, float)
    pts = np.vstack([half[::-1] * [1, -1, 1], half[1:]])
    C = spline(pts, n)
    return C + G.BC


def build_cockpit(part):
    """One-piece bar and stem, spacers and cover, hoods, levers, tape (all steer with the fork)."""
    pcs = []
    # spacers + headset cover (painted, separate)
    sp_a = G.HTT + 0.5 * G.up
    sp_b = G.stem_base - 19.0 * G.up
    C = spline([sp_a, sp_b], 8)
    part.add(
        loft(
            C,
            np.broadcast_to(G.fwd, C.shape),
            kamm(60, 46, lead=0.40, trunc=0.80),
            PAINT,
            T=np.broadcast_to(G.up, C.shape),
        )
    )
    # steerer cap: carries the head tube's section up round the steerer, then the stem beam
    cap_a = G.stem_base - 19.5 * G.up
    cap_b = G.stem_base + 18.0 * G.up
    C = spline([cap_a, cap_b], 16)
    sc_ = np.linspace(0, 1, len(C))
    prof = [
        kamm(60, 46, lead=0.40, trunc=0.80) * (1.0 - 0.18 * smoothstep(0.7, 1.0, si) ** 2)
        for si in sc_
    ]
    pcs.append(
        loft(
            C,
            np.broadcast_to(G.fwd, C.shape),
            np.array(prof),
            CARBON,
            T=np.broadcast_to(G.up, C.shape),
        )
    )
    st_a = G.stem_base
    st_b = G.BC + 2 * G.stem_dir
    pcs.append(
        tube(
            [st_a, st_b],
            lambda si, i: superellipse(
                lerp(36, 30, si) - 13 * smoothstep(0.7, 1.0, si), lerp(34, 40, si), 2.8
            ),
            CARBON,
            30,
            hint=lead_perp(G.stem_dir, G.up),
        )
    )
    C = bar_path()
    T = tangents(C)
    # wing tops between |y| < 120, round from |y| > 150; tape from |y| > 146
    yrel = np.abs(C[:, 1])
    wing = 1 - smoothstep(110, 150, yrel)
    centre = np.argmin(yrel)
    A = frames_rmf(C, T, X)
    # make the frames symmetric: rebuild from the centre outward on each side
    A[centre:] = frames_rmf(C[centre:], T[centre:], X)
    A[: centre + 1] = frames_rmf(C[: centre + 1][::-1], T[: centre + 1][::-1], X)[::-1]
    bar_prof = [
        lerp(circle(23.8), kamm(46, 17, trunc=0.86, corner=0.6, lead=0.45), w) for w in wing
    ]
    keep = yrel < 152
    # split into the centre bar section (remeshed with the stem)
    idx = np.nonzero(keep)[0]
    pcs.append(loft(C[idx], A[idx], np.array(bar_prof)[idx], CARBON, T=T[idx]))
    body = union_remesh(
        pcs,
        VOXEL_COCKPIT,
        CARBON,
        [G.BC, G.stem_base + 12 * G.fwd],
        fillet_radius=40,
        fillet_iters=10,
        ratio=0.35,
    )
    part.add(body)
    # bar tape: helically wrapped, with the overlap step modelled
    for side in (-1, 1):
        sel = np.nonzero(side * C[:, 1] > 146)[0]
        Cs = C[sel]
        if side < 0:
            Cs = Cs[::-1]
        Cs = spline(Cs[::3], 420)
        Ts = tangents(Cs)
        As = frames_rmf(Cs, Ts, Z)
        s = np.concatenate([[0], np.cumsum(np.linalg.norm(np.diff(Cs, axis=0), axis=1))])
        n = 56
        base = circle(26.4, n)
        ang = np.arctan2(base[:, 1], base[:, 0])
        pitch = 21.0
        profs = []
        for si in s:
            ph = (si / pitch + side * ang / (2 * np.pi)) % 1.0
            step = 0.38 * (ph**1.5) - 0.12 * smoothstep(0.0, 0.08, ph) * (
                1 - smoothstep(0.08, 0.2, ph)
            )
            r = 13.2 + step
            if si < 2.0:
                r = r * 0.92
            if s[-1] - si < 3.0:
                r = r * (0.88 + 0.12 * (s[-1] - si) / 3.0)
            profs.append(np.stack([np.cos(ang) * r, np.sin(ang) * r], 1))
        part.add(loft(Cs, As, np.array(profs), TAPE, uv_scale=24.0, T=Ts))
        # finishing band at the tape's top end
        part.add(cylinder(Cs[0] - 1.0 * Ts[0], Cs[0] + 9.0 * Ts[0], 13.9, TAPE, n=56))
        # hood: body over the bar's front bend, horn up front; lever blade down the drop
        hw = BAR_WIDTH / 2
        base_pt = G.BC + xz(BAR_REACH - 22, 0, side * hw)
        xs = np.linspace(-14, 73, 56)
        kx = [-14, 0, 20, 40, 55, 63, 69, 73]
        ztop = np.interp(xs, kx, [8, 11, 15, 21, 30, 38, 40, 33])
        zbot = np.interp(xs, kx, [-9, -12, -16, -21, -22, -16, 2, 22])
        wid = np.interp(xs, kx, [24, 26, 27, 25, 22, 18, 12, 4])
        profs = []
        for i in range(len(xs)):
            h = ztop[i] - zbot[i]
            pr = superellipse(h, wid[i], 3.0, 48)
            pr[:, 0] += 0.5 * (ztop[i] + zbot[i])
            profs.append(pr)
        Ch = base_pt + np.stack([xs, np.full_like(xs, 0), np.zeros_like(xs)], 1) * [1, 0, 1]
        Ch[:, 1] += side * np.interp(xs, [-14, 70], [0, -6])
        Ah = np.broadcast_to(Z, Ch.shape)
        part.add(loft(Ch, Ah, np.array(profs), PLASTIC, T=np.broadcast_to(X, Ch.shape)))
        lever = [
            base_pt + xz(60, -14, side * -4),
            base_pt + xz(66, -50, side * -2),
            base_pt + xz(50, -95, side * 3),
            base_pt + xz(26, -116, side * 6),
        ]
        part.add(
            tube(
                lever,
                lambda si, i: kamm(lerp(18, 14, si), lerp(10, 8, si), trunc=0.9, corner=0.6),
                PLASTIC,
                40,
                hint=-X,
            )
        )
    # cap the drop ends
    return part


def build_fork_body():
    pcs = []
    crown_c = G.HTB - 18 * G.up + 4 * G.fwd
    joints = [crown_c]
    for side in (-1, 1):
        top = crown_c + 38 * side * Y - 2 * G.up
        bot = G.FA + side * 54 * Y + 4 * G.up
        mid = 0.5 * (top + bot) + 3 * G.fwd + side * 3 * Y
        d = unit(bot - top)

        def fb_prof(si, i):
            return kamm(lerp(44, 26, si), lerp(24, 14, si), trunc=0.8, corner=0.45)

        pcs.append(tube([top, mid, bot], fb_prof, PAINT, 60, hint=lead_perp(d, X)))
        pcs.append(cylinder(G.FA + side * 50.0 * Y, G.FA + side * 58.5 * Y, 11.5, PAINT))
        joints.append(top)
        joints.append(G.FA + side * 54 * Y + 8 * G.up)
    # crown: a wing across the top of the blades
    yy = np.linspace(-50, 50, 41)
    C = crown_c + yy[:, None] * Y
    t = np.abs(yy) / 50
    profs = [
        kamm(lerp(58, 44, ti**2), lerp(30, 20, ti**2), trunc=0.82, corner=0.5, lead=0.45)
        * math.sqrt(max(1 - ti**6, 0.05))
        for ti in t
    ]
    pcs.append(
        loft(
            C,
            np.broadcast_to(G.fwd, C.shape),
            np.array(profs),
            PAINT,
            T=np.broadcast_to(Y, C.shape),
        )
    )
    return union_remesh(
        pcs, VOXEL_FORK, PAINT, joints, fillet_radius=26, fillet_iters=10, ratio=0.35
    )


def build_fork():
    part = Part("Fork")
    part.add(build_fork_body())
    build_cockpit(part)
    part.add(caliper(G.FA, 150.0, FRONT_ROTOR_Y, ROTOR_DIAMETER / 2))
    part.add(cylinder(G.FA + 58.5 * Y, G.FA + 63.0 * Y, 9.5, ALLOY, n=48))
    part.add(cylinder(G.FA - 58.5 * Y, G.FA - 61.0 * Y, 8.0, ALLOY, n=48))
    return part


# ---------------------------------------------------------------------------------------------
# Wheels
# ---------------------------------------------------------------------------------------------


def rim_tyre_profiles():
    rb = BSD / 2
    hook_r = rb + 5.5
    hw = RIM_INNER_WIDTH / 2 + 2.5
    ow = RIM_OUTER_WIDTH / 2
    bed = hook_r - RIM_DEPTH
    side = np.array(
        [
            (hook_r, -hw),
            (hook_r - 1.2, -hw - 1.3),
            (hook_r - 4.0, -ow + 0.2),
            (hook_r - 16, -ow),
            (hook_r - 28, -ow + 0.5),
            (bed + 13, -ow + 1.8),
            (bed + 6, -ow + 4.2),
            (bed + 1.8, -7.0),
            (bed + 0.3, -3.5),
            (bed, 0.0),
        ]
    )
    half = spline(side, 34)
    rim = np.vstack([half, half[-2::-1] * [1, -1]])
    tr = WHEEL_DIAMETER / 2
    cr = tr - TYRE_WIDTH / 2
    psi = np.linspace(rad(-25), rad(205), 72)
    tyre = np.stack([cr + (tr - cr) * np.sin(psi), (TYRE_WIDTH / 2 + 0.6) * np.cos(psi)], 1)
    tyre = np.vstack(
        [
            [(hook_r, hw)],
            [(hook_r + 1.6, hw + 0.6)],
            tyre,
            [(hook_r + 1.6, -hw - 0.6)],
            [(hook_r, -hw)],
        ]
    )
    tyre = spline(tyre, 80)
    return rim, tyre, bed


def hub_profile(front):
    if front:
        e = OLD_FRONT / 2
        fl_r, fl_l = -34.0, 36.0
        rot = FRONT_ROTOR_Y
    else:
        e = OLD_REAR / 2
        fl_r, fl_l = -20.0, 33.0
        rot = REAR_ROTOR_Y
    pts = [
        (6.0, -e),
        (10.0, -e),
        (10.5, -e + 3),
        (13.0, -e + 6),
    ]
    if not front:
        pts += [(17.0, -e + 5.5), (17.0, fl_r - 3.0)]  # freehub body under the cassette
    pts += [
        (16.0, fl_r - 5.5),
        (27.0, fl_r - 4.0),
        (28.5, fl_r),
        (27.0, fl_r + 4.0),
        (16.0, fl_r + 6),
        (15.0, 0.5 * (fl_r + fl_l)),
        (16.0, fl_l - 6),
        (27.0, fl_l - 4.0),
        (28.5, fl_l),
        (27.0, fl_l + 4.0),
        (17.0, fl_l + 5.5),
        (19.5, rot - 4.0),
        (19.5, rot - 1.0),
        (14.0, rot + 1.0),
        (12.0, e - 4),
        (10.5, e - 3),
        (10.0, e),
        (6.0, e),
    ]
    return np.array(pts), (fl_r, fl_l)


def rotor_shape(phase=0.0):
    r_out = ROTOR_DIAMETER / 2
    outer = circle_pts(r_out, 240)
    holes = [circle_pts(17.5, 48)]
    for k in range(6):
        a = phase + k * math.pi / 3
        # cut-outs between six curved arms
        a0, a1 = a + 0.17, a + math.pi / 3 - 0.17
        pts_in = [polar(27, x) for x in np.linspace(a0, a1, 10)]
        pts_out = [polar(r_out - 19, x + 0.22) for x in np.linspace(a1, a0, 12)]
        holes.append(chaikin(np.array(pts_in + pts_out), 3))
    # braking-band holes
    for _row, (rr, dd, n, off) in enumerate(
        ((r_out - 12.5, 4.6, 36, 0.0), (r_out - 6.5, 4.0, 36, 0.5))
    ):
        for k in range(n):
            a = phase + (k + off) * 2 * math.pi / n
            c = polar(rr, a)
            holes.append(circle_pts(dd / 2, 14, 0, c[0], c[1]))
    return [outer, *holes]


def build_wheel(front, cassette_phase=None):
    name = "WheelFront" if front else "WheelRear"
    part = Part(name)
    centre = G.FA if front else G.RA
    O = np.zeros(3)  # build at the origin, place later
    rim, tyre, bed = rim_tyre_profiles()
    pr = revolve(rim, CARBON, O, closed=False, uv_scale=32.0)
    pt = revolve(tyre, RUBBER, O, closed=False, u_tiles=100)
    pr.merge(pt)
    part.add(pr)
    hub, (fl_r, fl_l) = hub_profile(front)
    part.add(revolve(hub, PLASTIC, O, seg=96, closed=True))
    # spokes
    n = SPOKES_FRONT if front else SPOKES_REAR
    fl_rad = 26.0
    for k in range(n):
        th = (k + 0.5) * 2 * math.pi / n
        left = k % 2 == 0
        m = k // 2
        cross = CROSS_FRONT if front else (CROSS_REAR_NON_DRIVE if left else CROSS_REAR_DRIVE)
        sign = 1 if m % 2 == 0 else -1
        hub_th = th + sign * cross * 2 * math.pi / (n / 2) if cross else th
        fy = fl_l if left else fl_r
        if cross:
            fy += (1.6 if sign > 0 else -1.6) * (1 if left else -1)
        rim_y = 2.4 if left else -2.4
        p_hub = np.array([fl_rad * math.cos(hub_th), fy, fl_rad * math.sin(hub_th)])
        p_rim = np.array([(bed - 9.0) * math.cos(th), rim_y, (bed - 9.0) * math.sin(th)])
        d = unit(p_rim - p_hub)
        part.add(cylinder(p_hub - 1.0 * d, p_rim, SPOKE_DIAMETER / 2, ALLOY, n=32))
        part.add(cylinder(p_hub - 1.0 * d, p_hub + 4.0 * d, 1.9, ALLOY, n=32))  # head
        p_n0 = np.array([(bed + 1.5) * math.cos(th), rim_y * 0.6, (bed + 1.5) * math.sin(th)])
        part.add(cylinder(p_n0, p_rim - 1.5 * d, 1.9, ALLOY, n=32, r1=1.6))  # nipple
    # rotor
    ry = FRONT_ROTOR_Y if front else REAR_ROTOR_Y
    part.add(filled_shape(rotor_shape(), 1.8, ALLOY, mat4(R_XZ, (0, ry, 0)), bevel=0.25))
    # rotor bolts / centre-lock ring
    part.add(
        revolve(
            np.array([(17.5, ry + 0.9), (25.0, ry + 0.9), (25.0, ry + 3.5), (17.5, ry + 3.5)]),
            ALLOY,
            O,
            seg=96,
            closed=True,
        )
    )
    if not front:
        for k, teeth in enumerate(CASSETTE):
            y = SMALL_COG_Y + COG_STEP * k
            ph = cassette_phase if (teeth == GEAR[1] and cassette_phase is not None) else 0.37 * k
            out = sprocket_outline(teeth, ph)
            holes = [circle_pts(17.6, 64)]
            R = sprocket_radius(teeth)
            if teeth >= 19:
                for j in range(5):
                    a = j * 2 * math.pi / 5 + 0.1 * k
                    holes.append(ring_band(R - 7.5, 24.5, a + 0.2, a + 2 * math.pi / 5 - 0.2, 14))
            part.add(filled_shape([out, *holes], 1.6, ALLOY, mat4(R_XZ, (0, y, 0)), bevel=0.25))
        # lockring
        part.add(
            revolve(
                np.array(
                    [
                        (17.0, SMALL_COG_Y - 1.0),
                        (20.5, SMALL_COG_Y - 1.0),
                        (20.5, SMALL_COG_Y - 4.5),
                        (17.0, SMALL_COG_Y - 4.5),
                    ]
                )[::-1],
                ALLOY,
                O,
                seg=96,
                closed=True,
            )
        )
    return part, centre


# ---------------------------------------------------------------------------------------------
# Cranks, pedals, chain
# ---------------------------------------------------------------------------------------------


def build_cranks(ring_phase):
    part = Part("Cranks")
    np.zeros(3)
    a = rad(CRANK_ANGLE)
    for side in (-1, 1):
        ang = a if side < 0 else a + math.pi
        u = np.array([math.cos(ang), 0, math.sin(ang)])
        w = np.array([-math.sin(ang), 0, math.cos(ang)])
        y_root, y_end = side * 50.0, side * (CRANK_FACE_Y - 7.0)
        rs = np.linspace(-22, CRANK_LENGTH + 13.5, 70)
        C = (
            rs[:, None] * u[None, :]
            + np.interp(rs, [-22, 20, CRANK_LENGTH], [y_root, y_root, y_end])[:, None] * Y
        )
        L = rs[-1] - rs[0]
        t = (rs - rs[0]) / L
        profs = []
        for ti, r in zip(t, rs, strict=False):
            wdt = np.interp(
                r, [-22, 0, 25, CRANK_LENGTH - 10, CRANK_LENGTH + 13.5], [40, 44, 34, 25, 27]
            )
            thk = np.interp(r, [-22, 25, CRANK_LENGTH], [17, 17, 14])
            cap = math.sqrt(max(1 - ((2 * ti - 1) ** 16), 0.03))
            profs.append(superellipse(wdt * cap, thk * (0.7 + 0.3 * cap), 3.4, SEG))
        part.add(
            loft(
                C,
                np.broadcast_to(w, C.shape),
                np.array(profs),
                CARBON,
                T=np.broadcast_to(u, C.shape),
            )
        )
        # pedal eye insert
        eye = CRANK_LENGTH * u
        part.add(
            cylinder(
                eye + side * (CRANK_FACE_Y - 1.0) * Y,
                eye + side * (CRANK_FACE_Y + 0.2) * Y,
                8.5,
                ALLOY,
                n=48,
            )
        )
        # crank bolt cap / spindle end
        part.add(cylinder(side * 56.0 * Y, side * 59.0 * Y, 13.0, ALLOY, n=48))
    # spider: four arms behind the outer ring
    u0 = a
    for k in range(4):
        ang = u0 + rad(45) + k * math.pi / 2
        u = np.array([math.cos(ang), 0, math.sin(ang)])
        w = np.array([-math.sin(ang), 0, math.cos(ang)])
        C = np.stack([rr * u + (-42.0) * Y for rr in np.linspace(14, 63, 20)])
        profs = [superellipse(lerp(26, 15, ti), 4.0, 3.0) for ti in np.linspace(0, 1, 20)]
        part.add(
            loft(
                C,
                np.broadcast_to(w, C.shape),
                np.array(profs),
                CARBON,
                T=np.broadcast_to(u, C.shape),
            )
        )
        bolt = 55.0 * u
        part.add(cylinder(bolt - 47.5 * Y, bolt - 49.3 * Y, 4.6, ALLOY, n=32))
        part.add(cylinder(bolt - 36.5 * Y, bolt - 35.2 * Y, 4.0, ALLOY, n=32))
    part.add(cylinder(-36.0 * Y, -48.0 * Y, 22.0, CARBON, n=64))
    # chainrings
    for teeth, y in zip(CHAINRINGS, CHAINRING_Y, strict=False):
        ph = ring_phase if teeth == GEAR[0] else 0.0
        out = sprocket_outline(teeth, ph)
        # inner edge: a band behind the teeth, with four tabs reaching in to the spider bolts
        r_band = sprocket_radius(teeth) - (20.0 if teeth == CHAINRINGS[0] else 13.0)
        th = np.linspace(0, 2 * np.pi, 720, endpoint=False)
        bolt_angles = u0 + rad(45) + np.arange(4) * np.pi / 2
        dd = np.abs((th[:, None] - bolt_angles[None, :] + np.pi) % (2 * np.pi) - np.pi).min(1)
        tab = 1 - smoothstep(rad(7), rad(19), dd)
        rr = r_band - (r_band - 47.0) * tab
        holes = [resample_closed(np.stack([rr * np.cos(th), rr * np.sin(th)], 1), 360)]
        thick = 3.6 if teeth == CHAINRINGS[0] else 3.0
        part.add(filled_shape([out, *holes], thick, ALLOY, mat4(R_XZ, (0, y, 0)), bevel=0.35))
    return part, G.BB


def pedal_part(side):
    """Clipless road pedal. Local frame: origin on the spindle at the crank face, +Y outboard
    for the right pedal (mirrored for the left)."""
    name = "PedalR" if side < 0 else "PedalL"
    part = Part(name)
    s = -1.0 if side < 0 else 1.0
    # spindle collar
    part.add(cylinder(np.zeros(3), s * 14.0 * Y, 8.0, ALLOY, n=48))
    part.add(cylinder(s * 13.0 * Y, s * 22.0 * Y, 6.5, ALLOY, n=48))
    # body
    sec = np.array(
        [
            (34, 0),
            (31, 7),
            (20, 11.5),
            (0, 12.5),
            (-25, 9.5),
            (-47, 6.5),
            (-56, 3),
            (-57, -2),
            (-50, -5.5),
            (-25, -8.5),
            (0, -12.5),
            (20, -12),
            (31, -6),
        ],
        float,
    )
    sec = resample_closed(chaikin(sec, 3), 64)
    # reorder so the profile starts at the rear centre (-x), +b side first: a=z? use (a=x, b=z)
    ys = np.linspace(18, 18 + 2 * PEDAL_BODY_CENTRE - 36, 40)
    t = np.linspace(-1, 1, len(ys))
    profs = []
    for ti in t:
        cap = math.sqrt(max(1 - ti**10, 0.04))
        sx_f = (0.62 + 0.38 * (1 - ti**2)) * cap
        sx_r = (0.85 + 0.15 * (1 - ti**2)) * cap
        x = np.where(sec[:, 0] > 0, sec[:, 0] * sx_f, sec[:, 0] * sx_r)
        z = sec[:, 1] * (0.55 + 0.45 * cap)
        profs.append(np.stack([x, z], 1))
    C = s * ys[:, None] * Y[None, :]
    T = np.broadcast_to(s * Y, C.shape)
    # loft profile coords are (a along A, b along B=T x A); with A=X: B = (sY) x X = -s Z
    profs = np.array(profs)
    profs[:, :, 1] *= -s
    part.add(loft(C, np.broadcast_to(X, C.shape), profs, PLASTIC, T=T))
    # cleat bail plate at the rear
    yc = s * PEDAL_BODY_CENTRE
    bail = np.array(
        [(-58, 2), (-56, 6.5), (-40, 9.8), (-30, 10.2), (-30, 7.5), (-40, 7.2), (-54, 3.5)], float
    )
    part.add(prism(chaikin(bail, 2), yc - 19, yc + 19, ALLOY))
    return part


def build_pedals():
    out = []
    a = rad(CRANK_ANGLE)
    for side in (-1, 1):
        ang = a if side < 0 else a + math.pi
        eye = (
            G.BB
            + CRANK_LENGTH * np.array([math.cos(ang), 0, math.sin(ang)])
            + side * CRANK_FACE_Y * Y
        )
        out.append((pedal_part(side), eye))
    return out


def link_plate(length=CHAIN_PITCH, r_end=4.25, waist=3.1, n=24):
    s = np.concatenate([np.linspace(-np.pi / 2, np.pi / 2, n)])
    pts = []
    # right end cap, top edge, left end cap, bottom edge (anticlockwise)
    for a in np.linspace(-np.pi / 2, np.pi / 2, n):
        pts.append((length / 2 + r_end * math.cos(a), r_end * math.sin(a)))
    for x in np.linspace(length / 2, -length / 2, 18)[1:-1]:
        pts.append((x, waist + (r_end - waist) * (2 * x / length) ** 4))
    for a in np.linspace(np.pi / 2, 3 * np.pi / 2, n):
        pts.append((-length / 2 + r_end * math.cos(a), r_end * math.sin(a)))
    for x in np.linspace(-length / 2, length / 2, 18)[1:-1]:
        pts.append((x, -(waist + (r_end - waist) * (2 * x / length) ** 4)))
    del s
    return np.array(pts)


def build_chain(pins):
    part = Part("Chain")
    inner = link_plate(r_end=4.3, waist=3.2)
    outer = link_plate(r_end=4.0, waist=2.9)
    # template pieces in link space: x along the link, y across, z in the chain plane
    n = len(pins)
    roller = cylinder(xz(0, 0, -1.15), xz(0, 0, 1.15), 3.85, ALLOY, n=24)
    pinhead = [cylinder(xz(0, 0, s * 2.85), xz(0, 0, s * 3.25), 1.85, ALLOY, n=16) for s in (-1, 1)]
    plates = {
        0: [prism(inner, -1.95, -1.15, ALLOY), prism(inner, 1.15, 1.95, ALLOY)],
        1: [prism(outer, -2.85, -2.05, ALLOY), prism(outer, 2.05, 2.85, ALLOY)],
    }
    acc = None
    for i in range(n):
        p0, p1 = pins[i], pins[(i + 1) % n]
        xa = unit(p1 - p0)
        ya = unit(Y - np.dot(Y, xa) * xa)
        za = np.cross(xa, ya)
        R = np.stack([xa, ya, za], 1)
        M = mat4(R, 0.5 * (p0 + p1))
        Mp = mat4(R, p0)
        kind = i % 2
        for tmpl in plates[kind]:
            pc = Piece(tmpl.V.copy())
            pc.groups = [[g[0].copy(), None, g[2]] for g in tmpl.groups]
            pc.transform(M)
            acc = pc if acc is None else acc.merge(pc)
        assert acc is not None
        for tmpl in [roller] + (pinhead if kind == 1 else []):
            pc = Piece(tmpl.V.copy())
            pc.groups = [[g[0].copy(), None, g[2]] for g in tmpl.groups]
            pc.transform(Mp)
            acc.merge(pc)
        if kind == 1:
            for tmpl in pinhead:
                pc = Piece(tmpl.V.copy())
                pc.groups = [[g[0].copy(), None, g[2]] for g in tmpl.groups]
                pc.transform(mat4(R, p1))
                acc.merge(pc)
    part.add(acc, orient=False)
    return part


# =============================================================================================
# Materials and generated textures
# =============================================================================================


def save_png(path, rgb, colorspace="sRGB"):
    h, w = rgb.shape[:2]
    img = bpy.data.images.new(os.path.basename(path), w, h, alpha=False, float_buffer=False)
    px = np.ones((h, w, 4), np.float32)
    px[:, :, :3] = rgb
    img.pixels.foreach_set(px[::-1].ravel())  # Blender rows go bottom-up
    img.filepath_raw = path
    img.file_format = "PNG"
    img.save()
    img.colorspace_settings.name = colorspace
    return img


def normal_from_height(h, strength):
    gx = (np.roll(h, -1, 1) - np.roll(h, 1, 1)) * 0.5 * strength
    gy = (np.roll(h, -1, 0) - np.roll(h, 1, 0)) * 0.5 * strength
    # image rows run top-down here; OpenGL green = +V (up), so dh/dv = -gy
    n = np.stack([-gx, gy, np.ones_like(h)], -1)
    n /= np.linalg.norm(n, axis=-1, keepdims=True)
    return n * 0.5 + 0.5


def texture_carbon(out, size=1024, tows=16):
    """2x2 twill weave of 3K tows: base colour and normal map, tileable."""
    y, x = np.mgrid[0:size, 0:size] / size * tows
    i, j = np.floor(x).astype(int), np.floor(y).astype(int)
    fx, fy = x - i, y - j
    warp_on_top = ((i + j) % 4) < 2
    across = np.where(warp_on_top, fx, fy)
    along = np.where(warp_on_top, fy, fx)
    rng = np.random.default_rng(7)
    streak = rng.normal(0, 1, (size, size))
    # fibre streaks along each tow
    from_streak = (
        np.where(
            warp_on_top,
            np.roll(streak, 1, 0) + streak + np.roll(streak, -1, 0),
            np.roll(streak, 1, 1) + streak + np.roll(streak, -1, 1),
        )
        / 3
    )
    hump = np.sin(np.pi * across) ** 0.6
    dive = 1 - 0.35 * (np.sin(np.pi * along) ** 6 * 0)  # tows are flat along their length
    h = hump * dive + 0.04 * from_streak
    base = np.where(warp_on_top, 0.050, 0.030) * (0.65 + 0.35 * hump) + 0.006 * from_streak
    base = np.clip(base, 0.005, 0.2)
    rgb = np.stack([base, base * 1.02, base * 1.08], -1) ** (1 / 2.2)  # store as sRGB
    save_png(os.path.join(out, "T_Carbon_BaseColor.png"), rgb, "sRGB")
    save_png(os.path.join(out, "T_Carbon_Normal.png"), normal_from_height(h, 2.2), "Non-Color")


def texture_rubber(out, size=1024):
    """Tyre: u runs round the wheel (one tile = 1/100 turn), v across the tyre bead to bead.
    Black tread band with a fine file tread, tan sidewalls, black bead strip at the rim."""
    v = (np.arange(size) + 0.5) / size
    u = (np.arange(size) + 0.5) / size
    U, V = np.meshgrid(u, v)
    dc = np.abs(V - 0.5)
    tread = 1 - smoothstep(0.215, 0.235, dc)
    bead = smoothstep(0.455, 0.47, dc)
    tan = np.array([0.42, 0.25, 0.12])
    black = np.array([0.018, 0.018, 0.018])
    w = np.clip(tread + bead, 0, 1)[..., None]
    rgb = black * w + tan * (1 - w)
    rgb = np.clip(rgb, 0, 1) ** (1 / 2.2)
    # file tread: small diamonds (2 mm round the wheel x 1.2 mm across) on the tread's shoulders
    du, dv = U * 10.6, V * 54.0
    diamonds = np.abs(((du + dv) % 1.0) - 0.5) + np.abs(((du - dv) % 1.0) - 0.5)
    knurl = smoothstep(0.35, 0.55, diamonds) * (smoothstep(0.045, 0.07, dc)) * tread
    # sidewall casing ribs, faint
    ribs = 0.25 * np.sin(V * 2 * np.pi * 140) * (1 - tread) * (1 - bead)
    h = -0.6 * knurl + 0.08 * ribs
    save_png(os.path.join(out, "T_Rubber_BaseColor.png"), rgb, "sRGB")
    save_png(os.path.join(out, "T_Rubber_Normal.png"), normal_from_height(h, 3.0), "Non-Color")


def make_materials(out):
    texture_carbon(out)
    texture_rubber(out)
    mats = {}

    def principled(name, color, metal=0.0, rough=0.5, coat=0.0, coat_rough=0.03, aniso=0.0):
        m = bpy.data.materials.new(name)
        with contextlib.suppress(Exception):
            m.use_nodes = True
        nt = m.node_tree
        b = nt.nodes.get("Principled BSDF")
        b.inputs["Base Color"].default_value = (*color, 1.0)
        b.inputs["Metallic"].default_value = metal
        b.inputs["Roughness"].default_value = rough
        b.inputs["Coat Weight"].default_value = coat
        b.inputs["Coat Roughness"].default_value = coat_rough
        b.inputs["Anisotropic"].default_value = aniso
        m.diffuse_color = (*color, 1.0)
        mats[name] = m
        return m, nt, b

    def image_node(nt, path, noncolor):
        img = bpy.data.images.load(path, check_existing=True)
        if noncolor:
            img.colorspace_settings.name = "Non-Color"
        node = nt.nodes.new("ShaderNodeTexImage")
        node.image = img
        return node

    def add_textures(nt, b, base, normal, strength):
        if base:
            tb = image_node(nt, os.path.join(out, base), False)
            nt.links.new(tb.outputs["Color"], b.inputs["Base Color"])
        if normal:
            tn = image_node(nt, os.path.join(out, normal), True)
            nm = nt.nodes.new("ShaderNodeNormalMap")
            nm.inputs["Strength"].default_value = strength
            nt.links.new(tn.outputs["Color"], nm.inputs["Color"])
            nt.links.new(nm.outputs["Normal"], b.inputs["Normal"])

    principled(PAINT, PAINT_COLOR, metal=0.75, rough=0.32, coat=1.0, coat_rough=0.02)
    principled(DECAL, DECAL_COLOR, metal=0.0, rough=0.35, coat=1.0, coat_rough=0.02)
    _m, nt, b = principled(CARBON, (0.03, 0.03, 0.033), rough=0.32, coat=0.5, coat_rough=0.06)
    add_textures(nt, b, "T_Carbon_BaseColor.png", "T_Carbon_Normal.png", 0.5)
    _m, nt, b = principled(RUBBER, (0.02, 0.02, 0.02), rough=0.78)
    add_textures(nt, b, "T_Rubber_BaseColor.png", "T_Rubber_Normal.png", 0.8)
    principled(ALLOY, (0.80, 0.80, 0.80), metal=1.0, rough=0.28, aniso=0.6)
    principled(PLASTIC, (0.016, 0.016, 0.017), rough=0.55)
    principled(TAPE, (0.012, 0.012, 0.012), rough=0.85)
    return mats


# =============================================================================================
# Blender objects
# =============================================================================================


def set_smooth(me, angle_deg):
    me.shade_smooth()
    npoly = len(me.polygons)
    lt = np.empty(npoly, np.int64)
    me.polygons.foreach_get("loop_total", lt)
    le = np.empty(len(me.loops), np.int64)
    me.loops.foreach_get("edge_index", le)
    lf = np.repeat(np.arange(npoly), lt)
    fn = np.empty(npoly * 3)
    me.polygons.foreach_get("normal", fn)
    fn = fn.reshape(-1, 3)
    order = np.argsort(le, kind="stable")
    es, fs = le[order], lf[order]
    same = es[:-1] == es[1:]
    e, f1, f2 = es[:-1][same], fs[:-1][same], fs[1:][same]
    cosang = (fn[f1] * fn[f2]).sum(1)
    sharp = np.zeros(len(me.edges), bool)
    sharp[e[cosang < math.cos(rad(angle_deg))]] = True
    attr = me.attributes.get("sharp_edge") or me.attributes.new("sharp_edge", "BOOLEAN", "EDGE")
    attr.data.foreach_set("value", sharp)


def make_object(part, mats, M_world, parent):
    """Create the Blender object for a part. M_world (mm) is the part's pivot frame."""
    Minv = np.linalg.inv(M_world)
    verts, faces, uvs, mids = [], [], [], []
    names = []
    off = 0
    for pc in part.pieces:
        V = pc.V @ Minv[:3, :3].T + Minv[:3, 3]
        verts.append(V)
        for F, uv, m in pc.groups:
            if uv is None:
                uv = box_uv(pc.V, F)
            if m not in names:
                names.append(m)
            faces.append(F + off)
            uvs.append(uv.reshape(-1, 2))
            mids.append(np.full(len(F), names.index(m)))
        off += len(V)
    V = np.vstack(verts) * 0.001
    flist = []
    for F in faces:
        flist.extend(F.tolist())
    me = bpy.data.meshes.new(part.name)
    me.from_pydata(V.tolist(), [], flist)
    uvl = me.uv_layers.new(name="UVMap")
    uvl.data.foreach_set("uv", np.vstack(uvs).ravel())
    for nme in names:
        me.materials.append(mats[nme])
    me.polygons.foreach_set("material_index", np.concatenate(mids).astype(np.int32))
    me.validate(clean_customdata=False)
    me.update()
    set_smooth(me, 32.0)
    ob = bpy.data.objects.new(part.name, me)
    bpy.context.scene.collection.objects.link(ob)
    M = Matrix(M_world.tolist())
    M.translation = M.translation * 0.001
    ob.matrix_world = M
    ob.parent = parent
    ob.matrix_parent_inverse = parent.matrix_world.inverted()
    return ob


def tri_count(ob):
    lt = np.empty(len(ob.data.polygons), np.int64)
    ob.data.polygons.foreach_get("loop_total", lt)
    return int((lt - 2).sum())


# =============================================================================================
# Review renders
# =============================================================================================


def setup_render_scene(quick):
    sc = bpy.context.scene
    sc.render.engine = "CYCLES"
    sc.cycles.device = "CPU"
    sc.cycles.samples = 48 if quick else 160
    sc.cycles.use_denoising = True
    sc.render.resolution_x = 1600
    sc.render.resolution_y = 900
    sc.render.resolution_percentage = 50 if quick else 100
    sc.view_settings.view_transform = "AgX"
    sc.view_settings.look = "AgX - Medium High Contrast"
    world = bpy.data.worlds.new("Studio")
    sc.world = world
    nt = world.node_tree
    bg = nt.nodes.get("Background")
    tc = nt.nodes.new("ShaderNodeTexCoord")
    sep = nt.nodes.new("ShaderNodeSeparateXYZ")
    ramp = nt.nodes.new("ShaderNodeValToRGB")
    ramp.color_ramp.elements[0].position = 0.45
    ramp.color_ramp.elements[0].color = (0.10, 0.10, 0.11, 1)
    ramp.color_ramp.elements[1].position = 0.75
    ramp.color_ramp.elements[1].color = (0.85, 0.88, 0.95, 1)
    mapn = nt.nodes.new("ShaderNodeMapRange")
    mapn.inputs["From Min"].default_value = -1
    mapn.inputs["From Max"].default_value = 1
    nt.links.new(tc.outputs["Generated"], sep.inputs[0])
    nt.links.new(sep.outputs["Z"], mapn.inputs["Value"])
    nt.links.new(mapn.outputs["Result"], ramp.inputs["Fac"])
    nt.links.new(ramp.outputs["Color"], bg.inputs["Color"])
    bg.inputs["Strength"].default_value = 0.35
    # floor with a curved backdrop
    bpy.ops.mesh.primitive_plane_add(size=30, location=(0, 0, 0))
    floor = bpy.context.active_object
    floor.name = "_floor"
    fm = bpy.data.materials.new("_floor")
    fb = fm.node_tree.nodes.get("Principled BSDF")
    fb.inputs["Base Color"].default_value = (0.16, 0.16, 0.17, 1)
    fb.inputs["Roughness"].default_value = 0.6
    floor.data.materials.append(fm)

    def area(name, loc, target, size, power, color=(1, 1, 1)):
        ld = bpy.data.lights.new(name, "AREA")
        ld.size = size
        ld.energy = power
        ld.color = color
        lo = bpy.data.objects.new(name, ld)
        sc.collection.objects.link(lo)
        lo.location = loc
        d = np.array(target) - np.array(loc)
        lo.rotation_euler = Matrix.Identity(3).to_euler()
        from mathutils import Vector  # ty: ignore[unresolved-import]

        lo.rotation_euler = Vector(d.tolist()).to_track_quat("-Z", "Y").to_euler()
        return lo

    area("_key", (1.6, -2.4, 2.6), (0.2, 0, 0.5), 2.5, 260)
    area("_fill", (-1.0, 2.6, 1.8), (0.2, 0, 0.5), 3.0, 110)
    area("_rim", (-2.8, -0.6, 1.6), (0.2, 0, 0.6), 1.8, 160, (0.95, 0.97, 1.0))
    area("_top", (0.2, 0.0, 3.2), (0.2, 0, 0.4), 4.0, 120)
    cam_data = bpy.data.cameras.new("_cam")
    cam = bpy.data.objects.new("_cam", cam_data)
    sc.collection.objects.link(cam)
    sc.camera = cam
    return cam, [floor]


def render_views(out, cam, quick):
    from mathutils import Vector  # ty: ignore[unresolved-import]

    views = {
        "render_drive_side": ((0.10, -3.15, 0.58), (0.10, 0.0, 0.52), 50),
        "render_non_drive_side": ((0.10, 3.15, 0.58), (0.10, 0.0, 0.52), 50),
        "render_drivetrain": ((-0.12, -0.95, 0.46), (-0.20, -0.04, 0.30), 50),
        "render_cockpit": ((0.05, -0.42, 1.28), (0.52, 0.0, 0.88), 40),
        "render_chase_cam": ((-1.75, -0.45, 1.55), (0.25, 0.0, 0.55), 40),
    }
    sc = bpy.context.scene
    paths = []
    for name, (loc, tgt, lens) in views.items():
        cam.location = loc
        d = Vector(tgt) - Vector(loc)
        cam.rotation_euler = d.to_track_quat("-Z", "Y").to_euler()
        cam.data.lens = lens
        cam.data.dof.use_dof = False
        sc.render.filepath = os.path.join(out, name + ".png")
        t0 = time.time()
        bpy.ops.render.render(write_still=True)
        print(f"rendered {name} in {time.time() - t0:.0f}s")
        paths.append(sc.render.filepath)
    return paths


# =============================================================================================
# Main
# =============================================================================================


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    out = os.path.dirname(os.path.abspath(__file__))
    opts = {"out": out, "render": True, "quick": False}
    i = 0
    while i < len(argv):
        if argv[i] == "--out":
            opts["out"] = os.path.abspath(argv[i + 1])
            i += 1
        elif argv[i] == "--no-render":
            opts["render"] = False
        elif argv[i] == "--quick":
            opts["quick"] = True
        i += 1
    return opts


def main():
    opts = parse_args()
    out = opts["out"]
    os.makedirs(out, exist_ok=True)
    t_start = time.time()
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    sc.unit_settings.system = "METRIC"
    sc.unit_settings.scale_length = 1.0

    print(f"wheelbase {G.wheelbase:.1f} mm, axle-to-crown {G.axle_to_crown:.1f} mm")
    print(f"front axle {G.FA.round(1)}, rear axle {G.RA.round(1)}, BB {G.BB.round(1)}")

    mats = make_materials(out)
    chain_info = solve_chain()
    angle, pins, _circles, _pul = chain_info
    print(f"chain: {len(pins)} links, cage angle {angle:.2f} deg")
    ring_phase = phase_on(pins, (G.BB[0], G.BB[2]), sprocket_radius(GEAR[0]))
    cog_phase = phase_on(pins, (G.RA[0], G.RA[2]), sprocket_radius(GEAR[1]))

    root = bpy.data.objects.new("Bike", None)
    root.empty_display_type = "ARROWS"
    root.empty_display_size = 0.3
    sc.collection.objects.link(root)

    objs = {}
    t = time.time()
    frame = build_frame(chain_info)
    objs["Frame"] = make_object(frame, mats, mat4(), root)
    print(f"Frame {time.time() - t:.0f}s")
    t = time.time()
    fork = build_fork()
    Rf = np.array(Matrix.Rotation(-rad(90 - HEAD_ANGLE), 3, "Y"))
    objs["Fork"] = make_object(fork, mats, mat4(Rf, G.HTC), root)
    print(f"Fork {time.time() - t:.0f}s")
    t = time.time()
    for front in (True, False):
        part, centre = build_wheel(front, None if front else cog_phase - G.RA[0] * 0)
        # wheel pieces were built at the origin; their pivot is the axle
        for pc in part.pieces:
            pc.transform(mat4(None, centre))
        objs[part.name] = make_object(part, mats, mat4(None, centre), root)
    print(f"Wheels {time.time() - t:.0f}s")
    cr, bb = build_cranks(ring_phase)
    for pc in cr.pieces:
        pc.transform(mat4(None, bb))
    objs["Cranks"] = make_object(cr, mats, mat4(None, bb), root)
    for pedal, eye in build_pedals():
        for pc in pedal.pieces:
            pc.transform(mat4(None, eye))
        objs[pedal.name] = make_object(pedal, mats, mat4(None, eye), root)
    t = time.time()
    objs["Chain"] = make_object(build_chain(pins), mats, mat4(), root)
    print(f"Chain {time.time() - t:.0f}s")

    # sanity: lowest point of each tyre
    for nm in ("WheelFront", "WheelRear"):
        ob = objs[nm]
        zs = [(ob.matrix_world @ v.co).z for v in ob.data.vertices]
        print(f"{nm} lowest z {min(zs) * 1000:.2f} mm")

    stats = {nm: tri_count(ob) for nm, ob in objs.items()}
    stats["_total"] = sum(stats.values())
    pivots = {}
    for nm, ob in objs.items():
        loc = ob.matrix_world.translation
        pivots[nm] = {
            "location_m": [round(loc.x, 5), round(loc.y, 5), round(loc.z, 5)],
            "rotation_deg": [round(math.degrees(a), 3) for a in ob.matrix_world.to_euler()],
        }
    geo = {
        "wheelbase_mm": round(G.wheelbase, 1),
        "axle_to_crown_mm": round(G.axle_to_crown, 1),
        "chain_links": len(pins),
        "rd_cage_angle_deg": round(angle, 2),
        "steering_axis_up": [round(v, 5) for v in G.up],
    }
    with open(os.path.join(out, "stats.json"), "w") as f:
        json.dump({"triangles": stats, "pivots": pivots, "geometry": geo}, f, indent=2)
        f.write("\n")
    print(json.dumps(stats, indent=1))

    # FBX: metres in Blender -> centimetres in UE (UnitScaleFactor 100); +X forward, +Z up kept
    bpy.ops.object.select_all(action="DESELECT")
    root.select_set(True)
    for ob in objs.values():
        ob.select_set(True)
    bpy.context.view_layer.objects.active = root
    bpy.ops.export_scene.fbx(
        filepath=os.path.join(out, "bike.fbx"),
        use_selection=True,
        object_types={"EMPTY", "MESH"},
        apply_unit_scale=True,
        apply_scale_options="FBX_SCALE_ALL",
        # identity axis mapping: the file is Z-up with Blender's coordinates as-is, so every
        # node keeps an identity (or, for the fork, its pure head-angle) rotation
        bake_space_transform=False,
        axis_forward="Y",
        axis_up="Z",
        mesh_smooth_type="FACE",
        use_tspace=True,
        use_mesh_modifiers=True,
        add_leaf_bones=False,
        bake_anim=False,
        path_mode="RELATIVE",
        embed_textures=False,
        use_custom_props=False,
    )
    bpy.ops.wm.save_as_mainfile(filepath=os.path.join(out, "bike.blend"), compress=True)
    print(f"built and exported in {time.time() - t_start:.0f}s")

    if opts["render"]:
        cam, _helpers = setup_render_scene(opts["quick"])
        render_views(out, cam, opts["quick"])
    print(f"done in {time.time() - t_start:.0f}s")


if __name__ == "__main__":
    main()
