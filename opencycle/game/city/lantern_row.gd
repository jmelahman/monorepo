class_name LanternRow
extends RefCounted
## A street somebody still looks after, in their own way: oxide-red gates over the road one
## after another, ropes of paper lanterns strung wall to wall above them, and shrines on the
## pavement with their smoke going up. It's the one warm place down here, and it isn't friendly.

const GATE_MIN := 9.0
const GATE_MAX := 14.0
const GATE_LOW := 5.8
const GATE_HIGH := 7.0
const POST := 0.62
const OXIDE := Color(0.72, 0.2, 0.1)
const OXIDE_DARK := Color(0.34, 0.1, 0.06)
const ENTRY_HIGH := 10.5
const ENTRY_POST := 1.5
const ROPE_EVERY := 5.5
const ROPE_LOW := 6.8
const ROPE_HIGH := 11.0
const SAG_MIN := 0.5
const SAG_MAX := 1.5
const ROPE_LANTERNS_MIN := 6
const ROPE_LANTERNS_MAX := 10
const LANTERN_DEAD := 0.14
const LANTERN_AMBER := 0.16
## Vertex colours stop at 1. [member Quarters.Parts.paper] is what takes these past white.
const PAPER := Color(1.0, 0.2, 0.05)
const PAPER_AMBER := Color(1.0, 0.46, 0.1)
const PAPER_DEAD := Color(0.09, 0.022, 0.012)
## From the cap down: the middle of a lantern, where the paper's thin, is what glows.
const PROFILE_RADIUS: PackedFloat32Array = [0.28, 0.62, 0.92, 1.0, 0.92, 0.62, 0.28]
const PROFILE_HIGH: PackedFloat32Array = [0.72, 0.56, 0.3, 0.0, -0.3, -0.56, -0.72]
const PROFILE_GLOW: PackedFloat32Array = [0.3, 0.62, 1.0, 1.0, 0.8, 0.45]
const LANTERN_SIDES := 12
const GLOW := Color(1.0, 0.3, 0.1)
const GLOW_EVERY := 2
const SHADOW_EVERY := 4
const SHADOW_HIGH := 14.0
const BANNER_CHANCE := 0.55
const BANNERS: Array[Color] = [
	Color(1.0, 0.16, 0.08), Color(0.9, 0.56, 0.14), Color(0.3, 0.3, 0.34)
]
const SHRINE_EVERY := 26.0
const CANDLE := Color(4.0, 2.1, 0.7)


## The gates, ropes and shrines of [param stretch] whose place lies in [param section].
static func build(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	stretch: Districts.Stretch,
	section: StreetSection,
	parts: Quarters.Parts
) -> void:
	var index := 0
	var s := stretch.from + GATE_MAX
	while s < stretch.to - GATE_MIN:
		var dice := Rng.derive(index, [110])
		if section.holds(s):
			kit.at(s)
			_gate(kit, root, route, s, index, dice, parts)
		s += lerpf(GATE_MIN, GATE_MAX, Rng.unit(dice, 0))
		index += 1
	index = 0
	s = stretch.from + ROPE_EVERY
	while s < stretch.to:
		if section.holds(s):
			kit.at(s)
			_rope(kit, route, s, Rng.derive(index, [111]), parts)
		s += ROPE_EVERY
		index += 1
	index = 0
	s = stretch.from + SHRINE_EVERY * 0.5
	while s < stretch.to - 6.0:
		if section.holds(s):
			kit.at(s)
			_shrine(kit, root, route, s, index, Rng.derive(index, [112]), parts)
		s += SHRINE_EVERY
		index += 1
	for end: float in [stretch.from + 2.0, stretch.to - 2.0]:
		if section.holds(end):
			kit.at(end)
			_entry(kit, root, route, end, parts)


static func _gate(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	s: float,
	index: int,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var basis := Quarters._frame(route, s)
	var across := route.right(s)
	var base := route.at(s)
	var high := lerpf(GATE_LOW, GATE_HIGH, Rng.unit(dice, 1))
	var reach := NeonBiome.STREET_HALF + 0.55
	var worn := lerpf(0.6, 1.05, Rng.unit(dice, 2))
	var paint := OXIDE * worn
	for side: float in [-1.0, 1.0]:
		var foot := base + across * side * reach
		kit.block(
			parts.steel,
			Transform3D(basis, foot + Vector3.UP * high * 0.5),
			Vector3(POST, high, POST),
			paint,
			0.04
		)
		# Where it's been hit and stood in the wet, the paint has gone.
		kit.block(
			parts.rust,
			Transform3D(basis, foot + Vector3.UP * 0.55),
			Vector3(POST + 0.06, 1.1, POST + 0.06),
			Color(0.62, 0.5, 0.42),
			0.03
		)
	var over := reach + 1.5
	kit.block(
		parts.steel,
		Transform3D(basis, base + Vector3.UP * (high + 0.18)),
		Vector3(over * 2.0, 0.42, 0.5),
		paint,
		0.04
	)
	# The tie below it, and the stubs between: the plainest way to say what these gates say.
	kit.box(
		parts.steel,
		Transform3D(basis, base + Vector3.UP * (high - 1.0)),
		Vector3(reach * 2.0, 0.22, 0.3),
		OXIDE_DARK * worn
	)
	var stub := -reach * 0.66
	while stub < reach:
		kit.box(
			parts.steel,
			Transform3D(basis, base + across * stub + Vector3.UP * (high - 0.42)),
			Vector3(0.16, 0.8, 0.2),
			OXIDE_DARK * worn
		)
		stub += reach * 0.66
	var cap := base + Vector3.UP * (high + 0.72)
	kit.block(
		parts.steel, Transform3D(basis, cap), Vector3((over + 0.5) * 2.0, 0.12, 1.0), paint, 0.02
	)
	for side: float in [-1.0, 1.0]:
		var end := cap + across * side * (over + 0.5)
		kit.bar(parts.steel, end, end + across * side * 0.5 + Vector3.UP * 0.55, 0.12, paint)
		var strut := base + across * side * reach
		kit.box(
			parts.steel,
			Transform3D(basis, strut + Vector3.UP * (high + 0.5)),
			Vector3(0.24, 0.4, 0.3),
			OXIDE_DARK * worn
		)
		if Rng.unit(dice, 3 + int(side + 1.0)) < BANNER_CHANCE:
			var hang := strut - across * side * lerpf(0.9, 3.4, Rng.unit(dice, 8))
			_banner(kit, basis, hang + Vector3.UP * (high - 1.1), dice, parts)
	var hung := base + across * lerpf(-1.5, 1.5, Rng.unit(dice, 10))
	var size := lerpf(0.42, 0.58, Rng.unit(dice, 11))
	var drop := high - 1.1 - size * 1.3
	kit.bar(parts.steel, hung + Vector3.UP * (high - 1.0), hung + Vector3.UP * drop, 0.02, paint)
	_lantern(kit, hung + Vector3.UP * (drop - size * 0.7), size, PAPER, parts)
	if index % GLOW_EVERY == 0:
		root.add_child(_glow(hung + Vector3.UP * (drop - size * 0.7), 4.0, 15.0))
	if index % SHADOW_EVERY == 0:
		root.add_child(_from_above(base + Vector3.UP * SHADOW_HIGH))


## Two widths of cloth stitched, so it hangs with a bend in it and the lanterns catch the bend.
static func _banner(
	kit: CityMesh, basis: Basis, top: Vector3, dice: int, parts: Quarters.Parts
) -> void:
	var wide := lerpf(0.4, 0.62, Rng.unit(dice, 12))
	var long := lerpf(2.0, 3.3, Rng.unit(dice, 13))
	var hue := BANNERS[int(Rng.unit(dice, 14) * BANNERS.size())]
	var lean := basis.z * lerpf(-0.14, 0.14, Rng.unit(dice, 15))
	var half := basis.x * wide * 0.5
	var knee := top - Vector3.UP * long * 0.55 + lean
	var hem := top - Vector3.UP * long - lean * 0.4
	var facing := basis.z
	kit.quad(parts.cloth, top - half, top + half, knee + half, knee - half, facing, hue)
	kit.quad(parts.cloth, knee - half, knee + half, hem + half, hem - half, facing, hue * 0.8)


static func _rope(kit: CityMesh, route: Route, s: float, dice: int, parts: Quarters.Parts) -> void:
	var across := route.right(s)
	var base := route.at(s)
	var wall := Quarters._wall()
	var high := lerpf(ROPE_LOW, ROPE_HIGH, Rng.unit(dice, 0))
	var sag := lerpf(SAG_MIN, SAG_MAX, Rng.unit(dice, 1))
	var tilt := lerpf(-0.6, 0.6, Rng.unit(dice, 2))
	var count := (
		ROPE_LANTERNS_MIN + int(Rng.unit(dice, 3) * (ROPE_LANTERNS_MAX - ROPE_LANTERNS_MIN + 1))
	)
	var hang := func(x: float) -> Vector3:
		var t := x / wall
		var y := high + tilt * t - sag * (1.0 - t * t)
		return base + across * x + Vector3.UP * y
	var cord := Color(0.05, 0.04, 0.035)
	var pieces := 8
	for piece: int in pieces:
		var a: float = lerpf(-wall, wall, float(piece) / pieces)
		var b: float = lerpf(-wall, wall, float(piece + 1) / pieces)
		var from: Vector3 = hang.call(a)
		var to: Vector3 = hang.call(b)
		kit.bar(parts.steel, from, to, 0.022, cord)
	for lantern: int in count:
		var x := lerpf(-wall, wall, (lantern + 0.5) / count)
		x += lerpf(-0.3, 0.3, Rng.unit(dice, 10 + lantern))
		var roll := Rng.unit(dice, 40 + lantern)
		var hue := PAPER
		if roll < LANTERN_DEAD:
			hue = PAPER_DEAD
		elif roll < LANTERN_DEAD + LANTERN_AMBER:
			hue = PAPER_AMBER
		var size := lerpf(0.2, 0.32, Rng.unit(dice, 70 + lantern))
		var top: Vector3 = hang.call(x)
		_lantern(kit, top - Vector3.UP * (size * 0.95), size, hue, parts)


## A paper lantern, as rings of colour, with a black cap and a tassel: at this size that's all
## that says lantern and not lamp.
static func _lantern(
	kit: CityMesh, at: Vector3, size: float, hue: Color, parts: Quarters.Parts
) -> void:
	for band: int in PROFILE_GLOW.size():
		var glow := PROFILE_GLOW[band]
		var color := Color(hue.r * glow, hue.g * glow * glow, hue.b * glow * glow)
		kit.ring(
			parts.paper,
			at + Vector3.UP * PROFILE_HIGH[band] * size,
			at + Vector3.UP * PROFILE_HIGH[band + 1] * size,
			PROFILE_RADIUS[band] * size,
			PROFILE_RADIUS[band + 1] * size,
			color,
			LANTERN_SIDES
		)
	var dark := Color(0.03, 0.025, 0.02)
	var up := Vector3.UP * size
	kit.tube(parts.steel, at + up * 0.68, at + up * 0.82, size * 0.32, dark, 6)
	kit.tube(parts.steel, at - up * 0.68, at - up * 0.8, size * 0.32, dark, 6)
	kit.bar(parts.steel, at - up * 0.8, at - up * 1.7, 0.018, Color(0.3, 0.05, 0.03))


static func _glow(at: Vector3, energy: float, reach: float) -> OmniLight3D:
	var lamp := OmniLight3D.new()
	lamp.light_color = GLOW
	lamp.light_energy = energy
	lamp.light_volumetric_fog_energy = 0.25
	lamp.omni_range = reach
	lamp.position = at
	return lamp


## From up past the ropes, so everything strung and hung over the road lands on it as shadow.
static func _from_above(at: Vector3) -> SpotLight3D:
	var beam := SpotLight3D.new()
	beam.light_color = Color(1.0, 0.42, 0.18)
	beam.light_energy = 12.0
	beam.light_volumetric_fog_energy = 0.1
	beam.spot_range = SHADOW_HIGH + 6.0
	beam.spot_angle = 58.0
	beam.spot_attenuation = 0.7
	beam.shadow_enabled = true
	beam.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
	beam.position = at
	return beam


## A cabinet on the pavement with its back to the wall: candles, a pot of sticks and their smoke.
static func _shrine(
	kit: CityMesh,
	root: Node3D,
	route: Route,
	s: float,
	index: int,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var side := 1.0 if Rng.unit(dice, 0) < 0.5 else -1.0
	var basis := Quarters._frame(route, s)
	var out := route.right(s) * side
	var off := Quarters._wall() - 0.5
	var foot := route.at(s) + out * off + Vector3.UP * StreetGround.KERB_HEIGHT
	var red := OXIDE * lerpf(0.55, 0.9, Rng.unit(dice, 1))
	var size := Vector3(0.7, 0.9, 1.5)
	kit.block(
		parts.concrete,
		Transform3D(basis, foot + Vector3.UP * size.y * 0.5),
		size,
		Color(0.55, 0.56, 0.56),
		0.04
	)
	var box_high := 1.5
	var inside := foot + Vector3.UP * (size.y + box_high * 0.5)
	kit.box(
		parts.steel,
		Transform3D(basis, inside + out * 0.32),
		Vector3(0.06, box_high, size.z),
		red * 0.5
	)
	for end: float in [-1.0, 1.0]:
		kit.box(
			parts.steel,
			Transform3D(basis, inside + basis.z * end * size.z * 0.5),
			Vector3(size.x, box_high, 0.06),
			red
		)
	var roof := foot + Vector3.UP * (size.y + box_high + 0.05)
	kit.block(parts.steel, Transform3D(basis, roof), Vector3(1.0, 0.1, 1.9), red, 0.02)
	for candle: int in 5:
		var along := lerpf(-0.55, 0.55, (candle + 0.5) / 5.0)
		var tall := lerpf(0.08, 0.22, Rng.unit(dice, 2 + candle))
		var at := foot + out * lerpf(-0.2, 0.15, Rng.unit(dice, 9 + candle)) + basis.z * along
		at += Vector3.UP * (size.y + tall * 0.5)
		kit.box(
			parts.glow, Transform3D(basis, at), Vector3(0.04, tall, 0.04), Color(0.9, 0.75, 0.5)
		)
		var flame := at + Vector3.UP * (tall * 0.5 + 0.03)
		kit.box(parts.glow, Transform3D(basis, flame), Vector3(0.025, 0.06, 0.025), CANDLE)
	var lamp := OmniLight3D.new()
	lamp.light_color = Color(1.0, 0.58, 0.24)
	lamp.light_energy = 2.2
	lamp.light_volumetric_fog_energy = 0.8
	lamp.omni_range = 7.0
	lamp.position = foot + Vector3.UP * (size.y + 0.5) - out * 0.3
	root.add_child(lamp)
	root.add_child(Weather.steam(route, Vector2(s, side * off), float(index) * 0.71))


## The way in: a gate meant to be seen down the road, with lanterns in a row under its beam.
static func _entry(
	kit: CityMesh, root: Node3D, route: Route, s: float, parts: Quarters.Parts
) -> void:
	var basis := Quarters._frame(route, s)
	var across := route.right(s)
	var base := route.at(s)
	var wall := Quarters._wall()
	for side: float in [-1.0, 1.0]:
		var foot := base + across * side * (wall - ENTRY_POST * 0.5)
		kit.block(
			parts.steel,
			Transform3D(basis, foot + Vector3.UP * ENTRY_HIGH * 0.5),
			Vector3(ENTRY_POST, ENTRY_HIGH, ENTRY_POST * 1.4),
			OXIDE * 0.8,
			0.06
		)
		kit.block(
			parts.concrete,
			Transform3D(basis, foot + Vector3.UP * 1.0),
			Vector3(ENTRY_POST + 0.6, 2.0, ENTRY_POST * 1.4 + 0.6),
			Color(0.5, 0.5, 0.5),
			0.08
		)
	for layer: int in 3:
		var high := ENTRY_HIGH + 0.45 + layer * 0.95
		var wide := (wall + 1.0 + layer * 1.3) * 2.0
		kit.block(
			parts.steel,
			Transform3D(basis, base + Vector3.UP * high),
			Vector3(wide, 0.7, 1.5 - layer * 0.25),
			OXIDE * (0.85 - layer * 0.14),
			0.05
		)
	var count := 7
	for lantern: int in count:
		var x := lerpf(-wall + 3.0, wall - 3.0, (lantern + 0.5) / count)
		var top := base + across * x + Vector3.UP * ENTRY_HIGH
		kit.bar(parts.steel, top, top - Vector3.UP * 0.9, 0.025, OXIDE_DARK)
		_lantern(kit, top - Vector3.UP * 1.45, 0.62, PAPER, parts)
	root.add_child(_glow(base + Vector3.UP * (ENTRY_HIGH - 2.5), 9.0, 26.0))
	# Out in front, or the side the rider comes to is just a black shape.
	root.add_child(_glow(base - route.tangent(s) * 5.0 + Vector3.UP * 6.0, 7.0, 18.0))
