"""A test world for the landscape pipeline: 2 km of rolling grassland with a road through it.

Writes to /workspace/ue/world_gen/test:
- height.r16: 16-bit heights, 2017 x 2017 at 1 m, rows along X;
- grass.r8 and dirt.r8: the landscape's paint layers;
- track.json: the road's centreline every metre, in cm;
- turf_N.gltf: short grass patches for the landscape's grass type.

trial_world.py builds the level from these. Run with system Python: python3 -I make_trial_world.py
"""

import itertools
import json
import math
import os
import random
import struct
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer
from make_houses import norm

OUT = "/workspace/ue/world_gen/test"
QUADS = 63  # per component
COMPONENTS = 32
SIZE = COMPONENTS * QUADS + 1  # vertices a side, a metre apart
SPACING = 100.0  # cm
Z_SCALE = 400.0  # the landscape's Z scale: Z_SCALE / 128 cm a height step
ROAD_HALF = 150.0  # cm
SHOULDER = 500.0  # cm of levelled ground beside the road


def noise(x, y, seed):
    """Smooth value noise in [-1, 1] on unit cells."""

    def h(ix, iy):
        n = (ix * 374761393 + iy * 668265263 + seed * 2147483647) & 0xFFFFFFFF
        n = ((n ^ (n >> 13)) * 1274126177) & 0xFFFFFFFF
        return ((n ^ (n >> 16)) & 0xFFFF) / 32767.5 - 1.0

    ix, iy = math.floor(x), math.floor(y)
    fx, fy = x - ix, y - iy
    sx, sy = fx * fx * (3 - 2 * fx), fy * fy * (3 - 2 * fy)
    a = h(ix, iy) + (h(ix + 1, iy) - h(ix, iy)) * sx
    b = h(ix, iy + 1) + (h(ix + 1, iy + 1) - h(ix, iy + 1)) * sx
    return a + (b - a) * sy


def hills(x, y):
    """Ground height in cm at (x, y) cm, before the road is cut in."""
    m = 0.01
    z = 0.0
    for octave, (scale, amp) in enumerate(((600, 2500), (220, 900), (80, 250), (25, 60))):
        z += amp * noise(x * m / scale, y * m / scale, 11 + octave)
    return z


def road():
    """The road's centreline every metre: a wandering line across the square, in cm."""
    pts = []
    half = (SIZE - 1) * SPACING / 2
    for k in range(1800):
        t = k / 1799
        x = -half * 0.85 + 2 * half * 0.85 * t
        y = 22000 * math.sin(t * 5.1) + 9000 * math.sin(t * 13.0 + 1.0)
        pts.append((x, y))
    # Respace to a metre.
    out = [pts[0]]
    carry = 0.0
    for a, b in itertools.pairwise(pts):
        seg = math.dist(a, b)
        d = 100.0 - carry
        while d <= seg:
            t = d / seg
            out.append((a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t))
            d += 100.0
        carry = seg - (d - 100.0)
    # The road's own height: the hills, smoothed along its length, so it climbs and falls gently.
    raw = [hills(x, y) for x, y in out]
    span = 60
    smooth = []
    for i in range(len(raw)):
        window = raw[max(0, i - span) : i + span + 1]
        smooth.append(sum(window) / len(window))
    return [(x, y, z) for (x, y), z in zip(out, smooth, strict=True)]


def main():
    os.makedirs(OUT, exist_ok=True)
    centre = road()
    half = (SIZE - 1) * SPACING / 2
    # Per vertex: distance to the road and its height there, from a coarse grid of the centreline.
    cell = 2000.0
    buckets = {}
    for i, (x, y, _z) in enumerate(centre):
        buckets.setdefault((int(x // cell), int(y // cell)), []).append(i)
    heights = bytearray()
    grass = bytearray()
    dirt = bytearray()
    for j in range(SIZE):
        y = -half + j * SPACING
        for i in range(SIZE):
            x = -half + i * SPACING
            z = hills(x, y)
            best, bz = 1e18, 0.0
            cx, cy = int(x // cell), int(y // cell)
            for dx in (-1, 0, 1):
                for dy in (-1, 0, 1):
                    for k in buckets.get((cx + dx, cy + dy), ()):
                        px, py, pz = centre[k]
                        d = (px - x) ** 2 + (py - y) ** 2
                        if d < best:
                            best, bz = d, pz
            d = math.sqrt(best)
            # Cut and fill to the road: level across it and its shoulder, easing out over 15 m.
            blend = (
                1.0 if d < ROAD_HALF + SHOULDER else max(0.0, 1 - (d - ROAD_HALF - SHOULDER) / 1500)
            )
            blend = blend * blend * (3 - 2 * blend)
            z = z + (bz - 8 - z) * blend
            heights += struct.pack("<H", max(0, min(65535, round(32768 + z * 128 / Z_SCALE))))
            # Dirt under the road and a worn edge, patches of dirt from noise, grass elsewhere.
            edge = 1.0 if d < ROAD_HALF + 40 else max(0.0, 1 - (d - ROAD_HALF - 40) / 120)
            patch = max(0.0, noise(x / 900, y / 900, 3) * 1.6 - 0.9)
            w = min(1.0, edge + patch)
            dirt.append(round(255 * w))
            grass.append(255 - round(255 * w))
    with open(f"{OUT}/height.r16", "wb") as f:
        f.write(heights)
    with open(f"{OUT}/grass.r8", "wb") as f:
        f.write(grass)
    with open(f"{OUT}/dirt.r8", "wb") as f:
        f.write(dirt)
    with open(f"{OUT}/track.json", "w") as f:
        json.dump({"centre": centre, "size": SIZE, "spacing": SPACING, "z_scale": Z_SCALE}, f)
    for k in range(4):
        turf(k).write(f"{OUT}/turf_{k}.gltf")
    print(f"world test: {SIZE}x{SIZE}, road {len(centre)} m")


def turf(seed):
    """A patch of short grass about 60 cm across: 180 thin blades 4-16 cm high (720 triangles),
    so laid edge to edge by the landscape's grass type it covers the ground as a sward, not as
    clumps."""
    rng = random.Random(seed * 7 + 3)
    mesh = gltf_writer.Mesh()
    part = mesh.part("Turf")
    for _ in range(180):
        r = 30 * math.sqrt(rng.random())
        a = rng.uniform(0, 2 * math.pi)
        base = (math.cos(a) * r, math.sin(a) * r, -1.0)
        yaw = rng.uniform(0, 2 * math.pi)
        lean = math.radians(rng.uniform(5, 35))
        height = rng.uniform(4, 16) * (1.4 if rng.random() < 0.08 else 1.0)
        width = rng.uniform(0.25, 0.45)
        out = (math.cos(yaw), math.sin(yaw), 0.0)
        side = (-math.sin(yaw), math.cos(yaw), 0.0)
        g = rng.uniform(0.7, 1.05)
        dry = rng.random() < 0.08
        tint = (
            (0.40 * g, 0.36 * g, 0.20 * g)
            if dry
            else ((0.10 + rng.uniform(-0.02, 0.03)) * g, 0.20 * g, 0.05 * g)
        )
        normal = norm((out[0] * 0.3, out[1] * 0.3, 1.0))
        rows = []
        n = 2
        for k in range(n + 1):
            t = k / n
            bend = lean * (0.3 + 0.9 * t)
            d = height * t
            c = (
                base[0] + out[0] * math.sin(bend) * d,
                base[1] + out[1] * math.sin(bend) * d,
                base[2] + math.cos(bend) * d,
            )
            w = width * (1 - t) + 0.03
            shade = 0.5 + 0.5 * t
            col = (tint[0] * shade, tint[1] * shade, tint[2] * shade, 1)
            rows.append(
                [
                    part.vertex(
                        tuple(c[i] + side[i] * w * s for i in range(3)), normal, (0, t), col
                    )
                    for s in (-1, 1)
                ]
            )
        for k in range(n):
            a0, a1 = rows[k]
            b0, b1 = rows[k + 1]
            # One face: M_Turf is two-sided, which halves the triangles across a field of these.
            part.triangle(a0, a1, b1)
            part.triangle(a0, b1, b0)
    return mesh


if __name__ == "__main__":
    main()
