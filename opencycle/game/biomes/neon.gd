class_name NeonBiome
extends RefCounted
## A cyberpunk street at night: rain-wet road, towers of lit windows, neon overhead.
## Everything here is generated, with no art to fetch but the road's asphalt.

const ASPHALT := "res://assets/polyhaven/asphalt_02/asphalt_02_%s_2k.jpg"
const BUILDING: Shader = preload("res://game/building.gdshader")
const SIGN: Shader = preload("res://game/sign.gdshader")
const LENGTH := 200.0
const TILE := 4.0
const STREET_HALF := Route.HALF_WIDTH + 1.0
const LAMP_HEIGHT := 6.5


static func build(parent: Node3D, run_seed: int, route: Route, quality: String) -> void:
	var street := Street.generate(run_seed, LENGTH)
	var start := -Street.REACH
	var end := LENGTH + Street.REACH
	parent.add_child(_road(route, start, end))
	parent.add_child(_towers(route, street))
	parent.add_child(_signs(route, street, quality == "max"))
	parent.add_child(_lamps(route, street))
	_environment(parent, quality == "max")


static func _road(route: Route, start: float, end: float) -> Node3D:
	var root := Node3D.new()
	root.name = "Road"
	var verge := STREET_HALF + Street.SIDEWALK
	var pavement := StandardMaterial3D.new()
	pavement.albedo_color = Color(0.09, 0.09, 0.1)
	pavement.roughness = 0.35
	_ribbon(root, route, start, end, -verge, verge, 0.0, pavement)
	var wet := StandardMaterial3D.new()
	wet.albedo_texture = load(ASPHALT % "diffuse")
	wet.albedo_color = Color(0.32, 0.32, 0.36)
	wet.normal_enabled = true
	wet.normal_scale = 0.35
	wet.normal_texture = load(ASPHALT % "nor_gl")
	# Damp all over; the reflections of the signs are what sell the night.
	wet.roughness = 0.14
	wet.metallic = 0.25
	wet.texture_filter = BaseMaterial3D.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS_ANISOTROPIC
	_ribbon(root, route, start, end, -STREET_HALF, STREET_HALF, 0.05, wet)
	var paint := StandardMaterial3D.new()
	paint.albedo_color = Color(0.7, 0.7, 0.66)
	paint.roughness = 0.8
	var s := start
	while s < end:
		_ribbon(root, route, s, minf(s + 3.0, end), -0.07, 0.07, 0.06, paint)
		s += 9.0
	return root


static func _ribbon(
	root: Node3D,
	route: Route,
	start: float,
	end: float,
	left: float,
	right: float,
	lift: float,
	material: Material
) -> void:
	var surface := SurfaceTool.new()
	surface.begin(Mesh.PRIMITIVE_TRIANGLES)
	var s := start
	while s < end:
		var next := minf(s + Route.STEP, end)
		var corners: Array[Array] = [
			[s, left], [next, left], [s, right], [s, right], [next, left], [next, right]
		]
		for corner: Array in corners:
			var along: float = corner[0]
			var across: float = corner[1]
			surface.set_normal(Vector3.UP)
			surface.set_tangent(Plane(1.0, 0.0, 0.0, 1.0))
			surface.set_uv(Vector2(across, along) / TILE)
			surface.add_vertex(route.at(along) + route.right(along) * across + Vector3.UP * lift)
		s = next
	surface.set_material(material)
	var instance := MeshInstance3D.new()
	instance.mesh = surface.commit()
	instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	root.add_child(instance)


static func _towers(route: Route, street: Street) -> MeshInstance3D:
	var surface := SurfaceTool.new()
	surface.begin(Mesh.PRIMITIVE_TRIANGLES)
	for index: int in street.towers.size():
		var tower := street.towers[index]
		var base := route.at(tower.s)
		var along := route.tangent(tower.s)
		var outward := route.right(tower.s) * tower.side
		var front := base + outward * (STREET_HALF + Street.SIDEWALK)
		var color := Color.from_hsv(tower.hue, 0.75, 1.0, float(index % 97) / 97.0)
		var bottom := base.y - 8.0
		var width := tower.width
		var depth := tower.depth
		var floor_height := bottom
		var total := tower.height + 8.0
		for tier: int in tower.tiers + 1:
			var rise := total / (tower.tiers + 1)
			var inset := tier * 3.0
			_box(
				surface,
				front + outward * inset,
				along,
				outward,
				Vector3(width, rise, depth),
				floor_height,
				color
			)
			floor_height += rise
			width = maxf(width - 4.0, 6.0)
			depth = maxf(depth - 6.0, 6.0)
	var material := ShaderMaterial.new()
	material.shader = BUILDING
	surface.set_material(material)
	var instance := MeshInstance3D.new()
	instance.name = "Towers"
	instance.mesh = surface.commit()
	instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	return instance


## Four walls and a roof, UV'd in metres so windows come out the same size everywhere.
static func _box(
	surface: SurfaceTool,
	front: Vector3,
	along: Vector3,
	outward: Vector3,
	size: Vector3,
	bottom: float,
	color: Color
) -> void:
	var half := along * size.x * 0.5
	var a := front - half
	var b := front + half
	var c := b + outward * size.z
	var d := a + outward * size.z
	_wall(surface, a, b, size.x, bottom, bottom + size.y, color)
	_wall(surface, b, c, size.z, bottom, bottom + size.y, color)
	_wall(surface, c, d, size.x, bottom, bottom + size.y, color)
	_wall(surface, d, a, size.z, bottom, bottom + size.y, color)
	var top := Vector3.UP * (bottom + size.y)
	_quad(
		surface,
		_at(a, top.y),
		_at(b, top.y),
		_at(c, top.y),
		_at(d, top.y),
		Vector3.UP,
		color,
		Vector2.ZERO
	)


static func _wall(
	surface: SurfaceTool,
	from: Vector3,
	to: Vector3,
	span: float,
	low: float,
	high: float,
	color: Color
) -> void:
	var normal := (to - from).cross(Vector3.UP).normalized()
	_quad(
		surface,
		_at(from, low),
		_at(to, low),
		_at(to, high),
		_at(from, high),
		normal,
		color,
		Vector2(span, high - low)
	)


static func _at(plan: Vector3, y: float) -> Vector3:
	return Vector3(plan.x, y, plan.z)


static func _quad(
	surface: SurfaceTool,
	a: Vector3,
	b: Vector3,
	c: Vector3,
	d: Vector3,
	normal: Vector3,
	color: Color,
	uv: Vector2
) -> void:
	var corners: Array[Array] = [
		[a, Vector2(0.0, 0.0)],
		[c, Vector2(uv.x, uv.y)],
		[b, Vector2(uv.x, 0.0)],
		[a, Vector2(0.0, 0.0)],
		[d, Vector2(0.0, uv.y)],
		[c, Vector2(uv.x, uv.y)],
	]
	# Both windings: which way round a wall comes out depends on the side of the road.
	var facing := normal.dot((b - a).cross(c - a)) < 0.0
	if not facing:
		corners.reverse()
	for corner: Array in corners:
		var at: Vector3 = corner[0]
		surface.set_color(color)
		surface.set_normal(normal if facing else -normal)
		var mapped: Vector2 = corner[1]
		surface.set_uv(mapped)
		surface.add_vertex(at)


static func _signs(route: Route, street: Street, best: bool) -> Node3D:
	var root := Node3D.new()
	root.name = "Signs"
	var lights_left := 48 if best else 14
	# Only the signs the ride passes close to get a real light; the rest just glow.
	for hung: Street.Sign in street.signs:
		var along := route.tangent(hung.s)
		var outward := route.right(hung.s) * hung.side
		var wall := (
			route.at(hung.s)
			+ outward * (STREET_HALF + Street.SIDEWALK - hung.overhang * 0.5 - 0.15)
		)
		var color := Color.from_hsv(hung.hue, 0.85, 1.0)
		var quad := QuadMesh.new()
		quad.size = hung.size
		var material := ShaderMaterial.new()
		material.shader = SIGN
		material.set_shader_parameter("glow", color)
		material.set_shader_parameter("size", hung.size)
		material.set_shader_parameter("text", hung.s)
		quad.material = material
		var panel := MeshInstance3D.new()
		panel.mesh = quad
		panel.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		var centre := wall + Vector3.UP * (hung.elevation + hung.size.y * 0.5)
		# A blade faces along the street, a banner faces across it.
		var facing := along if hung.overhang > 0.0 else outward
		panel.transform = Transform3D(Basis.looking_at(facing, Vector3.UP), centre)
		root.add_child(panel)
		if lights_left > 0 and hung.s > -10.0 and hung.s < LENGTH + 30.0:
			lights_left -= 1
			var glow := OmniLight3D.new()
			glow.light_color = color
			glow.light_energy = 5.0
			glow.omni_range = 24.0
			glow.omni_attenuation = 1.4
			glow.position = centre - outward * 1.2
			root.add_child(glow)
	return root


static func _lamps(route: Route, street: Street) -> MultiMeshInstance3D:
	var box := BoxMesh.new()
	box.size = Vector3(0.14, 0.1, 2.2)
	var material := StandardMaterial3D.new()
	material.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	material.albedo_color = Color(4.0, 3.2, 2.4)
	box.material = material
	var multimesh := MultiMesh.new()
	multimesh.transform_format = MultiMesh.TRANSFORM_3D
	multimesh.mesh = box
	multimesh.instance_count = street.lamps.size() * 2
	var i := 0
	for s: float in street.lamps:
		for side: float in [-1.0, 1.0]:
			var place := (
				route.at(s) + route.right(s) * side * STREET_HALF + Vector3.UP * LAMP_HEIGHT
			)
			multimesh.set_instance_transform(
				i, Transform3D(Basis.looking_at(route.right(s)), place)
			)
			i += 1
	var instance := MultiMeshInstance3D.new()
	instance.name = "Lamps"
	instance.multimesh = multimesh
	# Light from every other lamp on the stretch the ride sees, alternating sides.
	var side := 1.0
	for s: float in street.lamps:
		if s < -10.0 or s > LENGTH + 30.0:
			continue
		var pool := OmniLight3D.new()
		pool.light_color = Color(1.0, 0.8, 0.6)
		pool.light_energy = 2.5
		pool.omni_range = 18.0
		pool.position = route.at(s) + route.right(s) * side * STREET_HALF + Vector3.UP * LAMP_HEIGHT
		instance.add_child(pool)
		side = -side
	instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	return instance


static func _environment(parent: Node3D, best: bool) -> void:
	var sky_material := ProceduralSkyMaterial.new()
	sky_material.sky_top_color = Color(0.015, 0.012, 0.04)
	sky_material.sky_horizon_color = Color(0.2, 0.05, 0.22)
	sky_material.ground_horizon_color = Color(0.1, 0.03, 0.12)
	sky_material.ground_bottom_color = Color(0.0, 0.0, 0.0)
	sky_material.sun_angle_max = 0.0
	var sky := Sky.new()
	sky.sky_material = sky_material
	var environment := Environment.new()
	environment.background_mode = Environment.BG_SKY
	environment.sky = sky
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	environment.ambient_light_color = Color(0.3, 0.3, 0.5)
	environment.ambient_light_energy = 0.9
	environment.reflected_light_source = Environment.REFLECTION_SOURCE_SKY
	environment.tonemap_mode = Environment.TONE_MAPPER_AGX
	environment.glow_enabled = true
	environment.glow_intensity = 0.45
	environment.glow_bloom = 0.03
	environment.glow_hdr_threshold = 1.4
	environment.ssr_enabled = true
	environment.ssr_max_steps = 96 if best else 32
	environment.ssr_depth_tolerance = 0.5
	environment.fog_enabled = true
	environment.fog_mode = Environment.FOG_MODE_DEPTH
	environment.fog_light_color = Color(0.13, 0.06, 0.2)
	environment.fog_depth_begin = 15.0
	environment.fog_depth_end = 300.0
	environment.ssao_enabled = best
	environment.volumetric_fog_enabled = best
	environment.volumetric_fog_density = 0.012
	var world := WorldEnvironment.new()
	world.environment = environment
	parent.add_child(world)
	var viewport := parent.get_viewport()
	viewport.use_taa = true
	if best:
		viewport.msaa_3d = Viewport.MSAA_4X
	else:
		viewport.scaling_3d_mode = Viewport.SCALING_3D_MODE_FSR
		viewport.scaling_3d_scale = 0.67
