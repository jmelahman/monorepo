class_name Terrain
extends RefCounted
## The ground as a function of plan position: rough hills that rise away from the
## road and blend down onto it, so the road cuts and banks through them.

const PAD := 700.0
## How far from the road's edge the ground has fully left the road's level.
const BLEND := 32.0
## The road's surface sits this far above the ground under it, so they never fight.
const ROAD_LIFT := 0.06
## A shoulder of flat ground beyond the road's edge.
const SHOULDER := 1.6
const HILL_CELL := 8.0
const HILL_SCALE := 1.0 / 260.0
const DETAIL_SCALE := 1.0 / 7.0

var bounds: Rect2
var route: Route

var _hills := PackedFloat32Array()
var _columns := 0
var _rows := 0
var _detail: ValueNoise


static func generate(run_seed: int, road: Route) -> Terrain:
	var terrain := Terrain.new()
	terrain.route = road
	terrain.bounds = road.extent().grow(PAD)
	terrain._detail = ValueNoise.new(Rng.derive(run_seed, [3]))
	terrain._bake_hills(ValueNoise.new(Rng.derive(run_seed, [2])))
	return terrain


## The height of the ground at a plan position.
func height(x: float, z: float) -> float:
	var near := route.nearest(x, z)
	var road_y := route.at(near.x).y - ROAD_LIFT
	var distance := absf(near.y) - Route.HALF_WIDTH - SHOULDER
	var blend := smoothstep(0.0, BLEND, distance)
	if blend <= 0.0:
		return road_y
	# Taller and higher with distance: a valley, with the road along its floor.
	var amplitude := 3.0 + minf(distance / 300.0, 1.0) * 60.0
	var rise := minf(distance, 700.0) * 0.07
	var natural := road_y + _hill(x, z) * amplitude + rise
	natural += _detail.value(x * DETAIL_SCALE, z * DETAIL_SCALE) * 0.35
	return lerpf(road_y, natural, blend)


## The ground's unit normal at a plan position, measured over [param span] metres.
func normal(x: float, z: float, span: float = 2.0) -> Vector3:
	var dx := height(x + span, z) - height(x - span, z)
	var dz := height(x, z + span) - height(x, z - span)
	return Vector3(-dx, 2.0 * span, -dz).normalized()


func _bake_hills(noise: ValueNoise) -> void:
	_columns = int(bounds.size.x / HILL_CELL) + 2
	_rows = int(bounds.size.y / HILL_CELL) + 2
	_hills.resize(_columns * _rows)
	for row: int in _rows:
		for column: int in _columns:
			var x := (bounds.position.x + column * HILL_CELL) * HILL_SCALE
			var z := (bounds.position.y + row * HILL_CELL) * HILL_SCALE
			_hills[row * _columns + column] = noise.fractal(x, z, 4)


func _hill(x: float, z: float) -> float:
	var fx := clampf((x - bounds.position.x) / HILL_CELL, 0.0, _columns - 1.001)
	var fz := clampf((z - bounds.position.y) / HILL_CELL, 0.0, _rows - 1.001)
	var column := int(fx)
	var row := int(fz)
	var i := row * _columns + column
	var near := lerpf(_hills[i], _hills[i + 1], fx - column)
	var far := lerpf(_hills[i + _columns], _hills[i + _columns + 1], fx - column)
	return lerpf(near, far, fz - row)
