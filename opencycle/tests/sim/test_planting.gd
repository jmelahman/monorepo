extends GdUnitTestSuite

var _route: Route
var _terrain: Terrain
var _plants: Dictionary[Planting.Kind, Array]


func before() -> void:
	_route = Route.generate(9)
	_terrain = Terrain.generate(9, _route)
	_plants = Planting.generate(9, _terrain)


func test_planting_is_deterministic() -> void:
	var again := Planting.generate(9, _terrain)
	for kind: Planting.Kind in _plants:
		assert_int(again[kind].size()).is_equal(_plants[kind].size())
	var first: Transform3D = _plants[Planting.Kind.FIR][0]
	var second: Transform3D = again[Planting.Kind.FIR][0]
	assert_bool(first.is_equal_approx(second)).is_true()


func test_both_species_grow() -> void:
	assert_int(_plants[Planting.Kind.FIR].size()).is_greater(1000)
	assert_int(_plants[Planting.Kind.PINE].size()).is_greater(1000)


func test_no_tree_grows_on_the_road() -> void:
	for kind: Planting.Kind in [Planting.Kind.FIR, Planting.Kind.PINE]:
		for entry: Variant in _plants[kind]:
			var placed: Transform3D = entry
			var off_road := absf(_route.nearest(placed.origin.x, placed.origin.z).y)
			assert_float(off_road).is_greater(Route.HALF_WIDTH)


func test_trees_stand_on_the_ground() -> void:
	var placed: Transform3D = _plants[Planting.Kind.PINE][10]
	var ground := _terrain.height(placed.origin.x, placed.origin.z)
	assert_float(placed.origin.y).is_equal_approx(ground, 0.001)
