class_name Box
extends Control
## One element of a view's tree, drawn and laid out the way the web stylesheet
## would: its own background, border and padding, and its children in a column,
## a row, a grid, a row of keys or the board.
##
## Layout is this class's own rather than Godot's containers, for one reason:
## CSS sizes a box from its content in two passes, widths down the tree and then
## heights back up at those widths, and a wrapped paragraph's height is only
## known once its width is. Godot's containers ask each child for one minimum
## size that already has to know its height, so a wrapping [Label] reports the
## height it had at whatever width it was last given, the container sizes it,
## the label changes its mind, and the screen settles a frame or two later.
## Every render here is a whole screen built from scratch (see
## [code]game/render.gd[/code]), so that would be a visible settle on every
## keystroke. Here a parent asks [method min_w], [method nat_w] and
## [method h_for] of each child and places it once; a child lays out its own
## children when its size is set.
##
## Text is shaped with [TextParagraph] and drawn by the box itself, so a
## measurement and a drawing can never disagree about where a line breaks.
## Mixed runs (a sentence with a bold word in another colour) are the one
## thing a paragraph cannot colour, and those go to a [RichTextLabel], measured
## with a paragraph of the same spans in the same fonts.

## Inherited props at the root of every tree: body's font and colour.
const FONT_PX := 16.0
const FONT_NAMES: PackedStringArray = [
	"Inter", "Noto Sans", "Cantarell", "DejaVu Sans", "sans-serif"
]
const TEXT_TAGS := ["span", "strong", "b", "em", "a", "small", "code", "label", "i"]

## What the builder hands every box: the rule table and the theme in force.
static var style: Style
## The page's height in CSS pixels, for [code]maxh[/code].
static var page_h := 800.0
## Called with a node's handler id when it is clicked.
static var on_click := Callable()

static var _fonts: Dictionary[int, Font] = {}

## The tree element this box draws; empty for text between elements and for
## the inner box of a scroller.
var node: Dictionary = {}
## As [method Style.entry], for matching; empty on an anonymous box.
var entry: Dictionary = {}
var parent_box: Box = null
## True on the inner box of a scroller: it lays out, but matches nothing.
var wrapper := false
## The props matched on this element, and the inherited ones in force here.
var own: Dictionary = {}
var inh: Dictionary = {}
## col row grid keys board tiles tilerow meter bar switch text rich icon empty
var mode := "col"
var text := ""
var flow: Array[Control] = []
var abs_kids: Array[Control] = []
## The board sets a tile's type size from its tile size, as the web's
## container-query font does.
var font_px := 0.0
var fill := 0.0
var fill_solve := 0.0
var met := false
var checked := false
var texture: Texture2D = null
## A link to open, for [code]a[href][/code] outside running text.
var href := ""
var click_id := -1
## A click handled on this side (a [code]<details>[/code] summary).
var local_click := Callable()
## Rich runs: [code]{text, w, f, c, strike, href}[/code].
var spans: Array[Dictionary] = []

var scroll: ScrollContainer = null
var inner: Box = null
var rich: RichTextLabel = null

var _box: StyleBoxFlat = null
var _outline: StyleBoxFlat = null
## A square box's borders, drawn by [method _hairlines] rather than the style.
var _edges: Array[float] = []
var _outline_off := 0.0
var _para: TextParagraph = null
var _color := Color.WHITE
var _pad: Array[float] = [0.0, 0.0, 0.0, 0.0]
var _h_cache: Dictionary[String, float] = {}


## The page's font at [param weight], tracked out by [param spacing] whole
## pixels. Letter spacing is a [FontVariation] over the one system font rather
## than a property of the paragraph, which has none; the web's is in em, so
## [method _tracking] converts it at the box's own size.
static func font(weight: int, spacing := 0) -> Font:
	var w := clampi(weight, 100, 900)
	var key := w * 1000 + clampi(spacing, -99, 99) + 100
	if not _fonts.has(key):
		if spacing != 0:
			var variation := FontVariation.new()
			variation.base_font = font(w)
			variation.spacing_glyph = spacing
			_fonts[key] = variation
			return variation
		var sys := SystemFont.new()
		sys.font_names = FONT_NAMES
		sys.font_weight = w
		sys.antialiasing = TextServer.FONT_ANTIALIASING_GRAY
		sys.hinting = TextServer.HINTING_LIGHT
		sys.subpixel_positioning = TextServer.SUBPIXEL_POSITIONING_AUTO
		_fonts[key] = sys
	return _fonts[key]


func _init() -> void:
	mouse_filter = MOUSE_FILTER_IGNORE
	clip_contents = false


## Matches this box against the table and works out what it inherits. Called
## once by the builder and again by [method restyle].
func apply() -> void:
	if not entry.is_empty():
		own = style.match(chain())
	var up: Dictionary = parent_box.inh if parent_box != null else _root_inh()
	inh = up.duplicate()
	var vars: Dictionary = up["vars"]
	vars = vars.duplicate()
	var own_vars: Dictionary = own.get("vars", {})
	vars.merge(own_vars, true)
	inh["vars"] = vars
	for key: String in Style.INHERITED:
		if own.has(key):
			inh[key] = style.color(own[key], vars) if key == "c" else own[key]
	_color = inh["c"]
	_pad = _padding()
	_box = _stylebox()
	_outline = null
	var ol: Variant = own.get("ol")
	if ol is Array:
		var spec: Array = ol
		_outline = StyleBoxFlat.new()
		_outline.draw_center = false
		_outline.border_color = style.color(spec[0], vars)
		_outline.set_border_width_all(whole(spec[1]))
		_outline.set_corner_radius_all(whole(_radius()[0]) + whole(spec[2]))
		_outline_off = num(spec[2]) + num(spec[1])
	var alpha: float = own.get("op", 1.0)
	modulate = Color(1, 1, 1, alpha)
	var sc: float = own.get("sc", 1.0)
	scale = Vector2(sc, sc)
	_shape()
	_h_cache.clear()
	queue_redraw()


func _root_inh() -> Dictionary:
	return {
		"f": FONT_PX,
		"w": 400,
		"c": style.color("fg"),
		"up": false,
		"ls": 0.0,
		"ta": "l",
		"strike": false,
		"vars": {},
	}


## Ancestors' entries, outermost first, then this box's own.
func chain() -> Array:
	var out: Array = []
	var at: Box = self
	while at != null:
		if not at.entry.is_empty() and not at.wrapper:
			out.push_front(at.entry)
		at = at.parent_box
	return out


func has_class(cls_name: String) -> bool:
	if entry.is_empty():
		return false
	var cls: PackedStringArray = entry["cls"]
	return cls.has(cls_name)


## Adds or removes a class and restyles this subtree, as the web build's
## animation toggles [code].fired[/code] or [code].solved[/code] on a node.
func set_class(cls_name: String, on: bool) -> void:
	if entry.is_empty() or has_class(cls_name) == on:
		return
	var cls: PackedStringArray = entry["cls"]
	if on:
		cls.append(cls_name)
	else:
		cls.remove_at(cls.find(cls_name))
	entry["cls"] = cls
	restyle()


func restyle() -> void:
	apply()
	for child: Control in flow + abs_kids:
		if child is Box:
			(child as Box).restyle()
	if inner != null:
		inner.inh = inh
		inner.restyle()
	if rich != null:
		_fill_rich()
	_arrange()


func set_text(value: String) -> void:
	text = value
	_shape()
	_h_cache.clear()
	queue_redraw()


# --------------------------------------------------------------- measuring


## The narrowest this box can be: its longest word, or its widest child.
func min_w() -> float:
	if own.has("W"):
		return num(own["W"])
	var m := 0.0
	match mode:
		"text":
			m = 0.0 if own.get("clip", false) else (_line_w() if _nowrap() else _word_w())
		"rich":
			m = _rich_word_w()
		"icon":
			m = _icon_px().x
		"col":
			for child: Control in flow:
				m = maxf(m, Box.min_of(child))
		"row", "keys":
			for child: Control in flow:
				m += Box.min_of(child)
			m += _gap() * maxf(flow.size() - 1, 0)
		"grid":
			m = _sum(Flex.grid_cols(flow, own, INF, true)) + _gap() * (Flex.cols(own) - 1)
		"scroll":
			m = inner.min_w()
		"board", "tiles", "tilerow":
			m = 120.0
	m += _pad[1] + _pad[3]
	return maxf(m, num(own.get("minw", 0.0)))


## The width this box would take with room to spare: its text on one line, or
## its children at theirs.
func nat_w() -> float:
	if own.has("W"):
		return num(own["W"])
	var m := 0.0
	match mode:
		"text":
			m = _line_w()
		"rich":
			m = _rich_line_w()
		"icon":
			m = _icon_px().x
		"col":
			for child: Control in flow:
				m = maxf(m, Box.nat_of(child))
		"row", "keys":
			for child: Control in flow:
				m += Box.nat_of(child)
			m += _gap() * maxf(flow.size() - 1, 0)
		"grid":
			m = _sum(Flex.grid_cols(flow, own, INF, false)) + _gap() * (Flex.cols(own) - 1)
		"scroll":
			m = inner.nat_w()
		"board", "tiles", "tilerow":
			m = 360.0
	m += _pad[1] + _pad[3]
	if own.has("maxw"):
		m = minf(m, num(own["maxw"]))
	return maxf(m, min_w())


## This box's height at width [param w]. [param natural] asks for a scroller's
## whole content rather than the nothing it can shrink to.
func h_for(w: float, natural: bool) -> float:
	if own.has("H"):
		return num(own["H"])
	var key := "%d:%s" % [roundi(w * 4.0), natural]
	if _h_cache.has(key):
		return _h_cache[key]
	var iw := maxf(w - _pad[1] - _pad[3], 0.0)
	var h := 0.0
	match mode:
		"text":
			h = _text_h(iw)
		"rich":
			h = _rich_h(iw)
		"icon":
			h = _icon_px().y
		"col":
			for child: Control in flow:
				var cw := _col_child_w(child, iw)
				h += Box.h_of(child, cw, natural) + _mt(child)
			h += _gap() * maxf(flow.size() - 1, 0)
		"row", "keys":
			var widths := Flex.row_widths(flow, own, iw)
			for i: int in flow.size():
				h = maxf(h, Box.h_of(flow[i], widths[i], natural) + _mt(flow[i]))
		"grid":
			var grid := Flex.grid(flow, own, iw, natural)
			h = grid["h"]
		"scroll":
			h = inner.h_for(iw, true) if natural else 0.0
		"board", "tiles", "tilerow":
			h = num(own.get("minh", 160.0))
		"switch", "meter", "bar":
			h = 0.0
	h += _pad[0] + _pad[2]
	h = maxf(h, num(own.get("minh", 0.0)))
	_h_cache[key] = h
	return h


static func min_of(c: Control) -> float:
	return (c as Box).min_w() if c is Box else c.custom_minimum_size.x


static func nat_of(c: Control) -> float:
	return (c as Box).nat_w() if c is Box else c.custom_minimum_size.x


static func h_of(c: Control, w: float, natural: bool) -> float:
	return (c as Box).h_for(w, natural) if c is Box else c.custom_minimum_size.y


## A number out of a style table or a tree, which reach here as Variants and
## are floats whenever they came through JSON.
static func num(value: Variant) -> float:
	return type_convert(value, TYPE_FLOAT)


static func whole(value: Variant) -> int:
	return type_convert(value, TYPE_INT)


static func css(c: Control) -> Dictionary:
	return (c as Box).own if c is Box else {}


# ----------------------------------------------------------------- layout


func _notification(what: int) -> void:
	if what == NOTIFICATION_RESIZED:
		# `transform-origin: center`, for the pops the animation scales by.
		pivot_offset = size / 2.0
		_arrange()


func _arrange() -> void:
	var inner_rect := Rect2(
		_pad[3],
		_pad[0],
		maxf(size.x - _pad[1] - _pad[3], 0.0),
		maxf(size.y - _pad[0] - _pad[2], 0.0)
	)
	match mode:
		"col":
			_arrange_col(inner_rect)
		"row", "keys":
			_arrange_row(inner_rect)
		"grid":
			_arrange_grid(inner_rect)
		"scroll":
			scroll.position = inner_rect.position
			scroll.size = inner_rect.size
			# Room for the bar, so the content is measured at the width it gets.
			var h := inner.h_for(maxf(inner_rect.size.x - 12.0, 0.0), true)
			inner.custom_minimum_size = Vector2(0, h)
		"board":
			_arrange_board(inner_rect)
		"tiles":
			_arrange_tiles(inner_rect)
		"tilerow":
			_arrange_tilerow(inner_rect)
		"rich":
			var h := _rich_h(inner_rect.size.x)
			var y := inner_rect.position.y
			if _center_v():
				y += (inner_rect.size.y - h) / 2.0
			rich.position = Vector2(inner_rect.position.x, y)
			rich.size = Vector2(inner_rect.size.x, h + 2.0)
	for child: Control in abs_kids:
		_place_abs(child)
	pivot_offset = size / 2.0


func _arrange_col(r: Rect2) -> void:
	var n := flow.size()
	var widths: Array[float] = []
	var heights: Array[float] = []
	var mins: Array[float] = []
	var total := _gap() * maxf(n - 1, 0)
	for child: Control in flow:
		var cw := _col_child_w(child, r.size.x)
		var hn := Box.h_of(child, cw, true)
		var hm := Box.h_of(child, cw, false)
		var cap: float = css(child).get("maxh", 0.0)
		if cap > 0.0:
			hn = maxf(minf(hn, cap * page_h), hm)
		widths.append(cw)
		heights.append(hn)
		mins.append(hm)
		total += hn + _mt(child)
	var free := r.size.y - total
	var offset := 0.0
	var between := 0.0
	var pushed := -1
	if free < 0.0:
		# Too tall: take it out of whatever can scroll, in proportion.
		var slack := 0.0
		for i: int in n:
			slack += heights[i] - mins[i]
		if slack > 0.0:
			var share := minf(-free / slack, 1.0)
			for i: int in n:
				heights[i] -= (heights[i] - mins[i]) * share
	elif free > 0.0:
		var grow := 0.0
		for child: Control in flow:
			grow += num(css(child).get("grow", 0.0))
		for i: int in n:
			if css(flow[i]).get("push", false):
				pushed = i
				break
		if grow > 0.0:
			for i: int in n:
				heights[i] += free * num(css(flow[i]).get("grow", 0.0)) / grow
		elif pushed < 0:
			match own.get("j", "start"):
				"center":
					offset = free / 2.0
				"end":
					offset = free
				"between":
					between = free / maxf(n - 1, 1)
	var y := r.position.y + offset
	for i: int in n:
		var child := flow[i]
		y += _mt(child)
		if i == pushed:
			y += maxf(free, 0.0)
		var x := r.position.x + _align_x(widths[i], r.size.x)
		child.position = Vector2(x, y)
		child.size = Vector2(widths[i], heights[i])
		y += heights[i] + _gap() + between


func _arrange_row(r: Rect2) -> void:
	var n := flow.size()
	var widths := Flex.row_widths(flow, own, r.size.x)
	var used := _sum(widths) + _gap() * maxf(n - 1, 0)
	var free := r.size.x - used
	var x := r.position.x
	var between := 0.0
	var any_grow := false
	for child: Control in flow:
		any_grow = any_grow or num(css(child).get("grow", 0.0)) > 0.0
	if free > 0.0 and not any_grow:
		match own.get("j", "start"):
			"center":
				x += free / 2.0
			"end":
				x += free
			"between":
				between = free / maxf(n - 1, 1)
	var align: String = own.get("a", "stretch")
	for i: int in n:
		var child := flow[i]
		var h := r.size.y
		var y := r.position.y
		# A fixed height or an icon keeps its size under stretch, and sits in
		# the middle rather than at the top, which is what every row that
		# holds one in the stylesheet asks for anyway.
		if align != "stretch" or child is not Box or css(child).has("H"):
			h = minf(Box.h_of(child, widths[i], true), r.size.y)
			match align:
				"start":
					pass
				"end":
					y += r.size.y - h
				_:
					y += (r.size.y - h) / 2.0
		child.position = Vector2(x, y)
		child.size = Vector2(widths[i], h)
		x += widths[i] + _gap() + between


func _arrange_grid(r: Rect2) -> void:
	var grid := Flex.grid(flow, own, r.size.x, true)
	var cells: Array = grid["cells"]
	for cell: Array in cells:
		var child: Control = cell[0]
		var rect: Rect2 = cell[1]
		child.position = r.position + rect.position
		child.size = rect.size


## The board: square tiles as large as fit, up to [constant Style.TILE_MAX],
## centred across and hung from the top, as [code].grid[/code]'s
## [code]min(100%, 100cqh * cols/rows)[/code] and [code]margin: 0 auto auto[/code] do.
func _arrange_board(r: Rect2) -> void:
	if flow.is_empty():
		return
	var grid := flow[0] as Box
	var dims := grid.board_dims()
	var cols := dims.x
	var rows := dims.y
	var gap := 5.0
	var s := minf((r.size.x - (cols - 1) * gap) / cols, (r.size.y - (rows - 1) * gap) / rows)
	s = floorf(clampf(s, 12.0, Style.TILE_MAX))
	var gw := cols * s + (cols - 1) * gap
	var gh := rows * s + (rows - 1) * gap
	grid.font_px = s
	grid.position = Vector2(r.position.x + (r.size.x - gw) / 2.0, r.position.y)
	grid.size = Vector2(gw, gh)
	grid._arrange()


func board_dims() -> Vector2:
	var vars: Dictionary = node.get("s", {})
	return Vector2(num(vars.get("--cols", 5)), num(vars.get("--rows", 6)))


func _arrange_tiles(_r: Rect2) -> void:
	var s := font_px
	for i: int in flow.size():
		var row := flow[i] as Box
		if row == null:
			continue
		row.font_px = s
		row.position = Vector2(0, i * (s + 5.0))
		row.size = Vector2(size.x, s)
		row._arrange()


func _arrange_tilerow(_r: Rect2) -> void:
	var s := font_px
	for i: int in flow.size():
		var tile := flow[i] as Box
		if tile == null:
			continue
		tile.font_px = roundf(s * 0.45)
		tile._shape()
		tile.position = Vector2(i * (s + 5.0), 0)
		tile.size = Vector2(s, s)
		tile.queue_redraw()


func _place_abs(child: Control) -> void:
	var spec: Array = css(child).get("abs", ["tr", 0, 0])
	var anchor: String = spec[0]
	var dx: float = spec[1]
	var dy: float = spec[2]
	var w := size.x if anchor == "below" else Box.nat_of(child)
	var h := Box.h_of(child, w, true)
	var at := Vector2.ZERO
	match anchor:
		"tr":
			at = Vector2(size.x - w + dx, dy)
		"bl":
			at = Vector2(dx, size.y - h + dy)
		"br":
			at = Vector2(size.x - w + dx, size.y - h + dy)
		"below":
			at = Vector2(dx, size.y + dy)
	child.position = at
	child.size = Vector2(w, h)
	child.z_index = 5 if anchor == "below" else 2


func _col_child_w(child: Control, iw: float) -> float:
	var c := css(child)
	if c.has("W"):
		return num(c["W"])
	var w := iw
	var align: String = own.get("a", "stretch")
	if not c.get("fillw", false) and align != "stretch":
		w = minf(Box.nat_of(child), iw)
	if c.has("maxw"):
		w = minf(w, num(c["maxw"]))
	if child is not Box:
		w = child.custom_minimum_size.x
	return w


func _align_x(w: float, iw: float) -> float:
	var free := iw - w
	if free <= 0.0:
		return 0.0
	var align: String = own.get("a", "stretch")
	if align == "start":
		return 0.0
	if align == "end":
		return free
	# stretch with a cap is `margin: 0 auto`, which centres; so does center.
	return free / 2.0


func _gap() -> float:
	return Flex.gap_of(own)


func _mt(child: Control) -> float:
	return num(css(child).get("mt", 0.0))


static func _sum(values: Array[float]) -> float:
	var total := 0.0
	for v: float in values:
		total += v
	return total


# ------------------------------------------------------------------- text


func _px() -> float:
	return font_px if font_px > 0.0 else num(inh.get("f", FONT_PX))


func _weight() -> int:
	return whole(inh.get("w", 400))


func _nowrap() -> bool:
	return own.get("nowrap", false) or own.get("clip", false)


## [code]letter-spacing[/code], from em to whole pixels at this box's size.
## Rounded because a [FontVariation] spaces glyphs by integers: at the 11 and
## 12px the tracked labels are set in, 0.14em is 1.5 to 1.7px and comes out 2.
func _tracking() -> int:
	return roundi(num(inh.get("ls", 0.0)) * _px())


func _shown_text() -> String:
	return text.to_upper() if inh.get("up", false) else text


func _shape() -> void:
	if mode != "text":
		return
	_para = TextParagraph.new()
	_para.add_string(_shown_text(), Box.font(_weight(), _tracking()), whole(roundf(_px())))
	_para.break_flags = (
		TextServer.BREAK_MANDATORY
		if _nowrap()
		else TextServer.BREAK_MANDATORY | TextServer.BREAK_WORD_BOUND | TextServer.BREAK_ADAPTIVE
	)
	if own.get("clip", false):
		_para.max_lines_visible = 1
		_para.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_para.alignment = _halign()


func _halign() -> HorizontalAlignment:
	var ta: String = inh.get("ta", "l")
	if own.has("j") and _is_row_like():
		ta = {"center": "c", "end": "r"}.get(own["j"], ta)
	if own.get("a") == "center" and not _is_row_like():
		ta = "c"
	return (
		HORIZONTAL_ALIGNMENT_CENTER
		if ta == "c"
		else HORIZONTAL_ALIGNMENT_RIGHT if ta == "r" else HORIZONTAL_ALIGNMENT_LEFT
	)


func _is_row_like() -> bool:
	var d: String = own.get("d", "button" if node.get("t") == "button" else "")
	return d in ["row", "button", "keys"]


func _center_v() -> bool:
	if _is_row_like():
		return (
			own.get("a", "center" if node.get("t") == "button" else "stretch")
			in ["center", "stretch"]
		)
	return own.get("j", "start") == "center" or own.has("H") or node.get("t") == "button"


func _word_w() -> float:
	var f := Box.font(_weight(), _tracking())
	var px := whole(roundf(_px()))
	var widest := 0.0
	for word: String in _shown_text().split(" ", false):
		widest = maxf(widest, f.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1, px).x)
	return ceilf(widest) + 1.0


func _line_w() -> float:
	if _para == null:
		return 0.0
	_para.width = -1
	return ceilf(_para.get_size().x) + 1.0


func _text_h(iw: float) -> float:
	if _para == null or text == "":
		return 0.0
	_para.width = iw
	return ceilf(_para.get_size().y)


func _rich_para(iw: float) -> TextParagraph:
	var para := TextParagraph.new()
	for span: Dictionary in spans:
		var t: String = span["text"]
		para.add_string(t, Box.font(whole(span["w"])), whole(roundf(num(span["f"]))))
	para.break_flags = (
		TextServer.BREAK_MANDATORY | TextServer.BREAK_WORD_BOUND | TextServer.BREAK_ADAPTIVE
	)
	para.width = iw
	return para


func _rich_word_w() -> float:
	var widest := 0.0
	for span: Dictionary in spans:
		var f := Box.font(whole(span["w"]))
		for word: String in str(span["text"]).split(" ", false):
			widest = maxf(
				widest, f.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1, whole(span["f"])).x
			)
	return ceilf(widest) + 1.0


func _rich_line_w() -> float:
	return ceilf(_rich_para(-1).get_size().x) + 2.0


func _rich_h(iw: float) -> float:
	return ceilf(_rich_para(iw).get_size().y)


## The label that draws the rich runs, from [member spans].
func _fill_rich() -> void:
	rich.clear()
	rich.add_theme_font_override("normal_font", Box.font(_weight()))
	rich.add_theme_font_size_override("normal_font_size", whole(roundf(_px())))
	rich.add_theme_color_override("default_color", _color)
	var h := _halign()
	if h == HORIZONTAL_ALIGNMENT_CENTER:
		rich.push_paragraph(HORIZONTAL_ALIGNMENT_CENTER)
	elif h == HORIZONTAL_ALIGNMENT_RIGHT:
		rich.push_paragraph(HORIZONTAL_ALIGNMENT_RIGHT)
	for span: Dictionary in spans:
		rich.push_font(Box.font(whole(span["w"])), whole(roundf(num(span["f"]))))
		var tint: Color = span["c"]
		rich.push_color(tint)
		if span.get("strike", false):
			rich.push_strikethrough()
		var link: String = span.get("href", "")
		if link != "":
			rich.push_meta(link)
			rich.push_underline()
		rich.add_text(str(span["text"]))
		if link != "":
			rich.pop()
			rich.pop()
		if span.get("strike", false):
			rich.pop()
		rich.pop()
		rich.pop()


# ---------------------------------------------------------------- drawing


func _padding() -> Array[float]:
	var out: Array[float] = [0.0, 0.0, 0.0, 0.0]
	var p: Variant = own.get("p", 0)
	if p is Array:
		var list: Array = p
		if list.size() == 2:
			out = [num(list[0]), num(list[1]), num(list[0]), num(list[1])]
		else:
			out = [num(list[0]), num(list[1]), num(list[2]), num(list[3])]
	else:
		out.fill(num(p))
	var b := _borders()
	for i: int in 4:
		out[i] += b[i]
	out[0] += num(own.get("pt", 0.0))
	return out


func _borders() -> Array[float]:
	var b: Variant = own.get("b", 0)
	var out: Array[float] = [0.0, 0.0, 0.0, 0.0]
	if b is Array:
		var list: Array = b
		out = [num(list[0]), num(list[1]), num(list[2]), num(list[3])]
	else:
		out.fill(num(b))
	return out


## Borders of a square box, each at least one screen pixel; see [method _stylebox].
func _hairlines(rect: Rect2) -> void:
	if _edges.is_empty():
		return
	var vars: Dictionary = inh["vars"]
	var color := style.color(own.get("bc", "line"), vars)
	# The content scale is in the viewport's final transform, not the canvas's.
	var screen := get_viewport().get_final_transform() * get_global_transform_with_canvas()
	var least := 1.0 / maxf(screen.get_scale().y, 0.01)
	var e: Array[float] = []
	for i: int in 4:
		e.append(maxf(_edges[i], least) if _edges[i] > 0.0 else 0.0)
	# A side with no border is a zero-width rect, which draws nothing.
	draw_rect(Rect2(rect.position, Vector2(rect.size.x, e[0])), color)
	draw_rect(Rect2(0, rect.size.y - e[2], rect.size.x, e[2]), color)
	draw_rect(Rect2(0, 0, e[3], rect.size.y), color)
	draw_rect(Rect2(rect.size.x - e[1], 0, e[1], rect.size.y), color)


func _radius() -> Array[float]:
	var r: Variant = own.get("r", 0)
	var out: Array[float] = [0.0, 0.0, 0.0, 0.0]
	if r is Array:
		var list: Array = r
		out = [num(list[0]), num(list[1]), num(list[2]), num(list[3])]
	else:
		out.fill(num(r))
	return out


func _stylebox() -> StyleBoxFlat:
	var vars: Dictionary = inh["vars"]
	var bg := style.color(own.get("bg"), vars)
	var borders := _borders()
	var lip: float = own.get("lip", 0.0)
	if bg.a == 0.0 and _sum(borders) == 0.0 and lip == 0.0:
		return null
	# A style's border widths are whole canvas units, and the canvas is scaled
	# by viewport height / 800, so at 720 a 1px rule is 0.9 of a screen pixel
	# and, on a square box with nothing to anti-alias, it falls between two
	# rows of pixel centres and is not drawn at all: one codex divider in eight
	# went missing, a different one at each scroll offset. Snapping transforms
	# to the pixel grid only moved which one. So square boxes draw their own
	# edges as float rects no thinner than a screen pixel, and rounded ones keep
	# the style, whose curve is anti-aliased and so always leaves a trace.
	_edges = []
	if _sum(_radius()) == 0.0 and lip == 0.0 and _sum(borders) > 0.0:
		_edges = borders
		if bg.a == 0.0:
			return null
		borders = [0.0, 0.0, 0.0, 0.0]
	var box := StyleBoxFlat.new()
	box.bg_color = bg
	box.draw_center = bg.a > 0.0
	box.border_color = style.color(own.get("bc", "line"), vars)
	box.border_width_top = whole(borders[0])
	box.border_width_right = whole(borders[1])
	box.border_width_bottom = whole(borders[2])
	box.border_width_left = whole(borders[3])
	var r := _radius()
	box.corner_radius_top_left = whole(r[0])
	box.corner_radius_top_right = whole(r[1])
	box.corner_radius_bottom_right = whole(r[2])
	box.corner_radius_bottom_left = whole(r[3])
	box.corner_detail = 6
	box.anti_aliasing = true
	if lip > 0.0:
		# `box-shadow: inset 0 -Npx 0 shade`, as a darker bottom edge: a flat
		# style has one border colour, so the lip takes the bottom border over.
		box.border_width_bottom = whole(borders[2] + lip)
		if borders[2] == 0.0:
			box.border_color = bg.darkened(0.28)
		box.border_blend = false
	return box


func _draw() -> void:
	var rect := Rect2(Vector2.ZERO, size)
	var vars: Dictionary = inh["vars"]
	match mode:
		"meter":
			var radius := size.y / 2.0
			_bar(rect, style.color("line"), radius)
			_bar(
				Rect2(0, 0, size.x * clampf(fill_solve, 0, 1), size.y),
				style.color(["mix", "chips", 28, "transparent"]),
				radius
			)
			_bar(
				Rect2(0, 0, size.x * clampf(fill, 0, 1), size.y),
				style.color("green" if met else "chips"),
				radius
			)
			return
		"bar":
			var radius := size.y / 2.0
			if _box != null:
				draw_style_box(_box, rect)
			_bar(Rect2(0, 0, size.x * clampf(fill, 0, 1), size.y), _color, radius)
			return
		"switch":
			if _box != null:
				draw_style_box(_box, rect)
			var knob := size.y / 2.0 - 2.0
			var x := size.x - knob - 2.0 if checked else knob + 2.0
			draw_circle(Vector2(x, size.y / 2.0), knob, _color, true, -1.0, true)
			return
	if _box != null:
		draw_style_box(_box, rect)
	_hairlines(rect)
	if _outline != null:
		draw_style_box(_outline, rect.grow(_outline_off))
	if mode == "text" and _para != null and text != "":
		var iw := size.x - _pad[1] - _pad[3]
		_para.width = iw
		var th := _para.get_size().y
		var y := _pad[0]
		if _center_v():
			y = _pad[0] + (size.y - _pad[0] - _pad[2] - th) / 2.0
		var at := Vector2(_pad[3], y)
		_para.draw(get_canvas_item(), at, _color)
		if inh.get("strike", false):
			for line: int in _para.get_line_count():
				var line_size := _para.get_line_size(line)
				var lx := at.x
				if _halign() == HORIZONTAL_ALIGNMENT_CENTER:
					lx += (iw - line_size.x) / 2.0
				elif _halign() == HORIZONTAL_ALIGNMENT_RIGHT:
					lx += iw - line_size.x
				var ly := at.y + line * line_size.y + line_size.y * 0.55
				draw_line(
					Vector2(lx, ly), Vector2(lx + line_size.x, ly), _color, maxf(1.0, _px() / 12.0)
				)
	elif mode == "icon" and texture != null:
		var px := _icon_px()
		draw_texture_rect(texture, Rect2((size - px) / 2.0, px), false)
	var dot: Variant = own.get("dot")
	if dot != null:
		var c := style.color(dot, vars)
		if c.a > 0.0:
			draw_circle(Vector2(size.x - 6.0, 6.0), 4.0, Color(c, 0.35), true, -1.0, true)
			draw_circle(Vector2(size.x - 6.0, 6.0), 2.5, c, true, -1.0, true)


func _bar(rect: Rect2, color: Color, radius: float) -> void:
	if rect.size.x <= 0.0:
		return
	var box := StyleBoxFlat.new()
	box.bg_color = color
	box.set_corner_radius_all(whole(minf(radius, rect.size.x / 2.0)))
	box.anti_aliasing = true
	draw_style_box(box, rect)


func _icon_px() -> Vector2:
	var w: float = own.get("W", 18.0)
	var h: float = own.get("H", w)
	return Vector2(w, h)


# ------------------------------------------------------------------ input


func make_clickable() -> void:
	mouse_filter = MOUSE_FILTER_STOP
	mouse_default_cursor_shape = CURSOR_POINTING_HAND
	mouse_entered.connect(func() -> void: self_modulate = Color(1.15, 1.15, 1.15))
	mouse_exited.connect(func() -> void: self_modulate = Color.WHITE)


func _gui_input(event: InputEvent) -> void:
	var button := event as InputEventMouseButton
	if button == null or button.button_index != MOUSE_BUTTON_LEFT or button.pressed:
		return
	if not Rect2(Vector2.ZERO, size).has_point(button.position):
		return
	accept_event()
	press()


## What a click on this box does, and Enter on it once Tab is there ([Focus]).
func press() -> void:
	if local_click.is_valid():
		local_click.call()
	elif click_id >= 0 and on_click.is_valid():
		on_click.call(click_id)
	elif href != "":
		OS.shell_open(href)
