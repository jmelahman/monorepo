class_name Route
extends RefCounted
## A road through the world, as a function of distance along it. Rides happen
## over [code][0, LENGTH][/code]; the road carries on [constant MARGIN] metres at
## both ends so nobody ever sees it stop.
##
## The road runs roughly along +X and never doubles back, which is what lets
## [method nearest] be a table lookup and the terrain ask it for every vertex.

const LENGTH := 1000.0
const MARGIN := 300.0
const STEP := 2.0
const HALF_WIDTH := 3.0
## Distance over which one grade blends into the next.
const GRADE_BLEND := 60.0
## A road longer than [constant LENGTH] rolls gently from there on.
const ROLL := 0.025
const ROLL_LONG := 190.0
## Grade by distance: a climb, a rolling middle, then a long descent.
const GRADES: Array[Vector2] = [
	Vector2(-MARGIN, 0.01),
	Vector2(0.0, 0.06),
	Vector2(220.0, 0.005),
	Vector2(520.0, -0.055),
	Vector2(LENGTH, -0.01),
]

var _points := PackedVector3Array()
## Level unit direction of travel at each point; asked for far too often to derive.
var _tangents := PackedVector3Array()
var _x_start := 0.0
## Distance along the road at every [constant STEP] metres of X.
var _s_at_x := PackedFloat32Array()


static func generate(run_seed: int, length := LENGTH) -> Route:
	var route := Route.new()
	var phases: Array[float] = []
	for i: int in 3:
		phases.append(TAU * float(Rng.derive(run_seed, [1, i])) / float(Rng.MASK))
	var count := int((length + 2.0 * MARGIN) / STEP) + 1
	route._points.resize(count)
	var position := Vector3.ZERO
	for i: int in count:
		route._points[i] = position
		var s := -MARGIN + i * STEP
		var heading := (
			0.5 * sin(s / 170.0 + phases[0])
			+ 0.3 * sin(s / 83.0 + phases[1])
			+ 0.12 * sin(s / 41.0 + phases[2])
		)
		position += Vector3(cos(heading), _grade_at(s), sin(heading)) * STEP
	route._tangents.resize(count)
	for i: int in count:
		var flat := route._points[mini(i + 1, count - 1)] - route._points[maxi(i - 1, 0)]
		route._tangents[i] = Vector3(flat.x, 0.0, flat.z).normalized()
	route._index_by_x()
	return route


static func _grade_at(s: float) -> float:
	var grade := GRADES[0].y
	for knot: Vector2 in GRADES:
		grade = lerpf(grade, knot.y, smoothstep(knot.x, knot.x + GRADE_BLEND, s))
	return (
		grade + ROLL * sin((s - LENGTH) / ROLL_LONG) * smoothstep(LENGTH, LENGTH + GRADE_BLEND, s)
	)


## The road's centre, [param s] metres from the start of the ride.
func at(s: float) -> Vector3:
	var f := clampf((s + MARGIN) / STEP, 0.0, _points.size() - 1.001)
	var i := int(f)
	return _points[i].lerp(_points[i + 1], f - i)


## The unit direction of travel at [param s], level with the horizon.
func tangent(s: float) -> Vector3:
	var f := clampf((s + MARGIN) / STEP, 0.0, _points.size() - 1.001)
	var i := int(f)
	return _tangents[i].lerp(_tangents[i + 1], f - i).normalized()


## The unit direction to the rider's right at [param s].
func right(s: float) -> Vector3:
	return tangent(s).cross(Vector3.UP)


## The closest stretch of road to a point: [code]x[/code] is its distance along
## the road and [code]y[/code] how far the point is from it, in metres, positive
## to the rider's right. Close to the road this is close to exact; far away it is
## a good estimate, which is all the terrain and the planting ask of it.
func nearest(x: float, z: float) -> Vector2:
	var f := clampf((x - _x_start) / STEP, 0.0, _s_at_x.size() - 1.001)
	var i := int(f)
	var s := lerpf(_s_at_x[i], _s_at_x[i + 1], f - i)
	var centre := at(s)
	var along := tangent(s)
	# One step of correction: on a bend, the road opposite in X isn't the nearest.
	s += along.dot(Vector3(x - centre.x, 0.0, z - centre.z))
	centre = at(s)
	return Vector2(s, right(s).dot(Vector3(x - centre.x, 0.0, z - centre.z)))


## The bounds of the road in plan, as [code]Rect2(x, z, w, d)[/code].
func extent() -> Rect2:
	var rect := Rect2(_points[0].x, _points[0].z, 0.0, 0.0)
	for point: Vector3 in _points:
		rect = rect.expand(Vector2(point.x, point.z))
	return rect


func _index_by_x() -> void:
	_x_start = _points[0].x
	var count := int((_points[-1].x - _x_start) / STEP) + 2
	_s_at_x.resize(count)
	var cursor := 0
	for i: int in count:
		var x := _x_start + i * STEP
		while cursor < _points.size() - 2 and _points[cursor + 1].x < x:
			cursor += 1
		var span := _points[cursor + 1].x - _points[cursor].x
		var f := clampf((x - _points[cursor].x) / span, 0.0, 1.0)
		_s_at_x[i] = -MARGIN + (cursor + f) * STEP
