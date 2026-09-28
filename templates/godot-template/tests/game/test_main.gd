extends GdUnitTestSuite
## The scene, driven through real input actions. These need a renderer and an
## input pipeline, which is why scripts/test.sh runs under Xvfb rather than
## --headless.

const MAIN := "res://game/main.tscn"


func before_test() -> void:
	Saves.erase("run")


func after_test() -> void:
	Saves.erase("run")
	get_tree().paused = false


func test_a_fresh_launch_starts_a_new_game() -> void:
	var runner := scene_runner(MAIN)
	var main := runner.scene()
	var state: RunState = main.get("state")
	assert_int(state.score).is_equal(0)
	assert_str((main.find_child("Score") as Label).text).is_equal("Score: 0 / 50")
	assert_bool((main.find_child("Bank") as Button).disabled).is_true()


func test_the_roll_action_rolls_and_saves() -> void:
	var runner := scene_runner(MAIN)
	runner.simulate_action_pressed("roll")
	await await_idle_frame()
	var state: RunState = runner.get_property("state")
	assert_int(state.rolls + state.turn).is_equal(1)
	assert_dict(RunState.from_dict(Saves.read("run")).to_dict()).is_equal(state.to_dict())


func test_a_saved_run_is_resumed() -> void:
	var saved := RunState.start(5)
	saved.score = 20
	Saves.write("run", saved.to_dict())
	var runner := scene_runner(MAIN)
	var state: RunState = runner.get_property("state")
	assert_int(state.score).is_equal(20)


func test_pause_opens_the_options_and_pauses_the_tree() -> void:
	var runner := scene_runner(MAIN)
	runner.simulate_action_pressed("pause")
	await await_idle_frame()
	assert_bool((runner.find_child("Options") as Control).visible).is_true()
	assert_bool(get_tree().paused).is_true()
	# Roll is ignored behind the options.
	runner.simulate_action_pressed("roll")
	await await_idle_frame()
	var state: RunState = runner.get_property("state")
	assert_int(state.rolls).is_equal(0)
