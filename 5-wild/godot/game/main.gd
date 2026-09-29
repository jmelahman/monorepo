extends Control
## The game: 5-wild's web interface, drawn by Godot.
##
## Nothing here decides anything about play. [WildShell] runs the web build's
## controller and views in QuickJS and hands back the screen as a tree; this
## scene draws the tree ([Render], [Box], [Style]), sends back what the player
## clicked or typed, and performs what the shell says to perform: sounds, the
## refusal toast and shake, and the scoring cascade ([Animator]). Every
## sentence on screen is therefore the web's, from [code]src/ui/lang[/code],
## in whichever of its languages the player picked from its own settings sheet.
##
## The flow is the template's, one hop longer: input -> shell -> effects ->
## [method _present] -> save. As on the web, every input redraws the whole screen,
## so nothing here updates a node in place except the cascade, which the shell
## is told has finished ([method WildShell.settle]) before the next redraw.
##
## What Godot keeps for itself is what a browser would have owned: the window
## (F11 for fullscreen), the disk (the store is written through [code]Saves[/code]
## in the web build's own key-value shape, so a record reads the same in both),
## and the device's light or dark preference at first launch.

const STORE := "store"
## The height the stylesheet was written for: a phone held upright is about
## 800 CSS px tall, and every size in [Style] is in those px. The window's
## height is scaled to it, so a desktop gets the phone layout at a size meant
## for a desk rather than a phone's at a desk's distance.
const CSS_HEIGHT := 800.0
## How far above the bottom edge the toast sits, by screen, as
## [code]--toast-lift[/code] does: clear of the keyboard on a round and of the
## footer in the shop, so the refusal is never drawn over what it refuses.
const TOAST_LIFT := {"round-screen": 3 * Style.KEY_H + 12 + 8 + 16, "shop-screen": 58 + 8 + 16}
const TOAST_MS := 2200
const TOAST_FADE := 0.26

var shell: WildShell
## The last [method WildShell.render], kept so a local change (a
## [code]<details>[/code] opened) can redraw without asking the shell.
var shown: Dictionary = {}

var _style := Style.new()
var _render := Render.new(_style)
var _screen: Box = null
var _sheet: Box = null
var _focus := Focus.new()
var _toast: PanelContainer
var _toast_label: Label
var _toast_tween: Tween

@onready var _background: ColorRect = %Background
@onready var _stage: Control = %Stage
@onready var _layer: Control = %Sheet
@onready var _fx: Control = %Fx
@onready var _sound: Sound = %Sound
@onready var _animator: Animator = %Animator


func _ready() -> void:
	# Stretch fits the 1080-line viewport to the window; this fits the page to
	# the viewport, so 800 CSS px span the window's height.
	var base: float = ProjectSettings.get_setting("display/window/size/viewport_height", 1080)
	get_window().content_scale_factor = base / CSS_HEIGHT
	# A box placed absolutely is raised (see Box._place_abs), and z_index is
	# sorted across the whole canvas, not within a parent, so the keyboard's
	# letter counts drew through the sheet above them. The sheet and what plays
	# over everything sit higher than anything a screen raises itself to.
	_layer.z_index = 10
	_fx.z_index = 20
	# The ring is drawn over a sheet, and under what plays over everything.
	_focus.z_index = 15
	add_child(_focus)
	_render.redraw = _rebuild
	Box.on_click = _on_click
	_animator.setup(_render, _fx, _num, _sound.cue)
	_build_toast()
	resized.connect(_layout)
	shell = WildShell.open()
	if shell == null:
		return
	var saved: Dictionary = Saves.read(STORE).get("items", {})
	var device := "dark" if DisplayServer.is_dark_mode() else "light"
	_apply(shell.boot(saved, device))
	_present()


## _input rather than _unhandled_input, so typing works whatever the pointer
## is over, and because gdUnit's scene runner calls _unhandled_input directly as
## well as feeding the event through, which would type every letter twice.
func _input(event: InputEvent) -> void:
	if shell == null:
		return
	var mouse := event as InputEventMouseButton
	if mouse != null and mouse.pressed:
		_focus.pointer()
	if _navigate(event):
		get_viewport().set_input_as_handled()
		return
	if mouse != null and mouse.pressed and _animator.running:
		# A tap during the cascade asks for the end of it, as on the web.
		_animator.skip()
		get_viewport().set_input_as_handled()
		return
	var key := event as InputEventKey
	if key != null and key.pressed and not key.echo and key.keycode == KEY_F11:
		Settings.set_value("display/fullscreen", not Settings.get_bool("display/fullscreen"))
	elif event.is_action_pressed("pause"):
		press("escape")
	elif event.is_action_pressed("submit"):
		press("enter")
	elif event.is_action_pressed("erase", true):
		_type("back")
	elif _letter(event) != "":
		_type(_letter(event).to_lower())
	else:
		return
	get_viewport().set_input_as_handled()


## Tab, and the keys that press what it landed on; true if the event was one.
##
## Enter and Space press the focused box, as a browser's would, with the one
## exception [code]app.ts[/code] makes: Enter over a letter of the board's own
## keyboard is how the guess gets submitted with a hand still on the keys, so
## it falls through to submit. The picker's keyboard sits in a sheet, where a
## letter pressed is a letter placed. The arrows (and a pad's D-pad) are left
## to Godot's neighbour search once something is focused, and from nothing
## they start where Tab would.
func _navigate(event: InputEvent) -> bool:
	# Shift+Tab matches the plain Tab action too, so it is asked first.
	var back := event.is_action_pressed("ui_focus_prev", true)
	if back or event.is_action_pressed("ui_focus_next", true):
		_focus.step(back)
		return true
	var box := _focus.focused()
	if box != null:
		return event.is_action_pressed("ui_accept") and _accept(event, box)
	for way: String in ["ui_up", "ui_down", "ui_left", "ui_right"]:
		if event.is_action_pressed(way, true):
			_focus.step(way in ["ui_up", "ui_left"])
			return true
	return false


## Presses [param box] unless [param event] is Enter over the board's keyboard.
func _accept(event: InputEvent, box: Box) -> bool:
	if event.is_action_pressed("submit") and _render.find(["sheet"]) == null:
		var at := box
		while at != null:
			if at.has_class("keyboard"):
				return false
			at = at.parent_box
	box.press()
	return true


## A letter or a backspace for the round. Outside a sheet it also lets go of
## focus ([method Focus.drop]); inside one the sheet eats the letter, and focus
## on its settings stays put.
func _type(key_name: String) -> void:
	if _render.find(["sheet"]) == null:
		_focus.drop()
	press(key_name)


## A key, by the shell's name for it. Public so tests and the agent bridge
## type as a player does.
func press(key_name: String) -> void:
	if _animator.running:
		_animator.skip()
		return
	_apply(shell.key(key_name))


## The run as the engine holds it, for tests and the agent bridge.
func state() -> Dictionary:
	return shell.state()


## A run from [param run_seed], past the title, for tests and the agent
## bridge: a known seed is a known answer.
func start_seeded(run_seed: int) -> void:
	_apply(shell.start_seeded(run_seed))


## Whether the screen in view is [param kind] ([code]title[/code],
## [code]round-screen[/code], [code]shop-screen[/code]), for tests and the bridge.
func showing(kind: String) -> bool:
	return _screen != null and _screen.has_class(kind)


## A letter from the key's printed character rather than its physical key, so
## AZERTY types an A where the A is. Accents fold as the web build's do: the
## engine's alphabet is 26 letters and CAFÉ is CAFE.
static func _letter(event: InputEvent) -> String:
	var key := event as InputEventKey
	if key == null or not key.pressed or key.echo or key.ctrl_pressed or key.alt_pressed:
		return ""
	# Synthetic keys (gdUnit's, the bridge's) carry a keycode and no unicode.
	var code := key.unicode if key.unicode else int(key.keycode)
	var folded := _fold(String.chr(code).to_upper())
	return folded if folded.length() == 1 and folded >= "A" and folded <= "Z" else ""


static func _fold(ch: String) -> String:
	const FROM := "ÀÁÂÃÄÅÇÈÉÊËÌÍÎÏÑÒÓÔÕÖÙÚÛÜÝ"
	const TO := "AAAAAACEEEEIIIINOOOOOUUUUY"
	var i := FROM.find(ch)
	return TO[i] if i >= 0 else ch


func _on_click(id: int) -> void:
	if _animator.running:
		_animator.skip()
		return
	_apply(shell.click(id))


## Does what the shell said to. The order is the web's: the screen is drawn
## before anything that plays over it, and the cascade last, since it plays over
## the screen the submit drew and ends in a render of its own.
func _apply(effects: Dictionary) -> void:
	var cues: Array = effects.get("cues", [])
	for cue: Dictionary in cues:
		_sound.cue(cue)
	if effects.get("render", false) or effects.has("animate"):
		_present()
	if effects.has("toast"):
		_show_toast(str(effects["toast"]))
	if effects.get("shake", false):
		var round_state: Dictionary = shell.state().get("round", {})
		var guesses: Array = round_state.get("guesses", [])
		_animator.reject(str(guesses.size()))
	if effects.has("bump"):
		_animator.bump(str(effects["bump"]))
	if effects.has("open"):
		OS.shell_open(str(effects["open"]))
	_save()
	if effects.has("animate") and _screen != null:
		var script: Dictionary = effects["animate"]
		await _animator.play(script, _screen)
		_apply(shell.settle())


func _present() -> void:
	shown = shell.render()
	var cues: Array = shown.get("cues", [])
	for cue: Dictionary in cues:
		_sound.cue(cue)
	_sound.muted = shown.get("muted", false)
	var music_off: bool = shown.get("musicOff", false)
	_sound.music(not music_off, str(shown.get("track", "promises")))
	_animator.speed = type_convert(shown.get("speed", 1), TYPE_FLOAT)
	TranslationServer.set_locale(str(shown.get("lang", "en")))
	_rebuild()


## The last trees, drawn from scratch. The theme is set first because every
## colour a box resolves is resolved as it is built.
func _rebuild() -> void:
	_style.set_theme(str(shown.get("theme", "dark")), str(shown.get("decor", "all")))
	_background.color = _style.color("bg")
	theme = _theme()
	_render.clear()
	for old: Box in [_screen, _sheet]:
		if old != null:
			old.queue_free()
	_screen = null
	_sheet = null
	if shown.get("screen") is Dictionary:
		var screen: Dictionary = shown["screen"]
		_screen = _render.build(screen)
	if shown.get("sheet") is Dictionary:
		var sheet: Dictionary = shown["sheet"]
		_sheet = _render.build(sheet)
	if _screen != null:
		_stage.add_child(_screen)
	if _sheet != null:
		_layer.add_child(_sheet)
		# A click on the sheet itself is not a click on the backdrop behind it,
		# which is what dismisses; the browser gets this from event targets.
		for panel: Box in _render.find_all(["sheet"]) + _render.find_all(["pack-sheet"]):
			panel.mouse_filter = Control.MOUSE_FILTER_STOP
	_layout()
	var roots: Array[Box] = []
	for root: Box in [_screen, _sheet]:
		if root != null:
			roots.append(root)
	var panel := _render.find(["sheet"])
	_focus.settle(roots, panel if panel != null else _screen, _style.color("fg"))


func _layout() -> void:
	var area := size
	Box.page_h = area.y
	if _screen != null:
		var maxw: float = Box.css(_screen).get("maxw", 480.0)
		var w := minf(area.x, maxw)
		_screen.position = Vector2(roundf((area.x - w) / 2.0), 0)
		_screen.size = Vector2(w, area.y)
		_screen._arrange()
	if _sheet != null:
		_sheet.position = Vector2.ZERO
		_sheet.size = area
		_sheet._arrange()
	_place_toast()


func _num(value: float) -> String:
	return shell.num(value)


## The store goes to disk whenever the shell wrote to it: after every input, as
## the web build writes localStorage after every dispatch.
func _save() -> void:
	var items := shell.flush()
	if not items.is_empty():
		Saves.write(STORE, {"items": items})


# ------------------------------------------------------------------ chrome


## Godot's own controls, tooltips mostly, in the page's font and colours.
func _theme() -> Theme:
	var t := Theme.new()
	t.default_font = Box.font(400)
	t.default_font_size = 13
	var panel := StyleBoxFlat.new()
	panel.bg_color = _style.color("panel-2")
	panel.border_color = _style.color("line")
	panel.set_border_width_all(1)
	panel.set_corner_radius_all(8)
	panel.set_content_margin_all(8)
	t.set_stylebox("panel", "TooltipPanel", panel)
	t.set_color("font_color", "TooltipLabel", _style.color("fg"))
	return t


func _build_toast() -> void:
	_toast = PanelContainer.new()
	_toast.name = "Toast"
	_toast.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_toast.modulate.a = 0.0
	var box := StyleBoxFlat.new()
	box.bg_color = Color(0, 0, 0, 0.8)
	box.set_corner_radius_all(8)
	box.content_margin_left = 14
	box.content_margin_right = 14
	box.content_margin_top = 8
	box.content_margin_bottom = 8
	_toast.add_theme_stylebox_override("panel", box)
	_toast_label = Label.new()
	_toast_label.name = "ToastText"
	_toast_label.add_theme_font_override("font", Box.font(400))
	_toast_label.add_theme_font_size_override("font_size", 12)
	_toast_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_toast.add_child(_toast_label)
	_fx.add_child(_toast)


## The refusal's sentence. Held for reading time, which is not a motion and so
## not on the speed setting; see [code]TOAST[/code] in app.ts.
func _show_toast(text: String) -> void:
	_toast_label.text = text
	_toast_label.add_theme_color_override("font_color", _style.color("on-tile"))
	_place_toast()
	if _toast_tween != null:
		_toast_tween.kill()
	_toast_tween = create_tween()
	_toast_tween.tween_property(_toast, "modulate:a", 1.0, TOAST_FADE)
	_toast_tween.tween_interval(TOAST_MS / 1000.0)
	_toast_tween.tween_property(_toast, "modulate:a", 0.0, TOAST_FADE)


func _place_toast() -> void:
	if _toast == null:
		return
	var lift := 16.0
	if _screen != null:
		for kind: String in TOAST_LIFT:
			if _screen.has_class(kind):
				lift = TOAST_LIFT[kind]
	_toast_label.custom_minimum_size = Vector2.ZERO
	_toast.reset_size()
	var w := minf(_toast.get_combined_minimum_size().x, size.x * 0.9)
	_toast.size = Vector2(w, 0)
	_toast.reset_size()
	_toast.position = Vector2((size.x - _toast.size.x) / 2.0, size.y - lift - _toast.size.y)
