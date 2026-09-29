class_name Flex
extends RefCounted
## How a row and a grid share their width among their children: the part of
## [Box]'s layout that is an algorithm rather than a box, and so a set of pure
## functions of the children ([param flow]) and the box's own props
## ([param own]). A box asks these twice over, once for its own narrowest and
## natural widths (at [code]INF[/code]) and once to place its children, and
## the two must agree, which they can only fail to by reading different state;
## here there is none to read.

## The keys in the keyboard's widest row, which sets the unit every key in
## every row is built from ([code]--key-w[/code]).
const KEYS_WIDE := 10


## Widths along a row. A row of keys is [method key_widths]. Otherwise,
## children that grow start from nothing and share the room by their grow,
## never going under their narrowest (flex: N 1 0); the rest take their natural
## width and give it back toward their narrowest when the row is too full
## (flex: 0 1 auto).
static func row_widths(flow: Array[Control], own: Dictionary, iw: float) -> Array[float]:
	if own.get("d") == "keys":
		return key_widths(flow, own, iw)
	var n := flow.size()
	var widths: Array[float] = []
	var mins: Array[float] = []
	var grows: Array[float] = []
	var avail := iw - gap_of(own) * maxf(n - 1, 0)
	var fixed := 0.0
	var grow_min := 0.0
	for child: Control in flow:
		var c := Box.css(child)
		var g: float = c.get("grow", 0.0)
		var mn := Box.min_of(child)
		mins.append(mn)
		grows.append(g)
		if g > 0.0 and not c.has("W"):
			widths.append(0.0)
			grow_min += mn
		else:
			var nw := Box.nat_of(child)
			widths.append(nw)
			fixed += nw
	var over := fixed + grow_min - avail
	if over > 0.0:
		var slack := 0.0
		for i: int in n:
			if widths[i] > 0.0:
				slack += widths[i] - mins[i]
		if slack > 0.0:
			var share := minf(over / slack, 1.0)
			fixed = 0.0
			for i: int in n:
				if widths[i] > 0.0:
					widths[i] -= (widths[i] - mins[i]) * share
					fixed += widths[i]
	var room := avail - fixed
	var active: Array[int] = []
	for i: int in n:
		if grows[i] > 0.0 and not Box.css(flow[i]).has("W"):
			active.append(i)
	while not active.is_empty():
		var total := 0.0
		for i: int in active:
			total += grows[i]
		var frozen := -1
		for i: int in active:
			if maxf(room, 0.0) * grows[i] / total < mins[i]:
				frozen = i
				break
		if frozen < 0:
			for i: int in active:
				widths[i] = maxf(room, 0.0) * grows[i] / total
			break
		widths[frozen] = mins[frozen]
		room -= mins[frozen]
		active.erase(frozen)
	return widths


## Wordle's proportions, as the web's: every key a tenth of the row less its
## gaps, never a share of the room, so the nine-key row comes out narrower and
## centres rather than stretching, and each key sits over the same column in
## every row. A wide key is 1.5 keys plus the half gap it swallows.
static func key_widths(flow: Array[Control], own: Dictionary, iw: float) -> Array[float]:
	var gap := gap_of(own)
	var unit := maxf((iw - gap * (KEYS_WIDE - 1)) / KEYS_WIDE, 0.0)
	var widths: Array[float] = []
	for child: Control in flow:
		var k: float = Box.css(child).get("keyw", 1.0)
		widths.append(unit * k + gap * (k - 1.0))
	return widths


static func cols(own: Dictionary) -> int:
	return Box.whole(own.get("cols", 1))


## Column widths for [param iw]. A ratio column is [code]minmax(auto, Nfr)[/code]:
## its share, but never under its widest cell's narrowest. With [param iw] at
## INF this is the grid's own narrowest ([param least]) or natural width.
static func grid_cols(
	flow: Array[Control], own: Dictionary, iw: float, least: bool
) -> Array[float]:
	var n := cols(own)
	var spec: Array = own.get("colw", [])
	var floor_w: Array[float] = []
	floor_w.resize(n)
	floor_w.fill(0.0)
	var nat: Array[float] = floor_w.duplicate()
	var col := 0
	for child: Control in flow:
		if Box.css(child).get("span", false):
			col = 0
			continue
		floor_w[col] = maxf(floor_w[col], Box.min_of(child))
		nat[col] = maxf(nat[col], Box.nat_of(child))
		col = (col + 1) % n
	var widths: Array[float] = []
	var fixed := 0.0
	for i: int in n:
		var s: Variant = spec[i] if i < spec.size() else 1
		if typeof(s) == TYPE_STRING and str(s) == "a":
			widths.append(nat[i] if not least else floor_w[i])
			fixed += widths[i]
		elif typeof(s) == TYPE_STRING and str(s).begins_with("p"):
			widths.append(str(s).substr(1).to_float())
			fixed += widths[i]
		else:
			widths.append(-Box.num(s))
	if iw == INF:
		# The grid's own width: every ratio column at the width its cells ask,
		# scaled so the ratios hold.
		var unit := 0.0
		for i: int in n:
			if widths[i] < 0.0:
				unit = maxf(unit, (floor_w[i] if least else nat[i]) / -widths[i])
		for i: int in n:
			if widths[i] < 0.0:
				widths[i] = unit * -widths[i]
		return widths
	var room := iw - gap_of(own) * (n - 1) - fixed
	# Ratio columns under their floor are frozen at it, as in a row.
	var active: Array[int] = []
	var shares: Array[float] = []
	for i: int in n:
		shares.append(-widths[i] if widths[i] < 0.0 else 0.0)
		if widths[i] < 0.0:
			active.append(i)
	while not active.is_empty():
		var total := 0.0
		for i: int in active:
			total += shares[i]
		var frozen := -1
		for i: int in active:
			if maxf(room, 0.0) * shares[i] / total < floor_w[i]:
				frozen = i
				break
		if frozen < 0:
			for i: int in active:
				widths[i] = maxf(room, 0.0) * shares[i] / total
			break
		widths[frozen] = floor_w[frozen]
		room -= floor_w[frozen]
		active.erase(frozen)
	return widths


## Places every cell: [code]{h, cells: [[control, rect]]}[/code].
static func grid(flow: Array[Control], own: Dictionary, iw: float, natural: bool) -> Dictionary:
	var n := cols(own)
	var widths := grid_cols(flow, own, iw, false)
	var gap := gap_of(own)
	var rgap: float = own.get("rgap", gap)
	var centred: bool = own.get("a") == "center"
	var rows: Array = []
	var current: Array = []
	for child: Control in flow:
		if Box.css(child).get("span", false):
			if not current.is_empty():
				rows.append(current)
				current = []
			rows.append([child])
			continue
		current.append(child)
		if current.size() == n:
			rows.append(current)
			current = []
	if not current.is_empty():
		rows.append(current)
	var cells: Array = []
	var y := 0.0
	for r: int in rows.size():
		var row: Array = rows[r]
		var first: Control = row[0]
		var spanning: bool = row.size() == 1 and Box.css(first).get("span", false)
		var h := 0.0
		for i: int in row.size():
			var w := iw if spanning else widths[i]
			var cell: Control = row[i]
			h = maxf(h, Box.h_of(cell, w, natural))
		var x := 0.0
		for i: int in row.size():
			var w := iw if spanning else widths[i]
			var cell_h := h
			var child: Control = row[i]
			if Box.css(child).has("H"):
				cell_h = Box.num(Box.css(child)["H"])
			elif centred:
				cell_h = Box.h_of(child, w, natural)
			# align-items: center sits a short cell mid-row; otherwise it
			# stretches to the row, and its text draws from the top.
			var top := y + (h - cell_h) / 2.0 if centred else y
			cells.append([child, Rect2(x, top, w, cell_h)])
			x += w + gap
		y += h + (rgap if r < rows.size() - 1 else 0.0)
	return {"h": y, "cells": cells}


static func gap_of(own: Dictionary) -> float:
	return Box.num(own.get("gap", 0.0))
