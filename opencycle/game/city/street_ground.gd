class_name StreetGround
extends RefCounted
## Road, kerbs and pavement. The road is the route's ribbon; the pavement stands a real
## kerb's height above it, because that step and its shadow are half of what makes a
## street look like one.

const ROAD := "res://assets/polyhaven/asphalt_02/asphalt_02_%s_2k.jpg"
const PAVING := "res://assets/polyhaven/square_concrete_pavers/square_concrete_pavers_%s_2k.jpg"
const ROAD_SHADER: Shader = preload("res://game/road.gdshader")
const SHEEN_SHADER: Shader = preload("res://game/wet_sheen.gdshader")
const PAINT_SHADER: Shader = preload("res://game/road_paint.gdshader")
const KERB_HEIGHT := 0.15
const KERB_WIDTH := 0.26
const PAINT_LIFT := 0.012
const TUNNEL_REACH := 7.0
const YELLOW_PAINT := Vector3(0.3, 0.22, 0.06)
const WHITE_PAINT := Vector3(0.46, 0.46, 0.42)
const PAINT_WORN := 0.55


static func build(route: Route, street: Street, length: float, section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "Ground"
	var kit := CityMesh.new()
	var start := -Street.REACH
	var end := length + Street.REACH
	var half := NeonBiome.STREET_HALF
	var used: Dictionary[String, Material] = Once.made("ground", _materials)
	var road := used["road"]
	var patch := used["patch"]
	var covered_patch := used["covered_patch"]
	var covered := used["covered"]
	var paving := used["paving"]
	var kerb := used["kerb"]
	var yellow := used["yellow"]
	var white := used["white"]
	var covered_yellow := used["covered_yellow"]
	var covered_white := used["covered_white"]
	var s := start
	while s < end:
		var next := minf(s + Route.STEP, end)
		if not section.holds(s):
			s = next
			continue
		kit.at(s)
		var under_roof := street.districts.kind_at(s) == Districts.Kind.UNDERPASS
		_strip(kit, route, s, next, -half, half, 0.0, covered if under_roof else road, 1.0)
		for side: float in [-1.0, 1.0]:
			var a := half * side
			var b := (half + KERB_WIDTH) * side
			var wall := (half + Street.SIDEWALK) * side
			_strip(kit, route, s, next, a, b, KERB_HEIGHT, kerb, 1.0)
			_strip(kit, route, s, next, b, wall, KERB_HEIGHT, paving, 1.0)
			_kerb_face(kit, route, s, next, a, kerb, side)
			var line := (half - 0.28) * side
			var paint := covered_yellow if under_roof else yellow
			_strip(kit, route, s, next, line - 0.05, line + 0.05, PAINT_LIFT, paint, 1.0)
			var inner := (half - 0.46) * side
			_strip(kit, route, s, next, inner - 0.05, inner + 0.05, PAINT_LIFT, paint, 1.0)
		s = next
	for patched: Vector4 in street.patches:
		if not section.holds(patched.x):
			continue
		kit.at(patched.x)
		var patched_covered := street.districts.kind_at(patched.x) == Districts.Kind.UNDERPASS
		_strip(
			kit,
			route,
			patched.x,
			patched.x + patched.z,
			patched.y - patched.w * 0.5,
			patched.y + patched.w * 0.5,
			0.006,
			covered_patch if patched_covered else patch,
			1.0
		)
	s = start
	while s < end:
		if section.holds(s):
			kit.at(s)
			_arrow(kit, route, s, -1.4, _arrow_paint(street, s, white, covered_white))
			var back := s + 6.0
			_arrow(kit, route, back, 1.4, _arrow_paint(street, back, white, covered_white))
		s += 42.0
	kit.commit(root)
	return root


static func _materials() -> Dictionary[String, Material]:
	return {
		"road": _road_material(ROAD, Color(1.0, 1.0, 1.0)),
		# Fresher asphalt of the same kind: darker, and only by a little.
		"patch": _road_material(ROAD, Color(0.8, 0.8, 0.82)),
		"covered_patch": _road_material(ROAD, Color(0.74, 0.74, 0.76), 0.0, TUNNEL_REACH),
		# Dry under the roof, where all it can mirror is the tubes a few metres overhead.
		"covered": _road_material(ROAD, Color(0.92, 0.92, 0.92), 0.0, TUNNEL_REACH),
		"paving": _paving(),
		"kerb": _plain(Color(0.42, 0.41, 0.39), 0.75),
		"yellow": _paint(YELLOW_PAINT, PAINT_WORN),
		"white": _paint(WHITE_PAINT, PAINT_WORN),
		"covered_yellow": _paint(YELLOW_PAINT, PAINT_WORN, TUNNEL_REACH),
		"covered_white": _paint(WHITE_PAINT, PAINT_WORN, TUNNEL_REACH),
	}


static func _strip(
	kit: CityMesh,
	route: Route,
	from: float,
	to: float,
	left: float,
	right: float,
	lift: float,
	material: Material,
	tile: float
) -> void:
	var s := from
	while s < to:
		var next := minf(s + Route.STEP, to)
		var up := Vector3.UP * lift
		var a := route.at(s) + route.right(s) * left + up
		var b := route.at(s) + route.right(s) * right + up
		var c := route.at(next) + route.right(next) * right + up
		var d := route.at(next) + route.right(next) * left + up
		(
			kit
			. face(
				material,
				[a, b, c, d],
				[
					Vector2(left, s) * tile,
					Vector2(right, s) * tile,
					Vector2(right, next) * tile,
					Vector2(left, next) * tile,
				],
				Vector3.UP
			)
		)
		s = next


static func _kerb_face(
	kit: CityMesh, route: Route, from: float, to: float, at: float, material: Material, side: float
) -> void:
	var a := route.at(from) + route.right(from) * at
	var b := route.at(to) + route.right(to) * at
	var up := Vector3.UP * KERB_HEIGHT
	kit.quad(material, a, b, b + up, a + up, route.right(from) * -side)


## A lane arrow painted on the road.
static func _arrow(kit: CityMesh, route: Route, s: float, offset: float, paint: Material) -> void:
	_strip(kit, route, s, s + 2.4, offset - 0.08, offset + 0.08, PAINT_LIFT, paint, 1.0)
	var along := route.tangent(s)
	var across := route.right(s)
	var tip := route.at(s + 3.6) + across * offset + Vector3.UP * PAINT_LIFT
	var left := route.at(s + 2.4) + across * (offset - 0.34) + Vector3.UP * PAINT_LIFT
	var right := route.at(s + 2.4) + across * (offset + 0.34) + Vector3.UP * PAINT_LIFT
	kit.face(
		paint,
		[left, right, tip, tip + along * 0.0],
		[Vector2.ZERO, Vector2.RIGHT, Vector2.ONE, Vector2.UP],
		Vector3.UP
	)


static func _road_material(
	path: String, shade: Color, puddles := 1.0, reach := 28.0
) -> ShaderMaterial:
	var material := ShaderMaterial.new()
	material.shader = ROAD_SHADER
	material.set_shader_parameter("albedo", load(path % "diffuse"))
	material.set_shader_parameter("normals", load(path % "nor_gl"))
	material.set_shader_parameter("rough", load(path % "rough"))
	material.set_shader_parameter("tint", shade)
	material.set_shader_parameter("puddles", puddles)
	material.next_pass = _sheen(1.0, puddles, reach)
	return material


static func _arrow_paint(street: Street, s: float, open: Material, covered: Material) -> Material:
	return covered if street.districts.kind_at(s) == Districts.Kind.UNDERPASS else open


static func _sheen(strength: float, puddles: float, reach := 28.0, worn := 0.0) -> ShaderMaterial:
	var sheen := ShaderMaterial.new()
	sheen.shader = SHEEN_SHADER
	sheen.set_shader_parameter("strength", strength)
	sheen.set_shader_parameter("puddles", puddles)
	sheen.set_shader_parameter("reach", reach)
	sheen.set_shader_parameter("worn", worn)
	sheen.set_shader_parameter("sky", 0.0 if reach <= TUNNEL_REACH else 1.0)
	return sheen


static func _paint(colour: Vector3, worn: float, reach := 28.0) -> ShaderMaterial:
	var material := ShaderMaterial.new()
	material.shader = PAINT_SHADER
	material.set_shader_parameter("paint", colour)
	material.set_shader_parameter("worn", worn)
	material.next_pass = _sheen(0.7, 0.0, reach, worn)
	return material


static func _paving() -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_texture = load(PAVING % "diffuse")
	# Taking the texture's warmth out: it reads as terracotta under warm shop light.
	material.albedo_color = Color(0.55, 0.62, 0.72)
	material.normal_enabled = true
	material.normal_texture = load(PAVING % "nor_gl")
	material.roughness_texture = load(PAVING % "rough")
	material.roughness = 0.75
	material.uv1_scale = Vector3(0.5, 0.5, 1.0)
	material.texture_filter = BaseMaterial3D.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS_ANISOTROPIC
	material.next_pass = _sheen(0.4, 0.0)
	return material


static func _plain(colour: Color, roughness: float) -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_color = colour
	material.roughness = roughness
	return material
