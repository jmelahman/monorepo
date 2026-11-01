class_name Landmarks
extends RefCounted
## Draws what [Skyline] decides: the far towers, the giant screens, the crossings and the
## neon on the near towers. Lit for looks: everything here is a shape made of light.

const SPIRE: Shader = preload("res://game/spire.gdshader")
const SCREEN: Shader = preload("res://game/screen.gdshader")
const STEEL := Color(0.05, 0.05, 0.06)
const SIGN_SHEET := "res://assets/baked/signs.webp"


static func build(route: Route, skyline: Skyline, section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "Landmarks"
	var kit := CityMesh.new()
	var used: Dictionary[String, Material] = Once.made("landmarks", _materials)
	var neon := used["neon"]
	var steel := used["steel"]
	var far := used["far"]
	var under := used["under"]
	var sheet := SignSheet.load_sheet()
	var picture: Texture2D = load(SIGN_SHEET)
	for spire: Skyline.Spire in skyline.spires:
		if section.holds(spire.s):
			kit.at(spire.s)
			_spire(kit, route, spire, far, neon)
	for trim: Skyline.Trim in skyline.trims:
		if section.holds(trim.tower.s):
			kit.at(trim.tower.s)
			_trim(kit, route, trim, neon)
	for screen: Skyline.Screen in skyline.screens:
		if section.holds(screen.s):
			kit.at(screen.s)
			_screen(kit, route, screen, sheet, picture, steel)
	for span: Skyline.Span in skyline.spans:
		if not section.holds(span.s):
			continue
		kit.at(span.s)
		if span.rail:
			_rail(kit, root, route, span, steel, under, neon)
		else:
			_bridge(kit, root, route, span, steel, under, neon)
	kit.commit(root)
	for beam: Skyline.Beam in skyline.beams:
		if section.holds(beam.s):
			root.add_child(_beam(route, beam))
	return root


static func _materials() -> Dictionary[String, Material]:
	var neon := StandardMaterial3D.new()
	neon.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	neon.vertex_color_use_as_albedo = true
	neon.albedo_color = Color(3.0, 3.0, 3.0)
	var steel := StandardMaterial3D.new()
	steel.albedo_color = STEEL
	steel.roughness = 0.55
	steel.metallic = 0.4
	var far := ShaderMaterial.new()
	far.shader = SPIRE
	var under := Facades.scan("rusty_metal_02", 1.0, Color(0.6, 0.58, 0.56))
	under.metallic = 0.0
	return {"neon": neon, "steel": steel, "far": far, "under": under}


static func _neon(hue: float) -> Color:
	return Color.from_hsv(hue, 0.85, 1.0)


static func _spire(
	kit: CityMesh, route: Route, spire: Skyline.Spire, far: Material, neon: Material
) -> void:
	var along := route.tangent(spire.s)
	var across := route.right(spire.s)
	var base := route.at(spire.s) + across * spire.offset
	var basis := Basis(across, Vector3.UP, along)
	var colour := _neon(spire.hue)
	var low := -8.0
	var width := spire.width
	var step := spire.height / spire.tiers
	for tier: int in spire.tiers:
		var high := low + step
		var middle := base + Vector3.UP * ((low + high) * 0.5)
		kit.box(far, Transform3D(basis, middle), Vector3(width, high - low, width), colour)
		low = high
		width *= 0.72
	var tip := base + Vector3.UP * low
	kit.bar(neon, tip, tip + Vector3.UP * (spire.height * 0.14), 0.9, Color(1.0, 0.1, 0.08))


static func _trim(kit: CityMesh, route: Route, trim: Skyline.Trim, neon: Material) -> void:
	var tower := trim.tower
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var base := route.at(tower.s)
	var front := base + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK - 0.06)
	var colour := _neon(trim.hue)
	var low := Vector3.UP * Facades.GROUND_FLOOR
	var high := Vector3.UP * (tower.height / (tower.tiers + 1) + Facades.GROUND_FLOOR)
	var half := along * (tower.width * 0.5 - 0.1)
	kit.bar(neon, front - half + low, front - half + high, 0.1, colour)
	kit.bar(neon, front + half + low, front + half + high, 0.1, colour)
	kit.bar(neon, front - half + high, front + half + high, 0.1, colour)


static func _screen(
	kit: CityMesh,
	route: Route,
	screen: Skyline.Screen,
	sheet: SignSheet,
	picture: Texture2D,
	steel: Material
) -> void:
	var along := route.tangent(screen.s)
	var outward := route.right(screen.s) * screen.side
	var wall_off := NeonBiome.STREET_HALF + Street.SIDEWALK
	var ground := route.at(screen.s)
	var pick := sheet.pick(screen.trade, screen.banner, screen.pick)
	var rect: Rect2 = pick["uv"]
	var material := ShaderMaterial.new()
	material.shader = SCREEN
	material.set_shader_parameter("sheet", picture)
	material.set_shader_parameter(
		"picture", Vector4(rect.position.x, rect.position.y, rect.size.x, rect.size.y)
	)
	material.set_shader_parameter("colour_a", _neon(screen.hue))
	material.set_shader_parameter("colour_b", _neon(screen.hue_b))
	material.set_shader_parameter("size", screen.size)
	material.set_shader_parameter("phase", screen.phase)
	var up := Vector3.UP * screen.size.y
	var uvs: Array[Vector2] = [
		Vector2(0.0, 1.0), Vector2(1.0, 1.0), Vector2(1.0, 0.0), Vector2(0.0, 0.0)
	]
	var mirrored: Array[Vector2] = [uvs[1], uvs[0], uvs[3], uvs[2]]
	var lift := Vector3.UP * screen.elevation
	if screen.banner:
		var across := outward * screen.size.x
		var low := ground + outward * (wall_off - screen.size.x - 0.4) + lift
		kit.face(
			material,
			[low, low + across, low + across + up, low + up],
			uvs if screen.side > 0.0 else mirrored,
			-along,
			Color.WHITE
		)
		var middle := low + across * 0.5 + up * 0.5 + along * 0.2
		var basis := Basis(outward, Vector3.UP, along)
		var box := Vector3(screen.size.x + 0.3, screen.size.y + 0.3, 0.36)
		kit.box(steel, Transform3D(basis, middle), box)
		for at: float in [0.1, 0.5, 0.9]:
			var bar := middle + Vector3.UP * (screen.size.y * (at - 0.5))
			kit.bar(steel, bar, bar + outward * (screen.size.x * 0.5 + 0.5), 0.14)
		return
	var half := along * screen.size.x * 0.5
	var at := ground + outward * (wall_off - 0.1) + lift
	kit.face(
		material,
		[at - half, at + half, at + half + up, at - half + up],
		mirrored if screen.side > 0.0 else uvs,
		-outward,
		Color.WHITE
	)


static func _bridge(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	span: Skyline.Span,
	steel: Material,
	under: Material,
	neon: Material
) -> void:
	var along := route.tangent(span.s)
	var across := route.right(span.s)
	var basis := Basis(across, Vector3.UP, along)
	var middle := route.at(span.s) + Vector3.UP * span.height
	var run := (NeonBiome.STREET_HALF + Street.SIDEWALK) * 2.0 + 1.0
	var depth := Skyline.BRIDGE_DEPTH
	kit.box(steel, Transform3D(basis, middle), Vector3(run, 3.0, depth))
	var colour := _neon(span.hue)
	var half := across * run * 0.5
	var face := along * (depth * 0.5 + 0.03)
	# A lit gallery along each side, and a line of light under the deck.
	var gallery := ShaderMaterial.new()
	gallery.shader = Facades.WINDOW
	var dim := Color(0.55, 0.75, 0.85, 0.3)
	for side: float in [-1.0, 1.0]:
		kit.box(
			gallery,
			Transform3D(basis, middle + face * side + Vector3.UP * 0.25),
			Vector3(run - 0.8, 1.1, 0.04),
			dim
		)
		# Posts in front, so it's a row of dim windows and not a bar of light.
		var post := -run * 0.5
		while post < run * 0.5:
			var at := middle + face * side * 1.03 + across * post + Vector3.UP * 0.25
			kit.box(steel, Transform3D(basis, at), Vector3(0.3, 1.3, 0.08))
			post += 2.2
		# Tucked up behind the girders, so it can't peek out from under the far side.
		var edge := middle + face * side + Vector3.UP * -1.2
		kit.bar(neon, edge - half, edge + half, 0.07, colour)
	_underside(kit, route, span, middle + Vector3.UP * -1.5, run, depth, under)
	var glow := OmniLight3D.new()
	glow.light_color = colour
	glow.light_energy = 1.5
	glow.light_volumetric_fog_energy = 1.0
	glow.omni_range = 16.0
	glow.position = middle - Vector3.UP * 2.0
	root.add_child(glow)


## What a crossing is really held up by: girders the length of it, ribs across them, and the
## services slung underneath. [param soffit] is the middle of the box's flat bottom.
static func _underside(
	kit: CityMesh,
	route: Route,
	span: Skyline.Span,
	soffit: Vector3,
	run: float,
	depth: float,
	under: Material
) -> void:
	var along := route.tangent(span.s)
	var across := route.right(span.s)
	var basis := Basis(across, Vector3.UP, along)
	var dice := Rng.derive(int(span.phase * 100000.0), [91])
	# The lining plate, so the soffit isn't left as a bare flat colour.
	kit.box(
		under,
		Transform3D(basis, soffit - Vector3.UP * 0.02),
		Vector3(run, 0.04, depth),
		Color(0.8, 0.8, 0.8)
	)
	for side: float in [-1.0, 1.0]:
		var at := soffit + along * (depth * 0.5 - 0.22) * side - Vector3.UP * 0.32
		kit.box(under, Transform3D(basis, at), Vector3(run, 0.56, 0.3))
		kit.box(under, Transform3D(basis, at - Vector3.UP * 0.3), Vector3(run, 0.05, 0.56))
	var rib := -run * 0.5 + 0.6
	while rib < run * 0.5:
		var at := soffit + across * rib - Vector3.UP * 0.14
		kit.box(under, Transform3D(basis, at), Vector3(0.16, 0.26, depth - 0.5))
		rib += 1.6
	# Pipes and a cable tray, down the side that isn't lit the most.
	for pipe: int in 3:
		var thick := lerpf(0.06, 0.15, Rng.unit(dice, pipe))
		var off := along * lerpf(-0.45, 0.45, (pipe + 0.5) / 3.0) * depth * 0.6
		var low := soffit + off - Vector3.UP * (0.34 + thick)
		var half := across * run * 0.5
		kit.bar(under, low - half, low + half, thick, Color(0.5, 0.5, 0.5))
	var hanger := -run * 0.5 + 1.0
	while hanger < run * 0.5:
		var at := soffit + across * hanger
		kit.bar(under, at - along * depth * 0.3, at - along * depth * 0.3 - Vector3.UP * 0.5, 0.03)
		kit.bar(under, at + along * depth * 0.3, at + along * depth * 0.3 - Vector3.UP * 0.5, 0.03)
		hanger += 3.1


static func _rail(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	span: Skyline.Span,
	steel: Material,
	under: Material,
	neon: Material
) -> void:
	var along := route.tangent(span.s)
	var across := route.right(span.s)
	var basis := Basis(across, Vector3.UP, along)
	var middle := route.at(span.s) + Vector3.UP * span.height
	var run := Train.HALF_RUN * 2.0
	var depth := Skyline.RAIL_DEPTH
	kit.box(steel, Transform3D(basis, middle), Vector3(run, 1.8, depth))
	_underside(kit, route, span, middle + Vector3.UP * -0.9, run, depth, under)
	var colour := _neon(span.hue)
	var half := across * Train.HALF_RUN
	for side: float in [-1.0, 1.0]:
		var edge := middle + along * (depth * 0.5 + 0.04) * side - Vector3.UP * 0.85
		kit.bar(neon, edge - half, edge + half, 0.14, colour)
		# A parapet the train shows over.
		var wall := middle + along * (depth * 0.5 - 0.1) * side + Vector3.UP * 1.3
		kit.box(steel, Transform3D(basis, wall), Vector3(run, 0.9, 0.2))
	var row := -3.0
	while row <= 3.0:
		var lamp := middle + along * row - Vector3.UP * 0.92
		kit.bar(neon, lamp - across * 7.0, lamp + across * 7.0, 0.07, Color(0.75, 0.85, 1.0) * 0.25)
		row += 1.5
	var glow := OmniLight3D.new()
	glow.light_color = Color(0.7, 0.8, 1.0).lerp(colour, 0.4)
	glow.light_energy = 3.0
	glow.light_volumetric_fog_energy = 1.2
	glow.omni_range = 26.0
	glow.position = middle - Vector3.UP * 3.0
	root.add_child(glow)
	root.add_child(_train(middle + Vector3.UP * 2.3, across, along, span.phase))


static func _train(centre: Vector3, across: Vector3, along: Vector3, phase: float) -> Train:
	var train := Train.new()
	train.centre = centre
	train.across = across
	train.phase = phase
	var body := BoxMesh.new()
	body.size = Vector3(90.0, 3.0, 3.2)
	var hull := StandardMaterial3D.new()
	hull.albedo_color = Color(0.07, 0.07, 0.09)
	hull.metallic = 0.7
	hull.roughness = 0.3
	body.material = hull
	var windows := BoxMesh.new()
	windows.size = Vector3(88.0, 0.9, 3.3)
	var lit := StandardMaterial3D.new()
	lit.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	lit.albedo_color = Color(4.0, 3.6, 2.8)
	windows.material = lit
	var basis := Basis(across, Vector3.UP, along)
	for mesh: BoxMesh in [body, windows]:
		var part := MeshInstance3D.new()
		part.mesh = mesh
		part.basis = basis
		part.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		train.add_child(part)
	return train


static func _beam(route: Route, beam: Skyline.Beam) -> SweepingBeam:
	var light := SweepingBeam.new()
	light.position = (
		route.at(beam.s)
		+ route.right(beam.s) * beam.side * (NeonBiome.STREET_HALF + Street.SIDEWALK + 2.0)
		+ Vector3.UP * beam.height
	)
	light.across = route.right(beam.s)
	light.along = route.tangent(beam.s)
	light.phase = beam.phase
	light.light_color = Color.from_hsv(beam.hue, 0.45, 1.0)
	light.light_energy = 40.0
	light.light_volumetric_fog_energy = 5.0
	light.light_specular = 0.0
	light.spot_range = 260.0
	light.spot_angle = 3.5
	light.spot_attenuation = 0.4
	light.set_ride_time(0.0)
	return light
