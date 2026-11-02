class_name Districts
extends RefCounted
## What the ride passes through, as stretches of the road: the market it starts in, an
## underpass with the roof down low, the works, a plaza where the deck opens up, a flooded
## cistern crossed on a causeway, a shopping street out in the open, a row of lanterns and gates,
## and a yard breaking up a hull. A ride that stays one place all the way along is over after its
## first minute.

enum Kind { MARKET, UNDERPASS, WORKS, PLAZA, CISTERN, LANTERNS, YARD, RETAIL }

## How far past the ride's ends things stay as they are.
const FOREVER := 10000.0
## In riding order, as a kind and how long it lasts. The market takes up whatever's left.
const ORDER: Array[Array] = [
	[Kind.MARKET, 230.0, 270.0],
	[Kind.UNDERPASS, 130.0, 170.0],
	[Kind.MARKET, 40.0, 60.0],
	[Kind.WORKS, 190.0, 230.0],
	[Kind.PLAZA, 100.0, 130.0],
	[Kind.MARKET, 90.0, 120.0],
	[Kind.CISTERN, 240.0, 280.0],
	[Kind.MARKET, 50.0, 70.0],
	[Kind.RETAIL, 220.0, 260.0],
	[Kind.MARKET, 40.0, 60.0],
	[Kind.LANTERNS, 220.0, 260.0],
	[Kind.MARKET, 60.0, 90.0],
	[Kind.YARD, 260.0, 300.0],
]
const GANTRY_MIN := 18.0
const GANTRY_MAX := 38.0
const GANTRY_LOW := 6.4
const GANTRY_HIGH := 9.5
const PIPES_MIN := 2
const PIPES_MAX := 6


class Stretch:
	extends RefCounted
	var kind := Kind.MARKET
	var from := 0.0
	var to := 0.0


## A pipe bridge over the road at the works.
class Gantry:
	extends RefCounted
	var s := 0.0
	var height := 0.0
	var pipes := 0
	var dice := 0


var stretches: Array[Stretch] = []
var gantries: Array[Gantry] = []


static func generate(run_seed: int, length: float) -> Districts:
	var districts := Districts.new()
	var base := Rng.derive(run_seed, [70])
	var s := 0.0
	for index: int in ORDER.size():
		var entry: Array = ORDER[index]
		var low: float = entry[1]
		var high: float = entry[2]
		var long := lerpf(low, high, Rng.unit(base, index))
		if s + long > length:
			break
		var kind: Kind = entry[0]
		districts._add(kind, s, s + long)
		s += long
	districts._add(Kind.MARKET, s, FOREVER)
	districts._cross_the_works(base)
	return districts


func _add(kind: Kind, from: float, to: float) -> void:
	var stretch := Stretch.new()
	stretch.kind = kind
	stretch.from = from
	stretch.to = to
	stretches.append(stretch)


func _cross_the_works(base: int) -> void:
	for stretch: Stretch in of_kind(Kind.WORKS):
		var s := stretch.from + GANTRY_MIN
		var index := 0
		while s < stretch.to - GANTRY_MIN * 0.5:
			var cell := Rng.derive(base, [100, index])
			var gantry := Gantry.new()
			gantry.s = s
			gantry.height = lerpf(GANTRY_LOW, GANTRY_HIGH, Rng.unit(cell, 0))
			gantry.pipes = PIPES_MIN + int(Rng.unit(cell, 1) * (PIPES_MAX - PIPES_MIN + 1))
			gantry.dice = Rng.derive(cell, [1])
			gantries.append(gantry)
			s += lerpf(GANTRY_MIN, GANTRY_MAX, Rng.unit(cell, 2))
			index += 1


## Whether the buildings come out here, leaving the street no walls.
static func stands_open(kind: Kind) -> bool:
	return kind in [Kind.PLAZA, Kind.CISTERN, Kind.YARD]


## Whether the road has a roof of its own here, under the deck.
static func roofed(kind: Kind) -> bool:
	return kind in [Kind.UNDERPASS, Kind.CISTERN]


## Whether the deck itself stops here, leaving nothing between the street and the weather.
static func sky_open(kind: Kind) -> bool:
	return kind == Kind.RETAIL


func kind_at(s: float) -> Kind:
	for stretch: Stretch in stretches:
		if s >= stretch.from and s < stretch.to:
			return stretch.kind
	return Kind.MARKET


func of_kind(kind: Kind) -> Array[Stretch]:
	var found: Array[Stretch] = []
	for stretch: Stretch in stretches:
		if stretch.kind == kind:
			found.append(stretch)
	return found


## How much rain gets down to the road here, from 0 under a roof to 1. Fades over a few
## metres at either end of a roof.
func open_to_the_sky(s: float) -> float:
	var open := 1.0
	for stretch: Stretch in stretches:
		if not roofed(stretch.kind):
			continue
		var inside := minf(s - stretch.from, stretch.to - s)
		open = minf(open, 1.0 - clampf((inside + 2.0) / 6.0, 0.0, 1.0))
	return open
