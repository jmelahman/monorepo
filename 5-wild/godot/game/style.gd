class_name Style
extends RefCounted
## 5-wild's stylesheet ([code]src/ui/style.css[/code]), translated into a table
## the renderer matches against the trees [code]js/shell.ts[/code] hands out.
##
## Only what a desktop window needs came across. The web sheet spends most of
## its length on three things this build does not have: a phone (safe areas,
## [code]svh[/code], long-press tips), a browser's defects (the container-query
## flash, the reduced-motion block) and CSS animations, which are Godot tweens
## here and live in [code]game/animator.gd[/code]. What is left is the look:
## tokens, boxes, type. The class names are the web build's, so a view that
## changes shape on the web shows up here as a node that matches fewer rules,
## not as a crash, and the fix is a line in this table.
##
## A rule is [code][selector, props][/code]. Selectors are a subset of CSS:
## compounds of [code]tag[/code], [code].class[/code], [code][attr][/code],
## [code][attr=value][/code], [code]!class[/code] (not) and [code]:first[/code]
## / [code]:last[/code], joined by spaces as descendant steps. The document
## root is a virtual ancestor carrying [code]light[/code] or [code]dark[/code],
## and [code]quiet[/code] / [code]plain[/code] for the decor setting, which is
## how the web build's [code]:root[data-theme][/code], [code].quiet[/code] and
## [code].plain[/code] read here. Later rules win, as they mostly do in the
## source, and the few places the web leant on specificity instead are ordered
## by hand.
##
## Sizes are CSS pixels (1rem = 16). [code]game/main.gd[/code] scales the
## whole window so that a CSS pixel is a sensible size on a desktop screen.
##
## Props, all optional:
## [codeblock]
## d       layout: col row grid keys board meter bar switch   (default by tag)
## cols    grid columns; colw per column: ratio, "a" (auto) or "pN" (N px)
## span    a grid child that takes a whole row
## gap     gap between children; rgap between grid rows
## p       padding: N, [v, h] or [t, r, b, l]
## mt pt   extra space above the box / inside its top
## push    margin-top: auto (pushed to the bottom of a column)
## bg bc   background and border colours; b border width (N or [t, r, b, l])
## r       corner radius (N or [tl, tr, br, bl]); lip an inset bottom shadow
## f w c   font size, weight, colour; up uppercase; ta l/c/r; strike
## ls      letter-spacing in em (inherited, as on the web)
## a j     align-items / justify-content: start center end stretch between
## grow    flex-grow along the parent's axis
## W H     fixed size (W alone sizes an icon square); minw minh
## maxw    width cap, centred; maxh height cap (a fraction of the viewport)
## fillw   width: 100% inside a column that does not stretch
## hide op display: none; opacity
## abs     [anchor, dx, dy]: tr bl below, out of flow
## scroll  overflow-y: auto; clip one line with an ellipsis; nowrap
## dot     the rarity / modifier dot the web draws with ::after
## vars    {rare, mod, edge}: custom properties, inherited
## keyw    a key's width in keys (the wide ones are 1.5)
## [/codeblock]

## Inherited, as CSS inherits them.
const INHERITED := ["f", "w", "c", "up", "ls", "ta", "strike"]

const DARK := {
	"bg": "#0e0f13",
	"panel": "#191b22",
	"panel-2": "#22242e",
	"line": "#2e313d",
	"fg": "#e8e8ea",
	"muted": "#8b8d98",
	"green": "#538d4e",
	"yellow": "#b59f3b",
	"gray": "#3a3a3c",
	"chips": "#4a9eff",
	"mult": "#ff5c5c",
	"gold": "#f0b429",
	"rare-common": "#6b7280",
	"rare-uncommon": "#3d8f6a",
	"rare-rare": "#3f6fd8",
	"rare-legendary": "#a855f7",
	"on-tile": "#e8e8ea",
	"on-bright": "#0e0f13",
	"key": "#565758",
	"key-gray-fg": "#8b8d98",
	"broken-bg": "#1b1c20",
	"broken-fg": "#4a4b52",
	"shade": "#000000",
	"scrim": "#000000aa",
	"title-tint": "#2a3350",
	"row-note-bg": ["mix", "yellow", 22, "#000000"],
	"row-note-fg": "yellow",
	"mod-steel": "#9fb4c8",
	"mod-glass": "#7fd8e8",
	"mod-lucky": "#ff8fd0",
	"mod-echo": "#f2a65a",
	"mod-anchor": "#7ee787",
	"value-base": ["mix", "fg", 72, "transparent"],
	"wash": 16,
	"wash-tray": 26,
	"edge": 40,
}

const LIGHT := {
	"bg": "#f4f3ef",
	"panel": "#ffffff",
	"panel-2": "#ebeae5",
	"line": "#d6d5cf",
	"fg": "#1c1d22",
	"muted": "#6b6d76",
	"gray": "#787c7e",
	"chips": "#1f6fd6",
	"mult": "#d93b3b",
	"gold": "#a87400",
	"on-tile": "#ffffff",
	"on-bright": "#ffffff",
	"key": "#d6d7db",
	"key-gray-fg": "#ffffffb3",
	"broken-bg": "#e4e3de",
	"broken-fg": "#a7a8ae",
	"shade": "#00000059",
	"scrim": "#00000066",
	"title-tint": "#dde3f3",
	"row-note-bg": ["mix", "yellow", 18, "panel"],
	"row-note-fg": "#6f5f14",
	"mod-steel": "#4f6d8a",
	"mod-glass": "#13899c",
	"mod-lucky": "#c2338a",
	"mod-echo": "#b8650f",
	"mod-anchor": "#2c8a3c",
	"wash": 30,
	"wash-tray": 42,
	"edge": 70,
}

## The pastel modifier colours, which the web switches back to on a coloured
## tile or key in either theme, since the tile colours are fixed across themes.
const PASTEL := {
	"steel": "#9fb4c8",
	"glass": "#7fd8e8",
	"lucky": "#ff8fd0",
	"echo": "#f2a65a",
	"anchor": "#7ee787"
}

const TILE_MAX := 72.0
const KEY_H := 52.0
## [code]--boss-band[/code]: two lines of a boss's rule at its 10px and 1.3
## line height, plus 5px above and below. Fixed rather than fitted, because a
## round without a boss reserves it too; see [method Render.build].
const BOSS_BAND := 2 * 1.3 * 10 + 10

var theme := "dark"
var root_classes: PackedStringArray = ["dark"]

## Rules indexed by the tag or class their last compound must have, so a node
## is only tested against rules that could match it.
var _index: Dictionary[String, Array] = {}
var _colors: Dictionary[String, Color] = {}


func _init() -> void:
	for order: int in Rules.TABLE.size():
		var rule: Array = Rules.TABLE[order]
		var steps := _parse(str(rule[0]))
		var last: Dictionary = steps[-1]
		var classes: PackedStringArray = last["classes"]
		var key: String = "." + classes[0] if not classes.is_empty() else str(last["tag"])
		if key == "":
			key = "*"
		if not _index.has(key):
			_index[key] = []
		_index[key].append({"steps": steps, "props": rule[1], "order": order})
	set_theme("dark", "all")


func set_theme(which: String, decor: String) -> void:
	theme = "light" if which == "light" else "dark"
	root_classes = [theme]
	if decor == "minimal":
		root_classes.append("quiet")
	elif decor == "none":
		root_classes.append_array(["quiet", "plain"])
	_colors.clear()


## One element's own props, from every rule that matches it. [param node] is a
## tree element; [param chain] is its ancestors, outermost first, each as
## [code]{t, cls, a, i, n}[/code] (see [method entry]).
func match(chain: Array) -> Dictionary:
	var me: Dictionary = chain[-1]
	var hits: Array = []
	for key: String in _keys(me):
		for rule: Dictionary in _index.get(key, []):
			var steps: Array = rule["steps"]
			if _matches(steps, chain):
				hits.append(rule)
	hits.sort_custom(func(x: Dictionary, y: Dictionary) -> bool: return x["order"] < y["order"])
	var props := {}
	for rule: Dictionary in hits:
		var own: Dictionary = rule["props"]
		for key: String in own:
			if key == "vars" and props.has("vars"):
				var merged: Dictionary = props["vars"]
				var more: Dictionary = own["vars"]
				merged = merged.duplicate()
				merged.merge(more, true)
				props["vars"] = merged
			else:
				props[key] = own[key]
	return props


## What a tree element looks like to the matcher: its tag, classes and
## attributes, and its place among its siblings for [code]:first[/code] and
## [code]:last[/code].
static func entry(node: Dictionary, index: int, count: int) -> Dictionary:
	var cls: String = node.get("c", "")
	return {
		"t": node.get("t", ""),
		"cls": cls.split(" ", false),
		"a": node.get("a", {}),
		"i": index,
		"n": count,
	}


## A colour expression resolved against the theme and [param vars].
# gdlint:disable=max-returns
func color(expr: Variant, vars: Dictionary = {}) -> Color:
	if expr == null:
		return Color.TRANSPARENT
	if expr is Array:
		var mix: Array = expr
		var a := color(mix[1], vars)
		var share := _percent(mix[2]) / 100.0
		if typeof(mix[3]) == TYPE_STRING and mix[3] == "transparent":
			return Color(a.r, a.g, a.b, a.a * share)
		var b := color(mix[3], vars)
		return b.lerp(a, share)
	var text: String = str(expr)
	if text == "transparent":
		return Color.TRANSPARENT
	if text.begins_with("#"):
		return Color.html(text)
	if text.begins_with("$"):
		var parts := text.substr(1).split("|")
		if vars.has(parts[0]):
			return color(vars[parts[0]], vars)
		return color(parts[1], vars) if parts.size() > 1 else Color.TRANSPARENT
	if _colors.has(text):
		return _colors[text]
	var tokens := LIGHT if theme == "light" else DARK
	var value: Variant = tokens.get(text, DARK.get(text, "#ff00ff"))
	var resolved := color(value, vars)
	_colors[text] = resolved
	return resolved


# gdlint:enable=max-returns


func _percent(value: Variant) -> float:
	if typeof(value) == TYPE_STRING:
		var tokens := LIGHT if theme == "light" else DARK
		return type_convert(tokens.get(value, DARK.get(value, 50)), TYPE_FLOAT)
	return type_convert(value, TYPE_FLOAT)


func _keys(me: Dictionary) -> Array[String]:
	var keys: Array[String] = ["*", str(me["t"])]
	for cls: String in me["cls"]:
		keys.append("." + cls)
	return keys


## Descendant matching, right to left: the last step must be this node, and
## each earlier step some ancestor further out than the one before it. The
## virtual root is the outermost ancestor of every chain.
func _matches(steps: Array, chain: Array) -> bool:
	var last_step: Dictionary = steps[-1]
	var me: Dictionary = chain[-1]
	if not _compound(last_step, me):
		return false
	var at := chain.size() - 2
	for s in range(steps.size() - 2, -1, -1):
		var step: Dictionary = steps[s]
		var found := false
		while at >= -1:
			var candidate: Dictionary = (
				chain[at] if at >= 0 else {"t": "", "cls": root_classes, "a": {}, "i": 0, "n": 1}
			)
			at -= 1
			if _compound(step, candidate):
				found = true
				break
		if not found:
			return false
	return true


func _compound(step: Dictionary, me: Dictionary) -> bool:
	var tag: String = step["tag"]
	if tag != "" and tag != me["t"]:
		return false
	var have: PackedStringArray = me["cls"]
	for cls: String in step["classes"]:
		if not have.has(cls):
			return false
	for cls: String in step["not"]:
		if have.has(cls):
			return false
	var attrs: Dictionary = me["a"]
	var want: Dictionary = step["attrs"]
	for key: String in want:
		if not attrs.has(key) or (want[key] != null and str(attrs[key]) != want[key]):
			return false
	return _position(str(step["pos"]), me)


## [code]:first-child[/code] and [code]:last-child[/code], or true for a step
## that asks neither.
static func _position(pos: String, me: Dictionary) -> bool:
	match pos:
		"first":
			return me["i"] == 0
		"last":
			return me["i"] == type_convert(me["n"], TYPE_INT) - 1
	return true


static func _parse(selector: String) -> Array:
	var steps: Array = []
	var regex := RegEx.create_from_string("([.!:]?)([\\w-]+)|\\[([\\w-]+)(?:=([\\w-]+))?\\]")
	for part: String in selector.split(" ", false):
		# Built in locals and stored at the end: a PackedStringArray read out of
		# a Dictionary is a copy, and appending to it would change nothing.
		var tag := ""
		var classes: PackedStringArray = []
		var nots: PackedStringArray = []
		var attrs := {}
		var pos := ""
		for found: RegExMatch in regex.search_all(part):
			if found.get_string(3) != "":
				var want: Variant = null
				if found.get_string(4) != "":
					want = found.get_string(4)
				attrs[found.get_string(3)] = want
				continue
			var word := found.get_string(2)
			match found.get_string(1):
				".":
					classes.append(word)
				"!":
					nots.append(word)
				":":
					pos = word
				_:
					tag = word if word != "*" else ""
		steps.append({"tag": tag, "classes": classes, "not": nots, "attrs": attrs, "pos": pos})
	return steps
