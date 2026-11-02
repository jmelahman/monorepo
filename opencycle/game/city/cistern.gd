class_name Cistern
extends RefCounted
## A flooded hall under the deck, crossed on a causeway: columns in rows off into the dark,
## each with its one lamp, standing in black water that gives all of it back. Nothing here is
## for anyone. The city just needed something to hold itself up, and something to drain into.

const STILL_WATER: Shader = preload("res://game/still_water.gdshader")
const ROOF := 18.0
const SLAB := 2.0
const OUT := 66.0
const WATER := -0.12
const PARAPET := 0.26
## What the road has either side of it out here, in place of a pavement.
const WALK := 0.7
const PITCH := 12.0
const ROWS: PackedFloat32Array = [9.5, 21.5, 33.5, 45.5, 57.5]
const COLUMN := 1.9
const PLINTH := 2.8
const LAMP_HIGH := 5.4
const LAMP_DEAD := 0.22
const LAMP := Color(4.2, 2.5, 1.1)
## The first row's lamps are warm and the rest cold, which is what pushes the far rows away.
const LAMP_FAR := Color(1.5, 2.4, 3.6)
const LIGHT_NEAR := Color(1.0, 0.62, 0.3)
const LIGHT_FAR := Color(0.5, 0.72, 1.0)
const MOUTH := 8.5
const STONE := Color(0.95, 0.97, 1.0)
## Each row of columns is lit from its own lamps, and every so many bays one is a real light.
const LIT_EVERY: PackedInt32Array = [2, 3, 4]
const LIT_ENERGY: PackedFloat32Array = [3.5, 5.0, 8.0]
const LIT_RANGE: PackedFloat32Array = [20.0, 26.0, 34.0]
const FILL_EVERY := 2
const FILL_ENERGY := 3.0
const FILL_REACH := 22.0
const SHAFT_EVERY := 3
const SHAFT_OFFSET := 15.5
const GRATING := Color(0.62, 0.86, 1.15)
const GRATING_SLATS := 7
const COLD := Color(0.62, 0.8, 1.0)
const RAIL_POST := 2.8

static var _water: Material


## The bays of [param stretch] whose start lies in [param section].
static func build(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	section: StreetSection,
	parts: Quarters.Parts
) -> void:
	if not _water:
		_water = _water_material()
	var index := 0
	var s := stretch.from
	while s < stretch.to:
		if section.holds(s):
			kit.at(s)
			_bay(kit, root, route, s, minf(PITCH, stretch.to - s), index, parts)
		s += PITCH
		index += 1
	for end: float in [stretch.from, stretch.to]:
		if section.holds(end):
			kit.at(end)
			_mouth(kit, route, end, parts)


## How far out from the road's middle the causeway stops and the water starts.
static func edge() -> float:
	return NeonBiome.STREET_HALF + StreetGround.KERB_WIDTH + WALK


static func _water_material() -> Material:
	var water := StandardMaterial3D.new()
	water.albedo_color = Color(0.0, 0.0, 0.0)
	water.roughness = 0.3
	water.metallic = 0.0
	# Lamps this close would otherwise leave a pale smear on it instead of a reflection.
	water.specular_mode = BaseMaterial3D.SPECULAR_DISABLED
	var mirror := ShaderMaterial.new()
	mirror.shader = STILL_WATER
	water.next_pass = mirror
	return water


static func _bay(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	s: float,
	long: float,
	index: int,
	parts: Quarters.Parts
) -> void:
	var basis := Quarters._frame(route, s)
	var middle := s + long * 0.5
	var wall := edge()
	var ceiling := Vector3.UP * (ROOF + SLAB * 0.5 + 0.02 * (index % 2))
	kit.box(
		parts.concrete,
		Transform3D(Quarters._frame(route, middle), route.at(middle) + ceiling),
		Vector3(OUT * 2.0, SLAB, long * 1.3),
		STONE * 0.7
	)
	kit.box(
		parts.concrete,
		Transform3D(basis, route.at(s) + Vector3.UP * (ROOF - 0.8)),
		Vector3(OUT * 2.0, 1.6, 1.5),
		STONE * 0.8
	)
	for side: float in [-1.0, 1.0]:
		_flank(kit, route, s, long, side, wall, parts)
		var out := route.right(s) * side
		for row: int in ROWS.size():
			var foot := route.at(s) + out * ROWS[row]
			var dice := Rng.derive(index, [97, row, int(side)])
			var lit := Rng.unit(dice, 0) > LAMP_DEAD
			_column(kit, basis, foot, -side, lit, LAMP if row == 0 else LAMP_FAR, parts)
			if lit and row < LIT_EVERY.size() and index % LIT_EVERY[row] == 0:
				var glow := foot + out * -(COLUMN * 0.5 + 1.5) + Vector3.UP * LAMP_HIGH
				var hue := LIGHT_NEAR if row == 0 else LIGHT_FAR
				root.add_child(_column_light(glow, hue, LIT_ENERGY[row], LIT_RANGE[row]))
			# The beams that run along the hall, one over each row of columns.
			var next := route.at(s + long) + route.right(s + long) * side * ROWS[row]
			var lift := Vector3.UP * (ROOF - 0.8)
			kit.bar(parts.concrete, foot + lift, next + lift, 1.5, STONE * 0.75)
		var far := route.at(middle) + route.right(middle) * side * OUT
		kit.box(
			parts.concrete,
			Transform3D(basis, far + Vector3.UP * ((ROOF + WATER) * 0.5)),
			Vector3(1.0, ROOF - WATER, long * 1.3),
			STONE * 0.5
		)
	# Off to one side of the road and then the other, for the faces the rider is coming to.
	var fill_side := 1.0 if index % FILL_EVERY == 0 else -1.0
	root.add_child(_fill(route.at(s) + route.right(s) * fill_side * 5.0 + Vector3.UP * 6.5))
	if index % SHAFT_EVERY == 2:
		var side := 1.0 if index % (SHAFT_EVERY * 2) < SHAFT_EVERY else -1.0
		var under := route.at(middle) + route.right(middle) * side * SHAFT_OFFSET
		_grating(kit, basis, under, parts)
		root.add_child(_shaft(under))


## Where the cold comes in: a slatted hole in the roof, set up between the beams.
static func _grating(kit: CityMesh, basis: Basis, under: Vector3, parts: Quarters.Parts) -> void:
	var soffit := under + Vector3.UP * (ROOF - 0.03)
	kit.box(parts.glow, Transform3D(basis, soffit), Vector3(5.0, 0.04, 5.0), GRATING)
	var dark := Color(0.05, 0.05, 0.055)
	for slat: int in GRATING_SLATS:
		var along := lerpf(-2.5, 2.5, float(slat) / (GRATING_SLATS - 1))
		var at := soffit - Vector3.UP * 0.12 + basis.z * along
		kit.box(parts.steel, Transform3D(basis, at), Vector3(5.2, 0.2, 0.34), dark)


## The water either side of the causeway, the causeway's edge going down into it, and the rail.
static func _flank(
	kit: CityMesh,
	route: Route,
	s: float,
	long: float,
	side: float,
	wall: float,
	parts: Quarters.Parts
) -> void:
	var next := s + long
	var low := minf(wall * side, OUT * side)
	var high := maxf(wall * side, OUT * side)
	StreetGround._strip(kit, route, s, next, low, high, WATER, _water, 1.0)
	var middle := s + long * 0.5
	var basis := Quarters._frame(route, middle)
	var lip := route.at(middle) + route.right(middle) * side * (wall + 0.25)
	kit.box(
		parts.concrete,
		Transform3D(basis, lip + Vector3.UP * ((PARAPET + WATER) * 0.5)),
		Vector3(0.5, PARAPET - WATER, long * 1.05),
		STONE * 0.8
	)
	var rail := Vector3.UP * 1.25
	var from := route.at(s) + route.right(s) * side * (wall + 0.1)
	var to := route.at(next) + route.right(next) * side * (wall + 0.1)
	var paint := Color(0.5, 0.42, 0.3)
	kit.tube(parts.steel, from + rail, to + rail, 0.04, paint, 6)
	var post := 0.0
	while post < long:
		var at := from.lerp(to, post / long)
		kit.bar(parts.steel, at + Vector3.UP * 0.7, at + rail, 0.05, paint)
		post += RAIL_POST


static func _column(
	kit: CityMesh,
	basis: Basis,
	foot: Vector3,
	facing: float,
	lit: bool,
	hue: Color,
	parts: Quarters.Parts
) -> void:
	var tall := ROOF - WATER
	kit.block(
		parts.concrete,
		Transform3D(basis, foot + Vector3.UP * (WATER + tall * 0.5)),
		Vector3(COLUMN, tall, COLUMN),
		STONE,
		0.14
	)
	# A collar at the waterline, stained where the level has been.
	kit.block(
		parts.concrete,
		Transform3D(basis, foot + Vector3.UP * (WATER + 0.9)),
		Vector3(PLINTH, 1.8, PLINTH),
		STONE * 0.55,
		0.2
	)
	# Stepping out at the top, to take the beams.
	for step: int in 3:
		var wide := COLUMN + 0.7 * (step + 1)
		var high := ROOF - 2.8 + 0.7 * step
		kit.block(
			parts.concrete,
			Transform3D(basis, foot + Vector3.UP * high),
			Vector3(wide, 0.7, wide),
			STONE * 0.85,
			0.06
		)
	if lit:
		var at := foot + basis.x * facing * (COLUMN * 0.5 + 0.08) + Vector3.UP * LAMP_HIGH
		kit.box(parts.glow, Transform3D(basis, at), Vector3(0.12, 0.7, 0.26), hue)
		kit.box(
			parts.steel,
			Transform3D(basis, at + Vector3.UP * 0.42),
			Vector3(0.3, 0.06, 0.4),
			Color(0.2, 0.2, 0.2)
		)


## The wall the hall ends in, with a way through it for the road.
static func _mouth(kit: CityMesh, route: Route, s: float, parts: Quarters.Parts) -> void:
	var basis := Quarters._frame(route, s)
	var wall := edge()
	var base := route.at(s)
	for side: float in [-1.0, 1.0]:
		var middle := (wall + OUT) * 0.5
		kit.block(
			parts.concrete,
			Transform3D(
				basis, base + route.right(s) * side * middle + Vector3.UP * ((ROOF + WATER) * 0.5)
			),
			Vector3(OUT - wall, ROOF - WATER + SLAB, 2.0),
			STONE * 0.7,
			0.1
		)
	kit.block(
		parts.concrete,
		Transform3D(basis, base + Vector3.UP * ((MOUTH + ROOF + SLAB) * 0.5)),
		Vector3(wall * 2.0, ROOF + SLAB - MOUTH, 2.0),
		STONE * 0.7,
		0.1
	)


static func _column_light(at: Vector3, hue: Color, energy: float, reach: float) -> OmniLight3D:
	var lamp := OmniLight3D.new()
	lamp.light_color = hue
	lamp.light_energy = energy
	lamp.light_volumetric_fog_energy = 0.5
	lamp.omni_range = reach
	lamp.position = at
	return lamp


static func _fill(at: Vector3) -> OmniLight3D:
	var lamp := OmniLight3D.new()
	lamp.light_color = Color(1.0, 0.68, 0.38)
	lamp.light_energy = FILL_ENERGY
	lamp.light_volumetric_fog_energy = 0.3
	lamp.omni_range = FILL_REACH
	lamp.position = at
	return lamp


static func _shaft(under: Vector3) -> SpotLight3D:
	var beam := SpotLight3D.new()
	beam.light_color = COLD
	beam.light_energy = 26.0
	beam.light_volumetric_fog_energy = 2.0
	beam.light_specular = 0.4
	beam.spot_range = ROOF + 6.0
	beam.spot_angle = 22.0
	beam.spot_attenuation = 0.6
	beam.shadow_enabled = true
	beam.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
	beam.position = under + Vector3.UP * (ROOF - 0.3)
	return beam
