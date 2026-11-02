class_name ResidentialRow
extends RefCounted
## Where the people who can afford a view live: pale terraces stepping back from the road under an
## open sky, each flat with its lit window, its glass rail and its planter. Nothing to sell, nobody
## shouting, and every so often a stair climbing through the block to the levels above.

const BAY := 6.0
const FRONT := 11.5
const STOREY := 3.3
## How far each level stands behind the one below, which is the terrace on its roof.
const STEP := 3.4
const RECESS := 1.2
const LEVELS_LOW := 6
const LEVELS_HIGH := 11
const BAYS_TO_A_BLOCK := 7
const GROUND_OUT := 54.0
const PALE := Color(0.95, 0.93, 0.88, 0.08)
const SHADE := Color(0.62, 0.62, 0.62, 0.08)
const DARK := Color(0.05, 0.06, 0.07)
const FRAME := Color(0.16, 0.17, 0.18)
## Rooms, warm and mostly dim: a window is what makes a wall a home, but none of them should bloom.
const ROOMS: Array[Color] = [
	Color(0.72, 0.5, 0.28),
	Color(0.62, 0.4, 0.2),
	Color(0.7, 0.58, 0.42),
	Color(0.42, 0.56, 0.6),
]
const ROOM_DARK := 0.38
const ROOM_LIT := 0.8
const WINDOW_HIGH := 2.5
const BLIND_CHANCE := 0.4
const RAIL_HIGH := 1.05
const PLANTER_HIGH := 0.45
const FERN := "res://assets/polyhaven/fern_02/fern_02_1k.gltf"
const FERN_TOLERANCE := 0.03
const FERN_REACH := 110.0
const FERNS_TO_A_BAY := 3
const FERN_SCALE := 0.9
const GARDEN_IN := 8.6
const GARDEN_OUT := 10.8
const GARDEN_HIGH := 0.6
const FOOT_LIGHT := Color(0.85, 0.72, 0.5)
const LIT_EVERY := 3
## The level a second row of lights works from, so the upper terraces aren't left in the dark.
const UPPER_WASH := 3.0
const WASH_ENERGY := 3.0
const WASH_REACH := 18.0
const WASH_HUE := Color(1.0, 0.76, 0.52)


class Used:
	extends RefCounted
	var glass: Material
	var lit: Material
	var room: Material
	var ferns: Kit


static func _used() -> Used:
	var used := Used.new()
	used.glass = Once.made("retail_glass", RetailRow._glass)
	used.lit = Once.made("retail_lit", RetailRow._lit)
	var room: StandardMaterial3D = RetailRow._lit()
	room.albedo_color = Color(ROOM_LIT, ROOM_LIT, ROOM_LIT)
	used.room = room
	used.ferns = Kit.new(FERN, FERN_TOLERANCE)
	return used


## The bays of [param stretch] whose start lies in [param section], and the deck's two ends.
static func build(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	section: StreetSection,
	parts: Quarters.Parts
) -> void:
	var used: Used = Once.made("residential", _used)
	var index := 0
	var s := stretch.from
	while s < stretch.to:
		if section.holds(s):
			kit.at(s)
			used.ferns.at(s)
			var long := minf(BAY, stretch.to - s)
			for side: float in [-1.0, 1.0]:
				_bay(kit, root, route, s, long, side, index, used, parts)
		s += BAY
		index += 1
	for end: float in [stretch.from, stretch.to]:
		if section.holds(end):
			kit.at(end)
			RetailRow._deck_end(kit, route, end, used.lit, parts)
	used.ferns.commit(root, FERN_REACH)


static func _bay(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	s: float,
	long: float,
	side: float,
	index: int,
	used: Used,
	parts: Quarters.Parts
) -> void:
	var near := NeonBiome.STREET_HALF + Street.SIDEWALK
	var low := minf(near * side, GROUND_OUT * side)
	var high := maxf(near * side, GROUND_OUT * side)
	var lift := StreetGround.KERB_HEIGHT
	StreetGround._strip(kit, route, s, s + long, low, high, lift, parts.pavers, 1.0)
	var middle := s + long * 0.5
	var basis := Quarters._frame(route, middle)
	var base := route.at(middle)
	var out := route.right(middle) * side
	var block := floori(float(index) / BAYS_TO_A_BLOCK)
	var dice := Rng.derive(block, [141, int(side)])
	var levels := LEVELS_LOW + int(Rng.unit(dice, 0) * (LEVELS_HIGH - LEVELS_LOW + 1))
	var court := index % BAYS_TO_A_BLOCK == BAYS_TO_A_BLOCK - 1
	_garden(kit, basis, base, out, long, index, side, used, parts)
	if court:
		_stair(kit, basis, base, out, long, levels, used, parts)
	else:
		for level: int in levels:
			var cell := Rng.derive(dice, [index, level])
			_flat(kit, basis, base, out, long, level, level == levels - 1, cell, used, parts)
	if index % LIT_EVERY == 0:
		root.add_child(_wash(base + out * (FRONT - 2.0) + Vector3.UP * 2.6))
	if index % (LIT_EVERY * 2) == 0:
		var up_there := base + out * (FRONT + UPPER_WASH * STEP) + Vector3.UP * UPPER_WASH * STOREY
		root.add_child(_wash(up_there + Vector3.UP * 2.6))


## The low planted wall between the pavement and the first flats, with a light along its foot.
static func _garden(
	kit: CityMesh,
	basis: Basis,
	base: Vector3,
	out: Vector3,
	long: float,
	index: int,
	side: float,
	used: Used,
	parts: Quarters.Parts
) -> void:
	var wide := GARDEN_OUT - GARDEN_IN
	var middle := (GARDEN_IN + GARDEN_OUT) * 0.5
	kit.block(
		parts.concrete,
		Transform3D(basis, base + out * middle + Vector3.UP * GARDEN_HIGH * 0.5),
		Vector3(wide, GARDEN_HIGH, long - 0.25),
		PALE,
		0.05
	)
	var foot := base + out * (GARDEN_IN - 0.03) + Vector3.UP * 0.14
	var run := basis.z * (long * 0.5 - 0.3)
	kit.bar(used.lit, foot - run, foot + run, 0.03, FOOT_LIGHT)
	var dice := Rng.derive(index, [142, int(side)])
	for fern: int in FERNS_TO_A_BAY * 2:
		var along := (float(fern) + Rng.unit(dice, fern)) / (FERNS_TO_A_BAY * 2) - 0.5
		var across := lerpf(GARDEN_IN + 0.4, GARDEN_OUT - 0.4, Rng.unit(dice, 10 + fern))
		var at := base + out * across + basis.z * along * long + Vector3.UP * GARDEN_HIGH
		_fern(used, at, Rng.unit(dice, 20 + fern), 1.4)


## One flat: its room, the fins either side of it, and the terrace out in front.
static func _flat(
	kit: CityMesh,
	basis: Basis,
	base: Vector3,
	out: Vector3,
	long: float,
	level: int,
	top: bool,
	cell: int,
	used: Used,
	parts: Quarters.Parts
) -> void:
	var front := FRONT + level * STEP
	var floor_at := base + Vector3.UP * (level * STOREY)
	var wall := front + RECESS
	# Twice as deep as it has to be, so nothing shows through to the sky behind.
	var mass := Vector3(STEP * 2.0, STOREY, long + 0.02)
	var centre := floor_at + out * (wall + STEP) + Vector3.UP * STOREY * 0.5
	kit.box(parts.concrete, Transform3D(basis, centre), mass, PALE)
	var face := floor_at + out * wall + Vector3.UP * (0.3 + WINDOW_HIGH * 0.5)
	var wide := long - 1.3
	var pane := Vector3(0.05, WINDOW_HIGH, wide)
	if Rng.unit(cell, 0) < ROOM_DARK:
		kit.box(parts.steel, Transform3D(basis, face - out * 0.03), pane, DARK)
	else:
		var hue: Color = ROOMS[int(Rng.unit(cell, 1) * ROOMS.size())]
		kit.box(used.room, Transform3D(basis, face - out * 0.03), pane, hue)
		if Rng.unit(cell, 2) < BLIND_CHANCE:
			var drop := lerpf(0.4, 1.5, Rng.unit(cell, 3))
			var blind := face - out * 0.06 + Vector3.UP * (WINDOW_HIGH - drop) * 0.5
			var slats := Vector3(0.03, drop, wide)
			kit.box(parts.concrete, Transform3D(basis, blind), slats, SHADE)
	for mullion: float in [-0.5, -0.17, 0.17, 0.5]:
		var at := face - out * 0.09 + basis.z * wide * mullion
		kit.box(parts.steel, Transform3D(basis, at), Vector3(0.07, WINDOW_HIGH, 0.07), FRAME)
	for end: float in [-0.5, 0.5]:
		var fin := floor_at + out * (front + RECESS * 0.5) + basis.z * long * end
		var fin_size := Vector3(RECESS + 0.2, STOREY, 0.28)
		var place := Transform3D(basis, fin + Vector3.UP * STOREY * 0.5)
		kit.block(parts.concrete, place, fin_size, PALE, 0.04)
	var soffit := floor_at + out * (front + RECESS * 0.5) + Vector3.UP * (STOREY - 0.14)
	var slab := Vector3(RECESS + 0.5, 0.28, long + 0.02)
	kit.block(parts.concrete, Transform3D(basis, soffit), slab, PALE, 0.03)
	if level > 0:
		_terrace(kit, basis, floor_at, out, front - STEP + RECESS, long, cell, used, parts)
	if top:
		_terrace(kit, basis, floor_at + Vector3.UP * STOREY, out, wall, long, cell + 1, used, parts)


## A planter along the terrace's outer edge, with a glass rail above it.
static func _terrace(
	kit: CityMesh,
	basis: Basis,
	floor_at: Vector3,
	out: Vector3,
	edge: float,
	long: float,
	cell: int,
	used: Used,
	parts: Quarters.Parts
) -> void:
	var planter := floor_at + out * (edge + 0.42) + Vector3.UP * PLANTER_HIGH * 0.5
	var size := Vector3(0.75, PLANTER_HIGH, long - 0.5)
	kit.block(parts.concrete, Transform3D(basis, planter), size, SHADE, 0.03)
	var run := basis.z * (long * 0.5 - 0.15)
	var at := floor_at + out * (edge + 0.03)
	var rail := Vector3.UP * RAIL_HIGH
	kit.quad(
		used.glass, at - run, at - run + rail, at + run + rail, at + run, -out, Color.WHITE, 1.0
	)
	kit.bar(parts.steel, at - run + rail, at + run + rail, 0.05, FRAME)
	for fern: int in FERNS_TO_A_BAY:
		var along := (float(fern) + Rng.unit(cell, 10 + fern)) / FERNS_TO_A_BAY - 0.5
		var plant := floor_at + out * (edge + 0.42) + basis.z * along * (long - 0.8)
		_fern(used, plant + Vector3.UP * PLANTER_HIGH, Rng.unit(cell, 20 + fern), 1.0)


## A way up through the block: a flight for each level, between lit rails.
static func _stair(
	kit: CityMesh,
	basis: Basis,
	base: Vector3,
	out: Vector3,
	long: float,
	levels: int,
	used: Used,
	parts: Quarters.Parts
) -> void:
	var treads := 6
	var wide := long - 2.0
	for level: int in levels:
		for tread: int in treads:
			var u := float(tread) / treads
			var along := FRONT + (level + u) * STEP
			var high := (level + u) * STOREY
			var size := Vector3(STEP / treads + 0.02, high + 0.2, wide)
			var at := base + out * along + Vector3.UP * (high + 0.2) * 0.5
			kit.box(parts.concrete, Transform3D(basis, at), size, PALE)
		var from := base + out * (FRONT + level * STEP) + Vector3.UP * (level * STOREY + 1.0)
		var to := from + out * STEP + Vector3.UP * STOREY
		for end: float in [-0.5, 0.5]:
			var shift := basis.z * wide * end
			kit.bar(used.lit, from + shift, to + shift, 0.03, FOOT_LIGHT)


static func _fern(used: Used, at: Vector3, roll: float, size: float) -> void:
	var turned := Basis(Vector3.UP, roll * TAU).scaled(Vector3.ONE * FERN_SCALE * size)
	for part: String in used.ferns.parts():
		used.ferns.place(part, Transform3D(turned, at))


static func _wash(at: Vector3) -> OmniLight3D:
	var lamp := OmniLight3D.new()
	lamp.light_color = WASH_HUE
	lamp.light_energy = WASH_ENERGY
	lamp.light_volumetric_fog_energy = 0.1
	lamp.omni_range = WASH_REACH
	lamp.position = at
	return lamp
