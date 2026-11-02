class_name VideoPanel
extends PanelContainer
## The video options, for the menu and for pausing a ride, where each change shows straight away.

signal changed
signal closed

const FIRST_DISPLAY_OPTION := "fullscreen"

var _options: VideoOptions
var _rows: Dictionary[String, OptionRow] = {}
var _preset: OptionRow
var _about: Label


func _init(options: VideoOptions) -> void:
	_options = options
	add_theme_stylebox_override("panel", UiStyle.panel_box())
	custom_minimum_size.x = 720.0
	var column := VBoxContainer.new()
	column.add_theme_constant_override("separation", 2)
	add_child(column)
	column.add_child(UiStyle.label("Video", 52, UiStyle.INK, true))
	_preset = OptionRow.new("Preset")
	_preset.stepped.connect(_step_preset)
	_preset.focus_entered.connect(_explain.bind("Sets everything above Fullscreen in one go."))
	column.add_child(_preset)
	for key: String in VideoOptions.OPTIONS:
		if key == FIRST_DISPLAY_OPTION:
			column.add_child(_gap())
		var option: Dictionary = VideoOptions.OPTIONS[key]
		var label_text: String = option["label"]
		var about_text: String = option["about"]
		var row := OptionRow.new(label_text)
		row.stepped.connect(_step.bind(key))
		row.focus_entered.connect(_explain.bind(about_text))
		column.add_child(row)
		_rows[key] = row
	column.add_child(_gap())
	_about = UiStyle.label("", UiStyle.SMALL, UiStyle.DIM)
	_about.custom_minimum_size.y = 34.0
	column.add_child(_about)
	var back := UiStyle.button("Back")
	back.pressed.connect(close)
	column.add_child(back)
	_refresh()


func open() -> void:
	show()
	_refresh()
	_preset.grab_focus()


func close() -> void:
	hide()
	closed.emit()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


func _gap() -> Control:
	var gap := Control.new()
	gap.custom_minimum_size.y = 10.0
	return gap


func _explain(text: String) -> void:
	_about.text = text


func _step(by: int, key: String) -> void:
	var choices: Array = VideoOptions.OPTIONS[key]["choices"]
	_options.set_value(key, posmod(_options.value(key) + by, choices.size()))
	_changed()


## Goes round the presets, past wherever a custom set of options is.
func _step_preset(by: int) -> void:
	var names: Array[String] = []
	names.assign(VideoOptions.PRESETS.keys())
	var at := names.find(_options.preset())
	if at < 0:
		at = 0 if by > 0 else names.size()
		at -= by if by > 0 else 0
	_options.use_preset(names[posmod(at + by, names.size())])
	_changed()


func _changed() -> void:
	_options.save()
	_refresh()
	changed.emit()


func _refresh() -> void:
	_preset.show_value(_options.preset())
	for key: String in _rows:
		var choices: Array = VideoOptions.OPTIONS[key]["choices"]
		var choice: String = choices[_options.value(key)]
		_rows[key].show_value(choice)
