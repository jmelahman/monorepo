extends GdUnitTestSuite
## 5-wild's golden vectors (../test/golden/vectors.json), replayed in QuickJS.
##
## Two claims, tested separately because they fail for different reasons.
## Parity: QuickJS runs src/engine to the same scores, gold and holdings Node
## recorded, which is the portability contract test/golden.test.ts states. The
## seam: driving the same run one action at a time through [Wild], as the game
## does, ends at exactly the state the engine reaches on its own, so nothing is
## lost on the way through GDScript. Saving mid-run and resuming in a new
## engine is part of the second, since that is the other way a run crosses.

const GOLDEN := "res://js/build/golden.js"

var _vectors: Array[Dictionary] = []
var _answers := ""
var _allowed := ""
var _golden: JsContext


func before() -> void:
	var web := ProjectSettings.globalize_path("res://").path_join("..")
	var parsed: Array = JSON.parse_string(
		FileAccess.get_file_as_string(web.path_join("test/golden/vectors.json"))
	)
	_vectors.assign(parsed)
	_answers = FileAccess.get_file_as_string("res://words/en/answers.txt")
	_allowed = FileAccess.get_file_as_string("res://words/en/allowed.txt")
	_golden = JsContext.new()
	assert_bool(_golden.load(FileAccess.get_file_as_string(GOLDEN), "golden.js")).is_true()


func test_vectors_are_current() -> void:
	assert_array(_vectors).is_not_empty()
	var wild := Wild.open()
	for vector: Dictionary in _vectors:
		(
			assert_int(type_convert(vector["contentVersion"], TYPE_INT))
			. override_failure_message("%s: re-record with `npm run golden`" % vector["name"])
			. is_equal(wild.content_version())
		)


func test_parity() -> void:
	for vector: Dictionary in _vectors:
		var out := _golden.invoke(
			"fivewildGolden.replay", [JSON.stringify(vector), _answers, _allowed]
		)
		assert_str(_golden.get_error()).is_empty()
		var replayed: Dictionary = JSON.parse_string(out)
		(
			assert_array(replayed["refused"])
			. override_failure_message("%s: an action was refused" % vector["name"])
			. is_empty()
		)
		(
			assert_dict(replayed["expected"])
			. override_failure_message("%s: replayed differently" % vector["name"])
			. is_equal(vector["expected"])
		)


func test_seam() -> void:
	for vector: Dictionary in _vectors:
		var want := _golden.invoke(
			"fivewildGolden.finalState", [JSON.stringify(vector), _answers, _allowed]
		)
		var actions: Array = vector["actions"]
		var half := floori(actions.size() / 2.0)

		var run_seed: int = type_convert(vector["seed"], TYPE_INT)
		var ascension: int = type_convert(vector.get("ascension", 0), TYPE_INT)

		var wild := _english()
		wild.start(run_seed, ascension)
		for i in half:
			var action: Dictionary = actions[i]
			wild.dispatch(action)
		# The save, and a different engine picking it up.
		var resumed := _english()
		assert_bool(resumed.resume(wild.save_text())).is_true()
		for i in range(half, actions.size()):
			var action: Dictionary = actions[i]
			resumed.dispatch(action)

		(
			assert_str(resumed.save_text())
			. override_failure_message(
				"%s: the run ended differently through Wild" % vector["name"]
			)
			. is_equal(want)
		)


func test_refusal_is_an_event() -> void:
	var wild := _english()
	wild.start(1)
	var step := wild.dispatch({"type": "submit"})
	var events: Array = step["events"]
	assert_str(events[0]["type"]).is_equal("rejected")


func _english() -> Wild:
	var wild := Wild.open()
	assert_bool(wild.set_language("en")).is_true()
	return wild
