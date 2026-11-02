class_name LoadingScreen
extends CanvasLayer
## Covers the build, and the first frames of the street while its shaders compile behind this.

const FADE := 0.5
const BAR_SIZE := Vector2(620.0, 6.0)

var _title: String
var _screen: Control
var _bar: ColorRect
var _shown := 0.0
var _wanted := 0.0


func _init(title: String) -> void:
	layer = 100
	_title = title


func _ready() -> void:
	_screen = Control.new()
	_screen.set_anchors_preset(Control.PRESET_FULL_RECT)
	_screen.theme = UiStyle.theme()
	add_child(_screen)
	var black := ColorRect.new()
	black.color = Color.BLACK
	black.set_anchors_preset(Control.PRESET_FULL_RECT)
	_screen.add_child(black)
	_screen.add_child(UiStyle.backdrop(0.92))
	var column := VBoxContainer.new()
	column.add_theme_constant_override("separation", 14)
	column.position = Vector2(96.0, 780.0)
	_screen.add_child(column)
	column.add_child(UiStyle.heading("Getting the street ready"))
	column.add_child(UiStyle.label(_title, 76, UiStyle.INK, true))
	var track := ColorRect.new()
	track.color = UiStyle.FAINT
	track.custom_minimum_size = BAR_SIZE
	column.add_child(track)
	_bar = ColorRect.new()
	_bar.color = UiStyle.ACCENT
	_bar.size = Vector2(0.0, BAR_SIZE.y)
	track.add_child(_bar)
	column.add_child(
		UiStyle.label("Esc pauses the ride.   H hides the HUD.", UiStyle.SMALL, UiStyle.DIM)
	)


func show_progress(fraction: float) -> void:
	_wanted = clampf(fraction, 0.0, 1.0)


func _process(delta: float) -> void:
	_shown = lerpf(_shown, _wanted, minf(delta * 8.0, 1.0))
	_bar.size.x = BAR_SIZE.x * _shown


func finish() -> void:
	var tween := create_tween()
	tween.tween_property(_screen, "modulate:a", 0.0, FADE)
	tween.tween_callback(queue_free)
