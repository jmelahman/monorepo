class_name CityMesh
extends RefCounted
## Collects boxes and quads into one mesh per material per stretch of street, so a long
## street is a few dozen draws and whatever is behind the rider costs nothing. Everything
## is built in world space. Sides are told apart by the outward hint each face is given,
## since one side of the road mirrors the other and winding can't be assumed.

const CHUNK := 48.0


class Bin:
	extends RefCounted
	var positions := PackedVector3Array()
	var normals := PackedVector3Array()
	var tangents := PackedFloat32Array()
	var colors := PackedColorArray()
	var uvs := PackedVector2Array()
	var uv2s := PackedVector2Array()
	var material: Material


## Which stretch of street what's added next belongs to.
var chunk := 0
var _bins: Dictionary[Vector2i, Bin] = {}
var _ids: Dictionary[Material, int] = {}


func at(s: float) -> void:
	chunk = floori(s / CHUNK)


## One flat face a-b-c-d, UV'd in metres from a so v runs a to d, facing [param outward].
func quad(
	material: Material,
	a: Vector3,
	b: Vector3,
	c: Vector3,
	d: Vector3,
	outward: Vector3,
	color := Color.WHITE,
	uv_scale := 1.0,
	uv_origin := Vector2.ZERO,
	extra := Vector2.ZERO,
) -> void:
	var u := (b - a).length() * uv_scale
	var v := (d - a).length() * uv_scale
	face(
		material,
		[a, b, c, d],
		[
			uv_origin,
			uv_origin + Vector2(u, 0.0),
			uv_origin + Vector2(u, v),
			uv_origin + Vector2(0.0, v)
		],
		outward,
		color,
		extra
	)


## A face with its corner UVs given, for atlases and anything not tiled in metres.
func face(
	material: Material,
	corners: Array[Vector3],
	mapped: Array[Vector2],
	outward: Vector3,
	color := Color.WHITE,
	extra := Vector2.ZERO,
) -> void:
	var bin := _bin(material)
	var a := corners[0]
	var b := corners[1]
	var d := corners[3]
	var normal := (b - a).cross(d - a).normalized()
	# Godot's front faces wind clockwise, so a face looking along the cross product takes
	# its corners in the order that seems backwards.
	var order: Array[int] = [0, 2, 1, 0, 3, 2]
	if normal.dot(outward) < 0.0:
		normal = -normal
		order = [0, 1, 2, 0, 2, 3]
	var tangent := (b - a).normalized()
	var flip := signf(normal.cross(tangent).dot(d - a))
	for i: int in order:
		bin.positions.append(corners[i])
		bin.normals.append(normal)
		bin.tangents.append_array([tangent.x, tangent.y, tangent.z, flip])
		bin.colors.append(color)
		bin.uvs.append(mapped[i])
		bin.uv2s.append(extra)


## A box from its centre, given as a transform whose basis carries its orientation.
func box(
	material: Material,
	place: Transform3D,
	size: Vector3,
	color := Color.WHITE,
	uv_scale := 1.0,
) -> void:
	var half := size * 0.5
	var basis := place.basis
	var ex := basis.x.normalized() * half.x
	var ey := basis.y.normalized() * half.y
	var ez := basis.z.normalized() * half.z
	var o := place.origin
	quad(
		material,
		o - ex - ey + ez,
		o + ex - ey + ez,
		o + ex + ey + ez,
		o - ex + ey + ez,
		ez,
		color,
		uv_scale
	)
	quad(
		material,
		o + ex - ey - ez,
		o - ex - ey - ez,
		o - ex + ey - ez,
		o + ex + ey - ez,
		-ez,
		color,
		uv_scale
	)
	quad(
		material,
		o + ex - ey + ez,
		o + ex - ey - ez,
		o + ex + ey - ez,
		o + ex + ey + ez,
		ex,
		color,
		uv_scale
	)
	quad(
		material,
		o - ex - ey - ez,
		o - ex - ey + ez,
		o - ex + ey + ez,
		o - ex + ey - ez,
		-ex,
		color,
		uv_scale
	)
	quad(
		material,
		o - ex + ey + ez,
		o + ex + ey + ez,
		o + ex + ey - ez,
		o - ex + ey - ez,
		ey,
		color,
		uv_scale
	)
	quad(
		material,
		o - ex - ey - ez,
		o + ex - ey - ez,
		o + ex - ey + ez,
		o - ex - ey + ez,
		-ey,
		color,
		uv_scale
	)


## A straight bar of square section between two points, for tube, rail, cable and frame.
func bar(
	material: Material, from: Vector3, to: Vector3, thick: float, color := Color.WHITE
) -> void:
	var axis := to - from
	var side := axis.cross(Vector3.UP)
	if side.length_squared() < 0.0001:
		side = axis.cross(Vector3.RIGHT)
	var across := side.normalized()
	var other := axis.cross(across).normalized()
	box(
		material,
		Transform3D(Basis(across, other, axis.normalized()), (from + to) * 0.5),
		Vector3(thick, thick, axis.length()),
		color
	)


## Turns everything added into meshes under [param root].
func commit(root: Node3D) -> void:
	for key: Vector2i in _bins:
		var bin: Bin = _bins[key]
		var arrays := []
		arrays.resize(Mesh.ARRAY_MAX)
		arrays[Mesh.ARRAY_VERTEX] = bin.positions
		arrays[Mesh.ARRAY_NORMAL] = bin.normals
		arrays[Mesh.ARRAY_TANGENT] = bin.tangents
		arrays[Mesh.ARRAY_COLOR] = bin.colors
		arrays[Mesh.ARRAY_TEX_UV] = bin.uvs
		arrays[Mesh.ARRAY_TEX_UV2] = bin.uv2s
		var mesh := ArrayMesh.new()
		mesh.add_surface_from_arrays(Mesh.PRIMITIVE_TRIANGLES, arrays)
		mesh.surface_set_material(0, bin.material)
		var instance := MeshInstance3D.new()
		instance.mesh = mesh
		instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		root.add_child(instance)


func _bin(material: Material) -> Bin:
	if material not in _ids:
		_ids[material] = _ids.size()
	var key := Vector2i(chunk, _ids[material])
	if key not in _bins:
		var bin := Bin.new()
		bin.material = material
		_bins[key] = bin
	return _bins[key]
