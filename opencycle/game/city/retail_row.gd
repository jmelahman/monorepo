class_name RetailRow
extends RefCounted
## Out from under the deck, for once: a shopping street with weather and a sky. Low blocks, every
## shop open, glass arcades along both pavements and strings of bulbs across the road, and at each
## end the raw cut edge of the city's lid, which the rider has been living under until now.

const BAY := 6.0
## The arcade's glass falls from the shopfronts to its columns, out near the kerb.
const ARCADE_HIGH := 5.6
const ARCADE_LOW := 4.7
const ARCADE_OUT := 4.9
const GLASS_GONE := 0.1
## Vertex colours stop at 1, so what has to bloom gets the rest from its material.
const LIT := 1.7
const STRIP := Color(1.0, 0.74, 0.46)
const STRIP_DEAD := 0.12
const LIGHT_EVERY := 3
const LIGHT_ENERGY := 4.0
const LIGHT_REACH := 13.0
const LIGHT_HUE := Color(1.0, 0.76, 0.5)
const COLUMN := 0.2
const STRUCTURE := Color(0.72, 0.76, 0.8)
const FESTOON_EVERY := 2
const FESTOON_LOW := 7.2
const FESTOON_HIGH := 9.5
const FESTOON_SAG := 0.7
const FESTOON_POINTS := 16
const BULB := Color(1.0, 0.68, 0.38)
const BULB_DEAD := 0.1
const DECK_END_DEEP := 9.0
const DECK_END_HIGH := 15.0
const DECK_END_BEAMS := 15
const DECK_END_LAMP := Color(0.6, 0.78, 1.0)
const STONE := Color(0.6, 0.62, 0.68)
const ROOF_SET_BACK := 5.0
const ROOF_THINGS := 3
const TANK_CHANCE := 0.45
const MAST_CHANCE := 0.35
const AVIATION := Color(1.0, 0.1, 0.05)


## The bays of [param stretch] whose start lies in [param section], and the deck's two ends.
static func build(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	section: StreetSection,
	parts: Quarters.Parts
) -> void:
	var glass: Material = Once.made("retail_glass", _glass)
	var lit: Material = Once.made("retail_lit", _lit)
	var index := 0
	var s := stretch.from
	while s < stretch.to:
		if section.holds(s):
			kit.at(s)
			var next := minf(s + BAY, stretch.to)
			for side: float in [-1.0, 1.0]:
				_arcade(kit, root, route, s, next, side, index, glass, lit, parts)
			if index % FESTOON_EVERY == 0:
				_festoon(kit, route, s, index, lit, parts)
		s += BAY
		index += 1
	for end: float in [stretch.from, stretch.to]:
		if section.holds(end):
			kit.at(end)
			_deck_end(kit, route, end, lit, parts)


## What the rooflines of the low blocks have collected, for something against the sky.
static func roofs(
	kit: CityMesh, route: Route, street: Street, section: StreetSection, parts: Quarters.Parts
) -> void:
	for tower: Street.Tower in street.towers:
		if section.holds(tower.s) and Districts.sky_open(street.districts.kind_at(tower.s)):
			kit.at(tower.s)
			_roof(kit, route, tower, parts)


static func _roof(kit: CityMesh, route: Route, tower: Street.Tower, parts: Quarters.Parts) -> void:
	var basis := Quarters._frame(route, tower.s)
	var wall := NeonBiome.STREET_HALF + Street.SIDEWALK
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var roof := route.at(tower.s) + outward * wall + Vector3.UP * tower.height
	for thing: int in ROOF_THINGS:
		var dice := Rng.derive(tower.dice, [133, thing])
		var run := lerpf(-0.42, 0.42, Rng.unit(dice, 0)) * tower.width
		var back := lerpf(ROOF_SET_BACK, tower.depth - 2.0, Rng.unit(dice, 1))
		var at := roof + along * run + outward * back
		var roll := Rng.unit(dice, 2)
		if roll < TANK_CHANCE:
			var tall := lerpf(2.4, 4.2, Rng.unit(dice, 3))
			var legs := Vector3.UP * 1.6
			kit.bar(parts.steel, at, at + legs, 0.5, STRUCTURE * 0.6)
			kit.tube(parts.rust, at + legs, at + legs + Vector3.UP * tall, 1.5, STRUCTURE, 10)
		elif roll < TANK_CHANCE + MAST_CHANCE:
			var high := lerpf(6.0, 13.0, Rng.unit(dice, 3))
			kit.bar(parts.steel, at, at + Vector3.UP * high, 0.1, STRUCTURE * 0.5)
			var tip := Transform3D(basis, at + Vector3.UP * high)
			kit.box(parts.glow, tip, Vector3.ONE * 0.16, AVIATION)
		else:
			var size := Vector3(lerpf(2.0, 4.5, Rng.unit(dice, 3)), 1.7, 2.4)
			var place := Transform3D(basis, at + Vector3.UP * size.y * 0.5)
			kit.block(parts.steel, place, size, STRUCTURE, 0.05)


static func _lit() -> Material:
	var lit := StandardMaterial3D.new()
	lit.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	lit.vertex_color_use_as_albedo = true
	lit.albedo_color = Color(LIT, LIT, LIT)
	return lit


static func _glass() -> Material:
	var glass := StandardMaterial3D.new()
	glass.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	glass.albedo_color = Color(0.62, 0.74, 0.78, 0.3)
	glass.roughness = 0.12
	glass.metallic = 0.6
	glass.cull_mode = BaseMaterial3D.CULL_DISABLED
	return glass


static func _arcade(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	s: float,
	next: float,
	side: float,
	index: int,
	glass: Material,
	lit: Material,
	parts: Quarters.Parts
) -> void:
	var wall := NeonBiome.STREET_HALF + Street.SIDEWALK
	var dice := Rng.derive(index, [131, int(side)])
	var high := Vector3.UP * ARCADE_HIGH
	var low := Vector3.UP * ARCADE_LOW
	var back_here := route.at(s) + route.right(s) * side * wall + high
	var back_there := route.at(next) + route.right(next) * side * wall + high
	var edge_here := route.at(s) + route.right(s) * side * ARCADE_OUT + low
	var edge_there := route.at(next) + route.right(next) * side * ARCADE_OUT + low
	# A pane or two gone, which is as much neglect as anyone lets show out here.
	if Rng.unit(dice, 0) > GLASS_GONE:
		var a := edge_here if side > 0.0 else back_here
		var b := back_here if side > 0.0 else edge_here
		var c := back_there if side > 0.0 else edge_there
		var d := edge_there if side > 0.0 else back_there
		kit.quad(glass, a, b, c, d, Vector3.UP, Color.WHITE, 1.0)
	kit.bar(parts.steel, back_here, edge_here, 0.14, STRUCTURE)
	kit.bar(parts.steel, edge_here, edge_there, 0.18, STRUCTURE)
	kit.bar(parts.steel, back_here, back_there, 0.14, STRUCTURE)
	var foot := route.at(s) + route.right(s) * side * ARCADE_OUT
	kit.bar(parts.steel, foot, foot + low, COLUMN, STRUCTURE)
	kit.bar(parts.steel, foot, foot + Vector3.UP * 0.5, COLUMN * 1.7, STRUCTURE * 0.7)
	if Rng.unit(dice, 1) > STRIP_DEAD:
		var under := Vector3.DOWN * 0.13
		kit.bar(lit, edge_here + under, edge_there + under, 0.035, STRIP)
	if index % LIGHT_EVERY == 0:
		var middle := (edge_here + back_there) * 0.5 + Vector3.DOWN * 0.8
		root.add_child(_arcade_light(middle))


static func _arcade_light(at: Vector3) -> OmniLight3D:
	var lamp := OmniLight3D.new()
	lamp.light_color = LIGHT_HUE
	lamp.light_energy = LIGHT_ENERGY
	lamp.light_volumetric_fog_energy = 0.12
	lamp.omni_range = LIGHT_REACH
	lamp.position = at
	return lamp


## A sagging string of bulbs from one side of the street to the other.
static func _festoon(
	kit: CityMesh, route: Route, s: float, index: int, lit: Material, parts: Quarters.Parts
) -> void:
	var dice := Rng.derive(index, [132])
	var wall := NeonBiome.STREET_HALF + Street.SIDEWALK
	var high := lerpf(FESTOON_LOW, FESTOON_HIGH, Rng.unit(dice, 0))
	var skew := lerpf(-2.5, 2.5, Rng.unit(dice, 1))
	var cable := Color(0.05, 0.05, 0.05)
	var previous := Vector3.ZERO
	for step: int in FESTOON_POINTS + 1:
		var u := float(step) / FESTOON_POINTS
		var at := s + skew * (u - 0.5)
		var droop := FESTOON_SAG * (1.0 - pow(u * 2.0 - 1.0, 2.0))
		var across := lerpf(-wall, wall, u)
		var point := route.at(at) + route.right(at) * across + Vector3.UP * (high - droop)
		if step > 0:
			kit.bar(parts.steel, previous, point, 0.02, cable)
		if Rng.unit(dice, 10 + step) > BULB_DEAD:
			var bulb := point + Vector3.DOWN * 0.09
			kit.box(lit, Transform3D(Basis.IDENTITY, bulb), Vector3.ONE * 0.09, BULB)
		previous = point


## Where the deck stops: its whole thickness in section, and the beams that hold it up.
static func _deck_end(
	kit: CityMesh, route: Route, s: float, lit: Material, parts: Quarters.Parts
) -> void:
	var basis := Quarters._frame(route, s)
	var wide := Overhead.DECK_WIDE * 2.0
	var base := route.at(s)
	var underside := Undercity.DECK_HEIGHT
	kit.block(
		parts.concrete,
		Transform3D(basis, base + Vector3.UP * (underside + DECK_END_HIGH * 0.5)),
		Vector3(wide, DECK_END_HIGH, DECK_END_DEEP),
		STONE,
		0.2
	)
	for beam: int in DECK_END_BEAMS:
		var across := lerpf(-0.5, 0.5, float(beam) / (DECK_END_BEAMS - 1)) * wide * 0.6
		var at := base + route.right(s) * across + Vector3.UP * (underside - 1.4)
		kit.block(
			parts.concrete,
			Transform3D(basis, at),
			Vector3(1.6, 2.8, DECK_END_DEEP + 1.0),
			STONE * 0.85,
			0.08
		)
	var lamps := Vector3.UP * (underside - 0.2)
	var run := route.right(s) * wide * 0.3
	kit.bar(lit, base + lamps - run, base + lamps + run, 0.12, DECK_END_LAMP)
