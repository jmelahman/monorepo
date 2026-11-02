class_name RoadMap
extends RefCounted
## Where every road in the city runs, so that what's built along one doesn't land on another.

const CELL := 64.0
const SAMPLE := 8.0

var _cells: Dictionary[Vector2i, PackedVector2Array] = {}


func add(road: Route) -> void:
	var s := 0.0
	while s <= road.length:
		var at := road.at(s)
		var key := _cell(at.x, at.z)
		if key not in _cells:
			_cells[key] = PackedVector2Array()
		_cells[key].append(Vector2(at.x, at.z))
		s += SAMPLE


## Whether any road passes within [param reach] metres, which has to fit in a [constant CELL].
func near(x: float, z: float, reach: float) -> bool:
	var middle := _cell(x, z)
	var here := Vector2(x, z)
	for across: int in range(-1, 2):
		for along: int in range(-1, 2):
			var key := middle + Vector2i(across, along)
			if key not in _cells:
				continue
			for point: Vector2 in _cells[key]:
				if here.distance_squared_to(point) < reach * reach:
					return true
	return false


static func _cell(x: float, z: float) -> Vector2i:
	return Vector2i(floori(x / CELL), floori(z / CELL))
