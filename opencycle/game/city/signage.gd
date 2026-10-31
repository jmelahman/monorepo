class_name Signage
extends RefCounted
## Hangs the street's signs: a lit face from the sheet, in a steel frame, on arms back to
## the wall they hang from. The frame and arms are why a sign reads as a real thing.

const SHEET := "res://assets/baked/signs.webp"
const SHADER: Shader = preload("res://game/sign.gdshader")
const BOX_DEPTH := 0.22
const SPILL_SPACING := 8.0
const SPILL_BEHIND := 15.0
const SPILL_AHEAD := 45.0
const COOL := Color(0.1, 0.8, 1.0)
const HOT := Color(1.0, 0.15, 0.5)


static func build(route: Route, street: Street, length: float) -> Node3D:
	var root := Node3D.new()
	root.name = "Signs"
	var sheet := SignSheet.load_sheet()
	var kit := CityMesh.new()
	var face := ShaderMaterial.new()
	face.shader = SHADER
	face.set_shader_parameter("sheet", load(SHEET))
	var steel := StandardMaterial3D.new()
	steel.albedo_color = Color(0.11, 0.11, 0.12)
	steel.roughness = 0.6
	steel.metallic = 0.5
	var brightest: Dictionary[Vector2i, Street.Sign] = {}
	for index: int in street.signs.size():
		var hung := street.signs[index]
		kit.at(hung.s)
		_hang(kit, route, hung, sheet, face, steel, float(index % 251) / 251.0)
		if hung.overhang > 0.0 and hung.s > -SPILL_BEHIND and hung.s < length + SPILL_AHEAD:
			var cell := Vector2i(floori(hung.s / SPILL_SPACING), int(hung.side))
			if cell not in brightest or brightest[cell].size.x < hung.size.x:
				brightest[cell] = hung
	kit.commit(root)
	for cell: Vector2i in brightest:
		root.add_child(_spill(route, brightest[cell], sheet))
	return root


## The colour a big sign throws on everything near it, which is what ties the signs to
## the street. One for each short stretch: every sign having its own would cost too much.
static func _spill(route: Route, hung: Street.Sign, sheet: SignSheet) -> OmniLight3D:
	var pick := sheet.pick(hung.trade, hung.tall, hung.pick)
	var outward := route.right(hung.s) * hung.side
	# Well off the wall, or the wall behind gets a hot spot where the sign should just tint it.
	var reach := minf(
		NeonBiome.STREET_HALF + Street.SIDEWALK - hung.overhang + hung.size.x * 0.5,
		NeonBiome.STREET_HALF
	)
	var glow := OmniLight3D.new()
	var colour: Color = pick["glow"]
	# Average colours of a sign are muddy, while the eye only sees what's saturated.
	var own := Color.from_hsv(colour.h, clampf(colour.s * 1.6, 0.6, 0.95), 1.0)
	# One side of the street leans cool and the other hot, whatever the signs say: the two
	# colours playing against each other are what keep a frame from going all one pink.
	glow.light_color = own.lerp(COOL if hung.side < 0.0 else HOT, 0.45)
	glow.light_energy = 1.2 + hung.size.x * hung.size.y * 0.2
	glow.light_specular = 0.5
	glow.light_volumetric_fog_energy = 1.2
	glow.omni_range = 14.0 + hung.size.x * 2.5
	glow.omni_attenuation = 1.3
	glow.position = (
		route.at(hung.s)
		+ outward * reach
		- route.tangent(hung.s) * 0.8
		+ Vector3.UP * (hung.elevation + hung.size.y * 0.5)
	)
	return glow


static func _hang(
	kit: CityMesh,
	route: Route,
	hung: Street.Sign,
	sheet: SignSheet,
	face: Material,
	steel: Material,
	seed_value: float
) -> void:
	var along := route.tangent(hung.s)
	var outward := route.right(hung.s) * hung.side
	var wall_off := NeonBiome.STREET_HALF + Street.SIDEWALK
	var ground := route.at(hung.s)
	var pick := sheet.pick(hung.trade, hung.tall, hung.pick)
	var mapped: Rect2 = pick["uv"]
	var colour := Color(lerpf(0.75, 1.1, seed_value), hung.flicker, seed_value)
	var y := ground.y + hung.elevation
	var size := hung.size
	var top_uv := mapped.position.y
	var low_uv := mapped.end.y
	var uvs: Array[Vector2] = [
		Vector2(mapped.position.x, low_uv),
		Vector2(mapped.end.x, low_uv),
		Vector2(mapped.end.x, top_uv),
		Vector2(mapped.position.x, top_uv),
	]
	# The same picture for corners laid out the other way round as seen from the road.
	var mirrored: Array[Vector2] = [uvs[1], uvs[0], uvs[3], uvs[2]]
	if hung.overhang <= 0.0:
		# Flat on the fascia, facing the road, on a shallow box.
		var at := ground + outward * (wall_off - 0.12)
		var half := along * size.x * 0.5
		var a := Vector3(at.x, y, at.z)
		var corners: Array[Vector3] = [
			a - half, a + half, a + half + Vector3.UP * size.y, a - half + Vector3.UP * size.y
		]
		kit.face(face, corners, mirrored if hung.side > 0.0 else uvs, -outward, colour)
		return
	# Facing back down the road, towards whoever is riding up it.
	var inner := wall_off - hung.overhang
	var at_inner := ground + outward * inner
	var low := Vector3(at_inner.x, y, at_inner.z)
	var across := outward * size.x
	var up := Vector3.UP * size.y
	var front := low
	# Read left to right from the rider's side, whichever side of the road it hangs on.
	kit.face(
		face,
		[front, front + across, front + across + up, front + up],
		uvs if hung.side > 0.0 else mirrored,
		-along,
		colour
	)
	# The box behind the face, and the arms that carry it back to the wall.
	var middle := low + across * 0.5 + up * 0.5 + along * BOX_DEPTH * 0.52
	var basis := Basis(outward, Vector3.UP, along)
	kit.box(steel, Transform3D(basis, middle), Vector3(size.x + 0.08, size.y + 0.08, BOX_DEPTH))
	var wall_point := ground + outward * wall_off
	var behind := along * BOX_DEPTH * 0.5
	for height: float in [0.18, 0.82]:
		var arm_y := y + size.y * height
		var from := low + across * 0.55
		kit.bar(
			steel,
			Vector3(from.x, arm_y, from.z) + behind,
			Vector3(wall_point.x, arm_y, wall_point.z) + behind,
			0.05
		)
	# A brace: a heavy sign doesn't hang from horizontal arms alone.
	kit.bar(
		steel,
		Vector3(low.x, y + size.y, low.z) + across * 0.6 + behind,
		Vector3(wall_point.x, y + size.y + 1.0, wall_point.z) + behind,
		0.035
	)
