extends GdUnitTestSuite
## The scene, driven through real key events. These need a renderer and an
## input pipeline, which is why scripts/test.sh runs under Xvfb rather than
## --headless.
##
## What they check is the seam, not the game: that a key reaches the shell,
## that what the shell says comes back as something on screen, and that the
## store reaches the disk. The rules are the golden vectors' business, and the
## views are the web build's.

const MAIN := "res://game/main.tscn"
const STORE := "store"
## The web build's run save; see Storage in 5-wild's CLAUDE.md.
const RUN_KEY := "5wild:run:v2"
const SEED := 7


func before_test() -> void:
	Saves.erase(STORE)


func after_test() -> void:
	Saves.erase(STORE)


func test_a_fresh_launch_shows_the_title() -> void:
	var runner := scene_runner(MAIN)
	assert_bool(runner.invoke("showing", "title")).is_true()


func test_typing_the_answer_solves_the_round_and_saves() -> void:
	var runner := scene_runner(MAIN)
	runner.invoke("start_seeded", SEED)
	assert_bool(runner.invoke("showing", "round-screen")).is_true()
	var run: Dictionary = runner.invoke("state")
	var answer: String = run["round"]["answer"]
	for ch in answer.to_upper():
		runner.simulate_key_pressed(OS.find_keycode_from_string(ch))
		await await_idle_frame()
	run = runner.invoke("state")
	assert_str(run["round"]["draft"]).is_equal(answer)
	runner.simulate_action_pressed("submit")
	await await_idle_frame()
	run = runner.invoke("state")
	assert_bool(run["round"]["solved"]).is_true()
	var items: Dictionary = Saves.read(STORE).get("items", {})
	assert_bool(items.has(RUN_KEY)).is_true()


func test_a_refusal_is_shown_not_thrown() -> void:
	var runner := scene_runner(MAIN)
	runner.invoke("start_seeded", SEED)
	runner.simulate_action_pressed("submit")
	await await_idle_frame()
	# The sentence is the web catalog's, so all this can know is that there is one.
	assert_str((runner.find_child("ToastText") as Label).text).is_not_empty()


func test_a_saved_run_is_resumed() -> void:
	var first := scene_runner(MAIN)
	first.invoke("start_seeded", SEED)
	first.simulate_key_pressed(KEY_Q)
	await await_idle_frame()
	var second := scene_runner(MAIN)
	var run: Dictionary = second.invoke("state")
	assert_float(run["seed"]).is_equal(float(SEED))
	# The engine keeps the draft lowercase, as it does the answer.
	assert_str(run["round"]["draft"]).is_equal("q")


func test_pause_opens_a_sheet_and_escape_closes_it() -> void:
	var runner := scene_runner(MAIN)
	runner.invoke("start_seeded", SEED)
	runner.simulate_action_pressed("pause")
	await await_idle_frame()
	var shown: Dictionary = runner.get_property("shown")
	assert_bool(shown.get("sheet") is Dictionary).is_true()
	runner.simulate_action_pressed("pause")
	await await_idle_frame()
	shown = runner.get_property("shown")
	assert_bool(shown.get("sheet") is Dictionary).is_false()


func test_tab_stays_inside_an_open_sheet() -> void:
	var runner := scene_runner(MAIN)
	runner.invoke("start_seeded", SEED)
	runner.simulate_action_pressed("pause")
	await await_idle_frame()
	var sheet := _find(runner.scene(), "sheet")
	assert_object(sheet).is_not_null()
	# Enough presses to go round more than once, both ways.
	for i in 40:
		runner.simulate_key_pressed(KEY_TAB)
		await await_idle_frame()
		assert_bool(_inside(sheet, runner)).is_true()
	runner.simulate_key_press(KEY_SHIFT)
	for i in 40:
		runner.simulate_key_pressed(KEY_TAB)
		await await_idle_frame()
		assert_bool(_inside(sheet, runner)).is_true()
	runner.simulate_key_release(KEY_SHIFT)


func test_enter_presses_a_setting_and_focus_stays_on_it() -> void:
	var runner := scene_runner(MAIN)
	runner.invoke("start_seeded", SEED)
	runner.simulate_action_pressed("pause")
	await await_idle_frame()
	for i in 40:
		if _name(_focused(runner)) == "speed":
			break
		runner.simulate_key_pressed(KEY_TAB)
		await await_idle_frame()
	assert_str(_name(_focused(runner))).is_equal("speed")
	var shown: Dictionary = runner.get_property("shown")
	var was: Variant = shown.get("speed")
	runner.simulate_key_pressed(KEY_ENTER)
	await await_idle_frame()
	shown = runner.get_property("shown")
	assert_that(shown.get("speed")).is_not_equal(was)
	# The press rebuilt the sheet, and the name carried focus across it.
	assert_str(_name(_focused(runner))).is_equal("speed")


func test_enter_over_a_letter_key_still_submits() -> void:
	var runner := scene_runner(MAIN)
	runner.invoke("start_seeded", SEED)
	var run: Dictionary = runner.invoke("state")
	for ch in str(run["round"]["answer"]).to_upper():
		runner.simulate_key_pressed(OS.find_keycode_from_string(ch))
		await await_idle_frame()
	var keyboard := _find(runner.scene(), "keyboard")
	for i in 80:
		if _inside(keyboard, runner):
			break
		runner.simulate_key_pressed(KEY_TAB)
		await await_idle_frame()
	assert_bool(_inside(keyboard, runner)).is_true()
	runner.simulate_key_pressed(KEY_ENTER)
	await await_idle_frame()
	run = runner.invoke("state")
	assert_bool(run["round"]["solved"]).is_true()


static func _focused(runner: GdUnitSceneRunner) -> Control:
	return runner.scene().get_viewport().gui_get_focus_owner()


static func _inside(box: Box, runner: GdUnitSceneRunner) -> bool:
	var focused := _focused(runner)
	return focused != null and box.is_ancestor_of(focused)


static func _name(control: Control) -> String:
	var box := control as Box
	if box == null:
		return ""
	var attrs: Dictionary = box.node.get("a", {})
	return str(attrs.get("data-focus", ""))


static func _find(at: Node, cls: String) -> Box:
	var box := at as Box
	if box != null and box.has_class(cls):
		return box
	for kid: Node in at.get_children():
		var found := _find(kid, cls)
		if found != null:
			return found
	return null
