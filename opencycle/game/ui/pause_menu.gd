class_name PauseMenu
extends CanvasLayer
## Stops the ride, with the street still in view so a video option can be judged as it changes.

signal resumed
signal restarted
signal ended
signal video_changed

var _column: PanelContainer
var _shade: ColorRect
var _video: VideoPanel
var _resume: Button


func _init(options: VideoOptions) -> void:
	layer = 20
	process_mode = Node.PROCESS_MODE_ALWAYS
	visible = false
	_video = VideoPanel.new(options)


func _ready() -> void:
	var screen := Control.new()
	screen.set_anchors_preset(Control.PRESET_FULL_RECT)
	screen.theme = UiStyle.theme()
	screen.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(screen)
	_shade = ColorRect.new()
	_shade.color = Color(0.0, 0.0, 0.0, 0.45)
	_shade.set_anchors_preset(Control.PRESET_FULL_RECT)
	_shade.mouse_filter = Control.MOUSE_FILTER_IGNORE
	screen.add_child(_shade)
	_column = PanelContainer.new()
	_column.add_theme_stylebox_override("panel", UiStyle.panel_box())
	_column.custom_minimum_size.x = 420.0
	_column.position = Vector2(96.0, 300.0)
	screen.add_child(_column)
	var list := VBoxContainer.new()
	list.add_theme_constant_override("separation", 4)
	_column.add_child(list)
	list.add_child(UiStyle.label("Paused", 52, UiStyle.INK, true))
	_resume = _entry(list, "Resume", resume)
	_entry(list, "Restart ride", restarted.emit)
	_entry(list, "Video", _show_video)
	_entry(list, "End ride", ended.emit)
	_video.position = Vector2(96.0, 60.0)
	_video.hide()
	_video.changed.connect(video_changed.emit)
	_video.closed.connect(_show_list)
	screen.add_child(_video)


func _entry(list: VBoxContainer, title: String, action: Callable) -> Button:
	var entry := UiStyle.button(title)
	entry.pressed.connect(action)
	list.add_child(entry)
	return entry


func open() -> void:
	visible = true
	get_tree().paused = true
	Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
	_show_list()


func resume() -> void:
	visible = false
	get_tree().paused = false
	Input.mouse_mode = Input.MOUSE_MODE_HIDDEN
	resumed.emit()


func _unhandled_input(event: InputEvent) -> void:
	if visible and _column.visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		resume()


func _show_video() -> void:
	_column.hide()
	_shade.hide()
	_video.open()


func _show_list() -> void:
	_shade.show()
	_column.show()
	_resume.grab_focus()
