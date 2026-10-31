class_name Furniture
extends RefCounted
## What a street accumulates: lamp posts, railings, road signs, bins, crates, cables.

const CRATE := "res://assets/polyhaven/plastic_crate_02/plastic_crate_02_1k.gltf"
const UTILITY := "res://assets/polyhaven/utility_box_01/utility_box_01_1k.gltf"
const STOOL := "res://assets/polyhaven/plastic_monobloc_chair_01/plastic_monobloc_chair_01_1k.gltf"
const CARTON := "res://assets/polyhaven/cardboard_box_01/cardboard_box_01_1k.gltf"
const BAG := "res://assets/polyhaven/trashbag/trashbag_1k.gltf"
const BIN := "res://assets/polyhaven/metal_trash_can/metal_trash_can_1k.gltf"
const BIN_SCALE := 0.75
const COVER := "res://assets/polyhaven/water_manhole_cover/water_manhole_cover_1k.gltf"
const LADDER := "res://assets/polyhaven/ladder_sectioned_01/ladder_sectioned_01_1k.gltf"
const LAMP_HEIGHT := 7.2
const ARM := 1.9
const RAIL_HEIGHT := 1.05
const SIDES := 12


static func build(route: Route, street: Street, length: float) -> Node3D:
	var root := Node3D.new()
	root.name = "Furniture"
	var kit := CityMesh.new()
	var steel := _plain(Color(0.2, 0.2, 0.21), 0.5, 0.6)
	var rail := _plain(Color(0.58, 0.58, 0.55), 0.5, 0.3)
	var cable := _plain(Color(0.02, 0.02, 0.02), 0.7, 0.0)
	var red := _plain(Color(0.72, 0.05, 0.05), 0.45, 0.0)
	var white := _plain(Color(0.85, 0.85, 0.82), 0.45, 0.0)
	var board := _plain(Color(0.14, 0.26, 0.5), 0.45, 0.0)
	var lamp_glow := StandardMaterial3D.new()
	lamp_glow.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	lamp_glow.albedo_color = Color(6.0, 4.6, 2.9)
	for railing: Vector3 in street.railings:
		_railing(kit, route, railing, rail)
	var side := 1.0
	for s: float in street.lamps:
		kit.at(s)
		_lamp(kit, route, s, side, steel, lamp_glow)
		if s > -15.0 and s < length + 40.0:
			root.add_child(_lamp_light(route, s, side))
		side = -side
	_cables(kit, route, length, cable)
	var placed: Dictionary[String, Array] = {}
	for path: String in [CRATE, UTILITY, STOOL, CARTON, BAG, BIN, COVER, LADDER]:
		placed[path] = [] as Array[Transform3D]
	for prop: Street.Prop in street.props:
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
				_post(kit, route, prop, ground, steel, board)
			"no_entry":
				_no_entry(kit, route, prop, ground, steel, red, white)
			"bin":
				placed[BIN].append(Transform3D(turned.scaled(Vector3.ONE * BIN_SCALE), ground))
			"utility":
				var square := Basis(route.tangent(prop.s), Vector3.UP, route.right(prop.s))
				placed[UTILITY].append(Transform3D(square, ground))
	for cover: Vector2 in street.covers:
		var at := route.at(cover.x) + route.right(cover.x) * cover.y + Vector3.UP * 0.01
		placed[COVER].append(Transform3D(Basis.IDENTITY, at))
	kit.commit(root)
	for path: String in placed:
		var list: Array[Transform3D] = []
		list.assign(placed[path])
		instances(root, path, list)
	return root


## A run of painted guard rail along the kerb: posts, a top rail, a mid rail, and a
## kick plate low down.
static func _railing(kit: CityMesh, route: Route, railing: Vector3, material: Material) -> void:
	var offset := (NeonBiome.STREET_HALF + StreetGround.KERB_WIDTH + 0.12) * railing.z
	var lift := Vector3.UP * StreetGround.KERB_HEIGHT
	var s := railing.x
	while s < railing.y:
		var next := minf(s + 2.0, railing.y)
		kit.at(s)
		var a := route.at(s) + route.right(s) * offset + lift
		var b := route.at(next) + route.right(next) * offset + lift
		kit.bar(material, a, a + Vector3.UP * RAIL_HEIGHT, 0.05)
		kit.bar(material, a + Vector3.UP * RAIL_HEIGHT, b + Vector3.UP * RAIL_HEIGHT, 0.04)
		kit.bar(material, a + Vector3.UP * 0.62, b + Vector3.UP * 0.62, 0.03)
		kit.bar(material, a + Vector3.UP * 0.22, b + Vector3.UP * 0.22, 0.03)
		s = next


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


static func _lamp_light(route: Route, s: float, side: float) -> OmniLight3D:
	var pool := OmniLight3D.new()
	pool.light_color = Color(0.7, 0.85, 1.0)
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
static func _cables(kit: CityMesh, route: Route, length: float, material: Material) -> void:
	var wall := NeonBiome.STREET_HALF + Street.SIDEWALK
	var index := 0
	var s := -Street.REACH * 0.5
	while s < length + Street.REACH * 0.5:
		var dice := Rng.derive(index, [88])
		kit.at(s)
		var count := 1 + int(Rng.unit(dice, 0) * 3.0)
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
	var basis := Basis(route.right(prop.s), Vector3.UP, route.tangent(prop.s))
	kit.box(board, Transform3D(basis, ground + Vector3.UP * 2.7), Vector3(0.6, 0.6, 0.02))


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


static func instances(root: Node3D, path: String, places: Array[Transform3D]) -> void:
	if places.is_empty():
		return
	var scene: PackedScene = load(path)
	var node := scene.instantiate()
	for child: Node in node.find_children("*", "MeshInstance3D", true, false):
		var source := child as MeshInstance3D
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
			multimesh.mesh = source.mesh
			multimesh.instance_count = list.size()
			for i: int in list.size():
				var place: Transform3D = list[i]
				multimesh.set_instance_transform(i, place * within)
			var instance := MultiMeshInstance3D.new()
			instance.multimesh = multimesh
			root.add_child(instance)
	node.free()


static func _plain(colour: Color, roughness: float, metallic: float) -> StandardMaterial3D:
	var material := StandardMaterial3D.new()
	material.albedo_color = colour
	material.roughness = roughness
	material.metallic = metallic
	return material
