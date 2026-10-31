class_name TerrainMesh
extends RefCounted
## Turns a [Terrain] into chunks of mesh. The rider stays on the road, so a chunk's
## detail is fixed at build time by its distance from the road rather than switched
## at runtime by its distance from the camera.

const CHUNK := 64.0
const GROUND := "res://assets/polyhaven/forrest_ground_03/forrest_ground_03_%s_2k.jpg"
## Metres of texture per tile; the ground is mapped from world position.
const TILE := 2.0
const SKIRT := 3.0


static func build(terrain: Terrain) -> Node3D:
	var root := Node3D.new()
	root.name = "Terrain"
	var tint := ValueNoise.new(77)
	var material := _material()
	var rect := terrain.bounds
	for row: int in int(rect.size.y / CHUNK) + 1:
		for column: int in int(rect.size.x / CHUNK) + 1:
			var origin := rect.position + Vector2(column, row) * CHUNK
			var instance := MeshInstance3D.new()
			instance.mesh = _chunk(terrain, tint, origin, material)
			instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
			root.add_child(instance)
	return root


static func _chunk(
	terrain: Terrain, tint: ValueNoise, origin: Vector2, material: Material
) -> ArrayMesh:
	var centre := origin + Vector2.ONE * CHUNK * 0.5
	var distance := absf(terrain.route.nearest(centre.x, centre.y).y)
	var step := 2.0 if distance < 90.0 else (8.0 if distance < 260.0 else 32.0)
	var cells := int(CHUNK / step)
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
			var shade := 0.82 + tint.fractal(x / 40.0, z / 40.0, 2) * 0.25
			colors.append(Color(shade, shade * 1.02, shade * 0.95))
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


static func _material() -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_texture = load(GROUND % "diffuse")
	material.normal_enabled = true
	material.normal_texture = load(GROUND % "nor_gl")
	material.roughness_texture = load(GROUND % "rough")
	material.ao_enabled = true
	material.ao_texture = load(GROUND % "ao")
	material.albedo_color = Color(0.5, 0.46, 0.38)
	material.vertex_color_use_as_albedo = true
	material.uv1_triplanar = true
	material.uv1_world_triplanar = true
	material.uv1_scale = Vector3.ONE / TILE
	material.texture_filter = BaseMaterial3D.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS_ANISOTROPIC
	return material
