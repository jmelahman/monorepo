class_name Facades
extends RefCounted
## The buildings above the shops, as brutalist slabs in real geometry: deep fins, ribbon
## windows, egg-crate frames and near-blank monoliths in raw concrete, some with floors
## stepping out over the pavement. Nearly every window is dark, and what's been bolted on
## since is rusting: drains, conduit, air-con and the odd steel plate over a pane.

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
	kit.box(
		concrete,
		beam.place(0.0, BEAM_FROM + BEAM_HIGH * 0.5, 0.05),
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
		tier.dice = Rng.derive(tower.dice, [tier_index])
		if out > 0.0:
			# The soffit where this tier steps out past the one below.
			kit.box(
				concrete,
				tier.place(0.0, tier.low + 0.2, -0.9),
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
		kit.box(
			concrete,
			beam.place(
				end * (tower.width * 0.5 - 0.35), BEAM_FROM + (top - BEAM_FROM) * 0.5, deepest - 0.9
			),
			Vector3(0.7, top - BEAM_FROM, 2.4),
			tone
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
		kit.box(
			concrete,
			tier.place(at, (tier.low + tier.high) * 0.5, 0.0),
			Vector3(0.34, tier.high - tier.low, 1.5),
			tier.tone
		)
	_panes(kit, tier, used, bays, 0.2, -0.7, 0.3, 0.3)
	_hang_units(tier, bays, wide, -0.3, tower.age, units)


## Heavy bands of spandrel, with a long slot of window under each.
static func _ribbons(kit: CityMesh, tier: Tier, tower: Street.Tower, used: Materials) -> void:
	var concrete := used.concrete[tower.facade]
	_backing(kit, tier, concrete, -1.0)
	var y := tier.low
	while y < tier.high - 0.01:
		var band := tier.floor_height * 0.56
		var top := minf(y + band, tier.high)
		kit.box(
			concrete,
			tier.place(0.0, (y + top) * 0.5, -0.2),
			Vector3(tier.width, top - y, 0.5),
			tier.tone
		)
		y += tier.floor_height
	var bays := _bays(tier, 3.8)
	_panes(kit, tier, used, bays, 0.02, -0.95, tier.floor_height * 0.56, 0.0)


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
		kit.box(
			concrete,
			tier.place(at, (tier.low + tier.high) * 0.5, -0.5),
			Vector3(0.3, tier.high - tier.low, 1.2),
			tier.tone
		)
	_panes(kit, tier, used, bays, 0.15, -1.05, 0.42, 0.55)
	_hang_units(tier, bays, wide, -0.9, tower.age, units)


## A wall and nothing else, with the odd slit in it and a louvre where the plant is.
static func _monolith(kit: CityMesh, tier: Tier, tower: Street.Tower, used: Materials) -> void:
	var concrete := used.concrete[tower.facade]
	_backing(kit, tier, concrete, 0.0)
	# Shadow gaps cast into it, a storey apart, for something to read at this scale.
	var y := tier.low + tier.floor_height
	while y < tier.high:
		kit.box(
			used.metal,
			tier.place(0.0, y, 0.015),
			Vector3(tier.width, 0.06, 0.03),
			Color(0.1, 0.1, 0.1)
		)
		y += tier.floor_height
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
		kit.box(concrete, tier.place(0.0, y, out), Vector3(tier.width, thick, deep), tier.tone)
		y += tier.floor_height


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
	kit.box(concrete, tier.place(left - 0.1, mid, out + deep * 0.5), Vector3(0.2, tall, deep), tone)
	kit.box(
		concrete, tier.place(right + 0.1, mid, out + deep * 0.5), Vector3(0.2, tall, deep), tone
	)
	var wide := right - left + 0.4
	kit.box(
		concrete,
		tier.place((left + right) * 0.5, high + 0.1, out + deep * 0.5),
		Vector3(wide, 0.2, deep),
		tone
	)
	kit.box(
		concrete,
		tier.place((left + right) * 0.5, low - 0.1, out + deep * 0.5),
		Vector3(wide, 0.2, deep),
		tone
	)
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
	if roll < 0.1:
		# Steel over a pane that was broken years ago, gone to rust.
		kit.quad(used.rust, a, b, c, d, tier.facing)
		return
	var colour := Color(0.0, 0.0, 0.0, 0.0)
	if roll > 1.0 - LIT_CHANCE:
		colour = LIGHTS[int(Rng.unit(dice, 1) * LIGHTS.size())]
		colour.a = lerpf(0.35, 1.1, Rng.unit(dice, 2))
	kit.quad(used.glass, a, b, c, d, tier.facing, colour)


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
	if tower.age > 0.65 and tower.width > 12.0:
		var at := (Rng.unit(tower.dice, 902) - 0.5) * (tower.width - 4.0)
		var out := tower.jetty + 0.4
		kit.box(
			used.metal,
			beam.place(at, (from + top) * 0.5, out),
			Vector3(1.1, top - from, 0.7),
			Color(0.5, 0.48, 0.45)
		)


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
		material.set_shader_parameter("tile", tile)
		used.concrete.append(material)
	used.glass = ShaderMaterial.new()
	used.glass.shader = WINDOW
	used.metal = StandardMaterial3D.new()
	used.metal.vertex_color_use_as_albedo = true
	used.metal.albedo_color = Color(0.3, 0.3, 0.3)
	used.metal.metallic = 0.6
	used.metal.roughness = 0.55
	used.rust = StandardMaterial3D.new()
	used.rust.albedo_color = Color(0.13, 0.075, 0.05)
	used.rust.metallic = 0.3
	used.rust.roughness = 0.75
	return used
