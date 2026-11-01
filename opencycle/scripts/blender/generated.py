"""Brings a generated model into the game's scale.

blender -b --python generated.py -- IN OUT TALL TRIS

Stands it on the origin, centred, the given number of metres tall, reduced to about that many
triangles, with its textures at most TEXTURE pixels across.
"""

import sys

import bpy
from mathutils import Matrix, Vector

TEXTURE = 2048


def main(source, out, tall, triangles):
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.ops.import_scene.gltf(filepath=source)
    pieces = [thing for thing in bpy.context.scene.objects if thing.type == "MESH"]
    bpy.context.view_layer.objects.active = pieces[0]
    for thing in pieces:
        thing.select_set(True)
    if len(pieces) > 1:
        bpy.ops.object.join()
    thing = bpy.context.view_layer.objects.active
    bpy.ops.object.parent_clear(type="CLEAR_KEEP_TRANSFORM")
    bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
    before = sum(len(face.vertices) - 2 for face in thing.data.polygons)
    reduce = thing.modifiers.new("reduce", "DECIMATE")
    reduce.ratio = min(triangles / before, 1.0)
    bpy.ops.object.modifier_apply(modifier=reduce.name)
    corners = [Vector(corner) for corner in thing.bound_box]
    low = Vector(min(corner[axis] for corner in corners) for axis in range(3))
    high = Vector(max(corner[axis] for corner in corners) for axis in range(3))
    fit = tall / (high.z - low.z)
    middle = (low + high) * 0.5
    thing.data.transform(
        Matrix.Diagonal((fit, fit, fit, 1.0)) @ Matrix.Translation((-middle.x, -middle.y, -low.z))
    )
    thing.name = thing.data.name = "generated"
    for image in bpy.data.images:
        print("image", image.name, image.size[0], image.size[1], image.colorspace_settings.name)
        longest = max(image.size)
        if longest > TEXTURE:
            image.scale(image.size[0] * TEXTURE // longest, image.size[1] * TEXTURE // longest)
    after = sum(len(face.vertices) - 2 for face in thing.data.polygons)
    print(f"triangles {before} -> {after}, fit {fit:.3f}")
    bpy.ops.export_scene.gltf(filepath=out, export_format="GLB", export_image_format="WEBP")


if __name__ == "__main__":
    args = sys.argv[sys.argv.index("--") + 1 :]
    main(args[0], args[1], float(args[2]), int(args[3]))
