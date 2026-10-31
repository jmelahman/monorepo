extends GdUnitTestSuite

const KERB := 4.0
const LENGTH := 200.0


func _make() -> Array:
	var route := Route.generate(4)
	var street := Street.generate(4, LENGTH, KERB)
	return [street, route, Skyline.generate(4, street, route, LENGTH)]


func test_same_seed_gives_the_same_skyline() -> void:
	var a: Skyline = _make()[2]
	var b: Skyline = _make()[2]
	assert_int(a.spires.size()).is_equal(b.spires.size())
	assert_int(a.screens.size()).is_equal(b.screens.size())
	assert_int(a.spans.size()).is_equal(b.spans.size())
	assert_float(a.spires[5].height).is_equal(b.spires[5].height)
	assert_float(a.spans[1].height).is_equal(b.spans[1].height)


func test_far_towers_keep_off_the_road() -> void:
	var made := _make()
	var route: Route = made[1]
	var skyline: Skyline = made[2]
	for spire: Skyline.Spire in skyline.spires:
		var at := route.at(spire.s) + route.right(spire.s) * spire.offset
		assert_float(absf(route.nearest(at.x, at.z).y)).is_greater(spire.width * 0.5)


func test_the_ride_passes_screens_and_a_railway() -> void:
	var skyline: Skyline = _make()[2]
	assert_int(skyline.screens.size()).is_greater(3)
	var rails := 0
	for span: Skyline.Span in skyline.spans:
		if span.rail and span.s > 0.0 and span.s < LENGTH:
			rails += 1
	assert_int(rails).is_greater(0)


func test_no_sign_hangs_through_a_bridge() -> void:
	var made := _make()
	var street: Street = made[0]
	var skyline: Skyline = made[2]
	for span: Skyline.Span in skyline.spans:
		var reach := (Skyline.RAIL_DEPTH if span.rail else Skyline.BRIDGE_DEPTH) * 0.5
		for hung: Street.Sign in street.signs:
			var through := (
				absf(hung.s - span.s) < reach
				and hung.elevation < span.height
				and hung.elevation + hung.size.y > span.height
			)
			assert_bool(through).is_false()
