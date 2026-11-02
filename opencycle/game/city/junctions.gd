class_name Junctions
extends RefCounted
## Where two roads part: the building that comes to a point between them, which is what tells a
## rider there was a choice. And where a road nobody's riding stops being built, a wall in the fog.

const BAY := 3.2
const STOREY := 3.4
const GROUND_FLOOR := 4.6
## Above the deck nobody sees it, so from there it's just a block.
const FLOORS_TO := 36.0
const TOP := 62.0
## Behind the lots of the two streets, where they've taken over.
const BEHIND_LOTS := 0.5
const PAST_BACK := Street.LOT_MAX
const NOSE := 2.3
const LIT_CHANCE := 0.3
const ROOM := Color(1.0, 0.68, 0.38)
const STRIP := Color(0.45, 0.82, 1.0)
## The material lifts these by RetailRow.LIT, and the tonemap's generous: a lit surface is far
## darker here than it looks on screen, and only the tubes should get anywhere near blooming.
const SHOP := Color(0.05, 0.032, 0.018)
const TUBE := Color(0.62, 0.5, 0.36)
const BLIND := Color(0.034, 0.022, 0.012)
const STALL_RISER := 0.7
const SHOP_DEEP := 0.38
const SHELVES := 3
const GOODS_PER_SHELF := 5
const GOODS: Array[Color] = [
	Color(0.07, 0.014, 0.01),
	Color(0.012, 0.04, 0.05),
	Color(0.06, 0.04, 0.012),
	Color(0.035, 0.035, 0.04),
	Color(0.008, 0.008, 0.01),
]
const SHUTTERED := 0.45
const SHUTTER := Color(0.3, 0.32, 0.36)
const LIGHT_EVERY := 4
const STONE := Color(0.72, 0.74, 0.78)
const END_WIDE := 110.0
const END_HIGH := 60.0


static func build(section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "Junctions"
	var kit := CityMesh.new()
	var parts: Quarters.Parts = Once.made("quarters", Quarters._parts)
	for fork: Fork in section.segment.forks:
		var start := fork.along(fork.prow)
		var end := fork.along(fork.back + PAST_BACK)
		if fork.draws and section.takes(minf(start, end), maxf(start, end)):
			kit.at(start)
			_prow(kit, root, fork, parts)
	if section.ends_at >= 0.0:
		kit.at(section.from)
		var road := section.segment.road
		var place := road.at(section.ends_at) + Vector3.UP * END_HIGH * 0.5
		var frame := Transform3D(Quarters._frame(road, section.ends_at), place)
		kit.block(parts.concrete, frame, Vector3(END_WIDE, END_HIGH, 2.0), STONE * 0.5, 0.2)
	kit.commit(root)
	return root


static func _glass() -> Material:
	var glass := StandardMaterial3D.new()
	glass.albedo_color = Color(0.03, 0.04, 0.05)
	glass.roughness = 0.08
	glass.metallic = 0.8
	return glass


static func _lit() -> Material:
	return Once.made("retail_lit", RetailRow._lit)


## The two walls back from the point, and the point itself.
static func _prow(kit: CityMesh, root: Node3D, fork: Fork, parts: Quarters.Parts) -> void:
	var glass: Material = Once.made("prow_glass", _glass)
	var bays := ceili((fork.back + PAST_BACK - fork.prow) / BAY)
	for bay: int in bays:
		var near := fork.prow + bay * BAY
		var far := near + BAY
		var back_here := BEHIND_LOTS if near > fork.back else 0.0
		var back_there := BEHIND_LOTS if far > fork.back else 0.0
		var dice := Rng.derive(bay, [141, fork.other_name.hash()])
		var one := fork.wall_here(near, back_here)
		var two := fork.wall_here(far, back_there)
		var inward := fork.wall_there(near, 0.0) - one
		_bay(kit, one, two, inward, Rng.derive(dice, [0]), glass, parts)
		one = fork.wall_there(near, back_here)
		two = fork.wall_there(far, back_there)
		_bay(kit, one, two, -inward, Rng.derive(dice, [1]), glass, parts)
		if bay % LIGHT_EVERY == 1:
			root.add_child(_wash(fork.wall_here(near, -2.5)))
			root.add_child(_wash(fork.wall_there(near, -2.5)))
	var tip := (fork.wall_here(fork.prow, 0.0) + fork.wall_there(fork.prow, 0.0)) * 0.5
	_nose(kit, root, tip, fork, parts)


## Spill from the shops, and a little way up the floors above them.
static func _wash(at: Vector3) -> OmniLight3D:
	var wash := OmniLight3D.new()
	wash.light_color = Color(1.0, 0.72, 0.46)
	wash.light_energy = 2.5
	# It hangs right in front of the glass, which would only show it as a white spot.
	wash.light_specular = 0.0
	wash.light_volumetric_fog_energy = 0.1
	wash.omni_range = 14.0
	wash.position = at + Vector3.UP * 3.5
	return wash


## One bay of wall: a shop below, then a floor at a time, each with a sill and a window set back.
static func _bay(
	kit: CityMesh,
	a: Vector3,
	b: Vector3,
	inward: Vector3,
	dice: int,
	glass: Material,
	parts: Quarters.Parts
) -> void:
	var run := b - a
	var along := run.normalized()
	var basis := Basis(along, Vector3.UP, along.cross(Vector3.UP))
	var middle := (a + b) * 0.5
	var wide := run.length()
	var into := inward.normalized()
	if Rng.unit(dice, 100) < SHUTTERED:
		var shutter := Vector3(wide, GROUND_FLOOR - 0.4, 0.1)
		kit.box(parts.iron, _at(basis, middle, GROUND_FLOOR * 0.5 - 0.2), shutter, SHUTTER)
	else:
		_shop(kit, basis, middle, wide, into, dice, parts)
	var fascia := Vector3(wide, 0.8, 1.0)
	kit.block(parts.concrete, _at(basis, middle, GROUND_FLOOR), fascia, STONE * 0.7, 0.05)
	kit.bar(parts.steel, a, a + Vector3.UP * GROUND_FLOOR, 0.16, STONE * 0.4)
	var storey := 0
	var height := GROUND_FLOOR + 0.4
	while height < FLOORS_TO:
		var mid := height + STOREY * 0.5
		var roll := Rng.unit(dice, storey)
		var window := _at(basis, middle, mid + 0.2)
		if roll < LIT_CHANCE:
			_room(kit, basis, middle, mid + 0.2, wide, into, Rng.derive(dice, [storey]), parts)
		else:
			kit.box(glass, window, Vector3(wide - 0.5, STOREY - 1.0, 0.8))
		var sill := Vector3(wide, 0.6, 1.2)
		kit.block(parts.concrete, _at(basis, middle, height + 0.3), sill, STONE * 0.85, 0.05)
		for corner: Vector3 in [a, b]:
			var pier := Vector3(0.45, STOREY, 1.1)
			kit.block(parts.concrete, _at(basis, corner, mid), pier, STONE, 0.05)
		height += STOREY
		storey += 1
	# Solid behind all that, and on up through the deck.
	var core := Vector3(wide + 0.4, TOP, 0.4)
	var behind := middle + inward.normalized() * 0.6
	kit.box(parts.concrete, _at(basis, behind, TOP * 0.5), core, STONE * 0.4)


## A shop you can see into: a dim back wall, shelves of stock, a tube along the ceiling, and glass.
static func _shop(
	kit: CityMesh,
	basis: Basis,
	middle: Vector3,
	wide: float,
	into: Vector3,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var room_high := GROUND_FLOOR - 0.4
	var back := middle + into * SHOP_DEEP
	var bright := lerpf(0.6, 1.4, Rng.unit(dice, 101))
	var wall := Vector3(wide, room_high, 0.04)
	kit.box(_lit(), _at(basis, back, room_high * 0.5), wall, SHOP * bright)
	var riser := Vector3(wide, STALL_RISER, 0.12)
	kit.block(parts.concrete, _at(basis, middle, STALL_RISER * 0.5), riser, STONE * 0.5, 0.03)
	var tube := Vector3(wide * 0.7, 0.05, 0.08)
	kit.box(_lit(), _at(basis, middle + into * 0.15, room_high - 0.12), tube, TUBE)
	var along := basis.x
	for shelf: int in SHELVES:
		var height := STALL_RISER + 0.5 + shelf * 0.72
		var board := Vector3(wide, 0.04, 0.26)
		var on := back - into * 0.14
		kit.box(parts.steel, _at(basis, on, height), board, SHUTTER * 0.4)
		for item: int in GOODS_PER_SHELF:
			var roll := Rng.derive(dice, [shelf, item])
			var run := ((item + 0.5) / GOODS_PER_SHELF - 0.5) * wide
			var tall := lerpf(0.2, 0.52, Rng.unit(roll, 0))
			var goods := Vector3(wide / GOODS_PER_SHELF * 0.8, tall, 0.18)
			var hue: Color = GOODS[int(Rng.unit(roll, 1) * GOODS.size())]
			var foot := on + along * run
			kit.box(_lit(), _at(basis, foot, height + 0.02 + tall * 0.5), goods, hue)
	var pane := Vector3(wide, room_high - STALL_RISER, 0.02)
	var glass_at := room_high * 0.5 + STALL_RISER * 0.5
	var glass: Material = Once.made("retail_glass", RetailRow._glass)
	kit.box(glass, _at(basis, middle, glass_at), pane)
	var mullion := Vector3(0.07, room_high, 0.12)
	kit.box(parts.steel, _at(basis, middle, room_high * 0.5), mullion, SHUTTER * 0.3)


## A window with someone home: dim, mostly behind a blind, with a bar down the middle of the frame.
static func _room(
	kit: CityMesh,
	basis: Basis,
	middle: Vector3,
	height: float,
	wide: float,
	into: Vector3,
	dice: int,
	parts: Quarters.Parts
) -> void:
	var high := STOREY - 1.2
	var size := Vector3(wide - 0.6, high, 0.8)
	var warmth := lerpf(0.015, 0.06, Rng.unit(dice, 0))
	kit.box(_lit(), _at(basis, middle, height), size, ROOM * warmth)
	var face := middle - into * 0.41
	var drop := high * lerpf(0.25, 0.8, Rng.unit(dice, 1))
	var blind := Vector3(wide - 0.6, drop, 0.01)
	var top := height + high * 0.5 - drop * 0.5
	kit.box(_lit(), _at(basis, face, top), blind, BLIND * lerpf(0.6, 1.2, Rng.unit(dice, 2)))
	var bar := Vector3(0.05, high, 0.03)
	kit.box(parts.steel, _at(basis, middle - into * 0.42, height), bar, SHUTTER * 0.3)


static func _at(basis: Basis, foot: Vector3, height: float) -> Transform3D:
	return Transform3D(basis, foot + Vector3.UP * height)


## A round glass corner with light running up it, where the roads part.
static func _nose(
	kit: CityMesh, root: Node3D, tip: Vector3, fork: Fork, parts: Quarters.Parts
) -> void:
	var top := tip + Vector3.UP * FLOORS_TO
	kit.tube(parts.concrete, tip, tip + Vector3.UP * TOP, NOSE, STONE * 0.8, 16)
	var towards := (tip - fork.wall_here(fork.prow + 20.0, 0.0)).normalized()
	var across := towards.cross(Vector3.UP)
	for side: float in [-1.0, 0.0, 1.0]:
		var line := across * side * 1.5 + towards * (NOSE + 0.02 - absf(side) * 0.55)
		kit.bar(_lit(), tip + line, top + line, 0.07, STRIP)
	var high := GROUND_FLOOR
	while high < FLOORS_TO:
		var band := tip + Vector3.UP * high
		kit.tube(parts.concrete, band, band + Vector3.UP * 0.5, NOSE + 0.35, STONE, 16)
		high += STOREY
	var wash := OmniLight3D.new()
	wash.light_color = Color(0.55, 0.8, 1.0)
	wash.light_energy = 7.0
	wash.light_volumetric_fog_energy = 0.4
	wash.omni_range = 26.0
	wash.position = tip + towards * 6.0 + Vector3.UP * 5.0
	root.add_child(wash)
