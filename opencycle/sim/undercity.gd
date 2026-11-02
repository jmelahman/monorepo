class_name Undercity
extends RefCounted
## The city above the street, as plain data: this road runs along the bottom of a shaft,
## under a concrete deck carrying the levels where the sky is. Pylons hold it up, ribs and
## ducts run under it, traffic streams past below it, and what light gets down here comes
## through a slot over the road.

const DECK_HEIGHT := 33.0
## How far the slot opens either side of the road, over a plaza.
const OCULUS_HALF := 18.0
const OCULUS_EASE := 18.0
const SLOT_HALF := 4.0
## How far past the ride's ends the deck carries on.
const REACH := 240.0
const PYLON_MIN := 42.0
const PYLON_MAX := 70.0
const RIB_MIN := 11.0
const RIB_MAX := 24.0
const SHAFT_MIN := 24.0
const SHAFT_MAX := 46.0
const DUCTS := 6
const LANES := 7
const LANE_LOW := 21.0
const LANE_HIGH := 30.0


class Pylon:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var width := 0.0
	var depth := 0.0


class Rib:
	extends RefCounted
	var s := 0.0
	var drop := 0.0
	var long := 0.0
	var lit := false


class Duct:
	extends RefCounted
	var height := 0.0
	## Metres from the road's centre, signed by side of the road.
	var offset := 0.0
	var thick := 0.0


class Shaft:
	extends RefCounted
	var s := 0.0
	var offset := 0.0
	var power := 0.0


class Lane:
	extends RefCounted
	var height := 0.0
	var offset := 0.0
	## Along the road or against it, which also decides headlights or tail lights.
	var direction := 1.0
	var speed := 0.0
	var phase := 0.0


var pylons: Array[Pylon] = []
var ribs: Array[Rib] = []
var ducts: Array[Duct] = []
var shafts: Array[Shaft] = []
var lanes: Array[Lane] = []
var districts: Districts


static func generate(run_seed: int, street: Street, kerb: float, length: float) -> Undercity:
	var city := Undercity.new()
	city.districts = street.districts
	var wall := kerb + Street.SIDEWALK
	city._raise_pylons(run_seed, street, wall, length)
	city._frame_deck(run_seed, wall, length)
	city._open_slot(run_seed, length)
	city._open_lanes(run_seed, kerb)
	return city


func _raise_pylons(run_seed: int, street: Street, wall: float, length: float) -> void:
	for side: float in [-1.0, 1.0]:
		var base := Rng.derive(run_seed, [60, int(side)])
		var s := -REACH + Rng.unit(base, 0) * PYLON_MAX
		var index := 0
		while s < length + REACH:
			var cell := Rng.derive(base, [index])
			var pylon := Pylon.new()
			pylon.s = s
			pylon.side = side
			pylon.width = lerpf(3.5, 6.0, Rng.unit(cell, 0))
			pylon.depth = minf(lerpf(2.0, 3.2, Rng.unit(cell, 1)), wall * 0.3)
			# Nothing to stand against out on a plaza, and nothing to hold up past the deck's end.
			if not _unframed(s):
				pylons.append(pylon)
				Skyline._clear_signs(street, s, pylon.width * 0.5 + 1.0, 0.0, DECK_HEIGHT, side)
			s += lerpf(PYLON_MIN, PYLON_MAX, Rng.unit(cell, 2))
			index += 1


func _unframed(s: float) -> bool:
	var kind := districts.kind_at(s)
	return Districts.stands_open(kind) or Districts.sky_open(kind)


func _frame_deck(run_seed: int, wall: float, length: float) -> void:
	var base := Rng.derive(run_seed, [61])
	var s := -REACH
	var index := 0
	while s < length + REACH:
		var cell := Rng.derive(base, [index])
		var rib := Rib.new()
		rib.s = s
		rib.drop = lerpf(1.4, 6.5, pow(Rng.unit(cell, 0), 2.0))
		rib.long = lerpf(1.2, 3.4, Rng.unit(cell, 1))
		rib.lit = Rng.unit(cell, 2) < 0.5
		if not _unframed(s):
			ribs.append(rib)
		s += lerpf(RIB_MIN, RIB_MAX, Rng.unit(cell, 3))
		index += 1
	for i: int in DUCTS:
		var cell := Rng.derive(base, [1000 + i])
		var duct := Duct.new()
		duct.height = lerpf(18.0, DECK_HEIGHT - 1.5, Rng.unit(cell, 0))
		var hugs := Rng.unit(cell, 1) * 2.0 - 1.0
		duct.offset = signf(hugs) * lerpf(wall - 1.4, wall - 0.5, absf(hugs))
		duct.thick = lerpf(0.6, 1.6, Rng.unit(cell, 2))
		ducts.append(duct)


func _open_slot(run_seed: int, length: float) -> void:
	var base := Rng.derive(run_seed, [62])
	var s := -30.0
	var index := 0
	while s < length + 80.0:
		var cell := Rng.derive(base, [index])
		var shaft := Shaft.new()
		shaft.s = s
		shaft.offset = (Rng.unit(cell, 0) * 2.0 - 1.0) * SLOT_HALF * 0.7
		shaft.power = lerpf(0.6, 1.0, Rng.unit(cell, 1))
		# Under a roof, there'd be nothing to see of it.
		var kind := districts.kind_at(s)
		if not Districts.roofed(kind) and not Districts.sky_open(kind):
			shafts.append(shaft)
		s += lerpf(SHAFT_MIN, SHAFT_MAX, Rng.unit(cell, 2))
		index += 1


func _open_lanes(run_seed: int, kerb: float) -> void:
	var base := Rng.derive(run_seed, [63])
	for i: int in LANES:
		var cell := Rng.derive(base, [i])
		var lane := Lane.new()
		lane.height = lerpf(LANE_LOW, LANE_HIGH, (i + Rng.unit(cell, 0)) / LANES)
		lane.direction = 1.0 if i % 2 == 0 else -1.0
		lane.offset = lane.direction * lerpf(1.0, kerb + 1.0, Rng.unit(cell, 1))
		lane.speed = lerpf(16.0, 34.0, Rng.unit(cell, 2))
		lane.phase = Rng.unit(cell, 3) * 400.0
		lanes.append(lane)


## How far the slot in the deck reaches either side of the road here.
func slot_half(s: float) -> float:
	var open := 0.0
	for stretch: Districts.Stretch in districts.stretches:
		if stretch.kind not in [Districts.Kind.PLAZA, Districts.Kind.YARD]:
			continue
		var inside := minf(s - stretch.from, stretch.to - s)
		open = maxf(open, smoothstep(0.0, OCULUS_EASE, inside))
	return lerpf(SLOT_HALF, OCULUS_HALF, open)
