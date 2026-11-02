class_name Street
extends RefCounted
## What stands where along a city street, as plain data: towers shoulder to shoulder
## down both sides, shops along their feet, and the signs, poles and clutter that belong
## to those shops. The rules, not the art. A shop's trade decides its sign, its goods
## and its light, so a street reads as businesses and not as random boxes.

## How far past the ride's ends the street carries on.
const REACH := 170.0
const LOT_MIN := 9.0
const LOT_MAX := 24.0
const SIDEWALK := 4.0
const LAMP_SPACING := 44.0
const SHOP_MIN := 3.2
const SHOP_MAX := 7.5
const SIGN_BASE := 4.6
## From the wall to over the road's edge, leaving the middle open to the sky above it.
const MAX_OVERHANG := 4.6
const CLOSED_CHANCE := 0.78
const CLOSED_UNDERPASS := 0.9
const CLOSED_WORKS := 0.95
const CLOSED_LANTERNS := 0.35
const CLOSED_RETAIL := 0.08
## Most buildings down here have nobody left to advertise.
const SIGNED_TOWERS := 0.25
const SIGNED_RETAIL := 0.7
## Out in the open, buildings stop where they like, and hanging signs clear the arcades.
const RETAIL_LOW := 13.0
const RETAIL_HIGH := 23.0
const RETAIL_SIGN_BASE := 6.4
const FACADES := 4
## How many of the brutalist forms [Facades] knows.
const FORMS := 4
## How far a building without signs may step out over the pavement as it rises.
const MAX_JETTY := 1.5
## Trades as [code]tools/bake_signs.gd[/code] names them on the sign sheet.
const TRADES: PackedStringArray = [
	"pharmacy",
	"hotel",
	"noodle",
	"massage",
	"electronics",
	"gold",
	"exchange",
	"clinic",
	"implant",
	"data",
	"robotics",
	"cafe",
]
## The trades that take a street front; the rest are upstairs, and only have signs.
const STREET_TRADES: PackedStringArray = [
	"pharmacy",
	"noodle",
	"noodle",
	"electronics",
	"robotics",
	"exchange",
	"gold",
	"cafe",
]


class Tower:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var width := 0.0
	var depth := 0.0
	var height := 0.0
	## Which wall finish, up to [constant FACADES].
	var facade := 0
	var bay := 2.4
	var storey := 3.1
	## 0 for newly built, 1 for decades of streaks and stains.
	var age := 0.0
	var tint := Color.WHITE
	## Tiers step back as they rise: 0 for a plain slab.
	var tiers := 0
	## Where along the wall air-con units hang, as [bay, storey] grid cells.
	var units: Array[Vector2i] = []
	## Which of the forms it takes, and how far its floors step out over the pavement.
	var form := 0
	var jetty := 0.0
	var dice := 0


class Shop:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var width := 0.0
	var depth := 0.0
	var trade := ""
	## A shuttered shop has no light and no goods.
	var open := true
	var pick := 0.0


class Sign:
	extends RefCounted
	var s := 0.0
	var side := 1.0
	var size := Vector2.ZERO
	var elevation := 0.0
	## Metres out from the wall to the far edge, 0 for flat on the shop's fascia.
	var overhang := 0.0
	var trade := ""
	var tall := false
	## Chooses among the sheet's designs for this trade.
	var pick := 0.0
	## 0 for steady, more for a sign on its way out.
	var flicker := 0.0


class Prop:
	extends RefCounted
	var kind := ""
	var s := 0.0
	## Metres from the road's centre, signed by side of the road.
	var offset := 0.0
	var turn := 0.0


var towers: Array[Tower] = []
var shops: Array[Shop] = []
var signs: Array[Sign] = []
var props: Array[Prop] = []
## Where a stretch of kerb railing runs, as [start, end, side].
var railings: Array[Vector3] = []
## Where the road was dug up and patched, as [s, offset, length, width].
var patches: Array[Vector4] = []
var lamps: Array[float] = []
## Covers set in the road, as [s, offset].
var covers: Array[Vector2] = []
var districts: Districts
## Where the street stops having a wall, for the plaza: from, to and side.
var openings: Array[Vector3] = []


## A street along the route over [code][0, length][/code].
static func generate(run_seed: int, length: float, kerb: float) -> Street:
	var street := Street.new()
	street.districts = Districts.generate(run_seed, length)
	for side: float in [-1.0, 1.0]:
		street._line_side(run_seed, side, length, kerb)
		street._dress_kerb(run_seed, side, length, kerb)
	street._dig_up_road(run_seed, length, kerb)
	var s := -REACH
	while s < length + REACH:
		if not Districts.roofed(street.districts.kind_at(s)):
			street.lamps.append(s)
		s += LAMP_SPACING
	return street


func _line_side(run_seed: int, side: float, length: float, kerb: float) -> void:
	var base := Rng.derive(run_seed, [40, int(side)])
	var s := -REACH
	var index := 0
	while s < length + REACH:
		var cell := Rng.derive(base, [index])
		var tower := Tower.new()
		tower.side = side
		tower.width = lerpf(LOT_MIN, LOT_MAX, Rng.unit(cell, 0))
		tower.s = s + tower.width * 0.5
		# Whole lots come out for the plaza, so its edges fall where the buildings end.
		if Districts.stands_open(districts.kind_at(tower.s)):
			_leave_open(s, s + tower.width, side)
			s += tower.width
			index += 1
			continue
		tower.depth = lerpf(14.0, 26.0, Rng.unit(cell, 1))
		# All of them run up into the deck, so there is no skyline down here.
		tower.height = lerpf(46.0, 70.0, Rng.unit(cell, 2))
		if Districts.sky_open(districts.kind_at(tower.s)):
			tower.height = lerpf(RETAIL_LOW, RETAIL_HIGH, Rng.unit(cell, 2))
		tower.facade = int(Rng.unit(cell, 4) * FACADES)
		tower.bay = lerpf(2.1, 3.4, Rng.unit(cell, 5))
		tower.storey = lerpf(2.9, 3.5, Rng.unit(cell, 6))
		tower.age = Rng.unit(cell, 7)
		var pale := lerpf(0.55, 1.0, Rng.unit(cell, 8))
		var warm := lerpf(-0.08, 0.08, Rng.unit(cell, 9))
		tower.tint = Color(pale + warm, pale, pale - warm)
		tower.tiers = int(Rng.unit(cell, 10) * 3.0)
		_hang_units(cell, tower)
		tower.form = int(Rng.unit(cell, 11) * FORMS)
		tower.dice = Rng.derive(cell, [3])
		towers.append(tower)
		_fill_with_shops(Rng.derive(cell, [1]), tower, kerb)
		_hang_signs(Rng.derive(cell, [2]), tower)
		s += tower.width
		index += 1


func _leave_open(from: float, to: float, side: float) -> void:
	# Vector3 holds these in less precision than they were added up in.
	if not openings.is_empty() and is_equal_approx(openings[-1].y, from) and openings[-1].z == side:
		openings[-1] = Vector3(openings[-1].x, to, side)
	else:
		openings.append(Vector3(from, to, side))


func _hang_units(cell: int, tower: Tower) -> void:
	var bays := int(tower.width / tower.bay)
	var storeys := mini(int(tower.height / tower.storey), 14)
	var chance := lerpf(0.05, 0.4, tower.age)
	for j: int in range(2, storeys):
		for i: int in bays:
			if Rng.unit(Rng.derive(cell, [i, j]), 0) < chance:
				tower.units.append(Vector2i(i, j))


func _fill_with_shops(cell: int, tower: Tower, kerb: float) -> void:
	var from := tower.s - tower.width * 0.5
	var end := tower.s + tower.width * 0.5
	var index := 0
	while end - from >= SHOP_MIN:
		var slot := Rng.derive(cell, [index])
		var width := lerpf(SHOP_MIN, SHOP_MAX, Rng.unit(slot, 0))
		# No sliver left over: the last shop takes what remains.
		if end - from - width < SHOP_MIN:
			width = end - from
		var shop := Shop.new()
		shop.side = tower.side
		shop.width = width
		shop.s = from + width * 0.5
		shop.depth = lerpf(4.5, 8.0, Rng.unit(slot, 1))
		shop.trade = STREET_TRADES[int(Rng.unit(slot, 2) * STREET_TRADES.size())]
		shop.open = Rng.unit(slot, 3) > _closed_chance(shop.s)
		shop.pick = Rng.unit(slot, 4)
		shops.append(shop)
		if shop.open:
			_add_fascia(slot, shop)
			_spill_onto_pavement(slot, shop, kerb)
		from += width
		index += 1


func _closed_chance(s: float) -> float:
	match districts.kind_at(s):
		Districts.Kind.UNDERPASS:
			return CLOSED_UNDERPASS
		Districts.Kind.WORKS:
			return CLOSED_WORKS
		Districts.Kind.LANTERNS:
			return CLOSED_LANTERNS
		Districts.Kind.RETAIL:
			return CLOSED_RETAIL
		_:
			return CLOSED_CHANCE


## The shop's own name, flat above its door.
func _add_fascia(slot: int, shop: Shop) -> void:
	var fascia := Sign.new()
	fascia.side = shop.side
	fascia.s = shop.s
	var high := minf(1.3, shop.width * 0.4)
	fascia.size = Vector2(high * 2.0, high)
	fascia.elevation = 3.05
	fascia.trade = shop.trade
	fascia.pick = Rng.unit(slot, 5)
	signs.append(fascia)


## What a shop puts out in front of itself, near its door and leaving a way through.
func _spill_onto_pavement(slot: int, shop: Shop, kerb: float) -> void:
	var count := int(Rng.unit(slot, 6) * 1.4)
	for i: int in count:
		var prop := Prop.new()
		var roll := Rng.unit(slot, 10 + i)
		prop.kind = "crate" if roll < 0.45 else ("carton" if roll < 0.8 else "stool")
		prop.s = shop.s + (Rng.unit(slot, 20 + i) - 0.5) * shop.width * 0.85
		prop.offset = shop.side * (kerb + SIDEWALK - 0.5 - Rng.unit(slot, 30 + i) * 0.7)
		prop.turn = Rng.unit(slot, 40 + i) * TAU
		props.append(prop)


## The odd sign for someone upstairs, out over the road and facing up it. Few enough down
## here that each one is a landmark, and mostly narrow vertical blades.
func _hang_signs(cell: int, tower: Tower) -> void:
	var kind := districts.kind_at(tower.s)
	var signed := kind in [Districts.Kind.MARKET, Districts.Kind.LANTERNS, Districts.Kind.RETAIL]
	var share := SIGNED_RETAIL if kind == Districts.Kind.RETAIL else SIGNED_TOWERS
	if Rng.unit(cell, 8) > share or not signed:
		if Rng.unit(cell, 9) < 0.4:
			tower.jetty = lerpf(0.5, MAX_JETTY, Rng.unit(cell, 10))
		return
	var from := tower.s - tower.width * 0.5
	var at := from + 0.8 + Rng.unit(cell, 0) * 1.5
	var column := 0
	while at < tower.s + tower.width * 0.5 - 0.5:
		var slot := Rng.derive(cell, [column])
		var lowest := RETAIL_SIGN_BASE if kind == Districts.Kind.RETAIL else SIGN_BASE
		var elevation := lowest + Rng.unit(slot, 0) * 1.5
		var stack := 1 + int(Rng.unit(slot, 1) * 2.0)
		for level: int in stack:
			var unit := Rng.derive(slot, [level, 3])
			var hung := Sign.new()
			hung.side = tower.side
			hung.s = at + (Rng.unit(unit, 0) - 0.5) * 0.5
			hung.trade = TRADES[int(Rng.unit(unit, 1) * TRADES.size())]
			hung.pick = Rng.unit(unit, 2)
			hung.tall = Rng.unit(unit, 3) < 0.75
			# Mostly modest, with the odd enormous one that sets the whole view.
			var scale := lerpf(0.8, 1.9, pow(Rng.unit(unit, 4), 2.2))
			hung.size = (
				Vector2(minf(scale, 2.2), scale * 2.0)
				if hung.tall
				else Vector2(minf(scale * 2.0, MAX_OVERHANG - 0.5), scale)
			)
			hung.elevation = elevation
			# Standing off the wall a little, with the widest reaching well over the road.
			hung.overhang = minf(hung.size.x + 0.5 + Rng.unit(unit, 5) * 1.4, MAX_OVERHANG)
			hung.flicker = pow(Rng.unit(unit, 6), 3.0)
			signs.append(hung)
			elevation += hung.size.y + 0.25 + Rng.unit(unit, 7) * 1.1
			if elevation + 2.0 > tower.height:
				break
		at += 3.0 + Rng.unit(slot, 2) * 4.0
		column += 1


## Railings, poles and bins along the kerb, in irregular runs: a street collects these
## over decades, and nobody spaced them out evenly.
func _dress_kerb(run_seed: int, side: float, length: float, kerb: float) -> void:
	var base := Rng.derive(run_seed, [41, int(side)])
	var s := -REACH
	var index := 0
	while s < length + REACH:
		var cell := Rng.derive(base, [index])
		var run := lerpf(3.0, 14.0, Rng.unit(cell, 0))
		if Rng.unit(cell, 1) < 0.55 and not Districts.stands_open(districts.kind_at(s)):
			railings.append(Vector3(s, s + run, side))
		var pole := Prop.new()
		var roll := Rng.unit(cell, 2)
		pole.kind = (
			"no_entry"
			if roll < 0.04
			else ("post" if roll < 0.6 else ("bin" if roll < 0.8 else "utility"))
		)
		pole.s = s + run + 0.4
		pole.offset = side * (kerb + 0.35 + Rng.unit(cell, 3) * 0.15)
		pole.turn = 0.0
		# The hall's causeway has room beside the road for a sign and nothing else.
		if pole.kind == "post" or districts.kind_at(pole.s) != Districts.Kind.CISTERN:
			props.append(pole)
		s += run + lerpf(1.5, 9.0, Rng.unit(cell, 4))
		index += 1


func _dig_up_road(run_seed: int, length: float, kerb: float) -> void:
	var base := Rng.derive(run_seed, [42])
	var s := -REACH
	var index := 0
	while s < length + REACH:
		var cell := Rng.derive(base, [index])
		var long := lerpf(1.2, 9.0, Rng.unit(cell, 0))
		var wide := lerpf(0.7, kerb * 0.9, Rng.unit(cell, 1))
		var offset := (Rng.unit(cell, 2) * 2.0 - 1.0) * (kerb - wide * 0.5)
		patches.append(Vector4(s, offset, long, wide))
		if Rng.unit(cell, 4) < 0.4:
			covers.append(Vector2(s + long + 0.8, (Rng.unit(cell, 5) * 2.0 - 1.0) * (kerb - 0.8)))
		s += long + lerpf(1.5, 16.0, Rng.unit(cell, 3))
		index += 1
