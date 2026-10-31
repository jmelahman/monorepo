extends GdUnitTestSuite
## The properties the sim leans on: same inputs, same draw; different
## coordinates, independent draws; every face reachable and roughly even.


func test_derive_is_a_pure_function_of_its_inputs() -> void:
	assert_int(Rng.derive(42, [3, 1])).is_equal(Rng.derive(42, [3, 1]))
	assert_int(Rng.derive(42, [3, 1])).is_not_equal(Rng.derive(42, [1, 3]))
	assert_int(Rng.derive(42, [3, 1])).is_not_equal(Rng.derive(43, [3, 1]))


## Pinned, so a change to the mixer that would silently re-deal every saved
## run and every recorded replay fails here first.
func test_derive_is_stable_across_versions() -> void:
	# Checked against an independent Python murmur3 fmix32.
	assert_int(Rng.derive(1, [2, 3])).is_equal(3981506232)
	assert_int(Rng.derive(12345, [0])).is_equal(1011272156)


func test_derive_stays_in_32_bits() -> void:
	for i: int in 1000:
		var value := Rng.derive(-i, [i, i * 7919, -1])
		assert_int(value).is_between(0, Rng.MASK)


func test_range_at_covers_the_range_evenly() -> void:
	var counts: Dictionary[int, int] = {}
	for i: int in 6000:
		var face := Rng.range_at(7, [i], 1, 6)
		counts[face] = counts.get(face, 0) + 1
	assert_array(counts.keys()).contains_exactly_in_any_order([1, 2, 3, 4, 5, 6])
	for face: int in counts:
		# 1000 expected; six sigma of a binomial(6000, 1/6) is about 170.
		assert_int(counts[face]).is_between(830, 1170)
