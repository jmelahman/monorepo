class_name Render
extends RefCounted
## Builds a view's tree ([code]serialize[/code] in [code]js/dom-shim.ts[/code])
## into [Box]es: the web build's [code]views.ts[/code], drawn by Godot.
##
## Like the web build, every render is the whole screen built from scratch, so
## nothing here updates a node in place; the one exception is the scoring
## animation, which restyles the boxes it finds through [member by_class]
## rather than asking the shell for a render per frame.
##
## Kept on this side, because the web gets them from the browser:
## - [code]<details>[/code] opening and closing. Which ones are open is
##   remembered here by summary text, so a rebuild does not shut the codex
##   section the player just opened.
## - Links. [code]a[href][/code] opens in the system browser.
## - Tooltips, from [code]data-tip[/code], and from [code]aria-label[/code]
##   on a control with nothing else to read.

## Every element box, by each of its classes, in document order.
var by_class: Dictionary[String, Array] = {}
## [code]<details>[/code] the player toggled, by summary text.
var open_details: Dictionary[String, bool] = {}
## Called after a local toggle, to draw the last trees again.
var redraw := Callable()

var _icons: Dictionary[String, Texture2D] = {}


func _init(style: Style) -> void:
	Box.style = style


## [param tree]'s root as a box, or null if the stylesheet hides it.
func build(tree: Dictionary) -> Box:
	var root := _element(tree, null, 0, 1)
	_reserve_band(root)
	return root


func clear() -> void:
	by_class.clear()


## [code].round-screen:not(:has(.hud .boss)) .grid-wrap[/code], the one rule
## in the web sheet that looks down the tree, which [Style]'s selectors cannot:
## a round without a boss keeps the boss band's height as room above the board,
## so every round's tiles are the same size in the same place, and the first
## round's coaching card, which hangs from the header's foot, lies over that
## room rather than over the row being typed. The test is made here and
## answered as a class of this build's own, [code]band-room[/code], so the
## room itself is a rule like any other.
func _reserve_band(root: Box) -> void:
	if root == null or not root.has_class("round-screen") or not find_all(["boss"]).is_empty():
		return
	var board := find(["grid-wrap"])
	if board != null:
		board.set_class("band-room", true)


## The first box with every class in [param classes], or null.
func find(classes: PackedStringArray, attr := "", value := "") -> Box:
	var all := find_all(classes, attr, value)
	return all[0] if not all.is_empty() else null


func find_all(classes: PackedStringArray, attr := "", value := "") -> Array[Box]:
	var out: Array[Box] = []
	for box: Box in by_class.get(classes[0], []):
		if not is_instance_valid(box):
			continue
		var ok := true
		for cls: String in classes:
			ok = ok and box.has_class(cls)
		if attr != "":
			var attrs: Dictionary = box.node.get("a", {})
			ok = ok and str(attrs.get(attr, "")) == value
		if ok:
			out.append(box)
	return out


func _element(node: Dictionary, parent: Box, index: int, count: int) -> Box:
	var box := Box.new()
	box.parent_box = parent
	box.node = node
	box.entry = Style.entry(node, index, count)
	box.apply()
	if box.own.get("hide", false):
		box.free()
		return null
	var tag: String = node.get("t", "div")
	var attrs: Dictionary = node.get("a", {})
	for cls: String in box.entry["cls"]:
		if not by_class.has(cls):
			by_class[cls] = []
		by_class[cls].append(box)

	if tag == "svg":
		box.mode = "icon"
		box.texture = _icon(node, box)
		return _finish(box, node)

	var kids: Array = node.get("k", [])
	if tag == "details":
		kids = _details(node, kids)

	var d: String = box.own.get("d", "")
	if parent != null and parent.mode == "board":
		d = "tiles"
	elif parent != null and parent.mode == "tiles":
		d = "tilerow"
	if d in ["meter", "bar", "switch"]:
		box.mode = d
		_gauge(box, kids)
		return _finish(box, node)

	# What each element child would be, so this box can tell text from layout
	# before building anything: absolutely placed children do not count.
	var chain := box.chain()
	var flow_kids: Array = []
	var abs_kids: Array = []
	var inline := true
	var any_text := false
	var shown := 0
	for kid: Variant in kids:
		if kid is String:
			any_text = any_text or str(kid).strip_edges() != ""
			flow_kids.append(kid)
			continue
		var element: Dictionary = kid
		var probe: Array = chain.duplicate()
		probe.append(Style.entry(element, shown, kids.size()))
		var props := Box.style.match(probe)
		if props.get("hide", false):
			continue
		shown += 1
		if props.has("abs"):
			abs_kids.append(element)
			continue
		flow_kids.append(element)
		var ktag: String = element.get("t", "div")
		inline = inline and ktag in Box.TEXT_TAGS and not props.has("d") and _only_text(element)

	# A box that is itself a flex container makes its text and spans flex items,
	# each its own box with the gap between, not runs of one paragraph: a
	# settings value ("promises" and its chevron, a flag and its language's name)
	# lost its gap to exactly that.
	inline = inline and d == ""
	if d == "":
		d = "row" if tag == "button" else "col"
	if flow_kids.is_empty():
		box.mode = "empty"
	elif (
		flow_kids.all(func(k: Variant) -> bool: return k is String)
		and d not in ["board", "tiles", "tilerow"]
	):
		box.mode = "text"
		box.set_text("".join(flow_kids).strip_edges())
	elif any_text and inline:
		box.mode = "rich"
		_rich(box, flow_kids)
	else:
		box.mode = d
	if box.own.get("scroll", false) and box.mode not in ["text", "rich", "empty"]:
		_scroller(box)

	if box.mode not in ["text", "rich", "empty"]:
		var holder := box.inner if box.inner != null else box
		var i := 0
		for kid: Variant in flow_kids:
			var child: Control = null
			if kid is String:
				if str(kid).strip_edges() == "":
					continue
				child = _text(str(kid).strip_edges(), holder)
			else:
				var element: Dictionary = kid
				child = _element(element, holder, i, kids.size())
			i += 1
			if child != null:
				holder.flow.append(child)
				holder.add_child(child)
	for kid: Dictionary in abs_kids:
		var child := _element(kid, box, 0, 1)
		if child != null:
			box.abs_kids.append(child)
			box.add_child(child)
	if tag == "a" and attrs.has("href") and not node.has("on"):
		box.href = str(attrs["href"])
		box.make_clickable()
	return _finish(box, node)


## Tip, handler, disabled state: what every element gets whatever it holds.
func _finish(box: Box, node: Dictionary) -> Box:
	var attrs: Dictionary = node.get("a", {})
	var tip: String = attrs.get("data-tip", "")
	if tip == "" and box.mode in ["icon", "empty"] or tip == "" and _no_words(node):
		tip = attrs.get("aria-label", "")
	if tip != "":
		box.tooltip_text = tip
		box.mouse_filter = Control.MOUSE_FILTER_PASS
	if node.has("local"):
		var label: String = node["local"]
		var open: bool = node["open"]
		box.local_click = func() -> void:
			open_details[label] = not open
			redraw.call()
		box.make_clickable()
	elif node.has("on") and not attrs.has("disabled"):
		box.click_id = type_convert(node["on"], TYPE_INT)
		box.make_clickable()
	return box


## Text between elements, which the web's inline layout would run on with them.
func _text(value: String, parent: Box) -> Box:
	var box := Box.new()
	box.parent_box = parent
	box.mode = "text"
	box.apply()
	box.set_text(value)
	return box


## A scroller keeps the element's box and padding and puts its children in an
## inner box, which a [ScrollContainer] moves.
func _scroller(box: Box) -> void:
	var layout := box.mode
	box.mode = "scroll"
	box.scroll = ScrollContainer.new()
	box.scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	box.scroll.mouse_filter = Control.MOUSE_FILTER_PASS
	box.add_child(box.scroll)
	var inner := Box.new()
	inner.wrapper = true
	inner.parent_box = box
	inner.node = box.node
	inner.mode = layout
	for key: String in ["d", "gap", "rgap", "a", "j", "cols", "colw"]:
		if box.own.has(key):
			inner.own[key] = box.own[key]
	# A gutter before the bar, as a desktop browser leaves one: the container
	# only takes the bar's own width off, and text ran up against it.
	inner.own["p"] = [0, 6, 0, 0]
	inner.apply()
	inner.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	box.inner = inner
	box.scroll.add_child(inner)


func _details(node: Dictionary, kids: Array) -> Array:
	var summary: Dictionary = {}
	for kid: Variant in kids:
		if _is_summary(kid):
			summary = kid
	if summary.is_empty():
		return kids
	var label := _plain(summary)
	var attrs: Dictionary = node.get("a", {})
	var open: bool = open_details.get(label, attrs.has("open"))
	# The web draws the triangle with ::before; here it is text, run into the
	# summary's leading text, however deep that is (the codex wraps its title in
	# a span). As a node of its own it was a third flex item, and a codex row's
	# space-between put the section's name in the middle of the row instead of
	# beside its marker.
	var marked := _lead(summary, "▾ " if open else "▸ ")
	marked["local"] = label
	marked["open"] = open
	var out: Array = []
	for kid: Variant in kids:
		if _is_summary(kid):
			out.append(marked)
		elif open:
			out.append(kid)
	return out


## [param node] with [param text] run into its first leading text, descending
## through leading elements; prepended as a node of its own where there is none.
func _lead(node: Dictionary, text: String) -> Dictionary:
	var out := node.duplicate()
	var kids: Array = node.get("k", [])
	var rest := kids.duplicate()
	if not rest.is_empty() and rest[0] is String:
		rest[0] = text + str(rest[0]).strip_edges(true, false)
	elif not rest.is_empty() and rest[0] is Dictionary:
		var first: Dictionary = rest[0]
		rest[0] = _lead(first, text)
	else:
		rest.push_front(text)
	out["k"] = rest
	return out


## Meters and switches draw themselves; their children only carry the numbers.
func _gauge(box: Box, kids: Array) -> void:
	for kid: Variant in kids:
		if kid is not Dictionary:
			continue
		var element: Dictionary = kid
		var vars: Dictionary = element.get("s", {})
		var cls: String = element.get("c", "")
		var value := _fraction(str(vars.get("--fill", "0")))
		if cls.contains("meter-solve"):
			box.fill_solve = value
		else:
			box.fill = value
	if box.mode == "meter":
		var score := find(["hud-score"])
		box.met = score != null and score.has_class("met")
	if box.mode == "switch":
		var at := box.parent_box
		while at != null:
			var attrs: Dictionary = at.node.get("a", {})
			if attrs.has("aria-checked"):
				box.checked = str(attrs["aria-checked"]) == "true"
				break
			at = at.parent_box


static func _fraction(value: String) -> float:
	if value.ends_with("%"):
		return value.trim_suffix("%").to_float() / 100.0
	return value.to_float()


func _rich(box: Box, kids: Array) -> void:
	box.spans = []
	_spans(kids, box.chain(), box.inh, box.spans)
	box.rich = RichTextLabel.new()
	box.rich.bbcode_enabled = false
	box.rich.scroll_active = false
	box.rich.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.rich.selection_enabled = false
	box.rich.mouse_filter = Control.MOUSE_FILTER_IGNORE
	for span: Dictionary in box.spans:
		if str(span.get("href", "")) != "":
			box.rich.mouse_filter = Control.MOUSE_FILTER_PASS
			box.rich.meta_underlined = true
			box.rich.meta_clicked.connect(func(meta: Variant) -> void: OS.shell_open(str(meta)))
			break
	box.add_child(box.rich)
	box._fill_rich()


func _spans(kids: Array, chain: Array, inh: Dictionary, out: Array[Dictionary]) -> void:
	for i: int in kids.size():
		var kid: Variant = kids[i]
		if kid is String:
			var up: bool = inh.get("up", false)
			(
				out
				. append(
					{
						"text": str(kid).to_upper() if up else str(kid),
						"w": inh["w"],
						"f": inh["f"],
						"c": inh["c"],
						"strike": inh.get("strike", false),
						"href": inh.get("href", ""),
					}
				)
			)
			continue
		var element: Dictionary = kid
		var probe: Array = chain.duplicate()
		probe.append(Style.entry(element, i, kids.size()))
		var props := Box.style.match(probe)
		if props.get("hide", false):
			continue
		var next := inh.duplicate()
		var vars: Dictionary = inh["vars"]
		var more: Dictionary = props.get("vars", {})
		vars = vars.duplicate()
		vars.merge(more, true)
		next["vars"] = vars
		for key: String in Style.INHERITED:
			if props.has(key):
				next[key] = Box.style.color(props[key], vars) if key == "c" else props[key]
		var attrs: Dictionary = element.get("a", {})
		if element.get("t") == "a" and attrs.has("href"):
			next["href"] = str(attrs["href"])
		var inner: Array = element.get("k", [])
		_spans(inner, probe, next, out)


static func _is_summary(kid: Variant) -> bool:
	if kid is not Dictionary:
		return false
	var element: Dictionary = kid
	return element.get("t") == "summary"


static func _only_text(element: Dictionary) -> bool:
	for kid: Variant in element.get("k", []):
		if kid is Dictionary:
			var inner: Dictionary = kid
			if inner.get("t") not in Box.TEXT_TAGS or not _only_text(inner):
				return false
	return true


static func _no_words(node: Dictionary) -> bool:
	return _plain(node).strip_edges() == ""


static func _plain(node: Dictionary) -> String:
	var out := ""
	for kid: Variant in node.get("k", []):
		if kid is Dictionary:
			var element: Dictionary = kid
			out += _plain(element)
		else:
			out += str(kid)
	return out


## An SVG as a texture at twice its drawn size, so it stays crisp under the
## window's scale. Line icons are drawn by the stylesheet on the web
## ([code].icon { stroke: currentColor; fill: none }[/code]), so those
## attributes go into the markup here, in the colour in force.
func _icon(node: Dictionary, box: Box) -> Texture2D:
	var markup: String = node.get("svg", "")
	var px: float = box.own.get("W", 18.0)
	var color: Color = box.inh["c"]
	var line := box.has_class("icon")
	var filled: bool = box.own.get("fill", false)
	var key := "%s|%d|%s|%s" % [markup.md5_text(), px, color.to_html(), filled]
	if _icons.has(key):
		return _icons[key]
	var hex := "#" + color.to_html(false)
	markup = markup.replace("currentColor", hex)
	if line:
		var paint := (
			'fill="%s" stroke="none"' % hex
			if filled
			else (
				'fill="none" stroke="%s" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"'
				% hex
			)
		)
		markup = markup.replace("<svg ", "<svg %s " % paint)
	var view := RegEx.create_from_string('viewBox="[\\d.-]+ [\\d.-]+ ([\\d.]+) ([\\d.]+)"').search(
		markup
	)
	var vw := view.get_string(1).to_float() if view != null else 24.0
	if not markup.contains(" width="):
		var vh := view.get_string(2).to_float() if view != null else vw
		markup = markup.replace("<svg ", '<svg width="%s" height="%s" ' % [vw, vh])
	var image := Image.new()
	if image.load_svg_from_string(markup, px * 2.0 / vw) != OK:
		return null
	var texture := ImageTexture.create_from_image(image)
	_icons[key] = texture
	return texture
