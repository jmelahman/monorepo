class_name Replay
extends RefCounted
## A scripted ride: the speed and the camera's offset from the road, as plain data
## keyed by distance, so a ride is the same every time and on every machine. What
## drives the camera is only ever a time; nothing downstream knows it came from a
## script, so anything that can produce a position can replace this.

const EYE_HEIGHT := 1.5
const LOOK_AHEAD := 22.0
const TABLE_STEP := 0.5

## Metres the ride covers, up to the last [code]speed[/code] entry.
var length := 0.0

## Seconds to reach each [constant TABLE_STEP] metres of the route.
var _times := PackedFloat32Array()
var _camera: Array[Dictionary] = []


## Builds a replay from [code]{"speed": [{at_m, kmh}], "camera": [{at_m, lateral,
## height}]}[/code], each list sorted by [code]at_m[/code].
static func from_dict(data: Dictionary) -> Replay:
	var replay := Replay.new()
	var speed: Array = data["speed"]
	var camera: Array = data["camera"]
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
	var s := distance_at(t)
	var eye := _point(route, s, EYE_HEIGHT)
	var target := _point(route, s + LOOK_AHEAD, EYE_HEIGHT)
	return Transform3D(Basis.IDENTITY, eye).looking_at(target, Vector3.UP)


func _point(route: Route, s: float, eye: float) -> Vector3:
	var lateral := _camera_value(s, "lateral")
	var height := _camera_value(s, "height")
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
