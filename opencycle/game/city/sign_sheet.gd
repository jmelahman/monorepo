class_name SignSheet
extends RefCounted
## Finds designs on the sign sheet that tools/bake_signs.gd draws: which cells suit a
## trade, wide or tall, and where each sits on the sheet.

const META := "res://assets/baked/signs.json"

var _size := 1.0
var _cells: Dictionary[String, Array] = {}


static func load_sheet() -> SignSheet:
	var sheet := SignSheet.new()
	var parsed: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(META))
	var size: float = parsed["sheet"]
	sheet._size = size
	var cells: Array = parsed["cells"]
	for entry: Variant in cells:
		var cell: Dictionary = entry
		var tall: bool = cell["tall"]
		var trade: String = cell["trade"]
		var key := "%s/%s" % [trade, tall]
		if key not in sheet._cells:
			sheet._cells[key] = []
		sheet._cells[key].append(cell)
	return sheet


## The part of the sheet to show, in UV, for a trade and a pick in [code][0, 1)[/code].
func pick(trade: String, tall: bool, roll: float) -> Dictionary:
	var choices: Array = _cells["%s/%s" % [trade, tall]]
	var cell: Dictionary = choices[int(roll * choices.size())]
	var x: float = cell["x"]
	var y: float = cell["y"]
	var w: float = cell["w"]
	var h: float = cell["h"]
	var style: String = cell["style"]
	# A texel in from the edge, so mipmaps don't bleed the sign next door.
	var rect := Rect2(x + 2.0, y + 2.0, w - 4.0, h - 4.0)
	return {"uv": Rect2(rect.position / _size, rect.size / _size), "style": style}
