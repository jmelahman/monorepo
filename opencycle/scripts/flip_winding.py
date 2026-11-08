"""Reverses every triangle in the given glTF files written by gltf_writer (one-off fix for tiles
written before make_terrain.py's winding was corrected). Run each file through it once only."""

import base64
import json
import struct
import sys

for path in sys.argv[1:]:
    with open(path) as fh:
        doc = json.load(fh)
    prefix, data = doc["buffers"][0]["uri"].split(",", 1)
    buf = bytearray(base64.b64decode(data))
    for mesh in doc["meshes"]:
        for prim in mesh["primitives"]:
            acc = doc["accessors"][prim["indices"]]
            view = doc["bufferViews"][acc["bufferView"]]
            start = view.get("byteOffset", 0) + acc.get("byteOffset", 0)
            count = acc["count"]
            idx = list(struct.unpack_from(f"<{count}I", buf, start))
            for i in range(0, count, 3):
                idx[i + 1], idx[i + 2] = idx[i + 2], idx[i + 1]
            struct.pack_into(f"<{count}I", buf, start, *idx)
    doc["buffers"][0]["uri"] = prefix + "," + base64.b64encode(bytes(buf)).decode()
    with open(path, "w") as fh:
        json.dump(doc, fh)
    print("flipped", path)
