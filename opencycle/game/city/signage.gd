class_name Signage
extends RefCounted
## Hangs the street's signs: a lit face from the sheet, in a steel frame, on arms back to
## the wall they hang from. The frame and arms are why a sign reads as a real thing.

const SHEET := "res://assets/baked/signs.webp"
const SHADER: Shader = preload("res://game/sign.gdshader")
const BOX_DEPTH := 0.22


static func build(route: Route, street: Street) -> Node3D:
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
	for index: int in street.signs.size():
		var hung := street.signs[index]
		kit.at(hung.s)
		_hang(kit, route, hung, sheet, face, steel, float(index % 251) / 251.0)
	kit.commit(root)
	return root


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
