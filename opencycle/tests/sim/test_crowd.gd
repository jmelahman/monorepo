extends GdUnitTestSuite

const KERB := 4.0
const LOOKS := 14


func _crowd(street: Street) -> Crowd:
	return Crowd.generate(4, street, -170.0, 370.0, KERB, LOOKS)


func _street() -> Street:
	return Street.generate(4, 200.0, KERB)


func test_same_seed_gives_the_same_people() -> void:
	var a := _crowd(_street())
	var b := _crowd(_street())
	assert_int(a.people.size()).is_equal(b.people.size())
	for i: int in a.people.size():
		assert_float(a.people[i].s).is_equal(b.people[i].s)
		assert_int(a.people[i].look).is_equal(b.people[i].look)


func test_a_street_is_busy_with_every_look_and_some_standing() -> void:
	var crowd := _crowd(_street())
	var looks: Dictionary[int, bool] = {}
	var standing := 0
	for person: Crowd.Person in crowd.people:
		looks[person.look] = true
		if person.speed == 0.0:
			standing += 1
	assert_int(crowd.people.size()).is_greater(400)
	assert_int(looks.size()).is_equal(LOOKS)
	assert_int(standing).is_greater(5)


func test_everyone_stays_on_the_pavement() -> void:
	for person: Crowd.Person in _crowd(_street()).people:
		assert_float(absf(person.offset)).is_greater(KERB + 0.5)
		assert_float(absf(person.offset)).is_less(KERB + Street.SIDEWALK - 0.5)


func test_walkers_stay_inside_the_street_as_time_passes() -> void:
	var crowd := _crowd(_street())
	for time: float in [0.0, 37.5, 1000.0]:
		for person: Crowd.Person in crowd.people:
			var s := crowd.s_at(person, time)
			assert_bool(s >= crowd.start and s < crowd.end).is_true()


func test_nobody_in_a_lane_catches_anybody_else() -> void:
	var crowd := _crowd(_street())
	var seen: Dictionary[Vector2, float] = {}
	for person: Crowd.Person in crowd.people:
		if person.speed > 0.0:
			var lane := Vector2(signf(person.offset), person.speed)
			seen[lane] = person.direction * person.speed
	for person: Crowd.Person in crowd.people:
		if person.speed > 0.0:
			var lane := Vector2(signf(person.offset), person.speed)
			assert_float(person.direction * person.speed).is_equal(seen[lane])


func test_people_standing_are_clear_of_what_is_on_the_pavement() -> void:
	var street := _street()
	for person: Crowd.Person in _crowd(street).people:
		if person.speed == 0.0:
			for prop: Street.Prop in street.props:
				var at := Vector2(person.s, person.offset)
				assert_float(at.distance_to(Vector2(prop.s, prop.offset))).is_greater_equal(
					Crowd.PROP_CLEARANCE
				)
