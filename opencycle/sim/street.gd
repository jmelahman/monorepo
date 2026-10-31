class_name Street
extends RefCounted
## What stands where along a city street, as plain data: towers shoulder to shoulder
## down both sides, and signs and lamps fixed to them. The rules, not the art.

## How far past the ride's ends the street carries on.
const REACH := 170.0
const LOT_MIN := 11.0
const LOT_MAX := 26.0
const SIDEWALK := 4.0
const LAMP_SPACING := 22.0


class Tower:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var width := 0.0
	var depth := 0.0
	var height := 0.0
	var hue := 0.0
	## Tiers step back as they rise: 0 for a plain slab.
	var tiers := 0


class Sign:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var size := Vector2.ZERO
	var elevation := 0.0
	## Metres out over the sidewalk: 0 flat on the wall, more for a blade sign.
	var overhang := 0.0
	var hue := 0.0


var towers: Array[Tower] = []
var signs: Array[Sign] = []
var lamps: Array[float] = []


## A street along the route over [code][0, length][/code].
static func generate(run_seed: int, length: float) -> Street:
	var street := Street.new()
	for side: float in [-1.0, 1.0]:
		street._line_side(run_seed, side, length)
	var s := -REACH
	while s < length + REACH:
		street.lamps.append(s)
		s += LAMP_SPACING
	return street


func _line_side(run_seed: int, side: float, length: float) -> void:
	var base := Rng.derive(run_seed, [40, int(side)])
	var s := -REACH
	var index := 0
	while s < length + REACH:
		var cell := Rng.derive(base, [index])
		var tower := Tower.new()
		tower.side = side
		tower.width = lerpf(LOT_MIN, LOT_MAX, Rng.unit(cell, 0))
		tower.s = s + tower.width * 0.5
		tower.depth = lerpf(14.0, 26.0, Rng.unit(cell, 1))
		# Mostly mid-rise with the odd skyscraper, so the skyline has a rhythm.
		tower.height = (
			lerpf(22.0, 60.0, Rng.unit(cell, 2)) * (2.4 if Rng.unit(cell, 3) > 0.8 else 1.0)
		)
		# Two palettes: cyan-to-blue and magenta-to-amber.
		var warm := Rng.unit(cell, 4) > 0.5
		tower.hue = (
			fposmod(lerpf(0.86, 1.08, Rng.unit(cell, 5)), 1.0)
			if warm
			else lerpf(0.5, 0.62, Rng.unit(cell, 5))
		)
		tower.tiers = int(Rng.unit(cell, 6) * 3.0)
		towers.append(tower)
		_hang_signs(cell, tower)
		s += tower.width
		index += 1


func _hang_signs(cell: int, tower: Tower) -> void:
	for i: int in 3:
		if Rng.unit(cell, 10 + i) > 0.62:
			continue
		var hung := Sign.new()
		hung.side = tower.side
		hung.s = tower.s + (Rng.unit(cell, 20 + i) - 0.5) * tower.width * 0.7
		hung.hue = fposmod(tower.hue + (Rng.unit(cell, 30 + i) - 0.5) * 0.18, 1.0)
		if Rng.unit(cell, 40 + i) > 0.45:
			# A tall blade, end-on to the rider, sticking out over the sidewalk.
			hung.size = Vector2(
				lerpf(1.4, 2.4, Rng.unit(cell, 50 + i)), lerpf(5.0, 14.0, Rng.unit(cell, 60 + i))
			)
			hung.overhang = hung.size.x
			hung.elevation = lerpf(5.0, 12.0, Rng.unit(cell, 70 + i))
		else:
			# A wide banner flat on the shopfront.
			hung.size = Vector2(
				lerpf(4.0, tower.width * 0.7, Rng.unit(cell, 50 + i)),
				lerpf(1.0, 2.6, Rng.unit(cell, 60 + i))
			)
			hung.elevation = lerpf(3.2, 9.0, Rng.unit(cell, 70 + i))
		signs.append(hung)
