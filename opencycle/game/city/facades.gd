class_name Facades
extends RefCounted
## The buildings above the shops: stepped boxes in real wall finishes, with windows the
## shader cuts into them, and air-con units hung on the front.

const SHADER: Shader = preload("res://game/facade.gdshader")
const FINISHES: Array[Array] = [
	["rectangular_facade_tiles", 2.0],
	["square_tiled_wall", 1.6],
	["worn_plaster_wall", 3.0],
	["concrete_wall_004", 3.0],
]
const AIRCON := "res://assets/polyhaven/exterior_aircon_unit/exterior_aircon_unit_1k.gltf"
const GROUND_FLOOR := 4.1
const BELOW := 8.0


static func build(route: Route, street: Street) -> Node3D:
	var root := Node3D.new()
	root.name = "Facades"
	var kit := CityMesh.new()
	var materials: Array[ShaderMaterial] = []
	for finish: Array in FINISHES:
		var texture: String = finish[0]
		var tile: float = finish[1]
		materials.append(_finish(texture, tile))
	var units: Array[Transform3D] = []
	var frame := StandardMaterial3D.new()
	frame.albedo_color = Color(0.1, 0.1, 0.1)
	frame.roughness = 0.7
	for tower: Street.Tower in street.towers:
		kit.at(tower.s)
		_tower(kit, route, tower, materials[tower.facade], units, frame)
	kit.commit(root)
	Furniture.instances(root, AIRCON, units)
	return root


static func _tower(
	kit: CityMesh,
	route: Route,
	tower: Street.Tower,
	material: Material,
	units: Array[Transform3D],
	frame: Material
) -> void:
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var base := route.at(tower.s)
	var front := base + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	var colour := Color(tower.tint.r, tower.tint.g, tower.tint.b, tower.age)
	var layout := Vector2(tower.bay, tower.storey)
	var rise := tower.height / (tower.tiers + 1)
	var width := tower.width
	var depth := tower.depth
	var bottom := GROUND_FLOOR
	for tier: int in tower.tiers + 1:
		var plan := front + outward * (tier * 3.0)
		var left := plan - along * width * 0.5
		var right := plan + along * width * 0.5
		var top := bottom + rise
		_wall(kit, material, left, right, base.y, bottom, top, -outward, colour, layout)
		var back := outward * depth
		# The sides are party walls, blank, and run down below the neighbours' roofs.
		var low := base.y - BELOW if tier == 0 else base.y + bottom
		var from := low - base.y
		_wall(kit, material, left + back, left, base.y, from, top, -along, colour, Vector2.ZERO)
		_wall(kit, material, right, right + back, base.y, from, top, along, colour, Vector2.ZERO)
		var roof := Vector3.UP * (base.y + top - plan.y)
		kit.quad(
			frame, left + roof, right + roof, right + back + roof, left + back + roof, Vector3.UP
		)
		bottom = top
		width = maxf(width - 4.0, 6.0)
		depth = maxf(depth - 6.0, 6.0)
	# Units hang from the lowest tier's wall, on the same grid the shader draws windows on.
	var first_top := GROUND_FLOOR + rise
	var left_end := front - along * tower.width * 0.5
	for unit: Vector2i in tower.units:
		var y := GROUND_FLOOR + unit.y * tower.storey + tower.storey * 0.16
		if y + 0.7 > first_top:
			continue
		var at := left_end + along * ((unit.x + 0.5) * tower.bay)
		var place := Vector3(at.x, base.y + y, at.z) - outward * 0.3
		# A real unit is about a metre wide; rotated to face out from the wall, on a bracket.
		var basis := Basis(along, Vector3.UP, -outward)
		units.append(Transform3D(basis, place))
		kit.box(
			frame,
			Transform3D(basis, place - Vector3.UP * 0.04),
			Vector3(0.9, 0.04, 0.54),
			Color.WHITE
		)


static func _wall(
	kit: CityMesh,
	material: Material,
	from: Vector3,
	to: Vector3,
	ground: float,
	low: float,
	high: float,
	facing: Vector3,
	colour: Color,
	layout: Vector2
) -> void:
	var a := Vector3(from.x, ground + low, from.z)
	var b := Vector3(to.x, ground + low, to.z)
	var c := Vector3(to.x, ground + high, to.z)
	var d := Vector3(from.x, ground + high, from.z)
	var span := (to - from).length()
	kit.face(
		material,
		[a, b, c, d],
		[Vector2(0.0, low), Vector2(span, low), Vector2(span, high), Vector2(0.0, high)],
		facing,
		colour,
		layout
	)


static func _finish(texture: String, tile: float) -> ShaderMaterial:
	var path := "res://assets/polyhaven/%s/%s_%%s_2k.jpg" % [texture, texture]
	var material := ShaderMaterial.new()
	material.shader = SHADER
	material.set_shader_parameter("albedo", load(path % "diffuse"))
	material.set_shader_parameter("normals", load(path % "nor_gl"))
	material.set_shader_parameter("rough", load(path % "rough"))
	material.set_shader_parameter("tile", tile)
	material.set_shader_parameter("first_storey", GROUND_FLOOR)
	return material
