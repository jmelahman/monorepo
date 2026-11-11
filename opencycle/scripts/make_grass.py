"""Writes the climb's own grass: clumps of real blades as glTF meshes in world_gen/grass.

The plant scans are a few flat cards with a photo of sparse weeds, 12 to 40 cm tall, and read
thin from every side (v12-v31). These clumps are geometry: each blade a tapered, bent strip, a
hundred or so to a clump, so a field of them has volume and no cut-out texture to thin with
distance. `oc_climb.py` imports them as the "climb" biome's GRASS rows and shades them with
M_Blade, which reads each vertex's UV: u is the blade's own random number (1.0 marks a seed
head), v its height along the blade from 0 at the root to 1 at the tip.

Run: python3 scripts/make_grass.py [out_dir]
"""

import itertools
import math
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gltf_writer import Mesh

OUT = "/workspace/ue/world_gen/grass"
SEED_HEAD = 1.0
# slug: (variants, blades, clump radius, (height low, high), (width low, high), (lean low, high)
# as a share of height, segments, seed stalks). Lengths in cm.
KINDS = {
    # Turf: the carpet's underlayer, and the alpine heath's grass.
    "blade_short": (3, 110, 26.0, (10.0, 24.0), (0.7, 1.2), (0.15, 0.7), 3, 0),
    # Meadow grass, shin high.
    "blade_mid": (3, 100, 28.0, (24.0, 46.0), (0.8, 1.4), (0.15, 0.6), 4, 0),
    # Knee high, with seed stalks standing above it.
    "blade_tall": (3, 80, 30.0, (42.0, 75.0), (0.9, 1.6), (0.12, 0.5), 5, 5),
}


def _cross(a, b):
    return (a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0])


def _strip(part, base, yaw, height, width, lean, segments, u, taper=2.2):
    """One blade: a strip from its root, bending over toward `yaw` as it rises, narrowing to a
    point. Its normals face along the bend; the material is two-sided."""
    ahead = (math.cos(yaw), math.sin(yaw), 0.0)
    side = (-ahead[1], ahead[0], 0.0)
    rows = []
    for i in range(segments):
        t = i / segments
        out = lean * height * t * t
        centre = (base[0] + ahead[0] * out, base[1] + ahead[1] * out, height * t)
        tangent = (ahead[0] * 2 * lean * t, ahead[1] * 2 * lean * t, 1.0)
        normal = _cross(side, tangent)
        half = width / 2 * (1 - t**taper)
        rows.append(
            tuple(
                part.vertex(
                    (centre[0] + side[0] * s * half, centre[1] + side[1] * s * half, centre[2]),
                    normal,
                    (u, t),
                    (u, t, 0.0, 1.0),
                )
                for s in (-1.0, 1.0)
            )
        )
    for (a, b), (c, d) in itertools.pairwise(rows):
        part.quad(a, b, d, c)
    out = lean * height
    tip = part.vertex(
        (base[0] + ahead[0] * out, base[1] + ahead[1] * out, height),
        _cross(side, (ahead[0] * 2 * lean, ahead[1] * 2 * lean, 1.0)),
        (u, 1.0),
        (u, 1.0, 0.0, 1.0),
    )
    part.triangle(rows[-1][0], rows[-1][1], tip)


def _seed_stalk(part, rng, base, height):
    """A thin stem above the blades with a spindle-shaped head, marked by u = SEED_HEAD."""
    yaw = rng.uniform(0, math.tau)
    lean = rng.uniform(0.05, 0.25)
    _strip(part, base, yaw, height, 0.35, lean, 4, rng.uniform(0.0, 0.9), taper=8.0)
    top = (
        base[0] + math.cos(yaw) * lean * height,
        base[1] + math.sin(yaw) * lean * height,
        height,
    )
    length, width = rng.uniform(5.0, 9.0), rng.uniform(1.2, 2.0)
    for turn in (0.0, math.pi / 2):
        side = (math.cos(yaw + turn), math.sin(yaw + turn), 0.0)
        normal = (-side[1], side[0], 0.0)
        ids = [
            part.vertex(p, normal, (SEED_HEAD, v), (SEED_HEAD, v, 0.0, 1.0))
            for p, v in (
                ((top[0], top[1], top[2] - length * 0.45), 0.7),
                ((top[0] + side[0] * width / 2, top[1] + side[1] * width / 2, top[2]), 0.85),
                ((top[0], top[1], top[2] + length * 0.55), 1.0),
                ((top[0] - side[0] * width / 2, top[1] - side[1] * width / 2, top[2]), 0.85),
            )
        ]
        part.quad(*ids)


def clump(rng, blades, radius, heights, widths, leans, segments, stalks):
    mesh = Mesh()
    part = mesh.part("blade")
    # Blades come in tufts of similar height, as grass grows, rather than one even fuzz.
    tufts = [
        (rng.uniform(0, math.tau), radius * math.sqrt(rng.random()), rng.random())
        for _ in range(max(4, blades // 12))
    ]
    for _ in range(blades):
        angle, reach, vigour = rng.choice(tufts)
        spread = rng.gauss(0, radius * 0.22)
        turn = rng.uniform(0, math.tau)
        base = (
            math.cos(angle) * reach + math.cos(turn) * spread,
            math.sin(angle) * reach + math.sin(turn) * spread,
        )
        share = min(1.0, max(0.0, 0.6 * vigour + 0.4 * rng.random()))
        height = heights[0] + (heights[1] - heights[0]) * share
        _strip(
            part,
            base,
            rng.uniform(0, math.tau),
            height,
            rng.uniform(*widths),
            rng.uniform(*leans),
            segments,
            rng.uniform(0.0, 0.9),
        )
    for _ in range(stalks):
        angle, reach = rng.uniform(0, math.tau), radius * 0.7 * math.sqrt(rng.random())
        base = (math.cos(angle) * reach, math.sin(angle) * reach)
        _seed_stalk(part, rng, base, heights[1] * rng.uniform(1.0, 1.25))
    return mesh


def main(out=OUT):
    os.makedirs(out, exist_ok=True)
    for slug, (variants, *shape) in KINDS.items():
        for i in range(variants):
            rng = random.Random(f"{slug}{i}")
            mesh = clump(rng, *shape)
            path = f"{out}/{slug}_{'ABCDEFGH'[i]}.gltf"
            mesh.write(path)
            part = mesh.parts["blade"]
            top = max(p[2] for p in part.positions)
            print(f"{path}: {len(part.indices) // 3} tris, {top:.0f} cm tall")


if __name__ == "__main__":
    main(*sys.argv[1:2])
