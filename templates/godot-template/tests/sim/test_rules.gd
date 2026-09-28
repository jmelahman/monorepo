extends GdUnitTestSuite
## The rules, driven with no scene: everything here runs in microseconds, which
## is the point of keeping the sim pure.


## A seed and turn whose first roll is [param face]; found by search rather
## than hard-coded, so the tests say what they need instead of which seed
## happened to give it.
func _seed_rolling(face: int) -> int:
	for s: int in 1000:
		if Rng.range_at(s, [0, 0], 1, 6) == face:
			return s
	fail("no seed under 1000 rolls a %d first" % face)
	return -1


func test_roll_adds_to_the_pot() -> void:
	var step := Rules.reduce(RunState.start(_seed_rolling(5)), {"type": "roll"})
	assert_int(step.state.pot).is_equal(5)
	assert_int(step.state.rolls).is_equal(1)
	assert_array(step.events).is_equal([{"type": &"rolled", "value": 5}])


func test_rolling_a_one_loses_the_pot_and_ends_the_turn() -> void:
	var state := RunState.start(_seed_rolling(Rules.BUST))
	state.pot = 12
	var step := Rules.reduce(state, {"type": "roll"})
	assert_int(step.state.pot).is_equal(0)
	assert_int(step.state.turn).is_equal(1)
	assert_int(step.state.score).is_equal(0)
	assert_str(step.events[1]["type"]).is_equal("busted")
	assert_int(step.events[1]["lost"]).is_equal(12)


func test_bank_moves_the_pot_to_the_score() -> void:
	var state := RunState.start(1)
	state.pot = 9
	var step := Rules.reduce(state, {"type": "bank"})
	assert_int(step.state.score).is_equal(9)
	assert_int(step.state.pot).is_equal(0)
	assert_int(step.state.turn).is_equal(1)


func test_banking_to_the_target_wins() -> void:
	var state := RunState.start(1)
	state.score = Rules.TARGET - 4
	state.pot = 4
	var step := Rules.reduce(state, {"type": "bank"})
	assert_bool(step.state.won).is_true()
	assert_str(step.events.back()["type"]).is_equal("won")


func test_refusals_leave_the_state_alone() -> void:
	var state := RunState.start(1)
	var step := Rules.reduce(state, {"type": "bank"})
	assert_object(step.state).is_same(state)
	assert_array(step.events).is_equal([{"type": &"refused", "code": &"empty_pot"}])
	state.won = true
	assert_str(Rules.reduce(state, {"type": "roll"}).events[0]["code"]).is_equal("game_over")
	assert_str(Rules.reduce(RunState.start(1), {"type": "fly"}).events[0]["code"]).is_equal(
		"unknown_action"
	)


func test_reduce_never_mutates_its_input() -> void:
	var state := RunState.start(3)
	var snapshot := state.to_dict()
	for i: int in 20:
		Rules.reduce(state, {"type": "roll" if i % 3 else "bank"})
	assert_dict(state.to_dict()).is_equal(snapshot)


## What a save and a replay both rest on: the state survives JSON, including
## JSON's habit of turning every int into a float.
func test_state_round_trips_through_json() -> void:
	var state := Rules.replay(99, _actions(40))
	var json := JSON.stringify(state.to_dict())
	var parsed: Dictionary = JSON.parse_string(json)
	var back := RunState.from_dict(parsed)
	assert_dict(back.to_dict()).is_equal(state.to_dict())


func test_replay_is_deterministic() -> void:
	assert_dict(Rules.replay(7, _actions(200)).to_dict()).is_equal(
		Rules.replay(7, _actions(200)).to_dict()
	)


## The shape of a balance check: many seeds, a fixed strategy, a distribution.
## Bank at 15 or more should win well inside 50 turns on nearly every seed; if
## a rules change breaks that, this says so before a player does.
func test_a_sensible_strategy_wins_in_reasonable_time() -> void:
	var turns: Array[int] = []
	for s: int in 300:
		var state := RunState.start(s)
		while not state.won and state.turn < 200:
			var action := "bank" if state.pot >= 15 else "roll"
			state = Rules.reduce(state, {"type": action}).state
		assert_bool(state.won).is_true()
		turns.append(state.turn)
	turns.sort()
	var median := turns[floori(turns.size() / 2.0)]
	assert_int(median).is_between(5, 20)


func _actions(count: int) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for i: int in count:
		out.append({"type": "bank" if i % 4 == 3 else "roll"})
	return out
