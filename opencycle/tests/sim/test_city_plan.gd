extends GdUnitTestSuite


func _plan() -> CityPlan:
	return CityPlan.new(4)


func test_every_course_closes_on_itself() -> void:
	var plan := _plan()
	for which: String in CityPlan.COURSES:
		var course := plan.course(which)
		var seam := course.at(0.0).distance_to(course.at(course.length - 0.001))
		assert_float(seam).is_less(0.1)
		assert_float(course.tangent(0.0).dot(course.tangent(course.length - 0.001))).is_greater(
			0.999
		)


func test_roads_meet_without_a_step_or_a_kink() -> void:
	var plan := _plan()
	for which: String in CityPlan.COURSES:
		var legs := plan.course(which).legs
		for index: int in legs.size():
			var road := legs[index].segment.road
			var next := legs[(index + 1) % legs.size()].segment.road
			var label := "%s %s" % [which, legs[index].segment.name]
			var gap := road.at(road.length).distance_to(next.at(0.0))
			assert_float(gap).override_failure_message(label).is_less(0.1)
			var line := road.tangent(road.length - 1.0).dot(next.tangent(1.0))
			assert_float(line).override_failure_message(label).is_greater(0.99)


func test_courses_nest_and_are_about_the_lengths_they_are_meant_to_be() -> void:
	var plan := _plan()
	assert_float(plan.course("inner").length).is_between(2400.0, 2800.0)
	assert_float(plan.course("medium").length).is_between(6000.0, 6800.0)
	assert_float(plan.course("full").length).is_between(13500.0, 15500.0)


func test_a_shared_road_is_the_same_street_on_every_course() -> void:
	var plan := _plan()
	var inner := plan.course("inner").legs[0].segment
	var full := plan.course("full").legs[0].segment
	assert_object(inner).is_same(full)
	var again := CityPlan.new(4).segments["N1"]
	assert_vector(again.road.at(300.0)).is_equal(inner.road.at(300.0))
	assert_int(again.city(4.0).street.towers.size()).is_equal(inner.city(4.0).street.towers.size())


func test_no_bend_is_too_tight_for_what_stands_along_it() -> void:
	var plan := _plan()
	for segment: CitySegment in plan.segments.values():
		var s := 0.0
		while s < segment.road.length - 10.0:
			var bend := segment.road.tangent(s).angle_to(segment.road.tangent(s + 10.0))
			var radius := 10.0 / maxf(bend, 0.0001)
			assert_float(radius).override_failure_message(segment.name).is_greater(100.0)
			s += 10.0


func test_roads_are_market_where_they_meet() -> void:
	var plan := _plan()
	for segment: CitySegment in plan.segments.values():
		var districts := segment.districts
		var far_end := segment.road.length - 30.0
		assert_int(districts.kind_at(10.0)).override_failure_message(segment.name).is_equal(
			Districts.Kind.MARKET
		)
		assert_int(districts.kind_at(far_end)).override_failure_message(segment.name).is_equal(
			Districts.Kind.MARKET
		)


func test_loops_reach_new_areas_as_they_grow() -> void:
	var plan := _plan()
	var seen := func(which: String) -> Array:
		var kinds := []
		for leg: Course.Leg in plan.course(which).legs:
			for stretch: Districts.Stretch in leg.segment.districts.stretches:
				if stretch.kind not in kinds:
					kinds.append(stretch.kind)
		return kinds
	assert_bool(Districts.Kind.RETAIL in seen.call("inner")).is_false()
	assert_bool(Districts.Kind.RETAIL in seen.call("medium")).is_true()
	assert_bool(Districts.Kind.RESIDENTIAL in seen.call("medium")).is_true()


func test_no_spire_stands_on_any_road() -> void:
	var plan := _plan()
	var roads: Array[Route] = []
	for segment: CitySegment in plan.segments.values():
		roads.append(segment.road)
	var segment: CitySegment = plan.segments["U1"]
	for spire: Skyline.Spire in segment.city(4.0).skyline.spires:
		var at := segment.road.at(spire.s) + segment.road.right(spire.s) * spire.offset
		for road: Route in roads:
			var s := 0.0
			while s < road.length:
				var away := Vector2(at.x, at.z).distance_to(Vector2(road.at(s).x, road.at(s).z))
				assert_float(away).is_greater(Skyline.SPIRE_NEAREST - spire.width * 0.5 - 8.0)
				s += 40.0


func test_course_wraps_at_the_finish() -> void:
	var course := _plan().course("inner")
	assert_vector(course.at(course.length + 50.0)).is_equal(course.at(50.0))
	assert_vector(course.at(-50.0)).is_equal(course.at(course.length - 50.0))


func _forks(plan: CityPlan) -> Array[Fork]:
	var forks: Array[Fork] = []
	for segment: CitySegment in plan.segments.values():
		forks.append_array(segment.forks)
	return forks


func test_each_junction_is_known_to_both_roads_and_drawn_from_one() -> void:
	var plan := _plan()
	var forks := _forks(plan)
	assert_int(forks.size()).is_equal(8)
	var drawn := forks.filter(func(fork: Fork) -> bool: return fork.draws)
	assert_int(drawn.size()).is_equal(4)


func test_a_ridden_road_is_never_also_a_road_not_taken_at_its_own_junction() -> void:
	var plan := _plan()
	for which: String in CityPlan.COURSES:
		for leg: Course.Leg in plan.course(which).legs:
			for fork: Fork in leg.segment.forks:
				var names: Array = CityPlan.COURSES[which]
				assert_bool(fork.other_name in names).override_failure_message(which).is_false()


func test_roads_part_level_with_room_for_a_building_between_them() -> void:
	var plan := _plan()
	for fork: Fork in _forks(plan):
		var label := "%s to %s" % [fork.along(0.0), fork.other_name]
		assert_float(fork.wall_here(0.0, 0.0).distance_to(fork.wall_there(0.0, 0.0))).is_greater(
			Fork.WALL
		)
		assert_float(fork.prow).override_failure_message(label).is_between(60.0, 120.0)
		assert_float(fork.back).override_failure_message(label).is_between(fork.prow, 150.0)
		for away: float in [0.0, 60.0, fork.back]:
			var here := fork.wall_here(away, 0.0).y
			var there := fork.wall_there(away, 0.0).y
			assert_float(absf(here - there)).override_failure_message(label).is_less(0.01)


func test_nothing_stands_in_a_fork_and_towers_past_it_stop_short_of_the_other_road() -> void:
	var plan := _plan()
	for segment: CitySegment in plan.segments.values():
		var street := segment.city(4.0).street
		for tower: Street.Tower in street.towers:
			assert_bool(street.forked(tower.side, tower.s)).is_false()
			var room := street.reach(tower.side, tower.s) - Fork.WALL
			assert_float(tower.depth).override_failure_message(segment.name).is_less_equal(
				room + 2.0
			)


func test_roads_are_market_as_far_as_they_are_forked() -> void:
	var plan := _plan()
	for segment: CitySegment in plan.segments.values():
		for fork: Fork in segment.forks:
			var away := 0.0
			while away < fork.back:
				var kind := segment.districts.kind_at(fork.along(away))
				assert_int(kind).override_failure_message(segment.name).is_equal(
					Districts.Kind.MARKET
				)
				away += 5.0


func test_a_longer_course_goes_through_at_least_as_many_places() -> void:
	var plan := CityPlan.new(1)
	var inner := plan.course("inner").places()
	var full := plan.course("full").places()
	assert_bool(inner.is_empty()).is_false()
	assert_int(full.size()).is_greater_equal(inner.size())
	assert_bool(Districts.Kind.MARKET in full).is_false()
