class_name Animator
extends Node
## The scoring cascade: [code]animate[/code] in [code]src/ui/app.ts[/code],
## performed from a script the shell wrote ahead of time.
##
## The web build walks the guess's events and makes its choices as it goes
## (which half of the readout moved, which rung the trigger cue climbs to, what
## floats). [code]script[/code] in [code]js/shell.ts[/code] makes the same
## choices in the same walk and records them, so this side only performs: it
## never reads an event, and a new event type is a change there, not here.
##
## It works on the screen [Render] just built, restyling the boxes it finds by
## class, which is what the web's version does to its DOM. Nothing here outlives
## the cascade: when it ends, the shell is told ([code]settle[/code]) and the
## screen is rebuilt from scratch, so a class left on or a scale left off is
## gone in the next frame either way.
##
## Durations are the web's, authored at speed ×1, and each one divides by
## [member speed] exactly as [code]beat[/code] does there: a wait here and the
## motion it waits on are always scaled together, or a class comes off a tile
## still flipping.

## Every duration below, in ms at ×1; see [code]FLIP[/code], [code]GAIN[/code]
## and [code]COUNT_UP[/code] in app.ts and the keyframes they pace.
const FLIP := 380.0
const GAIN := 760.0
const COUNT_UP := 340.0
const FLOAT := 900.0
const FLOAT_RISE := 40.0
const POP := 300.0
const BUMP := 320.0
const SHAKE := 400.0
## `@keyframes shake`, at each tenth of its length, as fractions of the amplitude.
const SHAKE_KEYS: Array[float] = [0.0, -0.4, 0.7, -1.0, 1.0, -1.0, 1.0, -1.0, 0.7, -0.4, 0.0]

## Set by the player's click; every wait still to come returns at once and
## every motion snaps to its end, as [code]skipping[/code] does there.
var skipping := false
var running := false
## The animation speed setting: 1, 2 or 3.
var speed := 1.0

var _render: Render
var _fx: Control
var _screen: Control
var _num: Callable
var _cue: Callable


## [param fx] is a layer over everything, for what rises off the board;
## [param num] formats a figure as the language in force does; [param cue]
## plays a sound cue.
func setup(render: Render, fx: Control, num: Callable, cue: Callable) -> void:
	_render = render
	_fx = fx
	_num = num
	_cue = cue


## Plays [param script] over [param screen], the box the round was just drawn
## into. Returns when the cascade has finished or been skipped.
func play(script: Dictionary, screen: Control) -> void:
	running = true
	skipping = false
	_screen = screen
	var row := str(roundi(Box.num(script.get("row", 0))))
	var row_box := _render.find(["row"], "data-row", row)
	var tiles: Array[Box] = []
	var note: Box = null
	if row_box != null:
		for child: Control in row_box.flow:
			if child is Box and (child as Box).has_class("tile"):
				tiles.append(child as Box)
		for child: Control in row_box.abs_kids:
			if child is Box and (child as Box).has_class("row-note"):
				note = child as Box
	for tile: Box in tiles:
		tile.set_class("pending", true)
	if note != null:
		note.set_class("pending", true)

	var readout := _render.find(["readout"])
	var chips := _within(readout, "chips")
	var mult := _within(readout, "mult")
	var score := _within(_render.find(["hud-score"]), "score")
	# Wound back, as there: the board was drawn after the guess was committed,
	# so it holds the figures this is about to build up to.
	if chips != null:
		chips.set_text(str(script.get("chips", "0")))
	if mult != null:
		mult.set_text(str(script.get("mult", "1")))
	var target: float = maxf(1.0, Box.num(script.get("target", 1)))
	var shown: float = script.get("score", 0.0)
	_meter(shown, target, score)

	var steps: Array = script.get("steps", [])
	for s: Dictionary in steps:
		var fired: Box = null
		if s.has("flip"):
			var flip: Dictionary = s["flip"]
			var index: int = type_convert(flip["index"], TYPE_INT)
			if index < tiles.size():
				_reveal(tiles[index])
				_tile_gain(tiles[index], str(flip["chips"]), flip.get("mult"))
				if s.get("note", false) and note != null:
					_later(FLIP / 2.0, note.set_class.bind("pending", false))
		if s.has("fire"):
			var fire: Dictionary = s["fire"]
			fired = _fired(fire, tiles)
			if fired != null:
				fired.set_class("fired", true)
		if s.has("float") and readout != null:
			_floater(readout, str(s["float"]))
		if s.get("chips") != null and chips != null:
			chips.set_text(str(s["chips"]))
			_pop(chips, 1.08, BUMP)
		if s.get("mult") != null and mult != null:
			mult.set_text(str(s["mult"]))
			_pop(mult, 1.08, BUMP)
		if s.get("solved", false) and readout != null:
			readout.set_class("solved", true)
		if s.has("count"):
			var count: Dictionary = s["count"]
			_count(score, Box.num(count["from"]), Box.num(count["to"]), target)
		if s.has("ratio"):
			_emphasize(chips, mult, Box.num(s["ratio"]))
		if s.has("cue"):
			_cue.call(s["cue"])
		await _wait(Box.num(s.get("wait", 0.0)))
		if fired != null and is_instance_valid(fired):
			fired.set_class("fired", false)

	# Belt and braces, as there: a skip taken mid-flip must not leave a tile or
	# the note held back until the next rebuild happens to drop them.
	for tile: Box in tiles:
		if is_instance_valid(tile):
			tile.set_class("pending", false)
			tile.scale = Vector2.ONE
	if note != null and is_instance_valid(note):
		note.set_class("pending", false)
	running = false


func skip() -> void:
	if running:
		skipping = true


## A refusal: the row being typed shakes, as [code].row.rejected[/code] does.
func reject(row: String) -> void:
	var box := _render.find(["row"], "data-row", row)
	if box != null:
		_shake(box, 6.0)


## [code]bump[/code] in app.ts: a one-shot pop on the node [param selector]
## names. The shell only ever names a single class.
func bump(selector: String) -> void:
	var box := _render.find([selector.trim_prefix(".").get_slice(" ", 0)])
	if box != null:
		_pop(box, 1.22, POP)


func _ms(ms: float) -> float:
	return ms / 1000.0 / maxf(speed, 1.0)


func _wait(ms: float) -> void:
	if skipping or ms <= 0.0:
		return
	var until := Time.get_ticks_msec() + ms / maxf(speed, 1.0)
	# Polled rather than one timer, so a click lands on the next frame and not
	# at the end of a 900ms solve.
	while not skipping and Time.get_ticks_msec() < until:
		await get_tree().process_frame


func _later(ms: float, then: Callable) -> void:
	if skipping:
		then.call()
		return
	get_tree().create_timer(_ms(ms)).timeout.connect(
		func() -> void:
			if is_instance_valid(then.get_object()):
				then.call()
	)


## Wordle's turn-over: the colour is let in at the trough, edge-on, where there
## is nothing to see.
func _reveal(tile: Box) -> void:
	if skipping:
		tile.set_class("pending", false)
		return
	var half := _ms(FLIP / 2.0)
	var tween := tile.create_tween()
	tween.tween_property(tile, "scale:y", 0.04, half)
	tween.tween_callback(tile.set_class.bind("pending", false))
	# From the trough explicitly: the restyle just put the scale back to 1.
	tween.tween_property(tile, "scale:y", 1.0, half).from(0.04)


func _fired(fire: Dictionary, tiles: Array[Box]) -> Box:
	var index: int = type_convert(fire.get("index", 0), TYPE_INT)
	match fire.get("at"):
		"tile":
			return tiles[index] if index < tiles.size() else null
		"relic":
			return _render.find(["relic"], "data-slot", str(index))
		"category":
			return _render.find(["category"])
	return null


## What a tile paid, said at the tile, when its colour appears. Hung off the fx
## layer rather than the tile for the reason app.ts gives: a child of the tile
## would be flattened by the very flip it announces.
func _tile_gain(tile: Box, chips: String, mult: Variant) -> void:
	# Five badges landing at once on a skip is noise; the player asked for the end.
	if skipping or _decor_off():
		return
	_later(FLIP / 2.0, _show_gain.bind(tile, chips, mult))


func _show_gain(tile: Box, chips: String, mult: Variant) -> void:
	var line := HBoxContainer.new()
	line.add_theme_constant_override("separation", 4)
	line.mouse_filter = Control.MOUSE_FILTER_IGNORE
	line.add_child(_label(chips, "chips", 11))
	if mult != null:
		line.add_child(_label(str(mult), "mult", 11))
	_fx.add_child(line)
	line.reset_size()
	var rect := tile.get_global_rect()
	var at := Vector2(rect.get_center().x - line.size.x / 2.0, rect.position.y + rect.size.y * 0.18)
	line.global_position = at + Vector2(0, 5.6)
	line.modulate.a = 0.0
	var span := _ms(GAIN)
	var tween := line.create_tween()
	tween.set_parallel()
	tween.tween_property(line, "modulate:a", 1.0, span * 0.2)
	tween.tween_property(line, "global_position:y", at.y - 4.0, span * 0.2).set_ease(Tween.EASE_OUT)
	tween.chain().set_parallel()
	tween.tween_property(line, "modulate:a", 0.0, span * 0.8)
	tween.tween_property(line, "global_position:y", at.y - 28.0, span * 0.8).set_ease(
		Tween.EASE_OUT
	)
	tween.chain().tween_callback(line.queue_free)


func _floater(readout: Box, text: String) -> void:
	var label := _label(text, "gold", 12)
	_fx.add_child(label)
	label.reset_size()
	var rect := readout.get_global_rect()
	label.global_position = Vector2(
		rect.get_center().x - label.size.x / 2.0, rect.position.y - label.size.y
	)
	var span := _ms(FLOAT)
	var tween := label.create_tween()
	tween.set_parallel()
	tween.tween_property(label, "modulate:a", 0.0, span).set_ease(Tween.EASE_OUT)
	tween.tween_property(label, "position:y", label.position.y - FLOAT_RISE, span).set_ease(
		Tween.EASE_OUT
	)
	tween.chain().tween_callback(label.queue_free)


func _label(text: String, token: String, px: int) -> Label:
	var label := Label.new()
	label.text = text
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_override("font", Box.font(800))
	label.add_theme_font_size_override("font_size", px)
	label.add_theme_color_override("font_color", Box.style.color(token))
	label.add_theme_color_override("font_shadow_color", Color(0, 0, 0, 0.75))
	label.add_theme_constant_override("shadow_offset_y", 1)
	return label


## The total counts from one figure to the other, the bar under it in step.
func _count(score: Box, from: float, to: float, target: float) -> void:
	if skipping or from == to:
		_meter(to, target, score)
		return
	var tween := _fx.create_tween()
	(
		tween
		. tween_method(
			func(value: float) -> void:
				if not is_instance_valid(score):
					return
				_meter(to if skipping else value, target, score),
			from,
			to,
			_ms(COUNT_UP)
		)
		. set_trans(Tween.TRANS_CUBIC)
		. set_ease(Tween.EASE_OUT)
	)


func _meter(value: float, target: float, score: Box) -> void:
	if score != null and is_instance_valid(score):
		var text: String = _num.call(roundf(value))
		score.set_text(text)
	var bar := _render.find(["meter"])
	if bar != null:
		bar.fill = minf(1.0, value / target)
		bar.met = value >= target
		bar.queue_redraw()
	var box := _render.find(["hud-score"])
	if box != null:
		box.set_class("met", value >= target)


## Weight of the reaction, by what the guess was worth against the target.
func _emphasize(chips: Box, mult: Box, ratio: float) -> void:
	var pop := 1.0 + minf(0.2, ratio * 0.25)
	for box: Box in [chips, mult]:
		if box != null:
			_pop(box, pop, POP)
	if ratio >= 0.5 and _screen != null:
		_shake(_screen, minf(8.0, 3.0 + ratio * 4.0))


func _pop(box: Control, to: float, ms: float) -> void:
	if skipping:
		return
	var half := _ms(ms / 2.0)
	var tween := box.create_tween()
	tween.tween_property(box, "scale", Vector2(to, to), half)
	tween.tween_property(box, "scale", Vector2.ONE, half)


func _shake(box: Control, px: float) -> void:
	if skipping:
		return
	var base := box.position.x
	var step := _ms(SHAKE) / (SHAKE_KEYS.size() - 1)
	var tween := box.create_tween()
	for k: float in SHAKE_KEYS.slice(1):
		tween.tween_property(box, "position:x", base + k * px, step)


## The first box under [param box] with class [param cls], or null.
static func _within(box: Box, cls: String) -> Box:
	if box == null:
		return null
	for child: Control in box.flow + box.abs_kids:
		if child is Box:
			var kid := child as Box
			if kid.has_class(cls):
				return kid
			var deeper := _within(kid, cls)
			if deeper != null:
				return deeper
	return null


func _decor_off() -> bool:
	return Box.style.root_classes.has("quiet") or Box.style.root_classes.has("plain")
