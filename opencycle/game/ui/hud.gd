class_name Hud
extends CanvasLayer
## What a rider wants to know, kept to the bottom of the screen and out of the way of the street.

const REFRESH := 0.1
const PLACE_SHOWS := 5.0
const PLACE_FADES := 1.0
const BAR_HIGH := 118.0
const MAP_SIZE := Vector2(380.0, 150.0)

var _replay: Replay
var _route: Route
var _course: Course
var _since := REFRESH
var _place_shown := -1
var _place_left := 0.0
var _gpu_viewport := RID()
var _speed: Label
var _distance: Label
var _time: Label
var _lap: Label
var _grade: Label
var _place: Label
var _rate: Label
var _map: CourseMap


func _init(replay: Replay, route: Route, course: Course) -> void:
	layer = 10
	_replay = replay
	_route = route
	_course = course


func _ready() -> void:
	var screen := Control.new()
	screen.set_anchors_preset(Control.PRESET_FULL_RECT)
	screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
	screen.theme = UiStyle.theme()
	add_child(screen)
	var plate := PanelContainer.new()
	plate.add_theme_stylebox_override("panel", UiStyle.panel_box())
	plate.mouse_filter = Control.MOUSE_FILTER_IGNORE
	plate.set_anchors_preset(Control.PRESET_BOTTOM_LEFT)
	plate.grow_vertical = Control.GROW_DIRECTION_BEGIN
	plate.position = Vector2(40.0, 1080.0 - 40.0 - BAR_HIGH)
	screen.add_child(plate)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 44)
	plate.add_child(row)
	_speed = _reading(row, "km/h", 76)
	_speed.add_theme_color_override("font_color", UiStyle.ACCENT)
	_distance = _reading(row, "km", 44)
	_time = _reading(row, "time", 44)
	_grade = _reading(row, "grade", 44)
	if _replay.laps:
		_lap = _reading(row, "lap", 44)
	if _course:
		var holder := PanelContainer.new()
		holder.add_theme_stylebox_override("panel", UiStyle.panel_box())
		holder.mouse_filter = Control.MOUSE_FILTER_IGNORE
		holder.set_anchors_preset(Control.PRESET_BOTTOM_RIGHT)
		holder.grow_horizontal = Control.GROW_DIRECTION_BEGIN
		holder.grow_vertical = Control.GROW_DIRECTION_BEGIN
		holder.position = Vector2(1920.0 - 40.0, 1080.0 - 40.0)
		screen.add_child(holder)
		_map = CourseMap.new()
		_map.custom_minimum_size = MAP_SIZE
		_map.mouse_filter = Control.MOUSE_FILTER_IGNORE
		holder.add_child(_map)
		_map.show_course(_course)
		holder.reset_size()
		holder.position = Vector2(1920.0, 1080.0) - holder.size - Vector2(40.0, 40.0)
	_place = UiStyle.label("", 44, UiStyle.INK, true)
	_place.position = Vector2(68.0, 40.0)
	_place.add_theme_color_override("font_shadow_color", Color(0.0, 0.0, 0.0, 0.8))
	_place.add_theme_constant_override("shadow_offset_y", 2)
	screen.add_child(_place)
	_rate = UiStyle.label("", UiStyle.SMALL, UiStyle.COLD)
	_rate.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	_rate.grow_horizontal = Control.GROW_DIRECTION_BEGIN
	_rate.position = Vector2(1920.0 - 40.0, 40.0)
	screen.add_child(_rate)
	plate.reset_size()
	plate.position = Vector2(40.0, 1080.0 - 40.0 - plate.size.y)


## A number over the small word for what it is.
func _reading(row: HBoxContainer, what: String, size: int) -> Label:
	var column := VBoxContainer.new()
	column.alignment = BoxContainer.ALIGNMENT_END
	column.add_theme_constant_override("separation", -6)
	row.add_child(column)
	var number := UiStyle.label("", size, UiStyle.INK, true)
	column.add_child(number)
	column.add_child(UiStyle.label(what, UiStyle.SMALL, UiStyle.DIM))
	return number


## Turns on the frame rate and GPU time, measured from [param viewport].
func show_frame_rate(viewport: RID, shown: bool) -> void:
	_gpu_viewport = viewport
	RenderingServer.viewport_set_measure_render_time(viewport, shown)
	if _rate:
		_rate.visible = shown


func show_at(time: float, delta: float) -> void:
	_since += delta
	_fade_place(delta)
	if _since < REFRESH:
		return
	_since = 0.0
	var ridden := _replay.ridden(time)
	_speed.text = "%.0f" % _replay.kmh_at(time)
	_distance.text = "%.2f" % (ridden / 1000.0)
	_time.text = UiStyle.clock(time)
	_grade.text = "%+.1f%%" % _route.gradient(ridden)
	if _lap:
		_lap.text = str(_replay.lap_at(time) + 1)
	if _course:
		_map.rider = fposmod(ridden, _course.length)
		_name_place(_course.kind_at(ridden))
	if _rate.visible:
		var gpu := RenderingServer.viewport_get_measured_render_time_gpu(_gpu_viewport)
		_rate.text = "%d fps   GPU %.1f ms" % [Engine.get_frames_per_second(), gpu]


## Says where the rider's got to on the way in, then gets out of the way.
func _name_place(kind: Districts.Kind) -> void:
	if kind == _place_shown:
		return
	_place_shown = kind
	_place.text = Districts.NAMES[kind]
	_place_left = PLACE_SHOWS


func _fade_place(delta: float) -> void:
	_place_left = maxf(_place_left - delta, 0.0)
	_place.modulate.a = clampf(_place_left / PLACE_FADES, 0.0, 1.0)
