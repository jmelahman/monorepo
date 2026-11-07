"""Writes Japanese cedar (sugi) for the coast ride: a foliage-spray texture and a few trees as glTF.

    textures/sugi_leaves_{diff,alpha,nor,arm}.png   a 512 px card: a branch fanning into clumps of
                                                    needles
    assets_gen/kit/K_sugi_<n>.gltf                  trees 16-24 m: a straight reddish trunk, bare
                                                    below,
                                                    a narrow cone of drooping branch cards above

Slots: SugiBark (vertex-coloured) and SugiLeaves (the card). Planted sugi stands in close rows on
the hills of rural Japan, so these are the hill woods' main tree and a common one in the jungle.
Run with system Python: python3 -I scripts/make_sugi.py
"""

import math
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer
import make_bamboo
import make_houses

TEX = "/workspace/ue/textures"
OUT = "/workspace/ue/assets_gen/kit"
SIZE = 512


def foliage_texture():
    """A branch runs down the card from the top centre (the trunk end); clumps of short needles
    crowd along it and its side shoots, widening toward the tip like a sugi spray."""
    rng = random.Random(7)
    colour = [[(0, 0, 0)] * SIZE for _ in range(SIZE)]
    alpha = [[0] * SIZE for _ in range(SIZE)]

    def dab(cx, cy, radius, shade):
        for py in range(int(max(0, cy - radius)), int(min(SIZE, cy + radius + 1))):
            for px in range(int(max(0, cx - radius)), int(min(SIZE, cx + radius + 1))):
                d = math.hypot(px - cx, py - cy) / radius
                if d > 1:
                    continue
                # Needles: streaks across the clump, so its edge is ragged and its face grainy.
                grain = 0.65 + 0.35 * rng.random()
                if d > 0.7 and rng.random() < (d - 0.7) * 2.2:
                    continue
                light = shade * grain * (1.05 - 0.35 * d)
                colour[py][px] = (
                    int(min(255, 46 * light)),
                    int(min(255, 74 * light)),
                    int(min(255, 40 * light)),
                )
                alpha[py][px] = 255

    # The main branch and its shoots, as (x, y) polylines.
    shoots = []
    main = [
        (SIZE / 2 + rng.uniform(-10, 10) * t, 6 + (SIZE - 30) * t)
        for t in (k / 20 for k in range(21))
    ]
    shoots.append((main, 1.0))
    for k in range(3, 20, 1):
        x0, y0 = main[k]
        side = -1 if k % 4 == 0 else 1
        reach = SIZE * (0.12 + 0.3 * k / 20)
        angle = math.radians(rng.uniform(35, 60))
        shoot = [
            (x0 + side * math.sin(angle) * reach * t, y0 + math.cos(angle) * reach * t)
            for t in (j / 8 for j in range(9))
        ]
        shoots.append((shoot, 0.8))
    for line, scale in shoots:
        for i, (x, y) in enumerate(line):
            if i < 2 and scale == 1.0:
                continue
            for _ in range(5):
                dab(
                    x + rng.uniform(-12, 12),
                    y + rng.uniform(-12, 12),
                    rng.uniform(18, 30) * scale * (0.6 + 0.6 * y / SIZE),
                    rng.uniform(0.75, 1.15),
                )
    # The bare twig through the middle near the trunk.
    for t in range(60):
        x, y = SIZE / 2, 6 + t
        for dx in (-2, -1, 0, 1, 2):
            colour[int(y)][int(x + dx)] = (60, 40, 30)
            alpha[int(y)][int(x + dx)] = 255
    make_bamboo.png(
        f"{TEX}/sugi_leaves_diff.png",
        SIZE,
        SIZE,
        [[c for p in row for c in p] for row in colour],
        3,
    )
    make_bamboo.png(f"{TEX}/sugi_leaves_alpha.png", SIZE, SIZE, alpha, 1)
    make_bamboo.png(f"{TEX}/sugi_leaves_nor.png", 4, 4, [[128, 128, 255] * 4 for _ in range(4)], 3)
    make_bamboo.png(f"{TEX}/sugi_leaves_arm.png", 4, 4, [[255, 200, 0] * 4 for _ in range(4)], 3)


def trunk(mesh, rng, height, radius):
    part = mesh.part("SugiBark")
    sides = 10
    segments = int(height / 150)
    prev = None
    wobble = [(rng.gauss(0, 3), rng.gauss(0, 3)) for _ in range(segments + 1)]
    for k in range(segments + 1):
        t = k / segments
        z = height * t - 30
        r = radius * (1 - 0.92 * t) * (1.25 if k == 0 else 1)
        cx, cy = wobble[k]
        shade = rng.uniform(0.85, 1.05)
        col = (0.45 * shade, 0.27 * shade, 0.18 * shade, 1)
        ring = []
        for j in range(sides + 1):
            a = 2 * math.pi * j / sides
            n = (math.cos(a), math.sin(a), 0)
            ring.append(
                part.vertex((cx + n[0] * r, cy + n[1] * r, z), n, (j / sides * 1.5, z / 150), col)
            )
        if prev:
            for j in range(sides):
                part.quad(prev[j], prev[j + 1], ring[j + 1], ring[j])
        prev = ring


def card(mesh, at, out, across, length, width):
    """A two-sided card with its top edge centred at `at`, reaching `length` along `out`."""
    part = mesh.part("SugiLeaves")
    corners = []
    for u, v in ((0, 0), (1, 0), (1, 1), (0, 1)):
        a = (u - 0.5) * width
        corners.append(
            (tuple(at[i] + across[i] * a + out[i] * v * length for i in range(3)), (u, v))
        )
    normal = make_houses.norm(make_houses.cross(across, out))
    if normal[2] < 0:
        normal = tuple(-n for n in normal)
    # Both faces share one normal, leaning up and out along the spray: seen from below, a card that
    # faced down would be lit only by the ground, and the pads read as black shards.
    shade = make_houses.norm(tuple(normal[i] + out[i] * 0.5 + (0, 0, 0.6)[i] for i in range(3)))
    front = [part.vertex(p, shade, uv) for p, uv in corners]
    back = [part.vertex(p, shade, uv) for p, uv in corners]
    cr = make_houses.cross(
        make_houses.sub(corners[1][0], corners[0][0]), make_houses.sub(corners[2][0], corners[0][0])
    )
    if sum(cr[i] * normal[i] for i in range(3)) >= 0:
        part.quad(*front)
        part.quad(back[3], back[2], back[1], back[0])
    else:
        part.quad(front[3], front[2], front[1], front[0])
        part.quad(*back)


def tree(seed):
    rng = random.Random(seed * 17 + 3)
    mesh = gltf_writer.Mesh()
    height = rng.uniform(1600, 2400)
    trunk(mesh, rng, height, rng.uniform(22, 32))
    crown_base = height * rng.uniform(0.38, 0.5)
    spread = rng.uniform(190, 260)
    z = crown_base
    while z < height - 60:
        t = (z - crown_base) / (height - crown_base)
        reach = spread * (1 - t) ** 0.85 + 50
        for _ in range(rng.randint(4, 6)):
            yaw = rng.uniform(0, 2 * math.pi)
            droop = math.radians(rng.uniform(12, 32) + 10 * (1 - t))
            out = (
                math.cos(yaw) * math.cos(droop),
                math.sin(yaw) * math.cos(droop),
                -math.sin(droop),
            )
            flat = (-math.sin(yaw), math.cos(yaw), 0)
            # The card lies near-flat; a second, rolled 70 degrees, gives the spray depth from the
            # side.
            roll = math.radians(70)
            up = make_houses.cross(flat, out)
            rolled = tuple(flat[i] * math.cos(roll) + up[i] * math.sin(roll) for i in range(3))
            r_trunk = 20 * (1 - z / height)
            at = (math.cos(yaw) * r_trunk, math.sin(yaw) * r_trunk, z)
            card(mesh, at, out, flat, reach, reach * 0.8)
            card(mesh, at, out, rolled, reach * 0.9, reach * 0.6)
        z += rng.uniform(42, 62)
    # The spire: crossed upright cards over the leader.
    for k in range(3):
        yaw = math.pi * k / 3
        across = (math.cos(yaw), math.sin(yaw), 0)
        card(mesh, (0, 0, height + 40), (0, 0, -1), across, 260, 90)
    return mesh


def tube(mesh, points, radii, colour, sides=8):
    """A bark tube through `points` (x, y, z), its radius at each from `radii`."""
    part = mesh.part("SugiBark")
    prev = None
    for k, (p, r) in enumerate(zip(points, radii, strict=False)):
        q = points[min(k + 1, len(points) - 1)] if k + 1 < len(points) else p
        o = points[max(k - 1, 0)]
        axis = make_houses.norm(make_houses.sub(q, o)) if q != o else (0, 0, 1)
        side = make_houses.norm(
            make_houses.cross(axis, (0, 0, 1) if abs(axis[2]) < 0.9 else (1, 0, 0))
        )
        up = make_houses.cross(side, axis)
        ring = []
        for j in range(sides + 1):
            a = 2 * math.pi * j / sides
            n = tuple(side[i] * math.cos(a) + up[i] * math.sin(a) for i in range(3))
            ring.append(
                part.vertex(
                    tuple(p[i] + n[i] * r for i in range(3)), n, (j / sides, k * 0.5), colour
                )
            )
        if prev:
            for j in range(sides):
                part.quad(prev[j], ring[j], ring[j + 1], prev[j + 1])
        prev = ring


def pad(mesh, rng, centre, radius):
    """A cloud-pruned pad: three to five overlapping lobes at slightly different heights, so it
    reads as a lumpy cloud of needles and not as a disc."""
    cx, cy, cz = centre
    lobes = rng.randint(3, 5)
    turn = rng.uniform(0, 2 * math.pi)
    lobe(mesh, rng, centre, radius * 0.7)
    for k in range(lobes - 1):
        t = turn + k * 2 * math.pi / (lobes - 1) + rng.uniform(-0.5, 0.5)
        off = radius * rng.uniform(0.3, 0.5)
        lobe(
            mesh,
            rng,
            (cx + math.cos(t) * off, cy + math.sin(t) * off, cz + rng.uniform(-12, 8)),
            radius * rng.uniform(0.45, 0.65),
        )


def lobe(mesh, rng, centre, radius):
    """One lobe of a pad: a low dark dome, so it reads as one mass, covered in brushes of stiff
    needles bursting up and out, which give it its texture close to and its ragged rim. Its outline
    is an uneven oval, not a circle."""
    part = mesh.part("MatsuNeedles")
    cx, cy, cz = centre
    thick = radius * rng.uniform(0.28, 0.4)
    squash, spin = rng.uniform(0.7, 1.0), rng.uniform(0, math.pi)
    w3, w5 = rng.uniform(0, 2 * math.pi), rng.uniform(0, 2 * math.pi)

    def edge(t):
        """The lobe's reach at angle t, as a fraction of its radius."""
        oval = 1.0 / math.hypot(math.cos(t - spin), math.sin(t - spin) / squash)
        return oval * (1 + 0.12 * math.sin(3 * t + w3) + 0.07 * math.sin(5 * t + w5))

    core = (0.07, 0.11, 0.05, 1)
    rings = [[(cx, cy, cz - thick * 0.55)] * 12]
    for k in range(5):
        el = math.radians(-40 + k * 26)
        rings.append(
            [
                (
                    cx + math.cos(t) * math.cos(el) * radius * 0.85 * edge(t),
                    cy + math.sin(t) * math.cos(el) * radius * 0.85 * edge(t),
                    cz + math.sin(el) * (thick if el > 0 else thick * 0.55),
                )
                for t in (j * 2 * math.pi / 12 for j in range(12))
            ]
        )
    rings.append([(cx, cy, cz + thick)] * 12)
    for k in range(len(rings) - 1):
        for j in range(12):
            q = [rings[k][j], rings[k][(j + 1) % 12], rings[k + 1][(j + 1) % 12], rings[k + 1][j]]
            vs = [
                part.vertex(
                    p,
                    make_houses.norm((p[0] - cx, p[1] - cy, abs(p[2] - cz) * 2 + thick)),
                    (0, 0),
                    core,
                )
                for p in q
            ]
            for tri in ((0, 1, 2), (0, 2, 3), (0, 2, 1), (0, 3, 2)):
                part.triangle(*(vs[i] for i in tri))
    # Brushes on a jittered grid over the dome, and a drooping ring of them under the rim.
    step = 8.0
    spots = []
    n = int(radius / step) + 1
    for i in range(-n, n + 1):
        for j in range(-n, n + 1):
            x, y = (i + rng.uniform(-0.4, 0.4)) * step, (j + rng.uniform(-0.4, 0.4)) * step
            d = math.hypot(x, y) / (radius * edge(math.atan2(y, x)))
            if d < 1.0:
                spots.append((x, y, math.sqrt(max(0.0, 1 - d * d)) * thick + rng.uniform(-3, 3), d))
                if rng.random() < 0.6:
                    spots.append(
                        (
                            x,
                            y,
                            -math.sqrt(max(0.0, 1 - d * d)) * thick * 0.5 - rng.uniform(0, 3),
                            -1,
                        )
                    )
    for _k in range(int(2 * math.pi * radius / step)):
        t = rng.uniform(0, 2 * math.pi)
        spots.append(
            (
                math.cos(t) * radius * 0.95 * edge(t),
                math.sin(t) * radius * 0.95 * edge(t),
                rng.uniform(-6, 4),
                1.1,
            )
        )
    for x, y, z, d in spots:
        base = (cx + x, cy + y, cz + z)
        g = rng.uniform(0.8, 1.2)
        tint = (
            (0.07 * g, 0.13 * g, 0.05 * g)
            if rng.random() > 0.15
            else (0.12 * g, 0.16 * g, 0.06 * g)
        )
        outward = math.atan2(y, x)
        for _ in range(8):
            yaw = rng.uniform(0, 2 * math.pi)
            # Up near the crown, out and down toward the rim.
            rise = math.radians(rng.uniform(20, 75) - 55 * min(d, 1.1) ** 2)
            if d < 0:
                # Under the pad: needles hang down and out.
                rise = math.radians(rng.uniform(-75, -25))
            elif d > 0.6:
                yaw = outward + rng.uniform(-1.0, 1.0)
            out = (math.cos(yaw) * math.cos(rise), math.sin(yaw) * math.cos(rise), math.sin(rise))
            side = (-math.sin(yaw), math.cos(yaw), 0.0)
            length = rng.uniform(13, 20)
            w = 0.9
            tip = tuple(base[i] + out[i] * length for i in range(3))
            shade = make_houses.norm((out[0] * 0.5, out[1] * 0.5, 1.0))
            c0 = (tint[0] * 0.6, tint[1] * 0.6, tint[2] * 0.6, 1)
            c1 = (tint[0], tint[1], tint[2], 1)
            left = part.vertex(tuple(base[i] + side[i] * w for i in range(3)), shade, (0, 0), c0)
            r = part.vertex(tuple(base[i] - side[i] * w for i in range(3)), shade, (0, 0), c0)
            t = part.vertex(tip, shade, (0, 0), c1)
            part.triangle(left, r, t)
            part.triangle(left, t, r)


def matsu(seed):
    """A pruned garden pine (niwaki): a leaning, kinked trunk and branches ending in flat pads."""
    rng = random.Random(seed * 13 + 1)
    mesh = gltf_writer.Mesh()
    bark = (0.42, 0.34, 0.3, 1)
    height = rng.uniform(320, 480)
    lean = rng.uniform(0, 2 * math.pi)
    points, x, y = [], 0.0, 0.0
    for k in range(9):
        t = k / 8
        x += math.cos(lean + math.sin(k * 1.3) * 0.9) * rng.uniform(0, 22) * (k > 0)
        y += math.sin(lean + math.sin(k * 1.3) * 0.9) * rng.uniform(0, 22) * (k > 0)
        points.append((x, y, -20 + height * t))
    tube(mesh, points, [16 * (1 - 0.7 * k / 8) for k in range(9)], bark)
    pad(mesh, rng, (points[-1][0], points[-1][1], points[-1][2] + 10), rng.uniform(70, 95))
    for _k in range(rng.randint(4, 6)):
        at = points[rng.randint(3, 7)]
        yaw = rng.uniform(0, 2 * math.pi)
        reach = rng.uniform(90, 170)
        end = (
            at[0] + math.cos(yaw) * reach,
            at[1] + math.sin(yaw) * reach,
            at[2] + rng.uniform(-10, 40),
        )
        mid = ((at[0] + end[0]) / 2, (at[1] + end[1]) / 2, max(at[2], end[2]) + 12)
        tube(mesh, [at, mid, end], [7, 5, 3], bark, 6)
        pad(mesh, rng, (end[0], end[1], end[2] + 8), rng.uniform(60, 85))
    return mesh


def main():
    foliage_texture()
    os.makedirs(OUT, exist_ok=True)
    for k in range(4):
        tree(k).write(f"{OUT}/K_sugi_{k}.gltf")
    for k in range(3):
        matsu(k).write(f"{OUT}/K_matsu_{k}.gltf")
    print("sugi and matsu written")


if __name__ == "__main__":
    main()
