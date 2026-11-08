"""Writes bamboo for the coast ride: a leaf-spray texture and a few culm clumps as glTF.

    textures/bamboo_leaves_{diff,alpha,nor,arm}.png   a 512 px card of narrow leaves on twigs
    assets_gen/kit/K_bamboo_<n>.gltf                  clumps of 3-7 culms, 8-13 m, with leaf cards

Slots: Culm (vertex-coloured: green, olive at the nodes) and BambooLeaves (the card).
Run with system Python: python3 -I scripts/make_bamboo.py
"""

import math
import os
import random
import struct
import sys
import zlib

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gltf_writer
import make_houses

TEX = "/workspace/ue/textures"
OUT = "/workspace/ue/assets_gen/kit"
SIZE = 512


def png(path, width, height, rows, channels):
    kind = {1: 0, 3: 2, 4: 6}[channels]
    raw = b"".join(b"\x00" + bytes(row) for row in rows)

    def chunk(tag, data):
        return (
            struct.pack(">I", len(data))
            + tag
            + data
            + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)
        )

    with open(path, "wb") as f:
        f.write(
            b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, kind, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 9))
            + chunk(b"IEND", b"")
        )


def leaf_texture():
    """Leaves hang from twigs that fan down from the card's top edge."""
    rng = random.Random(3)
    colour = [[(0, 0, 0)] * SIZE for _ in range(SIZE)]
    alpha = [[0] * SIZE for _ in range(SIZE)]
    twigs = []
    for k in range(5):
        x0 = SIZE * (0.15 + 0.7 * k / 4) + rng.uniform(-20, 20)
        angle = math.radians(rng.uniform(60, 120))
        twigs.append((x0, 8.0, angle))
    leaves = []
    for x0, y0, angle in twigs:
        length = rng.uniform(300, 460)
        for _ in range(9):
            t = rng.uniform(0.1, 1.0)
            bx, by = x0 + math.cos(angle) * length * t, y0 + math.sin(angle) * length * t
            side = rng.choice([-1, 1])
            a = angle + side * math.radians(rng.uniform(20, 55))
            leaves.append(
                (bx, by, a, rng.uniform(110, 170), rng.uniform(11, 16), rng.uniform(0.75, 1.1))
            )
        # Draw the twig itself.
        for k in range(int(length)):
            px, py = int(x0 + math.cos(angle) * k), int(y0 + math.sin(angle) * k)
            for dx in (-1, 0, 1):
                if 0 <= px + dx < SIZE and 0 <= py < SIZE:
                    colour[py][px + dx] = (70, 80, 40)
                    alpha[py][px + dx] = 255
    for bx, by, a, length, width, shade in leaves:
        ca, sa = math.cos(a), math.sin(a)
        x_lo, x_hi = (
            int(max(0, min(bx, bx + ca * length) - width)),
            int(min(SIZE - 1, max(bx, bx + ca * length) + width)),
        )
        y_lo, y_hi = (
            int(max(0, min(by, by + sa * length) - width)),
            int(min(SIZE - 1, max(by, by + sa * length) + width)),
        )
        for py in range(y_lo, y_hi + 1):
            for px in range(x_lo, x_hi + 1):
                dx, dy = px - bx, py - by
                u = (dx * ca + dy * sa) / length
                v = -dx * sa + dy * ca
                if not 0 <= u <= 1:
                    continue
                half = width * math.sin(math.pi * u**0.7) * (1 - u) ** 0.25
                if abs(v) > half:
                    continue
                midrib = 1 - 0.25 * math.exp(-((v / 1.2) ** 2))
                light = shade * (0.85 + 0.15 * (v / max(half, 1))) * midrib * (0.9 + 0.2 * u)
                colour[py][px] = (
                    int(min(255, 68 * light)),
                    int(min(255, 105 * light)),
                    int(min(255, 32 * light)),
                )
                alpha[py][px] = 255
    png(
        f"{TEX}/bamboo_leaves_diff.png",
        SIZE,
        SIZE,
        [[c for p in row for c in p] for row in colour],
        3,
    )
    png(f"{TEX}/bamboo_leaves_alpha.png", SIZE, SIZE, alpha, 1)
    png(f"{TEX}/bamboo_leaves_nor.png", 4, 4, [[128, 128, 255] * 4 for _ in range(4)], 3)
    png(f"{TEX}/bamboo_leaves_arm.png", 4, 4, [[255, 150, 0] * 4 for _ in range(4)], 3)


def culm(mesh, rng, base, height, radius, lean, lean_dir):
    part = mesh.part("Culm")
    sides = 8
    segments = int(height / 35)
    node_every = rng.uniform(28, 40)
    prev = None
    points = []
    for k in range(segments + 1):
        t = k / segments
        z = height * t
        bend = lean * t**1.6
        cx = base[0] + math.cos(lean_dir) * bend
        cy = base[1] + math.sin(lean_dir) * bend
        r = radius * (1 - 0.6 * t)
        near_node = abs((z % node_every) - node_every / 2) > node_every / 2 - 4
        shade = (0.55 + 0.25 * t) * (0.75 if near_node else 1.0)
        col = (0.55 * shade + 0.1, 0.75 * shade + 0.1, 0.3 * shade, 1)
        ring = []
        for j in range(sides + 1):
            a = 2 * math.pi * j / sides
            n = (math.cos(a), math.sin(a), 0)
            ring.append(
                part.vertex(
                    (cx + n[0] * r, cy + n[1] * r, base[2] + z), n, (j / sides, z / 100), col
                )
            )
        if prev:
            for j in range(sides):
                part.quad(prev[j], prev[j + 1], ring[j + 1], ring[j])
        prev = ring
        points.append((cx, cy, base[2] + z))
    return points


def leaf_card(mesh, at, yaw, droop, size):
    """A two-sided card hanging from `at`: its top edge there, leaning out along yaw and
    drooping."""
    part = mesh.part("BambooLeaves")
    w, h = size
    c, s = math.cos(yaw), math.sin(yaw)
    across = (-s, c, 0)
    down = (c * math.cos(droop), s * math.cos(droop), -math.sin(droop))
    corners = []
    for u, v in ((0, 0), (1, 0), (1, 1), (0, 1)):
        a = (u - 0.5) * w
        p = tuple(at[i] + across[i] * a + down[i] * v * h for i in range(3))
        corners.append((p, (u, v)))
    normal = make_houses.norm(make_houses.cross(across, down))
    front = [part.vertex(p, normal, uv) for p, uv in corners]
    back = [part.vertex(p, tuple(-n for n in normal), uv) for p, uv in corners]
    # Front and back with opposite windings, so the card shows from both sides with a true normal.
    cr = make_houses.cross(
        make_houses.sub(corners[1][0], corners[0][0]), make_houses.sub(corners[2][0], corners[0][0])
    )
    if sum(cr[i] * normal[i] for i in range(3)) >= 0:
        part.quad(*front)
        part.quad(back[3], back[2], back[1], back[0])
    else:
        part.quad(front[3], front[2], front[1], front[0])
        part.quad(*back)


def clump(seed):
    rng = random.Random(seed * 31 + 5)
    mesh = gltf_writer.Mesh()
    for _ in range(rng.randint(3, 7)):
        base = (rng.gauss(0, 60), rng.gauss(0, 60), -20)
        height = rng.uniform(800, 1300)
        points = culm(
            mesh,
            rng,
            base,
            height,
            rng.uniform(4.5, 7.0),
            rng.uniform(80, 260),
            rng.uniform(0, 2 * math.pi),
        )
        # Branches and their leaf sprays in the top 55%.
        for k in range(len(points) // 2, len(points), 2):
            p = points[k]
            for _ in range(rng.randint(1, 2)):
                leaf_card(
                    mesh,
                    p,
                    rng.uniform(0, 2 * math.pi),
                    math.radians(rng.uniform(35, 75)),
                    (rng.uniform(90, 140), rng.uniform(100, 150)),
                )
        top = points[-1]
        for _ in range(3):
            leaf_card(
                mesh,
                top,
                rng.uniform(0, 2 * math.pi),
                math.radians(rng.uniform(50, 85)),
                (110, 130),
            )
    return mesh


def main():
    leaf_texture()
    os.makedirs(OUT, exist_ok=True)
    for k in range(4):
        clump(k).write(f"{OUT}/K_bamboo_{k}.gltf")
    print("bamboo written")


if __name__ == "__main__":
    main()
