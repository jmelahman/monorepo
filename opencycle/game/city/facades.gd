class_name Facades
extends RefCounted
## The buildings above the shops, as brutalist slabs in real geometry: deep fins, ribbon
## windows, egg-crate frames and near-blank monoliths in raw concrete, some with floors
## stepping out over the pavement. Nearly every window is dark, and what's been bolted on
## since is rusting: drains, conduit, cable, louvres, air-con and steel over the panes. Each
## building is lit from close by, so its fins and ledges throw long shadows up its own face.

const CONCRETE: Shader = preload("res://game/brutal.gdshader")
const WINDOW: Shader = preload("res://game/brutal_window.gdshader")
const FINISHES: Array[Array] = [
	["wood_textured_concrete", 2.4],
	["ribbed_concrete_wall", 3.0],
	["preconcrete_wall_001", 4.0],
	["concrete_slab_wall", 4.0],
]
const AIRCON := "res://assets/polyhaven/exterior_aircon_unit/exterior_aircon_unit_1k.gltf"
const GROUND_FLOOR := 4.1
const BELOW := 8.0
## The heavy beam that carries the building over the shops.
const BEAM_FROM := 4.4
const BEAM_HIGH := 1.15
## Floors in a tier, when the building steps out over the pavement.
const TIER_FLOORS := 3
## Past the deck nobody will see them, so they stop being built floor by floor.
const DETAIL_TOP := 40.0
const LIT_CHANCE := 0.12
const BEVEL := 0.05
## How far the lights either end of the street reach back and ahead of the ride.
const LIT_BEHIND := 15.0
const LIT_AHEAD := 45.0
const COLD_WASH := Color(0.52, 0.72, 0.95)
const WARM_WASH := Color(1.0, 0.74, 0.52)
const GLASS_FRAME := 0.07
const MULLIONS := 1.25
const LIGHTS: Array[Color] = [
	Color(0.55, 0.8, 0.85),
	Color(0.7, 0.85, 0.8),
	Color(1.0, 0.55, 0.22),
	Color(0.5, 0.6, 0.95),
]


class Materials:
	extends RefCounted
	var concrete: Array[ShaderMaterial] = []
	var glass: ShaderMaterial
	var metal: StandardMaterial3D
	var rust: StandardMaterial3D
	var corrugated: StandardMaterial3D
	var lamp: StandardMaterial3D


## Where a tier of a building is, so the forms can lay things out on it.
class Tier:
	extends RefCounted
	var plane := Vector3.ZERO
	var along := Vector3.ZERO
	var facing := Vector3.ZERO
	var basis := Basis.IDENTITY
	var width := 0.0
	var low := 0.0
	var high := 0.0
	var floor_height := 0.0
	var tone := Color.WHITE
	var ledge := Vector2.ZERO
	var dice := 0

	func point(across: float, up: float, out := 0.0) -> Vector3:
		return plane + along * across + Vector3.UP * up + facing * out

	func place(across: float, up: float, out: float) -> Transform3D:
		return Transform3D(basis, point(across, up, out))


static func build(route: Route, street: Street) -> Node3D:
	var root := Node3D.new()
	root.name = "Facades"
	var kit := CityMesh.new()
	var materials := _materials()
	var units: Array[Transform3D] = []
	for tower: Street.Tower in street.towers:
		kit.at(tower.s)
		_tower(kit, route, tower, materials, units)
		if tower.s > -LIT_BEHIND and tower.s < NeonBiome.LENGTH + LIT_AHEAD:
			_light(kit, root, route, tower, materials)
	kit.commit(root)
	Furniture.instances(root, AIRCON, units)
	return root


static func _tower(
	kit: CityMesh, route: Route, tower: Street.Tower, used: Materials, units: Array[Transform3D]
) -> void:
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var facing := -outward
	var base := route.at(tower.s)
	var front := base + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	front.y = base.y
	var concrete := used.concrete[tower.facade]
	var tone := Color(tower.tint.r, tower.tint.g, tower.tint.b, tower.age)
	var across := Vector3.UP.cross(facing).normalized()

	# The ends and the back of the mass. The fronts of the floors go on it.
	var half := along * tower.width * 0.5
	var back := outward * tower.depth
	var left := front - half
	var right := front + half
	var low := Vector3.UP * -BELOW
	var high := Vector3.UP * tower.height
	kit.quad(concrete, left + back + low, left + low, left + high, left + back + high, -along, tone)
	kit.quad(
		concrete, right + low, right + back + low, right + back + high, right + high, along, tone
	)
	kit.quad(
		concrete,
		left + high,
		right + high,
		right + back + high,
		left + back + high,
		Vector3.UP,
		tone
	)

	var floor_height := tower.storey * 1.15
	var beam := Tier.new()
	beam.plane = front
	beam.along = along
	beam.facing = facing
	beam.basis = Basis(across, Vector3.UP, facing)
	beam.width = tower.width
	beam.tone = tone
	beam.ledge = Vector2(floor_height, front.y + BEAM_FROM + BEAM_HIGH)
	_solid(
		kit,
		concrete,
		beam,
		Vector3(0.0, BEAM_FROM + BEAM_HIGH * 0.5, 0.05),
		Vector3(tower.width, BEAM_HIGH, 1.3),
		tone
	)
	# Up to the beam, there's only the wall behind the shops' signs.
	kit.quad(
		concrete,
		beam.point(-tower.width * 0.5, GROUND_FLOOR),
		beam.point(tower.width * 0.5, GROUND_FLOOR),
		beam.point(tower.width * 0.5, BEAM_FROM),
		beam.point(-tower.width * 0.5, BEAM_FROM),
		facing,
		tone
	)

	var bottom := BEAM_FROM + BEAM_HIGH
	var tier_index := 0
	var top := minf(tower.height, DETAIL_TOP)
	while bottom < top:
		var tier := Tier.new()
		var out := minf(tower.jetty, tier_index * tower.jetty * 0.5)
		tier.plane = front + facing * out
		tier.along = along
		tier.facing = facing
		tier.basis = beam.basis
		tier.width = tower.width
		tier.low = bottom
		tier.high = minf(bottom + floor_height * TIER_FLOORS, top)
		tier.floor_height = floor_height
		tier.tone = tone
		tier.ledge = beam.ledge
		tier.dice = Rng.derive(tower.dice, [tier_index])
		if out > 0.0:
			# The soffit where this tier steps out past the one below.
			_solid(
				kit,
				concrete,
				tier,
				Vector3(0.0, tier.low + 0.2, -0.9),
				Vector3(tier.width, 0.4, 2.0),
				tone * Color(0.8, 0.8, 0.8, 1.0)
			)
		match tower.form:
			0:
				_fins(kit, tier, tower, used, units)
			1:
				_ribbons(kit, tier, tower, used)
			2:
				_egg_crate(kit, tier, tower, used, units)
			_:
				_monolith(kit, tier, tower, used)
		if tier_index > 0 and tower.form != 0 and tower.form != 2:
			_pod(kit, tier, used)
		bottom = tier.high
		tier_index += 1
	# Whatever's left above the detail, as plain wall running up through the deck.
	var last := front + facing * minf(tower.jetty, (tier_index - 1) * tower.jetty * 0.5)
	kit.quad(
		concrete,
		last - half + Vector3.UP * top,
		last + half + Vector3.UP * top,
		last + half + high,
		last - half + high,
		facing,
		tone
	)
	# Piers at the ends, which split it from its neighbours and hide the steps in plan.
	var deepest := minf(tower.jetty, (tier_index - 1) * tower.jetty * 0.5)
	for end: float in [-1.0, 1.0]:
		_solid(
			kit,
			concrete,
			beam,
			Vector3(
				end * (tower.width * 0.5 - 0.35), BEAM_FROM + (top - BEAM_FROM) * 0.5, deepest - 0.9
			),
			Vector3(0.7, top - BEAM_FROM, 2.4),
			tone,
			0.09
		)
	_services(kit, beam, tower, used, top)


## Tall fins between the bays with thin floors between them. Throws long shadows.
static func _fins(
	kit: CityMesh, tier: Tier, tower: Street.Tower, used: Materials, units: Array[Transform3D]
) -> void:
	var concrete := used.concrete[tower.facade]
	_backing(kit, tier, concrete, -0.75)
	_slabs(kit, tier, concrete, 0.34, 0.75, -0.3)
	var bays := _bays(tier, tower.bay * 1.3)
	var wide := tier.width / bays
	for bay: int in range(0, bays + 1):
		var at := -tier.width * 0.5 + bay * wide
		var middle := (tier.low + tier.high) * 0.5
		var tall := tier.high - tier.low
		_solid(kit, concrete, tier, Vector3(at, middle, 0.0), Vector3(0.34, tall, 1.5), tier.tone)
		# Stepped, wider at the root, so the shadow it throws has two edges to it.
		_solid(kit, concrete, tier, Vector3(at, middle, -0.5), Vector3(0.62, tall, 0.55), tier.tone)
	_panes(kit, tier, used, bays, 0.34, -0.7, 0.3, 0.3)
	_balconies(kit, tier, concrete, used, bays, 0.05, 0.2)
	_hang_units(tier, bays, wide, -0.3, tower.age, units)


## Heavy bands of spandrel, with a long slot of window under each.
static func _ribbons(kit: CityMesh, tier: Tier, tower: Street.Tower, used: Materials) -> void:
	var concrete := used.concrete[tower.facade]
	_backing(kit, tier, concrete, -1.0)
	var y := tier.low
	while y < tier.high - 0.01:
		var band := tier.floor_height * 0.56
		var top := minf(y + band, tier.high)
		_solid(
			kit,
			concrete,
			tier,
			Vector3(0.0, (y + top) * 0.5, -0.2),
			Vector3(tier.width, top - y, 0.5),
			tier.tone
		)
		# A cast nose along the top of the band, with the drip under it.
		_solid(
			kit,
			concrete,
			tier,
			Vector3(0.0, top - 0.09, 0.1),
			Vector3(tier.width, 0.18, 0.2),
			tier.tone
		)
		y += tier.floor_height
	var bays := _bays(tier, 3.8)
	var wide := tier.width / bays
	# Short nibs between the windows, so the slot isn't one long dark line.
	for bay: int in range(0, bays + 1):
		var at := -tier.width * 0.5 + bay * wide
		var level := tier.low + tier.floor_height * 0.56
		while level < tier.high - 0.01:
			var head := minf(level + tier.floor_height * 0.44, tier.high)
			_solid(
				kit,
				concrete,
				tier,
				Vector3(at, (level + head) * 0.5, -0.65),
				Vector3(0.28, head - level, 0.65),
				tier.tone
			)
			level += tier.floor_height
	_panes(kit, tier, used, bays, 0.14, -0.95, tier.floor_height * 0.56, 0.0)
	_spalls(kit, tier, concrete, used, 0.05)


## Deep frames on every window, the grid doing all the work.
static func _egg_crate(
	kit: CityMesh, tier: Tier, tower: Street.Tower, used: Materials, units: Array[Transform3D]
) -> void:
	var concrete := used.concrete[tower.facade]
	_backing(kit, tier, concrete, -1.1)
	_slabs(kit, tier, concrete, 0.42, 1.2, -0.5)
	var bays := _bays(tier, tower.bay)
	var wide := tier.width / bays
	for bay: int in range(0, bays + 1):
		var at := -tier.width * 0.5 + bay * wide
		_solid(
			kit,
			concrete,
			tier,
			Vector3(at, (tier.low + tier.high) * 0.5, -0.5),
			Vector3(0.3, tier.high - tier.low, 1.2),
			tier.tone
		)
	_panes(kit, tier, used, bays, 0.15, -1.05, 0.42, 0.55)
	_balconies(kit, tier, concrete, used, bays, -0.02, 0.3)
	_hang_units(tier, bays, wide, -0.9, tower.age, units)


## A wall and nothing else, with the odd slit in it and a louvre where the plant is.
static func _monolith(kit: CityMesh, tier: Tier, tower: Street.Tower, used: Materials) -> void:
	var concrete := used.concrete[tower.facade]
	_backing(kit, tier, concrete, 0.0)
	# Shadow gaps cast into it, a storey apart, for something to read at this scale.
	var y := tier.low + tier.floor_height
	while y < tier.high:
		_solid(
			kit,
			concrete,
			tier,
			Vector3(0.0, y, 0.03),
			Vector3(tier.width, 0.12, 0.1),
			tier.tone,
			0.02
		)
		y += tier.floor_height
	_louvres(kit, tier, concrete, used)
	_spalls(kit, tier, concrete, used, 0.0)
	var slots := maxi(int(tier.width / 4.5), 1)
	for slot: int in slots:
		var dice := Rng.derive(tier.dice, [slot])
		if Rng.unit(dice, 0) < 0.55:
			continue
		var at := (float(slot) + 0.5) / slots * tier.width - tier.width * 0.5
		var high_part := tier.high - 1.0
		var low_part := tier.low + 1.0
		if high_part - low_part < 1.0:
			continue
		var half := 0.28
		_opening(
			kit,
			tier,
			concrete,
			used,
			at - half,
			at + half,
			low_part,
			high_part,
			0.0,
			0.4,
			Rng.derive(dice, [1])
		)


## The wall behind whatever the form puts in front of it.
static func _backing(kit: CityMesh, tier: Tier, concrete: Material, out: float) -> void:
	var half := tier.width * 0.5
	kit.quad(
		concrete,
		tier.point(-half, tier.low, out),
		tier.point(half, tier.low, out),
		tier.point(half, tier.high, out),
		tier.point(-half, tier.high, out),
		tier.facing,
		tier.tone * Color(0.75, 0.75, 0.75, 1.0)
	)


static func _slabs(
	kit: CityMesh, tier: Tier, concrete: Material, thick: float, deep: float, out: float
) -> void:
	var y := tier.low + thick * 0.5
	while y < tier.high:
		_solid(
			kit, concrete, tier, Vector3(0.0, y, out), Vector3(tier.width, thick, deep), tier.tone
		)
		# The drip along the front edge, which is where the stains begin.
		_solid(
			kit,
			concrete,
			tier,
			Vector3(0.0, y - thick * 0.5 - 0.05, out + deep * 0.5 - 0.09),
			Vector3(tier.width, 0.1, 0.12),
			tier.tone,
			0.02
		)
		y += tier.floor_height


## Anything big enough to be looked at closely, with worn edges and stains from the ledges.
static func _solid(
	kit: CityMesh,
	material: Material,
	tier: Tier,
	centre: Vector3,
	size: Vector3,
	tone: Color,
	bevel := BEVEL,
) -> void:
	var place := tier.place(centre.x, centre.y, centre.z)
	kit.block(material, place, size, tone, bevel, tier.ledge)


static func _bays(tier: Tier, wanted: float) -> int:
	return maxi(roundi(tier.width / wanted), 1)


## One pane to each bay of each floor, set back to [param out].
static func _panes(
	kit: CityMesh,
	tier: Tier,
	used: Materials,
	bays: int,
	inset: float,
	out: float,
	from_floor: float,
	from_head: float
) -> void:
	var wide := tier.width / bays
	var floors := maxi(roundi((tier.high - tier.low) / tier.floor_height), 1)
	var tall := (tier.high - tier.low) / floors
	for level: int in floors:
		for bay: int in bays:
			var left := -tier.width * 0.5 + bay * wide + inset
			var right := left + wide - inset * 2.0
			var low := tier.low + level * tall + from_floor
			var high := tier.low + (level + 1) * tall - from_head
			if high <= low:
				continue
			_pane(kit, tier, used, left, right, low, high, out, Rng.derive(tier.dice, [bay, level]))


## A slit cut into a solid wall, with its reveal built, not just the glass.
static func _opening(
	kit: CityMesh,
	tier: Tier,
	concrete: Material,
	used: Materials,
	left: float,
	right: float,
	low: float,
	high: float,
	out: float,
	deep: float,
	dice: int
) -> void:
	# Proud frame around the slit rather than a hole, which comes to the same in shadow.
	var mid := (low + high) * 0.5
	var tall := high - low
	var tone := tier.tone
	var front := out + deep * 0.5
	var size := Vector3(0.2, tall, deep)
	_solid(kit, concrete, tier, Vector3(left - 0.1, mid, front), size, tone)
	_solid(kit, concrete, tier, Vector3(right + 0.1, mid, front), size, tone)
	var wide := right - left + 0.4
	var middle := (left + right) * 0.5
	var across := Vector3(wide, 0.2, deep)
	_solid(kit, concrete, tier, Vector3(middle, high + 0.1, front), across, tone)
	_solid(kit, concrete, tier, Vector3(middle, low - 0.1, front), across, tone)
	_pane(kit, tier, used, left, right, low, high, out + 0.02, dice)


static func _pane(
	kit: CityMesh,
	tier: Tier,
	used: Materials,
	left: float,
	right: float,
	low: float,
	high: float,
	out: float,
	dice: int
) -> void:
	var a := tier.point(left, low, out)
	var b := tier.point(right, low, out)
	var c := tier.point(right, high, out)
	var d := tier.point(left, high, out)
	var roll := Rng.unit(dice, 0)
	var middle := (left + right) * 0.5
	# The sill, thrown out past the glass, whatever's been done to the window since.
	var sill := Vector3(middle, low - 0.05, out + 0.16)
	_solid(
		kit, used.metal, tier, sill, Vector3(right - left, 0.1, 0.34), Color(0.3, 0.3, 0.3), 0.02
	)
	if roll < 0.06:
		# Steel over a pane that was broken years ago, gone to rust.
		kit.quad(used.rust, a, b, c, d, tier.facing)
		return
	if roll < 0.11:
		kit.quad(used.corrugated, a, b, c, d, tier.facing)
		return
	var colour := Color(0.0, 0.0, 0.0, 0.0)
	if roll > 1.0 - LIT_CHANCE:
		colour = LIGHTS[int(Rng.unit(dice, 1) * LIGHTS.size())]
		colour.a = lerpf(0.35, 1.1, Rng.unit(dice, 2))
	kit.quad(used.glass, a, b, c, d, tier.facing, colour)
	_frame(kit, tier, used, left, right, low, high, out)


## A steel frame standing off the glass, with a bar every so often to break up the pane.
static func _frame(
	kit: CityMesh,
	tier: Tier,
	used: Materials,
	left: float,
	right: float,
	low: float,
	high: float,
	out: float
) -> void:
	var tint := Color(0.16, 0.16, 0.16)
	var front := out + 0.05
	var wide := right - left
	var tall := high - low
	var mid := (low + high) * 0.5
	var flat := Vector3(wide, GLASS_FRAME, 0.1)
	var post := Vector3(GLASS_FRAME, tall, 0.1)
	kit.box(used.metal, tier.place((left + right) * 0.5, high, front), flat, tint)
	kit.box(used.metal, tier.place((left + right) * 0.5, low, front), flat, tint)
	kit.box(used.metal, tier.place(left, mid, front), post, tint)
	kit.box(used.metal, tier.place(right, mid, front), post, tint)
	var bar := left + MULLIONS
	while bar < right - 0.3:
		kit.box(used.metal, tier.place(bar, mid, front), post, tint)
		bar += MULLIONS


## A low wall across some bays, where a window is a balcony, with a rusting rail along it.
static func _balconies(
	kit: CityMesh,
	tier: Tier,
	concrete: Material,
	used: Materials,
	bays: int,
	out: float,
	chance: float
) -> void:
	var wide := tier.width / bays
	var floors := maxi(roundi((tier.high - tier.low) / tier.floor_height), 1)
	var tall := (tier.high - tier.low) / floors
	for level: int in floors:
		for bay: int in bays:
			var dice := Rng.derive(tier.dice, [bay, level, 11])
			if Rng.unit(dice, 0) > chance:
				continue
			var at := -tier.width * 0.5 + (bay + 0.5) * wide
			var floor_y := tier.low + level * tall
			var size := Vector3(wide - 0.34, 0.95, 0.14)
			_solid(kit, concrete, tier, Vector3(at, floor_y + 0.475, out), size, tier.tone, 0.03)
			var rail := floor_y + 1.2
			var half := (wide - 0.4) * 0.5
			kit.bar(
				used.rust, tier.point(at - half, rail, out), tier.point(at + half, rail, out), 0.035
			)
			var post := -half
			while post <= half:
				kit.bar(
					used.rust,
					tier.point(at + post, floor_y + 0.95, out),
					tier.point(at + post, rail, out),
					0.025
				)
				post += 0.9


## A panel of real slats in a concrete surround, where the plant behind it breathes.
static func _louvres(kit: CityMesh, tier: Tier, concrete: Material, used: Materials) -> void:
	if Rng.unit(tier.dice, 40) < 0.45 or tier.width < 7.0:
		return
	var wide := lerpf(2.2, 4.2, Rng.unit(tier.dice, 41))
	var tall := minf(tier.floor_height * 1.5, tier.high - tier.low - 0.6)
	var at := (Rng.unit(tier.dice, 42) - 0.5) * (tier.width - wide - 2.0)
	var low := tier.low + (tier.high - tier.low - tall) * 0.5
	var mid := low + tall * 0.5
	var surround := Vector3(0.18, tall + 0.36, 0.3)
	var lintel := Vector3(wide, 0.18, 0.3)
	_solid(kit, concrete, tier, Vector3(at - wide * 0.5, mid, 0.1), surround, tier.tone, 0.02)
	_solid(kit, concrete, tier, Vector3(at + wide * 0.5, mid, 0.1), surround, tier.tone, 0.02)
	_solid(kit, concrete, tier, Vector3(at, low - 0.09, 0.1), lintel, tier.tone, 0.02)
	_solid(kit, concrete, tier, Vector3(at, low + tall + 0.09, 0.1), lintel, tier.tone, 0.02)
	kit.box(
		used.metal, tier.place(at, mid, 0.01), Vector3(wide, tall, 0.02), Color(0.03, 0.03, 0.03)
	)
	var slat := low + 0.08
	var tilt := tier.basis * Basis(Vector3.RIGHT, 0.7)
	while slat < low + tall:
		var place := Transform3D(tilt, tier.point(at, slat, 0.12))
		kit.box(used.metal, place, Vector3(wide - 0.05, 0.015, 0.2), Color(0.3, 0.29, 0.27))
		slat += 0.16


## A room hung off the front of the building on brackets, with one slot of window in it.
static func _pod(kit: CityMesh, tier: Tier, used: Materials) -> void:
	if Rng.unit(tier.dice, 50) > 0.28 or tier.width < 9.0 or tier.high - tier.low < 6.0:
		return
	var concrete := used.concrete[0]
	var wide := lerpf(2.6, 4.2, Rng.unit(tier.dice, 51))
	var tall := tier.floor_height * 1.1
	var deep := lerpf(1.2, 2.1, Rng.unit(tier.dice, 52))
	var at := (Rng.unit(tier.dice, 53) - 0.5) * (tier.width - wide - 2.0)
	var low := tier.low + tier.floor_height * 0.6
	var centre := Vector3(at, low + tall * 0.5, deep * 0.5)
	_solid(kit, concrete, tier, centre, Vector3(wide, tall, deep), tier.tone, 0.07)
	_pane(
		kit,
		tier,
		used,
		at - wide * 0.4,
		at + wide * 0.4,
		low + 0.9,
		low + 2.0,
		deep + 0.01,
		tier.dice
	)
	# Brackets back to the wall, and water's way down off it.
	for side: float in [-0.38, 0.38]:
		var x := at + wide * side
		kit.bar(used.rust, tier.point(x, low, deep - 0.2), tier.point(x, low - 1.5, 0.05), 0.09)
	var drain := at + wide * 0.45
	kit.bar(used.rust, tier.point(drain, low, deep * 0.5), tier.point(drain, low - 1.0, 0.1), 0.05)


## Where the cover has blown off and the reinforcing shows, gone to rust.
static func _spalls(
	kit: CityMesh, tier: Tier, concrete: Material, used: Materials, out: float
) -> void:
	for index: int in 2:
		var dice := Rng.derive(tier.dice, [60, index])
		if Rng.unit(dice, 0) > 0.5:
			continue
		var wide := lerpf(0.5, 1.7, Rng.unit(dice, 1))
		var tall := lerpf(0.3, 0.8, Rng.unit(dice, 2))
		var at := (Rng.unit(dice, 3) - 0.5) * (tier.width - wide - 1.0)
		var y := lerpf(tier.low + 0.5, tier.high - 0.5, Rng.unit(dice, 4))
		var hole := tier.tone * Color(0.3, 0.3, 0.3, 1.0)
		_solid(
			kit, concrete, tier, Vector3(at, y, out + 0.01), Vector3(wide, tall, 0.02), hole, 0.0
		)
		var bar := y - tall * 0.3
		while bar < y + tall * 0.4:
			kit.bar(
				used.rust,
				tier.point(at - wide * 0.45, bar, out + 0.04),
				tier.point(at + wide * 0.45, bar, out + 0.04),
				0.018
			)
			bar += 0.16


static func _hang_units(
	tier: Tier, bays: int, wide: float, out: float, age: float, units: Array[Transform3D]
) -> void:
	var floors := maxi(roundi((tier.high - tier.low) / tier.floor_height), 1)
	for level: int in floors:
		for bay: int in bays:
			var dice := Rng.derive(tier.dice, [bay, level, 7])
			if Rng.unit(dice, 0) > lerpf(0.02, 0.14, age):
				continue
			var at := -tier.width * 0.5 + (bay + 0.5) * wide
			var y := tier.low + level * tier.floor_height + 0.5
			units.append(
				Transform3D(Basis(tier.along, Vector3.UP, tier.facing), tier.point(at, y, out))
			)


## What's been run up the front since: drains, conduit, and a duct on the worst of them.
static func _services(
	kit: CityMesh, beam: Tier, tower: Street.Tower, used: Materials, top: float
) -> void:
	var from := BEAM_FROM + BEAM_HIGH
	var runs := 1 + int(Rng.unit(tower.dice, 900) * 3.0)
	for run: int in runs:
		var dice := Rng.derive(tower.dice, [901, run])
		var at := (Rng.unit(dice, 0) - 0.5) * (tower.width - 2.0)
		var out := tower.jetty + 0.14
		var thick := lerpf(0.05, 0.16, Rng.unit(dice, 1))
		var ends := lerpf(from + 6.0, top, Rng.unit(dice, 2))
		kit.bar(used.rust, beam.point(at, 0.4, 0.14), beam.point(at, from, 0.14), thick)
		kit.bar(used.rust, beam.point(at, from, out), beam.point(at, ends, out), thick)
		kit.bar(used.rust, beam.point(at, from, 0.14), beam.point(at, from, out), thick)
		# Held on by brackets, a storey or so apart, each with a collar round the pipe.
		var clip := from + 1.0
		while clip < ends:
			kit.bar(used.metal, beam.point(at, clip, out - 0.1), beam.point(at, clip, out), 0.03)
			var band := beam.place(at, clip, out)
			var ring := Vector3(thick + 0.04, 0.05, thick + 0.04)
			kit.box(used.metal, band, ring, Color(0.2, 0.2, 0.2))
			clip += 2.6
	_cables(kit, beam, tower, used)
	if tower.age > 0.65 and tower.width > 12.0:
		var at := (Rng.unit(tower.dice, 902) - 0.5) * (tower.width - 4.0)
		var out := tower.jetty + 0.4
		kit.box(
			used.metal,
			beam.place(at, (from + top) * 0.5, out),
			Vector3(1.1, top - from, 0.7),
			Color(0.5, 0.48, 0.45)
		)


## Loose runs of cable along the front of the building, sagging between their fixings.
static func _cables(kit: CityMesh, beam: Tier, tower: Street.Tower, used: Materials) -> void:
	var bundles := 1 + int(Rng.unit(tower.dice, 910) * 2.0)
	var black := Color(0.04, 0.04, 0.04)
	for bundle: int in bundles:
		var dice := Rng.derive(tower.dice, [911, bundle])
		var y := lerpf(6.2, 11.0, Rng.unit(dice, 0))
		var out := tower.jetty + 0.2
		var from := -tower.width * 0.5 + 0.5
		var strands := 2 + int(Rng.unit(dice, 1) * 3.0)
		var fixing := from
		var span := lerpf(2.5, 4.5, Rng.unit(dice, 2))
		while fixing < tower.width * 0.5 - 0.5:
			var next := minf(fixing + span, tower.width * 0.5 - 0.5)
			kit.box(used.metal, beam.place(fixing, y, out - 0.05), Vector3(0.08, 0.12, 0.1), black)
			for strand: int in strands:
				var drop := y - strand * 0.045
				var hang := 0.09 + 0.05 * strand
				var mid := (fixing + next) * 0.5
				var a := beam.point(fixing, drop, out)
				var m := beam.point(mid, drop - hang, out)
				var b := beam.point(next, drop, out)
				kit.bar(used.metal, a, m, 0.016, black)
				kit.bar(used.metal, m, b, 0.016, black)
			fixing = next
			if next >= tower.width * 0.5 - 0.5:
				break


## The lights that give the building its depth: one low on its front, raking up it and
## throwing every fin and ledge's shadow long, and bulkheads under the beam for the shutters.
static func _light(
	kit: CityMesh, root: Node3D, route: Route, tower: Street.Tower, used: Materials
) -> void:
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var front := route.at(tower.s) + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	var warm := Rng.unit(tower.dice, 920) < 0.3
	var colour := WARM_WASH if warm else COLD_WASH
	var rake := SpotLight3D.new()
	rake.light_color = colour
	rake.light_energy = lerpf(14.0, 26.0, Rng.unit(tower.dice, 921))
	rake.light_specular = 0.25
	rake.light_volumetric_fog_energy = 0.3
	rake.spot_range = 30.0
	rake.spot_angle = 74.0
	rake.spot_attenuation = 0.7
	rake.shadow_enabled = true
	var drift := (Rng.unit(tower.dice, 922) - 0.5) * tower.width * 0.5
	var source := front - outward * (tower.jetty + 1.6) + along * drift + Vector3.UP * 5.9
	var target := front + along * drift + Vector3.UP * 15.0
	rake.basis = Basis.looking_at(target - source, along)
	rake.position = source
	root.add_child(rake)
	# A bulkhead over some of the shutters, most of which nobody's replaced.
	var count := maxi(roundi(tower.width / 9.0), 1)
	for index: int in count:
		var dice := Rng.derive(tower.dice, [923, index])
		if Rng.unit(dice, 0) < 0.45:
			continue
		var at := ((index + 0.5) / count - 0.5) * tower.width
		var fitting := front - outward * 0.75 + along * at + Vector3.UP * (BEAM_FROM - 0.05)
		kit.at(tower.s)
		var shell := Transform3D(Basis(along, Vector3.UP, outward), fitting)
		kit.box(used.metal, shell, Vector3(0.5, 0.1, 0.2), Color(0.12, 0.12, 0.12))
		kit.box(
			used.lamp,
			Transform3D(Basis(along, Vector3.UP, outward), fitting - Vector3.UP * 0.06),
			Vector3(0.42, 0.02, 0.14),
			colour
		)
		var bulkhead := SpotLight3D.new()
		bulkhead.light_color = colour
		bulkhead.light_energy = lerpf(1.5, 3.5, Rng.unit(dice, 1))
		bulkhead.light_specular = 0.3
		bulkhead.light_volumetric_fog_energy = 0.2
		bulkhead.spot_range = 8.0
		bulkhead.spot_angle = 58.0
		bulkhead.basis = Basis.looking_at(Vector3.DOWN + outward * 0.25, along)
		bulkhead.position = fitting - Vector3.UP * 0.1
		root.add_child(bulkhead)


static func _scan(texture: String, metres: float, tint: Color) -> StandardMaterial3D:
	var path := "res://assets/polyhaven/%s/%s_%%s_2k.jpg" % [texture, texture]
	var material := StandardMaterial3D.new()
	material.albedo_texture = load(path % "diffuse")
	material.albedo_color = tint
	material.vertex_color_use_as_albedo = true
	material.normal_enabled = true
	material.normal_texture = load(path % "nor_gl")
	material.roughness_texture = load(path % "rough")
	material.roughness_texture_channel = BaseMaterial3D.TEXTURE_CHANNEL_GREEN
	material.ao_enabled = true
	material.ao_texture = load(path % "ao")
	material.metallic = 0.25
	# Laid on in world metres, since bars and boxes have no useful UVs of their own.
	material.uv1_triplanar = true
	material.uv1_world_triplanar = true
	material.uv1_scale = Vector3.ONE / metres
	return material


static func _materials() -> Materials:
	var used := Materials.new()
	for finish: Array in FINISHES:
		var texture: String = finish[0]
		var tile: float = finish[1]
		var path := "res://assets/polyhaven/%s/%s_%%s_2k.jpg" % [texture, texture]
		var material := ShaderMaterial.new()
		material.shader = CONCRETE
		material.set_shader_parameter("albedo", load(path % "diffuse"))
		material.set_shader_parameter("normals", load(path % "nor_gl"))
		material.set_shader_parameter("rough", load(path % "rough"))
		material.set_shader_parameter("occlusion", load(path % "ao"))
		material.set_shader_parameter("tile", tile)
		used.concrete.append(material)
	used.glass = ShaderMaterial.new()
	used.glass.shader = WINDOW
	used.metal = _scan("rusty_painted_metal", 2.0, Color(0.75, 0.75, 0.75))
	used.rust = _scan("rusty_metal_02", 1.5, Color(0.85, 0.8, 0.75))
	used.corrugated = _scan("rusty_corrugated_iron", 2.5, Color(0.7, 0.68, 0.66))
	used.lamp = StandardMaterial3D.new()
	used.lamp.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	used.lamp.vertex_color_use_as_albedo = true
	used.lamp.albedo_color = Color(3.0, 3.0, 3.0)
	return used
