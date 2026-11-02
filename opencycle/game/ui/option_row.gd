class_name OptionRow
extends Button
## One setting: its name, and its value between arrows. Left and right change it; a click moves on
## to the next and a right click goes back.

signal stepped(by: int)

var _value: Label


func _init(title: String) -> void:
	custom_minimum_size.y = 42.0
	button_mask = MOUSE_BUTTON_MASK_LEFT | MOUSE_BUTTON_MASK_RIGHT
	mouse_entered.connect(grab_focus)
	var row := HBoxContainer.new()
	row.set_anchors_preset(Control.PRESET_FULL_RECT)
	row.offset_left = 20.0
	row.offset_right = -14.0
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(row)
	var name_label := UiStyle.label(title, UiStyle.TEXT)
	name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(name_label)
	_value = UiStyle.label("", UiStyle.TEXT, UiStyle.ACCENT)
	_value.custom_minimum_size.x = 170.0
	_value.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	row.add_child(_value)


func show_value(text_now: String) -> void:
	_value.text = "‹  %s  ›" % text_now


func _gui_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_left", true):
		stepped.emit(-1)
		accept_event()
	elif event.is_action_pressed("ui_right", true):
		stepped.emit(1)
		accept_event()
	elif event is InputEventMouseButton:
		var mouse := event as InputEventMouseButton
		if mouse.pressed and mouse.button_index == MOUSE_BUTTON_RIGHT:
			stepped.emit(-1)
			accept_event()
		elif mouse.pressed and mouse.button_index == MOUSE_BUTTON_LEFT:
			stepped.emit(1)
			accept_event()
	elif event.is_action_pressed("ui_accept"):
		stepped.emit(1)
		accept_event()
