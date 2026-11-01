class_name BreakersYard
extends RefCounted
## Where the deck opens over a yard breaking up something enormous: a hull on keel blocks, down
## to its ribs along most of its length, under white floods. Across the road the containers of
## what's already been cut up, and a crane reaching over the rider's head from one to the other.

const PITCH := 7.0
const DEPTH := 46.0
const WALL_TINT := Color(0.5, 0.52, 0.55)
const HULL_SIDE := -1.0
const HULL_OFFSET := 27.0
const HULL_BEAM := 12.5
const HULL_HIGH := 26.0
const KEEL := 1.5
const HULL_END := 16.0
const RIB_PIECES := 9
const RIB_THICK := 0.42
const STRINGERS: Array[int] = [2, 4, 6, 8]
const PLATED := 0.42
## Plate stays on towards the ends, and at the bottom, where the cutters get to it last.
const PLATED_END := 0.2
const STEEL := Color(0.42, 0.37, 0.32)
const RUST := Color(0.5, 0.4, 0.33)
## What's left of her paint: antifouling up to the old waterline, and dark topsides above it.
const BOTTOM_PAINT := Color(0.66, 0.2, 0.11)
const TOPSIDE_PAINT := Color(0.36, 0.38, 0.4)
const WATERLINE := 3
const RUSTED_THROUGH := 0.38
const PLATES_ALONG := 3
const PLATES_UP := 2
const PLATE_GONE := 0.07
const PLATE_SEAM := 0.015
const BLOCK_EVERY := 2
const FLOOD_EVERY := 5
const FLOOD_HIGH := 19.0
const FLOOD := Color(0.95, 0.95, 1.0)
const FLOOD_ENERGY := 120.0
const ROAD_LIGHT := Color(0.72, 0.84, 1.0)
const ROAD_LIGHT_ENERGY := 3.5
const ROAD_LIGHT_REACH := 20.0
const WELD_EVERY := 3
const WELD_CHANCE := 0.6
const WELD := Color(0.62, 0.8, 1.0)
const STACK_LONG := 12.0
const STACK_WIDE := 2.6
const STACK_HIGH := 2.7
const STACK_ROWS: PackedFloat32Array = [17.0, 20.2, 26.5, 29.7, 36.0, 39.2]
const STACK_MOST := 5
const STACK_GAP := 0.25
const STACK_LOST := 0.14
const CONTAINERS: Array[Color] = [
	Color(0.55, 0.2, 0.12),
	Color(0.2, 0.3, 0.36),
	Color(0.5, 0.42, 0.16),
	Color(0.32, 0.36, 0.3),
	Color(0.5, 0.5, 0.5),
]
const SODIUM := Color(1.0, 0.56, 0.2)
const FENCE_POST := 3.5
const FENCE_HIGH := 2.6
const FENCE_SHEET := 0.32
const SCRAP_EVERY := 11.0
const SCRAP_PIECES := 9
const CRANE_OFFSET := 13.5
const CRANE_HIGH := 27.0
const CRANE_WIDE := 2.2
const CRANE_REACH := 36.0
const CRANE_STEP := 2.4
const CRANE_PAINT := Color(0.62, 0.42, 0.1)


## The bays of [param stretch] whose start lies in [param section].
static func build(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	section: StreetSection,
	parts: Quarters.Parts
) -> void:
	var index := 0
	var s := stretch.from
	while s < stretch.to:
		if section.holds(s):
			kit.at(s)
			var long := minf(PITCH, stretch.to - s)
			_bay(kit, route, s, long, index, parts)
			_rib(kit, root, route, stretch, s, index, parts)
		s += PITCH
		index += 1
	index = 0
	s = stretch.from + STACK_LONG * 0.5
	while s < stretch.to - STACK_LONG * 0.5:
		if section.holds(s):
			kit.at(s)
			_stacks(kit, root, route, s, index, parts)
		s += STACK_LONG + 0.6
		index += 1
	index = 0
	s = stretch.from + SCRAP_EVERY
	while s < stretch.to - 4.0:
		if section.holds(s):
			kit.at(s)
			_scrap(kit, route, s, Rng.derive(index, [124]), parts)
		s += SCRAP_EVERY
		index += 1
	for end: float in [stretch.from, stretch.to]:
		if section.holds(end):
			kit.at(end)
			_end_wall(kit, route, end, parts)
	var middle := (stretch.from + stretch.to) * 0.5
	if section.holds(middle):
		kit.at(middle)
		_crane(kit, root, route, middle, parts)


## The ground either side of the road, the wall round the yard, and the fence along the road.
static func _bay(
	kit: CityMesh, route: Route, s: float, long: float, index: int, parts: Quarters.Parts
) -> void:
	var wall := Quarters._wall()
	var next := s + long
	var middle := s + long * 0.5
	var basis := Quarters._frame(route, middle)
	for side: float in [-1.0, 1.0]:
		var low := minf(wall * side, DEPTH * side)
		var high := maxf(wall * side, DEPTH * side)
		StreetGround._strip(
			kit, route, s, next, low, high, StreetGround.KERB_HEIGHT, parts.pavers, 1.0
		)
		var far := route.at(middle) + route.right(middle) * side * DEPTH
		kit.box(
			parts.concrete,
			Transform3D(basis, far + Vector3.UP * Undercity.DECK_HEIGHT * 0.5),
			Vector3(1.2, Undercity.DECK_HEIGHT, long * 1.2),
			WALL_TINT
		)
		_fence(kit, route, s, long, side, Rng.derive(index, [121, int(side)]), parts)


## Posts and rails, with sheet iron wired on wherever someone wanted to stop looking at it.
static func _fence(
	kit: CityMesh,
	route: Route,
	s: float,
	long: float,
	side: float,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var off := Quarters._wall() + 0.15
	var tint := Color(0.42, 0.4, 0.37)
	var up := Vector3.UP * (StreetGround.KERB_HEIGHT + FENCE_HIGH)
	var along := 0.0
	var panel := 0
	while along < long:
		var at := route.at(s + along) + route.right(s + along) * side * off
		var end := minf(along + FENCE_POST, long)
		var to := route.at(s + end) + route.right(s + end) * side * off
		kit.bar(parts.steel, at, at + up, 0.09, tint)
		kit.bar(parts.steel, at + up, to + up, 0.05, tint)
		kit.bar(parts.steel, at + up * 0.45, to + up * 0.45, 0.04, tint)
		if Rng.unit(dice, panel) < FENCE_SHEET:
			var shade := lerpf(0.5, 0.95, Rng.unit(dice, 20 + panel))
			var sheet := parts.iron if Rng.unit(dice, 40 + panel) < 0.6 else parts.worn
			var place := (at + to + up) * 0.5
			kit.box(
				sheet,
				Transform3D(Quarters._frame(route, s + along), place),
				Vector3(0.05, FENCE_HIGH, end - along),
				Color(shade, shade, shade)
			)
		along += FENCE_POST
		panel += 1


## How far the hull has come in from its full beam, this far along it: nothing at the ends.
static func _taper(stretch: Districts.Stretch, s: float) -> float:
	var t := inverse_lerp(stretch.from + HULL_END, stretch.to - HULL_END, s)
	if t <= 0.0 or t >= 1.0:
		return 0.0
	return pow(sin(t * PI), 0.45)


## A point on one frame: across the bottom from the keel, then straight up the side.
static func _frame_point(
	route: Route, stretch: Districts.Stretch, s: float, piece: int, out: float
) -> Vector3:
	var turn := float(piece) / RIB_PIECES * PI * 0.5
	var taper := _taper(stretch, s)
	var beam := HULL_BEAM * taper * pow(sin(turn), 0.55)
	# Bow and stern stand higher than the middle, as they do on anything that went to sea.
	var t := inverse_lerp(stretch.from, stretch.to, s)
	var sheer := HULL_HIGH * (0.86 + 0.28 * pow(absf(t * 2.0 - 1.0), 3.0))
	var y := KEEL + sheer * (1.0 - cos(turn))
	var across := HULL_SIDE * (HULL_OFFSET + out * beam)
	return route.at(s) + route.right(s) * across + Vector3.UP * y


## One of the hull's frames, the plate still on it and what's lighting it.
static func _rib(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	s: float,
	index: int,
	parts: Quarters.Parts
) -> void:
	var dice := Rng.derive(index, [122])
	if _taper(stretch, s) <= 0.0:
		return
	var next := s + PITCH
	var closes := _taper(stretch, next) <= 0.0
	for out: float in [-1.0, 1.0]:
		for piece: int in RIB_PIECES:
			var a := _frame_point(route, stretch, s, piece, out)
			var b := _frame_point(route, stretch, s, piece + 1, out)
			var shade := lerpf(0.7, 1.0, Rng.unit(dice, 100 + piece))
			kit.bar(parts.rust, a, b, RIB_THICK, STEEL * shade)
			if closes:
				continue
			var c := _frame_point(route, stretch, next, piece + 1, out)
			var d := _frame_point(route, stretch, next, piece, out)
			if piece + 1 in STRINGERS:
				kit.bar(parts.rust, b, c, RIB_THICK * 0.55, STEEL * 0.8)
			var chance := 0.92 if out > 0.0 else _plate_chance(stretch, s, piece)
			if Rng.unit(dice, 10 + piece + int(out + 1.0) * 20) < chance:
				var facing := route.right(s) * HULL_SIDE * -out
				var cell := Rng.derive(dice, [piece, int(out)])
				_plates(kit, [a, b, c, d], facing, piece, cell, parts)
	var keel := _frame_point(route, stretch, s, 0, 0.0)
	if not closes:
		kit.bar(parts.rust, keel, _frame_point(route, stretch, next, 0, 0.0), 0.8, STEEL * 0.7)
	if index % BLOCK_EVERY == 0:
		kit.block(
			parts.concrete,
			Transform3D(
				Quarters._frame(route, s),
				keel - Vector3.UP * (KEEL - StreetGround.KERB_HEIGHT) * 0.5
			),
			Vector3(5.0, KEEL, 1.4),
			WALL_TINT,
			0.08
		)
	if index % FLOOD_EVERY == 2:
		_flood(kit, root, route, s, parts)
	if index % WELD_EVERY == 1 and Rng.unit(dice, 90) < WELD_CHANCE:
		_welder(kit, root, route, stretch, s, dice, parts)


static func _plate_chance(stretch: Districts.Stretch, s: float, piece: int) -> float:
	var ends := absf(inverse_lerp(stretch.from, stretch.to, s) * 2.0 - 1.0)
	var low := 1.0 - float(piece) / RIB_PIECES
	return clampf(PLATED + ends * ends * 0.9 + low * low * PLATED_END, 0.0, 1.0)


## One bay of shell between two frames, as the separate plates it was welded up from.
static func _plates(
	kit: CityMesh,
	corners: Array[Vector3],
	facing: Vector3,
	piece: int,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var paint := BOTTOM_PAINT if piece < WATERLINE else TOPSIDE_PAINT
	for along: int in PLATES_ALONG:
		for up: int in PLATES_UP:
			var slot := (along * PLATES_UP + up) * 4
			if Rng.unit(dice, slot) < PLATE_GONE:
				continue
			var low := float(along) / PLATES_ALONG + PLATE_SEAM
			var high := float(along + 1) / PLATES_ALONG - PLATE_SEAM
			var foot := float(up) / PLATES_UP + PLATE_SEAM
			var head := float(up + 1) / PLATES_UP - PLATE_SEAM
			var a := _on_shell(corners, low, foot)
			var b := _on_shell(corners, low, head)
			var c := _on_shell(corners, high, head)
			var d := _on_shell(corners, high, foot)
			var shade := lerpf(0.55, 1.1, Rng.unit(dice, slot + 1))
			if Rng.unit(dice, slot + 2) < RUSTED_THROUGH:
				kit.quad(parts.rust, a, b, c, d, facing, RUST * shade, 0.5)
			else:
				kit.quad(parts.steel, a, b, c, d, facing, paint * shade, 0.5)
			kit.quad(parts.rust, a, b, c, d, -facing, RUST * 0.3 * shade, 0.5)


## Where [param along] and [param up] fall on a bay of shell, each from 0 to 1.
static func _on_shell(corners: Array[Vector3], along: float, up: float) -> Vector3:
	var first := corners[0].lerp(corners[1], up)
	return first.lerp(corners[3].lerp(corners[2], up), along)


## A mast at the road's edge, floodlighting the hull so that its ribs fall across each other.
static func _flood(
	kit: CityMesh, root: Node3D, route: Route, s: float, parts: Quarters.Parts
) -> void:
	var basis := Quarters._frame(route, s)
	var out := route.right(s) * HULL_SIDE
	var foot := route.at(s) + out * (Quarters._wall() + 1.5)
	var top := foot + Vector3.UP * FLOOD_HIGH
	var tint := Color(0.4, 0.4, 0.4)
	kit.tube(parts.steel, foot, top, 0.16, tint, 8)
	kit.block(
		parts.concrete,
		Transform3D(basis, foot + Vector3.UP * 0.5),
		Vector3(1.0, 1.0, 1.0),
		WALL_TINT,
		0.05
	)
	kit.box(parts.steel, Transform3D(basis, top), Vector3(0.3, 0.9, 2.4), tint)
	kit.box(parts.glow, Transform3D(basis, top + out * 0.17), Vector3(0.04, 0.72, 2.2), Color.WHITE)
	var beam := SpotLight3D.new()
	beam.light_color = FLOOD
	beam.light_energy = FLOOD_ENERGY
	beam.light_volumetric_fog_energy = 0.35
	beam.spot_range = 60.0
	beam.spot_angle = 62.0
	beam.spot_attenuation = 1.3
	beam.shadow_enabled = true
	root.add_child(beam)
	var aim := route.at(s + 6.0) + out * HULL_OFFSET + Vector3.UP * 9.0
	beam.look_at_from_position(top + out * 0.3, aim)
	# And what spills back off the lamp's housing, without which the road is just a silhouette.
	var spill := OmniLight3D.new()
	spill.light_color = ROAD_LIGHT
	spill.light_energy = ROAD_LIGHT_ENERGY
	spill.light_volumetric_fog_energy = 0.2
	spill.omni_range = ROAD_LIGHT_REACH
	spill.position = foot - out * 1.5 + Vector3.UP * 7.0
	root.add_child(spill)


## Somebody up on the frames, cutting. The rest of the hull goes blue-white around them.
static func _welder(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	s: float,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var piece := 3 + int(Rng.unit(dice, 91) * 5.0)
	var at := _frame_point(route, stretch, s, piece, -1.0)
	var basis := Quarters._frame(route, s)
	kit.box(parts.glow, Transform3D(basis, at), Vector3(0.3, 0.3, 0.3), Color.WHITE)
	var arc := OmniLight3D.new()
	arc.light_color = WELD
	arc.light_energy = 7.0
	arc.light_volumetric_fog_energy = 0.8
	arc.omni_range = 16.0
	arc.position = at
	root.add_child(arc)


## One column of containers, in each of the rows across the yard from the hull.
static func _stacks(
	kit: CityMesh, root: Node3D, route: Route, s: float, index: int, parts: Quarters.Parts
) -> void:
	var basis := Quarters._frame(route, s)
	var out := route.right(s) * -HULL_SIDE
	for row: int in STACK_ROWS.size():
		var dice := Rng.derive(index, [123, row])
		if Rng.unit(dice, 0) < STACK_LOST:
			continue
		var tall := 1 + int(Rng.unit(dice, 1) * STACK_MOST)
		var base := route.at(s) + out * STACK_ROWS[row]
		for level: int in tall:
			var hue := CONTAINERS[int(Rng.unit(dice, 2 + level) * CONTAINERS.size())]
			var shift := basis.z * lerpf(-0.3, 0.3, Rng.unit(dice, 10 + level))
			var high := StreetGround.KERB_HEIGHT + STACK_HIGH * (level + 0.5)
			kit.box(
				parts.worn if level % 2 == 0 else parts.iron,
				Transform3D(basis, base + shift + Vector3.UP * high),
				Vector3(STACK_WIDE - STACK_GAP, STACK_HIGH - 0.04, STACK_LONG),
				hue
			)
	if index % 2 == 0:
		var lamp := OmniLight3D.new()
		lamp.light_color = SODIUM
		lamp.light_energy = 12.0
		lamp.light_volumetric_fog_energy = 0.4
		lamp.omni_range = 24.0
		lamp.position = route.at(s) + out * 14.5 + Vector3.UP * 7.0
		root.add_child(lamp)
		kit.box(
			parts.glow,
			Transform3D(basis, lamp.position),
			Vector3(0.2, 0.12, 0.9),
			Color(1.0, 0.7, 0.3)
		)


## A heap of cut plate near the fence on the hull's side, tumbled however it landed.
static func _scrap(kit: CityMesh, route: Route, s: float, dice: int, parts: Quarters.Parts) -> void:
	var out := route.right(s) * HULL_SIDE
	var centre := route.at(s) + out * lerpf(Quarters._wall() + 2.5, 16.0, Rng.unit(dice, 0))
	for piece: int in SCRAP_PIECES:
		var slot := 10 + piece * 8
		var spread := out * lerpf(-1.8, 1.8, Rng.unit(dice, slot))
		spread += route.tangent(s) * lerpf(-2.6, 2.6, Rng.unit(dice, slot + 1))
		var high := StreetGround.KERB_HEIGHT + 0.15 + Rng.unit(dice, slot + 2) * 0.9
		var turned := Basis(Vector3.UP, Rng.unit(dice, slot + 3) * TAU)
		turned = turned.rotated(out, lerpf(-0.5, 0.5, Rng.unit(dice, slot + 4)))
		turned = turned.rotated(route.tangent(s), lerpf(-0.4, 0.4, Rng.unit(dice, slot + 5)))
		var size := Vector3(
			lerpf(1.0, 3.2, Rng.unit(dice, slot + 6)),
			0.1,
			lerpf(0.8, 2.2, Rng.unit(dice, slot + 7))
		)
		var tint := RUST * lerpf(0.5, 1.0, Rng.unit(dice, slot))
		kit.box(parts.rust, Transform3D(turned, centre + spread + Vector3.UP * high), size, tint)


## Closes the yard off where it meets the buildings, either side of the road.
static func _end_wall(kit: CityMesh, route: Route, s: float, parts: Quarters.Parts) -> void:
	var wall := Quarters._wall()
	var basis := Quarters._frame(route, s)
	var wide := DEPTH - wall
	for side: float in [-1.0, 1.0]:
		var at := route.at(s) + route.right(s) * side * (wall + wide * 0.5)
		kit.block(
			parts.concrete,
			Transform3D(basis, at + Vector3.UP * Undercity.DECK_HEIGHT * 0.5),
			Vector3(wide, Undercity.DECK_HEIGHT, 1.6),
			WALL_TINT,
			0.1
		)


## A tower crane among the containers, with its jib out over the road and the hull.
static func _crane(
	kit: CityMesh, root: Node3D, route: Route, s: float, parts: Quarters.Parts
) -> void:
	var across := route.right(s) * -HULL_SIDE
	var along := route.tangent(s)
	var foot := route.at(s) + across * CRANE_OFFSET
	var half := CRANE_WIDE * 0.5
	var corners: Array[Vector3] = [
		across * half + along * half,
		across * half - along * half,
		-across * half - along * half,
		-across * half + along * half,
	]
	for corner: int in 4:
		var low := foot + corners[corner]
		var next := foot + corners[(corner + 1) % 4]
		kit.bar(parts.steel, low, low + Vector3.UP * CRANE_HIGH, 0.2, CRANE_PAINT)
		var high := 0.0
		while high < CRANE_HIGH:
			var up := Vector3.UP * high
			kit.bar(parts.steel, low + up, next + up, 0.09, CRANE_PAINT * 0.8)
			kit.bar(
				parts.steel, low + up, next + up + Vector3.UP * CRANE_STEP, 0.07, CRANE_PAINT * 0.7
			)
			high += CRANE_STEP
	var top := foot + Vector3.UP * CRANE_HIGH
	var tip := top - across * CRANE_REACH
	var tail := top + across * 6.0
	var ridge := top + Vector3.UP * 3.2
	for lean: float in [-1.0, 1.0]:
		var offset := along * half * lean
		kit.bar(parts.steel, tail + offset, tip + offset, 0.2, CRANE_PAINT)
		var x := -6.0
		var flip := 1.0
		while x < CRANE_REACH:
			var from := top - across * x + offset
			var to := top - across * (x + CRANE_STEP) + Vector3.UP * flip * 1.4
			kit.bar(parts.steel, from, to, 0.07, CRANE_PAINT * 0.7)
			flip = 1.0 - flip
			x += CRANE_STEP
	kit.bar(parts.steel, ridge, tip, 0.05, Color(0.2, 0.2, 0.2))
	kit.bar(parts.steel, ridge, tail, 0.05, Color(0.2, 0.2, 0.2))
	kit.bar(parts.steel, top, ridge, 0.25, CRANE_PAINT)
	kit.box(
		parts.concrete,
		Transform3D(Quarters._frame(route, s), tail + Vector3.UP * -0.8),
		Vector3(4.0, 1.6, 2.0),
		WALL_TINT
	)
	# What it's holding: a piece of the hull, on its way across.
	var hook := top - across * CRANE_REACH * 0.62
	var load_at := hook - Vector3.UP * 13.0
	kit.bar(parts.steel, hook, load_at, 0.05, Color(0.15, 0.15, 0.15))
	var turned := Quarters._frame(route, s).rotated(along, 0.2)
	kit.box(
		parts.rust, Transform3D(turned, load_at - Vector3.UP * 2.2), Vector3(8.0, 4.4, 0.25), RUST
	)
	var red := OmniLight3D.new()
	red.light_color = Color(1.0, 0.1, 0.05)
	red.light_energy = 1.5
	red.omni_range = 7.0
	red.position = tip - Vector3.UP * 0.4
	kit.box(
		parts.glow,
		Transform3D(Basis.IDENTITY, red.position),
		Vector3(0.2, 0.2, 0.2),
		Quarters.HAZARD
	)
	root.add_child(red)
