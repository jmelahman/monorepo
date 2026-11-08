"""Writes a tileable wave normal map for the sea, textures/sea_waves_nor_dx.png (512 px, DirectX).

A sum of a few dozen sines whose wave vectors are whole numbers of cycles across the tile, so it
tiles exactly. Run with system Python: python3 -I scripts/make_wave.py
"""

import math
import random
import struct
import zlib

SIZE = 512
OUT = "/workspace/ue/textures/sea_waves_nor_dx.png"


def waves():
    rng = random.Random(7)
    result = []
    for _ in range(40):
        # Mostly from one direction (the swell), with chop from all around.
        k = rng.choice([2, 3, 3, 4, 5, 6, 8, 10, 13, 16, 20])
        angle = rng.gauss(0.4, 0.5) if rng.random() < 0.6 else rng.uniform(0, 2 * math.pi)
        kx, ky = round(k * math.cos(angle)), round(k * math.sin(angle))
        if kx == 0 and ky == 0:
            continue
        amplitude = 1.0 / (kx * kx + ky * ky) ** 0.75
        result.append((kx, ky, amplitude, rng.uniform(0, 2 * math.pi)))
    return result


def main():
    w = waves()
    rows = []
    strength = 2.0
    for j in range(SIZE):
        row = bytearray([0])
        v = j / SIZE
        for i in range(SIZE):
            u = i / SIZE
            dx = dy = 0.0
            for kx, ky, a, phase in w:
                c = math.cos(2 * math.pi * (kx * u + ky * v) + phase) * a * 2 * math.pi
                dx += c * kx
                dy += c * ky
            nx, ny, nz = -dx * strength / SIZE * 8, -dy * strength / SIZE * 8, 1.0
            length = math.sqrt(nx * nx + ny * ny + nz * nz)
            nx, ny, nz = nx / length, ny / length, nz / length
            # DirectX: green points down the texture.
            row += bytes(
                (
                    int((nx * 0.5 + 0.5) * 255),
                    int((-ny * 0.5 + 0.5) * 255),
                    int((nz * 0.5 + 0.5) * 255),
                )
            )
        rows.append(bytes(row))
    raw = zlib.compress(b"".join(rows), 9)

    def chunk(kind, data):
        return (
            struct.pack(">I", len(data))
            + kind
            + data
            + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
        )

    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", SIZE, SIZE, 8, 2, 0, 0, 0))
    png += chunk(b"IDAT", raw) + chunk(b"IEND", b"")
    with open(OUT, "wb") as f:
        f.write(png)
    print(OUT)


if __name__ == "__main__":
    main()
