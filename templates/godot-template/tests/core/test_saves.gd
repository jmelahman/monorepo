extends GdUnitTestSuite
## Saves write to user://, which scripts/env.sh points into .godot-home/ for
## every scripted run, so these never touch a real save.

const SLOT := "test_saves"


func before_test() -> void:
	Saves.erase(SLOT)


func after_test() -> void:
	Saves.erase(SLOT)


func test_round_trip() -> void:
	assert_bool(Saves.has(SLOT)).is_false()
	assert_dict(Saves.read(SLOT)).is_empty()
	assert_int(Saves.write(SLOT, {"score": 3})).is_equal(OK)
	assert_bool(Saves.has(SLOT)).is_true()
	# JSON narrows nothing: the caller gets 3.0 back and from_dict narrows it.
	assert_float(Saves.read(SLOT)["score"]).is_equal(3.0)


func test_overwrite_keeps_the_previous_save_as_backup() -> void:
	Saves.write(SLOT, {"n": 1})
	Saves.write(SLOT, {"n": 2})
	assert_float(Saves.read(SLOT)["n"]).is_equal(2.0)
	var path := Saves.DIR.path_join(SLOT + ".json")
	var backup: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(path + ".bak"))
	assert_dict(backup).contains_key_value("data", {"n": 1.0})


func test_a_corrupt_save_falls_back_to_the_backup() -> void:
	Saves.write(SLOT, {"n": 1})
	Saves.write(SLOT, {"n": 2})
	_overwrite(Saves.DIR.path_join(SLOT + ".json"), "{ truncated")
	assert_float(Saves.read(SLOT)["n"]).is_equal(1.0)


func test_a_save_from_a_newer_build_is_refused() -> void:
	Saves.write(SLOT, {"n": 1})
	_overwrite(
		Saves.DIR.path_join(SLOT + ".json"),
		JSON.stringify({"version": Saves.VERSION + 1, "data": {"n": 9}})
	)
	DirAccess.remove_absolute(Saves.DIR.path_join(SLOT + ".json.bak"))
	assert_dict(Saves.read(SLOT)).is_empty()


func _overwrite(path: String, text: String) -> void:
	var file := FileAccess.open(path, FileAccess.WRITE)
	file.store_string(text)
	file.close()
