class_name Overhead
extends RefCounted
## Draws [Undercity]: the deck closing off the sky, what holds it up and runs beneath it,
## the slot of light over the road, and the traffic going by above.

const CONCRETE := "res://assets/polyhaven/concrete_wall_004/concrete_wall_004_%s_2k.jpg"
const BRUTAL: Shader = preload("res://game/brutal.gdshader")
const TRAFFIC: Shader = preload("res://game/traffic.gdshader")
const STRETCH := 8.0
const DECK_WIDE := 70.0
const CONCRETE_METRES := 5.0
## The waffle cast into the deck's underside.
const COFFER := 5.0
const COFFER_DROP := 1.1
const SLOT_DEEP := 14.0
const HEADLIGHT := Color(0.85, 0.93, 1.0)
const TAIL_LIGHT := Color(1.0, 0.12, 0.06)
const STRIP := Color(0.16, 0.26, 0.4)
const HAZARD := Color(1.0, 0.08, 0.03)
const SHAFT_COLOUR := Color(0.62, 0.8, 1.0)


static func build(route: Route, city: Undercity, length: float) -> Node3D:
	var root := Node3D.new()
	root.name = "Overhead"
	var kit := CityMesh.new()
	var concrete := ShaderMaterial.new()
	concrete.shader = BRUTAL
	concrete.set_shader_parameter("albedo", load(CONCRETE % "diffuse"))
	concrete.set_shader_parameter("normals", load(CONCRETE % "nor_gl"))
	concrete.set_shader_parameter("rough", load(CONCRETE % "rough"))
	concrete.set_shader_parameter("tile", CONCRETE_METRES)
	var light := StandardMaterial3D.new()
	light.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	light.vertex_color_use_as_albedo = true
	light.albedo_color = Color(1.6, 1.6, 1.6)
	var sky := StandardMaterial3D.new()
	sky.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	sky.vertex_color_use_as_albedo = true
	var wall := NeonBiome.STREET_HALF + Street.SIDEWALK
	_deck(kit, route, length, concrete, sky)
	for rib: Undercity.Rib in city.ribs:
		kit.at(rib.s)
		_rib(kit, route, rib, wall, concrete, light)
	for pylon: Undercity.Pylon in city.pylons:
		kit.at(pylon.s)
		_pylon(kit, route, pylon, wall, concrete, light)
	for duct: Undercity.Duct in city.ducts:
		_duct(kit, route, duct, length, concrete)
	var traffic := ShaderMaterial.new()
	traffic.shader = TRAFFIC
	for lane: Undercity.Lane in city.lanes:
		_lane(kit, route, lane, length, traffic)
	kit.commit(root)
	for shaft: Undercity.Shaft in city.shafts:
		root.add_child(_shaft(route, shaft))
	return root


## The underside of the deck either side of the slot, and far above the slot, the glow of
## the levels that still get daylight.
static func _deck(
	kit: CityMesh, route: Route, length: float, concrete: Material, sky: Material
) -> void:
	var up := Vector3.UP * Undercity.DECK_HEIGHT
	var tile := 1.0 / CONCRETE_METRES
	var dark := Color(0.55, 0.58, 0.64)
	var s := -Undercity.REACH
	while s < length + Undercity.REACH:
		kit.at(s)
		var next := s + STRETCH
		var here := route.at(s) + up
		var there := route.at(next) + up
		var across := route.right(s)
		var onward := route.right(next)
		var slot := Undercity.SLOT_HALF
		for side: float in [-1.0, 1.0]:
			var a := here + across * side * slot
			var b := here + across * side * DECK_WIDE
			var c := there + onward * side * DECK_WIDE
			var d := there + onward * side * slot
			kit.quad(concrete, a, b, c, d, -Vector3.UP, dark, tile)
			var rise := Vector3.UP * SLOT_DEEP
			kit.quad(concrete, a, d, d + rise, a + rise, across * -side, dark * 1.6, tile)
		_coffers(kit, here, across, route.tangent(s), slot, concrete, dark)
		var glow := Color(0.5, 0.72, 0.95)
		var top := Vector3.UP * SLOT_DEEP
		kit.quad(
			sky,
			here + top - across * slot,
			here + top + across * slot,
			there + top + onward * slot,
			there + top - onward * slot,
			-Vector3.UP,
			glow
		)
		s = next


## Downstand beams both ways under the deck, which catch what light gets up there.
static func _coffers(
	kit: CityMesh,
	centre: Vector3,
	across: Vector3,
	along: Vector3,
	slot: float,
	concrete: Material,
	tint: Color
) -> void:
	var basis := Basis(across, Vector3.UP, along)
	var drop := Vector3.UP * COFFER_DROP * 0.5
	var run := DECK_WIDE - slot
	for side: float in [-1.0, 1.0]:
		var outer := centre + across * side * (slot + run * 0.5) - drop
		var size := Vector3(run, COFFER_DROP, 0.5)
		var steps := roundi(STRETCH / COFFER)
		for step: int in steps:
			var place := Transform3D(basis, outer + along * (step + 0.5) * STRETCH / steps)
			kit.box(concrete, place, size, tint)
		var beam := slot + COFFER * 0.5
		while beam < 30.0:
			var line := centre + across * side * beam + along * STRETCH * 0.5 - drop
			var long_way := Vector3(0.5, COFFER_DROP, STRETCH)
			kit.box(concrete, Transform3D(basis, line), long_way, tint)
			beam += COFFER


static func _rib(
	kit: CityMesh,
	route: Route,
	rib: Undercity.Rib,
	wall: float,
	concrete: Material,
	light: Material
) -> void:
	var along := route.tangent(rib.s)
	var across := route.right(rib.s)
	var basis := Basis(across, Vector3.UP, along)
	var deck := route.at(rib.s) + Vector3.UP * Undercity.DECK_HEIGHT
	var middle := deck - Vector3.UP * rib.drop * 0.5
	kit.block(
		concrete,
		Transform3D(basis, middle),
		Vector3((wall + 3.0) * 2.0, rib.drop, rib.long),
		Color(0.6, 0.63, 0.7),
		0.12
	)
	# Haunches down to the walls, so the load has somewhere to go.
	for side: float in [-1.0, 1.0]:
		var foot := deck + across * side * wall - Vector3.UP * (rib.drop + 7.0)
		var root := deck + across * side * (wall - 7.0) - Vector3.UP * rib.drop
		kit.bar(concrete, foot, root, 0.8, Color(0.3, 0.32, 0.36))
	if rib.lit:
		var low := deck - Vector3.UP * (rib.drop + 0.04)
		var reach := across * (wall - 1.0)
		kit.bar(light, low - reach, low + reach, 0.09, STRIP)


static func _pylon(
	kit: CityMesh,
	route: Route,
	pylon: Undercity.Pylon,
	wall: float,
	concrete: Material,
	light: Material
) -> void:
	var along := route.tangent(pylon.s)
	var outward := route.right(pylon.s) * pylon.side
	var basis := Basis(along, Vector3.UP, outward)
	var ground := route.at(pylon.s)
	var tint := Color(0.42, 0.44, 0.48)
	var face := ground + outward * (wall - pylon.depth)
	var high := Undercity.DECK_HEIGHT
	kit.block(
		concrete,
		Transform3D(basis, face + outward * pylon.depth * 0.5 + Vector3.UP * high * 0.5),
		Vector3(pylon.width, high, pylon.depth),
		tint,
		0.14
	)
	# A heavy foot and collars every so often, which is what makes it read as enormous.
	kit.block(
		concrete,
		Transform3D(basis, face + outward * pylon.depth * 0.5 + Vector3.UP * 1.6),
		Vector3(pylon.width + 1.0, 3.2, pylon.depth + 0.9),
		tint * 0.8,
		0.14
	)
	# Pilasters up its face, for something for the light to catch.
	for at: float in [-0.32, 0.0, 0.32]:
		kit.block(
			concrete,
			Transform3D(basis, face + along * pylon.width * at + Vector3.UP * (high * 0.5 + 1.6)),
			Vector3(pylon.width * 0.14, high - 3.2, 0.4),
			tint * 1.25,
			0.06
		)
	var collar := 9.0
	while collar < high:
		kit.block(
			concrete,
			Transform3D(basis, face + outward * pylon.depth * 0.5 + Vector3.UP * collar),
			Vector3(pylon.width + 0.5, 0.7, pylon.depth + 0.5),
			tint * 0.7,
			0.08
		)
		collar += 9.0
	var front := face - outward * 0.28 + along * pylon.width * 0.16
	kit.bar(light, front + Vector3.UP * 3.6, front + Vector3.UP * 18.0, 0.08, STRIP)
	for lamp: float in [0.45, 0.8]:
		var at := face - outward * 0.1 + along * pylon.width * 0.42 + Vector3.UP * high * lamp
		kit.bar(light, at, at + Vector3.UP * 0.3, 0.14, HAZARD)


## A long run of duct, following the road, under the deck or along a wall.
static func _duct(
	kit: CityMesh, route: Route, duct: Undercity.Duct, length: float, concrete: Material
) -> void:
	var s := -Undercity.REACH
	var tint := Color(0.2, 0.2, 0.21)
	while s < length + Undercity.REACH:
		kit.at(s)
		var from := route.at(s) + route.right(s) * duct.offset + Vector3.UP * duct.height
		var next := s + STRETCH
		var to := route.at(next) + route.right(next) * duct.offset + Vector3.UP * duct.height
		kit.bar(concrete, from, to, duct.thick, tint * 2.0)
		s = next


## One lane as a cross of two long quads, so it reads from below and from the side.
static func _lane(
	kit: CityMesh, route: Route, lane: Undercity.Lane, length: float, traffic: Material
) -> void:
	var colour := HEADLIGHT if lane.direction < 0.0 else TAIL_LIGHT
	var extra := Vector2(lane.speed * lane.direction, lane.phase)
	var half := 0.28
	var s := -Undercity.REACH
	while s < length + Undercity.REACH:
		kit.at(s)
		var next := s + STRETCH
		var here := route.at(s) + route.right(s) * lane.offset + Vector3.UP * lane.height
		var there := route.at(next) + route.right(next) * lane.offset + Vector3.UP * lane.height
		for across: Vector3 in [route.right(s) * half, Vector3.UP * half]:
			var uvs: Array[Vector2] = [
				Vector2(s, 0.0), Vector2(s, 1.0), Vector2(next, 1.0), Vector2(next, 0.0)
			]
			var corners: Array[Vector3] = [
				here - across, here + across, there + across, there - across
			]
			kit.face(traffic, corners, uvs, Vector3.DOWN, colour, extra)
		s = next


## A beam falling through the slot and hanging in the wet air.
static func _shaft(route: Route, shaft: Undercity.Shaft) -> SpotLight3D:
	var beam := SpotLight3D.new()
	beam.light_color = SHAFT_COLOUR
	# Dim on what it lands on and strong in the air, so it's the beam itself that shows.
	beam.light_energy = 5.0 * shaft.power
	beam.light_volumetric_fog_energy = 7.0
	beam.light_specular = 0.4
	beam.spot_range = Undercity.DECK_HEIGHT + 20.0
	beam.spot_angle = 13.0
	beam.spot_attenuation = 0.6
	beam.shadow_enabled = true
	beam.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
	beam.position = (
		route.at(shaft.s)
		+ route.right(shaft.s) * shaft.offset
		+ Vector3.UP * (Undercity.DECK_HEIGHT + 4.0)
	)
	return beam
