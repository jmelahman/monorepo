class_name CityPlan
extends RefCounted
## The city as a ladder of roads: two long parallel streets, and hairpins between them at the end
## of each area. Every course starts up the north street and comes home down the south one, and
## how far it goes before turning back is what makes it the inner, medium or full loop. A road is
## seeded by which road it is, so it's the same street on every course that rides it.
##
##     U0 ( N1 > | N2 > | N3 > ) U3      inner:  N1 U1 S1 U0
##        ( < S1 | < S2 | < S3 )         medium: N1 N2 U2 S2 S1 U0
##             U1     U2                 full:   N1 N2 N3 U3 S3 S2 S1 U0

## Half the distance between the two streets, which is also the radius of every hairpin.
const APART := 220.0
const INNER_LONG := 560.0
const MIDDLE_LONG := 1850.0
const OUTER_LONG := 4000.0
const COURSES: Dictionary[String, Array] = {
	"inner": ["N1", "U1", "S1", "U0"],
	"medium": ["N1", "N2", "U2", "S2", "S1", "U0"],
	"full": ["N1", "N2", "N3", "U3", "S3", "S2", "S1", "U0"],
}
## Market between everything else, and how much of it. What's left at a road's end is market too.
const GAP := 60.0
## The market a road starts with where it's leaving another, which is room for the two to part.
const FORK_GAP := 150.0
const SPREAD := 0.1

var segments: Dictionary[String, CitySegment] = {}
var roads := RoadMap.new()


func _init(run_seed: int) -> void:
	var inner := INNER_LONG
	var middle := inner + MIDDLE_LONG
	var outer := middle + OUTER_LONG
	var north := Vector3(0.0, 0.0, -APART)
	var south := Vector3(0.0, 0.0, APART)
	var round_a_hairpin := PI * APART
	var east := Vector3.RIGHT
	var undercity: Array = [[Districts.Kind.UNDERPASS, 150.0]]
	_add(run_seed, "N1", north, 0.0, inner, 0.0, undercity, 250.0)
	_add(run_seed, "N2", north + east * inner, 0.0, MIDDLE_LONG, 0.0, _open_north(), FORK_GAP)
	_add(run_seed, "N3", north + east * middle, 0.0, OUTER_LONG, 0.0, _works_north(), FORK_GAP)
	var hairpin: Array = [[Districts.Kind.WORKS, 170.0], [Districts.Kind.PLAZA, 115.0]]
	_add(run_seed, "U1", north + east * inner, 0.0, round_a_hairpin, PI, hairpin, FORK_GAP)
	var terraces: Array = [[Districts.Kind.RESIDENTIAL, 300.0]]
	_add(run_seed, "U2", north + east * middle, 0.0, round_a_hairpin, PI, terraces, FORK_GAP)
	var far_end: Array = [
		[Districts.Kind.LANTERNS, 220.0],
		[Districts.Kind.PLAZA, 115.0],
		[Districts.Kind.WORKS, 180.0]
	]
	_add(run_seed, "U3", north + east * outer, 0.0, round_a_hairpin, PI, far_end)
	_add(run_seed, "S3", south + east * outer, PI, OUTER_LONG, 0.0, _works_south())
	_add(run_seed, "S2", south + east * middle, PI, MIDDLE_LONG, 0.0, _open_south())
	_add(run_seed, "S1", south + east * inner, PI, inner, 0.0, [[Districts.Kind.CISTERN, 260.0]])
	var home: Array = [[Districts.Kind.LANTERNS, 210.0], [Districts.Kind.YARD, 255.0]]
	_add(run_seed, "U0", south, PI, round_a_hairpin, PI, home)
	_fork("N2", "U1", true)
	_fork("N3", "U2", true)
	_fork("S2", "U1", false)
	_fork("S3", "U2", false)


func course(which: String) -> Course:
	var legs: Array[CitySegment] = []
	for segment: String in COURSES[which]:
		legs.append(segments[segment])
	return Course.new(legs, segments)


static func _open_north() -> Array:
	return [
		[Districts.Kind.RETAIL, 300.0],
		[Districts.Kind.RESIDENTIAL, 260.0],
		[Districts.Kind.PLAZA, 120.0],
		[Districts.Kind.RETAIL, 260.0],
		[Districts.Kind.LANTERNS, 200.0],
	]


static func _open_south() -> Array:
	return [
		[Districts.Kind.RESIDENTIAL, 280.0],
		[Districts.Kind.RETAIL, 260.0],
		[Districts.Kind.UNDERPASS, 150.0],
		[Districts.Kind.RESIDENTIAL, 280.0],
		[Districts.Kind.PLAZA, 120.0],
		[Districts.Kind.RETAIL, 240.0],
	]


static func _works_north() -> Array:
	return [
		[Districts.Kind.WORKS, 230.0],
		[Districts.Kind.YARD, 290.0],
		[Districts.Kind.UNDERPASS, 160.0],
		[Districts.Kind.CISTERN, 270.0],
		[Districts.Kind.WORKS, 210.0],
		[Districts.Kind.PLAZA, 120.0],
		[Districts.Kind.LANTERNS, 240.0],
		[Districts.Kind.YARD, 280.0],
		[Districts.Kind.WORKS, 220.0],
		[Districts.Kind.UNDERPASS, 150.0],
		[Districts.Kind.CISTERN, 260.0],
		[Districts.Kind.WORKS, 200.0],
		[Districts.Kind.YARD, 270.0],
	]


static func _works_south() -> Array:
	return [
		[Districts.Kind.YARD, 280.0],
		[Districts.Kind.WORKS, 220.0],
		[Districts.Kind.LANTERNS, 230.0],
		[Districts.Kind.UNDERPASS, 170.0],
		[Districts.Kind.WORKS, 240.0],
		[Districts.Kind.CISTERN, 280.0],
		[Districts.Kind.PLAZA, 125.0],
		[Districts.Kind.YARD, 260.0],
		[Districts.Kind.WORKS, 190.0],
		[Districts.Kind.UNDERPASS, 140.0],
		[Districts.Kind.LANTERNS, 220.0],
		[Districts.Kind.YARD, 290.0],
		[Districts.Kind.WORKS, 210.0],
	]


func _add(
	run_seed: int,
	which: String,
	start: Vector3,
	heading: float,
	long: float,
	turn: float,
	kinds: Array,
	first_gap := GAP
) -> void:
	var dice := Rng.derive(run_seed, [90, which.hash()])
	var road := Route.through(dice, start, heading, long, turn)
	segments[which] = CitySegment.new(which, dice, road, _order(kinds, first_gap), roads)
	roads.add(road)


## Two roads that part at one's start, or come together at its end.
func _fork(drawn_from: String, with: String, at_start: bool) -> void:
	var one := segments[drawn_from]
	var two := segments[with]
	one.forks.append(Fork.new(one.road, two.road, with, at_start, true))
	two.forks.append(Fork.new(two.road, one.road, drawn_from, at_start, false))


## What [method Districts.generate] takes, from just what's special about a road.
static func _order(kinds: Array, first_gap: float) -> Array[Array]:
	var order: Array[Array] = []
	var gap := first_gap
	for entry: Array in kinds:
		var long: float = entry[1]
		order.append([Districts.Kind.MARKET, gap * (1.0 - SPREAD), gap * (1.0 + SPREAD)])
		order.append([entry[0], long * (1.0 - SPREAD), long * (1.0 + SPREAD)])
		gap = GAP
	return order
