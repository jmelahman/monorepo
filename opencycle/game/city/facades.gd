class_name Facades
extends RefCounted
## The buildings above the shops: stepped boxes in real wall finishes, with windows the
## shader cuts into them, and air-con units hung on the front.

const SHADER: Shader = preload("res://game/facade.gdshader")
const FINISHES: Array[Array] = [
	["rectangular_facade_tiles", 2.0],
	["square_tiled_wall", 1.6],
	["worn_plaster_wall", 3.0],
	["concrete_wall_004", 3.0],
]
const AIRCON := "res://assets/polyhaven/exterior_aircon_unit/exterior_aircon_unit_1k.gltf"
const GROUND_FLOOR := 4.1
const BELOW := 8.0
const KIT_DIR := "res://assets/polyhaven/modular_urban_apartments_facade/"
const KIT := KIT_DIR + "modular_urban_apartments_facade_1k.gltf"
const KIT_TEXTURES := KIT_DIR + "textures/modular_urban_apartments_facade_%s_%s_1k.jpg"
const ESCAPE := "res://assets/polyhaven/modular_fire_escape/modular_fire_escape_1k.gltf"
const KIT_FINISH: Shader = preload("res://game/kit_finish.gdshader")
const WINDOW: Shader = preload("res://game/window_room.gdshader")
## How wide the kit's modules are, before they're stretched to fit a building.
const MODULE := 3.0
## How far the tower stands back from the front of the block at its foot.
const SETBACK := 1.2
## Past this the block is a few pixels of detail, and the flat wall behind it takes over.
const KIT_REACH := 170.0
## Window styles: wall and window part names, from the bottom floor up.
const WINDOWS: Array[Array] = [
	["centered_large_01", "centered_large_02", "centered_large_03"],
	["centered_double_01", "centered_double_02", "centered_double_03"],
	["centered_small_01", "centered_small_02", "centered_small_03"],
	["offset_small_01", "offset_small_03", "offset_small_05"],
]


static func build(route: Route, street: Street) -> Node3D:
	var root := Node3D.new()
	root.name = "Facades"
	var kit := CityMesh.new()
	var materials: Array[ShaderMaterial] = []
	for finish: Array in FINISHES:
		var texture: String = finish[0]
		var tile: float = finish[1]
		materials.append(_finish(texture, tile))
	var units: Array[Transform3D] = []
	var frame := StandardMaterial3D.new()
	frame.albedo_color = Color(0.1, 0.1, 0.1)
	frame.roughness = 0.7
	var block := _block_kit()
	var escape := Kit.new(ESCAPE)
	var tube := StandardMaterial3D.new()
	tube.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	tube.vertex_color_use_as_albedo = true
	var metal := StandardMaterial3D.new()
	metal.albedo_color = Color(0.16, 0.15, 0.14)
	metal.vertex_color_use_as_albedo = true
	metal.metallic = 0.7
	metal.roughness = 0.45
	for tower: Street.Tower in street.towers:
		kit.at(tower.s)
		block.at(tower.s)
		escape.at(tower.s)
		_tower(kit, route, tower, materials[tower.facade], frame)
		_block(kit, block, escape, route, tower, materials[tower.facade], tube, metal, units)
	kit.commit(root)
	block.commit(root, KIT_REACH)
	escape.commit(root, KIT_REACH)
	Furniture.instances(root, AIRCON, units)
	return root


static func _tower(
	kit: CityMesh, route: Route, tower: Street.Tower, material: Material, frame: Material
) -> void:
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var base := route.at(tower.s)
	var front := base + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	var colour := Color(tower.tint.r, tower.tint.g, tower.tint.b, tower.age)
	var layout := Vector2(tower.bay, tower.storey)
	var rise := tower.height / (tower.tiers + 1)
	var width := tower.width
	var depth := tower.depth
	var bottom := GROUND_FLOOR
	for tier: int in tower.tiers + 1:
		var plan := front + outward * (SETBACK + tier * 3.0)
		var left := plan - along * width * 0.5
		var right := plan + along * width * 0.5
		var top := bottom + rise
		_wall(kit, material, left, right, base.y, bottom, top, -outward, colour, layout)
		var back := outward * depth
		# The sides are party walls, blank, and run down below the neighbours' roofs.
		var low := base.y - BELOW if tier == 0 else base.y + bottom
		var from := low - base.y
		_wall(kit, material, left + back, left, base.y, from, top, -along, colour, Vector2.ZERO)
		_wall(kit, material, right, right + back, base.y, from, top, along, colour, Vector2.ZERO)
		var roof := Vector3.UP * (base.y + top - plan.y)
		kit.quad(
			frame, left + roof, right + roof, right + back + roof, left + back + roof, Vector3.UP
		)
		bottom = top
		width = maxf(width - 4.0, 6.0)
		depth = maxf(depth - 6.0, 6.0)


## The old block at the tower's foot, in real geometry, with everything that's been bolted
## to it since: tube lighting, security cages, drainpipes, fire escapes and air-con.
static func _block(
	kit: CityMesh,
	parts: Kit,
	escape: Kit,
	route: Route,
	tower: Street.Tower,
	finish: Material,
	tube: Material,
	metal: Material,
	units: Array[Transform3D]
) -> void:
	if tower.block < 1:
		return
	var along := route.tangent(tower.s)
	var outward := route.right(tower.s) * tower.side
	var base := route.at(tower.s)
	var front := base + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	front.y = base.y
	var facing := -outward
	var across := Vector3.UP.cross(facing).normalized()
	var modules := maxi(roundi(tower.width / MODULE), 1)
	var wide := tower.width / modules
	var stretch := wide / MODULE
	var basis := Basis(across * stretch, Vector3.UP, facing)
	var top := GROUND_FLOOR + tower.block * Street.BLOCK_STOREY
	var style: Array = WINDOWS[tower.windows]
	var other: Array = WINDOWS[(tower.windows + 1 + tower.rhythm) % WINDOWS.size()]
	for column: int in modules:
		var centre := front + along * ((column + 0.5) * wide - tower.width * 0.5)
		# The kit's modules run from x -3 to 0, so a module starts half a width to one side.
		var start := centre + across * wide * 0.5
		var plain := tower.rhythm > 1 and column % (tower.rhythm + 1) == tower.rhythm
		for storey: int in tower.block:
			var dice := Rng.derive(tower.dice, [column, storey])
			var level := 0 if storey == 0 else (2 if storey == tower.block - 1 else 1)
			var names: Array = other if column % 2 == 1 and tower.rhythm > 1 else style
			var name: String = names[level]
			var here := start + Vector3.UP * (GROUND_FLOOR + storey * Street.BLOCK_STOREY)
			var at := Transform3D(basis, here)
			if plain:
				parts.place("wall_standard_standard_01", at, tower.paint)
				continue
			parts.place("wall_window_%s" % name, at, tower.paint)
			parts.place(
				"window_%s" % name, at, tower.trim, Color(Rng.unit(dice, 0), Rng.unit(dice, 1), 0.0)
			)
			var mid := centre + Vector3.UP * (GROUND_FLOOR + storey * Street.BLOCK_STOREY)
			if Rng.unit(dice, 2) < lerpf(0.04, 0.3, tower.age):
				_cage(kit, metal, mid, across, facing, wide)
			if Rng.unit(dice, 3) < lerpf(0.08, 0.4, tower.age):
				units.append(
					Transform3D(
						Basis(along, Vector3.UP, facing), mid + Vector3.UP * 2.42 + facing * 0.3
					)
				)
		var crown := Transform3D(basis, start + Vector3.UP * top)
		parts.place("crown_standard_standard_01", crown, tower.trim)
		var sill := Transform3D(basis, start + Vector3.UP * (GROUND_FLOOR - 0.2))
		parts.place("cornice_standard_standard_01", sill, tower.trim)
	var left := front - along * tower.width * 0.5
	var right := front + along * tower.width * 0.5
	var low := Vector3.UP * GROUND_FLOOR
	var high := Vector3.UP * top
	# The block's ends and roof, closing it off where it stands forward of the tower.
	var deep := outward * SETBACK
	kit.quad(
		finish,
		left + low,
		left + low + deep,
		left + high + deep,
		left + high,
		-along,
		Color(tower.paint, 0.5)
	)
	kit.quad(
		finish,
		right + low + deep,
		right + low,
		right + high,
		right + high + deep,
		along,
		Color(tower.paint, 0.5)
	)
	kit.quad(
		metal,
		left + high,
		right + high,
		right + high + deep,
		left + high + deep,
		Vector3.UP,
		Color.WHITE
	)
	# The tube under the crown, and down the corners on the landlords who went all in.
	var glow := Color(tower.tube.r * 3.2, tower.tube.g * 3.2, tower.tube.b * 3.2)
	var lip := facing * 0.2 - Vector3.UP * 0.1
	kit.bar(tube, left + high + lip, right + high + lip, 0.07, glow)
	if tower.tube_corners:
		var inset := along * 0.12
		kit.bar(tube, left + inset + low + lip, left + inset + high + lip, 0.07, glow)
		kit.bar(tube, right - inset + low + lip, right - inset + high + lip, 0.07, glow)
	# A drainpipe, with a cable run beside it.
	var pipe := left + along * 0.3 + facing * 0.09
	kit.bar(metal, pipe + low, pipe + high, 0.1, Color(0.8, 0.8, 0.8))
	kit.bar(
		metal, pipe + along * 0.16 + low, pipe + along * 0.16 + high, 0.035, Color(0.5, 0.5, 0.5)
	)
	if tower.fire_escape and modules >= 2:
		_fire_escape(escape, route, tower, facing, across)


## A cage of steel bars over a window, the kind a street gets when it stops feeling safe.
static func _cage(
	kit: CityMesh, metal: Material, foot: Vector3, across: Vector3, facing: Vector3, wide: float
) -> void:
	var half := across * wide * 0.36
	var low := foot + facing * 0.42 + Vector3.UP * 0.15
	var high := low + Vector3.UP * 2.45
	var bars := 7
	for i: int in bars:
		var at := half * (2.0 * i / (bars - 1.0) - 1.0)
		kit.bar(metal, low + at, high + at, 0.025)
	for y: float in [0.0, 0.8, 1.6, 2.45]:
		var rail := low + Vector3.UP * y
		kit.bar(metal, rail - half, rail + half, 0.03)
		kit.bar(metal, rail - half, rail - half - facing * 0.4, 0.03)
		kit.bar(metal, rail + half, rail + half - facing * 0.4, 0.03)


## Landings and stairs up the front, one flight to a storey, from above the shop signs.
static func _fire_escape(
	escape: Kit, route: Route, tower: Street.Tower, facing: Vector3, across: Vector3
) -> void:
	var outward := -facing
	var along := route.tangent(tower.s)
	var base := route.at(tower.s)
	var front := base + outward * (NeonBiome.STREET_HALF + Street.SIDEWALK)
	front.y = base.y
	var offset := (Rng.unit(tower.dice, 900) - 0.5) * maxf(tower.width - 6.0, 0.0)
	var landing := "modular_fire_escape_platform_middle"
	var railing := "modular_fire_escape_railing_middle"
	var stairs := "modular_fire_escape_stairs"
	var deck := escape.bounds(landing)
	var basis := Basis(across, Vector3.UP, facing)
	var squash := Basis(
		across, Vector3.UP * (Street.BLOCK_STOREY / escape.bounds(stairs).size.y), facing
	)
	# The kit's wall side is its low z, so that's what goes against the building.
	var stand := front + along * offset + facing * (-deck.position.z)
	for storey: int in range(1, tower.block):
		var floor_y := GROUND_FLOOR + storey * Street.BLOCK_STOREY - 0.15
		var deck_top := deck.position.y + deck.size.y
		var here := stand + Vector3.UP * (floor_y - deck_top)
		escape.place(landing, Transform3D(basis, here))
		var rail := escape.bounds(railing)
		var rail_nudge := across * (escape.origin(railing).x - escape.origin(landing).x)
		var rail_z := facing * (escape.origin(railing).z - escape.origin(landing).z)
		escape.place(
			railing,
			Transform3D(
				basis, here + rail_nudge + rail_z + Vector3.UP * (deck_top - rail.position.y)
			)
		)
		if storey > 1:
			var flight := escape.bounds(stairs)
			var stair_z := facing * (escape.origin(stairs).z - escape.origin(landing).z)
			var scale_y := Street.BLOCK_STOREY / flight.size.y
			var stair_y := floor_y - Street.BLOCK_STOREY - flight.position.y * scale_y
			var at := Vector3(here.x, stair_y, here.z) + stair_z
			escape.place(stairs, Transform3D(squash, at))


static func _block_kit() -> Kit:
	var parts := Kit.new(KIT)
	var grime := {"plaster": 0.75, "trim_01": 0.5, "trim_02": 0.5, "objects": 0.3}
	var materials: Dictionary[String, Material] = {}
	for key: String in grime:
		var material := ShaderMaterial.new()
		material.shader = KIT_FINISH
		material.set_shader_parameter("albedo", load(KIT_TEXTURES % [key, "diff"]))
		material.set_shader_parameter("normals", load(KIT_TEXTURES % [key, "nor_gl"]))
		material.set_shader_parameter("packed", load(KIT_TEXTURES % [key, "arm"]))
		material.set_shader_parameter("grime", grime[key])
		materials["modular_urban_apartments_facade_%s" % key] = material
	var glass := ShaderMaterial.new()
	glass.shader = WINDOW
	materials["modular_urban_apartments_facade_glass"] = glass
	parts.restyle(materials)
	return parts


static func _wall(
	kit: CityMesh,
	material: Material,
	from: Vector3,
	to: Vector3,
	ground: float,
	low: float,
	high: float,
	facing: Vector3,
	colour: Color,
	layout: Vector2
) -> void:
	var a := Vector3(from.x, ground + low, from.z)
	var b := Vector3(to.x, ground + low, to.z)
	var c := Vector3(to.x, ground + high, to.z)
	var d := Vector3(from.x, ground + high, from.z)
	var span := (to - from).length()
	kit.face(
		material,
		[a, b, c, d],
		[Vector2(0.0, low), Vector2(span, low), Vector2(span, high), Vector2(0.0, high)],
		facing,
		colour,
		layout
	)


static func _finish(texture: String, tile: float) -> ShaderMaterial:
	var path := "res://assets/polyhaven/%s/%s_%%s_2k.jpg" % [texture, texture]
	var material := ShaderMaterial.new()
	material.shader = SHADER
	material.set_shader_parameter("albedo", load(path % "diffuse"))
	material.set_shader_parameter("normals", load(path % "nor_gl"))
	material.set_shader_parameter("rough", load(path % "rough"))
	material.set_shader_parameter("tile", tile)
	material.set_shader_parameter("first_storey", GROUND_FLOOR)
	return material
