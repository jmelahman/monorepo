class_name Quarters
extends RefCounted
## Draws what makes each of [Districts]' stretches its own place: the underpass's low roof
## and sick green tubes, the pipes and sodium light over the works, and the open plaza with
## its colossal drums, under the one hole in the deck big enough to feel like sky.

const ROOF := 7.4
const ROOF_THICK := 1.4
const ROOF_STEP := 4.0
const BEAM_EVERY := 6.0
const COLUMN_EVERY := 12.0
const TUBE_RUN := 3.2
const TUBE_GAP := 1.6
const TUBE_DEAD := 0.14
const TUBE := Color(0.72, 1.05, 0.82)
const TUNNEL_LIGHT := Color(1.0, 1.0, 0.92)
## Every so many bays a dim pool between the shadowed spots. The bays with neither have failed
## tubes, and the dark between pools is what gives the tunnel length.
const TUNNEL_POOL_EVERY := 2
const TUNNEL_LIT_EVERY := 12.0
const TUNNEL_SHADOW_EVERY := 3
const HAZARD := Color(2.6, 0.12, 0.06)
const PORTAL_HIGH := 18.0
const SLEEVE_EVERY := 3.2
const SLEEVE_HIGH := 3.9
const SLEEVE_TOP := 11.5
const SLEEVE_CHANCE := 0.72
const WALL_PIPES: PackedFloat32Array = [5.3, 5.95, 7.1]
const BRACKET_EVERY := 4.0
const SODIUM := Color(1.0, 0.56, 0.2)
const STEAM_EVERY := 2
const PLAZA_DEPTH := 42.0
const DRUM_OFFSET := 25.0
const DRUM_EVERY := 30.0
const DRUM_THICK := 4.2
const TERRACE_DEEP := 8.0
const TERRACE_HIGH := 2.6
const STAIR_STEPS := 9
const FIN_EVERY := 7.0
const RECESS := 2.2
const PIER_WIDE := 0.9
const FLOOR_FIRST := 5.0
const FLOOR_HIGH := 4.2
const PARAPET := 1.3
const ROOM_LIT := 0.34
const GALLERY_LAMP := 0.7
const BENCH_OFFSET := 8.5
const BENCH_LONG := 3.4
const BENCH_EVERY := 11.0
const RING_TURN := 0.9
const RING_RADIUS := 11.0
const RING_PIECES := 40
const RING_OFFSET := 14.0
const RING_BLUE := Color(0.5, 1.8, 3.2)
const POOL_WIDE := 7.0
const SHAFT_EVERY := 36.0
const COLD := Color(0.62, 0.8, 1.0)
const UPLIGHT := Color(1.0, 0.72, 0.42)
const MODELLED := "res://assets/modelled/%s.glb"
const MONOLITH := "res://assets/generated/monolith.glb"
const PLATES := "res://assets/generated/plates.glb"
const MONOLITH_TALL := 31.5
const PLATES_TALL := 16.0
## Rusted steel drinks light that concrete throws back.
const PLATES_LIT := 3.5
const SCULPTURE_OFFSET := 16.0
const SCULPTURE_IN := 30.0
const LINTEL_LAMP := 0.5
const STAIR_LAMP_EVERY := 16.0
## Thin things go first when a model is coarsened, and a mullion is six centimetres wide.
const MODELLED_TOLERANCE := 0.008
const MODELLED_REACH := 500.0
const CLOTH_DIM := 0.3
const WASH_CHANCE := 0.5
const LETTINGS: Array[String] = ["bay_a", "bay_b", "bay_c", "bay_d", "bay_e", "bay_f", "bay_g"]


class Parts:
	extends RefCounted
	var concrete: ShaderMaterial
	var steel: StandardMaterial3D
	var rust: StandardMaterial3D
	var paint: StandardMaterial3D
	var iron: StandardMaterial3D
	var worn: StandardMaterial3D
	var pavers: StandardMaterial3D
	var glow: StandardMaterial3D
	var pool: StandardMaterial3D
	var housing: Kit


static func build(route: Route, street: Street, section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "Quarters"
	var kit := CityMesh.new()
	var parts: Parts = Once.made("quarters", _parts)
	for stretch: Districts.Stretch in street.districts.of_kind(Districts.Kind.UNDERPASS):
		if section.takes(stretch.from, stretch.to):
			_underpass(kit, root, route, stretch, parts)
	for stretch: Districts.Stretch in street.districts.of_kind(Districts.Kind.WORKS):
		if section.takes(stretch.from, stretch.to):
			_works(kit, root, route, stretch, parts)
	var index := 0
	for gantry: Districts.Gantry in street.districts.gantries:
		if section.holds(gantry.s):
			_gantry(kit, root, route, gantry, index, parts)
		index += 1
	for opening: Vector3 in street.openings:
		var kind := street.districts.kind_at((opening.x + opening.y) * 0.5)
		if kind == Districts.Kind.PLAZA and section.takes(opening.x, opening.y):
			_plaza(kit, root, route, opening, parts)
	for stretch: Districts.Stretch in street.districts.of_kind(Districts.Kind.CISTERN):
		Cistern.build(kit, root, route, stretch, section, parts)
	kit.commit(root)
	parts.housing.commit(root, MODELLED_REACH)
	return root


static func _parts() -> Parts:
	var parts := Parts.new()
	parts.concrete = Overhead.poured()
	parts.steel = Facades.scan("rusty_painted_metal", 1.2, Color(0.7, 0.7, 0.72))
	parts.rust = Facades.scan("rusty_metal_02", 1.0, Color(0.62, 0.56, 0.5))
	parts.paint = Facades.scan("rusty_painted_metal", 1.0, Color(0.5, 0.42, 0.14))
	parts.paint.metallic = 0.0
	parts.iron = Facades.scan("rusty_corrugated_iron", 2.4, Color(0.8, 0.76, 0.72))
	parts.worn = Facades.scan("worn_corrugated_iron", 2.4, Color(0.62, 0.66, 0.68))
	parts.pavers = Facades.scan("square_concrete_pavers", 3.0, Color(0.5, 0.54, 0.6))
	parts.pavers.metallic = 0.0
	parts.glow = StandardMaterial3D.new()
	parts.glow.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	parts.glow.vertex_color_use_as_albedo = true
	parts.pool = StandardMaterial3D.new()
	parts.pool.albedo_color = Color(0.015, 0.02, 0.025)
	parts.pool.roughness = 0.04
	parts.pool.metallic = 0.7
	var cloth := StandardMaterial3D.new()
	cloth.vertex_color_use_as_albedo = true
	cloth.roughness = 0.95
	cloth.cull_mode = BaseMaterial3D.CULL_DISABLED
	# Laundry's colours are chosen to read as colours, but it hangs in the same dark as the
	# concrete, and the concrete comes out far darker than a colour this size.
	cloth.albedo_color = Color(CLOTH_DIM, CLOTH_DIM, CLOTH_DIM)
	var glass := StandardMaterial3D.new()
	glass.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	glass.albedo_color = Color(0.04, 0.05, 0.06, 0.28)
	glass.roughness = 0.08
	glass.metallic = 0.7
	var modelled_concrete: ShaderMaterial = parts.concrete.duplicate()
	modelled_concrete.set_shader_parameter("modelled", true)
	var materials: Dictionary[String, Material] = {
		"concrete": modelled_concrete,
		"steel": parts.steel,
		"rust": parts.rust,
		"paint": parts.paint,
		"worn": parts.worn,
		"glow": parts.glow,
		"room": parts.glow,
		"cloth": cloth,
		"glass": glass,
	}
	parts.housing = Kit.new(MODELLED % "housing_bay", MODELLED_TOLERANCE)
	parts.housing.restyle(materials)
	return parts


static func _frame(route: Route, s: float) -> Basis:
	return Basis(route.right(s), Vector3.UP, route.tangent(s))


static func _wall() -> float:
	return NeonBiome.STREET_HALF + Street.SIDEWALK


static func _underpass(
	kit: CityMesh, root: Node3D, route: Route, stretch: Districts.Stretch, parts: Parts
) -> void:
	var wall := _wall()
	var roof_tint := Color(0.42, 0.46, 0.44)
	var s := stretch.from
	while s < stretch.to:
		kit.at(s)
		var long := minf(ROOF_STEP, stretch.to - s)
		var middle := route.at(s + long * 0.5) + Vector3.UP * (ROOF + ROOF_THICK * 0.5)
		kit.box(
			parts.concrete,
			Transform3D(_frame(route, s + long * 0.5), middle),
			Vector3((wall + 1.0) * 2.0, ROOF_THICK, long + 0.1),
			roof_tint
		)
		s += ROOF_STEP
	s = stretch.from + BEAM_EVERY * 0.5
	while s < stretch.to:
		kit.at(s)
		var place := route.at(s) + Vector3.UP * (ROOF - 0.5)
		kit.block(
			parts.concrete,
			Transform3D(_frame(route, s), place),
			Vector3(wall * 2.0, 1.0, 0.8),
			roof_tint * 1.15,
			0.06
		)
		s += BEAM_EVERY
	var index := 0
	s = stretch.from + COLUMN_EVERY * 0.5
	while s < stretch.to:
		kit.at(s)
		for side: float in [-1.0, 1.0]:
			_tunnel_column(kit, route, s, side, parts)
		if index % 2 == 0:
			root.add_child(_warning_lamp(kit, route, s, 1.0 if index % 4 == 0 else -1.0, parts))
		index += 1
		s += COLUMN_EVERY
	_tunnel_services(kit, route, stretch, parts)
	_tunnel_lights(root, route, stretch)
	for end: float in [stretch.from, stretch.to]:
		_portal(kit, route, end, parts)


static func _tunnel_column(
	kit: CityMesh, route: Route, s: float, side: float, parts: Parts
) -> void:
	var offset := route.right(s) * side * (NeonBiome.STREET_HALF + 1.2)
	var basis := _frame(route, s)
	var foot := route.at(s) + offset
	kit.block(
		parts.concrete,
		Transform3D(basis, foot + Vector3.UP * ROOF * 0.5),
		Vector3(0.8, ROOF, 1.5),
		Color(0.5, 0.52, 0.5),
		0.05
	)
	# Scuffed yellow to knee height: this is how far the cleaning machines hit it.
	kit.box(
		parts.paint,
		Transform3D(basis, foot + Vector3.UP * 0.55),
		Vector3(0.9, 1.1, 1.6),
		Color.WHITE
	)
	_column_fittings(kit, basis, foot, parts)
	# Flaring into the roof, which is the look of a thing holding up a city.
	var knee := foot + Vector3.UP * (ROOF - 2.2)
	var head := foot + route.right(s) * -side * 1.7 + Vector3.UP * (ROOF - 0.4)
	kit.bar(parts.concrete, knee, head, 0.55, Color(0.46, 0.48, 0.46))


## What's bolted to a column over the years: angle iron on its corners where lorries clip it,
## steel bands round a repair, and a downpipe with its brackets.
static func _column_fittings(kit: CityMesh, basis: Basis, foot: Vector3, parts: Parts) -> void:
	var worn := Color(0.34, 0.32, 0.3)
	for x: float in [-0.41, 0.41]:
		for z: float in [-0.76, 0.76]:
			var corner := foot + basis.x * x + basis.z * z
			kit.bar(parts.steel, corner, corner + Vector3.UP * 2.6, 0.05, worn)
	for high: float in [2.7, 3.3, 5.1]:
		kit.box(
			parts.steel,
			Transform3D(basis, foot + Vector3.UP * high),
			Vector3(0.86, 0.09, 1.56),
			worn
		)
	var pipe := foot + basis.z * 0.5 - basis.x * 0.45
	kit.tube(parts.steel, pipe, pipe + Vector3.UP * (ROOF - 0.6), 0.055, Color(0.42, 0.4, 0.34), 6)
	for high: float in [1.4, 3.0, 4.6]:
		var bracket := pipe + Vector3.UP * high
		kit.box(parts.steel, Transform3D(basis, bracket), Vector3(0.14, 0.05, 0.14), worn)


static func _warning_lamp(
	kit: CityMesh, route: Route, s: float, side: float, parts: Parts
) -> OmniLight3D:
	var at := (
		route.at(s) + route.right(s) * side * (NeonBiome.STREET_HALF + 0.75) + Vector3.UP * 3.0
	)
	kit.box(parts.glow, Transform3D(_frame(route, s), at), Vector3(0.1, 0.5, 0.26), HAZARD)
	var lamp := OmniLight3D.new()
	lamp.light_color = Color(1.0, 0.1, 0.05)
	lamp.light_energy = 1.4
	lamp.omni_range = 6.0
	lamp.light_volumetric_fog_energy = 0.4
	lamp.position = at
	return lamp


## Tubes along the roof with a few gone dead, and what runs along the walls beneath it.
static func _tunnel_services(
	kit: CityMesh, route: Route, stretch: Districts.Stretch, parts: Parts
) -> void:
	var wall := _wall()
	var index := 0
	var s := stretch.from + 1.0
	while s + TUBE_RUN < stretch.to:
		kit.at(s)
		var dice := Rng.derive(index, [90])
		for side: float in [-1.0, 1.0]:
			if Rng.unit(dice, int(side + 1.0)) < TUBE_DEAD:
				continue
			var low := Vector3.UP * (ROOF - 1.04)
			var from := route.at(s) + route.right(s) * side * 2.3 + low
			var end := s + TUBE_RUN
			var to := route.at(end) + route.right(end) * side * 2.3 + low
			kit.bar(parts.glow, from, to, 0.08, TUBE)
			var tray := Vector3.UP * 0.1
			kit.bar(parts.steel, from + tray, to + tray, 0.17, Color(0.3, 0.3, 0.28))
			for hanger: Vector3 in [from, to]:
				kit.bar(parts.steel, hanger, hanger + Vector3.UP * 1.1, 0.03, Color(0.3, 0.3, 0.28))
		s += TUBE_RUN + TUBE_GAP
		index += 1
	s = stretch.from
	while s < stretch.to:
		kit.at(s)
		var next := minf(s + ROOF_STEP, stretch.to)
		for side: float in [-1.0, 1.0]:
			for high: float in [5.35, 5.7, 6.2]:
				var from := route.at(s) + route.right(s) * side * (wall - 0.3)
				var to := route.at(next) + route.right(next) * side * (wall - 0.3)
				var lift := Vector3.UP * high
				kit.tube(parts.steel, from + lift, to + lift, 0.08, Color(0.5, 0.5, 0.48), 6)
		s = next


static func _tunnel_lights(root: Node3D, route: Route, stretch: Districts.Stretch) -> void:
	var index := 0
	var s := stretch.from + TUNNEL_LIT_EVERY * 0.5
	while s < stretch.to:
		var above := route.at(s) + Vector3.UP * (ROOF - 1.3)
		if index % TUNNEL_SHADOW_EVERY == 0:
			# Only some throw shadows, but the columns' shadows are what give the place depth.
			var spot := SpotLight3D.new()
			spot.light_color = TUNNEL_LIGHT
			spot.light_energy = 6.5
			spot.light_volumetric_fog_energy = 0.08
			spot.spot_range = 18.0
			spot.spot_angle = 72.0
			spot.spot_attenuation = 0.7
			spot.shadow_enabled = true
			spot.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
			spot.position = above
			root.add_child(spot)
		elif index % TUNNEL_POOL_EVERY == 0:
			var omni := OmniLight3D.new()
			omni.light_color = TUNNEL_LIGHT
			omni.light_energy = 2.4
			omni.light_volumetric_fog_energy = 0.0
			omni.omni_range = 11.0
			omni.position = above
			root.add_child(omni)
		for side: float in [-1.0, 1.0]:
			# The tubes don't reach the walls, so the shutters get old sodium lamps instead,
			# whose warmth against the tubes is the only colour down here.
			var fill := OmniLight3D.new()
			var warm := (index % 2 == 0) == (side > 0.0)
			fill.light_color = SODIUM if warm else TUNNEL_LIGHT
			fill.light_energy = 3.5 if warm else 1.0
			fill.light_volumetric_fog_energy = 0.0
			fill.omni_range = 10.0
			fill.position = route.at(s) + route.right(s) * side * (_wall() - 1.5) + Vector3.UP * 4.0
			root.add_child(fill)
		index += 1
		s += TUNNEL_LIT_EVERY


## The tunnel's mouth, with the city's weight sat on top and a warning band across its brow.
static func _portal(kit: CityMesh, route: Route, s: float, parts: Parts) -> void:
	kit.at(s)
	var wall := _wall()
	var basis := _frame(route, s)
	var high := PORTAL_HIGH - ROOF
	var centre := route.at(s) + Vector3.UP * (ROOF + high * 0.5)
	kit.block(
		parts.concrete,
		Transform3D(basis, centre),
		Vector3((wall + 2.0) * 2.0, high, 3.0),
		Color(0.56, 0.58, 0.6),
		0.12
	)
	var stripes := 18
	for stripe: int in stripes:
		var across := lerpf(-wall, wall, (stripe + 0.5) / stripes)
		var paint := Color(0.72, 0.5, 0.05) if stripe % 2 == 0 else Color(0.04, 0.04, 0.04)
		kit.box(
			parts.steel,
			Transform3D(basis, route.at(s) + route.right(s) * across + Vector3.UP * (ROOF + 0.45)),
			Vector3(wall * 2.0 / stripes, 0.9, 3.1),
			paint
		)


static func _works(
	kit: CityMesh, root: Node3D, route: Route, stretch: Districts.Stretch, parts: Parts
) -> void:
	var wall := _wall()
	for side: float in [-1.0, 1.0]:
		_sleeve_wall(kit, route, stretch, side, parts)
		var s := stretch.from
		while s < stretch.to:
			kit.at(s)
			var next := minf(s + ROOF_STEP, stretch.to)
			var depth := 0
			for high: float in WALL_PIPES:
				var off := wall - 0.45 - depth * 0.28
				var thick := 0.26 + depth * 0.14
				var a := route.at(s) + route.right(s) * side * off + Vector3.UP * high
				var b := route.at(next) + route.right(next) * side * off + Vector3.UP * high
				kit.tube(parts.rust, a, b, thick * 0.5)
				depth += 1
			if fmod(s - stretch.from, BRACKET_EVERY * 2.0) < ROOF_STEP:
				_bracket(kit, route, s, side, parts)
			s = next
	var lamp := stretch.from + 12.0
	var index := 0
	while lamp < stretch.to:
		var dice := Rng.derive(index, [91])
		if Rng.unit(dice, 0) < 0.5:
			var side := 1.0 if Rng.unit(dice, 1) < 0.5 else -1.0
			root.add_child(_wall_flood(route, lamp, side))
		lamp += 22.0
		index += 1


## Sheet iron over the first few floors, in panels hung by whoever needed to, when they did.
static func _sleeve_wall(
	kit: CityMesh, route: Route, stretch: Districts.Stretch, side: float, parts: Parts
) -> void:
	var wall := _wall()
	var index := 0
	var s := stretch.from
	while s < stretch.to:
		kit.at(s)
		var dice := Rng.derive(index, [92, int(side)])
		index += 1
		if Rng.unit(dice, 0) < SLEEVE_CHANCE:
			var top := lerpf(SLEEVE_HIGH + 3.0, SLEEVE_TOP, Rng.unit(dice, 1))
			var off := wall - 0.08 - Rng.unit(dice, 2) * 0.1
			var shade := lerpf(0.55, 1.0, Rng.unit(dice, 3))
			var sheet := parts.iron if Rng.unit(dice, 4) < 0.6 else parts.worn
			var place := (
				route.at(s) + route.right(s) * side * off + Vector3.UP * (SLEEVE_HIGH + top) * 0.5
			)
			kit.box(
				sheet,
				Transform3D(_frame(route, s), place),
				Vector3(0.06, top - SLEEVE_HIGH, SLEEVE_EVERY - 0.05),
				Color(shade, shade, shade)
			)
		s += SLEEVE_EVERY


static func _bracket(kit: CityMesh, route: Route, s: float, side: float, parts: Parts) -> void:
	var wall := _wall()
	var out := route.right(s) * side
	var from := route.at(s) + out * (wall - 0.05) + Vector3.UP * 7.6
	kit.bar(parts.steel, from, from - Vector3.UP * 2.8, 0.1, Color(0.5, 0.48, 0.45))
	for high: float in WALL_PIPES:
		var at := route.at(s) + out * (wall - 0.05) + Vector3.UP * (high - 0.26)
		kit.bar(parts.steel, at, at - out * 1.0, 0.09, Color(0.5, 0.48, 0.45))


static func _wall_flood(route: Route, s: float, side: float) -> OmniLight3D:
	var flood := OmniLight3D.new()
	flood.light_color = SODIUM
	flood.light_energy = 2.4
	flood.light_volumetric_fog_energy = 0.6
	flood.omni_range = 12.0
	flood.position = (route.at(s) + route.right(s) * side * (_wall() - 1.2) + Vector3.UP * 4.6)
	return flood


## A pipe bridge: legs at the kerbs, a trussed top and pipes over the road, with a sodium
## flood slung beneath it that the whole structure throws its shadow from.
static func _gantry(
	kit: CityMesh, root: Node3D, route: Route, gantry: Districts.Gantry, index: int, parts: Parts
) -> void:
	var s := gantry.s
	kit.at(s)
	var across := route.right(s)
	var along := route.tangent(s)
	var centre := route.at(s)
	var reach := NeonBiome.STREET_HALF + 1.7
	var up := Vector3.UP * gantry.height
	var tint := Color(0.62, 0.58, 0.5)
	for side: float in [-1.0, 1.0]:
		for lean: float in [-0.5, 0.5]:
			var foot := centre + across * side * reach + along * lean
			kit.bar(parts.steel, foot, foot + up, 0.2, tint)
		var low := centre + across * side * reach
		var brace := 1.4
		while brace + 1.3 < gantry.height:
			kit.bar(
				parts.steel,
				low - along * 0.5 + Vector3.UP * brace,
				low + along * 0.5 + Vector3.UP * (brace + 1.3),
				0.07,
				tint
			)
			brace += 1.3
	for lean: float in [-0.5, 0.5]:
		var a := centre - across * reach + up + along * lean
		var b := centre + across * reach + up + along * lean
		kit.bar(parts.steel, a, b, 0.16, tint)
		kit.bar(parts.steel, a + Vector3.UP * 0.9, b + Vector3.UP * 0.9, 0.12, tint)
	var x := -reach
	var flip := -0.5
	while x < reach:
		var foot := centre + across * x + up
		var head := centre + across * (x + 1.0) + up + Vector3.UP * 0.9
		kit.bar(parts.steel, foot + along * flip, head + along * flip, 0.06, tint)
		kit.bar(parts.steel, foot - along * flip, head - along * flip, 0.06, tint)
		x += 1.0
	for pipe: int in gantry.pipes:
		var dice := Rng.unit(gantry.dice, pipe)
		var thick := lerpf(0.18, 0.62, dice)
		var lean := lerpf(-0.65, 0.65, (pipe + 0.5) / gantry.pipes)
		var over := up + Vector3.UP * (1.05 + thick * 0.5) + along * lean
		var wall := _wall()
		kit.tube(
			parts.rust, centre - across * wall + over, centre + across * wall + over, thick * 0.5
		)
	var hung := (
		centre + up - Vector3.UP * 0.5 + across * lerpf(-2.0, 2.0, Rng.unit(gantry.dice, 20))
	)
	kit.box(parts.steel, Transform3D(_frame(route, s), hung), Vector3(0.7, 0.2, 0.4), tint * 0.5)
	kit.box(
		parts.glow,
		Transform3D(_frame(route, s), hung - Vector3.UP * 0.12),
		Vector3(0.55, 0.04, 0.28),
		Color(2.4, 1.35, 0.5)
	)
	var flood := SpotLight3D.new()
	flood.light_color = SODIUM
	flood.light_energy = 16.0
	flood.light_volumetric_fog_energy = 0.7
	flood.spot_range = gantry.height + 10.0
	flood.spot_angle = 62.0
	flood.spot_attenuation = 0.7
	flood.shadow_enabled = true
	flood.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
	flood.position = hung - Vector3.UP * 0.2
	root.add_child(flood)
	if index % STEAM_EVERY == 0:
		var cover := Vector2(s + 3.0, NeonBiome.STREET_HALF * 0.7)
		root.add_child(Weather.steam(route, cover, float(index) * 0.53))


## One side of the plaza: everything between the road's line of walls and a far wall.
static func _plaza(
	kit: CityMesh, root: Node3D, route: Route, opening: Vector3, parts: Parts
) -> void:
	var from := opening.x
	var to := opening.y
	var side := opening.z
	var wall := _wall()
	var left := minf(wall * side, PLAZA_DEPTH * side)
	var right := maxf(wall * side, PLAZA_DEPTH * side)
	StreetGround._strip(
		kit, route, from, to, left, right, StreetGround.KERB_HEIGHT, parts.pavers, 1.0
	)
	var middle := (from + to) * 0.5
	var bays := maxi(roundi((to - from) / FIN_EVERY), 1)
	var long := (to - from) / bays
	for bay: int in bays:
		var at := from + long * (bay + 0.5)
		kit.at(at)
		var out := route.right(at) * side
		_gallery_bay(kit, root, route.at(at) + out * PLAZA_DEPTH, -out, long, parts)
	var return_bays := maxi(roundi((PLAZA_DEPTH - wall) / FIN_EVERY), 1)
	var return_long := (PLAZA_DEPTH - wall) / return_bays
	for end: float in [from, to]:
		kit.at(end)
		var inward := route.tangent(end) * (1.0 if end == from else -1.0)
		var out := route.right(end) * side
		for bay: int in return_bays:
			var base := route.at(end) + out * (wall + return_long * (bay + 0.5))
			_gallery_bay(kit, root, base, inward, return_long, parts)
	_terrace(kit, root, route, from, to, side, parts)
	var drum := from + DRUM_EVERY * 0.5
	while drum < to:
		_drum(kit, root, route, drum, side, parts)
		drum += DRUM_EVERY
	_benches(kit, root, route, from, to, side, parts)
	if side < 0.0:
		_ring(kit, root, route, middle, side, parts)
		var plates: PackedScene = load(PLATES)
		_sculpture(root, route, to - SCULPTURE_IN, side, plates, PLATES_TALL, PLATES_LIT)
		var shaft := from + SHAFT_EVERY * 0.5
		var index := 0
		while shaft < to:
			var x := -9.0 if index % 2 == 0 else 8.0
			root.add_child(_sky_shaft(route.at(shaft) + route.right(shaft) * x))
			shaft += SHAFT_EVERY
			index += 1
	else:
		_monolith(root, route, middle, side)


## One bay of the housing around the plaza, floor on floor up to the deck: a street level
## that's just piers and a blind wall, and over it the modelled storeys, each its own letting.
static func _gallery_bay(
	kit: CityMesh, root: Node3D, base: Vector3, facing: Vector3, wide: float, parts: Parts
) -> void:
	var along := Vector3.UP.cross(facing).normalized()
	var basis := Basis(along, Vector3.UP, facing)
	var dice := Rng.derive(int(base.x * 7.0), [94, int(base.z * 7.0)])
	var tint := Color(0.62, 0.64, 0.68)
	kit.box(
		parts.concrete,
		Transform3D(basis, base - facing * RECESS + Vector3.UP * FLOOR_FIRST * 0.5),
		Vector3(wide + 0.1, FLOOR_FIRST, 0.6),
		tint * 0.5
	)
	for edge: float in [-0.5, 0.5]:
		var pier := base + along * edge * (wide - PIER_WIDE * 0.5) + facing * 0.2
		kit.block(
			parts.concrete,
			Transform3D(basis, pier + Vector3.UP * FLOOR_FIRST * 0.5),
			Vector3(PIER_WIDE * 0.5, FLOOR_FIRST, 2.0 + RECESS),
			tint * 0.85,
			0.06
		)
	var fitted := Basis(along * (wide / FIN_EVERY), Vector3.UP, facing)
	parts.housing.chunk = kit.chunk
	var floor_at := FLOOR_FIRST
	var storey := 0
	while floor_at + FLOOR_HIGH < Undercity.DECK_HEIGHT:
		var y := Vector3.UP * floor_at
		var pick := int(Rng.unit(dice, storey) * LETTINGS.size())
		parts.housing.place(LETTINGS[pick], Transform3D(fitted, base + y))
		if storey % 2 == 0 and Rng.unit(dice, 200 + storey) < GALLERY_LAMP:
			var lamp := OmniLight3D.new()
			lamp.light_color = UPLIGHT
			lamp.light_energy = 4.0
			lamp.light_volumetric_fog_energy = 0.15
			lamp.omni_range = 12.0
			lamp.position = base + facing * 0.2 + y + Vector3.UP * (FLOOR_HIGH - 1.0)
			root.add_child(lamp)
		floor_at += FLOOR_HIGH
		storey += 1
	if Rng.unit(dice, 302) < LINTEL_LAMP:
		_lintel_lamp(kit, root, base + facing * 1.2, basis, parts)
	if Rng.unit(dice, 300) < WASH_CHANCE:
		root.add_child(_housing_wash(base + facing * 3.0, Rng.unit(dice, 301) < 0.3))


## A caged lamp over the street level, for the stairs and the floor in front of them.
static func _lintel_lamp(
	kit: CityMesh, root: Node3D, at: Vector3, basis: Basis, parts: Parts
) -> void:
	var high := at + Vector3.UP * (FLOOR_FIRST - 0.5)
	kit.box(parts.glow, Transform3D(basis, high), Vector3(0.9, 0.08, 0.24), Color(2.2, 1.5, 0.9))
	var lamp := SpotLight3D.new()
	lamp.light_color = UPLIGHT
	lamp.light_energy = 16.0
	lamp.light_volumetric_fog_energy = 0.15
	lamp.spot_range = 20.0
	lamp.spot_angle = 68.0
	lamp.spot_attenuation = 0.7
	lamp.position = high - Vector3.UP * 0.1
	lamp.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
	root.add_child(lamp)


## A lamp on the street level's lintel, shining nearly straight up the front of the housing:
## only a light this close to the wall throws the shadows of the slabs and piers across it.
static func _housing_wash(at: Vector3, cold: bool) -> SpotLight3D:
	var wash := SpotLight3D.new()
	wash.light_color = COLD if cold else UPLIGHT
	wash.light_energy = 45.0
	wash.light_volumetric_fog_energy = 0.1
	wash.spot_range = Undercity.DECK_HEIGHT
	wash.spot_angle = 34.0
	wash.spot_attenuation = 0.8
	wash.shadow_enabled = true
	wash.position = at + Vector3.UP * FLOOR_FIRST
	wash.rotation_degrees = Vector3(90.0, 0.0, 0.0)
	return wash


static func _terrace(
	kit: CityMesh, root: Node3D, route: Route, from: float, to: float, side: float, parts: Parts
) -> void:
	var middle := (from + to) * 0.5
	var long := to - from
	var basis := _frame(route, middle)
	var deep_each := 0.42
	var high_each := TERRACE_HIGH / STAIR_STEPS
	var stairs_from := PLAZA_DEPTH - TERRACE_DEEP - STAIR_STEPS * deep_each
	kit.at(middle)
	for step: int in STAIR_STEPS:
		var rise := high_each * (step + 1)
		var off := stairs_from + (step + 0.5) * deep_each
		var place := route.at(middle) + route.right(middle) * side * off + Vector3.UP * rise * 0.5
		kit.box(
			parts.pavers,
			Transform3D(basis, place),
			Vector3(deep_each + 0.02, rise, long),
			Color(0.8, 0.8, 0.8)
		)
	var lamp_at := from + STAIR_LAMP_EVERY * 0.5
	while lamp_at < to:
		var out := route.right(lamp_at) * side * (stairs_from - 1.5)
		var lamp := OmniLight3D.new()
		lamp.light_color = UPLIGHT
		lamp.light_energy = 3.5
		lamp.light_volumetric_fog_energy = 0.1
		lamp.omni_range = 13.0
		lamp.position = route.at(lamp_at) + out + Vector3.UP * 0.8
		root.add_child(lamp)
		lamp_at += STAIR_LAMP_EVERY
	kit.at(middle)
	var top := route.right(middle) * side * (PLAZA_DEPTH - TERRACE_DEEP * 0.5)
	kit.box(
		parts.pavers,
		Transform3D(basis, route.at(middle) + top + Vector3.UP * TERRACE_HIGH * 0.5),
		Vector3(TERRACE_DEEP, TERRACE_HIGH, long),
		Color(0.8, 0.8, 0.8)
	)


## A drum holding up the deck, far bigger than it needs to look, on a stepped foot.
static func _drum(
	kit: CityMesh, root: Node3D, route: Route, s: float, side: float, parts: Parts
) -> void:
	kit.at(s)
	var base := route.at(s) + route.right(s) * side * DRUM_OFFSET
	var deck := Vector3.UP * Undercity.DECK_HEIGHT
	var tint := Color(0.6, 0.62, 0.66)
	var radius := DRUM_THICK * 0.5
	kit.tube(parts.concrete, base, base + deck, radius, tint, 20)
	kit.tube(parts.concrete, base, base + Vector3.UP * 0.9, radius + 0.5, tint * 0.75, 20)
	kit.tube(
		parts.concrete, base + deck - Vector3.UP * 2.6, base + deck, radius + 0.8, tint * 0.9, 20
	)
	var wash := OmniLight3D.new()
	wash.light_color = UPLIGHT
	wash.light_energy = 3.0
	wash.light_volumetric_fog_energy = 0.3
	wash.omni_range = 12.0
	wash.position = base - route.right(s) * side * (radius + 1.6) + Vector3.UP * 0.5
	root.add_child(wash)


## Concrete benches along the road's edge, lit from under the seat, for a sense of how big
## the rest of it is.
static func _benches(
	kit: CityMesh, root: Node3D, route: Route, from: float, to: float, side: float, parts: Parts
) -> void:
	var s := from + 9.0
	var index := 0
	while s < to - 6.0:
		kit.at(s)
		var basis := _frame(route, s)
		var out := route.right(s) * side
		var at := route.at(s) + out * BENCH_OFFSET + Vector3.UP * StreetGround.KERB_HEIGHT
		var seat := at + Vector3.UP * 0.42
		kit.block(
			parts.concrete,
			Transform3D(basis, seat),
			Vector3(0.7, 0.16, BENCH_LONG),
			Color(0.62, 0.64, 0.66),
			0.03
		)
		for foot: float in [-0.35, 0.35]:
			kit.box(
				parts.concrete,
				Transform3D(basis, at + basis.z * foot * BENCH_LONG + Vector3.UP * 0.17),
				Vector3(0.5, 0.34, 0.3),
				Color(0.5, 0.52, 0.54)
			)
		kit.box(
			parts.glow,
			Transform3D(basis, at + Vector3.UP * 0.32),
			Vector3(0.05, 0.03, BENCH_LONG * 0.6),
			Color(1.8, 1.2, 0.6)
		)
		if index % 2 == 0:
			var pool := OmniLight3D.new()
			pool.light_color = UPLIGHT
			pool.light_energy = 0.8
			pool.light_volumetric_fog_energy = 0.0
			pool.omni_range = 5.0
			pool.position = at + Vector3.UP * 0.25
			root.add_child(pool)
		index += 1
		s += BENCH_EVERY


## A colossal ring standing in a black pool, the only thing here that's still kept up.
## Set across the road's line, so it's a ring and not a stick from the saddle.
static func _ring(
	kit: CityMesh, root: Node3D, route: Route, s: float, side: float, parts: Parts
) -> void:
	kit.at(s)
	var across := route.right(s)
	var along := route.tangent(s)
	var face := (along * cos(RING_TURN) + across * sin(RING_TURN)).normalized()
	var centre := route.at(s) + across * side * RING_OFFSET
	var ground := Vector3.UP * StreetGround.KERB_HEIGHT
	var hub := centre + ground + Vector3.UP * (RING_RADIUS + 0.2)
	var previous := hub + face * RING_RADIUS
	for piece: int in range(1, RING_PIECES + 1):
		var turn := TAU * piece / RING_PIECES
		var point := hub + face * cos(turn) * RING_RADIUS + Vector3.UP * sin(turn) * RING_RADIUS
		kit.tube(parts.concrete, previous, point, 0.62, Color(0.72, 0.75, 0.8), 10)
		var band := previous.lerp(hub, 0.065)
		kit.tube(parts.glow, band, point.lerp(hub, 0.065), 0.09, RING_BLUE, 6)
		previous = point
	var rim_high := 0.3
	var pool_long := RING_RADIUS * 2.0 + 8.0
	var basis := _frame(route, s)
	var water := centre + ground + Vector3.UP * rim_high * 0.7
	kit.box(parts.pool, Transform3D(basis, water), Vector3(POOL_WIDE, 0.04, pool_long), Color.WHITE)
	var rim := centre + ground + Vector3.UP * rim_high * 0.5
	for edge: float in [-1.0, 1.0]:
		kit.block(
			parts.concrete,
			Transform3D(basis, rim + across * edge * POOL_WIDE * 0.5),
			Vector3(0.5, rim_high, pool_long + 0.5),
			Color(0.6, 0.62, 0.64),
			0.03
		)
		kit.block(
			parts.concrete,
			Transform3D(basis, rim + along * edge * pool_long * 0.5),
			Vector3(POOL_WIDE, rim_high, 0.5),
			Color(0.6, 0.62, 0.64),
			0.03
		)
	var inside := OmniLight3D.new()
	inside.light_color = Color(0.4, 0.72, 1.0)
	inside.light_energy = 5.0
	inside.light_volumetric_fog_energy = 0.25
	inside.omni_range = 30.0
	inside.position = hub
	root.add_child(inside)


## A slab leaning out over the plaza, turned towards the oncoming rider.
static func _monolith(root: Node3D, route: Route, s: float, side: float) -> void:
	var out := route.right(s) * side
	var facing := (-route.tangent(s) * 0.8 - out * 0.6).normalized()
	var foot := route.at(s) + out * 21.0 + Vector3.UP * StreetGround.KERB_HEIGHT
	_raise(root, load(MONOLITH) as PackedScene, foot, facing, MONOLITH_TALL)


## A sculpture on the plaza floor, set back from the road and looking at whoever is coming.
static func _sculpture(
	root: Node3D, route: Route, s: float, side: float, piece: PackedScene, tall: float, lit: float
) -> void:
	var out := route.right(s) * side
	var facing := (-route.tangent(s) * 0.7 - out * 0.7).normalized()
	var foot := route.at(s) + out * SCULPTURE_OFFSET + Vector3.UP * StreetGround.KERB_HEIGHT
	_raise(root, piece, foot, facing, tall, lit)


## Stands a piece on the ground, with lamps set right against it, warm from one side and cold
## from the other, since only a raking light shows relief.
static func _raise(
	root: Node3D, piece: PackedScene, foot: Vector3, facing: Vector3, tall: float, lit := 1.0
) -> void:
	var along := Vector3.UP.cross(facing).normalized()
	var standing: Node3D = piece.instantiate()
	standing.transform = Transform3D(Basis(along, Vector3.UP, facing), foot)
	root.add_child(standing)
	var size := tall / MONOLITH_TALL
	for lamp: Array in [
		[facing * 6.5 - along * 3.0, 70.0, Color(1.0, 0.84, 0.68)],
		[facing * 5.0 + along * 9.0, 60.0, COLD],
	]:
		var wash := SpotLight3D.new()
		wash.light_color = lamp[2]
		wash.light_energy = lamp[1] * size * lit
		wash.light_volumetric_fog_energy = 0.1
		wash.spot_range = 50.0 * sqrt(size)
		wash.spot_angle = 38.0
		wash.shadow_enabled = true
		var offset: Vector3 = lamp[0]
		wash.position = foot + offset * size + Vector3.UP * 0.5
		root.add_child(wash)
		wash.look_at_from_position(wash.position, foot + Vector3.UP * tall * 0.54)


## What daylight reaches down this far, wide enough to stand in.
static func _sky_shaft(ground: Vector3) -> SpotLight3D:
	var beam := SpotLight3D.new()
	beam.light_color = COLD
	beam.light_energy = 10.0
	beam.light_volumetric_fog_energy = 1.2
	beam.light_specular = 0.3
	beam.spot_range = Undercity.DECK_HEIGHT + 30.0
	beam.spot_angle = 30.0
	beam.spot_attenuation = 0.5
	beam.shadow_enabled = true
	beam.rotation_degrees = Vector3(-90.0, 0.0, 0.0)
	beam.position = ground + Vector3.UP * (Undercity.DECK_HEIGHT + 12.0)
	return beam
