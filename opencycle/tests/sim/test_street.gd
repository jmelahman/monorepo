extends GdUnitTestSuite

const KERB := 4.0


func _street() -> Street:
	return Street.generate(4, 200.0, KERB)


func test_same_seed_gives_the_same_street() -> void:
	var a := _street()
	var b := _street()
	assert_int(a.towers.size()).is_equal(b.towers.size())
	assert_int(a.signs.size()).is_equal(b.signs.size())
	assert_int(a.shops.size()).is_equal(b.shops.size())
	assert_int(a.props.size()).is_equal(b.props.size())
	assert_float(a.towers[3].height).is_equal(b.towers[3].height)
	assert_str(a.shops[9].trade).is_equal(b.shops[9].trade)


func test_towers_line_both_sides_past_the_ride() -> void:
	var street := _street()
	for side: float in [-1.0, 1.0]:
		var reach := 0.0
		for tower: Street.Tower in street.towers:
			if tower.side == side:
				reach = maxf(reach, tower.s)
		assert_float(reach).is_greater(250.0)


func test_shops_fill_each_tower_with_no_gaps() -> void:
	var street := _street()
	for tower: Street.Tower in street.towers:
		var filled := 0.0
		for shop: Street.Shop in street.shops:
			if shop.side == tower.side and absf(shop.s - tower.s) < tower.width * 0.5:
				filled += shop.width
		assert_float(filled).is_equal_approx(tower.width, 0.01)


func test_every_sign_and_shop_has_a_trade_on_the_sign_sheet() -> void:
	var street := _street()
	for shop: Street.Shop in street.shops:
		assert_bool(shop.trade in Street.TRADES).is_true()
	for hung: Street.Sign in street.signs:
		assert_bool(hung.trade in Street.TRADES).is_true()


func test_a_street_has_many_trades_and_some_closed_shops() -> void:
	var street := _street()
	var trades: Dictionary[String, bool] = {}
	var closed := 0
	for shop: Street.Shop in street.shops:
		trades[shop.trade] = true
		if not shop.open:
			closed += 1
	assert_int(trades.size()).is_greater(5)
	assert_int(closed).is_greater(0)
	assert_float(float(closed)).is_less(street.shops.size() / 3.0)


func test_hanging_signs_clear_a_tall_vehicle_and_stay_up_their_building() -> void:
	var street := _street()
	for hung: Street.Sign in street.signs:
		if hung.overhang > 0.0:
			assert_float(hung.elevation).is_greater_equal(Street.SIGN_BASE)


func test_signs_in_one_column_do_not_overlap() -> void:
	var street := _street()
	for tower: Street.Tower in street.towers.slice(0, 20):
		var hanging: Array[Street.Sign] = []
		for hung: Street.Sign in street.signs:
			if hung.side == tower.side and hung.overhang > 0.0:
				if absf(hung.s - tower.s) < tower.width * 0.5:
					hanging.append(hung)
		for i: int in hanging.size():
			for j: int in range(i + 1, hanging.size()):
				var a := hanging[i]
				var b := hanging[j]
				if absf(a.s - b.s) > 0.6:
					continue
				var apart := (
					a.elevation + a.size.y <= b.elevation or b.elevation + b.size.y <= a.elevation
				)
				assert_bool(apart).is_true()


func test_everything_on_the_pavement_stands_on_the_pavement() -> void:
	for prop: Street.Prop in _street().props:
		assert_float(absf(prop.offset)).is_greater(KERB)
		assert_float(absf(prop.offset)).is_less(KERB + Street.SIDEWALK)


func test_patches_stay_on_the_road() -> void:
	for patch: Vector4 in _street().patches:
		assert_float(absf(patch.y) + patch.w * 0.5).is_less_equal(KERB + 0.001)
