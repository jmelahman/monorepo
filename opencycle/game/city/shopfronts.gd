class_name Shopfronts
extends RefCounted
## The ground floor: real rooms behind open fronts, lit from their ceilings and full of
## goods from the floor up. What's in a shop comes from its trade. Each shop's light
## also falls on the pavement, which is most of the light in the street.

const GOODS: Shader = preload("res://game/goods.gdshader")
const SHUTTER := "res://assets/polyhaven/rollershutter_door/rollershutter_door_1k.gltf"
const OPENING := 3.0
const CEILING := 3.0
const SHELF_DEPTH := 0.42
const SHELF_RISE := 0.38
## Past this no box on a shelf is more than a few pixels, and they are most of the triangles.
const GOODS_RANGE := 80.0
## What each trade's goods look like: its colours, how big, and its light.
const TRADES: Dictionary[String, Dictionary] = {
	"pharmacy":
	{
		"goods":
		[Color("f4f4f0"), Color("2f7fd1"), Color("e8413a"), Color("33b36b"), Color("f7c52b")],
		"size": Vector3(0.14, 0.2, 0.1),
		"light": Color(0.92, 0.97, 1.0),
	},
	"electronics":
	{
		"goods": [Color("20252c"), Color("c6ccd4"), Color("3a3f48"), Color("1677c9")],
		"size": Vector3(0.2, 0.14, 0.1),
		"light": Color(0.85, 0.93, 1.0),
	},
	"robotics":
	{
		"goods": [Color("3d4148"), Color("7d858f"), Color("c9a227"), Color("24282e")],
		"size": Vector3(0.3, 0.24, 0.24),
		"light": Color(0.75, 0.9, 1.0),
	},
	"market":
	{
		"goods":
		[
			Color("e8413a"),
			Color("f7c52b"),
			Color("2f9e6e"),
			Color("f08a24"),
			Color("3f6fd8"),
			Color("e7e2d2"),
		],
		"size": Vector3(0.17, 0.26, 0.12),
		"light": Color(1.0, 0.96, 0.86),
	},
	"gold":
	{
		"goods": [Color("d7a928"), Color("8c1f24"), Color("f3df98")],
		"size": Vector3(0.22, 0.1, 0.16),
		"light": Color(1.0, 0.85, 0.62),
	},
	"exchange":
	{
		"goods": [Color("23442f"), Color("c5a64a"), Color("d8d4c4")],
		"size": Vector3(0.3, 0.2, 0.1),
		"light": Color(1.0, 0.93, 0.8),
	},
	"noodle":
	{
		"goods": [Color("c9b28a"), Color("9e3a22"), Color("e5ddc8"), Color("5a3a22")],
		"size": Vector3(0.22, 0.18, 0.22),
		"light": Color(1.0, 0.8, 0.55),
	},
	"cafe":
	{
		"goods": [Color("e5ddc8"), Color("7a4a26"), Color("c9352b"), Color("d9b35a")],
		"size": Vector3(0.14, 0.2, 0.14),
		"light": Color(1.0, 0.87, 0.68),
	},
}


static func build(route: Route, street: Street, length: float) -> Node3D:
	var root := Node3D.new()
	root.name = "Shopfronts"
	var kit := CityMesh.new()
	var materials := _materials()
	var goods: Dictionary[int, Array] = {}
	for index: int in street.shops.size():
		var shop := street.shops[index]
		kit.at(shop.s)
		_shell(kit, route, shop, materials)
		if shop.open:
			_stock(route, shop, goods, index)
			if shop.s > -12.0 and shop.s < length + 35.0:
				root.add_child(_light(route, shop))
	kit.commit(root)
	_add_goods(root, goods)
	_add_shutters(root, route, street)
	return root


static func _materials() -> Dictionary[String, Material]:
	var fitted: Dictionary[String, Material] = {}
	var ceiling := StandardMaterial3D.new()
	ceiling.albedo_color = Color(0.5, 0.5, 0.48)
	ceiling.roughness = 0.8
	fitted["ceiling"] = ceiling
	var wall := StandardMaterial3D.new()
	wall.albedo_color = Color(0.52, 0.5, 0.47)
	wall.roughness = 0.65
	wall.vertex_color_use_as_albedo = true
	fitted["wall"] = wall
	var floor_finish := StandardMaterial3D.new()
	floor_finish.albedo_color = Color(0.5, 0.48, 0.45)
	floor_finish.roughness = 0.3
	fitted["floor"] = floor_finish
	var tube := StandardMaterial3D.new()
	tube.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	tube.vertex_color_use_as_albedo = true
	tube.albedo_color = Color(2.2, 2.2, 2.2)
	fitted["tube"] = tube
	var shelf := StandardMaterial3D.new()
	shelf.albedo_color = Color(0.58, 0.58, 0.56)
	shelf.roughness = 0.5
	fitted["shelf"] = shelf
	var fascia := StandardMaterial3D.new()
	fascia.albedo_color = Color(0.3, 0.3, 0.32)
	fascia.vertex_color_use_as_albedo = true
	fascia.roughness = 0.55
	fitted["fascia"] = fascia
	return fitted


## A room, with no front wall: floor, ceiling, walls, tubes, shelf boards and the band of
## wall above the opening that carries the shop's name.
static func _shell(
	kit: CityMesh, route: Route, shop: Street.Shop, materials: Dictionary[String, Material]
) -> void:
	var along := route.tangent(shop.s)
	var outward := route.right(shop.s) * shop.side
	var ground := route.at(shop.s)
	var wall_off := NeonBiome.STREET_HALF + Street.SIDEWALK
	var mouth := ground + outward * wall_off + Vector3.UP * StreetGround.KERB_HEIGHT
	var half := along * shop.width * 0.5
	var back := outward * shop.depth
	var up := Vector3.UP * (Facades.GROUND_FLOOR - StreetGround.KERB_HEIGHT)
	# The band above the door and the lintel over everything up to the first storey.
	var band_low := Vector3.UP * (OPENING - StreetGround.KERB_HEIGHT)
	var fascia_tint := Color.from_hsv(
		shop.pick, 0.25, lerpf(0.5, 1.0, shop.pick * 3.0 - floorf(shop.pick * 3.0))
	)
	kit.quad(
		materials["fascia"],
		mouth - half + band_low,
		mouth + half + band_low,
		mouth + half + up,
		mouth - half + up,
		-outward,
		fascia_tint
	)
	# A party wall between each pair of shops, as a pillar on the street.
	kit.box(
		materials["fascia"],
		Transform3D(
			Basis(along, Vector3.UP, outward), mouth + half + band_low * 0.5 + outward * 0.05
		),
		Vector3(0.3, OPENING, 0.5),
		Color(0.32, 0.31, 0.3)
	)
	if not shop.open:
		return
	var inside := band_low
	var wall_tint := Color(1.0, 1.0, 1.0)
	kit.quad(
		materials["floor"],
		mouth - half,
		mouth + half,
		mouth + half + back,
		mouth - half + back,
		Vector3.UP
	)
	kit.quad(
		materials["ceiling"],
		mouth - half + inside,
		mouth + half + inside,
		mouth + half + inside + back,
		mouth - half + inside + back,
		-Vector3.UP
	)
	kit.quad(
		materials["wall"],
		mouth - half + back,
		mouth + half + back,
		mouth + half + back + inside,
		mouth - half + back + inside,
		-outward,
		wall_tint
	)
	kit.quad(
		materials["wall"],
		mouth - half + back,
		mouth - half,
		mouth - half + inside,
		mouth - half + back + inside,
		along,
		wall_tint
	)
	kit.quad(
		materials["wall"],
		mouth + half,
		mouth + half + back,
		mouth + half + back + inside,
		mouth + half + inside,
		-along,
		wall_tint
	)
	# A trestle at the door for the goods out front.
	var table := mouth - outward * 0.2 + Vector3.UP * 0.4
	kit.box(
		materials["shelf"],
		Transform3D(Basis(along, Vector3.UP, outward), table),
		Vector3(shop.width * 0.74, 0.8, 0.62)
	)
	# Fluorescent tubes in rows, running back from the street.
	var light: Color = TRADES[shop.trade]["light"]
	var rows := maxi(int(shop.width / 1.5), 1)
	for r: int in rows:
		var offset := along * ((r + 0.5) / rows - 0.5) * shop.width
		var from := mouth + offset + inside - Vector3.UP * 0.02 + outward * 0.3
		var to := from + outward * (shop.depth - 0.6)
		kit.bar(materials["tube"], from, to, 0.05, light)
	# Shelf boards on the back wall and both sides.
	var boards := int(CEILING / SHELF_RISE) - 1
	for b: int in boards:
		var y := Vector3.UP * (b + 1) * SHELF_RISE
		var rear := back - outward * SHELF_DEPTH * 0.5
		kit.box(
			materials["shelf"],
			Transform3D(Basis(along, Vector3.UP, outward), mouth + rear + y),
			Vector3(shop.width - 0.05, 0.025, SHELF_DEPTH)
		)
		for end: float in [-1.0, 1.0]:
			var run := mouth + half * end - along * end * SHELF_DEPTH * 0.5 + back * 0.5 + y
			kit.box(
				materials["shelf"],
				Transform3D(Basis(along, Vector3.UP, outward), run),
				Vector3(SHELF_DEPTH, 0.025, shop.depth - 0.1)
			)


## Fills the shelves: a row of goods along each board, a little uneven and with gaps.
static func _stock(
	route: Route, shop: Street.Shop, goods: Dictionary[int, Array], index: int
) -> void:
	var along := route.tangent(shop.s)
	var outward := route.right(shop.s) * shop.side
	var mouth := (
		route.at(shop.s)
		+ outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
		+ Vector3.UP * StreetGround.KERB_HEIGHT
	)
	var trade: Dictionary = TRADES[shop.trade]
	var palette: Array = trade["goods"]
	var size: Vector3 = trade["size"]
	var chunk := floori(shop.s / CityMesh.CHUNK)
	if chunk not in goods:
		goods[chunk] = []
	var list: Array = goods[chunk]
	var dice := Rng.derive(index, [77])
	var boards := int(CEILING / SHELF_RISE) - 1
	for b: int in boards:
		var y := Vector3.UP * ((b + 1) * SHELF_RISE + 0.0125)
		_row(
			list,
			mouth + outward * (shop.depth - SHELF_DEPTH * 0.5) + y,
			along,
			shop.width - 0.1,
			size,
			palette,
			Rng.derive(dice, [b, 0]),
			outward
		)
		for end: float in [-1.0, 1.0]:
			var run := (
				mouth
				+ along * end * (shop.width * 0.5 - SHELF_DEPTH * 0.5)
				+ outward * (shop.depth * 0.5)
				+ y
			)
			_row(
				list,
				run,
				outward,
				shop.depth - 0.2,
				size,
				palette,
				Rng.derive(dice, [b, int(end) + 2]),
				along * -end
			)
	# A display at the door: a low table or rack, heaped, reaching towards the pavement.
	var table := mouth - outward * 0.2 + Vector3.UP * 0.82
	_row(list, table, along, shop.width * 0.7, size, palette, Rng.derive(dice, [99, 1]), -outward)
	_row(
		list,
		table - outward * 0.24,
		along,
		shop.width * 0.7,
		size,
		palette,
		Rng.derive(dice, [99, 2]),
		-outward
	)
	_row(
		list,
		table + Vector3.UP * size.y,
		along,
		shop.width * 0.6,
		size,
		palette,
		Rng.derive(dice, [99, 3]),
		-outward
	)


static func _row(
	list: Array,
	centre: Vector3,
	run: Vector3,
	span: float,
	size: Vector3,
	palette: Array,
	dice: int,
	facing: Vector3
) -> void:
	var at := -span * 0.5
	var i := 0
	var basis := Basis(run, Vector3.UP, -facing)
	while at < span * 0.5:
		var scale := lerpf(0.75, 1.15, Rng.unit(dice, i * 3))
		var wide := size.x * scale
		if Rng.unit(dice, i * 3 + 1) > 0.08:
			var tall := size.y * lerpf(0.8, 1.15, Rng.unit(dice, i * 3 + 2))
			var shaped := basis.scaled_local(Vector3(wide, tall, size.z))
			var place := centre + run * (at + wide * 0.5) + Vector3.UP * tall * 0.5
			var colour: Color = palette[int(Rng.unit(dice, i * 3 + 7) * palette.size())]
			list.append([Transform3D(shaped, place), colour])
			# Stacked as high as fits under the next board, the way a shop stocks a shelf.
			var layers := int((SHELF_RISE - 0.04) / tall)
			for layer: int in range(1, layers):
				var above := place + Vector3.UP * tall * layer
				list.append([Transform3D(shaped, above), colour.darkened(layer * 0.04)])
		at += wide + 0.012
		i += 1


static func _add_goods(root: Node3D, goods: Dictionary[int, Array]) -> void:
	var mesh := BoxMesh.new()
	var material := ShaderMaterial.new()
	material.shader = GOODS
	mesh.material = material
	for chunk: int in goods:
		var list: Array = goods[chunk]
		var multimesh := MultiMesh.new()
		multimesh.transform_format = MultiMesh.TRANSFORM_3D
		multimesh.use_colors = true
		multimesh.mesh = mesh
		multimesh.instance_count = list.size()
		for i: int in list.size():
			var entry: Array = list[i]
			var place: Transform3D = entry[0]
			var colour: Color = entry[1]
			multimesh.set_instance_transform(i, place)
			multimesh.set_instance_color(i, colour)
		var instance := MultiMeshInstance3D.new()
		instance.multimesh = multimesh
		instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
		instance.visibility_range_end = GOODS_RANGE
		root.add_child(instance)


static func _light(route: Route, shop: Street.Shop) -> OmniLight3D:
	var outward := route.right(shop.s) * shop.side
	var glow := OmniLight3D.new()
	glow.light_color = TRADES[shop.trade]["light"]
	glow.light_energy = 1.05 + shop.width * 0.14
	glow.omni_range = 9.0 + shop.width * 0.6
	glow.omni_attenuation = 1.1
	glow.position = (
		route.at(shop.s)
		+ outward * (NeonBiome.STREET_HALF + Street.SIDEWALK + 0.2)
		+ Vector3.UP * 2.7
	)
	return glow


static func _add_shutters(root: Node3D, route: Route, street: Street) -> void:
	var scene: PackedScene = load(SHUTTER)
	var node := scene.instantiate()
	var meshes := node.find_children("*", "MeshInstance3D", true, false)
	var source := (meshes[0] as MeshInstance3D).mesh
	node.free()
	var bounds := source.get_aabb()
	var placed: Array[Transform3D] = []
	for shop: Street.Shop in street.shops:
		if shop.open:
			continue
		var along := route.tangent(shop.s)
		var outward := route.right(shop.s) * shop.side
		var at := (
			route.at(shop.s)
			+ outward * (NeonBiome.STREET_HALF + Street.SIDEWALK + 0.02)
			+ Vector3.UP * StreetGround.KERB_HEIGHT
		)
		var basis := Basis(
			along * (shop.width / bounds.size.x),
			Vector3.UP * (OPENING / bounds.size.y),
			outward * -1.0
		)
		var lift := -Vector3.UP * bounds.position.y * basis.y.length()
		placed.append(Transform3D(basis, at + lift))
	var multimesh := MultiMesh.new()
	multimesh.transform_format = MultiMesh.TRANSFORM_3D
	multimesh.mesh = source
	multimesh.instance_count = placed.size()
	for i: int in placed.size():
		multimesh.set_instance_transform(i, placed[i])
	var instance := MultiMeshInstance3D.new()
	instance.multimesh = multimesh
	root.add_child(instance)
