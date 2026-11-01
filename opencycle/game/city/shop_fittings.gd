class_name ShopFittings
extends RefCounted
## What a shop has had fixed into it over the years: the services across its ceiling, the
## jeweller's cases, the bureau's rate board and grille, the kitchen at the back of an eatery,
## and the stock that never made it further than the door.

const BATTEN_LONG := 1.25
const BATTEN_GAP := 0.5
const DEAD_TUBE := 0.12
const DUCT_WIDE := 0.42
const DUCT_HIGH := 0.24
const GRILLE_EVERY := 0.15
const CASE_FOOT := 0.92
const CASE_RISE := 0.5
const CASE_SHELVES := 3
const CASE_ITEMS_PER_METRE := 13.0
const STRIP_DIM := 0.4
const AMBER := Color(0.34, 0.1, 0.012)
const RATE_ROWS := 6
const CARTON_HIGH := 0.34
const CRATE_HIGH := 0.25


## Fluorescent battens down the room, with a duct and conduit run where they fit round them.
static func services(
	kit: CityMesh, room: Shopfronts.Room, materials: Dictionary[String, Material]
) -> void:
	var rows := maxi(int(room.width / 1.5), 1)
	for r: int in rows:
		var across := ((r + 0.5) / rows - 0.5) * room.width
		_battens(kit, room, materials, across, Rng.derive(room.dice, [60, r]))
	var side := -1.0 if Rng.unit(room.dice, 61) < 0.5 else 1.0
	var run := side * (room.width * 0.5 - DUCT_WIDE * 0.5 - 0.12)
	var into := room.depth * 0.5
	var duct := Basis(room.along, Vector3.UP, room.outward)
	var drop := room.height - DUCT_HIGH * 0.5 - 0.06
	kit.box(
		materials["paint"],
		Transform3D(duct, room.point(run, into, drop)),
		Vector3(DUCT_WIDE, DUCT_HIGH, room.depth),
		Color(0.5, 0.52, 0.52)
	)
	var hangers := maxi(roundi(room.depth / 1.4), 1)
	for hanger: int in hangers:
		var at := (hanger + 0.5) / hangers * room.depth
		var strap := Vector3(DUCT_WIDE + 0.03, 0.03, 0.05)
		kit.box(
			materials["steel"],
			Transform3D(duct, room.point(run, at, drop - DUCT_HIGH * 0.5)),
			strap
		)
	for pipe: int in 3:
		var at := Rng.unit(room.dice, 62 + pipe) * room.depth
		var low := room.height - 0.05 - pipe * 0.03
		var half := room.width * 0.5
		kit.bar(
			materials["steel"],
			room.point(-half, at, low),
			room.point(half, at, low),
			0.02 + pipe * 0.006,
			Color(1.4, 1.35, 1.25)
		)


static func _battens(
	kit: CityMesh,
	room: Shopfronts.Room,
	materials: Dictionary[String, Material],
	across: float,
	dice: int
) -> void:
	var at := 0.35 + Rng.unit(dice, 0) * 0.3
	var fitting := 0
	var up := room.height - 0.04
	while at + BATTEN_LONG < room.depth - 0.2:
		var middle := at + BATTEN_LONG * 0.5
		kit.box(
			materials["paint"],
			Transform3D(
				Basis(room.along, Vector3.UP, room.outward), room.point(across, middle, up)
			),
			Vector3(0.11, 0.06, BATTEN_LONG + 0.06),
			Color(0.75, 0.75, 0.72)
		)
		var dead := Rng.unit(dice, 1 + fitting) < DEAD_TUBE
		var tint := Color(0.06, 0.06, 0.06) if dead else room.light
		kit.bar(
			materials["tube"],
			room.point(across, at, up - 0.05),
			room.point(across, at + BATTEN_LONG, up - 0.05),
			0.045,
			tint
		)
		at += BATTEN_LONG + BATTEN_GAP
		fitting += 1


## A run of cases on a wall, all on hooks and small boxes under a strip light.
static func jewel_case(
	kit: CityMesh,
	room: Shopfronts.Room,
	materials: Dictionary[String, Material],
	centre: Vector3,
	facing: Vector3,
	span: float,
	stream: int
) -> void:
	var run := Vector3.UP.cross(facing).normalized()
	var basis := Basis(run, Vector3.UP, facing)
	var carcass := Color(0.32, 0.1, 0.09)
	var trim := Color(0.8, 0.62, 0.34)
	var tall := CASE_RISE * CASE_SHELVES
	kit.box(
		materials["paint"],
		Transform3D(basis, centre + Vector3.UP * (CASE_FOOT + tall * 0.5) + facing * 0.02),
		Vector3(span, tall, 0.04),
		carcass
	)
	kit.box(
		materials["paint"],
		Transform3D(basis, centre + Vector3.UP * CASE_FOOT * 0.5 + facing * 0.2),
		Vector3(span, CASE_FOOT, 0.4),
		carcass
	)
	var dice := Rng.derive(room.dice, [70, stream])
	var count := maxi(roundi(span * CASE_ITEMS_PER_METRE), 1)
	for shelf: int in CASE_SHELVES + 1:
		var y := Vector3.UP * (CASE_FOOT + shelf * CASE_RISE)
		kit.box(
			materials["paint"],
			Transform3D(basis, centre + y + facing * 0.15),
			Vector3(span, 0.025, 0.3),
			trim
		)
		if shelf == CASE_SHELVES:
			continue
		var top := centre + y + Vector3.UP * (CASE_RISE - 0.035) + facing * 0.24
		var strip := room.light * STRIP_DIM
		strip.a = 1.0
		kit.bar(materials["tube"], top - run * span * 0.48, top + run * span * 0.48, 0.025, strip)
		for item: int in count:
			var key := shelf * 200 + item * 4
			var along := (Rng.unit(dice, key) - 0.5) * span * 0.96
			var into := lerpf(0.08, 0.26, Rng.unit(dice, key + 1))
			var long := lerpf(0.05, 0.3, Rng.unit(dice, key + 2))
			var hung := Rng.unit(dice, key + 3) < 0.55
			var at := centre + y + run * along + facing * into
			if hung:
				# A chain or bangle on a hook, from the shelf above.
				var hook := at + Vector3.UP * (CASE_RISE - 0.03)
				kit.bar(materials["gold"], hook, hook - Vector3.UP * long, 0.008)
			else:
				var size := Vector3(long * 0.35 + 0.02, long * 0.25 + 0.015, 0.05)
				var place := Transform3D(basis, at + Vector3.UP * (0.012 + size.y * 0.5))
				kit.box(materials["gold"], place, size)
	for post: int in 3:
		var at := centre + run * ((post * 0.5 - 0.5) * span) + facing * 0.3
		kit.bar(materials["paint"], at, at + Vector3.UP * (CASE_FOOT + tall), 0.03, trim)


## Steel bars up from a counter to the ceiling, with the slot still open across the bottom.
static func grille(
	kit: CityMesh,
	room: Shopfronts.Room,
	materials: Dictionary[String, Material],
	into: float,
	wide: float,
	high: float
) -> void:
	var foot := high + 0.28
	var head := room.height
	var half := wide * 0.5
	var pale := Color(2.2, 2.2, 2.2)
	for rail: float in [foot, foot + 0.9]:
		kit.bar(
			materials["steel"],
			room.point(-half, into, rail),
			room.point(half, into, rail),
			0.02,
			pale
		)
	var bars := maxi(roundi(wide / GRILLE_EVERY), 1)
	for i: int in bars + 1:
		var across := (float(i) / bars - 0.5) * wide
		kit.bar(
			materials["steel"],
			room.point(across, into, foot),
			room.point(across, into, head),
			0.012,
			pale
		)
	for end: float in [-1.0, 1.0]:
		var post := end * half
		kit.bar(
			materials["steel"],
			room.point(post, into, high),
			room.point(post, into, foot),
			0.02,
			pale
		)


## A bureau's board of rates: a dark panel, each row a label and two figures in amber.
static func rate_board(
	kit: CityMesh, room: Shopfronts.Room, materials: Dictionary[String, Material]
) -> void:
	var wide := minf(room.width - 0.8, 2.6)
	var tall := 1.05
	var into := room.depth - 0.05
	var y := 1.7
	var basis := Basis(room.along, Vector3.UP, room.outward)
	kit.box(
		materials["paint"],
		Transform3D(basis, room.point(0.0, into, y)),
		Vector3(wide + 0.1, tall + 0.1, 0.06),
		Color(0.13, 0.13, 0.13)
	)
	var row_high := tall / RATE_ROWS
	for row: int in RATE_ROWS:
		var up := y - tall * 0.5 + (row + 0.5) * row_high
		var left := -wide * 0.46
		var label := lerpf(0.2, 0.34, Rng.unit(room.dice, 80 + row)) * wide
		var figures: Array[Array] = [
			[left, left + label, Color(0.12, 0.12, 0.11)],
			[-wide * 0.02, wide * 0.2, AMBER],
			[wide * 0.25, wide * 0.46, AMBER],
		]
		for figure: Array in figures:
			var from: float = figure[0]
			var to: float = figure[1]
			var tint: Color = figure[2]
			kit.bar(
				materials["tube"],
				room.point(from, into - 0.04, up),
				room.point(to, into - 0.04, up),
				row_high * 0.36,
				tint
			)


## The way through to the kitchen, and the hood over the pass in front of it.
static func kitchen(
	kit: CityMesh, room: Shopfronts.Room, materials: Dictionary[String, Material]
) -> void:
	var basis := Basis(room.along, Vector3.UP, room.outward)
	var side := -1.0 if Rng.unit(room.dice, 90) < 0.5 else 1.0
	var across := side * (room.width * 0.5 - 0.7)
	var into := room.depth - 0.02
	kit.box(
		materials["steel"],
		Transform3D(basis, room.point(across, into, 1.03)),
		Vector3(0.9, 2.06, 0.03),
		Color(0.2, 0.2, 0.2)
	)
	for end: float in [-1.0, 1.0]:
		var jamb := across + end * 0.47
		kit.bar(
			materials["paint"],
			room.point(jamb, into - 0.03, 0.0),
			room.point(jamb, into - 0.03, 2.1),
			0.05,
			Color(0.55, 0.55, 0.5)
		)
	kit.bar(
		materials["paint"],
		room.point(across - 0.47, into - 0.03, 2.1),
		room.point(across + 0.47, into - 0.03, 2.1),
		0.05,
		Color(0.55, 0.55, 0.5)
	)
	var hood := Vector3(room.width * 0.62, 0.42, 0.8)
	kit.box(
		materials["paint"],
		Transform3D(basis, room.point(-across * 0.35, room.depth - 0.5, room.height - 0.3)),
		hood,
		Color(0.72, 0.72, 0.7)
	)


## Printed menus and notices stuck along a wall, a little crooked and lit by the room.
static func notices(
	kit: CityMesh,
	sheet: SignSheet,
	trade: String,
	room: Shopfronts.Room,
	material: Material,
	centre: Vector3,
	facing: Vector3,
	span: float,
	stream: int
) -> void:
	var run := Vector3.UP.cross(facing).normalized()
	var count := maxi(roundi(span / 1.1), 1)
	for i: int in count:
		var key := stream * 40 + i * 4
		if Rng.unit(room.dice, key) < 0.25:
			continue
		var pick := sheet.pick(trade, false, Rng.unit(room.dice, key + 1))
		var rect: Rect2 = pick["uv"]
		var wide := lerpf(0.6, 0.85, Rng.unit(room.dice, key + 2))
		var tall := wide * rect.size.y / rect.size.x
		var lean := (Rng.unit(room.dice, key + 3) - 0.5) * 0.05
		var at := centre + run * ((i + 0.5 - count * 0.5) * span / count) + facing * 0.012
		var half := (run + Vector3.UP * lean) * wide * 0.5
		var rise := (Vector3.UP - run * lean) * tall * 0.5
		var mapped: Array[Vector2] = [
			Vector2(rect.position.x, rect.end.y),
			Vector2(rect.end.x, rect.end.y),
			Vector2(rect.end.x, rect.position.y),
			Vector2(rect.position.x, rect.position.y),
		]
		var corners: Array[Vector3] = [
			at - half - rise, at + half - rise, at + half + rise, at - half + rise
		]
		kit.face(material, corners, mapped, facing, Color(0.62, 0.6, 0.55))


## Whatever was delivered and never carried in: cartons and crates either side of the door.
static func delivered(room: Shopfronts.Room) -> void:
	for end: float in [-1.0, 1.0]:
		var key := 100 + int(end) * 8
		var roll := Rng.unit(room.dice, key)
		if roll < 0.35:
			continue
		var across := end * (room.width * 0.5 - lerpf(0.65, 1.0, Rng.unit(room.dice, key + 1)))
		var at := room.point(across, -lerpf(0.42, 0.62, Rng.unit(room.dice, key + 2)), 0.0)
		var turn := Basis(Vector3.UP, (Rng.unit(room.dice, key + 3) - 0.5) * 0.7)
		if roll < 0.7:
			room.cartons.append(Transform3D(turn, at))
			if Rng.unit(room.dice, key + 4) < 0.6:
				var top := Basis(Vector3.UP, Rng.unit(room.dice, key + 5) * 0.8) * turn
				room.cartons.append(Transform3D(top, at + Vector3.UP * CARTON_HIGH))
		else:
			room.crates.append(Transform3D(turn, at))
			room.crates.append(Transform3D(turn, at + Vector3.UP * CRATE_HIGH))
			if Rng.unit(room.dice, key + 4) < 0.5:
				room.crates.append(Transform3D(turn, at + Vector3.UP * CRATE_HIGH * 2.0))


## A carton or two pushed up on top of a run of shelving, where the spare stock goes.
static func overstock(
	room: Shopfronts.Room, top: Vector3, run: Vector3, span: float, dice: int
) -> void:
	var count := maxi(roundi(span * 0.8), 1)
	for i: int in count:
		if Rng.unit(dice, 400 + i * 2) < 0.4:
			continue
		var along := ((i + 0.5) / count - 0.5) * span
		var turn := Basis(Vector3.UP, Rng.unit(dice, 401 + i * 2) * 0.5 - 0.25)
		room.cartons.append(Transform3D(turn, top + run * along))
