class_name Furniture
extends RefCounted
## What a street accumulates: lamp posts, railings, road signs, bins, crates, cables.

const SIGN_SHADER: Shader = preload("res://game/street_sign.gdshader")
const SIGN_SYMBOLS: Array[int] = [0, 1, 2]
const CRATE := "res://assets/polyhaven/plastic_crate_02/plastic_crate_02_1k.gltf"
const UTILITY := "res://assets/polyhaven/utility_box_01/utility_box_01_1k.gltf"
const STOOL := "res://assets/polyhaven/plastic_monobloc_chair_01/plastic_monobloc_chair_01_1k.gltf"
const CARTON := "res://assets/polyhaven/cardboard_box_01/cardboard_box_01_1k.gltf"
const BAG := "res://assets/polyhaven/trashbag/trashbag_1k.gltf"
const BIN := "res://assets/polyhaven/metal_trash_can/metal_trash_can_1k.gltf"
const BIN_SCALE := 0.75
const COVER := "res://assets/polyhaven/water_manhole_cover/water_manhole_cover_1k.gltf"
## The scan's frame stands 4 cm proud of its origin, which read as a puck on the road.
const COVER_SUNK := 0.03
const LADDER := "res://assets/polyhaven/ladder_sectioned_01/ladder_sectioned_01_1k.gltf"
const LAMP_HEIGHT := 7.2
const LAMP_COLOUR := Color(0.7, 0.85, 1.0)
const SODIUM := Color(1.0, 0.62, 0.26)
const ARM := 1.9
const RAIL_HEIGHT := 1.05
const RAIL_PANEL := 2.0
const RAIL_FOOT := 0.2
const BALUSTER_EVERY := 0.14
const SIDES := 12


static func build(route: Route, street: Street, length: float, section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "Furniture"
	var kit := CityMesh.new()
	var used: Dictionary[String, Material] = Once.made("furniture", _materials)
	var steel := used["steel"]
	var rail := used["rail"]
	var red := used["red"]
	var white := used["white"]
	var boards: Array[ShaderMaterial] = Once.made("furniture_boards", _boards)
	for railing: Vector3 in street.railings:
		if section.takes(railing.x, railing.y):
			_railing(kit, route, railing, rail)
	var side := 1.0
	for s: float in street.lamps:
		if section.holds(s) and not street.forked(side, s):
			kit.at(s)
			_lamp(kit, route, s, side, steel, used["lamp_glow"])
			if s > -15.0 and s < length + 40.0:
				var works := street.districts.kind_at(s) == Districts.Kind.WORKS
				root.add_child(_lamp_light(route, s, side, SODIUM if works else LAMP_COLOUR))
		side = -side
	_cables(kit, route, street, length, used["cable"], section)
	var placed: Dictionary[String, Array] = {}
	for path: String in [CRATE, UTILITY, STOOL, CARTON, BAG, BIN, COVER, LADDER]:
		placed[path] = [] as Array[Transform3D]
	for prop: Street.Prop in street.props:
		if not section.holds(prop.s):
			continue
		kit.at(prop.s)
		var ground := (
			route.at(prop.s)
			+ route.right(prop.s) * prop.offset
			+ Vector3.UP * StreetGround.KERB_HEIGHT
		)
		var turned := Basis(Vector3.UP, prop.turn)
		match prop.kind:
			"crate":
				placed[CRATE].append(Transform3D(turned, ground))
				var stacked := turned.rotated(Vector3.UP, 0.2)
				placed[CRATE].append(Transform3D(stacked, ground + Vector3.UP * 0.24))
			"carton":
				placed[CARTON].append(Transform3D(turned, ground))
			"stool":
				placed[STOOL].append(Transform3D(turned, ground))
			"rubbish":
				placed[BAG].append(Transform3D(turned, ground))
				placed[BAG].append(
					Transform3D(turned.rotated(Vector3.UP, 2.0), ground + turned.x * 0.5)
				)
			"ladder":
				var wall := route.right(prop.s) * signf(prop.offset)
				var lean := Basis(route.tangent(prop.s), Vector3.UP, wall).rotated(
					route.tangent(prop.s), 0.14 * signf(prop.offset)
				)
				placed[LADDER].append(Transform3D(lean, ground))
			"post":
				_post(kit, route, prop, ground, steel, boards[int(prop.s) % boards.size()])
			"no_entry":
				_no_entry(kit, route, prop, ground, steel, red, white)
			"bin":
				placed[BIN].append(Transform3D(turned.scaled(Vector3.ONE * BIN_SCALE), ground))
			"utility":
				var square := Basis(route.tangent(prop.s), Vector3.UP, route.right(prop.s))
				placed[UTILITY].append(Transform3D(square, ground))
	for cover: Vector2 in street.covers:
		if not section.holds(cover.x):
			continue
		var at := route.at(cover.x) + route.right(cover.x) * cover.y + Vector3.DOWN * COVER_SUNK
		placed[COVER].append(Transform3D(Basis.IDENTITY, at))
	kit.commit(root)
	for path: String in placed:
		var list: Array[Transform3D] = []
		list.assign(placed[path])
		instances(root, path, list)
	return root


static func _materials() -> Dictionary[String, Material]:
	var rail := Facades.scan("rusty_painted_metal", 1.1, Color(1.15, 1.4, 1.45))
	# Paint, not bare metal, which would go black anywhere with nothing to reflect.
	rail.metallic = 0.0
	var lamp_glow := StandardMaterial3D.new()
	lamp_glow.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	lamp_glow.albedo_color = Color(6.0, 4.6, 2.9)
	return {
		"steel": _plain(Color(0.3, 0.31, 0.33), 0.55, 0.05),
		"rail": rail,
		"cable": _plain(Color(0.02, 0.02, 0.02), 0.7, 0.0),
		"red": _plain(Color(0.72, 0.05, 0.05), 0.45, 0.0),
		"white": _plain(Color(0.85, 0.85, 0.82), 0.45, 0.0),
		"lamp_glow": lamp_glow,
	}


static func _boards() -> Array[ShaderMaterial]:
	var boards: Array[ShaderMaterial] = []
	for symbol: int in SIGN_SYMBOLS:
		boards.append(_sign_face(symbol))
	return boards


## A run of the street's guard rail along the kerb, close-barred panels between bolted posts.
## Decades of parking have bent some of them, and some have gone altogether.
static func _railing(kit: CityMesh, route: Route, railing: Vector3, material: Material) -> void:
	var offset := (NeonBiome.STREET_HALF + StreetGround.KERB_WIDTH + 0.12) * railing.z
	var lift := Vector3.UP * StreetGround.KERB_HEIGHT
	var s := railing.x
	var index := 0
	while s < railing.y:
		var next := minf(s + RAIL_PANEL, railing.y)
		kit.at(s)
		var a := route.at(s) + route.right(s) * offset + lift
		var b := route.at(next) + route.right(next) * offset + lift
		_rail_post(kit, a, material)
		var dice := Rng.derive(roundi(railing.x * 10.0 + railing.z), [index])
		var roll := Rng.unit(dice, 0)
		if roll > 0.05:
			var dent := 0.0
			if roll < 0.16:
				dent = lerpf(0.08, 0.26, Rng.unit(dice, 1)) * railing.z
			_rail_panel(kit, a, b, route.right(s), dent, material)
		s = next
		index += 1
	var last := route.at(railing.y) + route.right(railing.y) * offset + lift
	_rail_post(kit, last, material)


static func _rail_post(kit: CityMesh, foot: Vector3, material: Material) -> void:
	kit.bar(material, foot, foot + Vector3.UP * (RAIL_HEIGHT + 0.03), 0.07)
	kit.bar(material, foot, foot + Vector3.UP * 0.012, 0.2)
	kit.bar(
		material,
		foot + Vector3.UP * (RAIL_HEIGHT + 0.03),
		foot + Vector3.UP * (RAIL_HEIGHT + 0.04),
		0.09
	)
	for corner: Vector3 in [
		Vector3(1, 0, 1), Vector3(1, 0, -1), Vector3(-1, 0, 1), Vector3(-1, 0, -1)
	]:
		var bolt := foot + corner * 0.07
		kit.bar(material, bolt, bolt + Vector3.UP * 0.03, 0.022)


## A panel of bars between two posts, pushed out of true by [param dent] metres at its middle.
static func _rail_panel(
	kit: CityMesh, a: Vector3, b: Vector3, across: Vector3, dent: float, material: Material
) -> void:
	var pieces := 1 if dent == 0.0 else 6
	for piece: int in pieces:
		var from := float(piece) / pieces
		var to := float(piece + 1) / pieces
		for rail: Array in [[RAIL_HEIGHT, 0.05], [RAIL_HEIGHT - 0.1, 0.022], [RAIL_FOOT, 0.04]]:
			var high: float = rail[0]
			var thick: float = rail[1]
			kit.bar(
				material,
				_rail_point(a, b, across, from, high, dent),
				_rail_point(a, b, across, to, high, dent),
				thick
			)
	var bars := maxi(roundi(a.distance_to(b) / BALUSTER_EVERY), 1)
	for bar: int in range(1, bars):
		var u := float(bar) / bars
		kit.bar(
			material,
			_rail_point(a, b, across, u, RAIL_FOOT, dent),
			_rail_point(a, b, across, u, RAIL_HEIGHT, dent),
			0.018
		)


static func _rail_point(
	a: Vector3, b: Vector3, across: Vector3, u: float, high: float, dent: float
) -> Vector3:
	var bulge := across * dent * 4.0 * u * (1.0 - u) * high / RAIL_HEIGHT
	return a.lerp(b, u) + bulge + Vector3.UP * high


static func _lamp(
	kit: CityMesh, route: Route, s: float, side: float, steel: Material, glow: Material
) -> void:
	var outward := route.right(s) * side
	var foot := (
		route.at(s)
		+ outward * (NeonBiome.STREET_HALF + StreetGround.KERB_WIDTH + 0.3)
		+ Vector3.UP * StreetGround.KERB_HEIGHT
	)
	kit.bar(steel, foot, foot + Vector3.UP * 3.0, 0.16)
	kit.bar(steel, foot + Vector3.UP * 3.0, foot + Vector3.UP * LAMP_HEIGHT, 0.11)
	var top := foot + Vector3.UP * LAMP_HEIGHT
	var tip := top - outward * ARM + Vector3.UP * 0.25
	kit.bar(steel, top, tip, 0.07)
	var basis := Basis(route.tangent(s), Vector3.UP, outward)
	kit.box(steel, Transform3D(basis, tip), Vector3(0.28, 0.1, 0.62))
	kit.box(glow, Transform3D(basis, tip - Vector3.UP * 0.06), Vector3(0.2, 0.02, 0.5))


static func _lamp_light(route: Route, s: float, side: float, colour: Color) -> OmniLight3D:
	var pool := OmniLight3D.new()
	pool.light_color = colour
	pool.light_energy = 1.1
	pool.omni_range = 14.0
	var outward := route.right(s) * side
	pool.position = (
		route.at(s)
		+ outward * (NeonBiome.STREET_HALF - ARM + 0.56)
		+ Vector3.UP * (LAMP_HEIGHT - 0.2)
	)
	return pool


## Sagging cables strung across the street between buildings, bundled in twos and threes.
static func _cables(
	kit: CityMesh,
	route: Route,
	street: Street,
	length: float,
	material: Material,
	section: StreetSection
) -> void:
	var wall := NeonBiome.STREET_HALF + Street.SIDEWALK
	var index := 0
	var s := -Street.REACH * 0.5
	while s < length + Street.REACH * 0.5:
		var dice := Rng.derive(index, [88])
		if not section.holds(s):
			s += lerpf(7.0, 24.0, Rng.unit(dice, 4))
			index += 1
			continue
		kit.at(s)
		# Only strung where there's a wall each side and room above the road.
		var kind := street.districts.kind_at(s)
		var walled := not street.forked(1.0, s) and not street.forked(-1.0, s)
		var strung := walled and kind in [Districts.Kind.MARKET, Districts.Kind.WORKS]
		var count := (1 + int(Rng.unit(dice, 0) * 3.0)) if strung else 0
		var high := lerpf(8.0, 13.0, Rng.unit(dice, 1))
		var skew := lerpf(-3.0, 3.0, Rng.unit(dice, 2))
		for c: int in count:
			var y := high + c * 0.14
			var sag := 0.7 + Rng.unit(dice, 3) * 0.4
			var previous := route.at(s) - route.right(s) * wall + Vector3.UP * y
			for step: int in range(1, 11):
				var u := float(step) / 10.0
				var at := s + skew * u
				var across := lerpf(-wall, wall, u)
				var droop := sag * (1.0 - pow(u * 2.0 - 1.0, 2.0))
				var point := route.at(at) + route.right(at) * across + Vector3.UP * (y - droop)
				kit.bar(material, previous, point, 0.022)
				previous = point
		s += lerpf(7.0, 24.0, Rng.unit(dice, 4))
		index += 1


## A post with a small blue road sign.
static func _post(
	kit: CityMesh,
	route: Route,
	prop: Street.Prop,
	ground: Vector3,
	steel: Material,
	board: Material
) -> void:
	kit.bar(steel, ground, ground + Vector3.UP * 3.0, 0.07)
	kit.bar(steel, ground, ground + Vector3.UP * 0.35, 0.11)
	var across := route.right(prop.s)
	var up := Vector3.UP
	var facing := -route.tangent(prop.s)
	var basis := Basis(across, up, -facing)
	var centre := ground + up * 2.7
	kit.box(steel, Transform3D(basis, centre + facing * 0.06), Vector3(0.62, 0.62, 0.02))
	for height: float in [-0.2, 0.2]:
		var band := centre + up * height
		kit.bar(steel, band, band + facing * 0.06, 0.05)
	var face := centre + facing * 0.072
	(
		kit
		. face(
			board,
			[
				face - across * 0.3 - up * 0.3,
				face + across * 0.3 - up * 0.3,
				face + across * 0.3 + up * 0.3,
				face - across * 0.3 + up * 0.3,
			],
			[Vector2(0.0, 1.0), Vector2(1.0, 1.0), Vector2(1.0, 0.0), Vector2(0.0, 0.0)],
			facing
		)
	)


static func _no_entry(
	kit: CityMesh,
	route: Route,
	prop: Street.Prop,
	ground: Vector3,
	steel: Material,
	red: Material,
	white: Material
) -> void:
	kit.bar(steel, ground, ground + Vector3.UP * 3.0, 0.07)
	var across := route.right(prop.s)
	var up := Vector3.UP
	var centre := ground + Vector3.UP * 2.65 - route.tangent(prop.s) * 0.05
	var facing := -route.tangent(prop.s)
	var previous := centre + across * 0.38
	for i: int in range(1, SIDES + 1):
		var angle := TAU * i / SIDES
		var point := centre + (across * cos(angle) + up * sin(angle)) * 0.38
		kit.face(
			red,
			[centre, previous, point, point],
			[Vector2.ZERO, Vector2.ZERO, Vector2.ZERO, Vector2.ZERO],
			facing
		)
		previous = point
	var bar_at := centre + facing * 0.004
	kit.quad(
		white,
		bar_at - across * 0.28 - up * 0.06,
		bar_at + across * 0.28 - up * 0.06,
		bar_at + across * 0.28 + up * 0.06,
		bar_at - across * 0.28 + up * 0.06,
		facing
	)


## Scenes with one or more meshes, each drawn at every transform, chunked for culling.
## Where [param node] sits in a scene that isn't in the tree, so has no global transforms.
static func _within(node: Node3D, top: Node) -> Transform3D:
	var total := Transform3D.IDENTITY
	var at: Node = node
	while at != top:
		var parent := at as Node3D
		if parent:
			total = parent.transform * total
		at = at.get_parent()
	return total


static func instances(
	root: Node3D, path: String, places: Array[Transform3D], tolerance := Kit.STREET_TOLERANCE
) -> void:
	if places.is_empty():
		return
	var scene: PackedScene = load(path)
	var node := scene.instantiate()
	for child: Node in node.find_children("*", "MeshInstance3D", true, false):
		var source := child as MeshInstance3D
		var mesh := Kit.lean(source.mesh, tolerance)
		var within := _within(source, node)
		var cells: Dictionary[int, Array] = {}
		for place: Transform3D in places:
			var cell := floori(place.origin.z / CityMesh.CHUNK)
			if cell not in cells:
				cells[cell] = []
			cells[cell].append(place)
		for cell: int in cells:
			var list: Array = cells[cell]
			var multimesh := MultiMesh.new()
			multimesh.transform_format = MultiMesh.TRANSFORM_3D
			multimesh.mesh = mesh
			multimesh.instance_count = list.size()
			for i: int in list.size():
				var place: Transform3D = list[i]
				multimesh.set_instance_transform(i, place * within)
			var instance := MultiMeshInstance3D.new()
			instance.multimesh = multimesh
			root.add_child(instance)
	node.free()


static func _sign_face(symbol: int) -> ShaderMaterial:
	var material := ShaderMaterial.new()
	material.shader = SIGN_SHADER
	material.set_shader_parameter("symbol", symbol)
	return material


static func _plain(colour: Color, roughness: float, metallic: float) -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_color = colour
	material.roughness = roughness
	material.metallic = metallic
	return material
