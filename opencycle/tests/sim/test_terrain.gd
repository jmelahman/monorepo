extends GdUnitTestSuite


func test_ground_meets_the_road_under_it() -> void:
	var route := Route.generate(5)
	var terrain := Terrain.generate(5, route)
	for s: float in [0.0, 300.0, 900.0]:
		var at := route.at(s)
		assert_float(terrain.height(at.x, at.z)).is_equal_approx(at.y - Terrain.ROAD_LIFT, 0.01)


func test_same_seed_gives_the_same_ground() -> void:
	var route := Route.generate(5)
	var a := Terrain.generate(5, route).height(120.0, 300.0)
	var b := Terrain.generate(5, route).height(120.0, 300.0)
	assert_float(a).is_equal(b)
