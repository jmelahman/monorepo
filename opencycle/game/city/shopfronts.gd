class_name Shopfronts
extends RefCounted
## The ground floor: real rooms behind open fronts, each laid out and painted for its trade,
## with shelves of stock baked from real models by tools/bake_goods.gd. Each shop's light
## also falls on the pavement, which is most of the light in the street.

const GOODS: Shader = preload("res://game/goods.gdshader")
const CANVAS: Shader = preload("res://game/shop_surface.gdshader")
const SCREEN: Shader = preload("res://game/screen.gdshader")
const GOODS_SHEET := "res://assets/baked/goods.webp"
const SIGN_SHEET := "res://assets/baked/signs.webp"
const PROP := "res://assets/polyhaven/%s/%s_1k.gltf"
const OPENING := 3.0
const CEILING := 3.0
const SHELF_DEPTH := 0.42
const SHELF_RISE := 0.38
const SHELF_FOOT := 0.14
const SHELF_ROWS := 6
## Layout of the goods sheet, which has to match tools/bake_goods.gd.
const SHEET_COLUMNS := 2
const SHEET_ROWS := 6
const CARD_WIDTH := 1.2
## Metres of wall one repeat of the tile covers.
const TILE_METRES := 1.6
const PROP_REACH := 90.0
const AWNING_OUT := 1.7
const AWNING_DROP := 0.5
const AWNING_STRIPE := 0.45
const VALANCE := 0.2
## Height of the tile round a room, with old paint on plaster above it.
const DADO := 1.25
const PLASTER_METRES := 2.2
## What the paint fades towards, so a room isn't one clean colour from floor to ceiling.
const FADED := Color(0.6, 0.58, 0.52)
const GAP_CHANCE := 0.14
const PAINT_COATS: Array[Color] = [
	Color(0.55, 0.55, 0.52), Color(0.32, 0.42, 0.42), Color(0.5, 0.27, 0.2), Color(0.3, 0.3, 0.34)
]
## How each trade fits out its room: "cells" are its rows on the goods sheet, and "plan" is
## "shelves" for aisles of stock, "eatery" for tables and a counter, "jeweller" for a
## room of lit cases, "bureau" for a board of rates behind a grille, and "screens" for a wall of
## televisions.
const TRADES: Dictionary[String, Dictionary] = {
	"pharmacy":
	{
		"cells": [0, 1, 2],
		"plan": "shelves",
		"wall": Color(0.82, 0.96, 0.88),
		"floor": Color(0.75, 0.82, 0.8),
		"light": Color(0.92, 0.97, 1.0),
	},
	"electronics":
	{
		"cells": [6, 7, 8],
		"plan": "screens",
		"wall": Color(0.07, 0.13, 0.32),
		"floor": Color(0.16, 0.18, 0.24),
		"light": Color(0.85, 0.93, 1.0),
	},
	"robotics":
	{
		"cells": [8, 7, 6],
		"plan": "screens",
		"wall": Color(0.08, 0.3, 0.32),
		"floor": Color(0.2, 0.24, 0.26),
		"light": Color(0.75, 0.9, 1.0),
	},
	"market":
	{
		"cells": [3, 4, 5],
		"plan": "shelves",
		"wall": Color(1.0, 0.68, 0.2),
		"floor": Color(0.55, 0.45, 0.35),
		"light": Color(1.0, 0.96, 0.86),
	},
	"gold":
	{
		"cells": [11],
		"plan": "jeweller",
		"wall": Color(0.42, 0.03, 0.06),
		"floor": Color(0.3, 0.2, 0.14),
		"light": Color(1.0, 0.85, 0.62),
	},
	"exchange":
	{
		"cells": [11, 9],
		"plan": "bureau",
		"wall": Color(0.03, 0.24, 0.15),
		"floor": Color(0.26, 0.24, 0.2),
		"light": Color(1.0, 0.93, 0.8),
	},
	"noodle":
	{
		"cells": [9, 10],
		"plan": "eatery",
		"wall": Color(0.78, 0.11, 0.07),
		"floor": Color(0.42, 0.3, 0.22),
		"light": Color(1.0, 0.8, 0.55),
	},
	"cafe":
	{
		"cells": [10, 9],
		"plan": "eatery",
		"wall": Color(0.5, 0.27, 0.13),
		"floor": Color(0.4, 0.3, 0.22),
		"light": Color(1.0, 0.87, 0.68),
	},
}


## Where a shop's room is, so the things that fit it out don't each work that out again.
class Room:
	var along: Vector3
	var outward: Vector3
	var mouth: Vector3
	var width: float
	var depth: float
	var height: float
	var cells: Array
	var light: Color
	var dice: int
	var cartons: Array[Transform3D]
	var crates: Array[Transform3D]

	## A point in the room: across from its centre, in from the street, up from the floor.
	func point(across: float, into: float, up: float) -> Vector3:
		return mouth + along * across + outward * into + Vector3.UP * up


static func build(route: Route, street: Street, length: float) -> Node3D:
	var root := Node3D.new()
	root.name = "Shopfronts"
	var kit := CityMesh.new()
	var materials := _materials()
	var props := _props()
	var sheet := SignSheet.load_sheet()
	var cartons: Array[Transform3D] = []
	var crates: Array[Transform3D] = []
	for index: int in street.shops.size():
		var shop := street.shops[index]
		kit.at(shop.s)
		for prop: Kit in props.values():
			prop.at(shop.s)
		var room := _room(route, shop, index)
		room.cartons = cartons
		room.crates = crates
		_shell(kit, shop, room, materials)
		if shop.open:
			_fit_out(kit, props, sheet, shop, room, materials)
			if shop.s > -12.0 and shop.s < length + 35.0:
				root.add_child(_light(route, shop))
				root.add_child(_room_light(room))
	kit.commit(root)
	Furniture.instances(root, Furniture.CARTON, cartons)
	Furniture.instances(root, Furniture.CRATE, crates)
	for prop: Kit in props.values():
		prop.commit(root, PROP_REACH)
	return root


static func _room(route: Route, shop: Street.Shop, index: int) -> Room:
	var room := Room.new()
	room.along = route.tangent(shop.s)
	room.outward = route.right(shop.s) * shop.side
	room.mouth = (
		route.at(shop.s)
		+ room.outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
		+ Vector3.UP * StreetGround.KERB_HEIGHT
	)
	room.width = shop.width
	room.depth = shop.depth
	room.height = OPENING - StreetGround.KERB_HEIGHT
	var trade: Dictionary = TRADES[shop.trade]
	room.cells = trade["cells"]
	room.light = trade["light"]
	room.dice = Rng.derive(index, [77])
	return room


static func _materials() -> Dictionary[String, Material]:
	var fitted: Dictionary[String, Material] = {}
	fitted["tile"] = _lining("rectangular_facade_tiles", TILE_METRES)
	fitted["plaster"] = _lining("concrete_wall_004", PLASTER_METRES)
	fitted["paint"] = _lining("rusty_painted_metal", 1.3)
	fitted["ceiling"] = _lining("dirty_concrete", 2.4)
	fitted["floor"] = _lining("square_concrete_pavers", 1.8)
	var canvas := ShaderMaterial.new()
	canvas.shader = CANVAS
	fitted["canvas"] = canvas
	var tube := StandardMaterial3D.new()
	tube.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	tube.vertex_color_use_as_albedo = true
	tube.albedo_color = Color(1.8, 1.8, 1.8)
	fitted["tube"] = tube
	var steel := StandardMaterial3D.new()
	steel.albedo_color = Color(0.1, 0.1, 0.11)
	steel.vertex_color_use_as_albedo = true
	steel.metallic = 0.2
	steel.roughness = 0.5
	fitted["steel"] = steel
	var gold := StandardMaterial3D.new()
	gold.albedo_color = Color(1.0, 0.72, 0.28)
	gold.roughness = 0.3
	gold.emission_enabled = true
	gold.emission = Color(1.0, 0.62, 0.18)
	gold.emission_energy_multiplier = 0.9
	fitted["gold"] = gold
	var menu := StandardMaterial3D.new()
	menu.albedo_texture = load(SIGN_SHEET)
	menu.vertex_color_use_as_albedo = true
	menu.roughness = 0.8
	fitted["menu"] = menu
	var goods := ShaderMaterial.new()
	goods.shader = GOODS
	goods.set_shader_parameter("sheet", load(GOODS_SHEET))
	fitted["goods"] = goods
	return fitted


## A worn scan laid on in world metres and tinted per face, lit only by what's in the room.
static func _lining(scan: String, metres: float) -> StandardMaterial3D:
	var material := Facades.scan(scan, metres, Color.WHITE)
	material.metallic = 0.0
	return material


static func _props() -> Dictionary[String, Kit]:
	var props: Dictionary[String, Kit] = {}
	for model: String in [
		"chinese_stool",
		"small_wooden_table_01",
		"CashRegister_01",
		"hanging_industrial_lamp",
		"brass_pot_01",
	]:
		props[model] = Kit.new(PROP % [model, model])
	return props


## An open shop's room, with no front wall. [StreetWall] builds the frontage every shop has.
static func _shell(
	kit: CityMesh, shop: Street.Shop, room: Room, materials: Dictionary[String, Material]
) -> void:
	var along := room.along
	var outward := room.outward
	var mouth := room.mouth
	var half := along * shop.width * 0.5
	var back := outward * shop.depth
	var inside := Vector3.UP * room.height
	if not shop.open:
		return
	var trade: Dictionary = TRADES[shop.trade]
	var shade := lerpf(0.75, 1.1, shop.pick)
	var wall_tint: Color = trade["wall"] * shade
	var floor_tint: Color = trade["floor"]
	kit.quad(
		materials["floor"],
		mouth - half,
		mouth + half,
		mouth + half + back,
		mouth - half + back,
		Vector3.UP,
		floor_tint
	)
	kit.quad(
		materials["ceiling"],
		mouth - half + inside,
		mouth + half + inside,
		mouth + half + inside + back,
		mouth - half + inside + back,
		-Vector3.UP,
		Color(0.5, 0.48, 0.45)
	)
	_wall(kit, materials, mouth - half + back, mouth + half + back, -outward, wall_tint, room)
	_wall(kit, materials, mouth - half + back, mouth - half, along, wall_tint, room)
	_wall(kit, materials, mouth + half, mouth + half + back, -along, wall_tint, room)
	ShopFittings.services(kit, room, materials)


## A wall from a to b: tile to shoulder height, with a rail along it and old paint above.
static func _wall(
	kit: CityMesh,
	materials: Dictionary[String, Material],
	a: Vector3,
	b: Vector3,
	facing: Vector3,
	tint: Color,
	room: Room
) -> void:
	var dado := Vector3.UP * DADO
	var head := Vector3.UP * room.height
	kit.quad(materials["tile"], a, b, b + dado, a + dado, facing, tint)
	kit.quad(
		materials["plaster"], a + dado, b + dado, b + head, a + head, facing, tint.lerp(FADED, 0.45)
	)
	kit.bar(materials["steel"], a + dado + facing * 0.012, b + dado + facing * 0.012, 0.02)


static func _fit_out(
	kit: CityMesh,
	props: Dictionary[String, Kit],
	sheet: SignSheet,
	shop: Street.Shop,
	room: Room,
	materials: Dictionary[String, Material]
) -> void:
	var plan: String = TRADES[shop.trade]["plan"]
	if Rng.unit(room.dice, 40) < 0.1:
		_awning(kit, room, materials["canvas"], shop.pick)
	ShopFittings.delivered(room)
	match plan:
		"shelves":
			_wall_shelves(kit, room, materials, true)
			_gondola(kit, room, materials)
			_door_displays(kit, room, materials)
		"screens":
			_wall_shelves(kit, room, materials, false)
			_screen_wall(kit, sheet, shop, room, materials)
			_door_displays(kit, room, materials)
		"jeweller":
			_cases(kit, room, materials)
			_counter(kit, props, room, materials, room.depth * 0.55, true)
			ShopFittings.grille(
				kit, room, materials, room.depth * 0.55 - 0.1, room.width - 0.3, 1.0
			)
		"bureau":
			ShopFittings.rate_board(kit, room, materials)
			_counter(kit, props, room, materials, room.depth * 0.5, true)
			ShopFittings.grille(kit, room, materials, room.depth * 0.5 - 0.1, room.width - 0.3, 1.0)
		"eatery":
			_eatery_walls(kit, sheet, shop, room, materials)
			_counter(kit, props, room, materials, room.depth - 1.3, false)
			_tables(props, room)


## Cases round a jeweller's back and sides, from the counter back.
static func _cases(kit: CityMesh, room: Room, materials: Dictionary[String, Material]) -> void:
	ShopFittings.jewel_case(
		kit,
		room,
		materials,
		room.point(0.0, room.depth - 0.03, 0.0),
		-room.outward,
		room.width - 0.1,
		0
	)
	var inner := room.width * 0.5 - 0.03
	var long := room.depth * 0.42
	for end: float in [-1.0, 1.0]:
		ShopFittings.jewel_case(
			kit,
			room,
			materials,
			room.point(end * inner, room.depth - long * 0.5 - 0.45, 0.0),
			room.along * -end,
			long,
			2 + int(end)
		)


## The back of an eatery: its kitchen door and hood, a shelf of bottles, and the menus.
static func _eatery_walls(
	kit: CityMesh,
	sheet: SignSheet,
	shop: Street.Shop,
	room: Room,
	materials: Dictionary[String, Material]
) -> void:
	ShopFittings.kitchen(kit, room, materials)
	_shelf_bay(
		kit,
		room,
		materials,
		room.point(0.0, room.depth - 0.03, 1.28),
		-room.outward,
		room.width * 0.45,
		1,
		1
	)
	var inner := room.width * 0.5 - 0.005
	for end: float in [-1.0, 1.0]:
		ShopFittings.notices(
			kit,
			sheet,
			shop.trade,
			room,
			materials["menu"],
			room.point(end * inner, room.depth * 0.5, 1.75),
			room.along * -end,
			room.depth - 0.8,
			2 + int(end)
		)


## Boards and uprights down a run of wall, with stock on every board.
static func _shelf_bay(
	kit: CityMesh,
	room: Room,
	materials: Dictionary[String, Material],
	centre: Vector3,
	facing: Vector3,
	span: float,
	rows: int,
	stream: int
) -> void:
	var run := Vector3.UP.cross(facing).normalized()
	for row: int in rows:
		var y := Vector3.UP * (SHELF_FOOT + row * SHELF_RISE)
		var shelf := centre + y + facing * SHELF_DEPTH * 0.5
		kit.box(
			materials["steel"],
			Transform3D(Basis(run, Vector3.UP, facing), shelf),
			Vector3(span, 0.03, SHELF_DEPTH),
			Color(1.0, 1.0, 1.0)
		)
		_goods_row(
			kit,
			materials["goods"],
			room,
			centre + y + Vector3.UP * 0.015 + facing * (SHELF_DEPTH - 0.04),
			facing,
			span,
			Rng.derive(room.dice, [stream, row])
		)
	var top := Vector3.UP * (SHELF_FOOT + rows * SHELF_RISE)
	var board := centre + top + facing * SHELF_DEPTH * 0.5
	kit.box(
		materials["steel"],
		Transform3D(Basis(run, Vector3.UP, facing), board),
		Vector3(span, 0.03, SHELF_DEPTH)
	)
	if (centre + top).y - room.mouth.y + ShopFittings.CARTON_HIGH < room.height - 0.1:
		ShopFittings.overstock(room, board, run, span, Rng.derive(room.dice, [stream, 99]))
	var bays := maxi(roundi(span), 1)
	for bay: int in bays + 1:
		var post := centre + run * ((float(bay) / bays - 0.5) * span)
		kit.bar(materials["steel"], post, post + top, 0.035)


## Stock along a shelf, as two layers of cards so it shifts as you ride past.
static func _goods_row(
	kit: CityMesh,
	material: Material,
	room: Room,
	centre: Vector3,
	facing: Vector3,
	span: float,
	dice: int
) -> void:
	var run := Vector3.UP.cross(facing).normalized()
	var tall := SHELF_RISE - 0.03
	var light := room.light
	for layer: int in 2:
		var at := centre - facing * (0.24 * layer) - run * span * 0.5
		var along := 0.0
		var i := 0
		while along < span - 0.05:
			var key := i * 6 + layer * 500
			var wide := minf(lerpf(0.35, CARD_WIDTH, Rng.unit(dice, key)), span - along)
			var left := at + run * along
			var right := left + run * wide
			along += wide
			i += 1
			# The front row has holes in it, where it has sold through to the row behind.
			if layer == 0 and Rng.unit(dice, key + 1) < GAP_CHANCE:
				continue
			var cell: int = room.cells[int(Rng.unit(dice, key + 2) * room.cells.size())]
			var uv := _card(cell)
			var share := wide / CARD_WIDTH
			var from := uv.position.x + Rng.unit(dice, key + 3) * uv.size.x * (1.0 - share)
			var to := from + uv.size.x * share
			var rise := Vector3.UP * tall * lerpf(0.82, 1.0, Rng.unit(dice, key + 4))
			var tone := lerpf(0.72, 1.05, Rng.unit(dice, key + 5))
			var tint := (light if layer == 0 else light.darkened(0.45)) * tone
			tint.a = 1.0
			var corners: Array[Vector3] = [left, right, right + rise, left + rise]
			var mapped: Array[Vector2] = [
				Vector2(from, uv.end.y),
				Vector2(to, uv.end.y),
				Vector2(to, uv.position.y),
				Vector2(from, uv.position.y),
			]
			kit.face(material, corners, mapped, facing, tint)


## Where a cell lies on the goods sheet.
static func _card(cell: int) -> Rect2:
	var size := Vector2(1.0 / SHEET_COLUMNS, 1.0 / SHEET_ROWS)
	var at := Vector2(cell % SHEET_COLUMNS, floori(float(cell) / SHEET_COLUMNS)) * size
	var inset := Vector2(0.001, 0.001)
	return Rect2(at + inset, size - inset * 2.0)


## Shelves up the back wall and, unless the room needs them clear, along both sides.
static func _wall_shelves(
	kit: CityMesh, room: Room, materials: Dictionary[String, Material], sides: bool
) -> void:
	_shelf_bay(
		kit,
		room,
		materials,
		room.point(0.0, room.depth - 0.03, 0.0),
		-room.outward,
		room.width - 0.1,
		SHELF_ROWS,
		1
	)
	if not sides:
		return
	var inner := room.width * 0.5 - 0.03
	for end: float in [-1.0, 1.0]:
		_shelf_bay(
			kit,
			room,
			materials,
			room.point(end * inner, (room.depth + 0.3) * 0.5, 0.0),
			room.along * -end,
			room.depth - 0.5,
			SHELF_ROWS,
			2 + int(end)
		)


## A freestanding aisle down the middle of a wide shop, stocked both ways and on the end.
static func _gondola(kit: CityMesh, room: Room, materials: Dictionary[String, Material]) -> void:
	if room.width < 3.6:
		return
	var long := room.depth - 2.6
	var middle := room.point(0.0, 1.5 + long * 0.5, 0.0)
	for end: float in [-1.0, 1.0]:
		_shelf_bay(kit, room, materials, middle, room.along * end, long, 4, 10 + int(end))
	_shelf_bay(
		kit, room, materials, room.point(0.0, 1.5, 0.0), -room.outward, SHELF_DEPTH * 2.0, 4, 12
	)


## Low stands either side of an open door, stocked towards the pavement.
static func _door_displays(
	kit: CityMesh, room: Room, materials: Dictionary[String, Material]
) -> void:
	var wide := room.width * 0.32
	var high := 0.72
	for end: float in [-1.0, 1.0]:
		var across := end * (room.width * 0.5 - wide * 0.5 - 0.1)
		var basis := Basis(room.along, Vector3.UP, room.outward)
		var coat := PAINT_COATS[int(Rng.unit(room.dice, 33 + int(end)) * PAINT_COATS.size())]
		kit.box(
			materials["paint"],
			Transform3D(basis, room.point(across, 0.25, 0.09 + (high - 0.09) * 0.5)),
			Vector3(wide, high - 0.09, 0.6),
			coat
		)
		kit.box(
			materials["steel"],
			Transform3D(basis, room.point(across, 0.27, 0.045)),
			Vector3(wide - 0.08, 0.09, 0.5)
		)
		_goods_row(
			kit,
			materials["goods"],
			room,
			room.point(across, -0.05, high),
			-room.outward,
			wide,
			Rng.derive(room.dice, [30, int(end)])
		)
		_goods_row(
			kit,
			materials["goods"],
			room,
			room.point(across, -0.06, 0.1),
			-room.outward,
			wide,
			Rng.derive(room.dice, [31, int(end)])
		)


## The back wall as a bank of working screens, in place of the shelves' top rows.
static func _screen_wall(
	kit: CityMesh,
	sheet: SignSheet,
	shop: Street.Shop,
	room: Room,
	materials: Dictionary[String, Material]
) -> void:
	var columns := maxi(int(room.width / 1.05), 1)
	var wide := (room.width - 0.2) / columns
	var tall := wide * 0.6
	var picture: Texture2D = load(SIGN_SHEET)
	for row: int in 3:
		for column: int in columns:
			var across := (column + 0.5 - columns * 0.5) * wide
			var up := 1.0 + (row + 0.5) * (tall + 0.04)
			var centre := room.point(across, room.depth - SHELF_DEPTH - 0.08, up)
			var roll := Rng.unit(room.dice, 200 + row * 16 + column)
			var pick := sheet.pick(shop.trade, false, roll)
			var rect: Rect2 = pick["uv"]
			var material := ShaderMaterial.new()
			material.shader = SCREEN
			material.set_shader_parameter("sheet", picture)
			material.set_shader_parameter(
				"picture", Vector4(rect.position.x, rect.position.y, rect.size.x, rect.size.y)
			)
			material.set_shader_parameter("colour_a", Color.from_hsv(roll, 0.4, 0.9))
			material.set_shader_parameter(
				"colour_b", Color.from_hsv(fmod(roll + 0.1, 1.0), 0.4, 0.9)
			)
			material.set_shader_parameter("sweep", 0.0)
			material.set_shader_parameter("size", Vector2(wide, tall))
			material.set_shader_parameter("phase", roll)
			material.set_shader_parameter("brightness", 0.85)
			kit.box(
				materials["steel"],
				Transform3D(Basis(room.along, Vector3.UP, room.outward), centre),
				Vector3(wide - 0.02, tall + 0.02, 0.12)
			)
			var run := Vector3.UP.cross(-room.outward).normalized()
			var half := run * (wide - 0.1) * 0.5
			var rise := Vector3.UP * (tall - 0.06) * 0.5
			var front := centre - room.outward * 0.07
			kit.face(
				material,
				[
					front - half - rise,
					front + half - rise,
					front + half + rise,
					front - half + rise
				],
				[Vector2(0.0, 1.0), Vector2(1.0, 1.0), Vector2(1.0, 0.0), Vector2(0.0, 0.0)],
				-room.outward
			)


## A counter across the room, for a jeweller's cases or a kitchen's pass.
static func _counter(
	kit: CityMesh,
	props: Dictionary[String, Kit],
	room: Room,
	materials: Dictionary[String, Material],
	into: float,
	glass: bool
) -> void:
	var high := 1.0
	var wide := room.width - 0.3
	var basis := Basis(room.along, Vector3.UP, room.outward)
	var coat := Color(0.3, 0.1, 0.09) if glass else Color(0.62, 0.42, 0.28)
	kit.box(
		materials["paint"],
		Transform3D(basis, room.point(0.0, into, 0.12 + (high - 0.12) * 0.5)),
		Vector3(wide, high - 0.12, 0.6),
		coat
	)
	kit.box(
		materials["steel"],
		Transform3D(basis, room.point(0.0, into + 0.03, 0.06)),
		Vector3(wide - 0.04, 0.12, 0.52)
	)
	kit.box(
		materials["paint"],
		Transform3D(basis, room.point(0.0, into, high + 0.02)),
		Vector3(wide + 0.06, 0.04, 0.68),
		Color(0.72, 0.7, 0.66)
	)
	if glass:
		# A lit case along the front, with what's for sale at eye height behind the glass.
		var case_glow := Color(0.045, 0.026, 0.012)
		kit.box(
			materials["tube"],
			Transform3D(basis, room.point(0.0, into - 0.305, 0.62)),
			Vector3(wide - 0.06, 0.52, 0.01),
			case_glow
		)
		for slab: int in maxi(roundi(wide * 9.0), 1):
			var across := (Rng.unit(room.dice, 500 + slab) - 0.5) * wide * 0.95
			var up := 0.4 + 0.21 * floorf(Rng.unit(room.dice, 540 + slab) * 3.0)
			kit.box(
				materials["gold"],
				Transform3D(basis, room.point(across, into - 0.33, up)),
				Vector3(lerpf(0.03, 0.09, Rng.unit(room.dice, 580 + slab)), 0.02, 0.02)
			)
	else:
		for pot: int in 3:
			var across := (pot - 1.0) * minf(wide * 0.3, 0.9)
			_prop(props["brass_pot_01"], room.point(across, into, high), pot * 1.3, 1.4)
	_prop(props["CashRegister_01"], room.point(wide * 0.3, into, high), PI)


## A few places to sit, under lamps on long flexes.
static func _tables(props: Dictionary[String, Kit], room: Room) -> void:
	var count := maxi(int(room.width / 1.7), 1)
	for i: int in count:
		var across := ((i + 0.5) / count - 0.5) * room.width
		var into := 1.5 + Rng.unit(room.dice, 300 + i) * 0.5
		var turn := Rng.unit(room.dice, 310 + i) * TAU
		_prop(props["small_wooden_table_01"], room.point(across, into, 0.0), turn)
		for stool: int in 2:
			var round_it := turn + stool * PI + 0.4
			var seat := Vector3(cos(round_it), 0.0, sin(round_it)) * 0.62
			_prop(props["chinese_stool"], room.point(across, into, 0.0) + seat, round_it)
		_prop(
			props["hanging_industrial_lamp"], room.point(across, into, room.height), 0.0, 1.0, true
		)


## All of a model at a spot, stood on it, or if [param hang] hung from it.
static func _prop(kit: Kit, where: Vector3, turn: float, scale := 1.0, hang := false) -> void:
	var low := INF
	var top := -INF
	for part: String in kit.parts():
		var bounds := kit.bounds(part)
		low = minf(low, bounds.position.y + kit.origin(part).y)
		top = maxf(top, bounds.end.y + kit.origin(part).y)
	var basis := Basis(Vector3.UP, turn).scaled(Vector3.ONE * scale)
	var lift := Vector3.UP * (-top if hang else -low) * scale
	for part: String in kit.parts():
		kit.place(part, Transform3D(basis, where + lift + basis * kit.origin(part)))


## A striped canvas over the pavement, lit through from the shop.
static func _awning(kit: CityMesh, room: Room, canvas: Material, pick: float) -> void:
	var stripe_a := Color.from_hsv(pick * 7.0 - floorf(pick * 7.0), 0.85, 0.75)
	var stripe_b := Color(0.86, 0.82, 0.72)
	var count := maxi(roundi(room.width / AWNING_STRIPE), 1)
	var wide := room.width / count
	var out := -AWNING_OUT
	var top := room.height - 0.03
	var low := top - AWNING_DROP
	for i: int in count:
		var a := (i - count * 0.5) * wide
		var b := a + wide
		var colour := stripe_a if i % 2 == 0 else stripe_b
		var pitch := room.point(b, out, low) - room.point(a, out, low)
		var sloped := room.point(a, 0.0, top) - room.point(a, out, low)
		var skin := pitch.cross(sloped).normalized()
		for side: float in [1.0, -1.0]:
			kit.quad(
				canvas,
				room.point(a, 0.0, top),
				room.point(b, 0.0, top),
				room.point(b, out, low),
				room.point(a, out, low),
				skin * side,
				colour
			)
			kit.quad(
				canvas,
				room.point(a, out, low),
				room.point(b, out, low),
				room.point(b, out, low - VALANCE),
				room.point(a, out, low - VALANCE),
				room.outward * side,
				colour
			)


## The light that actually gives a room its shape: from the tubes, down the walls and across
## the floor, darkening towards the corners.
static func _room_light(room: Room) -> OmniLight3D:
	var glow := OmniLight3D.new()
	glow.light_color = room.light
	glow.light_energy = 2.4
	glow.light_volumetric_fog_energy = 0.0
	glow.light_specular = 0.4
	glow.omni_range = maxf(room.width, room.depth) * 1.15
	glow.omni_attenuation = 1.0
	glow.position = room.point(0.0, room.depth * 0.5, room.height - 0.55)
	return glow


static func _light(route: Route, shop: Street.Shop) -> OmniLight3D:
	var outward := route.right(shop.s) * shop.side
	var glow := OmniLight3D.new()
	glow.light_color = TRADES[shop.trade]["light"]
	glow.light_energy = 0.8 + shop.width * 0.1
	# A pool of light on the pavement, and not a wall of glowing fog in front of the shop.
	glow.light_volumetric_fog_energy = 0.12
	glow.omni_range = 9.0 + shop.width * 0.6
	glow.omni_attenuation = 1.1
	glow.position = (
		route.at(shop.s)
		+ outward * (NeonBiome.STREET_HALF + Street.SIDEWALK + 0.2)
		+ Vector3.UP * 2.7
	)
	return glow
