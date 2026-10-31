class_name Skyline
extends RefCounted
## The city around and above the street, as plain data: far towers behind the near ones,
## giant screens on the tall blocks, bridges and a rail line across the road, neon on the
## tower edges and searchlights on the roofs. These are what make it a place worth riding
## through. Generating it clears away any street sign standing in a bridge's way.

## How far past the ride's ends the far towers carry on.
const REACH := 420.0
const SPIRE_NEAREST := 55.0
const SPIRE_FURTHEST := 380.0
const SPIRE_SPACING := 26.0
const SCREEN_TOWER := 34.0
const BANNER_WIDTH := 6.5
const BRIDGE_DEPTH := 3.2
const RAIL_DEPTH := 7.0
## Colours as hues: mostly cold blues, with the odd warning red or amber.
const HUES: PackedFloat32Array = [0.56, 0.52, 0.6, 0.56, 0.02, 0.08]


class Spire:
	extends RefCounted
	var s := 0.0
	## Metres from the road's centre, signed by side of the road.
	var offset := 0.0
	var width := 0.0
	var height := 0.0
	var hue := 0.0
	## Tiers step in as they rise.
	var tiers := 1


class Screen:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var size := Vector2.ZERO
	var elevation := 0.0
	## Hung off the wall and facing up the road, or flat on the wall facing across it.
	var banner := false
	var trade := ""
	var pick := 0.0
	var hue := 0.0
	var hue_b := 0.0
	var phase := 0.0


class Span:
	extends RefCounted
	var s := 0.0
	var height := 0.0
	var rail := false
	var hue := 0.0
	var phase := 0.0


class Trim:
	extends RefCounted
	var tower: Street.Tower
	var hue := 0.0


class Beam:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var height := 0.0
	var hue := 0.0
	var phase := 0.0


var spires: Array[Spire] = []
var screens: Array[Screen] = []
var spans: Array[Span] = []
var trims: Array[Trim] = []
var beams: Array[Beam] = []


static func generate(run_seed: int, street: Street, route: Route, length: float) -> Skyline:
	var skyline := Skyline.new()
	skyline._raise_spires(run_seed, route, length)
	skyline._dress_towers(run_seed, street, length)
	skyline._bridge_the_road(run_seed, street, length)
	return skyline


func _raise_spires(run_seed: int, route: Route, length: float) -> void:
	var base := Rng.derive(run_seed, [50])
	var index := 0
	var s := -REACH
	while s < length + REACH:
		for side: float in [-1.0, 1.0]:
			var cell := Rng.derive(base, [index, int(side)])
			var spire := Spire.new()
			spire.s = s + Rng.unit(cell, 0) * SPIRE_SPACING
			# Closer in along the street, so some of them show over the near roofs.
			var far := pow(Rng.unit(cell, 1), 1.5)
			spire.offset = side * lerpf(SPIRE_NEAREST, SPIRE_FURTHEST, far)
			spire.width = lerpf(18.0, 46.0, Rng.unit(cell, 2))
			spire.height = lerpf(90.0, 240.0, Rng.unit(cell, 3)) * lerpf(1.0, 2.2, far)
			spire.hue = HUES[int(Rng.unit(cell, 4) * HUES.size())]
			spire.tiers = 1 + int(Rng.unit(cell, 5) * 3.0)
			# On a bend a tower meant for the far side can land back on the road.
			var at := route.at(spire.s) + route.right(spire.s) * spire.offset
			if absf(route.nearest(at.x, at.z).y) > SPIRE_NEAREST - spire.width * 0.5:
				spires.append(spire)
		s += SPIRE_SPACING
		index += 1


func _dress_towers(run_seed: int, street: Street, length: float) -> void:
	var base := Rng.derive(run_seed, [51])
	for index: int in street.towers.size():
		var tower := street.towers[index]
		var cell := Rng.derive(base, [index])
		if Rng.unit(cell, 0) < 0.1:
			var trim := Trim.new()
			trim.tower = tower
			trim.hue = HUES[int(Rng.unit(cell, 1) * HUES.size())]
			trims.append(trim)
		if tower.height < SCREEN_TOWER or tower.s < -60.0 or tower.s > length + 120.0:
			continue
		if Rng.unit(cell, 2) < 0.12:
			var wall := _screen(Rng.derive(cell, [1]), tower)
			wall.s = tower.s
			wall.size = Vector2(tower.width * 0.8, minf(14.0, tower.width * 0.8))
			screens.append(wall)
		if Rng.unit(cell, 3) < 0.14:
			var banner := _screen(Rng.derive(cell, [2]), tower)
			banner.banner = true
			banner.s = tower.s + (Rng.unit(cell, 4) - 0.5) * tower.width * 0.6
			banner.size = Vector2(BANNER_WIDTH, 14.0)
			screens.append(banner)
			_clear_signs(street, banner.s, 1.5, banner.elevation, banner.size.y, tower.side)
		if tower.height > 60.0 and Rng.unit(cell, 5) < 0.1:
			var beam := Beam.new()
			beam.s = tower.s
			beam.side = tower.side
			beam.height = tower.height
			beam.hue = HUES[int(Rng.unit(cell, 6) * HUES.size())]
			beam.phase = Rng.unit(cell, 7)
			beams.append(beam)


func _screen(cell: int, tower: Street.Tower) -> Screen:
	var screen := Screen.new()
	screen.side = tower.side
	screen.elevation = lerpf(13.0, 16.0, Rng.unit(cell, 0))
	screen.trade = Street.TRADES[int(Rng.unit(cell, 1) * Street.TRADES.size())]
	screen.pick = Rng.unit(cell, 2)
	screen.hue = HUES[int(Rng.unit(cell, 3) * HUES.size())]
	screen.hue_b = HUES[int(Rng.unit(cell, 4) * HUES.size())]
	screen.phase = Rng.unit(cell, 5)
	return screen


func _bridge_the_road(run_seed: int, street: Street, length: float) -> void:
	var base := Rng.derive(run_seed, [52])
	var s := -40.0
	var index := 0
	while s < length + 160.0:
		var cell := Rng.derive(base, [index])
		var span := Span.new()
		span.s = s
		# Every third crossing carries the railway, high over the footbridges.
		span.rail = index % 3 == 1
		span.height = (
			lerpf(18.0, 20.0, Rng.unit(cell, 0))
			if span.rail
			else lerpf(9.5, 14.0, Rng.unit(cell, 0))
		)
		span.hue = HUES[int(Rng.unit(cell, 1) * HUES.size())]
		span.phase = Rng.unit(cell, 2)
		spans.append(span)
		var reach := RAIL_DEPTH if span.rail else BRIDGE_DEPTH
		for side: float in [-1.0, 1.0]:
			_clear_signs(street, s, reach * 0.5 + 0.5, span.height - 1.5, 4.5, side)
		s += lerpf(34.0, 62.0, Rng.unit(cell, 3))
		index += 1


## Takes away any hanging sign that would stand in the way of something bigger.
static func _clear_signs(
	street: Street, s: float, reach: float, from: float, rise: float, side: float
) -> void:
	var kept: Array[Street.Sign] = []
	for hung: Street.Sign in street.signs:
		var clashes := (
			hung.side == side
			and absf(hung.s - s) < reach
			and hung.elevation < from + rise
			and hung.elevation + hung.size.y > from
		)
		if not clashes:
			kept.append(hung)
	street.signs = kept
