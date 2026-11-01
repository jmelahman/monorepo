"""What the OpenCycle models are made with, run inside Blender.

A Model gathers pieces, each with a material named for what Godot swaps it for, and each with
a base colour. Godot lays the scanned textures on in world metres, so the shape and the colour
are all a model has to supply: real bevels catch the light, and the occlusion and grime are
baked into the vertex colours because nothing at runtime will find them.
"""

import itertools
import math
import random
from pathlib import Path

import bmesh
import bpy
from mathutils import Matrix, Vector, noise
from mathutils.bvhtree import BVHTree

OUT = Path(__file__).resolve().parents[2] / "assets" / "modelled"
SHARP = math.radians(50.0)
AO_REACH = 3.5
AO_RAYS = 12
AO_FLOOR = 0.16


class Model:
    def __init__(self, name):
        self.name = name
        self.bm = bmesh.new()
        self.colours = self.bm.loops.layers.float_color.new("Col")
        self.materials = []
        # Faces that are lit by a lamp in a room rather than darkened by their surroundings.
        self.lit_by = self.bm.faces.layers.int.new("lit_by")
        self.lamps = []

    def slot(self, material):
        if material not in self.materials:
            self.materials.append(material)
        return self.materials.index(material)

    def take(self, piece, material, colour, where=None, lit_by=0):
        """Moves a piece's faces into the model, frees it, and sets their material and colour."""
        slot = self.slot(material)
        rgba = _rgba(colour)
        made = {}
        for vert in piece.verts:
            made[vert] = self.bm.verts.new(where @ vert.co if where else vert.co)
        for face in piece.faces:
            new = self.bm.faces.new([made[v] for v in face.verts])
            new.material_index = slot
            new[self.lit_by] = lit_by
            for loop in new.loops:
                loop[self.colours] = rgba
        piece.free()

    def box(self, material, low, high, colour, bevel=0.0, where=None, lit_by=0):
        self.take(_cuboid(low, high, bevel), material, colour, where, lit_by)

    def bar(self, material, start, end, thick, colour, bevel=0.0):
        """A square section from start to end, its flat side kept up where it can be."""
        start, end = Vector(start), Vector(end)
        axis = end - start
        self.box(
            material,
            (-thick / 2, -thick / 2, 0.0),
            (thick / 2, thick / 2, axis.length),
            colour,
            bevel,
            _aimed(start, axis),
        )

    def tube(self, material, start, end, radius, colour, sides=8):
        start, end = Vector(start), Vector(end)
        axis = end - start
        piece = bmesh.new()
        low = []
        high = []
        for i in range(sides):
            turn = math.tau * i / sides
            at = Vector((math.cos(turn) * radius, math.sin(turn) * radius, 0.0))
            low.append(piece.verts.new(at))
            high.append(piece.verts.new(at + Vector((0.0, 0.0, axis.length))))
        for i in range(sides):
            j = (i + 1) % sides
            piece.faces.new((low[i], low[j], high[j], high[i]))
        self.take(piece, material, colour, _aimed(start, axis))

    def run(self, material, points, radius, colour, sides=6):
        """A cable or pipe through points."""
        for start, end in itertools.pairwise(points):
            self.tube(material, start, end, radius, colour, sides)

    def sheet(self, material, corner, across, down, cells, colour, ripple=None, lit_by=0):
        """A grid of quads, for what has to bend or to take a gradient of light."""
        corner, across, down = Vector(corner), Vector(across), Vector(down)
        piece = bmesh.new()
        rows = []
        for j in range(cells[1] + 1):
            row = []
            for i in range(cells[0] + 1):
                u, v = i / cells[0], j / cells[1]
                at = corner + across * u + down * v
                if ripple:
                    at += ripple(u, v)
                row.append(piece.verts.new(at))
            rows.append(row)
        for j in range(cells[1]):
            for i in range(cells[0]):
                piece.faces.new((rows[j][i], rows[j][i + 1], rows[j + 1][i + 1], rows[j + 1][i]))
        self.take(piece, material, colour, None, lit_by)

    def lamp(self, position, colour, power):
        """Adds a room's lamp, and returns what to pass as lit_by for the faces it lights."""
        self.lamps.append((Vector(position), Vector(colour), power))
        return len(self.lamps)

    def refine(self, longest, materials):
        """Splits the long edges of these materials, so a gradient has vertices to live on."""
        slots = {self.materials.index(m) for m in materials if m in self.materials}
        for _ in range(7):
            edges = [
                edge
                for edge in self.bm.edges
                if edge.calc_length() > longest
                and any(face.material_index in slots for face in edge.link_faces)
            ]
            if not edges:
                return
            bmesh.ops.triangulate(
                self.bm, faces=list({f for e in edges for f in e.link_faces if len(f.verts) > 3})
            )
            edges = [e for e in self.bm.edges if e.calc_length() > longest and _in(e, slots)]
            bmesh.ops.subdivide_edges(self.bm, edges=edges, cuts=1, use_grid_fill=False)

    def weather(self, material, big, fine):
        """Pushes a surface in and out by a few centimetres, since nothing cast is ever flat."""
        slot = self.materials.index(material)
        self.bm.normal_update()
        for vert in self.bm.verts:
            if not any(face.material_index == slot for face in vert.link_faces):
                continue
            wave = noise.noise(vert.co * 0.45) * big + noise.noise(vert.co * 2.7) * fine
            vert.co += vert.normal * wave

    def finish(self, age=0.7, grime=None):
        """Bakes the light and dirt into the colours and returns the finished object."""
        bmesh.ops.remove_doubles(self.bm, verts=self.bm.verts, dist=0.0002)
        self.bm.normal_update()
        self._bake(age, grime)
        _box_uvs(self.bm)
        mesh = bpy.data.meshes.new(self.name)
        self.bm.to_mesh(mesh)
        self.bm.free()
        for name in self.materials:
            material = bpy.data.materials.get(name) or bpy.data.materials.new(name)
            mesh.materials.append(material)
        mesh.color_attributes.active_color_name = "Col"
        mesh.attributes.default_color_name = "Col"
        thing = bpy.data.objects.new(self.name, mesh)
        bpy.context.collection.objects.link(thing)
        bpy.context.view_layer.objects.active = thing
        thing.select_set(True)
        mesh.shade_smooth()
        _harden(mesh)
        weights = thing.modifiers.new("weights", "WEIGHTED_NORMAL")
        weights.keep_sharp = True
        bpy.ops.object.modifier_apply(modifier=weights.name)
        thing.select_set(False)
        return thing

    def _bake(self, age, grime):
        tree = BVHTree.FromBMesh(self.bm)
        dice = random.Random(self.name)
        rays = [_on_sphere(dice) for _ in range(AO_RAYS)]
        for face in self.bm.faces:
            lit_by = face[self.lit_by]
            normal = face.normal
            for loop in face.loops:
                at = loop.vert.co + normal * 0.006
                colour = loop[self.colours]
                if lit_by:
                    shade = self._lamplight(tree, at, normal, self.lamps[lit_by - 1])
                    colour[0] *= shade[0]
                    colour[1] *= shade[1]
                    colour[2] *= shade[2]
                    loop[self.colours] = colour
                    continue
                open_air = _openness(tree, at, normal, rays)
                dirt = grime(at, normal) if grime else 1.0
                shade = (AO_FLOOR + (1.0 - AO_FLOOR) * open_air) * dirt
                colour[0] *= shade
                colour[1] *= shade
                colour[2] *= shade
                colour[3] = age
                loop[self.colours] = colour

    @staticmethod
    def _lamplight(tree, at, normal, lamp):
        position, colour, power = lamp
        towards = position - at
        far = towards.length
        towards.normalize()
        facing = max(normal.dot(towards), 0.0) * 0.8 + 0.2
        hit = tree.ray_cast(at, towards, far - 0.05)
        seen = 0.3 if hit[0] else 1.0
        light = power * facing * seen / (1.0 + far * far * 0.35)
        return colour * light


def _rgba(colour):
    return Vector((colour[0], colour[1], colour[2], colour[3] if len(colour) > 3 else 1.0))


def _in(edge, slots):
    return any(face.material_index in slots for face in edge.link_faces)


def _aimed(start, axis):
    turn = Vector((0.0, 0.0, 1.0)).rotation_difference(axis.normalized()).to_matrix().to_4x4()
    return Matrix.Translation(start) @ turn


def _on_sphere(dice):
    z = dice.uniform(-1.0, 1.0)
    turn = dice.uniform(0.0, math.tau)
    side = math.sqrt(1.0 - z * z)
    return Vector((math.cos(turn) * side, math.sin(turn) * side, z))


def _openness(tree, at, normal, rays):
    total = 0.0
    weight = 0.0
    for ray in rays:
        aim = ray if ray.dot(normal) > 0.0 else -ray
        lean = aim.dot(normal)
        hit = tree.ray_cast(at, aim, AO_REACH)
        total += lean * (1.0 if hit[0] is None else (hit[3] / AO_REACH) ** 0.7)
        weight += lean
    return total / weight if weight > 0.0 else 1.0


def _cuboid(low, high, bevel):
    piece = bmesh.new()
    x0, y0, z0 = low
    x1, y1, z1 = high
    corners = [
        piece.verts.new(c)
        for c in (
            (x0, y0, z0),
            (x1, y0, z0),
            (x1, y1, z0),
            (x0, y1, z0),
            (x0, y0, z1),
            (x1, y0, z1),
            (x1, y1, z1),
            (x0, y1, z1),
        )
    ]
    a, b, c, d, e, f, g, h = corners
    for quad in (
        (a, d, c, b),
        (e, f, g, h),
        (a, b, f, e),
        (b, c, g, f),
        (c, d, h, g),
        (d, a, e, h),
    ):
        piece.faces.new(quad)
    if bevel > 0.0:
        chamfer(piece, piece.edges, bevel)
    return piece


def chamfer(piece, edges, width, segments=2):
    bmesh.ops.bevel(
        piece,
        geom=list(edges),
        offset=width,
        offset_type="OFFSET",
        segments=segments,
        profile=0.5,
        affect="EDGES",
        clamp_overlap=True,
    )


def tapered(low_size, high_size, height, foot=(0.0, 0.0, 0.0), lean=(0.0, 0.0)):
    """A prism narrowing from low_size to high_size, its top shifted by lean, as a new bmesh."""
    piece = bmesh.new()
    rings = []
    for z, (wide, deep), shift in (
        (foot[2], low_size, (0.0, 0.0)),
        (foot[2] + height, high_size, lean),
    ):
        ring = []
        for sx, sy in ((-1, -1), (1, -1), (1, 1), (-1, 1)):
            ring.append(
                piece.verts.new(
                    (foot[0] + shift[0] + sx * wide / 2, foot[1] + shift[1] + sy * deep / 2, z)
                )
            )
        rings.append(ring)
    a, b = rings
    piece.faces.new(a[::-1])
    piece.faces.new(b)
    for i in range(4):
        j = (i + 1) % 4
        piece.faces.new((a[i], a[j], b[j], b[i]))
    return piece


def carve(body, cutters):
    """Cuts every cutter (bmesh, each freed here) out of a bmesh, returning the result."""
    bpy.ops.object.select_all(action="DESELECT")
    holder = _as_object(body, "carved")
    for index, cutter in enumerate(cutters):
        shape = _as_object(cutter, f"cutter{index}")
        modifier = holder.modifiers.new(f"cut{index}", "BOOLEAN")
        modifier.operation = "DIFFERENCE"
        modifier.solver = "MANIFOLD"
        modifier.object = shape
        bpy.context.view_layer.objects.active = holder
        bpy.ops.object.modifier_apply(modifier=modifier.name)
        bpy.data.objects.remove(shape)
    result = bmesh.new()
    result.from_mesh(holder.data)
    mesh = holder.data
    bpy.data.objects.remove(holder)
    bpy.data.meshes.remove(mesh)
    return result


def chamfer_creases(piece, width, segments=2):
    """Bevels every edge sharper than a few degrees, which is how a cast edge wears."""
    piece.normal_update()
    bmesh.ops.dissolve_limit(
        piece, angle_limit=math.radians(1.0), verts=piece.verts, edges=piece.edges
    )
    piece.normal_update()
    creases = [
        edge
        for edge in piece.edges
        if len(edge.link_faces) == 2 and edge.calc_face_angle(0.0) > math.radians(20.0)
    ]
    chamfer(piece, creases, width, segments)


def _as_object(piece, name):
    mesh = bpy.data.meshes.new(name)
    bmesh.ops.recalc_face_normals(piece, faces=piece.faces)
    piece.to_mesh(mesh)
    piece.free()
    thing = bpy.data.objects.new(name, mesh)
    bpy.context.collection.objects.link(thing)
    return thing


def _harden(mesh):
    """Keeps the corners of whatever wasn't bevelled hard, for the weights to respect."""
    work = bmesh.new()
    work.from_mesh(mesh)
    work.normal_update()
    for edge in work.edges:
        if len(edge.link_faces) != 2 or edge.calc_face_angle(0.0) > SHARP:
            edge.smooth = False
    work.to_mesh(mesh)
    work.free()


def _box_uvs(bm):
    """Only so that Godot can work out tangents; the scans ignore these and use world metres."""
    layer = bm.loops.layers.uv.new("UVMap")
    for face in bm.faces:
        n = face.normal
        ax, ay, az = abs(n.x), abs(n.y), abs(n.z)
        for loop in face.loops:
            co = loop.vert.co
            if az >= ax and az >= ay:
                loop[layer].uv = (co.x, co.y)
            elif ax >= ay:
                loop[layer].uv = (co.y, co.z)
            else:
                loop[layer].uv = (co.x, co.z)


def export(name):
    OUT.mkdir(parents=True, exist_ok=True)
    bpy.ops.export_scene.gltf(
        filepath=str(OUT / f"{name}.glb"),
        export_format="GLB",
        export_vertex_color="ACTIVE",
        export_active_vertex_color_when_no_material=True,
        export_normals=True,
        export_tangents=False,
        export_materials="EXPORT",
        export_yup=True,
        export_apply=False,
    )


def clear():
    bpy.ops.wm.read_factory_settings(use_empty=True)
