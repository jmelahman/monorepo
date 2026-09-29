class_name Focus
extends Control
## The keyboard's place on the page: which box Tab has landed on, the ring
## round it, and keeping both across the rebuild that follows every press.
##
## This is the half of [code]app.ts[/code] the shell leaves to Godot (see
## [code]key[/code] in [code]js/shell.ts[/code]), and it keeps the web's rules
## rather than Godot's defaults, because the views were written against them:
##
## - What Tab visits is what a browser's Tab would: a box with a handler, a link,
##   or a [code]tabindex[/code] of 0 or more (a relic card, there to be read).
##   The sheet's own [code]tabindex="-1"[/code] keeps it out.
## - An open [code].sheet[/code] is the whole page to the keyboard, as
##   [code]trapTab[/code] makes it. Here that is done by taking focus off
##   everything outside it, which traps the arrow keys and a pad's D-pad along
##   with Tab, since Godot's own neighbour search then has nothing else to find.
## - The ring is for the keyboard only, as [code]:focus-visible[/code] is. A
##   click also focuses what it hits, and a mouse press puts the ring away.
## - A rebuild frees the focused box, and only a [code]data-focus[/code] name
##   survives it, looked up in the new sheet if one is open and the new screen
##   if not ([code]holdFocus[/code]). Anything unnamed goes back to the top of
##   the tab order, as it does on the web; the views name what they want kept.

## [code]:focus-visible[/code]'s ring: 2px, 2px outside the control.
const RING_PX := 2.0
const RING_OFF := 2.0
## Where the web draws it inside instead, because the list clips to its
## corners (a settings row) or the box reaches the sheet's edges.
const INSET: PackedStringArray = ["setting", "sheet-body"]

## True while the keyboard is what moved focus last.
var keyboard := false

## The [code]data-focus[/code] of the box focus is on, carried across a rebuild.
var _name := ""
## The sheet if one is open, else the screen: all Tab can reach.
var _scope: Control = null
var _ring := StyleBoxFlat.new()


func _ready() -> void:
	mouse_filter = MOUSE_FILTER_IGNORE
	set_anchors_preset(PRESET_FULL_RECT)
	_ring.draw_center = false
	_ring.set_border_width_all(int(RING_PX))
	_ring.anti_aliasing = true
	get_viewport().gui_focus_changed.connect(_moved)


## After a rebuild: fence the keyboard into [param scope] and put it back on
## the box wearing the name it was on, if the new page has one.
func settle(roots: Array[Box], scope: Box, ring: Color) -> void:
	_scope = scope
	_ring.border_color = ring
	for root: Box in roots:
		for box: Box in _stops(root):
			box.focus_mode = (
				FOCUS_ALL if scope != null and scope.is_ancestor_of(box) else FOCUS_NONE
			)
	var named: Box = null
	if keyboard and _name != "" and scope != null:
		for box: Box in _stops(scope):
			if _attr(box, "data-focus") == _name:
				named = box
				break
	if named != null:
		named.grab_focus()
	else:
		_name = ""
	queue_redraw()


## Tab, or Shift+Tab with [param back]: the next stop in the scope, round
## the end to the start as the trap wraps. From nowhere, the first or the last.
func step(back: bool) -> void:
	keyboard = true
	var stops := _stops(_scope)
	if stops.is_empty():
		return
	var at := stops.find(get_viewport().gui_get_focus_owner())
	var next := (stops.size() - 1 if back else 0) if at < 0 else at + (-1 if back else 1)
	stops[posmod(next, stops.size())].grab_focus()


## The box the keyboard is on, or null when there is none or the mouse took
## over since: a clicked box is focused too, but Enter was not aimed at it.
func focused() -> Box:
	var holder := get_viewport().gui_get_focus_owner() as Box
	if not keyboard or holder == null or _scope == null or not _scope.is_ancestor_of(holder):
		return null
	return holder


## A mouse press: the ring goes, and focus stays for the next Tab to leave from.
func pointer() -> void:
	keyboard = false
	queue_redraw()


## Typing on the board, which is a statement that the round has the player's
## attention: focus lets go, and with it the name that would have brought it
## back after the render the letter causes. Otherwise Enter would press the
## switch still quietly holding it instead of playing the guess ([code]app.ts
## [/code] blurs for the same reason).
func drop() -> void:
	var holder := get_viewport().gui_get_focus_owner()
	if holder != null:
		holder.release_focus()
	_name = ""
	queue_redraw()


func _process(_delta: float) -> void:
	# The box moves under the ring: a scroll, a cascade, a sheet rising.
	if keyboard:
		queue_redraw()


func _draw() -> void:
	var box := focused()
	if box == null or not box.is_visible_in_tree():
		return
	var off := RING_OFF
	for cls: String in INSET:
		if box.has_class(cls):
			off = -RING_OFF
	# A StyleBoxFlat's border is drawn inside its rect, and an outline follows
	# the box's corners out by its own offset, so both grow by the ring.
	var r: Variant = box.own.get("r", 0)
	var radius := Box.num(r[0] if r is Array else r)
	var grow := off + RING_PX
	_ring.set_corner_radius_all(maxi(0, roundi(radius + grow)) if radius > 0 else 0)
	draw_set_transform_matrix(get_global_transform().affine_inverse() * box.get_global_transform())
	draw_style_box(_ring, Rect2(Vector2.ZERO, box.size).grow(grow))


func _moved(control: Control) -> void:
	var box := control as Box
	_name = _attr(box, "data-focus") if box != null else ""
	if keyboard:
		# A tabbed-to box inside a scroller is scrolled to, as a browser does.
		var at := control.get_parent()
		while at != null and at != _scope:
			if at is ScrollContainer:
				(at as ScrollContainer).ensure_control_visible(control)
			at = at.get_parent()
	queue_redraw()


## Every stop under [param root], in tree order, which is the page's.
static func _stops(root: Node) -> Array[Box]:
	var out: Array[Box] = []
	if root == null:
		return out
	for kid: Node in root.get_children():
		var box := kid as Box
		if box != null and _stop(box) and box.is_visible_in_tree():
			out.append(box)
		out.append_array(_stops(kid))
	return out


static func _stop(box: Box) -> bool:
	if box.click_id >= 0 or box.local_click.is_valid() or box.href != "":
		return true
	var index := _attr(box, "tabindex")
	return index != "" and not box.wrapper and Box.num(index) >= 0


static func _attr(box: Box, key: String) -> String:
	var attrs: Dictionary = box.node.get("a", {})
	return str(attrs.get(key, ""))
