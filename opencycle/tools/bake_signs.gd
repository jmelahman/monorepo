extends SceneTree
## Bakes the street's shop signs into one sheet, each drawn with real type: big
## characters over small print, a floor or a phone number, in the styles a real street
## mixes. [code]signs.webp[/code] is the sheet and [code]signs.json[/code] says which
## cell is which trade, so [Street] can give a pharmacy a pharmacy's sign.
##
## Text comes from Noto Sans and Serif CJK, under the SIL Open Font License, which the
## bake finds as system fonts. Names are invented.
##
## Needs a real GPU. Run it through scripts/bake.sh.

const OUT := "res://assets/baked/%s"
const SHEET := 4096
const WIDE_CELL := Vector2i(512, 256)
const TALL_CELL := Vector2i(256, 512)
const WIDE_ROWS := 10
const TALL_ROWS := 3
const WEBP_QUALITY := 0.92
const NAMES: Array[Array] = [
	["金星", "GOLD STAR"],
	["新華", "SUN WAH"],
	["大發", "TAI FAT"],
	["永利", "WING LEE"],
	["鴻運", "HUNG WAN"],
	["東方", "ORIENTAL"],
	["富豪", "FU HO"],
	["天星", "TIN SING"],
	["明珠", "PEARL"],
	["美麗", "MEI LAI"],
	["皇冠", "IMPERIAL"],
	["萬福", "MAN FUK"],
	["華興", "WAH HING"],
	["寶島", "PO TO"],
	["利群", "LEI KWAN"],
	["銀河", "GALAXY"],
]
## Trade as [Street] names it, its characters, its English, and small print.
const TRADES: Array[Array] = [
	["pharmacy", "大藥房", "DISPENSARY", "中西藥 專營批發"],
	["hotel", "賓館", "HOTEL", "時鐘酒店 歡迎光臨"],
	["noodle", "麵家", "NOODLES", "雲吞麵 牛腩 粥品"],
	["cafe", "茶餐廳", "CAFE", "奶茶 多士 常餐"],
	["massage", "足浴按摩", "FOOT MASSAGE", "養生 推拿 通宵營業"],
	["electronics", "數碼電子", "ELECTRONICS", "手機 配件 維修"],
	["gold", "金行", "GOLDSMITH", "足金 珠寶 鑽石"],
	["exchange", "找換", "MONEY CHANGER", "兌換 外幣 匯率"],
	["karaoke", "卡拉OK", "KARAOKE", "包廂 啤酒 派對"],
	["clinic", "牙科診所", "DENTAL CLINIC", "植牙 洗牙 醫生"],
	["arcade", "機動遊戲", "ARCADE", "電玩 夾公仔 賽車"],
	["hair", "髮廊", "HAIR SALON", "剪髮 電髮 染髮"],
	["implant", "義體診所", "CYBER CLINIC", "神經 植入 維修"],
	["data", "數據銀行", "MEMORY BANK", "記憶 備份 加密"],
	["robotics", "機械人", "ROBOTICS", "零件 改裝 回收"],
	["market", "士多", "MINI MART", "零食 飲品 煙酒"],
]
const STYLES: PackedStringArray = ["lightbox", "led", "neon", "board"]
const PALETTES: Array[Array] = [
	[Color("0c8a4e"), Color("ffffff"), Color("ffe66b")],
	[Color("0a1140"), Color("ff3b30"), Color("ffffff")],
	[Color("c4122f"), Color("fff6d8"), Color("ffe14d")],
	[Color("f4f1e6"), Color("c4122f"), Color("143a8a")],
	[Color("ffb300"), Color("7a0c12"), Color("111111")],
	[Color("0a5a8f"), Color("ffffff"), Color("9be7ff")],
	[Color("6a1b9a"), Color("fff0ff"), Color("ffd6f6")],
	[Color("101010"), Color("ff2fa4"), Color("35f2ff")],
	[Color("0d3b2e"), Color("ffe66b"), Color("ffffff")],
	[Color("ff5a1f"), Color("ffffff"), Color("2a0d00")],
]

var _sans: SystemFont
var _serif: SystemFont
var _latin: SystemFont

@warning_ignore_start("integer_division")


func _initialize() -> void:
	_bake.call_deferred()


func _bake() -> void:
	_sans = _font("Noto Sans CJK HK", 900)
	_serif = _font("Noto Serif CJK HK", 900)
	_latin = _font("Noto Sans CJK HK", 700)
	var viewport := SubViewport.new()
	viewport.size = Vector2i(SHEET, SHEET)
	viewport.disable_3d = true
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(viewport)
	var canvas := Control.new()
	canvas.size = Vector2(SHEET, SHEET)
	viewport.add_child(canvas)
	var cells := _layout()
	canvas.draw.connect(func() -> void: _draw_all(canvas, cells))
	canvas.queue_redraw()
	for i: int in 4:
		await RenderingServer.frame_post_draw
	var image: Image = viewport.get_texture().get_image()
	image.convert(Image.FORMAT_RGB8)
	var err := image.save_webp(OUT % "signs.webp", true, WEBP_QUALITY)
	if err != OK:
		push_error("bake: cannot write signs: %s" % error_string(err))
		quit(1)
		return
	var meta: Array[Dictionary] = []
	for cell: Dictionary in cells:
		var rect: Rect2i = cell["rect"]
		(
			meta
			. append(
				{
					"trade": cell["trade"],
					"tall": cell["tall"],
					"style": cell["style"],
					"x": rect.position.x,
					"y": rect.position.y,
					"w": rect.size.x,
					"h": rect.size.y,
				}
			)
		)
	var file := FileAccess.open(OUT % "signs.json", FileAccess.WRITE)
	file.store_string(JSON.stringify({"sheet": SHEET, "cells": meta}, " ") + "\n")
	file.close()
	print("bake: %d signs" % cells.size())
	quit()


func _font(family: String, weight: int) -> SystemFont:
	var font := SystemFont.new()
	font.font_names = PackedStringArray([family])
	font.font_weight = weight
	font.antialiasing = TextServer.FONT_ANTIALIASING_GRAY
	font.generate_mipmaps = false
	font.subpixel_positioning = TextServer.SUBPIXEL_POSITIONING_DISABLED
	return font


## Which design goes in which cell. Fixed, not random: the sheet is a committed asset,
## and walking the lists at different strides spreads names, trades and looks evenly.
func _layout() -> Array[Dictionary]:
	var cells: Array[Dictionary] = []
	var index := 0
	for row: int in WIDE_ROWS:
		for column: int in SHEET / WIDE_CELL.x:
			var at := Vector2i(column * WIDE_CELL.x, row * WIDE_CELL.y)
			cells.append(_design(index, Rect2i(at, WIDE_CELL), false))
			index += 1
	var top := WIDE_ROWS * WIDE_CELL.y
	for row: int in TALL_ROWS:
		for column: int in SHEET / TALL_CELL.x:
			var at := Vector2i(column * TALL_CELL.x, top + row * TALL_CELL.y)
			cells.append(_design(index, Rect2i(at, TALL_CELL), true))
			index += 1
	return cells


func _design(index: int, rect: Rect2i, tall: bool) -> Dictionary:
	var trade: Array = TRADES[index % TRADES.size()]
	var given: Array = NAMES[(index * 7 + index / TRADES.size()) % NAMES.size()]
	var style := STYLES[(index * 3 + index / 5) % STYLES.size()]
	return {
		"trade": trade[0],
		"tall": tall,
		"style": style,
		"rect": rect,
		"name": given,
		"words": trade,
		"palette": PALETTES[(index * 5 + index / 11) % PALETTES.size()],
		"serif": index % 3 == 0,
		"bare": index % 4 == 1,
		"number":
		"%d%d/F  %d %04d" % [1 + index % 2, index % 10, 2300 + index * 37, index * 613 % 9999],
	}


func _draw_all(canvas: Control, cells: Array[Dictionary]) -> void:
	for cell: Dictionary in cells:
		_draw_sign(canvas, cell)


func _draw_sign(canvas: Control, cell: Dictionary) -> void:
	var rect: Rect2i = cell["rect"]
	var area := Rect2(rect)
	var palette: Array = cell["palette"]
	var board: Color = palette[0]
	var ink: Color = palette[1]
	var accent: Color = palette[2]
	var style: String = cell["style"]
	if style == "neon" or style == "led":
		board = board.darkened(0.82)
		# Dark on dark can't be read, and no shop would pay for it.
		if ink.get_luminance() < 0.45:
			ink = Color("ffe9c9")
		if accent.get_luminance() < 0.45:
			accent = Color("ffffff")
	canvas.draw_rect(area, board)
	if style == "lightbox":
		_tubes(canvas, area)
	if style == "led":
		_dots(canvas, area, accent)
	if style == "board":
		_scuff(canvas, area, rect.position.x + rect.position.y)
	var inner := area.grow(-area.size.x * (0.1 if cell["tall"] else 0.06))
	if cell["tall"]:
		_lay_out_tall(canvas, cell, inner, ink, accent, style)
	else:
		_lay_out_wide(canvas, cell, inner, ink, accent, style)
	if style == "board":
		canvas.draw_rect(area.grow(-3.0), accent, false, 3.0)


func _lay_out_wide(
	canvas: Control, cell: Dictionary, inner: Rect2, ink: Color, accent: Color, style: String
) -> void:
	var words: Array = cell["words"]
	var given: Array = cell["name"]
	var given_name: String = given[0]
	var word: String = words[1]
	var title := given_name + word
	var font: Font = _serif if cell["serif"] else _sans
	if cell["bare"]:
		# A name that needs no explaining: just big characters.
		var big := _fit(font, title, inner.size.x, inner.size.y * 0.8)
		var low := inner.position.y + inner.size.y * 0.5 + big * 0.36
		_text(canvas, font, title, Vector2(inner.position.x, low), inner.size.x, big, ink, style)
		return
	var size := _fit(font, title, inner.size.x, inner.size.y * 0.5)
	var baseline := inner.position.y + inner.size.y * 0.5
	_text(canvas, font, title, Vector2(inner.position.x, baseline), inner.size.x, size, ink, style)
	var english := "%s %s" % [given[1], words[2]]
	var english_size := _fit(_latin, english, inner.size.x, inner.size.y * 0.18)
	_text(
		canvas,
		_latin,
		english,
		Vector2(inner.position.x, inner.position.y + inner.size.y * 0.72),
		inner.size.x,
		english_size,
		accent,
		style
	)
	var small := "%s   %s" % [words[3], cell["number"]]
	var small_size := _fit(_sans, small, inner.size.x, inner.size.y * 0.13)
	_text(
		canvas,
		_sans,
		small,
		Vector2(inner.position.x, inner.position.y + inner.size.y * 0.96),
		inner.size.x,
		small_size,
		ink,
		"board"
	)


func _lay_out_tall(
	canvas: Control, cell: Dictionary, inner: Rect2, ink: Color, accent: Color, style: String
) -> void:
	var words: Array = cell["words"]
	var given: Array = cell["name"]
	var given_name: String = given[0]
	var word: String = words[1]
	var title := given_name + word
	var font: Font = _serif if cell["serif"] else _sans
	var count := title.length()
	var english_height := inner.size.y * 0.12
	var step := (inner.size.y - english_height) / count
	var size := mini(int(step * 0.92), int(inner.size.x))
	for i: int in count:
		var baseline := inner.position.y + step * (i + 0.86)
		_text(
			canvas,
			font,
			title[i],
			Vector2(inner.position.x, baseline),
			inner.size.x,
			size,
			ink,
			style
		)
	var english: String = words[2]
	var english_size := _fit(_latin, english, inner.size.x, english_height * 0.6)
	_text(
		canvas,
		_latin,
		english,
		Vector2(inner.position.x, inner.end.y - english_height * 0.2),
		inner.size.x,
		english_size,
		accent,
		style
	)


## The largest size at which [param text] fits, no taller than [param height].
func _fit(font: Font, text: String, width: float, height: float) -> int:
	var size := maxi(int(height), 8)
	var measured := font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, size).x
	if measured > width:
		size = maxi(int(size * width / measured), 8)
	return size


func _text(
	canvas: Control,
	font: Font,
	text: String,
	at: Vector2,
	width: float,
	size: int,
	ink: Color,
	style: String
) -> void:
	var align := HORIZONTAL_ALIGNMENT_CENTER
	if style == "neon":
		# Tubes: a soft halo, the coloured glass, and a nearly white core.
		for ring: Array in [[18, 0.1], [10, 0.2], [5, 0.45]]:
			var halo := ink
			halo.a = ring[1]
			var thick: int = ring[0]
			canvas.draw_string_outline(font, at, text, align, width, size, thick, halo)
		canvas.draw_string(font, at, text, align, width, size, ink.lerp(Color.WHITE, 0.55))
		return
	if style == "lightbox":
		var edge := ink.darkened(0.55)
		canvas.draw_string_outline(font, at, text, align, width, size, 4, edge)
	canvas.draw_string(font, at, text, align, width, size, ink)


## A lit box is brighter over its tubes, in soft bands across the face.
func _tubes(canvas: Control, area: Rect2) -> void:
	for ring: int in 14:
		var inset := area.size * 0.5 * (ring + 1.0) / 16.0
		var band := Rect2(area.position + inset, area.size - inset * 2.0)
		canvas.draw_rect(band, Color(1.0, 1.0, 1.0, 0.012))


## A border of LEDs, three rows deep and in two colours like the real ones.
func _dots(canvas: Control, area: Rect2, colour: Color) -> void:
	var pitch := 9.0
	var second := colour.lerp(Color.WHITE, 0.6)
	var across := int(area.size.x / pitch)
	var down := int(area.size.y / pitch)
	for j: int in down:
		for i: int in across:
			var ring := mini(mini(i, across - 1 - i), mini(j, down - 1 - j))
			if ring > 1:
				continue
			var at := area.position + Vector2(i + 0.5, j + 0.5) * pitch
			canvas.draw_circle(at, 3.2, colour if (i + j) % 2 == 0 else second)


## Grime on a board that has been out in the weather.
func _scuff(canvas: Control, area: Rect2, seed_value: int) -> void:
	for i: int in 26:
		var u := fposmod(sin(seed_value + i * 12.9898) * 43758.5453, 1.0)
		var v := fposmod(sin(seed_value + i * 78.233) * 12543.1234, 1.0)
		var w := 6.0 + fposmod(u * 91.7, 1.0) * area.size.x * 0.2
		var h := area.size.y * (0.1 + fposmod(v * 37.1, 1.0) * 0.5)
		var streak := Rect2(area.position + Vector2(u, v * 0.5) * area.size, Vector2(w * 0.2, h))
		canvas.draw_rect(streak.intersection(area), Color(0.1, 0.08, 0.05, 0.12))
