class_name Fork
extends RefCounted
## Where another road leaves this one, or comes in to join it. The two run together at first, so
## each keeps to its own side of the line halfway between them. Further on there's room for a
## pavement on each, then a building's sharp end, and only then for proper lots back to back.

## From the road's middle to the line of its walls.
const WALL := Route.HALF_WIDTH + 1.0 + Street.SIDEWALK
## How far behind a wall a shop's room can go.
const ROOMS := 8.5
const PAST := 300.0

## Only its road and its name: its segment holds forks of its own, and they'd keep each other alive.
var other: Route
var other_name := ""
var side := 1.0
## Whether this road is the one that builds what stands between the two.
var draws := false
var at_start := true
## How far from the junction the walls have met, and how far before lots fit behind them.
var prow := 0.0
var back := 0.0

var _road: Route


func _init(
	road: Route, to: Route, to_name: String, starts_here: bool, drawn_from_here: bool
) -> void:
	_road = road
	other = to
	other_name = to_name
	at_start = starts_here
	draws = drawn_from_here
	var apart := other.at(_other_s(100.0)) - road.at(along(100.0))
	side = signf(apart.dot(road.right(along(100.0))))
	var away := 0.0
	while reach(along(away)) < WALL:
		away += 1.0
	prow = away
	while reach(along(away)) < WALL + ROOMS:
		away += 1.0
	back = away


## Where on this road [param away] metres from the junction is.
func along(away: float) -> float:
	return away if at_start else _road.length - away


## How far this road's ground goes on this side at [param s], before the other road's starts.
func reach(s: float) -> float:
	var away := _away(s)
	if away > PAST:
		return INF
	var here := along(away)
	var there := _other_s(away)
	var apart := _road.at(here).distance_to(other.at(there))
	var angle := _road.tangent(here).angle_to(other.tangent(there))
	return apart * 0.5 / cos(angle * 0.5) + 0.05


## Whether this side of the road has no lots at [param s].
func covers(s: float) -> bool:
	return _away(s) < back


## A point out on this road's wall line, [param away] metres from the junction.
func wall_here(away: float, set_back: float) -> Vector3:
	var s := along(away)
	return _road.at(s) + _road.right(s) * side * (WALL + set_back)


## The same on the other road's wall line, facing this one.
func wall_there(away: float, set_back: float) -> Vector3:
	var s := _other_s(away)
	return other.at(s) - other.right(s) * side * (WALL + set_back)


func _away(s: float) -> float:
	return maxf(s if at_start else _road.length - s, 0.0)


func _other_s(away: float) -> float:
	return away if at_start else other.length - away
