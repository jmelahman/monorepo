class_name Kit
extends RefCounted
## The parts of a modular model, placed many times. Poly Haven's kits lay their parts out
## side by side for display, so each part's own position in the file is ignored unless a
## caller asks for it. Copies are drawn together per stretch of street, like [CityMesh].

var chunk := 0
var _meshes: Dictionary[String, Mesh] = {}
var _origins: Dictionary[String, Vector3] = {}
var _placed: Dictionary[Vector2i, Array] = {}
var _names: Array[String] = []


func _init(path: String) -> void:
	var scene: PackedScene = load(path)
	var node := scene.instantiate()
	for child: Node in node.find_children("*", "MeshInstance3D", true, false):
		var part := child as MeshInstance3D
		_meshes[String(part.name)] = part.mesh
		_origins[String(part.name)] = part.position
		_names.append(String(part.name))
	node.free()


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
		instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		instance.visibility_range_end = reach
		root.add_child(instance)
