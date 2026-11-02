extends GdUnitTestSuite

const SAVED := "user://test_video.cfg"


func after_test() -> void:
	DirAccess.remove_absolute(SAVED)


func test_a_fresh_set_is_high_so_stills_stay_the_same() -> void:
	assert_str(VideoOptions.new().preset()).is_equal("High")


func test_nothing_saved_gets_the_default_preset() -> void:
	var options := VideoOptions.saved("user://nothing_here.cfg")
	assert_str(options.preset()).is_equal(VideoOptions.DEFAULT_PRESET)


func test_what_is_saved_comes_back() -> void:
	var options := VideoOptions.new()
	options.use_preset("Low")
	options.set_value("show_fps", 1)
	options.save(SAVED)
	var back := VideoOptions.saved(SAVED)
	assert_str(back.preset()).is_equal("Low")
	assert_bool(back.is_on("show_fps")).is_true()


func test_a_hand_edited_file_cannot_pick_a_choice_that_does_not_exist() -> void:
	var file := ConfigFile.new()
	file.set_value(VideoOptions.SECTION, "shadows", 99)
	file.set_value(VideoOptions.SECTION, "fog", "lots")
	file.save(SAVED)
	var options := VideoOptions.saved(SAVED)
	assert_int(options.value("shadows")).is_equal(3)
	var fallback: int = VideoOptions.PRESETS[VideoOptions.DEFAULT_PRESET]["fog"]
	assert_int(options.value("fog")).is_equal(fallback)


func test_changing_one_option_makes_a_preset_custom() -> void:
	var options := VideoOptions.new()
	options.set_value("rain", 0)
	assert_str(options.preset()).is_equal(VideoOptions.CUSTOM)


func test_switching_off_by_the_old_names() -> void:
	var options := VideoOptions.new()
	options.switch_off(PackedStringArray(["shadows", "probes"]))
	assert_bool(options.is_on("shadows")).is_false()
	assert_bool(options.is_on("reflections")).is_false()
	assert_bool(options.is_on("fog")).is_true()


func test_every_table_has_a_value_for_every_choice() -> void:
	var scales: Array = VideoOptions.OPTIONS["render_scale"]["choices"]
	var limits: Array = VideoOptions.OPTIONS["frame_limit"]["choices"]
	var shadows: Array = VideoOptions.OPTIONS["shadows"]["choices"]
	var textures: Array = VideoOptions.OPTIONS["textures"]["choices"]
	assert_int(VideoOptions.SCALES.size()).is_equal(scales.size())
	assert_int(VideoOptions.FRAME_LIMITS.size()).is_equal(limits.size())
	assert_int(VideoOptions.LAMP_SHADOWS.size()).is_equal(shadows.size())
	assert_int(VideoOptions.MIP_BIAS.size()).is_equal(textures.size())
