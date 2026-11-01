class_name StreetDressing
extends RefCounted
## What a street collects once it's been left alone: tin canopies over the shops, air
## conditioners and cable tacked on above them, bills and sprayed characters on the shutters,
## and rubbish left against them. Stands on the frontage [StreetWall] builds.

const PAPER: Shader = preload("res://game/street_paper.gdshader")
const AIRCON := "res://assets/polyhaven/exterior_aircon_unit/exterior_aircon_unit_1k.gltf"
const CANOPY_OPEN := 0.6
const CANOPY_CLOSED := 0.2
const AIRCON_CHANCE := 0.4
const CABLE_CHANCE := 0.6
const BILLED_CHANCE := 0.65
const SPRAYED_CHANCE := 0.3
const RUBBISH_CHANCE := 0.35
## How wide a shop's fascia sign may be, which nothing on the lintel should sit in front of.
const FASCIA_WIDE := 2.8
## The face of the lintel, which everything over the opening is fixed to.
const LINTEL_FACE := StreetWall.LINTEL_OUT + 0.01
const SPRAYS: Array[Color] = [
	Color(0.5, 0.05, 0.04),
	Color(0.04, 0.04, 0.05),
	Color(0.7, 0.68, 0.62),
	Color(0.12, 0.2, 0.42),
]


class Parts:
	extends RefCounted
	var sheeting: ShaderMaterial
	var bearer: StandardMaterial3D
	var cable: StandardMaterial3D
	var tube: StandardMaterial3D
	var bill: ShaderMaterial
	var spray: ShaderMaterial
	var units: Array[Transform3D] = []
	var bags: Array[Transform3D] = []
	var cartons: Array[Transform3D] = []
	var crates: Array[Transform3D] = []


static func build(route: Route, street: Street, section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "StreetDressing"
	var kit := CityMesh.new()
	var parts: Parts = Once.made("street_dressing", _parts)
	var sheet := SignSheet.load_sheet()
	for index: int in street.shops.size():
		var shop := street.shops[index]
		if not section.holds(shop.s):
			continue
		kit.at(shop.s)
		var front := StreetWall.front_of(route, shop, index)
		var dice := Rng.derive(index, [79])
		if Rng.unit(dice, 0) < (CANOPY_OPEN if shop.open else CANOPY_CLOSED):
			_canopy(kit, front, shop, dice, parts)
		if Rng.unit(dice, 1) < AIRCON_CHANCE:
			_aircon(kit, front, shop, dice, parts)
		if Rng.unit(dice, 2) < CABLE_CHANCE:
			_cables(kit, front, shop, dice, parts)
		if Rng.unit(dice, 3) < BILLED_CHANCE:
			_bills(kit, front, shop, dice, sheet, parts)
		if not shop.open:
			if Rng.unit(dice, 4) < SPRAYED_CHANCE:
				_sprayed(kit, front, shop, dice, sheet, parts)
			if Rng.unit(dice, 5) < RUBBISH_CHANCE:
				_rubbish(front, shop, dice, parts)
	kit.commit(root)
	Furniture.instances(root, AIRCON, parts.units)
	Furniture.instances(root, Furniture.BAG, parts.bags)
	Furniture.instances(root, Furniture.CARTON, parts.cartons)
	Furniture.instances(root, Furniture.CRATE, parts.crates)
	return root


## A sheet of tin on angle-iron bearers, falling away from the wall and held up by tie rods.
static func _canopy(
	kit: CityMesh, front: Facades.Tier, shop: Street.Shop, dice: int, parts: Parts
) -> void:
	var half := shop.width * 0.5 - StreetWall.PIER_WIDE * 0.5 - 0.05
	var deep := lerpf(1.1, 1.9, Rng.unit(dice, 10))
	var inner := Shopfronts.OPENING + 0.03
	var outer := inner - lerpf(0.14, 0.32, Rng.unit(dice, 11))
	var tint := Color.WHITE.darkened(Rng.unit(dice, 12) * 0.35)
	var start := Vector2(Rng.unit(dice, 13), Rng.unit(dice, 14))
	var skin := 1.0 / StreetWall.SHEETING_METRES
	var a := front.point(-half, inner, LINTEL_FACE)
	var b := front.point(half, inner, LINTEL_FACE)
	var c := front.point(half, outer, deep)
	var d := front.point(-half, outer, deep)
	# The same sheet seen from above and from the road below.
	kit.quad(parts.sheeting, a, b, c, d, Vector3.UP, tint, skin, start)
	kit.quad(parts.sheeting, a, b, c, d, Vector3.DOWN, tint * 0.8, skin, start)
	var count := maxi(roundi(half * 2.0 / 1.3), 2)
	for i: int in count:
		var x := lerpf(-half + 0.08, half - 0.08, float(i) / float(count - 1))
		kit.bar(
			parts.bearer,
			front.point(x, inner - 0.05, LINTEL_FACE),
			front.point(x, outer - 0.05, deep),
			0.05
		)
	kit.bar(parts.bearer, front.point(-half, outer - 0.03, deep), c - Vector3.UP * 0.03, 0.06)
	for end: float in [-1.0, 1.0]:
		var x := end * half * 0.72
		var eye := front.point(x, outer, deep * 0.85)
		kit.bar(parts.bearer, eye, front.point(x, inner + 0.85, LINTEL_FACE), 0.016)
	if shop.open:
		# A bare tube for the pavement, wired to the underside by someone who sold noodles.
		var x := half * (Rng.unit(dice, 15) - 0.5)
		var y := lerpf(inner, outer, 0.55) - 0.12
		kit.box(
			parts.tube,
			front.place(x, y, deep * 0.55),
			Vector3(1.2, 0.035, 0.035),
			Color(1.0, 0.92, 0.8)
		)


## An outdoor unit bracketed to the lintel, with its drain run down to the pavement.
static func _aircon(
	kit: CityMesh, front: Facades.Tier, shop: Street.Shop, dice: int, parts: Parts
) -> void:
	var clear := shop.width * 0.5 - 0.7
	var x := lerpf(-clear, clear, Rng.unit(dice, 20))
	if shop.open:
		# To one side of the shop's own sign, or not at all.
		if clear < FASCIA_WIDE * 0.5 + 0.3:
			return
		x = signf(x - 0.001) * lerpf(FASCIA_WIDE * 0.5 + 0.3, clear, Rng.unit(dice, 21))
	var y := lerpf(3.2, 3.5, Rng.unit(dice, 22))
	var out := LINTEL_FACE + 0.18
	parts.units.append(
		Transform3D(Basis(front.along, Vector3.UP, front.facing), front.point(x, y, out))
	)
	for end: float in [-0.3, 0.3]:
		kit.bar(
			parts.bearer,
			front.point(x + end, y - 0.02, LINTEL_FACE),
			front.point(x + end, y - 0.02, out + 0.22),
			0.035
		)
	var drain := x + 0.36
	kit.bar(parts.bearer, front.point(drain, y, out), front.point(drain, 0.45, out * 0.6), 0.018)


## What's been run along over the shops since, in sagging loose runs between clips.
static func _cables(
	kit: CityMesh, front: Facades.Tier, shop: Street.Shop, dice: int, parts: Parts
) -> void:
	var half := shop.width * 0.5
	var runs := 1 + int(Rng.unit(dice, 30) * 3.0)
	for run: int in runs:
		var y := lerpf(3.55, 3.98, Rng.unit(dice, 31 + run))
		var out := LINTEL_FACE + 0.02 + run * 0.018
		var thick := lerpf(0.012, 0.028, Rng.unit(dice, 35 + run))
		var spans := maxi(roundi(shop.width / 1.5), 1)
		for span: int in spans:
			var from := lerpf(-half, half, float(span) / spans)
			var to := lerpf(-half, half, float(span + 1) / spans)
			var sag := lerpf(0.02, 0.1, Rng.unit(Rng.derive(dice, [span, run]), 0))
			var last := front.point(from, y, out)
			for step: int in range(1, 6):
				var t := step / 5.0
				var dip := 4.0 * t * (1.0 - t) * sag
				var next := front.point(lerpf(from, to, t), y - dip, out)
				kit.bar(parts.cable, last, next, thick, Color(0.05, 0.05, 0.05))
				last = next


## Layers of bills, pasted over one another at about the height of an arm.
static func _bills(
	kit: CityMesh, front: Facades.Tier, shop: Street.Shop, dice: int, sheet: SignSheet, parts: Parts
) -> void:
	var clumps := 1 + int(Rng.unit(dice, 40) * 2.0)
	var half := shop.width * 0.5
	for clump: int in clumps:
		var cast := Rng.derive(dice, [41, clump])
		# On the pier, whose face is flat, or on what's behind the opening's edge.
		var on_pier := shop.open or Rng.unit(cast, 0) < 0.4
		var middle := half if on_pier else lerpf(-half + 1.0, half - 1.0, Rng.unit(cast, 1))
		var out := StreetWall.PIER_OUT + 0.006 if on_pier else StreetWall.RECESS + 0.012
		var reach := 0.14 if on_pier else 0.9
		var count := 2 + int(Rng.unit(cast, 2) * (2.0 if on_pier else 6.0))
		for i: int in count:
			var bill := Rng.derive(cast, [i])
			var x := middle + (Rng.unit(bill, 0) - 0.5) * reach
			var y := lerpf(1.0, 2.0, Rng.unit(bill, 1))
			var tall := on_pier or Rng.unit(bill, 2) < 0.55
			var size := Vector2(0.36, 0.56) if tall else Vector2(0.62, 0.34)
			var lean := (Rng.unit(bill, 3) - 0.5) * 0.09
			_stuck(
				kit,
				front,
				parts.bill,
				sheet.pick(shop.trade, tall, Rng.unit(bill, 4)),
				Vector3(x, y, out + i * 0.0015),
				size,
				lean,
				Color.WHITE.darkened(Rng.unit(bill, 5) * 0.3),
				shop.side
			)


## Characters sprayed across a shutter, a couple of big strokes high up.
static func _sprayed(
	kit: CityMesh, front: Facades.Tier, shop: Street.Shop, dice: int, sheet: SignSheet, parts: Parts
) -> void:
	var clear := shop.width * 0.5 - StreetWall.PIER_WIDE - 0.7
	if clear < 0.0:
		return
	var size := Vector2.ONE * lerpf(1.1, 1.8, Rng.unit(dice, 50)) * Vector2(1.7, 1.0)
	_stuck(
		kit,
		front,
		parts.spray,
		sheet.pick(shop.trade, false, Rng.unit(dice, 51)),
		Vector3(
			lerpf(-clear, clear, Rng.unit(dice, 52)),
			lerpf(0.55, 1.0, Rng.unit(dice, 53)),
			StreetWall.RECESS + 0.02
		),
		size,
		(Rng.unit(dice, 54) - 0.5) * 0.12,
		SPRAYS[int(Rng.unit(dice, 55) * SPRAYS.size())],
		shop.side
	)


## One flat design on the front, [param at] being across, up and out for its bottom middle.
static func _stuck(
	kit: CityMesh,
	front: Facades.Tier,
	material: Material,
	pick: Dictionary,
	at: Vector3,
	size: Vector2,
	lean: float,
	tint: Color,
	side: float
) -> void:
	var mapped: Rect2 = pick["uv"]
	var uvs: Array[Vector2] = [
		Vector2(mapped.position.x, mapped.end.y),
		Vector2(mapped.end.x, mapped.end.y),
		Vector2(mapped.end.x, mapped.position.y),
		Vector2(mapped.position.x, mapped.position.y),
	]
	var mirrored: Array[Vector2] = [uvs[1], uvs[0], uvs[3], uvs[2]]
	var half := size.x * 0.5
	var slip := lean * size.y
	var corners: Array[Vector3] = [
		front.point(at.x - half, at.y - lean * half, at.z),
		front.point(at.x + half, at.y + lean * half, at.z),
		front.point(at.x + half + slip, at.y + size.y + lean * half, at.z),
		front.point(at.x - half + slip, at.y + size.y - lean * half, at.z),
	]
	kit.face(material, corners, mirrored if side > 0.0 else uvs, front.facing, tint)


## Bags and flattened boxes left against a shutter, which nobody is coming for.
static func _rubbish(front: Facades.Tier, shop: Street.Shop, dice: int, parts: Parts) -> void:
	var half := shop.width * 0.5 - StreetWall.PIER_WIDE - 0.3
	var kerb := StreetGround.KERB_HEIGHT
	var middle := lerpf(-half, half, Rng.unit(dice, 60))
	var count := 2 + int(Rng.unit(dice, 61) * 4.0)
	for i: int in count:
		var item := Rng.derive(dice, [62, i])
		var x := clampf(middle + (Rng.unit(item, 0) - 0.5) * 1.8, -half, half)
		var out := lerpf(0.05, 0.6, Rng.unit(item, 1))
		var place := Transform3D(
			front.basis.rotated(Vector3.UP, Rng.unit(item, 2) * TAU), front.point(x, kerb, out)
		)
		var roll := Rng.unit(item, 3)
		if roll < 0.55:
			parts.bags.append(place)
		elif roll < 0.85:
			parts.cartons.append(place)
		else:
			parts.crates.append(place)


static func _parts() -> Parts:
	var parts := Parts.new()
	parts.sheeting = StreetWall.steel(StreetWall.SHEETING)
	parts.bearer = Facades.scan("rusty_metal_02", 0.8, Color(0.6, 0.55, 0.5))
	parts.bearer.metallic = 0.0
	parts.cable = StandardMaterial3D.new()
	parts.cable.vertex_color_use_as_albedo = true
	parts.cable.roughness = 0.5
	parts.tube = StandardMaterial3D.new()
	parts.tube.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	parts.tube.vertex_color_use_as_albedo = true
	parts.tube.albedo_color = Color(3.2, 3.2, 3.2)
	parts.bill = ShaderMaterial.new()
	parts.bill.shader = PAPER
	parts.bill.set_shader_parameter("sheet", load(Signage.SHEET))
	parts.spray = ShaderMaterial.new()
	parts.spray.shader = PAPER
	parts.spray.set_shader_parameter("sheet", load(Signage.SHEET))
	parts.spray.set_shader_parameter("spray", true)
	return parts
