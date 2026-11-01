extends GdUnitTestSuite


func test_every_anchor_lands_in_exactly_one_section() -> void:
	var sections: Array[StreetSection] = [
		StreetSection.new(0.0, 96.0), StreetSection.new(96.0, 192.0)
	]
	for s: float in [0.0, 95.9, 96.0, 191.9]:
		var holders := sections.filter(
			func(section: StreetSection) -> bool: return section.holds(s)
		)
		assert_int(holders.size()).is_equal(1)


func test_a_long_piece_is_kept_until_its_far_end() -> void:
	var section := StreetSection.new(0.0, 96.0)
	assert_bool(section.takes(50.0, 300.0)).is_true()
	assert_float(section.until).is_equal(300.0)


func test_only_the_first_section_takes_what_started_before_it() -> void:
	var opening := StreetSection.new(96.0, 192.0, true)
	var later := StreetSection.new(96.0, 192.0)
	assert_bool(opening.takes(10.0, 150.0)).is_true()
	assert_bool(later.takes(10.0, 150.0)).is_false()
	assert_bool(opening.takes(10.0, 50.0)).is_false()
