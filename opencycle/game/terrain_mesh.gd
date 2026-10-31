class_name TerrainMesh
extends RefCounted
## Turns a [Terrain] into chunks of mesh. The rider stays on the road, so a chunk's
## detail is fixed at build time by its distance from the road rather than switched
## at runtime by its distance from the camera.

const CHUNK := 64.0
## Chunks this far from the road merge into fours, since nothing there needs culling.
const MERGE_DISTANCE := 300.0
const GROUND_SHADER: Shader = preload("res://game/ground.gdshader")
const GROUND := "res://assets/polyhaven/forrest_ground_03/forrest_ground_03_%s_2k.jpg"
## Metres of ground per tile of texture.
const TILE := 2.0
const SKIRT := 3.0


static func build(terrain: Terrain) -> Node3D:
	var root := Node3D.new()
	root.name = "Terrain"
	var tint := ValueNoise.new(77)
	var material := _material()
	var rect := terrain.bounds
	for row: int in int(rect.size.y / (CHUNK * 2.0)) + 1:
		for column: int in int(rect.size.x / (CHUNK * 2.0)) + 1:
			var origin := rect.position + Vector2(column, row) * CHUNK * 2.0
			var centre := origin + Vector2.ONE * CHUNK
			if absf(terrain.route.nearest(centre.x, centre.y).y) > MERGE_DISTANCE:
				_add(root, _chunk(terrain, tint, origin, CHUNK * 2.0, material))
				continue
			for j: int in 2:
				for i: int in 2:
					var corner := origin + Vector2(i, j) * CHUNK
					_add(root, _chunk(terrain, tint, corner, CHUNK, material))
	return root


static func _add(root: Node3D, mesh: ArrayMesh) -> void:
	var instance := MeshInstance3D.new()
	instance.mesh = mesh
	instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	root.add_child(instance)


static func _chunk(
	terrain: Terrain, tint: ValueNoise, origin: Vector2, span: float, material: Material
) -> ArrayMesh:
	var centre := origin + Vector2.ONE * span * 0.5
	var distance := absf(terrain.route.nearest(centre.x, centre.y).y)
	var step := 2.0 if distance < 90.0 else (8.0 if distance < 260.0 else 32.0)
	var cells := int(span / step)
	var size := cells + 1
	# One extra ring around the grid, so edge normals match the chunk next door.
	var ring := size + 2
	var heights := PackedFloat32Array()
	heights.resize(ring * ring)
	for j: int in ring:
		for i: int in ring:
			heights[j * ring + i] = terrain.height(
				origin.x + (i - 1) * step, origin.y + (j - 1) * step
			)

	var positions := PackedVector3Array()
	var normals := PackedVector3Array()
	var colors := PackedColorArray()
	var indices := PackedInt32Array()
	for j: int in size:
		for i: int in size:
			var at := (j + 1) * ring + i + 1
			var x := origin.x + i * step
			var z := origin.y + j * step
			positions.append(Vector3(x, heights[at], z))
			normals.append(
				(
					Vector3(
						heights[at - 1] - heights[at + 1],
						2.0 * step,
						heights[at - ring] - heights[at + ring]
					)
					. normalized()
				)
			)
			var shade := 0.9 + tint.fractal(x / 40.0, z / 40.0, 2) * 0.35
			var moss := 0.5 + tint.fractal(x / 17.0 + 91.0, z / 17.0, 2) * 0.8
			colors.append(Color(shade, shade, shade, clampf(moss, 0.0, 1.0)))
	for j: int in cells:
		for i: int in cells:
			var a := j * size + i
			_quad(indices, a, a + 1, a + size, a + size + 1)
	_skirts(positions, normals, colors, indices, size)

	var arrays := []
	arrays.resize(Mesh.ARRAY_MAX)
	arrays[Mesh.ARRAY_VERTEX] = positions
	arrays[Mesh.ARRAY_NORMAL] = normals
	arrays[Mesh.ARRAY_COLOR] = colors
	arrays[Mesh.ARRAY_INDEX] = indices
	var mesh := ArrayMesh.new()
	mesh.add_surface_from_arrays(Mesh.PRIMITIVE_TRIANGLES, arrays)
	mesh.surface_set_material(0, material)
	return mesh


## A curtain under each edge, hiding the cracks where a chunk meets a coarser one.
static func _skirts(
	positions: PackedVector3Array,
	normals: PackedVector3Array,
	colors: PackedColorArray,
	indices: PackedInt32Array,
	size: int,
) -> void:
	var edges: Array[Array] = [
		range(0, size),
		range(size * (size - 1), size * size),
		range(0, size * size, size),
		range(size - 1, size * size, size),
	]
	for edge: Array in edges:
		var start := positions.size()
		for k: int in edge.size():
			var source: int = edge[k]
			positions.append(positions[source] - Vector3.UP * SKIRT)
			normals.append(normals[source])
			colors.append(colors[source])
			if k > 0:
				var previous: int = edge[k - 1]
				_quad(indices, previous, source, start + k - 1, start + k)
				_quad(indices, source, previous, start + k, start + k - 1)


static func _quad(indices: PackedInt32Array, a: int, b: int, c: int, d: int) -> void:
	indices.append_array([a, b, c, b, d, c])


static func _material() -> ShaderMaterial:
	var material := ShaderMaterial.new()
	material.shader = GROUND_SHADER
	material.set_shader_parameter("albedo", load(GROUND % "diffuse"))
	material.set_shader_parameter("normals", load(GROUND % "nor_gl"))
	material.set_shader_parameter("roughness_map", load(GROUND % "rough"))
	material.set_shader_parameter("ao_map", load(GROUND % "ao"))
	material.set_shader_parameter("tile", TILE)
	return material
