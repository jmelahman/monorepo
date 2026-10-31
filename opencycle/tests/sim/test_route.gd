extends GdUnitTestSuite


func test_same_seed_gives_the_same_road() -> void:
	assert_vector(Route.generate(7).at(512.3)).is_equal(Route.generate(7).at(512.3))


func test_different_seeds_give_different_roads() -> void:
	assert_bool(Route.generate(7).at(800.0).is_equal_approx(Route.generate(8).at(800.0))).is_false()


func test_road_never_doubles_back_along_x() -> void:
	for run_seed: int in [1, 2, 3, 4, 5]:
		var route := Route.generate(run_seed)
		var s := -Route.MARGIN
		while s < Route.LENGTH + Route.MARGIN:
			assert_float(route.tangent(s).x).is_greater(0.0)
			s += 10.0


func test_nearest_finds_a_point_on_the_road() -> void:
	var route := Route.generate(3)
	for s: float in [0.0, 250.0, 777.0]:
		var at := route.at(s) + route.right(s) * 4.0
		var found := route.nearest(at.x, at.z)
		assert_float(found.x).is_equal_approx(s, 1.5)
		assert_float(found.y).is_equal_approx(4.0, 0.5)


func test_ride_climbs_then_descends() -> void:
	var route := Route.generate(3)
	assert_float(route.at(200.0).y).is_greater(route.at(0.0).y + 8.0)
	assert_float(route.at(1000.0).y).is_less(route.at(500.0).y - 15.0)
