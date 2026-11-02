class_name UiStyle
extends RefCounted
## How the menus and the HUD look: type like a road sign, square corners, sodium on black.

const MEDIUM: Font = preload("res://assets/ui/BarlowCondensed-Medium.ttf")
const BOLD: Font = preload("res://assets/ui/BarlowCondensed-SemiBold.ttf")
const BACKDROP: Texture2D = preload("res://assets/ui/backdrop.jpg")
const INK := Color(0.94, 0.92, 0.87)
const DIM := Color(0.62, 0.64, 0.66)
const ACCENT := Color(1.0, 0.68, 0.32)
const COLD := Color(0.45, 0.82, 1.0)
const PANEL := Color(0.012, 0.016, 0.022, 0.84)
const FAINT := Color(0.94, 0.92, 0.87, 0.1)
const TEXT := 28
const SMALL := 24
const EDGE := 4

static var _theme: Theme


static func theme() -> Theme:
	if _theme:
		return _theme
	_theme = Theme.new()
	_theme.default_font = MEDIUM
	_theme.default_font_size = TEXT
	_theme.set_stylebox("normal", "Button", _plate(Color(0, 0, 0, 0), Color(0, 0, 0, 0)))
	_theme.set_stylebox("hover", "Button", _plate(FAINT, Color(0, 0, 0, 0)))
	_theme.set_stylebox("pressed", "Button", _plate(FAINT, ACCENT))
	_theme.set_stylebox("focus", "Button", _plate(FAINT, ACCENT))
	_theme.set_color("font_color", "Button", DIM)
	_theme.set_color("font_hover_color", "Button", INK)
	_theme.set_color("font_focus_color", "Button", INK)
	_theme.set_color("font_pressed_color", "Button", ACCENT)
	_theme.set_color("font_hover_pressed_color", "Button", ACCENT)
	_theme.set_constant("align_to_largest_stylebox", "Button", 1)
	_theme.set_color("font_color", "Label", INK)
	return _theme


## A plain panel behind text, so it reads over whatever the street's doing.
static func panel_box() -> StyleBoxFlat:
	var box := StyleBoxFlat.new()
	box.bg_color = PANEL
	box.content_margin_left = 28.0
	box.content_margin_right = 28.0
	box.content_margin_top = 20.0
	box.content_margin_bottom = 20.0
	return box


static func _plate(fill: Color, edge: Color) -> StyleBoxFlat:
	var box := StyleBoxFlat.new()
	box.bg_color = fill
	box.border_color = edge
	box.border_width_left = EDGE
	box.content_margin_left = 20.0
	box.content_margin_right = 20.0
	box.content_margin_top = 8.0
	box.content_margin_bottom = 8.0
	return box


static func label(text: String, size := TEXT, color := INK, bold := false) -> Label:
	var made := Label.new()
	made.text = text
	made.add_theme_font_size_override("font_size", size)
	made.add_theme_color_override("font_color", color)
	if bold:
		made.add_theme_font_override("font", BOLD)
	made.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return made


## A small heading in capitals.
static func heading(text: String) -> Label:
	return label(text.to_upper(), SMALL, ACCENT, true)


static func button(text: String) -> Button:
	var made := Button.new()
	made.text = text
	made.alignment = HORIZONTAL_ALIGNMENT_LEFT
	made.mouse_entered.connect(made.grab_focus)
	return made


## The street behind a screen, with [param shade] of black over it from the left.
static func backdrop(shade: float) -> Control:
	var holder := Control.new()
	holder.set_anchors_preset(Control.PRESET_FULL_RECT)
	holder.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var picture := TextureRect.new()
	picture.texture = BACKDROP
	picture.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	picture.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	picture.set_anchors_preset(Control.PRESET_FULL_RECT)
	picture.mouse_filter = Control.MOUSE_FILTER_IGNORE
	holder.add_child(picture)
	var fade := Gradient.new()
	fade.set_color(0, Color(0.0, 0.0, 0.0, shade))
	fade.set_color(1, Color(0.0, 0.0, 0.0, shade * 0.25))
	var texture := GradientTexture2D.new()
	texture.gradient = fade
	texture.width = 256
	texture.height = 4
	var over := TextureRect.new()
	over.texture = texture
	over.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	over.set_anchors_preset(Control.PRESET_FULL_RECT)
	over.mouse_filter = Control.MOUSE_FILTER_IGNORE
	holder.add_child(over)
	return holder


static func clock(seconds: float) -> String:
	var whole := int(seconds)
	@warning_ignore("integer_division")
	var hours := whole / 3600
	@warning_ignore("integer_division")
	var minutes := whole / 60 % 60
	var rest := whole % 60
	if hours > 0:
		return "%d:%02d:%02d" % [hours, minutes, rest]
	return "%02d:%02d" % [minutes, rest]
