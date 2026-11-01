class_name Kit
extends RefCounted
## The parts of a modular model, placed many times. Poly Haven's kits lay their parts out
## side by side for display, so each part's own position in the file is ignored unless a
## caller asks for it. Copies are drawn together per stretch of street, like [CityMesh].

## How far a scan's surface may stray, in metres, seen from the road. A cardboard box comes
## with 17,000 triangles, and the rider never gets close enough to see any of them.
const STREET_TOLERANCE := 0.05
## Shadows are soft and seen second-hand, so they can stray further.
const SHADOW_SLACK := 4.0

static var _lean: Dictionary[Array, Mesh] = {}

var chunk := 0
var _meshes: Dictionary[String, Mesh] = {}
var _origins: Dictionary[String, Vector3] = {}
var _placed: Dictionary[Vector2i, Array] = {}
var _names: Array[String] = []


func _init(path: String, tolerance := STREET_TOLERANCE) -> void:
	var scene: PackedScene = load(path)
	var node := scene.instantiate()
	for child: Node in node.find_children("*", "MeshInstance3D", true, false):
		var part := child as MeshInstance3D
		_meshes[String(part.name)] = Kit.lean(part.mesh, tolerance)
		_origins[String(part.name)] = part.position
		_names.append(String(part.name))
	node.free()


## [param mesh], down to the coarsest detail its import made that stays within
## [param tolerance]. Godot picks a multimesh's detail from the whole stretch of street it
## covers, so one that passes close to the camera draws every copy at full detail.
static func lean(mesh: Mesh, tolerance: float) -> Mesh:
	var key := [mesh.get_rid().get_id(), tolerance]
	if key not in _lean:
		var made := _coarsened(mesh, tolerance)
		made.shadow_mesh = _coarsened(mesh, tolerance * SHADOW_SLACK)
		_lean[key] = made
	return _lean[key]


static func _coarsened(mesh: Mesh, tolerance: float) -> ArrayMesh:
	var made := ArrayMesh.new()
	for surface: int in mesh.get_surface_count():
		var arrays := mesh.surface_get_arrays(surface)
		var kept: Dictionary = {}
		var lods: Array = RenderingServer.mesh_get_surface(mesh.get_rid(), surface).get("lods", [])
		var positions: PackedVector3Array = arrays[Mesh.ARRAY_VERTEX]
		for lod: Dictionary in lods:
			var data: PackedByteArray = lod["index_data"]
			var indices := _indices(data, positions.size())
			var edge: float = lod["edge_length"]
			if edge <= tolerance:
				arrays[Mesh.ARRAY_INDEX] = indices
			else:
				kept[edge] = indices
		made.add_surface_from_arrays(Mesh.PRIMITIVE_TRIANGLES, arrays, [], kept)
		made.surface_set_material(surface, mesh.surface_get_material(surface))
	return made


static func _indices(data: PackedByteArray, vertices: int) -> PackedInt32Array:
	if vertices > 65535:
		return data.to_int32_array()
	var indices := PackedInt32Array()
	indices.resize(data.size() >> 1)
	for i: int in indices.size():
		indices[i] = data.decode_u16(i * 2)
	return indices


## Swaps the kit's materials, by the name each has in the file, for every part using them.
func restyle(materials: Dictionary[String, Material]) -> void:
	for part: String in _meshes:
		var mesh := _meshes[part]
		for surface: int in mesh.get_surface_count():
			var was := mesh.surface_get_material(surface).resource_name
			if was in materials:
				mesh.surface_set_material(surface, materials[was])


func parts() -> Array[String]:
	return _names


func origin(part: String) -> Vector3:
	return _origins[part]


func bounds(part: String) -> AABB:
	return _meshes[part].get_aabb()


func at(s: float) -> void:
	chunk = floori(s / CityMesh.CHUNK)


func place(part: String, where: Transform3D, color := Color.WHITE, custom := Color.BLACK) -> void:
	var key := Vector2i(_names.find(part), chunk)
	if key not in _placed:
		_placed[key] = []
	_placed[key].append([where, color, custom])


## Turns what was placed into draws under [param root], which vanish past [param reach].
func commit(root: Node3D, reach: float) -> void:
	for key: Vector2i in _placed:
		var list: Array = _placed[key]
		var multimesh := MultiMesh.new()
		multimesh.transform_format = MultiMesh.TRANSFORM_3D
		multimesh.use_colors = true
		multimesh.use_custom_data = true
		multimesh.mesh = _meshes[_names[key.x]]
		multimesh.instance_count = list.size()
		for i: int in list.size():
			var entry: Array = list[i]
			var where: Transform3D = entry[0]
			var color: Color = entry[1]
			var custom: Color = entry[2]
			multimesh.set_instance_transform(i, where)
			multimesh.set_instance_color(i, color)
			multimesh.set_instance_custom_data(i, custom)
		var instance := MultiMeshInstance3D.new()
		instance.multimesh = multimesh
		instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_DOUBLE_SIDED
		instance.visibility_range_end = reach
		root.add_child(instance)
