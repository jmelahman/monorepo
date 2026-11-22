"""Builds a 5-building Chicago-inspired lakeside city skyline kit in Blender and writes
self-contained glTF 2.0 meshes (`feat_*.gltf`) into OC_SRC/skyline_kit.

Run headless with Blender 5.x:
    blender -b -P scripts/build_skyline_kit.py -- --out /path/to/dir [--render] [--preview PATH]
"""

import argparse
import math
import os
import sys

import bmesh  # ty: ignore[unresolved-import]
import bpy  # ty: ignore[unresolved-import]
from mathutils import Vector  # ty: ignore[unresolved-import]

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)

from build_harbour_kit import KitBuilder, clear_scene, make_mat  # noqa: E402


def _tapered_prism(b, mat, z0, z1, w0, d0, w1, d1, cx=0.0, cy=0.0):
    """Adds an axis-aligned tapered rectangular frustum from z0 to z1 to a KitBuilder."""
    hx0, hy0 = w0 * 0.5, d0 * 0.5
    hx1, hy1 = w1 * 0.5, d1 * 0.5
    pts = [
        (cx - hx0, cy - hy0, z0),
        (cx + hx0, cy - hy0, z0),
        (cx + hx0, cy + hy0, z0),
        (cx - hx0, cy + hy0, z0),
        (cx - hx1, cy - hy1, z1),
        (cx + hx1, cy - hy1, z1),
        (cx + hx1, cy + hy1, z1),
        (cx - hx1, cy + hy1, z1),
    ]
    vs = [b.bm.verts.new(p) for p in pts]
    faces = []
    for idx in (
        (0, 3, 2, 1),
        (4, 5, 6, 7),
        (0, 1, 5, 4),
        (1, 2, 6, 5),
        (2, 3, 7, 6),
        (3, 0, 4, 7),
    ):
        faces.append(b.bm.faces.new([vs[i] for i in idx]))
    bmesh.ops.recalc_face_normals(b.bm, faces=faces)
    b._tag(faces, mat, smooth=False)
    return faces


def _beam(b, mat, p0, p1, w=0.55, d=0.45):
    """Adds an oriented rectangular structural bar from p0 to p1."""
    a, c = Vector(p0), Vector(p1)
    delta = c - a
    length = delta.length
    if length < 1e-4:
        return
    mid = (a + c) * 0.5
    # Compute Euler angles aligning local +Z to delta
    quat = Vector((0.0, 0.0, 1.0)).rotation_difference(delta.normalized())
    eul = quat.to_euler("XYZ")
    b.box(
        mat,
        (mid.x, mid.y, mid.z),
        (w, d, length),
        rot=(math.degrees(eul.x), math.degrees(eul.y), math.degrees(eul.z)),
    )


def build_skyline_supertall_stepped():
    """~145 m bundled-tube stepped skyscraper (Willis/Sears Tower inspired) with buried
    foundation, 3x3 bundled dark steel & glass tubes, lit office & crown bands, and
    twin antenna spires reaching ~172 m."""
    b = KitBuilder("skyline_supertall_stepped")

    # Deep buried foundation (-10 m .. +0.5 m) + plaza podium
    b.box("concrete", (0.0, 0.0, -4.75), (34.4, 34.4, 10.5))
    b.box("concrete", (0.0, 0.0, 0.8), (33.4, 33.4, 1.6))
    b.box("steel", (0.0, 0.0, 3.8), (32.0, 32.0, 4.8))
    b.box("warm_glass", (0.0, 0.0, 3.4), (32.2, 30.2, 3.4))
    b.box("warm_glass", (0.0, 0.0, 3.4), (30.2, 32.2, 3.4))

    # 3x3 bundled tubes (each cell 10.5 m pitch, 10.25 m wide for crisp vertical reveals)
    tube_heights = {
        (-1, -1): 65.0,
        (0, -1): 118.0,
        (1, -1): 95.0,
        (-1, 0): 145.0,
        (0, 0): 145.0,
        (1, 0): 95.0,
        (-1, 1): 125.0,
        (0, 1): 125.0,
        (1, 1): 65.0,
    }
    pitch = 10.5
    tw = 10.25
    z_start = 6.0

    for (gx, gy), top_z in tube_heights.items():
        cx, cy = gx * pitch, gy * pitch
        h = top_z - z_start
        # Dark steel structural tube frame
        b.box("steel", (cx, cy, z_start + h * 0.5), (tw, tw, h))
        # Glass curtain skin slightly inset/proud on outer perimeter
        b.box("glass", (cx, cy, z_start + h * 0.5), (tw + 0.14, tw - 0.55, h - 1.2))
        b.box("glass", (cx, cy, z_start + h * 0.5), (tw - 0.55, tw + 0.14, h - 1.2))

        # Alternating dark steel mechanical/spandrel bands & glowing warm office floor bands
        z = z_start + 3.5
        band_idx = 0
        while z < top_z - 3.0:
            if band_idx % 7 == 0:
                # Dark mechanical louvre belt
                b.box("steel", (cx, cy, z), (tw + 0.24, tw + 0.24, 1.8))
            elif band_idx % 3 != 1:
                # Warm lit office band
                b.box("glass", (cx, cy, z), (tw + 0.20, tw - 0.70, 1.9))
                b.box("glass", (cx, cy, z), (tw - 0.70, tw + 0.20, 1.9))
            z += 4.2
            band_idx += 1

        # Illuminated crown cap + steel roof parapet on every tube step
        b.box("warm_glass", (cx, cy, top_z - 1.4), (tw + 0.22, tw + 0.22, 1.1))
        b.box("steel", (cx, cy, top_z - 0.25), (tw + 0.16, tw + 0.16, 0.7))

    # Penthouse mechanical hub & twin white/steel rooftop broadcast spires (to ~172 m)
    b.box("steel", (-5.25, 0.0, 146.8), (18.5, 8.6, 3.6))
    for sy in (-3.2, 3.2):
        sx = -5.25
        b.cyl("steel", (sx, sy, 148.4), (sx, sy, 154.0), 0.72, 0.55, seg=8)
        b.cyl("white", (sx, sy, 154.0), (sx, sy, 165.0), 0.52, 0.28, seg=8)
        b.cyl("white", (sx, sy, 165.0), (sx, sy, 172.0), 0.26, 0.08, seg=8)
        b.box("warm_glass", (sx, sy, 172.15), (0.36, 0.36, 0.36))
    return b


def build_skyline_tapered_braced():
    """~122 m tapered dark steel skyscraper (John Hancock Center inspired) with buried
    foundation, exterior diagonal X-bracing on all 4 facades, lit window & observation bands,
    and twin rooftop spires reaching ~148 m."""
    b = KitBuilder("skyline_tapered_braced")

    # Buried concrete foundation (-10 m .. +0.5 m) & dark steel podium
    b.box("concrete", (0.0, 0.0, -4.75), (31.6, 23.6, 10.5))
    b.box("concrete", (0.0, 0.0, 0.8), (30.8, 22.8, 1.6))
    b.box("steel", (0.0, 0.0, 3.4), (30.0, 22.0, 4.0))
    b.box("warm_glass", (0.0, 0.0, 3.1), (28.4, 22.2, 2.8))
    b.box("warm_glass", (0.0, 0.0, 3.1), (30.2, 20.4, 2.8))

    # Tapered glass curtain core (z = 5.4 .. 122.0 m)
    z_base, z_top = 5.4, 122.0
    w0, d0 = 29.6, 21.6
    w1, d1 = 19.6, 14.4
    _tapered_prism(b, "glass", z_base, z_top, w0, d0, w1, d1)

    def _half_extents(z):
        t = (z - z_base) / (z_top - z_base)
        return 0.5 * (w0 + (w1 - w0) * t), 0.5 * (d0 + (d1 - d0) * t)

    # 4 tapered corner steel mega-columns
    hx0, hy0 = _half_extents(z_base)
    hx1, hy1 = _half_extents(z_top)
    for sx in (-1.0, 1.0):
        for sy in (-1.0, 1.0):
            _beam(
                b,
                "steel",
                (sx * (hx0 + 0.12), sy * (hy0 + 0.12), z_base),
                (sx * (hx1 + 0.12), sy * (hy1 + 0.12), z_top),
                w=0.95,
                d=0.95,
            )

    # 5 stacked tiers of crisp exterior steel X-bracing + horizontal tie girders
    n_tiers = 5
    tier_zs = [z_base + (z_top - 4.0 - z_base) * (i / n_tiers) for i in range(n_tiers + 1)]
    for i in range(n_tiers):
        za, zb = tier_zs[i], tier_zs[i + 1]
        hxa, hya = _half_extents(za)
        hxb, hyb = _half_extents(zb)
        # Horizontal perimeter steel tie girder at tier boundary
        b.box("steel", (0.0, 0.0, za), (hxa * 2.0 + 0.6, hya * 2.0 + 0.6, 0.95))
        # X-bracing on +/-Y broad facades
        for sy in (-1.0, 1.0):
            ya, yb = sy * (hya + 0.28), sy * (hyb + 0.28)
            _beam(b, "steel", (-hxa, ya, za), (hxb, yb, zb), w=0.68, d=0.52)
            _beam(b, "steel", (hxa, ya, za), (-hxb, yb, zb), w=0.68, d=0.52)
        # X-bracing on +/-X side facades
        for sx in (-1.0, 1.0):
            xa, xb = sx * (hxa + 0.28), sx * (hxb + 0.28)
            _beam(b, "steel", (xa, -hya, za), (xb, hyb, zb), w=0.52, d=0.68)
            _beam(b, "steel", (xa, hya, za), (xb, -hyb, zb), w=0.52, d=0.68)

    # Scattered warm glowing office bands + bright observation deck band near top
    for i in range(18):
        zw = z_base + 4.5 + i * 5.9
        if zw > z_top - 10.0:
            break
        hx, hy = _half_extents(zw)
        if i % 2 == 0:
            b.box("glass", (0.0, 0.0, zw), (hx * 1.78, hy * 2.0 + 0.18, 1.6))
        if i % 3 != 1:
            b.box("glass", (0.0, 0.0, zw), (hx * 2.0 + 0.18, hy * 1.72, 1.6))

    # Observation deck glowing crown band (z = 112 .. 115.5 m)
    hx_obs, hy_obs = _half_extents(113.5)
    b.box("warm_glass", (0.0, 0.0, 113.6), (hx_obs * 2.0 + 0.30, hy_obs * 2.0 + 0.30, 3.0))
    b.box("steel", (0.0, 0.0, z_top - 1.2), (hx1 * 2.0 + 0.6, hy1 * 2.0 + 0.6, 2.4))

    # Twin tapered white/steel rooftop antenna spires reaching ~148 m
    for sx in (-4.8, 4.8):
        b.cyl("steel", (sx, 0.0, z_top), (sx, 0.0, 127.0), 0.65, 0.50, seg=8)
        b.cyl("white", (sx, 0.0, 127.0), (sx, 0.0, 140.0), 0.48, 0.26, seg=8)
        b.cyl("white", (sx, 0.0, 140.0), (sx, 0.0, 148.0), 0.24, 0.07, seg=8)
        b.box("warm_glass", (sx, 0.0, 148.15), (0.34, 0.34, 0.34))
    return b


def build_skyline_glass_tower():
    """~96 m sleek modern Lakefront glass curtain-wall skyscraper: buried foundation, concrete
    & steel lobby podium, stepped-back gleaming glass shafts with dark steel vertical mullions,
    lit office bands, crown terrace, and illuminated mechanical penthouse."""
    b = KitBuilder("skyline_glass_tower")
    w, d = 26.0, 22.0

    # Buried concrete foundation (-10 m .. +0.5 m) & double-height colonnade podium
    b.box("concrete", (0.0, 0.0, -4.75), (w + 1.4, d + 1.4, 10.5))
    b.box("concrete", (0.0, 0.0, 1.0), (w + 0.4, d + 0.4, 2.0))
    b.box("warm_glass", (0.0, 0.0, 4.8), (w - 1.2, d - 1.2, 6.2))
    # Perimeter steel podium columns & slab
    for i in range(7):
        px = -w * 0.48 + (w * 0.96) * (i / 6.0)
        for sy in (-1.0, 1.0):
            b.box("steel", (px, sy * (d * 0.5 - 0.2), 4.8), (0.55, 0.55, 6.6))
    for i in range(5):
        py = -d * 0.46 + (d * 0.92) * (i / 4.0)
        for sx in (-1.0, 1.0):
            b.box("steel", (sx * (w * 0.5 - 0.2), py, 4.8), (0.55, 0.55, 6.6))
    b.box("concrete", (0.0, 0.0, 8.2), (w + 0.3, d + 0.3, 1.1))

    # Lower curtain-wall shaft (z = 8.7 .. 58.0 m) & upper setback shaft (z = 58.0 .. 90.0 m)
    tiers = [
        (8.7, 58.0, 24.8, 20.8, 8, 6),
        (58.0, 90.0, 21.4, 17.4, 7, 5),
    ]
    for z0, z1, tw, td, nx, ny in tiers:
        h = z1 - z0
        zc = (z0 + z1) * 0.5
        b.box("glass", (0.0, 0.0, zc), (tw, td, h))
        # Dark steel vertical curtain-wall mullions on all 4 facades
        for i in range(nx):
            mx = -tw * 0.46 + (tw * 0.92) * (i / max(1, nx - 1))
            for sy in (-1.0, 1.0):
                b.box("steel", (mx, sy * (td * 0.5 + 0.14), zc), (0.34, 0.32, h))
        for j in range(ny):
            my = -td * 0.45 + (td * 0.90) * (j / max(1, ny - 1))
            for sx in (-1.0, 1.0):
                b.box("steel", (sx * (tw * 0.5 + 0.14), my, zc), (0.32, 0.34, h))
        # Horizontal steel spandrels & warm glowing office window strips
        z = z0 + 3.2
        idx = 0
        while z < z1 - 2.2:
            if idx % 3 == 0:
                b.box("steel", (0.0, 0.0, z), (tw + 0.32, td + 0.32, 0.72))
            else:
                b.box("glass", (0.0, 0.0, z), (tw + 0.16, td - 0.8, 1.85))
                b.box("glass", (0.0, 0.0, z), (tw - 0.8, td + 0.16, 1.85))
            z += 4.2
            idx += 1

    # Recessed crown terrace & illuminated mechanical penthouse (90 .. 96.5 m)
    b.box("concrete", (0.0, 0.0, 90.3), (21.8, 17.8, 0.8))
    b.box("steel", (0.0, 0.0, 91.1), (21.6, 17.6, 0.9))
    b.box("warm_glass", (0.0, 0.0, 93.2), (14.6, 10.8, 4.8))
    for i in range(6):
        fx = -6.2 + 12.4 * (i / 5.0)
        b.box("steel", (fx, 0.0, 93.3), (0.36, 11.2, 5.2))
    b.box("steel", (0.0, 0.0, 96.0), (15.4, 11.6, 0.9))
    return b


def build_skyline_artdeco():
    """~82 m classic Chicago limestone Art Deco tower: buried foundation, stepped plaster &
    concrete setbacks with strong vertical pilasters, recessed glass/warm_glass bays, and
    ornate stepped crown with copper spire finial."""
    b = KitBuilder("skyline_artdeco")

    # Buried concrete foundation (-10 m .. +0.5 m) & stone base podium
    b.box("concrete", (0.0, 0.0, -4.75), (25.4, 25.4, 10.5))
    b.box("concrete", (0.0, 0.0, 1.2), (24.4, 24.4, 2.4))
    b.box("plaster", (0.0, 0.0, 8.2), (23.8, 23.8, 11.8))
    # Tall arched-style lobby portals on front & back (-Y / +Y)
    for sy in (-1.0, 1.0):
        b.box("steel", (0.0, sy * 11.92, 6.4), (7.2, 0.36, 9.2))
        b.box("warm_glass", (0.0, sy * 12.04, 6.2), (6.0, 0.28, 8.4))
    b.box("concrete", (0.0, 0.0, 14.3), (24.4, 24.4, 0.85))

    # Three Art Deco setback tiers with continuous vertical limestone pilasters
    setbacks = [
        (14.7, 52.0, 22.4, 22.4, 5),
        (52.0, 68.0, 17.6, 17.6, 4),
        (68.0, 77.0, 12.6, 12.6, 3),
    ]
    for z0, z1, tw, td, n_pil in setbacks:
        h = z1 - z0
        zc = (z0 + z1) * 0.5
        # Recessed dark window core
        b.box("glass", (0.0, 0.0, zc), (tw - 0.6, td - 0.6, h))
        # Warm lit office window bands
        z = z0 + 3.2
        idx = 0
        while z < z1 - 2.4:
            if idx % 2 == 0:
                b.box("glass", (0.0, 0.0, z), (tw - 0.45, td - 2.2, 2.0))
                b.box("glass", (0.0, 0.0, z), (tw - 2.2, td - 0.45, 2.0))
            z += 4.3
            idx += 1

        # Strong projecting vertical Art Deco pilasters & corner piers
        for sx in (-1.0, 1.0):
            for sy in (-1.0, 1.0):
                b.box("plaster", (sx * (tw * 0.5 - 0.6), sy * (td * 0.5 - 0.6), zc), (1.7, 1.7, h))
        for i in range(n_pil):
            px = -tw * 0.34 + (tw * 0.68) * (i / max(1, n_pil - 1))
            for sy in (-1.0, 1.0):
                b.box("plaster", (px, sy * (td * 0.5 - 0.15), zc), (1.05, 0.95, h))
        for j in range(n_pil):
            py = -td * 0.34 + (td * 0.68) * (j / max(1, n_pil - 1))
            for sx in (-1.0, 1.0):
                b.box("plaster", (sx * (tw * 0.5 - 0.15), py, zc), (0.95, 1.05, h))
        # Setback cornice ledge
        b.box("concrete", (0.0, 0.0, z1 + 0.3), (tw + 0.7, td + 0.7, 0.7))

    # Ornate stepped Art Deco lantern crown + copper spire finial (77 .. 86 m)
    b.box("plaster", (0.0, 0.0, 78.8), (8.4, 8.4, 3.6))
    b.box("warm_glass", (0.0, 0.0, 79.0), (8.55, 5.2, 2.6))
    b.box("warm_glass", (0.0, 0.0, 79.0), (5.2, 8.55, 2.6))
    b.box("ochre", (0.0, 0.0, 81.2), (6.0, 6.0, 1.6))
    b.cyl("copper", (0.0, 0.0, 82.0), (0.0, 0.0, 84.5), 2.4, 0.65, seg=8, smooth=False)
    b.cyl("copper", (0.0, 0.0, 84.5), (0.0, 0.0, 88.0), 0.32, 0.06, seg=8)
    return b


def build_skyline_midrise():
    """~44 m Lake Shore Drive modern lakefront residential high-rise: buried foundation,
    concrete podium, alternating glass/warm_glass curtain walls, projecting horizontal concrete
    balcony slabs every floor, dark steel mullions, and rooftop penthouse."""
    b = KitBuilder("skyline_midrise")
    w, d = 28.0, 16.0

    # Buried concrete foundation (-10 m .. +0.5 m) & concrete lobby podium
    b.box("concrete", (0.0, 0.0, -4.75), (w + 1.4, d + 1.4, 10.5))
    b.box("concrete", (0.0, 0.0, 2.2), (w, d, 4.4))
    b.box("warm_glass", (0.0, 0.0, 2.2), (w - 3.0, d + 0.16, 3.2))
    for px in (-12.5, -6.2, 0.0, 6.2, 12.5):
        for sy in (-1.0, 1.0):
            b.box("steel", (px, sy * (d * 0.5 + 0.05), 2.2), (0.55, 0.45, 4.2))

    # 11 residential floors (z = 4.4 .. 40.7 m, 3.3 m floor-to-floor)
    n_floors = 11
    fl_h = 3.3
    z0 = 4.4
    core_w, core_d = w - 1.4, d - 1.4
    total_h = n_floors * fl_h
    b.box("glass", (0.0, 0.0, z0 + total_h * 0.5), (core_w, core_d, total_h))

    # Dark steel vertical mullions along broad & end facades
    for i in range(9):
        mx = -core_w * 0.46 + (core_w * 0.92) * (i / 8.0)
        for sy in (-1.0, 1.0):
            b.box(
                "steel",
                (mx, sy * (core_d * 0.5 + 0.08), z0 + total_h * 0.5),
                (0.28, 0.24, total_h),
            )
    for j in range(5):
        my = -core_d * 0.44 + (core_d * 0.88) * (j / 4.0)
        for sx in (-1.0, 1.0):
            b.box(
                "steel",
                (sx * (core_w * 0.5 + 0.08), my, z0 + total_h * 0.5),
                (0.24, 0.28, total_h),
            )

    # Projecting concrete balcony floor slabs & warm lit apartment bands per floor
    for fl in range(n_floors):
        zf = z0 + fl * fl_h
        # Horizontal projecting concrete slab
        b.box("concrete", (0.0, 0.0, zf + 0.15), (w + 0.6, d + 0.8, 0.30))
        # Subtle dark steel balcony guardrail band
        b.box("steel", (0.0, 0.0, zf + 0.95), (w + 0.48, d + 0.68, 0.10))
        # Alternating warm glowing apartment window bands
        if fl % 2 == 0:
            b.box("glass", (0.0, 0.0, zf + 1.65), (core_w + 0.14, core_d - 2.4, 2.2))
        else:
            b.box("glass", (0.0, 0.0, zf + 1.65), (core_w - 3.6, core_d + 0.14, 2.2))

    # Rooftop concrete slab, parapet & sleek mechanical penthouse (40.7 .. 44.8 m)
    roof_z = z0 + total_h
    b.box("concrete", (0.0, 0.0, roof_z + 0.2), (w + 0.6, d + 0.8, 0.45))
    b.box("concrete", (0.0, 0.0, roof_z + 0.9), (w - 0.2, d - 0.2, 1.1))
    b.box("steel", (0.0, 0.0, roof_z + 2.4), (16.2, 9.2, 3.6))
    b.box("warm_glass", (0.0, 0.0, roof_z + 2.5), (14.4, 9.32, 2.0))
    b.box("concrete", (0.0, 0.0, roof_z + 4.35), (16.8, 9.8, 0.45))
    return b


BUILDERS = {
    "skyline_supertall_stepped": build_skyline_supertall_stepped,
    "skyline_tapered_braced": build_skyline_tapered_braced,
    "skyline_glass_tower": build_skyline_glass_tower,
    "skyline_artdeco": build_skyline_artdeco,
    "skyline_midrise": build_skyline_midrise,
}


def render_preview(preview_path, built):
    """Renders a dramatic Chicago-lakefront skyline preview of all 5 towers."""
    clear_scene()
    scene = bpy.context.scene
    scene.render.engine = "CYCLES"
    scene.cycles.device = "CPU"
    scene.cycles.samples = 32
    scene.cycles.use_denoising = True
    scene.render.resolution_x = 1600
    scene.render.resolution_y = 900

    layout = [
        ("skyline_midrise", (-74.0, 8.0, 0.0), 8.0),
        ("skyline_glass_tower", (-38.0, -4.0, 0.0), -6.0),
        ("skyline_supertall_stepped", (0.0, 6.0, 0.0), 12.0),
        ("skyline_tapered_braced", (39.0, -2.0, 0.0), -10.0),
        ("skyline_artdeco", (75.0, 7.0, 0.0), 5.0),
    ]
    for name, loc, yaw in layout:
        ob = built[name].to_blender_object()
        ob.location = loc
        ob.rotation_euler = (0.0, 0.0, math.radians(yaw))

    bpy.ops.mesh.primitive_plane_add(size=600, location=(0.0, 0.0, 0.0))
    ground = bpy.context.active_object
    ground.data.materials.append(make_mat("ground_prev", (0.12, 0.16, 0.19), 0.55))

    world = bpy.data.worlds.new("SkylineWorld")
    scene.world = world
    world.use_nodes = True
    bg = world.node_tree.nodes.get("Background")
    if bg:
        bg.inputs["Color"].default_value = (0.28, 0.42, 0.65, 1.0)
        bg.inputs["Strength"].default_value = 0.70

    sun_data = bpy.data.lights.new("Sun", type="SUN")
    sun_data.energy = 3.6
    sun_obj = bpy.data.objects.new("Sun", sun_data)
    scene.collection.objects.link(sun_obj)
    sun_obj.rotation_euler = (math.radians(58.0), math.radians(16.0), math.radians(-38.0))

    cam_data = bpy.data.cameras.new("Cam")
    cam_data.lens = 28.0
    cam_data.clip_end = 1200.0
    cam_obj = bpy.data.objects.new("Cam", cam_data)
    scene.collection.objects.link(cam_obj)
    cam_obj.location = (0.0, -285.0, 76.0)
    direction = Vector((0.0, 0.0, 82.0)) - cam_obj.location
    cam_obj.rotation_euler = direction.to_track_quat("-Z", "Y").to_euler()
    scene.camera = cam_obj

    os.makedirs(os.path.dirname(os.path.abspath(preview_path)), exist_ok=True)
    scene.render.filepath = preview_path
    bpy.ops.render.render(write_still=True)
    print(f"Rendered preview: {preview_path}", flush=True)


def main():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--out", default="/home/jamison/.cache/opencycle/world_gen/ride_hub/skyline_kit"
    )
    ap.add_argument("--render", action="store_true")
    ap.add_argument("--preview", default="")
    args = ap.parse_args(argv)
    os.makedirs(args.out, exist_ok=True)

    built = {}
    for name, fn in BUILDERS.items():
        kb = fn()
        out_path = os.path.join(args.out, f"feat_{name}.gltf")
        kb.write_gltf(out_path)
        print(
            f"skyline_kit: {name:26s} {len(kb.bm.verts):5d}v {len(kb.bm.faces):5d}f -> {out_path}",
            flush=True,
        )
        built[name] = kb

    if args.render or args.preview:
        preview_path = args.preview or os.path.join(args.out, "skyline_kit_preview.png")
        render_preview(preview_path, built)
    for kb in built.values():
        kb.bm.free()


if __name__ == "__main__":
    main()
