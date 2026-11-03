class_name Flora
extends RefCounted
## Draws what [Planting] decided: everything as [MultiMesh]es, chunked so whatever
## is behind the rider or out of range costs nothing.
##
## Trees come in two forms from tools/bake_foliage.gd: a fan of slices up close and
## two crossed views far away. Both are a handful of quads, so the cost of a tree is
## the pixels it covers, not its triangles. A tree in [constant MODELLED] is instead
## modelled up close, a real trunk and branches with cards of needles, from
## scripts/blender/pine.py.

const CHUNK := 64.0
const SMALL_CHUNK := 36.0
const FAR_CHUNK := 128.0
const BAKED := "res://assets/baked/%s"
const FOLIAGE: Shader = preload("res://game/foliage.gdshader")
const TREES: Dictionary[Planting.Kind, String] = {
	Planting.Kind.FIR: "pine_tree_01",
	Planting.Kind.PINE: "pine_tree_01",
}
const MODELLED: Dictionary[Planting.Kind, String] = {
	Planting.Kind.FIR: "res://assets/modelled/pine.glb",
	Planting.Kind.PINE: "res://assets/modelled/pine.glb",
}
const NEEDLES: Shader = preload("res://game/needles.gdshader")
const NEAR_TREE_RANGE := 90.0
const FAR_TREE_RANGE := 2400.0
const SCENES: Dictionary[Planting.Kind, String] = {
	Planting.Kind.FERN: "res://assets/polyhaven/fern_02/fern_02_1k.gltf",
	Planting.Kind.GRASS: "res://assets/polyhaven/grass_medium_01/grass_medium_01_1k.gltf",
	Planting.Kind.ROCK: "res://assets/polyhaven/rock_moss_set_01/rock_moss_set_01_1k.gltf",
	Planting.Kind.BOULDER: "res://assets/polyhaven/boulder_01/boulder_01_1k.gltf",
	Planting.Kind.STUMP: "res://assets/polyhaven/tree_stump_01/tree_stump_01_1k.gltf",
}
const RANGES: Dictionary[Planting.Kind, float] = {
	Planting.Kind.FERN: 38.0,
	Planting.Kind.GRASS: 22.0,
	Planting.Kind.ROCK: 60.0,
	Planting.Kind.BOULDER: 110.0,
	Planting.Kind.STUMP: 45.0,
}


static func build(plants: Dictionary[Planting.Kind, Array], route: Route) -> Node3D:
	var root := Node3D.new()
	root.name = "Flora"
	for kind: Planting.Kind in TREES:
		_add_trees(root, kind, plants[kind], route)
	for kind: Planting.Kind in SCENES:
		_add_small(root, kind, plants[kind])
	return root


static func _add_trees(root: Node3D, kind: Planting.Kind, transforms: Array, route: Route) -> void:
	var tree := TREES[kind]
	var parsed: Variant = JSON.parse_string(FileAccess.get_file_as_string(BAKED % (tree + ".json")))
	var meta: Dictionary = parsed
	var slices: int = meta["slices"]
	var half_width: float = meta["width"]
	half_width *= 0.5
	var nears: Array[Mesh] = []
	if kind in MODELLED:
		nears = _modelled(MODELLED[kind])
	else:
		nears = [_fan(meta, slices, slices, 2, _card(BAKED % (tree + "_slices.webp"), half_width))]
	var far := _fan(meta, 2, 4, 1, _card(BAKED % (tree + "_impostor.webp"), half_width))
	# The rider stays on the road, so a tree far from it only ever needs its far form.
	var close_by: Array = []
	var distant_only: Array = []
	for entry: Variant in transforms:
		var placed: Transform3D = entry
		var off_road := absf(route.nearest(placed.origin.x, placed.origin.z).y)
		(close_by if off_road < NEAR_TREE_RANGE else distant_only).append(placed)
	for m: int in nears.size():
		var variant: Array = []
		for i: int in range(m, close_by.size(), nears.size()):
			variant.append(close_by[i])
		for chunk: Array in _chunks(variant, CHUNK):
			var close := _instance(nears[m], chunk)
			close.visibility_range_end = NEAR_TREE_RANGE
			root.add_child(close)
	for chunk: Array in _chunks(close_by, CHUNK):
		var distant := _instance(far, chunk)
		distant.visibility_range_begin = NEAR_TREE_RANGE
		distant.visibility_range_end = FAR_TREE_RANGE
		distant.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		root.add_child(distant)
	for chunk: Array in _chunks(distant_only, FAR_CHUNK):
		var distant := _instance(far, chunk)
		distant.visibility_range_end = FAR_TREE_RANGE
		distant.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		root.add_child(distant)


static func _add_small(root: Node3D, kind: Planting.Kind, transforms: Array) -> void:
	var meshes := _meshes(SCENES[kind], kind == Planting.Kind.GRASS or kind == Planting.Kind.FERN)
	var buckets: Array[Array] = []
	for i: int in meshes.size():
		buckets.append([])
	for i: int in transforms.size():
		buckets[i % meshes.size()].append(transforms[i])
	var soft := kind == Planting.Kind.GRASS or kind == Planting.Kind.FERN
	for m: int in meshes.size():
		for chunk: Array in _chunks(buckets[m], SMALL_CHUNK):
			var instance := _instance(meshes[m], chunk)
			instance.visibility_range_end = RANGES[kind]
			if soft:
				instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
			root.add_child(instance)


static func _chunks(transforms: Array, size: float) -> Array[Array]:
	var cells: Dictionary[Vector2i, Array] = {}
	for entry: Variant in transforms:
		var placed: Transform3D = entry
		var cell := Vector2i(floori(placed.origin.x / size), floori(placed.origin.z / size))
		if cell not in cells:
			cells[cell] = []
		cells[cell].append(placed)
	var chunks: Array[Array] = []
	chunks.assign(cells.values())
	return chunks


static func _instance(mesh: Mesh, transforms: Array) -> MultiMeshInstance3D:
	var multimesh := MultiMesh.new()
	multimesh.transform_format = MultiMesh.TRANSFORM_3D
	multimesh.use_colors = true
	multimesh.mesh = mesh
	multimesh.instance_count = transforms.size()
	for i: int in transforms.size():
		var placed: Transform3D = transforms[i]
		multimesh.set_instance_transform(i, placed)
		# A little of each plant's own colour, from where it stands.
		var shade := 0.82 + fposmod(placed.origin.x * 12.9898 + placed.origin.z * 78.233, 1.0) * 0.3
		multimesh.set_instance_color(i, Color(shade, shade, shade * 0.92))
	var instance := MultiMeshInstance3D.new()
	instance.multimesh = multimesh
	return instance


## A fan of [param planes] upright quads about the trunk, each showing one cell of a
## [param cells]-wide sheet. Normals lean outward, as if the crown were one rounded
## mass, which lights a card tree far better than each card's own flat normal does;
## more [param rows] let that lean vary up the tree too.
static func _fan(
	meta: Dictionary, planes: int, cells: int, rows: int, material: Material
) -> ArrayMesh:
	var height: float = meta["height"]
	var half: float = meta["width"]
	half *= 0.5
	var bottom: float = meta["y_min"]
	var surface := SurfaceTool.new()
	surface.begin(Mesh.PRIMITIVE_TRIANGLES)
	surface.set_material(material)
	for plane: int in planes:
		var angle := plane * PI / planes
		var across := Vector3(cos(angle), 0.0, -sin(angle))
		var u0 := float(plane) / cells
		var u1 := float(plane + 1) / cells
		for r: int in rows:
			var y0 := float(r) / rows
			var y1 := float(r + 1) / rows
			for corner: Vector2 in [
				Vector2(0, y0),
				Vector2(1, y0),
				Vector2(0, y1),
				Vector2(1, y0),
				Vector2(1, y1),
				Vector2(0, y1),
			]:
				var offset := across * (corner.x * 2.0 - 1.0) * half
				var position := offset + Vector3.UP * (bottom + corner.y * height)
				var lean := (offset + Vector3.UP * (corner.y - 0.35) * height * 0.5).normalized()
				surface.set_normal(lean)
				surface.set_uv(Vector2(lerpf(u0, u1, corner.x), 1.0 - corner.y))
				surface.add_vertex(position)
	return surface.commit()


static func _card(path: String, half_width: float) -> ShaderMaterial:
	var material := ShaderMaterial.new()
	material.shader = FOLIAGE
	material.set_shader_parameter("albedo", load(path))
	material.set_shader_parameter("half_width", half_width)
	return material


## The trees in a glTF from scripts/blender/pine.py, with their needles drawn by
## game/needles.gdshader.
static func _modelled(path: String) -> Array[Mesh]:
	var scene: PackedScene = load(path)
	var node := scene.instantiate()
	var found: Array[Mesh] = []
	for child: Node in node.find_children("*", "MeshInstance3D", true, false):
		var mesh: Mesh = (child as MeshInstance3D).mesh
		for s: int in mesh.get_surface_count():
			# Loading the glTF again gives back the same meshes, with their needles done.
			var material := mesh.surface_get_material(s) as BaseMaterial3D
			if material and material.resource_name.ends_with("needles"):
				var needles := ShaderMaterial.new()
				needles.shader = NEEDLES
				needles.set_shader_parameter("albedo", material.albedo_texture)
				mesh.surface_set_material(s, needles)
		found.append(mesh)
	node.free()
	return found


## The meshes in a glTF scene, each at the origin, with [param soft] ones made
## scissored and two-sided, since sorted blending is too costly over this much grass.
static func _meshes(path: String, soft: bool) -> Array[Mesh]:
	var scene: PackedScene = load(path)
	var node := scene.instantiate()
	var found: Array[Mesh] = []
	for child: Node in node.find_children("*", "MeshInstance3D", true, false):
		var instance := child as MeshInstance3D
		var mesh: Mesh = instance.mesh
		if soft:
			for s: int in mesh.get_surface_count():
				var material := mesh.surface_get_material(s).duplicate() as BaseMaterial3D
				material.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA_SCISSOR
				material.alpha_scissor_threshold = 0.4
				material.cull_mode = BaseMaterial3D.CULL_DISABLED
				material.vertex_color_use_as_albedo = true
				mesh.surface_set_material(s, material)
		found.append(mesh)
	node.free()
	return found
