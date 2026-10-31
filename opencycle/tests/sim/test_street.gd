extends GdUnitTestSuite


func test_same_seed_gives_the_same_street() -> void:
	var a := Street.generate(4, 200.0)
	var b := Street.generate(4, 200.0)
	assert_int(a.towers.size()).is_equal(b.towers.size())
	assert_int(a.signs.size()).is_equal(b.signs.size())
	assert_float(a.towers[3].height).is_equal(b.towers[3].height)


func test_towers_line_both_sides_past_the_ride() -> void:
	var street := Street.generate(4, 200.0)
	for side: float in [-1.0, 1.0]:
		var reach := 0.0
		for tower: Street.Tower in street.towers:
			if tower.side == side:
				reach = maxf(reach, tower.s)
		assert_float(reach).is_greater(250.0)
