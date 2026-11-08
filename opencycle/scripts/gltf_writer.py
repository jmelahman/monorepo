"""Writes glTF 2.0 meshes from plain Python lists, for terrain, paths and the house kit.

Coordinates in are Unreal's: centimetres, +X forward, +Y right, +Z up. The writer converts
to glTF's metres with +Y up (glTF x = UE x, glTF y = UE z, glTF z = UE y, which flips
handedness, so triangle winding is reversed too), and Unreal's importer converts back.

A Mesh holds named parts, each with its own material name; on import every part becomes a
material slot of that name, which the level builder swaps for its own material.
"""

import base64
import json
import math
import struct


class Part:
    def __init__(self):
        self.positions = []  # (x, y, z) in UE cm
        self.normals = []
        self.uvs = []
        self.colors = []  # (r, g, b, a) 0..1
        self.indices = []

    def vertex(self, p, n=(0, 0, 1), uv=(0, 0), color=(1, 1, 1, 1)):
        self.positions.append(tuple(p))
        self.normals.append(tuple(n))
        self.uvs.append(tuple(uv))
        self.colors.append(tuple(color))
        return len(self.positions) - 1

    def triangle(self, a, b, c):
        """Counter-clockwise seen from the front, in UE's left-handed view from above."""
        self.indices.extend((a, b, c))

    def quad(self, a, b, c, d):
        self.triangle(a, b, c)
        self.triangle(a, c, d)


class Mesh:
    def __init__(self):
        self.parts = {}

    def part(self, material):
        return self.parts.setdefault(material, Part())

    def box(self, material, centre, size, yaw=0.0, uv_scale=100.0, color=(1, 1, 1, 1)):
        """An axis-aligned box (then yawed about Z) with planar UVs in world units / uv_scale."""
        cx, cy, cz = centre
        hx, hy, hz = (s / 2 for s in size)
        c, s = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))

        def at(x, y, z):
            return (cx + x * c - y * s, cy + x * s + y * c, cz + z)

        def turn(n):
            return (n[0] * c - n[1] * s, n[0] * s + n[1] * c, n[2])

        faces = (  # normal, the four corners counter-clockwise from outside, uv axes
            ((1, 0, 0), [(hx, -hy, -hz), (hx, hy, -hz), (hx, hy, hz), (hx, -hy, hz)], (1, 2)),
            ((-1, 0, 0), [(-hx, hy, -hz), (-hx, -hy, -hz), (-hx, -hy, hz), (-hx, hy, hz)], (1, 2)),
            ((0, 1, 0), [(hx, hy, -hz), (-hx, hy, -hz), (-hx, hy, hz), (hx, hy, hz)], (0, 2)),
            ((0, -1, 0), [(-hx, -hy, -hz), (hx, -hy, -hz), (hx, -hy, hz), (-hx, -hy, hz)], (0, 2)),
            ((0, 0, 1), [(-hx, -hy, hz), (hx, -hy, hz), (hx, hy, hz), (-hx, hy, hz)], (0, 1)),
            ((0, 0, -1), [(-hx, hy, -hz), (hx, hy, -hz), (hx, -hy, -hz), (-hx, -hy, -hz)], (0, 1)),
        )
        part = self.part(material)
        for normal, corners, (u, v) in faces:
            ids = []
            for corner in corners:
                world = at(*corner)
                local = (corner[0] + cx, corner[1] + cy, corner[2] + cz)
                ids.append(
                    part.vertex(
                        world, turn(normal), (local[u] / uv_scale, -local[v] / uv_scale), color
                    )
                )
            part.quad(*ids)

    def write(self, path):
        """Writes a self-contained .gltf with the buffer embedded as base64."""
        blob = bytearray()
        views, accessors, primitives, materials = [], [], [], []

        def add(data, fmt, count, kind, component, target, minmax=None):
            while len(blob) % 4:
                blob.append(0)
            offset = len(blob)
            blob.extend(struct.pack(f"<{len(data)}{fmt}", *data))
            views.append(
                {
                    "buffer": 0,
                    "byteOffset": offset,
                    "byteLength": len(blob) - offset,
                    "target": target,
                }
            )
            accessor = {
                "bufferView": len(views) - 1,
                "componentType": component,
                "count": count,
                "type": kind,
            }
            if minmax:
                accessor["min"], accessor["max"] = minmax
            accessors.append(accessor)
            return len(accessors) - 1

        for name, part in self.parts.items():
            if not part.indices:
                continue
            pos = [(x / 100, z / 100, y / 100) for x, y, z in part.positions]
            nor = []
            for x, y, z in part.normals:
                length = math.sqrt(x * x + y * y + z * z) or 1.0
                nor.append((x / length, z / length, y / length))
            flat_pos = [v for p in pos for v in p]
            lo = [min(p[i] for p in pos) for i in range(3)]
            hi = [max(p[i] for p in pos) for i in range(3)]
            attributes = {
                "POSITION": add(flat_pos, "f", len(pos), "VEC3", 5126, 34962, (lo, hi)),
                "NORMAL": add([v for n in nor for v in n], "f", len(nor), "VEC3", 5126, 34962),
                "TEXCOORD_0": add(
                    [v for uv in part.uvs for v in uv], "f", len(part.uvs), "VEC2", 5126, 34962
                ),
                "COLOR_0": add(
                    [v for c in part.colors for v in c], "f", len(part.colors), "VEC4", 5126, 34962
                ),
            }
            flipped = []
            for i in range(0, len(part.indices), 3):
                a, b, c = part.indices[i : i + 3]
                flipped.extend((a, c, b))
            indices = add(flipped, "I", len(flipped), "SCALAR", 5125, 34963)
            materials.append(
                {
                    "name": name,
                    "pbrMetallicRoughness": {
                        "baseColorFactor": [0.5, 0.5, 0.5, 1],
                        "metallicFactor": 0,
                        "roughnessFactor": 0.8,
                    },
                }
            )
            primitives.append(
                {"attributes": attributes, "indices": indices, "material": len(materials) - 1}
            )

        doc = {
            "asset": {"version": "2.0", "generator": "opencycle gltf_writer"},
            "scene": 0,
            "scenes": [{"nodes": [0]}],
            "nodes": [{"mesh": 0, "name": "mesh"}],
            "meshes": [{"primitives": primitives}],
            "materials": materials,
            "buffers": [
                {
                    "byteLength": len(blob),
                    "uri": "data:application/octet-stream;base64,"
                    + base64.b64encode(bytes(blob)).decode(),
                }
            ],
            "bufferViews": views,
            "accessors": accessors,
        }
        with open(path, "w") as f:
            json.dump(doc, f)
