class_name Replay
extends RefCounted
## A scripted ride: the speed and the camera's offset from the road, as plain data
## keyed by distance, so a ride is the same every time and on every machine. What
## drives the camera is only ever a time; nothing downstream knows it came from a
## script, so anything that can produce a position can replace this.

const EYE_HEIGHT := 1.5
const LOOK_AHEAD := 22.0
const TABLE_STEP := 0.5
## How long before the end of a lap the script starts going back to how it began.
const LAP_BLEND := 100.0
## Seconds a reading of the speed is taken over.
const SPEED_OVER := 1.0

## Metres the ride covers, up to the last [code]speed[/code] entry.
var length := 0.0
## Whether a ride goes on past that, round the same loop again.
var laps := false

## Seconds to reach each [constant TABLE_STEP] metres of the route.
var _times := PackedFloat32Array()
var _camera: Array[Dictionary] = []


## Builds a replay from [code]{"speed": [{at_m, kmh}], "camera": [{at_m, lateral,
## height}]}[/code], each list sorted by [code]at_m[/code]. [param lap], if given, is the length of
## a loop to ride round and round: the script is cut to fit and eases back to its start by the end.
static func from_dict(data: Dictionary, lap := 0.0) -> Replay:
	var replay := Replay.new()
	var speed: Array = data["speed"]
	var camera: Array = data["camera"]
	if lap > 0.0:
		speed = _closed(speed, lap)
		camera = _closed(camera, lap)
		replay.laps = true
	replay._camera.assign(camera)
	var last: Dictionary = speed[-1]
	replay.length = last["at_m"]
	var count := int(replay.length / TABLE_STEP) + 1
	replay._times.resize(count)
	var elapsed := 0.0
	for i: int in count:
		replay._times[i] = elapsed
		elapsed += TABLE_STEP / (_speed_at(speed, i * TABLE_STEP) / 3.6)
	return replay


func duration() -> float:
	return _times[-1]


static func _closed(knots: Array, lap: float) -> Array:
	var kept := []
	var last: Dictionary = knots[0]
	for entry: Variant in knots:
		var knot: Dictionary = entry
		var at: float = knot["at_m"]
		if at < lap - LAP_BLEND:
			kept.append(knot)
			last = knot
	var held := last.duplicate()
	held["at_m"] = lap - LAP_BLEND
	kept.append(held)
	var start: Dictionary = knots[0]
	var first := start.duplicate()
	first["at_m"] = lap
	kept.append(first)
	return kept


## Metres ridden at [param t] seconds, counting every lap.
func ridden(t: float) -> float:
	if not laps:
		return distance_at(t)
	var done := floorf(t / duration())
	return done * length + distance_at(t - done * duration())


## Which lap [param t] seconds is on, counting from 0.
func lap_at(t: float) -> int:
	return int(floorf(t / duration())) if laps else 0


## How fast the ride is going at [param t] seconds, in km/h.
func kmh_at(t: float) -> float:
	var before := maxf(t - SPEED_OVER * 0.5, 0.0)
	return (ridden(before + SPEED_OVER) - ridden(before)) / SPEED_OVER * 3.6


## Metres along the route at [param t] seconds. Past the end, it stays at the end.
func distance_at(t: float) -> float:
	var low := 0
	var high := _times.size() - 1
	if t >= _times[high]:
		return length
	while high - low > 1:
		@warning_ignore("integer_division")
		var middle := (low + high) / 2
		if _times[middle] <= t:
			low = middle
		else:
			high = middle
	return (low + (t - _times[low]) / (_times[high] - _times[low])) * TABLE_STEP


## Where the camera is at [param t] seconds, looking up the road.
func camera_at(route: Route, t: float) -> Transform3D:
	var s := ridden(t)
	var eye := _point(route, s, EYE_HEIGHT)
	var target := _point(route, s + LOOK_AHEAD, EYE_HEIGHT)
	return Transform3D(Basis.IDENTITY, eye).looking_at(target, Vector3.UP)


func _point(route: Route, s: float, eye: float) -> Vector3:
	var script_at := fposmod(s, length) if laps else s
	var lateral := _camera_value(script_at, "lateral")
	var height := _camera_value(script_at, "height")
	return route.at(s) + route.right(s) * lateral + Vector3.UP * (eye + height)


func _camera_value(s: float, key: String) -> float:
	var previous: Dictionary = _camera[0]
	for knot: Dictionary in _camera:
		var at: float = knot["at_m"]
		if s <= at:
			var from: float = previous["at_m"]
			var a: float = previous[key]
			var b: float = knot[key]
			return lerpf(a, b, smoothstep(from, at, s)) if at > from else b
		previous = knot
	return previous[key]


static func _speed_at(speed: Array, s: float) -> float:
	var previous: Dictionary = speed[0]
	for entry: Variant in speed:
		var knot: Dictionary = entry
		var at: float = knot["at_m"]
		if s <= at:
			var from: float = previous["at_m"]
			var a: float = previous["kmh"]
			var b: float = knot["kmh"]
			return lerpf(a, b, (s - from) / (at - from)) if at > from else b
		previous = knot
	return previous["kmh"]
