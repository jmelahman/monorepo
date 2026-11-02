class_name Menu
extends Control
## Where the game starts: pick a loop, or set up the video. Run with arguments, it goes straight to
## the ride, because stills, clips and benches shouldn't have to know there's a menu.

const RIDE := "res://game/ride.tscn"
const REPLAY := "res://replays/neon.json"
const CITY_SEED := 1
const TITLES: Dictionary[String, String] = {
	"inner": "Inner loop",
	"medium": "Medium loop",
	"full": "Full loop",
}
const MAP_SIZE := Vector2(820.0, 180.0)

var _options: VideoOptions
var _courses: Dictionary[String, Course] = {}
var _laps: Dictionary[String, float] = {}
var _list: VBoxContainer
var _details: PanelContainer
var _video: VideoPanel
var _map: CourseMap
var _detail_title: Label
var _detail_stats: Label
var _detail_places: Label


## The name a course goes by on screen.
static func title_of(course: String) -> String:
	return TITLES.get(course, course.capitalize())


func _ready() -> void:
	if not Ride.arguments().is_empty():
		get_tree().change_scene_to_file.call_deferred(RIDE)
		return
	Session.recall()
	_options = VideoOptions.saved()
	_options.apply_to_window()
	Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
	_plan_courses()
	theme = UiStyle.theme()
	add_child(UiStyle.backdrop(0.95))
	_build_list()
	_build_details()
	_video = VideoPanel.new(_options)
	_video.position = Vector2(96.0, 60.0)
	_video.hide()
	_video.changed.connect(_options.apply_to_window)
	_video.closed.connect(_show_list)
	add_child(_video)
	_focus_course(Session.course)


func _plan_courses() -> void:
	var plan := CityPlan.new(CITY_SEED)
	var script: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(REPLAY))
	for key: String in CityPlan.COURSES:
		var course := plan.course(key)
		_courses[key] = course
		_laps[key] = Replay.from_dict(script, course.length).duration()


func _build_list() -> void:
	_list = VBoxContainer.new()
	_list.position = Vector2(96.0, 96.0)
	_list.custom_minimum_size.x = 620.0
	_list.add_theme_constant_override("separation", 6)
	add_child(_list)
	_list.add_child(UiStyle.label("OPENCYCLE", 124, UiStyle.INK, true))
	var gap := Control.new()
	gap.custom_minimum_size.y = 40.0
	_list.add_child(gap)
	_list.add_child(UiStyle.heading("Choose a ride"))
	for key: String in _courses:
		var entry := UiStyle.button(title_of(key))
		entry.custom_minimum_size.y = 72.0
		entry.add_theme_font_size_override("font_size", 40)
		entry.name = key
		entry.pressed.connect(_ride.bind(key))
		entry.focus_entered.connect(_describe.bind(key))
		var stats := "%.1f km   %s" % [_courses[key].length / 1000.0, _minutes(_laps[key])]
		var side := UiStyle.label(stats, UiStyle.TEXT, UiStyle.DIM)
		side.set_anchors_preset(Control.PRESET_FULL_RECT)
		side.offset_right = -20.0
		side.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		side.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		entry.add_child(side)
		_list.add_child(entry)
	var second_gap := Control.new()
	second_gap.custom_minimum_size.y = 24.0
	_list.add_child(second_gap)
	var video := UiStyle.button("Video")
	video.pressed.connect(_show_video)
	_list.add_child(video)
	var leave := UiStyle.button("Quit")
	leave.pressed.connect(get_tree().quit)
	_list.add_child(leave)


func _build_details() -> void:
	_details = PanelContainer.new()
	_details.add_theme_stylebox_override("panel", UiStyle.panel_box())
	_details.custom_minimum_size.x = MAP_SIZE.x + 56.0
	add_child(_details)
	var column := VBoxContainer.new()
	column.add_theme_constant_override("separation", 8)
	_details.add_child(column)
	_detail_title = UiStyle.label("", 52, UiStyle.INK, true)
	column.add_child(_detail_title)
	_detail_stats = UiStyle.label("", UiStyle.TEXT, UiStyle.DIM)
	column.add_child(_detail_stats)
	_map = CourseMap.new()
	_map.custom_minimum_size = MAP_SIZE
	_map.mouse_filter = Control.MOUSE_FILTER_IGNORE
	column.add_child(_map)
	column.add_child(UiStyle.heading("Through"))
	_detail_places = UiStyle.label("", UiStyle.TEXT, UiStyle.INK)
	_detail_places.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	column.add_child(_detail_places)


func _focus_course(key: String) -> void:
	var entry := _list.find_child(key, false, false) as Button
	entry.grab_focus()
	_describe(key)


func _describe(key: String) -> void:
	var course := _courses[key]
	_detail_title.text = title_of(key)
	_detail_stats.text = "%.1f km a lap, about %s" % [course.length / 1000.0, _minutes(_laps[key])]
	_map.show_course(course, true)
	var names := PackedStringArray()
	for kind: Districts.Kind in course.places():
		names.append(Districts.NAMES[kind])
	_detail_places.text = "  ·  ".join(names)
	_place_details.call_deferred()


## Once the text has been measured, because the panel fits it.
func _place_details() -> void:
	_details.reset_size()
	_details.position = Vector2(1920.0, 1080.0) - _details.size - Vector2(96.0, 96.0)


static func _minutes(seconds: float) -> String:
	return "%d min" % roundi(seconds / 60.0)


func _show_video() -> void:
	_list.hide()
	_details.hide()
	_video.open()


func _show_list() -> void:
	_details.show()
	_list.show()
	_focus_course(Session.course)


func _ride(key: String) -> void:
	Session.course = key
	Session.remember()
	get_tree().change_scene_to_file(RIDE)
