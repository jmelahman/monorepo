class_name Crowd
extends RefCounted
## Who is on the pavement and where they are at any moment, as a pure function of the seed
## and the ride's clock, so a replay shows the same people in the same places. Walkers keep
## to lanes at one speed each, which means nobody ever walks through anybody.

## Metres out from the kerb: clear of the poles along the kerb and the goods by the wall.
const LANES: Array[float] = [0.95, 1.45, 1.95, 2.45]
## Metres a second, one per lane, so the crowd isn't marching in step.
const LANE_SPEEDS: Array[float] = [1.3, 1.12, 1.05, 1.22]
## People in one lane are at least this far apart, and on average far more.
const MIN_GAP := 1.3
const MAX_GAP := 13.0
const LANE_WANDER := 0.08
const STAND_CHANCE := 0.4
## Clear of what a shop puts out, which stops within this of whoever stands near it.
const PROP_CLEARANCE := 0.75
## From the wall: in front of the shop, and behind the last lane of walkers.
const STAND_NEAR := 0.65
const STAND_FAR := 1.1


class Person:
	extends RefCounted
	var s := 0.0
	## Metres from the road's centre, signed by side of the road.
	var offset := 0.0
	## 1 or -1 along the route for walkers.
	var direction := 1.0
	## 0 for someone standing about.
	var speed := 0.0
	## Which of the baked people, up to the count asked for.
	var look := 0
	## How far through their walk they started, 0 to 1.
	var phase := 0.0
	var height := 1.0
	## Which way someone standing is facing, as a turn from along the route.
	var turn := 0.0


var people: Array[Person] = []
var start := 0.0
var end := 0.0


## People over [code][from, to][/code] of the route, beside the street they're on.
static func generate(
	run_seed: int, street: Street, from: float, to: float, kerb: float, looks: int
) -> Crowd:
	var crowd := Crowd.new()
	crowd.start = from
	crowd.end = to
	for side: float in [-1.0, 1.0]:
		for lane: int in LANES.size():
			crowd._fill_lane(Rng.derive(run_seed, [43, int(side), lane]), side, lane, kerb, looks)
	crowd._stand_at_shops(Rng.derive(run_seed, [44]), street, kerb, looks)
	return crowd


func _fill_lane(base: int, side: float, lane: int, kerb: float, looks: int) -> void:
	var s := start
	var index := 0
	while s < end - MIN_GAP:
		var cell := Rng.derive(base, [index])
		var person := _new_person(cell, looks)
		person.s = s
		person.offset = side * (kerb + LANES[lane] + (Rng.unit(cell, 3) - 0.5) * 2.0 * LANE_WANDER)
		# Each side keeps to its own way round, as a crowd sorts itself out.
		person.direction = side if lane * 2 < LANES.size() else -side
		person.speed = LANE_SPEEDS[lane]
		people.append(person)
		# Groups and gaps, not an even stream.
		s += lerpf(MIN_GAP, MAX_GAP, pow(Rng.unit(cell, 4), 2.0))
		index += 1


func _stand_at_shops(base: int, street: Street, kerb: float, looks: int) -> void:
	for index: int in street.shops.size():
		var shop := street.shops[index]
		var cell := Rng.derive(base, [index])
		if not shop.open or Rng.unit(cell, 3) > STAND_CHANCE:
			continue
		var person := _new_person(cell, looks)
		person.s = shop.s + (Rng.unit(cell, 4) - 0.5) * shop.width * 0.8
		var from_wall := lerpf(STAND_NEAR, STAND_FAR, Rng.unit(cell, 5))
		person.offset = shop.side * (kerb + Street.SIDEWALK - from_wall)
		person.turn = Rng.unit(cell, 6) * TAU
		if _is_clear_of_props(person, street):
			people.append(person)


func _is_clear_of_props(person: Person, street: Street) -> bool:
	var at := Vector2(person.s, person.offset)
	for prop: Street.Prop in street.props:
		if at.distance_to(Vector2(prop.s, prop.offset)) < PROP_CLEARANCE:
			return false
	return true


func _new_person(cell: int, looks: int) -> Person:
	var person := Person.new()
	person.look = int(Rng.unit(cell, 0) * looks)
	person.phase = Rng.unit(cell, 1)
	person.height = lerpf(0.93, 1.06, Rng.unit(cell, 2))
	return person


## How far along the route [param person] is at [param time], looping round the street's
## ends, which are out of sight of the ride.
func s_at(person: Person, time: float) -> float:
	return start + fposmod(person.s - start + person.direction * person.speed * time, end - start)
