class_name RoadMesh
extends RefCounted
## The road: a gravel shoulder, asphalt over it, and paint over that, each a ribbon
## following the route a little above the last.

const ASPHALT := "res://assets/polyhaven/asphalt_02/asphalt_02_%s_2k.jpg"
const GRAVEL := "res://assets/polyhaven/gravel_road/gravel_road_%s_2k.jpg"
const SHOULDER: Shader = preload("res://game/shoulder.gdshader")
const TILE := 4.0
const SHOULDER_WIDTH := 1.5
const LINE_WIDTH := 0.1
const LINE_INSET := 0.25
const DASH := 3.0
const DASH_GAP := 6.0


static func build(route: Route) -> Node3D:
	var root := Node3D.new()
	root.name = "Road"
	var half := Route.HALF_WIDTH
	var start := -Route.MARGIN
	var end := Route.LENGTH + Route.MARGIN
	var verge := half + SHOULDER_WIDTH
	_add(root, _ribbon(route, start, end, -verge, verge, -0.03), _shoulder(verge))
	_add(root, _ribbon(route, start, end, -half, half, 0.0), _textured(ASPHALT))
	var paint := _paint()
	var edge := half - LINE_INSET
	_add(root, _ribbon(route, start, end, edge - LINE_WIDTH, edge, 0.012), paint)
	_add(root, _ribbon(route, start, end, -edge, -edge + LINE_WIDTH, 0.012), paint)
	var surface := SurfaceTool.new()
	surface.begin(Mesh.PRIMITIVE_TRIANGLES)
	var s := start
	while s < end:
		var stop := minf(s + DASH, end)
		_strip(surface, route, s, stop, -LINE_WIDTH * 0.5, LINE_WIDTH * 0.5, 0.012)
		s += DASH + DASH_GAP
	surface.set_material(paint)
	_instance(root, surface.commit())
	return root


static func _add(root: Node3D, surface: SurfaceTool, material: Material) -> void:
	surface.set_material(material)
	_instance(root, surface.commit())


static func _instance(root: Node3D, mesh: ArrayMesh) -> void:
	var instance := MeshInstance3D.new()
	instance.mesh = mesh
	instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	root.add_child(instance)


static func _ribbon(
	route: Route, start: float, end: float, left: float, right: float, lift: float
) -> SurfaceTool:
	var surface := SurfaceTool.new()
	surface.begin(Mesh.PRIMITIVE_TRIANGLES)
	_strip(surface, route, start, end, left, right, lift)
	return surface


## Appends triangles covering a stretch of road between two offsets from its centre.
static func _strip(
	surface: SurfaceTool,
	route: Route,
	start: float,
	end: float,
	left: float,
	right: float,
	lift: float
) -> void:
	var s := start
	while s < end:
		var next := minf(s + Route.STEP, end)
		var a := _vertex(route, s, left, lift)
		var b := _vertex(route, s, right, lift)
		var c := _vertex(route, next, left, lift)
		var d := _vertex(route, next, right, lift)
		for corner: Array in [[a, s, left], [c, next, left], [b, s, right]]:
			_emit(surface, corner)
		for corner: Array in [[b, s, right], [c, next, left], [d, next, right]]:
			_emit(surface, corner)
		s = next


static func _emit(surface: SurfaceTool, corner: Array) -> void:
	var position: Vector3 = corner[0]
	var along: float = corner[1]
	var across: float = corner[2]
	surface.set_normal(Vector3.UP)
	surface.set_tangent(Plane(1.0, 0.0, 0.0, 1.0))
	surface.set_uv(Vector2(across, along) / TILE)
	surface.add_vertex(position)


static func _vertex(route: Route, s: float, offset: float, lift: float) -> Vector3:
	return route.at(s) + route.right(s) * offset + Vector3.UP * lift


static func _shoulder(verge: float) -> ShaderMaterial:
	var material := ShaderMaterial.new()
	material.shader = SHOULDER
	material.set_shader_parameter("albedo", load(GRAVEL % "diffuse"))
	material.set_shader_parameter("normals", load(GRAVEL % "nor_gl"))
	material.set_shader_parameter("roughness_map", load(GRAVEL % "rough"))
	material.set_shader_parameter("half_width", verge / TILE)
	return material


static func _textured(path: String) -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_texture = load(path % "diffuse")
	# Poly Haven's asphalt is shot sun-bleached; a country road in a forest reads darker.
	material.albedo_color = Color(0.55, 0.55, 0.56)
	material.normal_enabled = true
	material.normal_texture = load(path % "nor_gl")
	material.roughness_texture = load(path % "rough")
	material.texture_filter = BaseMaterial3D.TEXTURE_FILTER_LINEAR_WITH_MIPMAPS_ANISOTROPIC
	return material


static func _paint() -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_color = Color(0.6, 0.59, 0.55)
	material.roughness = 0.75
	return material
