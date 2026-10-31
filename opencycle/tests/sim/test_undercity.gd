extends GdUnitTestSuite

const KERB := 5.0
const LENGTH := 200.0


func _city() -> Undercity:
	return Undercity.generate(4, Street.generate(4, LENGTH, KERB), KERB, LENGTH)


func test_same_seed_gives_the_same_city_overhead() -> void:
	var a := _city()
	var b := _city()
	assert_int(a.ribs.size()).is_equal(b.ribs.size())
	assert_float(a.pylons[3].s).is_equal(b.pylons[3].s)
	assert_float(a.lanes[2].height).is_equal(b.lanes[2].height)


func test_pylons_stand_all_along_both_sides() -> void:
	for side: float in [-1.0, 1.0]:
		var passed := 0
		for pylon: Undercity.Pylon in _city().pylons:
			if pylon.side == side and pylon.s > 0.0 and pylon.s < LENGTH:
				passed += 1
		assert_int(passed).is_greater(1)


func test_no_sign_hangs_through_a_pylon() -> void:
	var street := Street.generate(4, LENGTH, KERB)
	var city := Undercity.generate(4, street, KERB, LENGTH)
	for pylon: Undercity.Pylon in city.pylons:
		for hung: Street.Sign in street.signs:
			if hung.side == pylon.side:
				assert_float(absf(hung.s - pylon.s)).is_greater_equal(pylon.width * 0.5 + 1.0)


func test_traffic_flies_under_the_deck() -> void:
	for lane: Undercity.Lane in _city().lanes:
		assert_float(lane.height).is_less(Undercity.DECK_HEIGHT)
