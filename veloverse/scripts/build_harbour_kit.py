"""Builds high-craft coastal harbour architecture, boats, piers, and lighthouse in Blender
and writes self-contained glTF 2.0 meshes (`feat_*.gltf`) into OC_SRC (default
`/home/jamison/.cache/opencycle/world_gen/ride_hub` or `/workspace/ue/world_gen/ride_hub`).

Run headless with Blender 5.x:
    blender -b -P scripts/build_harbour_kit.py -- --out /path/to/dir [--render]

All geometry is built in Blender metres (+X forward, +Y left, +Z up), UV-mapped at 1 UV unit
per metre so Unreal's `M_Feature_*` 2K Poly Haven PBR materials tile at true physical scale,
and written via `gltf_writer.py` (converting to UE cm +X forward, +Y right, +Z up).
"""

import argparse
import contextlib
import math
import os
import sys

import bmesh  # ty: ignore[unresolved-import]
import bpy  # ty: ignore[unresolved-import]
from mathutils import Matrix, Vector  # ty: ignore[unresolved-import]

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import gltf_writer  # noqa: E402

# ---------------------------------------------------------------------------------------------
# Mesh builder helpers (Blender bmesh in metres -> gltf_writer in UE cm)
# ---------------------------------------------------------------------------------------------


def clear_scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)


def make_mat(name, color=(0.5, 0.5, 0.5), rough=0.7, metal=0.0, emit=0.0):
    m = bpy.data.materials.get(name)
    if m is not None:
        return m
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    bsdf = m.node_tree.nodes.get("Principled BSDF")
    if bsdf:
        bsdf.inputs["Base Color"].default_value = (*color, 1.0)
        bsdf.inputs["Roughness"].default_value = rough
        bsdf.inputs["Metallic"].default_value = metal
        if emit > 0 and "Emission Strength" in bsdf.inputs:
            bsdf.inputs["Emission Color"].default_value = (*color, 1.0)
            bsdf.inputs["Emission Strength"].default_value = emit
    return m


# Preview colours matching make_features.py PAINTS + TEXTURES
MAT_SPECS = {
    "stone": ((0.36, 0.34, 0.31), 0.85, 0.0, 0.0),
    "plaster": ((0.76, 0.73, 0.67), 0.82, 0.0, 0.0),
    "ochre": ((0.78, 0.62, 0.42), 0.82, 0.0, 0.0),
    "slate": ((0.22, 0.24, 0.28), 0.72, 0.0, 0.0),
    "clay": ((0.52, 0.24, 0.16), 0.78, 0.0, 0.0),
    "siding": ((0.44, 0.38, 0.31), 0.86, 0.0, 0.0),
    "boards": ((0.18, 0.12, 0.08), 0.84, 0.0, 0.0),
    "concrete": ((0.42, 0.41, 0.38), 0.85, 0.0, 0.0),
    "wood": ((0.22, 0.15, 0.10), 0.88, 0.0, 0.0),
    "deck": ((0.34, 0.26, 0.18), 0.84, 0.0, 0.0),
    "white": ((0.80, 0.81, 0.79), 0.45, 0.0, 0.0),
    "red": ((0.46, 0.05, 0.04), 0.50, 0.0, 0.0),
    "navy": ((0.04, 0.08, 0.19), 0.45, 0.0, 0.0),
    "green": ((0.05, 0.18, 0.09), 0.52, 0.0, 0.0),
    "steel": ((0.26, 0.28, 0.30), 0.42, 0.85, 0.0),
    "cable": ((0.08, 0.08, 0.08), 0.60, 0.30, 0.0),
    "glass": ((0.05, 0.08, 0.10), 0.08, 0.10, 0.0),
    "warm_glass": ((0.95, 0.68, 0.32), 0.15, 0.0, 2.5),
    "copper": ((0.18, 0.34, 0.28), 0.42, 0.72, 0.0),
}


class KitBuilder:
    """Accumulates tagged bmesh geometry in Blender metres, computes box-projected metre UVs,
    and writes a UE-ready glTF via gltf_writer."""

    def __init__(self, name):
        self.name = name
        self.bm = bmesh.new()
        self.mat_names = []
        self.mat_map = {}

    def mat_idx(self, name):
        if name not in self.mat_map:
            self.mat_map[name] = len(self.mat_names)
            self.mat_names.append(name)
        return self.mat_map[name]

    def _tag(self, faces, mat_name, smooth=False):
        idx = self.mat_idx(mat_name)
        for f in faces:
            f.material_index = idx
            f.smooth = smooth

    def box(self, mat, center, size, rot=(0.0, 0.0, 0.0), bevel=0.0):
        sx, sy, sz = size
        m = (
            Matrix.Translation(center)
            @ Matrix.Rotation(math.radians(rot[2]), 4, "Z")
            @ Matrix.Rotation(math.radians(rot[1]), 4, "Y")
            @ Matrix.Rotation(math.radians(rot[0]), 4, "X")
            @ Matrix.Diagonal((sx, sy, sz, 1.0))
        )
        res = bmesh.ops.create_cube(self.bm, size=1.0, matrix=m)
        verts = res["verts"]
        faces = list({f for v in verts for f in v.link_faces})
        if bevel > 0.0:
            edges = list({e for f in faces for e in f.edges})
            bev = bmesh.ops.bevel(
                self.bm,
                geom=edges,
                offset=bevel,
                segments=1,
                affect="EDGES",
                clamp_overlap=True,
            )
            faces = list({f for f in faces if f.is_valid} | set(bev["faces"]))
        self._tag(faces, mat, smooth=False)
        return faces

    def cyl(self, mat, p0, p1, r0, r1=None, seg=12, cap=True, smooth=True):
        if r1 is None:
            r1 = r0
        a, b = Vector(p0), Vector(p1)
        d = b - a
        length = d.length
        if length < 1e-5:
            return []
        mid = (a + b) * 0.5
        quat = Vector((0, 0, 1)).rotation_difference(d.normalized())
        m = Matrix.Translation(mid) @ quat.to_matrix().to_4x4()
        res = bmesh.ops.create_cone(
            self.bm,
            cap_ends=cap,
            cap_tris=False,
            segments=seg,
            radius1=r0,
            radius2=r1,
            depth=length,
            matrix=m,
        )
        verts = res["verts"]
        faces = list({f for v in verts for f in v.link_faces})
        self._tag(faces, mat, smooth=smooth)
        return faces

    def prism_gable(self, mat, cx, cy, z_base, w, d, rise, axis="X"):
        """Triangular gable end walls (solid prism along X or Y)."""
        hx, hy = w * 0.5, d * 0.5
        if axis == "X":
            pts = [
                (cx - hx, cy - hy, z_base),
                (cx - hx, cy + hy, z_base),
                (cx - hx, cy, z_base + rise),
                (cx + hx, cy - hy, z_base),
                (cx + hx, cy + hy, z_base),
                (cx + hx, cy, z_base + rise),
            ]
        else:
            pts = [
                (cx - hx, cy - hy, z_base),
                (cx + hx, cy - hy, z_base),
                (cx, cy - hy, z_base + rise),
                (cx - hx, cy + hy, z_base),
                (cx + hx, cy + hy, z_base),
                (cx, cy + hy, z_base + rise),
            ]
        vs = [self.bm.verts.new(p) for p in pts]
        faces = []
        for idx in (
            (0, 2, 1),
            (3, 4, 5),
            (0, 1, 4, 3),
            (1, 2, 5, 4),
            (2, 0, 3, 5),
        ):
            faces.append(self.bm.faces.new([vs[i] for i in idx]))
        bmesh.ops.recalc_face_normals(self.bm, faces=faces)
        self._tag(faces, mat, smooth=False)
        return faces

    def roof_pitched(
        self,
        roof_mat,
        trim_mat,
        cx,
        cy,
        z_eave,
        w,
        d,
        pitch_deg,
        over_x=0.45,
        over_y=0.55,
        thick=0.18,
    ):
        """Pitched roof with overhanging slabs, ridge beam, fascia, bargeboards, and rafters."""
        pitch = math.radians(pitch_deg)
        rise = (d * 0.5) * math.tan(pitch)
        span_y = d * 0.5 + over_y
        slope_len = span_y / math.cos(pitch)
        full_x = w + 2.0 * over_x
        ridge_z = z_eave + rise

        # Two sloped roof slabs
        for sy in (-1.0, 1.0):
            mid_y = cy + sy * (span_y * 0.5)
            mid_z = ridge_z - (span_y * 0.5) * math.tan(pitch) + thick * 0.5 / math.cos(pitch)
            self.box(
                roof_mat,
                (cx, mid_y, mid_z),
                (full_x, slope_len, thick),
                rot=(-sy * pitch_deg, 0.0, 0.0),
            )
            # Exposed eave fascia board
            ey = cy + sy * span_y
            ez = ridge_z - span_y * math.tan(pitch)
            self.box(trim_mat, (cx, ey, ez), (full_x + 0.06, 0.06, 0.22))
            # Exposed rafter tails along the eaves
            n_rafters = max(4, int(full_x / 0.85))
            for i in range(n_rafters + 1):
                rx = cx - full_x * 0.48 + (full_x * 0.96) * (i / n_rafters)
                self.box(
                    trim_mat,
                    (rx, cy + sy * (d * 0.5 + over_y * 0.45), ez + 0.08),
                    (0.09, over_y * 0.95, 0.13),
                    rot=(-sy * pitch_deg, 0.0, 0.0),
                )
            # Bargeboards on both gable ends
            for sx in (-1.0, 1.0):
                gx = cx + sx * (full_x * 0.5 - 0.03)
                self.box(
                    trim_mat,
                    (gx, mid_y, mid_z - 0.04),
                    (0.08, slope_len + 0.12, 0.24),
                    rot=(-sy * pitch_deg, 0.0, 0.0),
                )

        # Ridge cap beam
        self.box(trim_mat, (cx, cy, ridge_z + thick * 0.72), (full_x + 0.12, 0.24, 0.20))
        return ridge_z

    def window(self, x, y, z, normal_y, w=0.95, h=1.25, lit=False, shutters=True):
        """Recessed mullioned window with stone sill, timber frame, lintel, and shutters."""
        s = 1.0 if normal_y > 0 else -1.0
        # Recessed glass pane
        glass_mat = "warm_glass" if lit else "glass"
        self.box(glass_mat, (x, y + s * 0.02, z), (w - 0.12, 0.04, h - 0.12))
        # Outer frame
        frame_t = 0.08
        self.box("boards", (x, y + s * 0.04, z + h * 0.5 - frame_t * 0.5), (w, 0.10, frame_t))
        self.box("boards", (x, y + s * 0.04, z - h * 0.5 + frame_t * 0.5), (w, 0.10, frame_t))
        for sx in (-1.0, 1.0):
            self.box(
                "boards",
                (x + sx * (w * 0.5 - frame_t * 0.5), y + s * 0.04, z),
                (frame_t, 0.10, h),
            )
        # Cross mullions
        self.box("white", (x, y + s * 0.045, z), (0.045, 0.04, h - 0.14))
        self.box("white", (x, y + s * 0.045, z + h * 0.08), (w - 0.14, 0.04, 0.045))
        # Stone sill & heavy timber lintel
        self.box("stone", (x, y + s * 0.09, z - h * 0.5 - 0.05), (w + 0.22, 0.22, 0.11))
        self.box("boards", (x, y + s * 0.07, z + h * 0.5 + 0.07), (w + 0.26, 0.16, 0.14))
        if shutters:
            sw = w * 0.44
            for sx in (-1.0, 1.0):
                self.box(
                    "boards", (x + sx * (w * 0.5 + sw * 0.5 + 0.03), y + s * 0.05, z), (sw, 0.05, h)
                )

    def door(self, x, y, z_floor, normal_y, w=1.15, h=2.15, double=False):
        s = 1.0 if normal_y > 0 else -1.0
        # Stone threshold step
        self.box("stone", (x, y + s * 0.22, z_floor - 0.06), (w + 0.45, 0.55, 0.18))
        # Timber jambs & lintel
        self.box("boards", (x, y + s * 0.06, z_floor + h + 0.09), (w + 0.34, 0.20, 0.18))
        for sx in (-1.0, 1.0):
            self.box(
                "boards",
                (x + sx * (w * 0.5 + 0.06), y + s * 0.05, z_floor + h * 0.5),
                (0.12, 0.16, h),
            )
        # Recessed plank door panel(s)
        self.box("wood", (x, y + s * 0.01, z_floor + h * 0.5), (w, 0.08, h))
        # Cross braces / iron hinges
        self.box("boards", (x, y + s * 0.055, z_floor + h * 0.78), (w - 0.08, 0.04, 0.11))
        self.box("boards", (x, y + s * 0.055, z_floor + h * 0.22), (w - 0.08, 0.04, 0.11))
        if double:
            self.box("boards", (x, y + s * 0.06, z_floor + h * 0.5), (0.07, 0.05, h))
        else:
            self.box("steel", (x + w * 0.34, y + s * 0.08, z_floor + h * 0.46), (0.06, 0.08, 0.18))

    def chimney(self, x, y, z_bot, z_top, w=0.85, d=0.72):
        h = z_top - z_bot
        self.box("stone", (x, y, z_bot + h * 0.5), (w, d, h))
        # Corbelled stone cap
        self.box("stone", (x, y, z_top + 0.08), (w + 0.18, d + 0.18, 0.16))
        # Clay chimney pots
        for sx in (-0.20, 0.20):
            self.cyl(
                "clay", (x + sx, y, z_top + 0.14), (x + sx, y, z_top + 0.52), 0.12, 0.10, seg=8
            )

    def dormer(self, x, y_wall, z_base, normal_y, w=1.45, h=1.25):
        s = 1.0 if normal_y > 0 else -1.0
        depth = 1.35
        cy = y_wall - s * (depth * 0.45)
        self.box("plaster", (x, cy, z_base + h * 0.5), (w, depth, h))
        self.prism_gable("plaster", x, cy, z_base + h, w, depth, 0.55, axis="Y")
        # Mini gable roof over dormer
        for sx in (-1.0, 1.0):
            self.box(
                "slate",
                (x + sx * (w * 0.32), cy + s * 0.1, z_base + h + 0.32),
                (w * 0.72, depth + 0.45, 0.10),
                rot=(0.0, sx * 34.0, 0.0),
            )
        self.window(
            x,
            cy + s * (depth * 0.5),
            z_base + h * 0.52,
            normal_y,
            w=0.78,
            h=0.88,
            lit=True,
            shutters=False,
        )

    def loft_hull(self, mat_hull, mat_deck, mat_trim, stations, closed_stern=True):
        """Builds a smooth curved boat hull from cross-section stations
        (port gunwale -> keel -> starboard gunwale)."""
        rings = [[self.bm.verts.new(p) for p in st] for st in stations]
        hull_faces = []
        deck_faces = []
        for i in range(len(rings) - 1):
            a, b = rings[i], rings[i + 1]
            for k in range(len(a) - 1):
                with contextlib.suppress(ValueError):
                    hull_faces.append(self.bm.faces.new([a[k], a[k + 1], b[k + 1], b[k]]))
            # Deck panel bridging port gunwale (0) to starboard gunwale (-1)
            with contextlib.suppress(ValueError):
                deck_faces.append(self.bm.faces.new([a[0], b[0], b[-1], a[-1]]))
        if closed_stern:
            stern = rings[0]
            with contextlib.suppress(ValueError):
                hull_faces.append(self.bm.faces.new(stern))
        bmesh.ops.recalc_face_normals(self.bm, faces=hull_faces + deck_faces)
        self._tag(hull_faces, mat_hull, smooth=True)
        self._tag(deck_faces, mat_deck, smooth=False)

        # Rub rail / gunwale along both sides
        for side_idx in (0, -1):
            pts = [Vector(st[side_idx]) for st in stations]
            for i in range(len(pts) - 1):
                self.cyl(mat_trim, pts[i], pts[i + 1], 0.065, 0.065, seg=6, cap=True, smooth=True)

    def to_blender_object(self):
        bmesh.ops.triangulate(self.bm, faces=self.bm.faces[:])
        me = bpy.data.meshes.new(self.name)
        self.bm.to_mesh(me)
        for mname in self.mat_names:
            col, r, met, em = MAT_SPECS.get(mname, ((0.5, 0.5, 0.5), 0.7, 0.0, 0.0))
            me.materials.append(make_mat(mname, col, r, met, em))
        ob = bpy.data.objects.new(self.name, me)
        bpy.context.scene.collection.objects.link(ob)
        return ob

    def write_gltf(self, path):
        """Triangulates, computes smooth/flat normals + metre box UVs, writes UE-cm glTF."""
        bm = self.bm.copy()
        bmesh.ops.triangulate(bm, faces=bm.faces[:])
        bm.normal_update()

        # Precompute vertex normals for smooth faces
        v_norm = {}
        for v in bm.verts:
            acc = Vector((0.0, 0.0, 0.0))
            for f in v.link_faces:
                if f.smooth:
                    acc += f.normal
            v_norm[v.index] = acc.normalized() if acc.length > 1e-6 else Vector((0.0, 0.0, 1.0))

        mesh = gltf_writer.Mesh()
        for f in bm.faces:
            if len(f.verts) != 3:
                continue
            mat_name = self.mat_names[f.material_index]
            part = mesh.part(mat_name)
            fn = f.normal
            ax, ay, az = abs(fn.x), abs(fn.y), abs(fn.z)
            # Box-project UV in world metres
            ids = []
            for v in f.verts:
                co = v.co
                n = v_norm[v.index] if f.smooth else fn
                if az >= ax and az >= ay:
                    uv = (co.x, -co.y)
                elif ax >= ay:
                    uv = (-co.y, -co.z)
                else:
                    uv = (co.x, -co.z)
                # Blender (+X fwd, +Y left, +Z up) -> UE cm (+X fwd, +Y right, +Z up)
                ue_p = (co.x * 100.0, -co.y * 100.0, co.z * 100.0)
                ue_n = (n.x, -n.y, n.z)
                ids.append(part.vertex(ue_p, ue_n, uv))
            # Flipping Y reverses handedness, so reverse winding to keep outward faces in UE
            part.triangle(ids[0], ids[2], ids[1])
        bm.free()
        mesh.write(path)


# ---------------------------------------------------------------------------------------------
# Harbour Town Buildings (4 distinct coastal Nordic-fantasy architectural styles)
# ---------------------------------------------------------------------------------------------


def build_harbour_house_a():
    """Two-storey coastal merchant house: battered stone ground floor, jettied overhanging
    plaster upper storey with dark half-timbering, steep slate roof, dormer, and stone chimney."""
    b = KitBuilder("harbour_house_a")
    w, d = 9.4, 6.6
    # Deep stone foundation plinth so slopes never expose air under corners
    b.box("stone", (0, 0, -0.7), (w + 0.35, d + 0.35, 2.2))
    # Ground floor stone masonry (z = 0.4 .. 3.1)
    b.box("stone", (0, 0, 1.75), (w, d, 2.7))
    # Corner stone quoins
    for sx in (-1.0, 1.0):
        for sy in (-1.0, 1.0):
            b.box("stone", (sx * w * 0.5, sy * d * 0.5, 1.75), (0.34, 0.34, 2.72))

    # Jettied upper floor overhanging 0.25 m on front/back
    uw, ud = w + 0.16, d + 0.50
    b.box("plaster", (0, 0, 4.45), (uw, ud, 2.70))
    b.prism_gable("plaster", 0, 0, 5.80, uw, ud, (ud * 0.5) * math.tan(math.radians(42.0)))

    # Timber jetty corbel beams & horizontal belt course
    b.box("boards", (0, 0, 3.10), (uw + 0.08, ud + 0.08, 0.20))
    for i in range(7):
        bx = -w * 0.44 + w * 0.88 * (i / 6.0)
        for sy in (-1.0, 1.0):
            b.box("boards", (bx, sy * (d * 0.5 + 0.14), 2.94), (0.16, 0.36, 0.24))

    # Half-timbering verticals & diagonals on upper walls
    for sy in (-1.0, 1.0):
        wy = sy * (ud * 0.5 + 0.02)
        b.box("boards", (0, wy, 5.75), (uw + 0.04, 0.09, 0.16))
        for bx in (-uw * 0.48, -uw * 0.18, uw * 0.18, uw * 0.48):
            b.box("boards", (bx, wy, 4.45), (0.16, 0.09, 2.70))
    for sx in (-1.0, 1.0):
        wx = sx * (uw * 0.5 + 0.02)
        b.box("boards", (wx, 0, 5.75), (0.09, ud + 0.04, 0.16))
        for by in (-ud * 0.48, 0.0, ud * 0.48):
            b.box("boards", (wx, by, 4.45), (0.09, 0.16, 2.70))

    # Steep pitched slate roof + dormer + chimney
    ridge_z = b.roof_pitched("slate", "boards", 0, 0, 5.80, uw, ud, 42.0, over_x=0.50, over_y=0.60)
    b.dormer(0.0, -ud * 0.5, 6.15, -1.0, w=1.55, h=1.20)
    b.chimney(w * 0.34, 0.4, 5.0, ridge_z + 1.15)

    # Door & windows (front is -Y in Blender -> +Y or -Y; both sides detailed!)
    b.door(-1.3, -d * 0.5, 0.40, -1.0, w=1.15, h=2.15)
    b.window(1.6, -d * 0.5, 1.85, -1.0, w=1.10, h=1.25, lit=True)
    b.window(-2.8, -ud * 0.5, 4.50, -1.0, w=1.00, h=1.25, lit=False)
    b.window(0.0, -ud * 0.5, 4.50, -1.0, w=1.05, h=1.25, lit=True)
    b.window(2.8, -ud * 0.5, 4.50, -1.0, w=1.00, h=1.25, lit=False)
    # Back windows
    for wx, lit in ((-2.4, False), (0.0, True), (2.4, False)):
        b.window(wx, d * 0.5, 1.85, 1.0, w=0.95, h=1.15, lit=lit)
        b.window(wx, ud * 0.5, 4.50, 1.0, w=0.95, h=1.20, lit=not lit)
    return b


def build_harbour_house_b():
    """Waterfront gable warehouse / boathouse: heavy stone base, weathered plank upper loft,
    clay tile roof, wide double cargo doors, gable hoist beam, and stone buttresses."""
    b = KitBuilder("harbour_house_b")
    w, d = 11.8, 7.6
    b.box("stone", (0, 0, -0.7), (w + 0.4, d + 0.4, 2.2))
    b.box("stone", (0, 0, 1.85), (w, d, 2.9))
    # Heavy stone pilasters/buttresses along long walls
    for sy in (-1.0, 1.0):
        for bx in (-4.8, -1.6, 1.6, 4.8):
            b.box("stone", (bx, sy * (d * 0.5 + 0.16), 1.60), (0.55, 0.36, 2.60))

    # Timber-clad upper warehouse loft
    b.box("siding", (0, 0, 4.55), (w - 0.1, d - 0.1, 2.55))
    b.prism_gable("siding", 0, 0, 5.82, w - 0.1, d - 0.1, (d * 0.5) * math.tan(math.radians(36.0)))
    b.box("boards", (0, 0, 3.30), (w + 0.14, d + 0.14, 0.22))

    # Clay-tile pitched roof
    ridge_z = b.roof_pitched(
        "clay", "boards", 0, 0, 5.82, w - 0.1, d - 0.1, 36.0, over_x=0.65, over_y=0.55
    )
    # Projecting timber hoist beam at the +X gable peak (facing quay)
    b.box("boards", (w * 0.5 + 0.95, 0.0, ridge_z - 0.55), (2.2, 0.22, 0.22))
    b.cyl(
        "steel",
        (w * 0.5 + 1.80, 0.0, ridge_z - 0.65),
        (w * 0.5 + 1.80, 0.0, ridge_z - 1.55),
        0.03,
        0.03,
        seg=6,
    )
    b.cyl(
        "steel",
        (w * 0.5 + 1.80, 0.0, ridge_z - 0.72),
        (w * 0.5 + 1.80, 0.0, ridge_z - 0.92),
        0.16,
        0.16,
        seg=10,
    )
    # Loft loading doors under hoist
    b.box("boards", (w * 0.5 + 0.02, 0.0, 4.75), (0.12, 1.80, 2.00))

    # Wide double cargo doors + side windows
    b.door(0.0, -d * 0.5, 0.40, -1.0, w=2.80, h=2.55, double=True)
    b.window(-3.6, -d * 0.5, 1.95, -1.0, w=1.05, h=1.05, lit=True, shutters=True)
    b.window(3.6, -d * 0.5, 1.95, -1.0, w=1.05, h=1.05, lit=False, shutters=True)
    for wx in (-3.6, 0.0, 3.6):
        b.window(wx, -d * 0.5, 4.65, -1.0, w=0.95, h=1.05, lit=(wx == 0.0), shutters=True)
        b.window(wx, d * 0.5, 1.95, 1.0, w=1.00, h=1.05, lit=(wx != 0.0), shutters=True)
        b.window(wx, d * 0.5, 4.65, 1.0, w=0.95, h=1.05, lit=False, shutters=True)
    return b


def build_harbour_house_c():
    """Coastal harbour tavern / inn: warm ochre plaster over stone, covered timber porch with
    balcony above, twin roof dormers, hanging sign bracket, and glowing tavern windows."""
    b = KitBuilder("harbour_house_c")
    w, d = 11.4, 7.4
    b.box("stone", (0, 0, -0.7), (w + 0.4, d + 0.4, 2.2))
    b.box("stone", (0, 0, 1.80), (w, d, 2.80))
    b.box("ochre", (0, 0, 4.55), (w, d, 2.75))
    b.prism_gable("ochre", 0, 0, 5.92, w, d, (d * 0.5) * math.tan(math.radians(39.0)))
    b.box("boards", (0, 0, 3.20), (w + 0.16, d + 0.16, 0.20))

    # Front covered timber porch & upper balcony (-Y side)
    porch_d = 1.90
    py = -(d * 0.5 + porch_d * 0.5)
    b.box("deck", (0, py, 0.42), (w * 0.86, porch_d, 0.18))
    b.box("stone", (0, py, -0.5), (w * 0.86, porch_d, 1.75))
    # Balcony floor / porch roof
    b.box("boards", (0, py, 3.16), (w * 0.88, porch_d + 0.12, 0.20))
    # Porch columns + knee braces
    for px in (-4.4, -1.6, 1.6, 4.4):
        b.box("boards", (px, -(d * 0.5 + porch_d - 0.12), 1.80), (0.20, 0.20, 2.72))
        b.box("boards", (px, -(d * 0.5 + porch_d - 0.12), 3.72), (0.12, 0.12, 1.05))
    # Balcony railing & balusters
    rail_y = -(d * 0.5 + porch_d - 0.12)
    b.box("boards", (0, rail_y, 4.22), (w * 0.86, 0.11, 0.09))
    b.box("boards", (0, rail_y, 3.36), (w * 0.86, 0.09, 0.08))
    for i in range(21):
        rx = -w * 0.42 + w * 0.84 * (i / 20.0)
        b.box("boards", (rx, rail_y, 3.79), (0.05, 0.05, 0.84))

    # Hanging inn sign
    b.box("steel", (-w * 0.5 - 0.65, -d * 0.5 + 0.4, 3.35), (1.30, 0.06, 0.06))
    b.box("boards", (-w * 0.5 - 0.80, -d * 0.5 + 0.4, 2.80), (0.95, 0.06, 0.72))

    # Roof, twin dormers, chimney
    ridge_z = b.roof_pitched("slate", "boards", 0, 0, 5.92, w, d, 39.0, over_x=0.55, over_y=0.60)
    b.dormer(-2.4, -d * 0.5, 6.20, -1.0, w=1.40, h=1.15)
    b.dormer(2.4, -d * 0.5, 6.20, -1.0, w=1.40, h=1.15)
    b.chimney(-w * 0.36, 0.6, 5.0, ridge_z + 1.25)

    # Doors & warm windows
    b.door(0.0, -d * 0.5, 0.50, -1.0, w=1.35, h=2.25, double=True)
    b.window(-3.1, -d * 0.5, 1.85, -1.0, w=1.25, h=1.30, lit=True)
    b.window(3.1, -d * 0.5, 1.85, -1.0, w=1.25, h=1.30, lit=True)
    for wx, lit in ((-3.2, True), (0.0, True), (3.2, False)):
        b.window(wx, -d * 0.5, 4.55, -1.0, w=1.05, h=1.25, lit=lit)
        b.window(wx, d * 0.5, 1.85, 1.0, w=1.05, h=1.20, lit=lit)
        b.window(wx, d * 0.5, 4.55, 1.0, w=1.00, h=1.20, lit=not lit)
    return b


def build_harbour_house_d():
    """Sturdy stone fisherman's cottage with side timber shed, deep windows, and tall chimney."""
    b = KitBuilder("harbour_house_d")
    w, d = 8.2, 5.8
    b.box("stone", (0, 0, -0.7), (w + 0.35, d + 0.35, 2.2))
    b.box("stone", (0, 0, 1.95), (w, d, 3.10))
    b.prism_gable("stone", 0, 0, 3.50, w, d, (d * 0.5) * math.tan(math.radians(40.0)))
    ridge_z = b.roof_pitched("slate", "boards", 0, 0, 3.50, w, d, 40.0, over_x=0.45, over_y=0.50)
    b.chimney(w * 0.38, 0.2, 2.8, ridge_z + 1.10)

    # Timber lean-to shed on -X gable end
    sw, sd = 2.6, 4.0
    b.box("stone", (-(w * 0.5 + sw * 0.5), 0.2, -0.7), (sw + 0.2, sd + 0.2, 2.1))
    b.box("siding", (-(w * 0.5 + sw * 0.5), 0.2, 1.40), (sw, sd, 2.10))
    b.box(
        "slate",
        (-(w * 0.5 + sw * 0.5 + 0.15), 0.2, 2.58),
        (sw + 0.55, sd + 0.55, 0.14),
        rot=(0.0, -18.0, 0.0),
    )

    b.door(-0.8, -d * 0.5, 0.40, -1.0, w=1.05, h=2.05)
    b.window(1.8, -d * 0.5, 1.95, -1.0, w=1.00, h=1.15, lit=True)
    b.window(-2.5, -d * 0.5, 1.95, -1.0, w=0.90, h=1.15, lit=False)
    for wx, lit in ((-1.8, True), (1.8, False)):
        b.window(wx, d * 0.5, 1.95, 1.0, w=0.95, h=1.10, lit=lit)
    return b


def build_harbour_warehouse():
    """Two-storey stone-and-timber waterfront warehouse with wide cargo doors, loft hoist beam,
    and pitched slate roof."""
    b = KitBuilder("harbour_warehouse")
    w, d = 14.0, 8.6
    # Deep stone foundation plinth + ground-floor stone walls
    b.box("stone", (0, 0, -0.8), (w + 0.45, d + 0.45, 2.4))
    b.box("stone", (0, 0, 1.95), (w, d, 3.10))
    # Upper timber-boarded storage loft
    b.box("siding", (0, 0, 4.75), (w + 0.15, d + 0.15, 2.55))
    # Timber corner posts & mid pilasters
    for sx in (-w * 0.5, -w * 0.17, w * 0.17, w * 0.5):
        for sy in (-d * 0.5, d * 0.5):
            b.box("boards", (sx, sy, 3.2), (0.28, 0.28, 5.8))
    b.prism_gable(
        "siding", 0, 0, 6.02, w + 0.15, d + 0.15, (d * 0.5) * math.tan(math.radians(36.0))
    )
    ridge_z = b.roof_pitched(
        "slate", "boards", 0, 0, 6.02, w + 0.15, d + 0.15, 36.0, over_x=0.65, over_y=0.65
    )
    b.chimney(w * 0.36, 0.4, 5.2, ridge_z + 1.0)
    # Wide double cargo doors on waterfront (-Y) face + canopy hood
    b.box("boards", (0.0, -d * 0.5 - 0.08, 1.65), (3.1, 0.18, 2.50))
    b.box("iron", (0.0, -d * 0.5 - 0.15, 1.65), (0.08, 0.08, 2.50))
    b.box("slate", (0.0, -d * 0.5 - 0.75, 3.15), (3.8, 1.5, 0.14), rot=(14.0, 0.0, 0.0))
    # Loft hoist beam projecting out from front eaves with pulley housing
    b.box("boards", (0.0, -d * 0.5 - 1.15, 6.15), (0.26, 2.4, 0.26))
    b.box("iron", (0.0, -d * 0.5 - 2.05, 5.75), (0.24, 0.24, 0.55))
    b.box("boards", (0.0, -d * 0.5 - 0.10, 4.75), (1.9, 0.18, 1.85))
    # Warm lit quay-side and rear windows
    for wx in (-4.4, 4.4):
        b.window(wx, -d * 0.5, 1.95, -1.0, w=1.20, h=1.25, lit=True)
        b.window(wx, -d * 0.5, 4.75, -1.0, w=1.05, h=1.05, lit=(wx > 0))
    for wx, lit in ((-4.2, True), (0.0, False), (4.2, True)):
        b.window(wx, d * 0.5, 1.95, 1.0, w=1.15, h=1.20, lit=lit)
        b.window(wx, d * 0.5, 4.75, 1.0, w=1.00, h=1.00, lit=False)
    return b


def build_harbour_cargo():
    """Dockside cluster of slatted timber crates, banded oak barrels, and coiled rope pallet."""
    b = KitBuilder("harbour_cargo")
    # Timber skid pallet
    b.box("boards", (0.0, 0.0, 0.08), (2.9, 2.1, 0.16))
    # Large and medium cargo crates with frame battens
    for cx, cy, cz, sx, sy, sz, yaw in (
        (-0.65, -0.25, 0.70, 1.25, 1.05, 1.08, 6.0),
        (0.55, -0.35, 0.58, 0.95, 0.90, 0.84, -8.0),
        (-0.45, -0.20, 1.56, 0.85, 0.80, 0.68, 14.0),
    ):
        b.box("siding", (cx, cy, cz), (sx, sy, sz), rot=(0.0, 0.0, yaw))
        b.box("boards", (cx, cy, cz + sz * 0.48), (sx + 0.06, sy + 0.06, 0.09), rot=(0.0, 0.0, yaw))
        b.box("boards", (cx, cy, cz - sz * 0.44), (sx + 0.06, sy + 0.06, 0.09), rot=(0.0, 0.0, yaw))
    # Three banded oak barrels beside the crates
    for bx, by, bh in ((0.65, 0.55, 0.96), (-0.10, 0.62, 0.96), (1.15, 0.15, 0.88)):
        b.cyl("boards", (bx, by, 0.16), (bx, by, 0.16 + bh), 0.32, 0.32, seg=10)
        b.cyl("iron", (bx, by, 0.16 + bh * 0.22), (bx, by, 0.16 + bh * 0.28), 0.335, 0.335, seg=10)
        b.cyl("iron", (bx, by, 0.16 + bh * 0.72), (bx, by, 0.16 + bh * 0.78), 0.335, 0.335, seg=10)
    # Coiled hemp rope stack
    b.cyl("rope", (-1.05, 0.55, 0.16), (-1.05, 0.55, 0.44), 0.34, 0.30, seg=12)
    return b


def build_harbour_market_stall():
    """3.8 m x 2.4 m coastal market stall with timber posts, counter table, slatted goods
    crates, striped awning roof, and a warm glowing hanging lantern."""
    b = KitBuilder("harbour_market_stall")
    w, d = 3.8, 2.4
    hx, hy = w * 0.5, d * 0.5
    post_x = hx - 0.14
    post_y = hy - 0.14
    front_z = 2.55
    back_z = 2.15

    # Corner timber posts with buried stone pad footings for uneven cobbles
    for sx in (-1.0, 1.0):
        for sy in (-1.0, 1.0):
            px, py = sx * post_x, sy * post_y
            top_z = front_z if sy < 0 else back_z
            b.box("stone", (px, py, -0.14), (0.28, 0.28, 0.36), bevel=0.02)
            b.box("boards", (px, py, top_z * 0.5), (0.14, 0.14, top_z), bevel=0.015)

    # Top longitudinal beams & sloped side rafters
    b.box("boards", (0.0, -post_y, front_z), (w - 0.06, 0.14, 0.16), bevel=0.015)
    b.box("boards", (0.0, post_y, back_z), (w - 0.06, 0.14, 0.16), bevel=0.015)

    slope_dy = post_y * 2.0
    slope_dz = front_z - back_z
    pitch_deg = math.degrees(math.atan2(slope_dz, slope_dy))
    rafter_len = math.hypot(slope_dy, slope_dz) + 0.72
    mid_z = (front_z + back_z) * 0.5 + 0.08
    for rx in (-post_x, -0.60, 0.60, post_x):
        b.box(
            "boards",
            (rx, 0.0, mid_z),
            (0.10, rafter_len, 0.12),
            rot=(pitch_deg, 0.0, 0.0),
            bevel=0.01,
        )

    # Knee braces at front posts
    for sx in (-1.0, 1.0):
        px = sx * post_x
        b.cyl(
            "boards",
            (px, -post_y, front_z - 0.52),
            (px - sx * 0.45, -post_y, front_z - 0.06),
            0.045,
            0.045,
            seg=6,
            smooth=False,
        )

    # Waist-high timber back wall & side rails
    b.box("siding", (0.0, post_y, 0.56), (w - 0.28, 0.06, 1.04))
    b.box("boards", (0.0, post_y, 1.10), (w - 0.20, 0.11, 0.09), bevel=0.01)
    for sx in (-1.0, 1.0):
        b.box("boards", (sx * post_x, 0.0, 0.92), (0.08, d - 0.34, 0.08))

    # Front timber counter table (facing -Y) + lower storage shelf
    counter_y = -0.68
    counter_w = w - 0.32
    b.box("deck", (0.0, counter_y, 0.90), (counter_w, 0.82, 0.07), bevel=0.012)
    b.box("siding", (0.0, counter_y - 0.35, 0.48), (counter_w - 0.08, 0.05, 0.78))
    b.box("boards", (0.0, counter_y + 0.02, 0.24), (counter_w - 0.12, 0.70, 0.05))
    for lx in (-1.62, -0.55, 0.55, 1.62):
        b.box("boards", (lx, counter_y + 0.32, 0.45), (0.09, 0.09, 0.86))

    # Slatted goods crates underneath the counter & beside rear wall
    under_crates = [
        (-1.12, 0.15, 0.26, 0.86, 0.64, 0.52, 5.0),
        (-0.08, 0.22, 0.24, 0.78, 0.58, 0.48, -7.0),
        (1.08, 0.18, 0.28, 0.90, 0.66, 0.56, 3.0),
    ]
    for cx, cy, cz, sx, sy, sz, yaw in under_crates:
        b.box("siding", (cx, cy, cz), (sx, sy, sz), rot=(0.0, 0.0, yaw))
        for bz in (cz - sz * 0.38, cz + sz * 0.38):
            b.box(
                "boards",
                (cx, cy, bz),
                (sx + 0.04, sy + 0.04, 0.06),
                rot=(0.0, 0.0, yaw),
            )

    # Oak barrel tucked at right end
    b.cyl("boards", (1.35, 0.62, 0.02), (1.35, 0.62, 0.82), 0.26, 0.26, seg=10)
    b.cyl("steel", (1.35, 0.62, 0.20), (1.35, 0.62, 0.24), 0.272, 0.272, seg=10)
    b.cyl("steel", (1.35, 0.62, 0.62), (1.35, 0.62, 0.66), 0.272, 0.272, seg=10)

    # Display crates on top of counter with colourful harbour goods
    top_crates = [
        (-1.05, counter_y - 0.02, 1.07, 0.82, 0.52, 0.26, -4.0, "red"),
        (0.05, counter_y + 0.02, 1.06, 0.78, 0.50, 0.24, 3.0, "ochre"),
        (1.08, counter_y - 0.03, 1.07, 0.76, 0.48, 0.26, -6.0, "green"),
    ]
    for cx, cy, cz, sx, sy, sz, yaw, goods_mat in top_crates:
        b.box("siding", (cx, cy, cz), (sx, sy, sz), rot=(0.0, 0.0, yaw))
        b.box(
            "boards",
            (cx, cy, cz + sz * 0.45),
            (sx + 0.04, sy + 0.04, 0.045),
            rot=(0.0, 0.0, yaw),
        )
        # Produce / goods mound inside each display tray
        for ox, oy, r_g in ((-0.18, -0.06, 0.14), (0.04, 0.05, 0.15), (0.21, -0.04, 0.13)):
            b.box(
                goods_mat,
                (cx + ox, cy + oy, cz + sz * 0.42),
                (r_g * 1.4, r_g * 1.3, r_g * 1.1),
                rot=(12.0, -8.0, yaw + 25.0),
                bevel=0.04,
            )

    # Striped sloped awning roof (clay / ochre / plaster alternating boards)
    awning_mats = ("clay", "ochre", "plaster")
    n_stripes = 15
    roof_w = w + 0.52
    stripe_w = roof_w / n_stripes
    awning_len = rafter_len + 0.10
    for i in range(n_stripes):
        sx = -roof_w * 0.5 + stripe_w * (i + 0.5)
        mat = awning_mats[i % len(awning_mats)]
        b.box(
            mat,
            (sx, -0.03, mid_z + 0.10),
            (stripe_w - 0.008, awning_len, 0.055),
            rot=(pitch_deg, 0.0, 0.0),
            bevel=0.008,
        )
        # Front scalloped awning valance strip
        b.box(
            mat,
            (sx, -post_y - 0.38, front_z + 0.05),
            (stripe_w - 0.012, 0.04, 0.22),
            bevel=0.008,
        )

    # Small warm glowing hanging lantern under front-right beam
    lx, ly, lz = 1.35, -post_y - 0.08, front_z - 0.08
    b.cyl("steel", (lx, ly, lz), (lx, ly, lz - 0.26), 0.012, 0.012, seg=6)
    b.cyl("steel", (lx, ly, lz - 0.26), (lx, ly, lz - 0.34), 0.03, 0.13, seg=6, smooth=False)
    b.box("warm_glass", (lx, ly, lz - 0.46), (0.15, 0.15, 0.22), bevel=0.015)
    b.box("steel", (lx, ly, lz - 0.59), (0.19, 0.19, 0.04), bevel=0.008)
    return b


def build_harbour_planter():
    """1.8 m x 0.85 m x 0.75 m carved stone rectangular street planter with beveled rim,
    dark soil bed, and multi-layered sculpted green shrub and flower tufts."""
    b = KitBuilder("harbour_planter")
    w, d, h = 1.80, 0.85, 0.75

    # Buried base pad + stepped carved stone plinth
    b.box("stone", (0.0, 0.0, -0.14), (w + 0.06, d + 0.06, 0.32), bevel=0.02)
    b.box("stone", (0.0, 0.0, 0.08), (w, d, 0.16), bevel=0.025)

    # Carved trough body with corner pilasters & recessed side panels
    body_w, body_d = w - 0.10, d - 0.10
    b.box("stone", (0.0, 0.0, 0.39), (body_w, body_d, 0.48), bevel=0.02)
    for sx in (-1.0, 1.0):
        for sy in (-1.0, 1.0):
            b.box(
                "stone",
                (sx * (body_w * 0.5 - 0.06), sy * (body_d * 0.5 - 0.06), 0.39),
                (0.18, 0.18, 0.50),
                bevel=0.02,
            )
    for sy in (-1.0, 1.0):
        b.box("stone", (0.0, sy * (body_d * 0.5 + 0.015), 0.39), (1.10, 0.05, 0.32), bevel=0.015)

    # Overhanging beveled stone rim enclosing the recessed soil bed
    rim_z = h - 0.055
    rim_t = 0.13
    for sy in (-1.0, 1.0):
        b.box(
            "stone",
            (0.0, sy * (d * 0.5 - rim_t * 0.5), rim_z),
            (w, rim_t, 0.11),
            bevel=0.025,
        )
    for sx in (-1.0, 1.0):
        b.box(
            "stone",
            (sx * (w * 0.5 - rim_t * 0.5), 0.0, rim_z),
            (rim_t, d - rim_t * 1.4, 0.11),
            bevel=0.025,
        )

    # Dark soil bed recessed inside the rim
    b.box("wood", (0.0, 0.0, 0.61), (w - 0.24, d - 0.24, 0.10), bevel=0.02)

    # Multi-layered sculpted green shrub mounds & outer leaf tufts
    shrubs = [
        (-0.46, 0.02, 0.78, (0.56, 0.48, 0.38), (6.0, -8.0, 18.0)),
        (0.02, -0.02, 0.84, (0.64, 0.54, 0.44), (-5.0, 7.0, -24.0)),
        (0.48, 0.03, 0.77, (0.54, 0.46, 0.36), (8.0, 6.0, 32.0)),
        (-0.22, 0.06, 0.94, (0.44, 0.38, 0.34), (-9.0, -6.0, 42.0)),
        (0.24, -0.05, 0.92, (0.42, 0.36, 0.32), (7.0, 9.0, -35.0)),
    ]
    for sx, sy, sz, size, rot in shrubs:
        b.box("green", (sx, sy, sz), size, rot=rot, bevel=0.09)

    leaf_tufts = [
        (-0.66, -0.14, 0.72, 0.24, 0.16, (18.0, -14.0, 12.0)),
        (-0.52, 0.17, 0.73, 0.22, 0.15, (-16.0, -10.0, 28.0)),
        (-0.18, -0.18, 0.74, 0.26, 0.17, (20.0, 6.0, -15.0)),
        (0.14, 0.19, 0.74, 0.25, 0.16, (-18.0, 8.0, 20.0)),
        (0.58, -0.15, 0.72, 0.24, 0.16, (15.0, 14.0, -22.0)),
        (0.65, 0.14, 0.72, 0.22, 0.15, (-14.0, 16.0, 35.0)),
        (0.0, 0.02, 1.06, 0.28, 0.22, (8.0, -10.0, 40.0)),
    ]
    for lx, ly, lz, rw, rh, rot in leaf_tufts:
        b.box("green", (lx, ly, lz), (rw, rw * 0.85, rh), rot=rot, bevel=0.045)

    # Small coastal blossom clusters nestled across the foliage
    flowers = [
        (-0.54, -0.08, 0.92, "red"),
        (-0.34, 0.12, 1.02, "ochre"),
        (-0.14, -0.14, 0.98, "white"),
        (0.05, 0.08, 1.12, "red"),
        (0.22, -0.12, 1.04, "ochre"),
        (0.42, 0.10, 0.96, "white"),
        (0.56, -0.06, 0.90, "red"),
        (-0.26, 0.0, 1.08, "ochre"),
    ]
    for fx, fy, fz, fmat in flowers:
        b.box(fmat, (fx, fy, fz), (0.11, 0.11, 0.09), rot=(15.0, -12.0, 25.0), bevel=0.025)
    return b


def build_lakefront_pavilion():
    """Sleek modern Chicago Lakefront Trail park pavilion (~7.5 m x 4.2 m, ~3.4 m tall):
    concrete pad, slender steel I-beam columns, cantilevered flat roof with timber slatted
    soffit, rear concrete & glass windscreen wall, and two modern timber-and-steel benches."""
    b = KitBuilder("lakefront_pavilion")
    w, d = 7.5, 4.2

    # Buried concrete pad slab so trailside slopes never expose air under edges
    b.box("concrete", (0.0, 0.0, -0.22), (w + 0.16, d + 0.16, 0.68), bevel=0.03)
    b.box("concrete", (0.0, 0.0, 0.12), (w, d, 0.08), bevel=0.02)

    # 4 slender dark steel I-beam columns inset for clean roof cantilevers
    col_x, col_y = 2.55, 1.25
    col_h = 3.04
    col_z0 = 0.16
    for sx in (-1.0, 1.0):
        for sy in (-1.0, 1.0):
            cx, cy = sx * col_x, sy * col_y
            b.box("steel", (cx, cy, col_z0 + 0.02), (0.30, 0.30, 0.04), bevel=0.008)
            # Steel I-beam profile (central web + twin flanges)
            b.box("steel", (cx, cy, col_z0 + col_h * 0.5), (0.04, 0.16, col_h))
            for fx in (-0.08, 0.08):
                b.box("steel", (cx + fx, cy, col_z0 + col_h * 0.5), (0.03, 0.18, col_h))

    # Steel primary roof girders spanning columns
    beam_z = col_z0 + col_h - 0.06
    for sy in (-1.0, 1.0):
        b.box("steel", (0.0, sy * col_y, beam_z), (w - 0.55, 0.14, 0.18), bevel=0.01)
    for sx in (-1.0, 1.0):
        b.box("steel", (sx * col_x, 0.0, beam_z), (0.14, d - 0.35, 0.18), bevel=0.01)

    # Cantilevered flat roof: steel perimeter fascia, recessed slate deck, slatted wood soffit
    roof_w, roof_d = 7.75, 4.45
    roof_z = 3.28
    b.box("slate", (0.0, 0.0, roof_z + 0.04), (roof_w - 0.12, roof_d - 0.12, 0.16), bevel=0.01)
    for sy in (-1.0, 1.0):
        b.box("steel", (0.0, sy * (roof_d * 0.5 - 0.06), roof_z), (roof_w, 0.12, 0.24), bevel=0.012)
    for sx in (-1.0, 1.0):
        b.box(
            "steel",
            (sx * (roof_w * 0.5 - 0.06), 0.0, roof_z),
            (0.12, roof_d - 0.16, 0.24),
            bevel=0.012,
        )
    # Warm timber slatted ceiling soffit + recessed linear warm light strips
    for i in range(19):
        sx = -3.42 + (6.84 * i / 18.0)
        b.box("siding", (sx, 0.0, roof_z - 0.12), (0.18, roof_d - 0.42, 0.05), bevel=0.006)
    for ly in (-0.75, 0.75):
        b.box("warm_glass", (0.0, ly, roof_z - 0.13), (4.6, 0.07, 0.03))

    # Rear architectural concrete & glass windscreen wall (at y = +1.35)
    wy = 1.38
    b.box("concrete", (-1.15, wy, 1.44), (3.45, 0.22, 2.56), bevel=0.025)
    # Glass & warm glowing accent windscreen bay on right rear
    b.box("steel", (1.68, wy, 1.44), (2.10, 0.10, 2.52))
    b.box("glass", (1.68, wy, 1.44), (1.96, 0.04, 2.38))
    b.box("warm_glass", (1.68, wy, 1.44), (0.42, 0.06, 2.34))
    for mx in (0.95, 2.40):
        b.box("steel", (mx, wy, 1.44), (0.07, 0.14, 2.52))

    # Two modern timber-and-steel park benches underneath
    for bx in (-1.55, 1.55):
        by = 0.22
        bw = 2.15
        for lx in (-0.82, 0.0, 0.82):
            b.box("steel", (bx + lx, by, 0.34), (0.06, 0.50, 0.36), bevel=0.008)
            b.box(
                "steel",
                (bx + lx, by + 0.23, 0.68),
                (0.055, 0.06, 0.42),
                rot=(-10.0, 0.0, 0.0),
            )
        for sy_off in (-0.17, -0.06, 0.05, 0.16):
            b.box("deck", (bx, by + sy_off, 0.54), (bw, 0.085, 0.045), bevel=0.008)
        for bz_off, by_off in ((0.70, 0.24), (0.83, 0.26)):
            b.box(
                "deck",
                (bx, by + by_off, bz_off),
                (bw, 0.045, 0.095),
                rot=(-10.0, 0.0, 0.0),
                bevel=0.008,
            )
    return b


def build_modern_boathouse():
    """Modern Chicago lakefront marina boathouse & clubhouse (~15 m x 8.5 m, 2 storeys):
    board-formed concrete plinth, double marina boat bays, cantilevered timber-and-plaster
    upper clubhouse with panoramic warm_glass windows, flat cantilevered roof, and balcony."""
    b = KitBuilder("modern_boathouse")
    w, d = 15.0, 8.5

    # Buried concrete foundation + ground-floor board-formed concrete mass (z = 0.0 .. 3.55)
    b.box("concrete", (0.0, 0.0, -0.45), (w + 0.50, d + 0.50, 1.30), bevel=0.04)
    b.box("concrete", (0.0, 0.0, 1.88), (w, d, 3.36), bevel=0.03)
    # Architectural concrete pilasters along front (-Y) and sides
    for px in (-7.25, -1.75, 3.45, 7.25):
        b.box("concrete", (px, -d * 0.5 - 0.06, 1.88), (0.48, 0.22, 3.38), bevel=0.02)

    # Wide double marina boat-bay doors on waterfront (-Y) face
    for bx in (-4.50, 0.85):
        b.box("steel", (bx, -d * 0.5 - 0.04, 1.62), (4.45, 0.18, 2.86), bevel=0.015)
        b.box("boards", (bx, -d * 0.5 - 0.06, 1.60), (4.20, 0.14, 2.66))
        for rx in (-1.4, 0.0, 1.4):
            b.box("steel", (bx + rx, -d * 0.5 - 0.12, 1.60), (0.07, 0.06, 2.66))
        for rz in (0.95, 2.25):
            b.box("steel", (bx, -d * 0.5 - 0.12, rz), (4.22, 0.06, 0.07))

    # Glazed marina lobby entrance on ground right (+X side, waterfront -Y)
    b.box("steel", (5.35, -d * 0.5 - 0.04, 1.72), (3.10, 0.16, 3.04), bevel=0.015)
    b.box("glass", (5.35, -d * 0.5 - 0.06, 1.72), (2.88, 0.08, 2.84))
    for mx in (4.35, 5.35, 6.35):
        b.box("steel", (mx, -d * 0.5 - 0.10, 1.72), (0.07, 0.10, 2.88))

    # Rear (+Y trailside) glazed lobby window bays
    for wx in (-4.2, 0.0, 4.2):
        b.box("steel", (wx, d * 0.5 + 0.03, 1.85), (2.90, 0.14, 1.80))
        b.box("glass", (wx, d * 0.5 + 0.05, 1.85), (2.72, 0.08, 1.64))
        b.box("steel", (wx, d * 0.5 + 0.08, 1.85), (0.06, 0.08, 1.68))

    # Cantilevered upper floor deck slab projecting 1.25 m toward the water (-Y)
    uw, ud = 14.8, 8.2
    uy = -0.85
    b.box("concrete", (0.0, uy, 3.68), (uw + 0.35, ud + 0.35, 0.28), bevel=0.025)
    for sx in (-6.2, -2.1, 2.1, 6.2):
        b.box("steel", (sx, uy - ud * 0.32, 3.45), (0.20, 3.2, 0.22), bevel=0.01)

    # Upper storey clubhouse volume (z = 3.82 .. 6.86) in plaster + warm timber cladding
    b.box("plaster", (-1.40, uy + 0.35, 5.34), (11.8, ud - 0.70, 3.04), bevel=0.02)
    # Warm timber accent cladding bands on upper rear & side walls
    b.box("siding", (-7.32, uy + 0.35, 5.34), (0.14, ud - 0.80, 2.90), bevel=0.015)
    b.box("siding", (-1.40, uy + (ud - 0.70) * 0.5 + 0.06, 5.34), (11.6, 0.14, 2.86), bevel=0.015)
    # Upper rear trail-facing panoramic ribbon window
    b.box("steel", (-1.40, uy + (ud - 0.70) * 0.5 + 0.12, 5.42), (8.6, 0.12, 1.75))
    b.box("glass", (-1.40, uy + (ud - 0.70) * 0.5 + 0.14, 5.42), (8.36, 0.08, 1.56))
    for mx in (-4.2, -1.4, 1.4):
        b.box("steel", (mx, uy + (ud - 0.70) * 0.5 + 0.17, 5.42), (0.08, 0.08, 1.60))

    # Panoramic waterfront curtain wall on upper cantilevered front (-Y) & east (+X)
    front_y = uy - (ud - 0.70) * 0.5 + 0.35
    b.box("steel", (-1.40, front_y, 5.34), (11.5, 0.16, 2.88))
    b.box("glass", (-1.40, front_y - 0.02, 5.34), (11.2, 0.10, 2.66))
    for mx in (-6.8, -4.6, -2.4, -0.2, 2.0, 4.15):
        b.box("steel", (mx, front_y - 0.06, 5.34), (0.09, 0.14, 2.74))
    b.box("steel", (-1.40, front_y - 0.06, 5.88), (11.4, 0.12, 0.08))
    # East clubhouse glass end wall overlooking balcony
    b.box("steel", (4.50, uy + 0.35, 5.34), (0.16, ud - 0.90, 2.84))
    b.box("warm_glass", (4.52, uy + 0.35, 5.34), (0.10, ud - 1.15, 2.64))

    # Upper lake-view cantilevered balcony terrace (+X end, x = 4.55 .. 7.45) with steel railing
    bal_x, bal_y = 6.00, uy
    bal_w, bal_d = 2.95, ud + 0.10
    b.box("deck", (bal_x, bal_y, 3.84), (bal_w, bal_d, 0.08), bevel=0.012)
    # Steel perimeter guardrail + glass infill around balcony edges
    rail_z = 4.38
    b.box("steel", (7.42, bal_y, rail_z + 0.50), (0.07, bal_d, 0.06))
    b.box("glass", (7.42, bal_y, rail_z), (0.03, bal_d - 0.12, 0.92))
    for sy in (-1.0, 1.0):
        ry = bal_y + sy * (bal_d * 0.5 - 0.04)
        b.box("steel", (bal_x, ry, rail_z + 0.50), (bal_w, 0.07, 0.06))
        b.box("glass", (bal_x, ry, rail_z), (bal_w - 0.12, 0.03, 0.92))
    for py in (-4.0, -2.0, 0.0, 2.0, 4.0):
        b.box("steel", (7.42, bal_y + py * 0.95, rail_z), (0.06, 0.06, 1.04))
    for px in (4.70, 6.05):
        for sy in (-1.0, 1.0):
            b.box("steel", (px, bal_y + sy * (bal_d * 0.5 - 0.04), rail_z), (0.06, 0.06, 1.04))

    # Sleek flat cantilevered slate/steel roof over entire upper storey & balcony
    roof_w, roof_d = uw + 1.80, ud + 1.90
    roof_z = 7.06
    b.box("slate", (0.0, uy, roof_z + 0.06), (roof_w - 0.16, roof_d - 0.16, 0.24), bevel=0.015)
    for sy in (-1.0, 1.0):
        b.box(
            "steel",
            (0.0, uy + sy * (roof_d * 0.5 - 0.08), roof_z),
            (roof_w, 0.16, 0.36),
            bevel=0.015,
        )
    for sx in (-1.0, 1.0):
        b.box(
            "steel",
            (sx * (roof_w * 0.5 - 0.08), uy, roof_z),
            (0.16, roof_d - 0.24, 0.36),
            bevel=0.015,
        )
    # Warm timber soffit under overhanging eaves
    b.box("siding", (0.0, uy, roof_z - 0.14), (roof_w - 0.36, roof_d - 0.36, 0.06))
    # Sleek rooftop clerestory skylight monitor
    b.box("steel", (-1.40, uy + 0.2, roof_z + 0.32), (5.80, 1.90, 0.34), bevel=0.015)
    b.box("warm_glass", (-1.40, uy + 0.2, roof_z + 0.34), (5.55, 1.94, 0.22))
    return b


def build_chicago_townhouse():
    """Classic 3-storey Chicago greystone / Lake Shore Drive townhouse (~8.4 m wide x 11.5 m
    deep, ~11.2 m tall): buried concrete foundation, rusticated stone plinth with entry stoop
    and steel handrails, plaster/clay facade with protruding 3-sided bay window, concrete
    cornice parapet, and rooftop deck pergola."""
    b = KitBuilder("chicago_townhouse")
    w, d = 8.4, 11.5
    hy = d * 0.5  # front face at y = -5.75

    # Buried concrete foundation (z = -2.0 .. 0.4 m)
    b.box("concrete", (0.0, 0.0, -0.80), (w + 0.24, d + 0.24, 2.40), bevel=0.03)

    # Rusticated stone/concrete ground-floor plinth (z = 0.4 .. 3.4 m)
    b.box("stone", (0.0, 0.0, 1.90), (w, d, 3.00), bevel=0.025)
    for cz in (1.15, 1.90, 2.65):
        b.box("concrete", (0.0, -hy - 0.02, cz), (w + 0.06, 0.08, 0.07))
    # Heavy concrete belt course separating ground plinth from upper brick/greystone
    b.box("concrete", (0.0, 0.0, 3.46), (w + 0.22, d + 0.20, 0.22), bevel=0.02)

    # Upper 2 storeys: clay brick side/rear body + Chicago greystone (plaster) front facade
    b.box("clay", (0.0, 0.25, 6.85), (w - 0.04, d - 0.50, 6.60), bevel=0.02)
    b.box("plaster", (0.0, -hy + 0.28, 6.85), (w, 0.56, 6.60), bevel=0.02)
    # Greystone corner pilasters on front facade
    for sx in (-1.0, 1.0):
        b.box("concrete", (sx * (w * 0.5 - 0.28), -hy - 0.02, 6.85), (0.58, 0.22, 6.62), bevel=0.02)

    # Front stepped entry stoop on left-front (x = -2.10, facing -Y) + steel handrails
    sx_door = -2.10
    b.box("concrete", (sx_door, -hy - 0.55, 0.50), (2.40, 1.15, 1.10), bevel=0.025)
    for i, (sy_step, sz_top) in enumerate(
        ((-hy - 1.35, 0.78), (-hy - 1.72, 0.52), (-hy - 2.08, 0.26))
    ):
        b.box(
            "concrete",
            (sx_door, sy_step, sz_top * 0.5 - 0.08),
            (2.30 - i * 0.04, 0.42, sz_top + 0.16),
            bevel=0.02,
        )
    # Steel stair handrails & newel posts
    for side_x in (sx_door - 1.06, sx_door + 1.06):
        b.cyl("steel", (side_x, -hy - 0.18, 1.02), (side_x, -hy - 0.18, 1.92), 0.03, 0.03, seg=8)
        b.cyl("steel", (side_x, -hy - 2.05, 0.22), (side_x, -hy - 2.05, 1.12), 0.03, 0.03, seg=8)
        b.cyl("steel", (side_x, -hy - 0.18, 1.90), (side_x, -hy - 2.05, 1.10), 0.026, 0.026, seg=8)
        b.cyl("steel", (side_x, -hy - 0.18, 1.48), (side_x, -hy - 2.05, 0.68), 0.016, 0.016, seg=6)
    # Recessed entry doorway & glowing parlor transom
    b.box("concrete", (sx_door, -hy - 0.05, 2.22), (1.72, 0.24, 2.38), bevel=0.02)
    b.box("boards", (sx_door, -hy + 0.02, 2.02), (1.28, 0.12, 1.96))
    b.box("warm_glass", (sx_door, -hy - 0.01, 3.12), (1.28, 0.08, 0.42))
    b.box("steel", (sx_door, -hy - 0.03, 2.90), (1.36, 0.12, 0.06))

    # Protruding 3-sided bay window on right-front (x = +1.85, all 3 storeys)
    bx = 1.85
    bay_y = -hy - 0.52
    # Corbelled stone bay base + belt trim + top cap
    b.box("concrete", (bx, bay_y + 0.10, 0.96), (2.75, 1.05, 0.34), bevel=0.03)
    b.box("concrete", (bx, bay_y + 0.04, 10.22), (2.96, 1.24, 0.36), bevel=0.03)
    # Center front bay face + 45-degree canted side wings
    b.box("plaster", (bx, -hy - 0.96, 5.62), (1.88, 0.24, 8.95), bevel=0.015)
    for side in (-1.0, 1.0):
        b.box(
            "plaster",
            (bx + side * 1.15, -hy - 0.52, 5.62),
            (0.95, 0.24, 8.95),
            rot=(0.0, 0.0, -side * 42.0),
            bevel=0.015,
        )
    # Bay windows across 3 storeys (warm_glass ground parlor, glass + steel mullions above)
    for floor_z, gmat in ((2.12, "warm_glass"), (5.12, "glass"), (8.18, "glass")):
        b.box("steel", (bx, -hy - 1.04, floor_z), (1.56, 0.14, 1.82))
        b.box(gmat, (bx, -hy - 1.06, floor_z), (1.40, 0.08, 1.66))
        b.box("steel", (bx, -hy - 1.08, floor_z), (0.06, 0.08, 1.68))
        b.box("steel", (bx, -hy - 1.08, floor_z + 0.22), (1.44, 0.08, 0.06))
        for side in (-1.0, 1.0):
            b.box(
                "steel",
                (bx + side * 1.19, -hy - 0.56, floor_z),
                (0.68, 0.14, 1.82),
                rot=(0.0, 0.0, -side * 42.0),
            )
            b.box(
                gmat,
                (bx + side * 1.21, -hy - 0.58, floor_z),
                (0.56, 0.08, 1.66),
                rot=(0.0, 0.0, -side * 42.0),
            )

    # Left-front upper storey windows & rear trail-facing windows
    for floor_z in (5.12, 8.18):
        b.box("concrete", (sx_door, -hy - 0.04, floor_z - 0.96), (1.64, 0.24, 0.14), bevel=0.015)
        b.box("concrete", (sx_door, -hy - 0.04, floor_z + 0.98), (1.72, 0.24, 0.20), bevel=0.015)
        b.box("steel", (sx_door, -hy - 0.02, floor_z), (1.44, 0.14, 1.82))
        b.box("glass", (sx_door, -hy - 0.04, floor_z), (1.28, 0.08, 1.66))
        b.box("steel", (sx_door, -hy - 0.06, floor_z), (0.05, 0.08, 1.68))
        # Rear facade windows (+Y)
        for rx, rgmat in ((-2.0, "glass"), (1.8, "warm_glass")):
            b.box("steel", (rx, hy + 0.02, floor_z), (1.42, 0.14, 1.76))
            b.box(rgmat, (rx, hy + 0.04, floor_z), (1.26, 0.08, 1.60))

    # Crisp concrete cornice & parapet roof (z = 10.2 .. 11.2 m)
    b.box("concrete", (0.0, -0.08, 10.35), (w + 0.44, d + 0.42, 0.38), bevel=0.03)
    b.box("slate", (0.0, 0.0, 10.56), (w - 0.30, d - 0.30, 0.16))
    for sy in (-1.0, 1.0):
        b.box("concrete", (0.0, sy * (hy - 0.16), 10.90), (w + 0.10, 0.34, 0.62), bevel=0.02)
    for sx in (-1.0, 1.0):
        b.box("concrete", (sx * (w * 0.5 - 0.16), 0.0, 10.90), (0.34, d - 0.32, 0.62), bevel=0.02)

    # Rooftop deck & timber/steel pergola
    b.box("deck", (0.0, -0.8, 10.68), (5.8, 6.2, 0.08), bevel=0.01)
    for px in (-2.4, 2.4):
        for py in (-3.4, 1.8):
            b.box("steel", (px, py, 11.55), (0.14, 0.14, 1.75))
    for px in (-2.4, 2.4):
        b.box("steel", (px, -0.8, 12.42), (0.14, 5.8, 0.16))
    for i in range(9):
        ry = -3.4 + 5.2 * (i / 8.0)
        b.box("boards", (0.0, ry, 12.54), (5.5, 0.09, 0.14), bevel=0.008)
    return b


def build_lakefront_overlook():
    """Modern Chicago Lakefront timber-and-concrete beach boardwalk overlook deck (~12 m wide
    x 5.5 m deep): buried concrete pier pilings, raised timber deck, low concrete & deck
    lakeside benches, and slender dark steel bollards with warm glowing caps."""
    b = KitBuilder("lakefront_overlook")
    w, d = 12.0, 5.5
    hx, hy = w * 0.5, d * 0.5

    # Buried concrete pier pilings (z = -2.5 .. 0.6 m) so beach slopes never float edges
    for px in (-5.4, -2.7, 0.0, 2.7, 5.4):
        for py in (-2.35, 0.0, 2.35):
            b.cyl("concrete", (px, py, -2.50), (px, py, 0.62), 0.24, 0.22, seg=10, smooth=False)
    # Concrete perimeter foundation grade beams & trail-side approach step
    b.box("concrete", (0.0, hy + 0.36, 0.24), (6.4, 0.85, 0.96), bevel=0.025)
    b.box("concrete", (0.0, hy + 0.92, -0.05), (6.4, 0.55, 0.65), bevel=0.025)

    # Heavy timber substructure joists & raised boardwalk deck platform (z = 0.6 .. 0.85 m)
    for jy in (-2.2, -0.75, 0.75, 2.2):
        b.box("boards", (0.0, jy, 0.54), (w - 0.25, 0.22, 0.22))
    b.box("deck", (0.0, 0.0, 0.725), (w, d, 0.25), bevel=0.02)
    # Dark steel perimeter edge fascia trim
    for sy in (-1.0, 1.0):
        b.box("steel", (0.0, sy * (hy - 0.03), 0.73), (w + 0.04, 0.07, 0.26), bevel=0.008)
    for sx in (-1.0, 1.0):
        b.box("steel", (sx * (hx - 0.03), 0.0, 0.73), (0.07, d - 0.06, 0.26), bevel=0.008)

    # Low concrete & deck lakeside benches along the front corners & side wings
    for bx in (-3.6, 3.6):
        by = -hy + 0.68
        for lx in (-1.35, 0.0, 1.35):
            b.box("concrete", (bx + lx, by, 1.04), (0.16, 0.58, 0.38), bevel=0.015)
        b.box("deck", (bx, by, 1.26), (3.40, 0.64, 0.09), bevel=0.015)
    for sx in (-1.0, 1.0):
        bx = sx * (hx - 0.72)
        for ly in (-0.85, 0.35, 1.55):
            b.box("concrete", (bx, ly, 1.04), (0.58, 0.16, 0.38), bevel=0.015)
        b.box("deck", (bx, 0.35, 1.26), (0.64, 3.00, 0.09), bevel=0.015)

    # Slender dark steel bollards with warm glowing caps along lakeside edge & trail entrance
    bollard_pts = [
        (-hx + 0.22, -hy + 0.22),
        (-1.85, -hy + 0.22),
        (1.85, -hy + 0.22),
        (hx - 0.22, -hy + 0.22),
        (-3.05, hy - 0.22),
        (3.05, hy - 0.22),
    ]
    for lx, ly in bollard_pts:
        b.cyl("steel", (lx, ly, 0.84), (lx, ly, 1.66), 0.075, 0.075, seg=10)
        b.cyl("warm_glass", (lx, ly, 1.66), (lx, ly, 1.78), 0.068, 0.068, seg=10)
        b.cyl("steel", (lx, ly, 1.78), (lx, ly, 1.84), 0.082, 0.072, seg=10)

    # Sleek lakeside steel safety railing along water-facing (-Y) front edge
    for rz in (1.28, 1.62):
        b.cyl(
            "steel", (-hx + 0.22, -hy + 0.22, rz), (hx - 0.22, -hy + 0.22, rz), 0.022, 0.022, seg=8
        )
    for rx in (-3.8, 0.0, 3.8):
        b.cyl("steel", (rx, -hy + 0.22, 0.84), (rx, -hy + 0.22, 1.64), 0.03, 0.03, seg=8)
    return b


def build_pch_guardrail():
    """6.0 m California Highway 1 coastal cliffside guardrail along X: 3 weathered timber
    posts, galvanized steel W-beam rail, and a center white/ochre delineator paddle."""
    b = KitBuilder("pch_guardrail")
    length = 6.05

    # 3 heavy weathered timber posts (buried z = -0.9 .. +0.82 m) + spacer blocks
    for px in (-2.0, 0.0, 2.0):
        b.box("boards", (px, 0.0, -0.04), (0.18, 0.18, 1.72), bevel=0.015)
        b.box("wood", (px, 0.13, 0.605), (0.16, 0.10, 0.34), bevel=0.01)

    # Galvanized steel W-beam guardrail on roadward (+Y) face (z = 0.45 .. 0.76 m)
    beam_y = 0.20
    b.box("steel", (0.0, beam_y, 0.605), (length, 0.03, 0.31), bevel=0.005)
    # Upper & lower projecting ribs + center valley rib for crisp W-beam profile
    b.box("steel", (0.0, beam_y + 0.025, 0.715), (length, 0.045, 0.08), bevel=0.008)
    b.box("steel", (0.0, beam_y + 0.025, 0.495), (length, 0.045, 0.08), bevel=0.008)
    b.box("steel", (0.0, beam_y - 0.015, 0.605), (length, 0.035, 0.07), bevel=0.006)
    # Post splice plates & bolt bosses
    for px in (-2.0, 0.0, 2.0):
        b.box("steel", (px, beam_y + 0.038, 0.605), (0.12, 0.03, 0.28), bevel=0.005)

    # Slender white reflective highway delineator paddle at center post (z = 0.82 .. 1.08 m)
    b.box("white", (0.0, 0.0, 0.95), (0.09, 0.026, 0.26), bevel=0.005)
    b.box("ochre", (0.0, 0.0, 1.01), (0.078, 0.032, 0.065), bevel=0.003)
    b.box("warm_glass", (0.0, 0.0, 0.93), (0.065, 0.032, 0.045))
    return b


def build_pch_turnout_wall():
    """10.0 m Big Sur / Highway 1 scenic turnout stone-and-concrete cliffside parapet wall
    along X with end stone piers, concrete coping cap, and roadward (+Y) timber bench."""
    b = KitBuilder("pch_turnout_wall")

    # Buried concrete foundation footing (z = -1.5 .. 0.0 m)
    b.box("concrete", (0.0, 0.0, -0.75), (10.30, 0.86, 1.50), bevel=0.03)

    # Battered low stone parapet wall (z = 0.0 .. 0.85 m)
    b.box("stone", (0.0, 0.0, 0.20), (9.20, 0.62, 0.42), bevel=0.03)
    b.box("stone", (0.0, 0.0, 0.62), (9.12, 0.55, 0.46), bevel=0.03)

    # Overhanging concrete coping cap (z = 0.85 .. 0.96 m)
    b.box("concrete", (0.0, 0.0, 0.905), (9.28, 0.64, 0.11), bevel=0.02)

    # Twin end stone piers (z = 0.0 .. 1.18 m) with stepped concrete pier caps
    for sx in (-4.72, 4.72):
        b.box("stone", (sx, 0.0, 0.59), (0.80, 0.78, 1.18), bevel=0.035)
        b.box("concrete", (sx, 0.0, 1.24), (0.90, 0.88, 0.12), bevel=0.025)

    # Weathered timber coastal overlook bench tucked on the roadward (+Y) side
    by = 0.72
    for lx in (-1.25, 0.0, 1.25):
        b.box("boards", (lx, by, 0.14), (0.14, 0.44, 0.56), bevel=0.015)
        b.box("steel", (lx, by - 0.16, 0.58), (0.05, 0.06, 0.36))
    for sy_off in (-0.12, 0.0, 0.12):
        b.box("wood", (0.0, by + sy_off, 0.45), (3.10, 0.105, 0.055), bevel=0.01)
    b.box("wood", (0.0, by - 0.17, 0.74), (3.10, 0.055, 0.16), rot=(-10.0, 0.0, 0.0), bevel=0.01)
    return b


def build_pch_route_sign():
    """Classic California State Route 1 / Pacific Coast Highway roadside sign (~2.6 m tall)
    facing -X toward oncoming riders, with green/white main plate and SCENIC sub-plate."""
    b = KitBuilder("pch_route_sign")

    # Concrete collar footing & buried timber post (z = -0.60 .. 2.55 m)
    b.cyl("concrete", (0.0, 0.0, -0.60), (0.0, 0.0, 0.08), 0.24, 0.20, seg=8, smooth=False)
    b.box("boards", (0.0, 0.0, 0.975), (0.14, 0.14, 3.15), bevel=0.012)

    # Main green highway sign plate (0.72 x 0.78 m) facing -X
    px = -0.09
    pz = 2.08
    pw, ph = 0.72, 0.78
    b.box("green", (px, 0.0, pz), (0.032, pw, ph), bevel=0.005)
    # Crisp white perimeter border strips on -X face
    bx = px - 0.018
    for sz in (-1.0, 1.0):
        b.box("white", (bx, 0.0, pz + sz * (ph * 0.5 - 0.025)), (0.012, pw - 0.02, 0.042))
    for sy in (-1.0, 1.0):
        b.box("white", (bx, sy * (pw * 0.5 - 0.025), pz), (0.012, 0.042, ph - 0.02))
    # California route shield emblem silhouette on -X face
    b.box("white", (bx - 0.005, 0.0, pz + 0.02), (0.012, 0.42, 0.44), bevel=0.015)
    b.box("green", (bx - 0.009, 0.0, pz + 0.16), (0.010, 0.34, 0.08))
    b.box("green", (bx - 0.009, 0.0, pz - 0.04), (0.010, 0.34, 0.24))
    b.box("white", (bx - 0.013, 0.0, pz - 0.04), (0.008, 0.07, 0.18))

    # Smaller white/ochre "SCENIC" sub-plate underneath
    sub_z = 1.48
    b.box("white", (px, 0.0, sub_z), (0.030, 0.66, 0.26), bevel=0.005)
    b.box("ochre", (bx, 0.0, sub_z + 0.055), (0.012, 0.58, 0.075))
    b.box("green", (bx, 0.0, sub_z - 0.045), (0.012, 0.50, 0.065))

    # Steel mounting bracket straps on post
    for mz in (1.48, 1.88, 2.28):
        b.box("steel", (-0.01, 0.0, mz), (0.16, 0.16, 0.035))
    return b


# ---------------------------------------------------------------------------------------------
# Hero Boats (curved hulls, cabins, rigging, deck gear)
# ---------------------------------------------------------------------------------------------


def _boat_stations(length, beam, freeboard, draft, bow_flare=1.25, stern_width=0.65, n=13):
    """Generates realistic boat cross-sections along +X with graceful sheer and flared bow."""
    stations = []
    half = beam * 0.5
    for i in range(n):
        t = i / (n - 1)  # 0 stern -> 1 bow
        x = (t - 0.5) * length
        if t < 0.995:
            # Fuller forward/midships, tapered cleanly to stem
            w_profile = math.sin(math.pi * (0.22 + 0.78 * t)) ** 0.72
            w_profile = max(w_profile, stern_width * (1.0 - t) + 0.03)
            w = half * min(1.0, w_profile * 1.06)
        else:
            w = 0.06
        # Classic sweeping sheerline (rises gracefully at bow, slightly at stern)
        sheer = (
            freeboard
            + 0.42 * max(0.0, (t - 0.45) / 0.55) ** 1.8
            + 0.12 * max(0.0, (0.3 - t) / 0.3) ** 1.6
        )
        flare = 1.0 + (bow_flare - 1.0) * max(0.0, (t - 0.5) / 0.5) ** 1.4
        dp = draft * (0.65 + 0.35 * math.sin(math.pi * t))
        stations.append(
            [
                (x, w * flare, sheer),
                (x, w * 0.98, sheer * 0.48),
                (x, w * 0.88, 0.0),
                (x, w * 0.55, -dp * 0.75),
                (x, 0.0, -dp),
                (x, -w * 0.55, -dp * 0.75),
                (x, -w * 0.88, 0.0),
                (x, -w * 0.98, sheer * 0.48),
                (x, -w * flare, sheer),
            ]
        )
    return stations


def build_boat_trawler():
    """16.5 m coastal fishing trawler: flared hull, bulwarks, raked wheelhouse with windows,
    mast + derrick boom, funnel, and stern gantry."""
    b = KitBuilder("boat_trawler")
    L, B = 16.5, 5.2
    stations = _boat_stations(
        L, B, freeboard=1.45, draft=1.35, bow_flare=1.22, stern_width=0.72, n=15
    )
    b.loft_hull("red", "deck", "white", stations)
    deck_z = 1.45

    # Bulwark railing posts around aft deck
    for x in (-6.8, -5.0, -3.2, -1.4):
        for sy in (-1.0, 1.0):
            b.cyl("white", (x, sy * 2.25, deck_z), (x, sy * 2.25, deck_z + 0.78), 0.05, 0.05, seg=6)
    for sy in (-1.0, 1.0):
        b.box("white", (-4.1, sy * 2.25, deck_z + 0.78), (5.6, 0.11, 0.09))

    # Forward wheelhouse & accommodation cabin
    b.box("white", (1.2, 0.0, deck_z + 1.25), (4.8, 3.2, 2.50))
    # Overhanging bridge visor roof
    b.box("white", (1.35, 0.0, deck_z + 2.58), (5.4, 3.55, 0.18))
    # Wrap-around bridge windows
    b.box("glass", (3.62, 0.0, deck_z + 1.85), (0.08, 2.65, 0.82))
    for sy in (-1.0, 1.0):
        b.box("glass", (1.6, sy * 1.62, deck_z + 1.85), (3.4, 0.08, 0.82))
    # Exhaust funnel behind wheelhouse
    b.cyl("navy", (-1.5, 0.0, deck_z + 1.4), (-1.7, 0.0, deck_z + 3.6), 0.34, 0.28, seg=10)
    b.cyl("red", (-1.65, 0.0, deck_z + 3.0), (-1.70, 0.0, deck_z + 3.5), 0.30, 0.29, seg=10)

    # Mast, radar bar, derrick boom & stays
    mast_x = 0.2
    b.cyl(
        "white", (mast_x, 0.0, deck_z + 2.5), (mast_x - 0.3, 0.0, deck_z + 9.2), 0.14, 0.07, seg=8
    )
    b.box("white", (mast_x - 0.1, 0.0, deck_z + 5.8), (0.14, 2.4, 0.10))
    b.box("white", (mast_x + 0.25, 0.0, deck_z + 4.4), (0.85, 0.18, 0.14))  # radar scanner
    # Cargo/net boom angled aft
    b.cyl("white", (mast_x, 0.0, deck_z + 3.1), (-5.2, 0.0, deck_z + 6.4), 0.09, 0.06, seg=8)
    # Stays/shrouds
    b.cyl(
        "cable", (mast_x - 0.28, 0.0, deck_z + 9.0), (7.6, 0.0, deck_z + 0.7), 0.025, 0.025, seg=4
    )
    for sy in (-1.0, 1.0):
        b.cyl(
            "cable",
            (mast_x - 0.2, 0.0, deck_z + 7.5),
            (mast_x - 0.4, sy * 2.35, deck_z + 0.4),
            0.025,
            0.025,
            seg=4,
        )

    # Stern A-frame net gantry
    for sy in (-1.0, 1.0):
        b.cyl("steel", (-6.8, sy * 2.0, deck_z), (-7.2, sy * 1.4, deck_z + 3.8), 0.11, 0.09, seg=8)
    b.cyl("steel", (-7.2, -1.4, deck_z + 3.8), (-7.2, 1.4, deck_z + 3.8), 0.10, 0.10, seg=8)
    # Deck fish boxes
    b.box("wood", (-3.8, 0.8, deck_z + 0.38), (1.3, 0.85, 0.72))
    b.box("wood", (-3.8, -0.7, deck_z + 0.38), (1.3, 0.85, 0.72))
    return b


def build_boat_sail():
    """11.5 m coastal sloop: sleek navy hull, white coachroof with portlights, tall rigged mast
    with spreaders, furled sail on boom, and cockpit."""
    b = KitBuilder("boat_sail")
    L, B = 11.5, 3.4
    stations = _boat_stations(
        L, B, freeboard=1.02, draft=0.95, bow_flare=1.15, stern_width=0.52, n=15
    )
    b.loft_hull("navy", "white", "wood", stations)
    deck_z = 1.02

    # Coachroof cabin
    b.box("white", (0.4, 0.0, deck_z + 0.34), (4.2, 1.95, 0.68), bevel=0.08)
    for sy in (-1.0, 1.0):
        b.box("glass", (0.5, sy * 0.98, deck_z + 0.38), (2.6, 0.05, 0.24))
    # Cockpit coaming & helm pedestal
    for sy in (-1.0, 1.0):
        b.box("deck", (-3.1, sy * 0.92, deck_z + 0.16), (2.8, 0.42, 0.32))
    b.cyl("steel", (-3.7, 0.0, deck_z), (-3.7, 0.0, deck_z + 0.95), 0.07, 0.06, seg=8)
    b.cyl("steel", (-3.6, 0.0, deck_z + 0.85), (-3.7, 0.0, deck_z + 0.85), 0.42, 0.42, seg=14)

    # Tall tapered mast, spreaders, boom + furled sail, stays
    mx = 0.9
    b.cyl("white", (mx, 0.0, deck_z + 0.6), (mx - 0.15, 0.0, deck_z + 12.8), 0.10, 0.055, seg=8)
    b.cyl(
        "white",
        (mx - 0.06, -1.15, deck_z + 6.8),
        (mx - 0.06, 1.15, deck_z + 6.8),
        0.035,
        0.035,
        seg=6,
    )
    # Boom & canvas furled sail cover
    b.cyl("white", (mx, 0.0, deck_z + 1.85), (mx - 4.4, 0.0, deck_z + 1.92), 0.065, 0.055, seg=8)
    b.cyl("navy", (mx - 0.2, 0.0, deck_z + 2.02), (mx - 4.2, 0.0, deck_z + 2.04), 0.14, 0.11, seg=8)
    # Standing rigging (forestay, backstay, shrouds)
    b.cyl(
        "cable",
        (mx - 0.15, 0.0, deck_z + 12.6),
        (L * 0.48, 0.0, deck_z + 0.35),
        0.018,
        0.018,
        seg=4,
    )
    b.cyl(
        "cable",
        (mx - 0.15, 0.0, deck_z + 12.6),
        (-L * 0.47, 0.0, deck_z + 0.20),
        0.018,
        0.018,
        seg=4,
    )
    for sy in (-1.0, 1.0):
        b.cyl(
            "cable",
            (mx - 0.12, 0.0, deck_z + 12.2),
            (mx - 0.06, sy * 1.15, deck_z + 6.8),
            0.016,
            0.016,
            seg=4,
        )
        b.cyl(
            "cable",
            (mx - 0.06, sy * 1.15, deck_z + 6.8),
            (mx - 0.25, sy * 1.52, deck_z + 0.1),
            0.016,
            0.016,
            seg=4,
        )
    return b


def build_boat_launch():
    """8.2 m coastal wooden/painted work launch with forward cuddy cabin & windshield."""
    b = KitBuilder("boat_launch")
    L, B = 8.2, 2.75
    stations = _boat_stations(
        L, B, freeboard=0.88, draft=0.65, bow_flare=1.18, stern_width=0.68, n=13
    )
    b.loft_hull("green", "deck", "wood", stations)
    deck_z = 0.88

    # Forward cuddy cabin + angled windshield
    b.box("white", (0.7, 0.0, deck_z + 0.58), (2.6, 1.85, 1.16), bevel=0.06)
    b.box("white", (0.65, 0.0, deck_z + 1.20), (2.9, 2.05, 0.10))
    b.box("glass", (2.02, 0.0, deck_z + 0.82), (0.06, 1.55, 0.52), rot=(0.0, -14.0, 0.0))
    for sy in (-1.0, 1.0):
        b.box("glass", (0.8, sy * 0.94, deck_z + 0.82), (1.7, 0.06, 0.48))
        b.box("deck", (-2.1, sy * 0.90, deck_z + 0.18), (2.4, 0.38, 0.34))
    # Stern mast light & grab rails
    b.cyl("steel", (0.5, 0.0, deck_z + 1.24), (0.3, 0.0, deck_z + 2.35), 0.035, 0.03, seg=6)
    return b


def build_boat_dinghy():
    """4.6 m traditional wooden coastal rowing skiff with open cockpit, thwarts, and rested oars."""
    b = KitBuilder("boat_dinghy")
    L, B = 4.6, 1.75
    stations = _boat_stations(
        L, B, freeboard=0.52, draft=0.32, bow_flare=1.15, stern_width=0.55, n=11
    )
    # Loft the hull, then add recessed dark bilge floor & raised thwarts
    b.loft_hull("wood", "deck", "boards", stations)
    # Recessed dark cockpit bilge floor + side benches so it reads as an open skiff
    b.box("boards", (0.0, 0.0, 0.54), (3.1, 1.15, 0.04))
    # Three wooden thwarts raised cleanly above cockpit floor
    for tx in (-1.1, 0.0, 1.15):
        b.box("deck", (tx, 0.0, 0.58), (0.34, 1.48, 0.05))
    # Pair of oars resting along inside
    for sy in (-0.42, 0.42):
        b.cyl("deck", (-1.5, sy, 0.64), (1.4, sy * 0.85, 0.64), 0.028, 0.024, seg=6)
        b.box("deck", (-1.65, sy, 0.64), (0.62, 0.14, 0.025))
    return b


def build_harbour_crane():
    """Traditional weathered timber harbour cargo crane with stacked oak barrels and crates."""
    b = KitBuilder("harbour_crane")
    # Heavy stone & timber pedestal base
    b.box("stone", (0, 0, -0.4), (2.4, 2.4, 1.8))
    b.box("boards", (0, 0, 0.62), (2.2, 2.2, 0.24))
    # Main vertical king-post + 4 angled stabilizing struts
    b.box("boards", (0, 0, 3.3), (0.36, 0.36, 5.4))
    for sx in (-1.0, 1.0):
        for sy in (-1.0, 1.0):
            b.cyl("boards", (sx * 0.95, sy * 0.95, 0.7), (0, 0, 2.55), 0.11, 0.11, seg=6)
    # Angled timber jib boom extending out along +X toward the water
    b.cyl("boards", (0, 0, 1.7), (4.6, 0, 6.2), 0.14, 0.11, seg=8)
    b.cyl("boards", (0, 0, 5.75), (4.5, 0, 6.15), 0.09, 0.09, seg=6)
    # Iron winch drum, pulley wheel & hanging cable + hook
    b.cyl("steel", (-0.35, -0.42, 1.55), (-0.35, 0.42, 1.55), 0.24, 0.24, seg=12)
    b.cyl("steel", (4.55, -0.10, 6.00), (4.55, 0.10, 6.00), 0.22, 0.22, seg=10)
    b.cyl("cable", (4.55, 0, 5.85), (4.55, 0, 3.10), 0.025, 0.025, seg=4)
    b.box("steel", (4.55, 0, 2.95), (0.18, 0.18, 0.32))
    # Stacked cargo crates & oak barrels beside base
    b.box("deck", (-1.85, 0.65, 0.45), (0.95, 0.95, 0.90))
    b.box("deck", (-1.75, -0.45, 0.38), (0.80, 0.80, 0.76))
    b.box("wood", (-1.80, 0.20, 1.25), (0.76, 0.76, 0.70), rot=(0, 0, 18))
    for bx, by, bz in ((1.65, 0.95, 0.0), (1.75, 1.75, 0.0), (1.05, 1.55, 0.0)):
        b.cyl("wood", (bx, by, bz), (bx, by, bz + 0.92), 0.30, 0.30, seg=10)
        b.cyl("steel", (bx, by, bz + 0.18), (bx, by, bz + 0.23), 0.315, 0.315, seg=10)
        b.cyl("steel", (bx, by, bz + 0.70), (bx, by, bz + 0.75), 0.315, 0.315, seg=10)
    return b


def build_stone_arch():
    """Weathered coastal stone gateway arch spanning a 7.5 m road (road runs along +X,
    arch piers stand at Y = +/- 4.6 m, clearance 6.4 m at crown). Twin warm lanterns."""
    b = KitBuilder("stone_arch")
    span_half = 4.6  # clear width ~7.0 m between inner faces
    pier_w, pier_d, pier_h = 2.4, 2.2, 5.2
    # Two heavy battered stone piers sunk 2 m into the ground
    for sy in (-1.0, 1.0):
        b.box("stone", (0, sy * span_half, -0.8), (pier_d + 0.4, pier_w + 0.4, 2.6))
        b.box("stone", (0, sy * span_half, pier_h * 0.5), (pier_d, pier_w, pier_h))
        # Capital cornice at springline
        b.box("stone", (0, sy * span_half, pier_h + 0.14), (pier_d + 0.34, pier_w + 0.34, 0.28))
        # Iron lantern bracket + warm lantern facing road
        ly = sy * (span_half - pier_w * 0.5 - 0.32)
        b.box("steel", (0, sy * (span_half - pier_w * 0.5 - 0.16), 3.65), (0.08, 0.42, 0.08))
        b.box("warm_glass", (0, ly, 3.35), (0.28, 0.28, 0.40))
        b.box("steel", (0, ly, 3.60), (0.36, 0.36, 0.08))
        b.box("steel", (0, ly, 3.12), (0.32, 0.32, 0.06))

    # Semicircular stone voussoir arch ring spanning between piers (radius 3.5 m, center z = 4.2)
    r_in, r_out = 3.5, 4.4
    zc = 4.2
    n_seg = 11
    for i in range(n_seg):
        a = math.radians(15.0 + 150.0 * (i + 0.5) / n_seg)
        rm = 0.5 * (r_in + r_out)
        ay = rm * math.cos(a)
        az = zc + rm * math.sin(a)
        is_key = i == n_seg // 2
        thick = (r_out - r_in) + (0.28 if is_key else 0.0)
        depth = pier_d + (0.24 if is_key else 0.08)
        arc_w = (math.radians(150.0) / n_seg) * r_out * 1.06
        b.box("stone", (0, ay, az), (depth, arc_w, thick), rot=(math.degrees(a) - 90.0, 0, 0))

    # Upper spandrel wall & parapet coping stones across the top
    b.box("stone", (0, 0, 7.65), (pier_d - 0.15, span_half * 2.0 + pier_w, 1.65))
    b.box("stone", (0, 0, 8.60), (pier_d + 0.30, span_half * 2.0 + pier_w + 0.36, 0.30))
    # Subtle stone battlements/crenels along parapet top
    for i in range(7):
        by = -span_half + (span_half * 2.0) * (i / 6.0)
        b.box("stone", (0, by, 9.05), (pier_d * 0.72, 0.95, 0.62))
    return b


# ---------------------------------------------------------------------------------------------
# Hero Timber Pier & Stone Lighthouse
# ---------------------------------------------------------------------------------------------


def build_pier(length):
    """Detailed timber harbour pier along +X: planked deck, stringers, cross-braced round pilings,
    timber curbs, mooring bollards, and harbour lamp posts."""
    b = KitBuilder(f"pier_{length}")
    w = 4.6
    # Longitudinal heavy timber stringers
    for sy in (-1.9, -0.65, 0.65, 1.9):
        b.box("boards", (length * 0.5, sy, -0.22), (length, 0.24, 0.30))
    # Planked deck (grouped into textured boards with subtle bevel gaps)
    b.box("deck", (length * 0.5, 0.0, -0.02), (length, w, 0.14))
    # Timber wheel-guard curbs on both edges
    for sy in (-1.0, 1.0):
        b.box("boards", (length * 0.5, sy * (w * 0.5 - 0.12), 0.14), (length, 0.22, 0.18))

    # Heavy pilings every 4.5 m with diagonal X-bracing under deck
    xs = [x for x in range(2, int(length), 4)]
    if xs[-1] < length - 1.5:
        xs.append(length - 1.0)
    for x in xs:
        for sy in (-1.0, 1.0):
            py = sy * (w * 0.5 - 0.22)
            # Round piling from seabed (-6.5 m) to bollard top (+0.65 m)
            top = 0.72 if (int(x) % 8 == 2) else 0.20
            b.cyl("wood", (x, py, -6.5), (x, py, top), 0.18, 0.16, seg=10)
            if top > 0.5:
                # Iron cleat / cap ring on mooring bollard piling
                b.cyl("steel", (x, py, top + 0.02), (x, py, top + 0.08), 0.17, 0.17, seg=10)
                b.cyl(
                    "steel",
                    (x, py - 0.28, top - 0.12),
                    (x, py + 0.28, top - 0.12),
                    0.045,
                    0.045,
                    seg=6,
                )
        # Cross cap beam & diagonal timber bracing
        b.box("boards", (x, 0.0, -0.45), (0.28, w + 0.10, 0.26))
        b.cyl("boards", (x, -2.0, -0.5), (x, 2.0, -2.8), 0.08, 0.08, seg=6)
        b.cyl("boards", (x, 2.0, -0.5), (x, -2.0, -2.8), 0.08, 0.08, seg=6)

    # Pier lantern posts every 18 m along one side + end beacon
    for x in range(14, int(length), 18):
        py = -(w * 0.5 - 0.25)
        b.cyl("steel", (x, py, 0.1), (x, py, 3.6), 0.065, 0.045, seg=8)
        b.box("steel", (x, py, 3.72), (0.36, 0.36, 0.08))
        b.box("warm_glass", (x, py, 3.92), (0.26, 0.26, 0.34))
        b.cyl("copper", (x, py, 4.10), (x, py, 4.35), 0.28, 0.04, seg=8)
    # End timber cross-beam
    b.box("boards", (length - 0.12, 0.0, 0.14), (0.24, w, 0.20))
    return b


def build_lighthouse():
    """Hero stone & banded coastal lighthouse with corbelled gallery, iron railing, glass lantern,
    glowing optic, weathered copper dome cupola, and attached stone keeper's cottage."""
    b = KitBuilder("lighthouse")
    # Heavy octagonal stone foundation plinth
    b.cyl("stone", (0, 0, -2.0), (0, 0, 2.4), 5.2, 4.4, seg=8, smooth=False)
    b.cyl("stone", (0, 0, 2.4), (0, 0, 2.85), 4.6, 4.6, seg=8, smooth=False)

    # Tapered masonry tower in crisp red & white bands (z = 2.85 .. 21.0)
    bands = [
        (2.85, 6.8, "white"),
        (6.8, 10.2, "red"),
        (10.2, 14.0, "white"),
        (14.0, 17.4, "red"),
        (17.4, 21.0, "white"),
    ]
    for z0, z1, mat in bands:
        r0 = 3.55 - (z0 - 2.85) * 0.068
        r1 = 3.55 - (z1 - 2.85) * 0.068
        b.cyl(mat, (0, 0, z0), (0, 0, z1), r0, r1, seg=20, cap=True, smooth=True)

    # Small recessed tower windows
    for wz, ang in ((7.8, 0.0), (12.2, 1.6), (16.4, 3.2)):
        r = 3.55 - (wz - 2.85) * 0.068
        wx, wy = r * math.cos(ang), r * math.sin(ang)
        b.box("boards", (wx, wy, wz), (0.28, 0.65, 0.95), rot=(0, 0, math.degrees(ang)))
        b.box(
            "warm_glass",
            (wx * 1.01, wy * 1.01, wz),
            (0.18, 0.48, 0.78),
            rot=(0, 0, math.degrees(ang)),
        )

    # Corbelled stone gallery deck at z = 21.0
    b.cyl("stone", (0, 0, 20.6), (0, 0, 21.2), 2.35, 3.05, seg=20, smooth=False)
    b.cyl("stone", (0, 0, 21.2), (0, 0, 21.55), 3.12, 3.12, seg=20, smooth=False)
    # Gallery iron railing
    for i in range(16):
        a = 2.0 * math.pi * i / 16.0
        a2 = 2.0 * math.pi * (i + 1) / 16.0
        rx, ry = 2.95 * math.cos(a), 2.95 * math.sin(a)
        rx2, ry2 = 2.95 * math.cos(a2), 2.95 * math.sin(a2)
        b.cyl("steel", (rx, ry, 21.55), (rx, ry, 22.65), 0.045, 0.045, seg=6)
        b.cyl("steel", (rx, ry, 22.60), (rx2, ry2, 22.60), 0.05, 0.05, seg=6)
        b.cyl("steel", (rx, ry, 22.10), (rx2, ry2, 22.10), 0.035, 0.035, seg=6)

    # Lantern room: lower stone drum, glass cylinder with iron mullions, glowing Fresnel core
    b.cyl("stone", (0, 0, 21.55), (0, 0, 22.50), 1.95, 1.95, seg=16, smooth=True)
    b.cyl("glass", (0, 0, 22.50), (0, 0, 25.00), 1.78, 1.78, seg=16, smooth=True)
    b.cyl("warm_glass", (0, 0, 22.95), (0, 0, 24.55), 0.92, 0.92, seg=12, smooth=True)
    for i in range(12):
        a = 2.0 * math.pi * i / 12.0
        b.cyl(
            "steel",
            (1.84 * math.cos(a), 1.84 * math.sin(a), 22.50),
            (1.84 * math.cos(a), 1.84 * math.sin(a), 25.05),
            0.045,
            0.045,
            seg=6,
        )

    # Weathered copper cupola dome & lightning finial
    b.cyl("copper", (0, 0, 25.00), (0, 0, 25.30), 2.15, 2.05, seg=16, smooth=True)
    b.cyl("copper", (0, 0, 25.30), (0, 0, 26.20), 1.95, 1.20, seg=16, smooth=True)
    b.cyl("copper", (0, 0, 26.20), (0, 0, 26.85), 1.20, 0.18, seg=16, smooth=True)
    b.cyl("steel", (0, 0, 26.80), (0, 0, 28.00), 0.06, 0.02, seg=6)

    # Attached stone keeper's cottage beside tower (+X)
    cx = 6.2
    b.box("stone", (cx, 0, -0.5), (6.4, 5.4, 3.0))
    b.box("stone", (cx, 0, 2.45), (6.0, 5.0, 2.95))
    b.prism_gable("stone", cx, 0, 3.92, 6.0, 5.0, 2.5 * math.tan(math.radians(38.0)))
    b.roof_pitched("slate", "boards", cx, 0, 3.92, 6.0, 5.0, 38.0, over_x=0.35, over_y=0.45)
    b.chimney(cx + 2.2, 0.3, 3.2, 6.9)
    b.door(cx - 0.6, -2.5, 0.98, -1.0, w=1.05, h=2.05)
    b.window(cx + 1.4, -2.5, 2.45, -1.0, w=0.95, h=1.10, lit=True)
    b.window(cx + 0.5, 2.5, 2.45, 1.0, w=0.95, h=1.10, lit=True)
    return b


# ---------------------------------------------------------------------------------------------
# Build all & optional Blender preview render
# ---------------------------------------------------------------------------------------------


def build_lsd_highway_seg():
    """Four-lane divided coastal boulevard/interstate segment (Lake Shore Drive, 20 m long x 16 m
    wide): dark asphalt carriageways, crisp white lane markings, concrete Jersey median barrier,
    curbs, deep slope skirt, and a central dual-arm cobra-head highway streetlight."""
    b = KitBuilder("lsd_highway_seg")
    length, width = 20.4, 16.0  # slight X overlap so curved segments join seamlessly
    # Concrete roadbed & deep slope skirt so edges never float on coastal grade
    b.box("concrete", (0.0, 0.0, -1.10), (length, width + 0.6, 2.30))
    # Dark asphalt 4-lane roadway deck
    b.box("slate", (0.0, 0.0, 0.09), (length + 0.02, width - 0.4, 0.14))
    # Outer concrete curbs
    for sy in (-1.0, 1.0):
        b.box("concrete", (0.0, sy * (width * 0.5 + 0.05), 0.18), (length, 0.42, 0.24))
    # Central concrete Jersey median barrier
    b.box("concrete", (0.0, 0.0, 0.34), (length, 0.88, 0.42))
    b.box("concrete", (0.0, 0.0, 0.72), (length, 0.46, 0.48))
    # White solid edge lines & inner median lines
    for ly in (-7.45, -0.88, 0.88, 7.45):
        b.box("white", (0.0, ly, 0.165), (length, 0.14, 0.02))
    # Dashed lane dividers separating fast/slow lanes on each carriageway
    for ly in (-4.16, 4.16):
        for lx in (-5.0, 5.0):
            b.box("white", (lx, ly, 0.165), (4.2, 0.14, 0.02))
    # Tall central dual-arm highway cobra-head streetlight pole
    b.cyl("steel", (0.0, 0.0, 0.90), (0.0, 0.0, 9.4), 0.14, 0.08, seg=10)
    for sy in (-1.0, 1.0):
        b.cyl("steel", (0.0, 0.0, 9.1), (0.0, sy * 2.6, 9.55), 0.06, 0.045, seg=8)
        b.box("steel", (0.0, sy * 2.75, 9.55), (0.36, 0.72, 0.14))
        b.box("warm_glass", (0.0, sy * 2.75, 9.46), (0.28, 0.56, 0.06))
    return b


def build_road_mark_bike():
    """Crisp white thermoplastic cycle-path bicycle stencil perpendicular across the lane (+Y)
    with a bold forward directional arrow pointing along travel (+X), flush to tarmac.
    Scaled 0.76x and shifted -0.57 m in Y so placements at +1.35 m sit centered at +0.78 m
    inside the right grey pavement lane, clear of both the yellow centreline and dark shoulder."""
    b = KitBuilder("road_mark_bike")
    z, t = 0.002, 0.004
    # UE5 glTF import negates Blender Y (y_ue = -y_bl), so +0.57 m in Blender shifts -57 cm
    # inward in UE5 (+1.35 m -> +0.78 m, centered in the right grey pavement lane).
    s, oy = 0.76, 0.57
    # Bicycle perpendicular to the lane (wheels across Y at -0.44 / +0.44, centered at x = -0.35)
    bx = -0.35
    for wy in (-0.44, 0.44):
        for i in range(12):
            a0 = 2.0 * math.pi * i / 12.0
            a1 = 2.0 * math.pi * (i + 1) / 12.0
            r = 0.25
            p0 = (bx + r * math.sin(a0), wy + r * math.cos(a0), z)
            p1 = (bx + r * math.sin(a1), wy + r * math.cos(a1), z)
            mx, my = (p0[0] + p1[0]) * 0.5, (p0[1] + p1[1]) * 0.5
            seg_l = math.hypot(p1[0] - p0[0], p1[1] - p0[1]) + 0.02
            ang = math.degrees(math.atan2(p1[1] - p0[1], p1[0] - p0[0]))
            b.box("white", (mx * s, my * s + oy, z), (seg_l * s, 0.065 * s, t), rot=(0.0, 0.0, ang))
    # Diamond bicycle frame across Y (top tube, seat tube, down tube, fork, handlebars & saddle)
    b.box("white", ((bx + 0.06) * s, oy, z), (0.065 * s, 0.72 * s, t))
    b.box("white", ((bx + 0.28) * s, -0.04 * s + oy, z), (0.065 * s, 0.54 * s, t))
    b.box(
        "white",
        ((bx + 0.17) * s, 0.24 * s + oy, z),
        (0.44 * s, 0.065 * s, t),
        rot=(0.0, 0.0, -34.0),
    )
    b.box(
        "white",
        ((bx + 0.17) * s, -0.24 * s + oy, z),
        (0.44 * s, 0.065 * s, t),
        rot=(0.0, 0.0, 34.0),
    )
    b.box("white", ((bx + 0.42) * s, 0.34 * s + oy, z), (0.065 * s, 0.22 * s, t))  # handlebars
    b.box("white", ((bx + 0.36) * s, -0.32 * s + oy, z), (0.075 * s, 0.22 * s, t))  # saddle
    # Bold forward directional arrow ahead of the bicycle (+X along travel)
    b.box("white", (0.88 * s, oy, z), (0.86 * s, 0.16 * s, t))
    b.box("white", (1.28 * s, 0.19 * s + oy, z), (0.56 * s, 0.14 * s, t), rot=(0.0, 0.0, -42.0))
    b.box("white", (1.28 * s, -0.19 * s + oy, z), (0.56 * s, 0.14 * s, t), rot=(0.0, 0.0, 42.0))
    return b


def build_road_mark_slow():
    """Bold white thermoplastic 'SLOW' road stencil painted flush across the lane ahead of bends.
    Scaled 0.72x and shifted -0.93 m in Y so 0.72x placements at +1.45 m sit centered at +0.78 m
    inside the right grey pavement lane, clear of both the yellow centreline and dark shoulder."""
    b = KitBuilder("road_mark_slow")
    z, t = 0.002, 0.004
    # UE5 glTF import negates Blender Y (y_ue = -y_bl), so +0.93 m in Blender at 0.72x scale
    # shifts -67 cm inward in UE5 (+1.45 m -> +0.78 m, centered in the right grey pavement lane).
    s, oy = 0.72, 0.93

    def p(cx, cy, sx, sy):
        # Negate cy because UE5 glTF import flips Blender Y (y_ue = -y_bl), so S-L-O-W reads
        # left-to-right for the oncoming rider.
        b.box("white", (cx * s, -cy * s + oy, z), (sx * s, sy * s, t))

    # Four bold block letters arranged across Y (facing oncoming rider along -X), elongated in X
    # S (y = -0.95)
    for ox in (-0.65, 0.0, 0.65):
        p(ox, -0.95, 0.14, 0.44)
    p(0.32, -1.14, 0.68, 0.12)
    p(-0.32, -0.76, 0.68, 0.12)
    # L (y = -0.32)
    p(0.0, -0.50, 1.42, 0.12)
    p(-0.65, -0.32, 0.14, 0.44)
    # O (y = +0.32)
    for ox in (-0.65, 0.65):
        p(ox, 0.32, 0.14, 0.44)
    for ly in (0.14, 0.50):
        p(0.0, ly, 1.42, 0.12)
    # W (y = +0.98)
    for ly in (0.76, 1.20):
        p(0.0, ly, 1.42, 0.11)
    p(-0.65, 0.98, 0.14, 0.50)
    p(-0.20, 0.98, 0.85, 0.10)
    return b


def build_road_mark_stop_bar():
    """Crisp horizontal white intersection stop/yield bar + zebra crosswalk stripes."""
    b = KitBuilder("road_mark_stop_bar")
    z, t = 0.002, 0.004
    # Solid horizontal stop line across half-lane (+Y) and yield dashes across (-Y)
    b.box("white", (0.0, 0.0, z), (0.45, 5.8, t))
    for iy in (-2.4, -1.44, -0.48, 0.48, 1.44, 2.4):
        b.box("white", (1.35, iy, z), (1.50, 0.44, t))
    return b


def build_road_mark_mile():
    """Clean green-and-white roadside km/mile distance marker post for cycle paths & coast road."""
    b = KitBuilder("road_mark_mile")
    b.box("steel", (0.0, 0.0, 0.45), (0.07, 0.07, 1.45))
    b.box("green", (0.0, 0.0, 1.02), (0.10, 0.24, 0.52), bevel=0.01)
    b.box("white", (0.0, 0.0, 1.12), (0.11, 0.18, 0.14))
    b.box("white", (0.0, 0.0, 0.92), (0.11, 0.14, 0.12))
    return b


BUILDERS = {
    "harbour_house_a": build_harbour_house_a,
    "harbour_house_b": build_harbour_house_b,
    "harbour_house_c": build_harbour_house_c,
    "harbour_house_d": build_harbour_house_d,
    "harbour_warehouse": build_harbour_warehouse,
    "harbour_cargo": build_harbour_cargo,
    "harbour_market_stall": build_harbour_market_stall,
    "harbour_planter": build_harbour_planter,
    "lakefront_pavilion": build_lakefront_pavilion,
    "modern_boathouse": build_modern_boathouse,
    "chicago_townhouse": build_chicago_townhouse,
    "lakefront_overlook": build_lakefront_overlook,
    "lsd_highway_seg": build_lsd_highway_seg,
    "road_mark_bike": build_road_mark_bike,
    "road_mark_slow": build_road_mark_slow,
    "road_mark_stop_bar": build_road_mark_stop_bar,
    "road_mark_mile": build_road_mark_mile,
    "pch_guardrail": build_pch_guardrail,
    "pch_turnout_wall": build_pch_turnout_wall,
    "pch_route_sign": build_pch_route_sign,
    "harbour_crane": build_harbour_crane,
    "stone_arch": build_stone_arch,
    "boat_trawler": build_boat_trawler,
    "boat_sail": build_boat_sail,
    "boat_launch": build_boat_launch,
    "boat_dinghy": build_boat_dinghy,
    "pier_60": lambda: build_pier(60),
    "pier_90": lambda: build_pier(90),
    "lighthouse": build_lighthouse,
}


def render_preview(out_dir, builders_out):
    """Renders a clean Cycles/EEVEE overview of the kit assets for quick visual QA."""
    clear_scene()
    scene = bpy.context.scene
    scene.render.engine = "BLENDER_EEVEE" if hasattr(bpy.types, "SceneEEVEE") else "CYCLES"
    scene.render.resolution_x = 1600
    scene.render.resolution_y = 900
    if scene.render.engine == "CYCLES":
        scene.cycles.samples = 24

    # Arrange showcase assets in two neat rows
    layout = [
        ("harbour_house_a", (-18, 6, 0), 0),
        ("harbour_house_b", (-5, 6, 0), 0),
        ("harbour_house_c", (8, 6, 0), 0),
        ("harbour_house_d", (19, 6, 0), 0),
        ("lighthouse", (31, 6, 0), 0),
        ("boat_trawler", (-15, -9, 0.5), 18),
        ("boat_sail", (-1, -9, 0.5), 18),
        ("boat_launch", (11, -9, 0.5), 18),
        ("boat_dinghy", (20, -9, 0.5), 18),
    ]
    for name, loc, yaw in layout:
        ob = builders_out[name].to_blender_object()
        ob.location = loc
        ob.rotation_euler = (0, 0, math.radians(yaw))

    # Ground plane
    bpy.ops.mesh.primitive_plane_add(size=140, location=(6, 0, 0))
    ground = bpy.context.active_object
    ground.data.materials.append(make_mat("ground_prev", (0.14, 0.19, 0.22), 0.7))

    # Sun + sky world
    world = bpy.data.worlds.new("PreviewWorld")
    scene.world = world
    world.use_nodes = True
    bg = world.node_tree.nodes.get("Background")
    if bg:
        bg.inputs["Color"].default_value = (0.35, 0.48, 0.68, 1.0)
        bg.inputs["Strength"].default_value = 0.8
    sun_data = bpy.data.lights.new("Sun", type="SUN")
    sun_data.energy = 3.5
    sun_obj = bpy.data.objects.new("Sun", sun_data)
    scene.collection.objects.link(sun_obj)
    sun_obj.rotation_euler = (math.radians(52), math.radians(12), math.radians(-35))

    cam_data = bpy.data.cameras.new("Cam")
    cam_data.lens = 35.0
    cam_obj = bpy.data.objects.new("Cam", cam_data)
    scene.collection.objects.link(cam_obj)
    cam_obj.location = (5, -42, 18)
    direction = Vector((5, 0, 4)) - cam_obj.location
    cam_obj.rotation_euler = direction.to_track_quat("-Z", "Y").to_euler()
    scene.camera = cam_obj

    preview_path = os.path.join(out_dir, "harbour_kit_preview.png")
    scene.render.filepath = preview_path
    bpy.ops.render.render(write_still=True)
    print(f"Rendered preview: {preview_path}", flush=True)


def main():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--out", default="/home/jamison/.cache/opencycle/world_gen/ride_hub/harbour_kit"
    )
    ap.add_argument("--render", action="store_true")
    args = ap.parse_args(argv)
    os.makedirs(args.out, exist_ok=True)

    built = {}
    for name, fn in BUILDERS.items():
        kb = fn()
        out_path = os.path.join(args.out, f"feat_{name}.gltf")
        kb.write_gltf(out_path)
        verts = len(kb.bm.verts)
        faces = len(kb.bm.faces)
        print(
            f"harbour_kit: {name:18s} {verts:5d} verts, {faces:5d} faces -> {out_path}", flush=True
        )
        built[name] = kb

    if args.render:
        render_preview(args.out, built)
    for kb in built.values():
        kb.bm.free()


if __name__ == "__main__":
    main()
