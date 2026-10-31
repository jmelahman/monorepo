class_name StreetWall
extends RefCounted
## What every frontage is built from at the height a rider actually sees: a concrete lintel,
## piers on heavy plinths with the pipes and boxes fixed to them since, and on the shops that
## have closed, a steel shutter set back in its housing and guides. Some are propped open on a
## light left on, some have a door let into them, and some have been boarded over.

const BRUTAL: Shader = preload("res://game/brutal.gdshader")
const STEEL: Shader = preload("res://game/street_metal.gdshader")
const CONCRETE := "dirty_concrete"
const CONCRETE_METRES := 3.6
## Metres of shutter one repeat of each scan covers.
const SHUTTERS: Dictionary[String, float] = {
	"rusted_shutter": 1.8,
	"rusty_metal_shutter": 1.9,
	"painted_metal_shutter": 2.0,
}
const SHEETING := "worn_corrugated_iron"
const SHEETING_METRES := 2.0
## How far a closed front sits behind the face of the wall.
const RECESS := -0.12
const PIER_WIDE := 0.5
const PIER_OUT := 0.34
const PIER_BACK := -0.2
const PLINTH_HIGH := 0.62
const LINTEL_OUT := 0.08
const HOUSING_HIGH := 0.26
const GUIDE := 0.07
const DOOR_WIDE := 1.0
const DOOR_HIGH := 2.2
const PROPPED_CHANCE := 0.14
const DOOR_CHANCE := 0.18
const BOARDED_CHANCE := 0.14
const LEAK := Color(0.42, 0.26, 0.13)
## The paint a shutter was given, whichever scan it's on, before the years got at it.
const PAINTS: Array[Color] = [
	Color(0.95, 0.95, 0.95),
	Color(0.8, 0.86, 0.95),
	Color(0.78, 0.92, 0.82),
	Color(1.0, 0.62, 0.56),
	Color(1.0, 0.86, 0.6),
]


class Parts:
	extends RefCounted
	var concrete: ShaderMaterial
	var shutters: Array[ShaderMaterial] = []
	var sheeting: ShaderMaterial
	var metal: StandardMaterial3D
	var rust: StandardMaterial3D
	var leak: StandardMaterial3D
	var dark: StandardMaterial3D


static func build(route: Route, street: Street) -> Node3D:
	var root := Node3D.new()
	root.name = "StreetWall"
	var kit := CityMesh.new()
	var parts := _parts()
	for index: int in street.shops.size():
		var shop := street.shops[index]
		kit.at(shop.s)
		var front := _front(route, shop, Rng.derive(index, [78]))
		_lintel(kit, front, shop, parts)
		_pier(kit, front, shop.width * 0.5, parts)
		_housing(kit, front, shop, parts)
		if not shop.open:
			_closed(kit, front, shop, parts)
	kit.commit(root)
	return root


static func _front(route: Route, shop: Street.Shop, dice: int) -> Facades.Tier:
	var outward := route.right(shop.s) * shop.side
	var front := Facades.Tier.new()
	front.plane = route.at(shop.s) + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	front.plane.y = route.at(shop.s).y
	front.along = route.tangent(shop.s)
	front.facing = -outward
	front.basis = Basis(Vector3.UP.cross(front.facing).normalized(), Vector3.UP, front.facing)
	front.width = shop.width
	var pale := lerpf(0.5, 0.85, Rng.unit(dice, 0))
	front.tone = Color(pale, pale, pale * 0.97, lerpf(0.5, 1.0, Rng.unit(dice, 1)))
	front.dice = dice
	return front


## The band over the opening, up to the first floor's wall, with a drip along its foot.
static func _lintel(kit: CityMesh, front: Facades.Tier, shop: Street.Shop, parts: Parts) -> void:
	var high := Facades.GROUND_FLOOR - Shopfronts.OPENING
	var band := Vector3(shop.width, high, 0.3)
	var middle := Shopfronts.OPENING + high * 0.5
	kit.block(parts.concrete, front.place(0.0, middle, LINTEL_OUT - 0.15), band, front.tone, 0.03)
	var drip := Vector3(shop.width, 0.09, 0.34)
	kit.block(
		parts.concrete,
		front.place(0.0, Shopfronts.OPENING - 0.03, LINTEL_OUT - 0.14),
		drip,
		front.tone * Color(0.8, 0.8, 0.8, 1.0),
		0.02
	)


## The party wall between two shops as a pier, on a plinth that's taken a few decades of knocks.
static func _pier(kit: CityMesh, front: Facades.Tier, at: float, parts: Parts) -> void:
	var deep := PIER_OUT - PIER_BACK
	var out := (PIER_OUT + PIER_BACK) * 0.5
	var high := Shopfronts.OPENING
	kit.block(
		parts.concrete,
		front.place(at, high * 0.5, out),
		Vector3(PIER_WIDE, high, deep),
		front.tone,
		0.04
	)
	kit.block(
		parts.concrete,
		front.place(at, PLINTH_HIGH * 0.5, out + 0.04),
		Vector3(PIER_WIDE + 0.14, PLINTH_HIGH, deep + 0.08),
		front.tone * Color(0.72, 0.72, 0.72, 1.0),
		0.05
	)
	_pier_fittings(kit, front, at, parts)


## Whatever has been fixed to a pier since: a drain, or conduit into a box of meters.
static func _pier_fittings(kit: CityMesh, front: Facades.Tier, at: float, parts: Parts) -> void:
	var roll := Rng.unit(front.dice, 2)
	var skin := PIER_OUT + 0.05
	var high := Shopfronts.OPENING + 0.9
	if roll < 0.3:
		var x := at + 0.12
		kit.bar(parts.rust, front.point(x, 0.12, skin), front.point(x, high, skin), 0.09)
		kit.bar(parts.rust, front.point(x, 0.12, skin), front.point(x, 0.03, skin + 0.14), 0.09)
		var clip := 0.7
		while clip < high:
			kit.box(
				parts.metal,
				front.place(x, clip, skin - 0.01),
				Vector3(0.13, 0.04, 0.13),
				Color(0.45, 0.45, 0.45)
			)
			clip += 1.15
	elif roll < 0.55:
		var x := at - 0.08
		kit.bar(
			parts.metal, front.point(x, 2.0, skin - 0.02), front.point(x, high, skin - 0.02), 0.04
		)
		kit.box(
			parts.metal,
			front.place(at, 1.6, skin + 0.03),
			Vector3(0.38, 0.8, 0.16),
			Color(0.55, 0.57, 0.55)
		)
		# The door of the box, and the lock nobody has the key to.
		kit.box(
			parts.rust,
			front.place(at, 1.6, skin + 0.115),
			Vector3(0.33, 0.74, 0.012),
			Color(0.7, 0.7, 0.7)
		)
		kit.box(
			parts.metal,
			front.place(at + 0.12, 1.6, skin + 0.13),
			Vector3(0.03, 0.09, 0.02),
			Color(0.2, 0.2, 0.2)
		)


## The box a shutter rolls up into and the channels it runs in, on a working front or a dead one.
static func _housing(kit: CityMesh, front: Facades.Tier, shop: Street.Shop, parts: Parts) -> void:
	var clear := shop.width - PIER_WIDE
	var y := Shopfronts.OPENING - HOUSING_HIGH * 0.5 - 0.07
	var tint := Color(0.55, 0.55, 0.55)
	kit.box(
		parts.metal, front.place(0.0, y, RECESS + 0.04), Vector3(clear, HOUSING_HIGH, 0.3), tint
	)
	var tall := Shopfronts.OPENING - 0.1
	for end: float in [-1.0, 1.0]:
		kit.box(
			parts.metal,
			front.place(end * (clear * 0.5 - GUIDE * 0.5), tall * 0.5, RECESS),
			Vector3(GUIDE, tall, 0.1),
			tint
		)


## How one closed shop was left.
static func _closed(kit: CityMesh, front: Facades.Tier, shop: Street.Shop, parts: Parts) -> void:
	var clear := shop.width - PIER_WIDE - GUIDE * 2.0
	var left := -clear * 0.5
	var right := clear * 0.5
	var top := Shopfronts.OPENING - HOUSING_HIGH - 0.07
	var kerb := StreetGround.KERB_HEIGHT
	var roll := Rng.unit(front.dice, 3)
	var pick := int(Rng.unit(front.dice, 4) * SHUTTERS.size())
	var paint := PAINTS[int(Rng.unit(front.dice, 5) * PAINTS.size())]
	var foot := kerb
	if roll < BOARDED_CHANCE:
		_boarded(kit, front, left, right, top, parts)
		return
	if roll < BOARDED_CHANCE + PROPPED_CHANCE:
		foot = kerb + lerpf(0.35, 0.8, Rng.unit(front.dice, 6))
		_behind(kit, front, left, right, kerb, foot + 0.2, parts)
	elif roll < BOARDED_CHANCE + PROPPED_CHANCE + DOOR_CHANCE and clear > DOOR_WIDE + 2.0:
		var hinge := left if Rng.unit(front.dice, 7) < 0.5 else right - DOOR_WIDE
		_door(kit, front, hinge, kerb, parts)
		if hinge == left:
			left += DOOR_WIDE + GUIDE
		else:
			right -= DOOR_WIDE + GUIDE
	var metres: float = SHUTTERS.values()[pick]
	_shutter(kit, front, left, right, foot, top, parts.shutters[pick], metres, paint)
	_bottom_bar(kit, front, left, right, foot, parts)


static func _shutter(
	kit: CityMesh,
	front: Facades.Tier,
	left: float,
	right: float,
	foot: float,
	top: float,
	material: Material,
	metres: float,
	paint: Color
) -> void:
	kit.quad(
		material,
		front.point(left, foot, RECESS),
		front.point(right, foot, RECESS),
		front.point(right, top, RECESS),
		front.point(left, top, RECESS),
		front.facing,
		paint,
		1.0 / metres,
		Vector2(Rng.unit(front.dice, 8), Rng.unit(front.dice, 9))
	)


## The heavy angle along a shutter's foot, with the hasps it's padlocked down by.
static func _bottom_bar(
	kit: CityMesh, front: Facades.Tier, left: float, right: float, foot: float, parts: Parts
) -> void:
	var middle := (left + right) * 0.5
	var wide := right - left
	var tint := Color(0.5, 0.5, 0.5)
	kit.box(
		parts.rust, front.place(middle, foot + 0.04, RECESS + 0.02), Vector3(wide, 0.08, 0.05), tint
	)
	for end: float in [-0.32, 0.32]:
		var x := middle + wide * end
		var hasp := Vector3(0.05, 0.14, 0.04)
		kit.box(parts.metal, front.place(x, foot + 0.06, RECESS + 0.06), hasp, tint)
		var lock := Vector3(0.06, 0.07, 0.03)
		kit.box(
			parts.metal, front.place(x, foot + 0.02, RECESS + 0.09), lock, Color(0.25, 0.25, 0.25)
		)


## The room a raised shutter gives onto, with a lamp still on somewhere further in.
static func _behind(
	kit: CityMesh,
	front: Facades.Tier,
	left: float,
	right: float,
	kerb: float,
	top: float,
	parts: Parts
) -> void:
	var back := RECESS - 1.6
	kit.quad(
		parts.leak,
		front.point(left, kerb, back),
		front.point(right, kerb, back),
		front.point(right, top, back),
		front.point(left, top, back),
		front.facing,
		LEAK
	)
	kit.quad(
		parts.dark,
		front.point(left, kerb, RECESS),
		front.point(right, kerb, RECESS),
		front.point(right, kerb, back),
		front.point(left, kerb, back),
		Vector3.UP,
		Color(0.12, 0.1, 0.08)
	)


## A steel door let into the front for whoever still has business in there.
static func _door(
	kit: CityMesh, front: Facades.Tier, hinge: float, kerb: float, parts: Parts
) -> void:
	var middle := hinge + DOOR_WIDE * 0.5
	var top := kerb + DOOR_HIGH
	var tint := Color(0.6, 0.62, 0.62)
	# Frame and overpanel, all the way up to the housing.
	var over := Shopfronts.OPENING - HOUSING_HIGH - 0.07 - top
	kit.box(
		parts.metal,
		front.place(middle, top + over * 0.5, RECESS),
		Vector3(DOOR_WIDE, over, 0.06),
		tint * 0.8
	)
	kit.box(
		parts.metal,
		front.place(middle, kerb + DOOR_HIGH * 0.5, RECESS - 0.08),
		Vector3(DOOR_WIDE - 0.1, DOOR_HIGH - 0.05, 0.05),
		tint
	)
	for end: float in [0.0, DOOR_WIDE]:
		kit.box(
			parts.metal,
			front.place(hinge + end, kerb + DOOR_HIGH * 0.5, RECESS),
			Vector3(0.07, DOOR_HIGH, 0.12),
			tint * 0.6
		)
	kit.box(
		parts.metal, front.place(middle, top, RECESS), Vector3(DOOR_WIDE, 0.07, 0.12), tint * 0.6
	)
	# Kick plate, pull, and a louvre low down for the air.
	kit.box(
		parts.rust,
		front.place(middle, kerb + 0.16, RECESS - 0.05),
		Vector3(DOOR_WIDE - 0.14, 0.26, 0.01),
		tint
	)
	kit.box(
		parts.metal,
		front.place(hinge + DOOR_WIDE - 0.14, kerb + 1.05, RECESS - 0.03),
		Vector3(0.03, 0.22, 0.04),
		Color(0.2, 0.2, 0.2)
	)
	var slat := kerb + 0.42
	while slat < kerb + 0.72:
		kit.box(
			parts.metal,
			front.place(middle, slat, RECESS - 0.045),
			Vector3(0.5, 0.02, 0.02),
			Color(0.25, 0.25, 0.25)
		)
		slat += 0.05


## Sheets of whatever was to hand, overlapping and not quite square, on a couple of rails.
static func _boarded(
	kit: CityMesh, front: Facades.Tier, left: float, right: float, top: float, parts: Parts
) -> void:
	var x := left
	var index := 0
	while x < right - 0.01:
		var dice := Rng.derive(front.dice, [20, index])
		var wide := minf(lerpf(0.9, 1.35, Rng.unit(dice, 0)), right - x)
		var skew := (Rng.unit(dice, 1) - 0.5) * 0.08
		var out := RECESS + 0.02 + (index % 2) * 0.025
		var tint := Color.WHITE.darkened(Rng.unit(dice, 2) * 0.35)
		var lean := Rng.unit(dice, 3) * 0.04
		kit.quad(
			parts.sheeting,
			front.point(x, 0.0, out),
			front.point(minf(x + wide + 0.05, right), 0.0, out),
			front.point(minf(x + wide + 0.05, right) + skew, top, out + lean),
			front.point(x + skew, top, out + lean),
			front.facing,
			tint,
			1.0 / SHEETING_METRES,
			Vector2(Rng.unit(dice, 4), Rng.unit(dice, 5))
		)
		x += wide
		index += 1
	var middle := (left + right) * 0.5
	for y: float in [0.45, top - 0.35]:
		kit.box(
			parts.rust,
			front.place(middle, y, RECESS + 0.09),
			Vector3(right - left, 0.09, 0.05),
			Color(0.6, 0.6, 0.6)
		)


static func _parts() -> Parts:
	var parts := Parts.new()
	var path := "res://assets/polyhaven/%s/%s_%%s_2k.jpg" % [CONCRETE, CONCRETE]
	parts.concrete = ShaderMaterial.new()
	parts.concrete.shader = BRUTAL
	parts.concrete.set_shader_parameter("albedo", load(path % "diffuse"))
	parts.concrete.set_shader_parameter("normals", load(path % "nor_gl"))
	parts.concrete.set_shader_parameter("rough", load(path % "rough"))
	parts.concrete.set_shader_parameter("occlusion", load(path % "ao"))
	parts.concrete.set_shader_parameter("tile", CONCRETE_METRES)
	for scan: String in SHUTTERS:
		parts.shutters.append(_steel(scan))
	parts.sheeting = _steel(SHEETING)
	parts.metal = Facades.scan("rusty_painted_metal", 1.2, Color(0.8, 0.8, 0.8))
	parts.rust = Facades.scan("rusty_metal_02", 1.0, Color(0.9, 0.85, 0.8))
	parts.leak = StandardMaterial3D.new()
	parts.leak.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	parts.leak.vertex_color_use_as_albedo = true
	parts.dark = StandardMaterial3D.new()
	parts.dark.vertex_color_use_as_albedo = true
	parts.dark.roughness = 0.4
	return parts


static func _steel(scan: String) -> ShaderMaterial:
	var path := "res://assets/polyhaven/%s/%s_%%s_2k.jpg" % [scan, scan]
	var material := ShaderMaterial.new()
	material.shader = STEEL
	material.set_shader_parameter("albedo", load(path % "diffuse"))
	material.set_shader_parameter("normals", load(path % "nor_gl"))
	material.set_shader_parameter("rough", load(path % "rough"))
	material.set_shader_parameter("occlusion", load(path % "ao"))
	return material
