extends GdUnitTestSuite

const KERB := 4.0
const LENGTH := 1000.0


func _street() -> Street:
	return Street.generate(4, LENGTH, KERB)


func _open_share(street: Street, kind: Districts.Kind) -> float:
	var open := 0
	var total := 0
	for shop: Street.Shop in street.shops:
		if shop.s > 0.0 and street.districts.kind_at(shop.s) == kind:
			total += 1
			open += 1 if shop.open else 0
	return float(open) / maxf(total, 1.0)


func test_same_seed_gives_the_same_districts() -> void:
	var a := Districts.generate(4, LENGTH)
	var b := Districts.generate(4, LENGTH)
	assert_int(a.stretches.size()).is_equal(b.stretches.size())
	assert_float(a.stretches[1].to).is_equal(b.stretches[1].to)


func test_the_ride_passes_through_every_kind() -> void:
	var districts := Districts.generate(4, LENGTH)
	for kind: Districts.Kind in Districts.Kind.values():
		assert_int(districts.of_kind(kind).size()).is_greater(0)


func test_stretches_join_up() -> void:
	var stretches := Districts.generate(4, LENGTH).stretches
	for index: int in range(1, stretches.size()):
		assert_float(stretches[index].from).is_equal(stretches[index - 1].to)


func test_a_short_ride_stays_in_the_market() -> void:
	var districts := Districts.generate(4, 200.0)
	assert_int(districts.stretches.size()).is_equal(1)
	assert_int(districts.kind_at(150.0)).is_equal(Districts.Kind.MARKET)


func test_the_market_keeps_more_shops_open_than_the_works() -> void:
	var street := _street()
	var market := _open_share(street, Districts.Kind.MARKET)
	assert_float(market).is_greater(_open_share(street, Districts.Kind.WORKS))


func test_nothing_stands_in_the_plaza() -> void:
	var street := _street()
	for stretch: Districts.Stretch in street.districts.of_kind(Districts.Kind.PLAZA):
		for tower: Street.Tower in street.towers:
			assert_bool(tower.s > stretch.from and tower.s < stretch.to).is_false()


func test_the_plaza_opens_on_both_sides() -> void:
	var street := _street()
	assert_int(street.openings.size()).is_equal(2)
	assert_float(street.openings[0].z).is_not_equal(street.openings[1].z)


func test_no_lamp_stands_in_the_underpass() -> void:
	var street := _street()
	for s: float in street.lamps:
		assert_int(street.districts.kind_at(s)).is_not_equal(Districts.Kind.UNDERPASS)


func test_the_deck_opens_over_the_plaza_alone() -> void:
	var street := _street()
	var city := Undercity.generate(4, street, KERB, LENGTH)
	var plaza := street.districts.of_kind(Districts.Kind.PLAZA)[0]
	var middle := (plaza.from + plaza.to) * 0.5
	assert_float(city.slot_half(middle)).is_equal(Undercity.OCULUS_HALF)
	assert_float(city.slot_half(50.0)).is_equal(Undercity.SLOT_HALF)


func test_rain_stops_under_the_roof() -> void:
	var districts := Districts.generate(4, LENGTH)
	var under := districts.of_kind(Districts.Kind.UNDERPASS)[0]
	assert_float(districts.open_to_the_sky((under.from + under.to) * 0.5)).is_equal(0.0)
	assert_float(districts.open_to_the_sky(20.0)).is_equal(1.0)
