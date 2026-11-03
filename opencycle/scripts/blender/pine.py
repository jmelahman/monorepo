"""Builds game-ready pines from Poly Haven's pine_tree_01, run inside Blender.

    blender -b --python scripts/blender/pine.py

The source trees are seven million triangles each, nearly all of them needles. This keeps the
trunk and branches as real geometry, simplified, and turns the needles into cards: it cuts the
crown into cubes, and each cube's needles become one quad laid along the plane they spread in,
painted with those same needles seen from outside the crown. So the crown keeps its real shape
and its real gaps, and a tree costs about twelve thousand triangles.

Writes assets/modelled/pine.glb: one object per tree, each with a bark surface and a needle
surface. The needle cards carry two things for game/needles.gdshader: a normal that leans out
from the trunk, so the crown lights as one rounded mass, and a vertex colour for how deep in
the crown a card sits.
"""

import math
import sys
from pathlib import Path

import bmesh
import bpy
import numpy as np  # ty: ignore[unresolved-import]  # Blender bundles it

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "assets" / "bake-sources" / "pine_tree_01"
OUT = ROOT / "assets" / "modelled" / "pine.glb"
# The cube the needles are cut into, in metres: smaller looks better close up and costs more.
CLUMP = 0.5
# Pixels along each card's side in the atlas.
CELL = 48
ATLAS = 4096
WOOD_TRIANGLES = 8000
# Each woody part's texture, and the glTF texture transform (offset, scale) it reads it with.
WOOD = {
    "trunk_a": ("trunk_a", (0.0, 0.0), (1.0, 1.0)),
    "trunk_b": ("trunk_b", (0.0, 0.0), (1.0, 1.0)),
    "trunk_c": ("bark", (0.0, -0.4), (1.4, 1.4)),
    "bark": ("bark", (0.0, 0.9), (1.2, 0.1)),
    "dead_branches": ("bark", (0.0, 0.0), (1.0, 1.0)),
}
# A card shows the needles nearest its outer face; this much of the clump's depth is darker.
SELF_SHADE = 0.45


def main():
    cache = _argument("--cache")
    trees = _load(cache)
    atlas = np.zeros((ATLAS, ATLAS, 4), np.float32)
    twig = _image(SOURCE / "textures" / "pine_tree_01_twig_diff_1k.jpg")
    bpy.ops.wm.read_factory_settings(use_empty=True)
    wood_materials = {}
    needles = _needle_material()
    cell = 0
    objects = []
    for name, load in trees.items():
        tree = load()
        names = [str(n) for n in tree["names"]]
        base = _base(tree, names)
        cards, cell = _cards(tree, names, base, twig, atlas, cell)
        wood = _wood(tree, names, base, wood_materials)
        objects.append(_join(name, wood, cards, needles))
        print(f"pine: {name} {len(cards['quads'])} cards, atlas cells used {cell}")
    _save_atlas(atlas, needles)
    _export(objects)


def _argument(flag):
    args = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    return args[args.index(flag) + 1] if flag in args else None


def _load(cache):
    """Each tree's triangles, uvs and material per triangle, from the source glTF.

    Importing the source takes minutes, so with --cache DIR the arrays are kept there."""
    if cache and list(Path(cache).glob("pine_*.npz")):
        return {
            _short(p.stem): (lambda p=p: dict(np.load(p)))
            for p in sorted(Path(cache).glob("pine_*.npz"))
        }
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.ops.import_scene.gltf(filepath=str(SOURCE / "pine_tree_01_1k.gltf"))
    trees = {}
    for ob in bpy.data.objects:
        if ob.type != "MESH":
            continue
        me = ob.data
        me.calc_loop_triangles()
        n = len(me.loop_triangles)
        co = np.empty(len(me.vertices) * 3, np.float32)
        me.vertices.foreach_get("co", co)
        world = np.array(ob.matrix_world, np.float32)
        co = co.reshape(-1, 3) @ world[:3, :3].T + world[:3, 3]
        verts = np.empty(n * 3, np.int32)
        me.loop_triangles.foreach_get("vertices", verts)
        loops = np.empty(n * 3, np.int32)
        me.loop_triangles.foreach_get("loops", loops)
        mat = np.empty(n, np.int32)
        me.loop_triangles.foreach_get("material_index", mat)
        uv = np.empty(len(me.loops) * 2, np.float32)
        me.uv_layers.active.data.foreach_get("uv", uv)
        name = _short(ob.name)
        tree = {
            "tris": co[verts.reshape(-1, 3)],
            "uvs": uv.reshape(-1, 2)[loops.reshape(-1, 3)],
            "mat": mat,
            "names": np.array([m.name for m in me.materials]),
        }
        if cache:
            Path(cache).mkdir(parents=True, exist_ok=True)
            np.savez(Path(cache) / f"pine_{ob.name}.npz", **tree)
        trees[name] = lambda tree=tree: tree
    return trees


def _short(name):
    return name.removeprefix("pine_").removeprefix("pine_tree_01_").removesuffix("_LOD0")


def _image(path):
    image = bpy.data.images.load(str(path))
    pixels = np.empty(image.size[0] * image.size[1] * 4, np.float32)
    image.pixels.foreach_get(pixels)
    return pixels.reshape(image.size[1], image.size[0], 4)


def _base(tree, names):
    """Where the trunk meets the ground: the tree's origin in the game."""
    trunk = [i for i, n in enumerate(names) if "trunk" in n]
    points = tree["tris"][np.isin(tree["mat"], trunk)].reshape(-1, 3)
    low = points[points[:, 2] < points[:, 2].min() + 1.0]
    return np.array([low[:, 0].mean(), low[:, 1].mean(), points[:, 2].min()], np.float32)


def _cards(tree, names, base, twig, atlas, cell):
    twigs = tree["mat"] == names.index("pine_tree_01_twig")
    tris = tree["tris"][twigs] - base
    uvs = tree["uvs"][twigs]
    # Corners, edge middles and centre: needle triangles are about a pixel across, so
    # these seven cover them.
    points = np.concatenate(
        [tris, (tris + np.roll(tris, 1, axis=1)) * 0.5, tris.mean(1, keepdims=True)], 1
    )
    coords = np.concatenate(
        [uvs, (uvs + np.roll(uvs, 1, axis=1)) * 0.5, uvs.mean(1, keepdims=True)], 1
    )
    centres = tris.mean(1)
    cube = np.floor(centres / CLUMP).astype(np.int64) + 1024
    keys = (cube[:, 0] << 22) | (cube[:, 1] << 11) | cube[:, 2]
    _, clump, counts = np.unique(keys, return_inverse=True, return_counts=True)
    order = np.argsort(clump, kind="stable")
    starts = np.concatenate([[0], np.cumsum(counts)])
    height = centres[:, 2].max()
    reach = _crown_reach(centres, height)
    quads, normals, shades, cells = [], [], [], []
    cols = ATLAS // CELL
    for c in range(len(counts)):
        if counts[c] < 12:
            continue
        members = order[starts[c] : starts[c + 1]]
        p = points[members].reshape(-1, 3)
        uv = coords[members].reshape(-1, 2)
        centre = p.mean(0)
        _, _, axes = np.linalg.svd(p - centre, full_matrices=False)
        across, up, face = axes
        out = np.array([centre[0], centre[1], 0.0])
        out = out / (np.linalg.norm(out) + 1e-6)
        if face @ out < 0:
            face = -face
            across = -across
        local = (p - centre) @ np.stack([across, up, face]).T
        lo = np.percentile(local[:, :2], 1, axis=0)
        hi = np.percentile(local[:, :2], 99, axis=0)
        size = np.maximum(hi - lo, 0.05)
        if cell >= cols * cols:
            raise SystemExit("pine: atlas full; raise ATLAS or CLUMP")
        col, row = cell % cols, cell // cols
        image = np.zeros((CELL, CELL, 4), np.float32)
        px = np.clip(((local[:, :2] - lo) / size * CELL).astype(int), 0, CELL - 1)
        depth = local[:, 2]
        near = depth.max()
        far = depth.min()
        shade = 1.0 - SELF_SHADE * (near - depth) / max(near - far, 1e-3)
        tx = np.clip((uv[:, 0] * twig.shape[1]).astype(int), 0, twig.shape[1] - 1)
        ty = np.clip((uv[:, 1] * twig.shape[0]).astype(int), 0, twig.shape[0] - 1)
        colour = twig[ty, tx, :3] * shade[:, None]
        # Deepest first, so the needles nearest the outer face are the ones left showing.
        draw = np.argsort(depth)
        image[px[draw, 1], px[draw, 0], :3] = colour[draw]
        image[px[draw, 1], px[draw, 0], 3] = 1.0
        covered = image[..., 3] > 0
        image[~covered, :3] = image[covered, :3].mean(0)
        atlas[row * CELL : (row + 1) * CELL, col * CELL : (col + 1) * CELL] = image
        corners = [
            centre + across * lo[0] + up * lo[1],
            centre + across * hi[0] + up * lo[1],
            centre + across * hi[0] + up * hi[1],
            centre + across * lo[0] + up * hi[1],
        ]
        quads.append(corners)
        # Half the card's own facing and half straight out from the trunk.
        lean = face * 0.5 + out + np.array([0.0, 0.0, 0.25])
        normals.append(lean / np.linalg.norm(lean))
        radial = math.hypot(centre[0], centre[1])
        inside = min(radial / max(reach(centre[2]), 0.3), 1.0)
        low = min(max((centre[2] - (height - 15.0)) / 10.0, 0.0), 1.0)
        shades.append((0.45 + 0.55 * inside) * (0.7 + 0.3 * low))
        inset = 0.5 / ATLAS
        u0, v0 = col * CELL / ATLAS + inset, row * CELL / ATLAS + inset
        u1, v1 = (col + 1) * CELL / ATLAS - inset, (row + 1) * CELL / ATLAS - inset
        cells.append([(u0, v0), (u1, v0), (u1, v1), (u0, v1)])
        cell += 1
    return {"quads": quads, "normals": normals, "shades": shades, "uvs": cells}, cell


def _crown_reach(centres, height):
    """How far the crown spreads from the trunk at a given height."""
    bands = np.linspace(0.0, height, 24)
    radial = np.hypot(centres[:, 0], centres[:, 1])
    band = np.clip(np.searchsorted(bands, centres[:, 2]) - 1, 0, len(bands) - 1)
    reach = np.zeros(len(bands))
    for b in range(len(bands)):
        r = radial[band == b]
        reach[b] = np.percentile(r, 95) if len(r) else 0.0

    def at(z):
        return float(np.interp(z, bands, reach))

    return at


def _wood(tree, names, base, materials):
    """The trunk and branches as one mesh, welded and simplified."""
    keep = [i for i, n in enumerate(names) if n.removeprefix("pine_tree_01_") in WOOD]
    picked = np.isin(tree["mat"], keep)
    tris = tree["tris"][picked] - base
    uvs = tree["uvs"][picked].copy()
    slots = tree["mat"][picked]
    for i in keep:
        _, offset, scale = WOOD[names[i].removeprefix("pine_tree_01_")]
        at = slots == i
        # Blender's v runs up and glTF's down, so the transform applies to 1 - v.
        uvs[at, :, 0] = offset[0] + scale[0] * uvs[at, :, 0]
        uvs[at, :, 1] = 1.0 - (offset[1] + scale[1] * (1.0 - uvs[at, :, 1]))
    used = sorted(set(slots.tolist()))
    me = bpy.data.meshes.new("wood")
    me.from_pydata(
        tris.reshape(-1, 3).tolist(), [], np.arange(len(tris) * 3).reshape(-1, 3).tolist()
    )
    layer = me.uv_layers.new(name="UVMap")
    layer.data.foreach_set("uv", uvs.reshape(-1).astype(np.float32))
    for i in used:
        part = names[i].removeprefix("pine_tree_01_")
        texture = WOOD[part][0]
        if texture not in materials:
            materials[texture] = _textured(texture)
        me.materials.append(materials[texture])
    me.polygons.foreach_set("material_index", np.searchsorted(used, slots).astype(np.int32))
    ob = bpy.data.objects.new("wood", me)
    bpy.context.scene.collection.objects.link(ob)
    bm = bmesh.new()
    bm.from_mesh(me)
    bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=0.0005)
    bm.to_mesh(me)
    bm.free()
    decimate = ob.modifiers.new("decimate", "DECIMATE")
    decimate.ratio = min(1.0, WOOD_TRIANGLES / len(me.polygons))
    bpy.context.view_layer.objects.active = ob
    bpy.ops.object.modifier_apply(modifier="decimate")
    return ob


def _textured(part):
    material = bpy.data.materials.new(f"pine_{part}")
    material.use_nodes = True
    nodes = material.node_tree.nodes
    shader = nodes["Principled BSDF"]
    textures = SOURCE / "textures"
    colour = nodes.new("ShaderNodeTexImage")
    colour.image = bpy.data.images.load(str(textures / f"pine_tree_01_{part}_diff_1k.jpg"))
    material.node_tree.links.new(colour.outputs["Color"], shader.inputs["Base Color"])
    normal_image = nodes.new("ShaderNodeTexImage")
    normal_image.image = bpy.data.images.load(str(textures / f"pine_tree_01_{part}_nor_gl_1k.jpg"))
    normal_image.image.colorspace_settings.name = "Non-Color"
    normal = nodes.new("ShaderNodeNormalMap")
    material.node_tree.links.new(normal_image.outputs["Color"], normal.inputs["Color"])
    material.node_tree.links.new(normal.outputs["Normal"], shader.inputs["Normal"])
    shader.inputs["Roughness"].default_value = 0.9
    return material


def _needle_material():
    material = bpy.data.materials.new("pine_needles")
    material.use_nodes = True
    nodes = material.node_tree.nodes
    shader = nodes["Principled BSDF"]
    image = nodes.new("ShaderNodeTexImage")
    image.image = bpy.data.images.new("pine_needles", ATLAS, ATLAS, alpha=True)
    material.node_tree.links.new(image.outputs["Color"], shader.inputs["Base Color"])
    material.node_tree.links.new(image.outputs["Alpha"], shader.inputs["Alpha"])
    material.surface_render_method = "DITHERED"
    return material


def _save_atlas(atlas, material):
    image = material.node_tree.nodes["Image Texture"].image
    image.pixels.foreach_set(atlas.reshape(-1))
    image.file_format = "PNG"
    image.pack()


def _join(name, wood, cards, needles):
    count = len(cards["quads"])
    me = bpy.data.meshes.new("needles")
    verts = np.array(cards["quads"], np.float32).reshape(-1, 3)
    me.from_pydata(verts.tolist(), [], np.arange(count * 4).reshape(-1, 4).tolist())
    me.materials.append(needles)
    layer = me.uv_layers.new(name="UVMap")
    layer.data.foreach_set("uv", np.array(cards["uvs"], np.float32).reshape(-1))
    colours = me.color_attributes.new("Col", "FLOAT_COLOR", "CORNER")
    shade = np.repeat(np.array(cards["shades"], np.float32), 4)
    colours.data.foreach_set(
        "color", np.stack([shade, shade, shade, np.ones_like(shade)], 1).reshape(-1)
    )
    me.normals_split_custom_set(np.repeat(np.array(cards["normals"]), 4, axis=0).tolist())
    ob = bpy.data.objects.new(name, me)
    bpy.context.scene.collection.objects.link(ob)
    bpy.ops.object.select_all(action="DESELECT")
    ob.select_set(True)
    wood.select_set(True)
    bpy.context.view_layer.objects.active = ob
    bpy.ops.object.join()
    ob.name = f"pine_{name}"
    return ob


def _export(objects):
    bpy.ops.object.select_all(action="DESELECT")
    for ob in objects:
        ob.select_set(True)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    bpy.ops.export_scene.gltf(
        filepath=str(OUT),
        use_selection=True,
        export_format="GLB",
        export_image_format="WEBP",
        export_vertex_color="ACTIVE",
        export_normals=True,
    )
    print(f"pine: wrote {OUT}")


main()
