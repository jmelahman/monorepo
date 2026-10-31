class_name Pedestrians
extends Node3D
## Draws [Crowd] as one quad a person, with a batch for each baked person since each has
## its own sheet. Only those near the rider get moved or drawn.

const META := "res://assets/baked/people.json"
## Past this they're lost in the fog and each other.
const RANGE := 150.0
## Above the road, up on the pavement.
const STANDS_ON := StreetGround.KERB_HEIGHT + 0.005
const BLOB_SIZE := 0.95
## How long someone standing takes to shift their weight and back, in seconds.
const STAND_CYCLE := 9.0
const FLOATS_PER_PERSON := 16
const FLOATS_PER_BLOB := 12

var _crowd: Crowd
var _route: Route
var _batches: Array[MultiMesh] = []
var _strides: Array[float] = []
var _by_look: Array[Array] = []
var _blobs: MultiMesh


static func build(run_seed: int, route: Route, street: Street, length: float) -> Pedestrians:
	var parsed: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(META))
	var names: Array = parsed["people"]
	var crowd := Crowd.generate(
		run_seed, street, -Street.REACH, length + Street.REACH, NeonBiome.STREET_HALF, names.size()
	)
	var built := Pedestrians.new()
	built.add_to_group(&"ride_clock")
	built._setup(route, crowd, parsed)
	return built


func _setup(route: Route, crowd: Crowd, meta: Dictionary) -> void:
	_route = route
	_crowd = crowd
	var names: Array = meta["people"]
	var strides: Array = meta["strides"]
	var height: float = meta["height"]
	var cell: Array = meta["cell"]
	var cell_wide: float = cell[0]
	var cell_high: float = cell[1]
	var width := height * cell_wide / cell_high
	var shader: Shader = load("res://game/person.gdshader")
	var quad := QuadMesh.new()
	quad.size = Vector2(width, height)
	quad.center_offset = Vector3(0.0, height * 0.5, 0.0)
	for look: int in names.size():
		var person_name: String = names[look]
		var material := ShaderMaterial.new()
		material.shader = shader
		material.set_shader_parameter(
			"albedo", load("res://assets/baked/person_%s.webp" % person_name)
		)
		material.set_shader_parameter(
			"normals", load("res://assets/baked/person_%s_n.webp" % person_name)
		)
		material.set_shader_parameter("walk_frames", meta["walk_frames"])
		material.set_shader_parameter("stand_frames", meta["stand_frames"])
		material.set_shader_parameter("views", meta["views"])
		var batch := MultiMesh.new()
		batch.transform_format = MultiMesh.TRANSFORM_3D
		batch.use_custom_data = true
		batch.mesh = quad
		_batches.append(batch)
		_by_look.append([])
		var stride: float = strides[look]
		_strides.append(stride)
		_add_batch(batch, material)
	for person: Crowd.Person in crowd.people:
		_by_look[person.look].append(person)
	for look: int in names.size():
		_batches[look].instance_count = _by_look[look].size()
		_batches[look].visible_instance_count = 0
	_add_blobs(crowd.people.size())


func _add_batch(batch: MultiMesh, material: Material) -> void:
	var instance := MultiMeshInstance3D.new()
	instance.multimesh = batch
	instance.material_override = material
	instance.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	# They move, so they're never where a box drawn round them at build time would say.
	instance.custom_aabb = AABB(Vector3.ONE * -2000.0, Vector3.ONE * 4000.0)
	add_child(instance)


## A soft dark patch under each person, which is most of what stands them on the ground.
func _add_blobs(count: int) -> void:
	var plane := PlaneMesh.new()
	plane.size = Vector2.ONE * BLOB_SIZE
	var shader: Shader = load("res://game/person_blob.gdshader")
	var material := ShaderMaterial.new()
	material.shader = shader
	_blobs = MultiMesh.new()
	_blobs.transform_format = MultiMesh.TRANSFORM_3D
	_blobs.mesh = plane
	_blobs.instance_count = count
	_blobs.visible_instance_count = 0
	_add_batch(_blobs, material)


## Called with the ride's clock once it has moved the camera.
func set_ride_time(time: float) -> void:
	var eye := get_viewport().get_camera_3d().global_position
	var eye_s := _route.nearest(eye.x, eye.z).x
	var blob_data := PackedFloat32Array()
	blob_data.resize(_blobs.instance_count * FLOATS_PER_BLOB)
	var blobs := 0
	for look: int in _batches.size():
		var list: Array = _by_look[look]
		var data := PackedFloat32Array()
		data.resize(list.size() * FLOATS_PER_PERSON)
		var drawn := 0
		for item: Crowd.Person in list:
			var s := _crowd.s_at(item, time)
			if absf(s - eye_s) > RANGE:
				continue
			var ground := _route.at(s) + _route.right(s) * item.offset
			ground.y += STANDS_ON
			var along := _route.tangent(s) * item.direction
			var heading := atan2(along.x, along.z) + item.turn
			var phase: float
			if item.speed > 0.0:
				phase = item.phase + item.speed * time / (_strides[look] * item.height)
			else:
				phase = item.phase + time / STAND_CYCLE
			var at := drawn * FLOATS_PER_PERSON
			data[at] = 1.0
			data[at + 3] = ground.x
			data[at + 5] = 1.0
			data[at + 7] = ground.y
			data[at + 10] = 1.0
			data[at + 11] = ground.z
			data[at + 12] = heading
			data[at + 13] = fposmod(phase, 1.0)
			data[at + 14] = item.height
			data[at + 15] = 1.0 if item.speed > 0.0 else 0.0
			drawn += 1
			var blob := blobs * FLOATS_PER_BLOB
			blob_data[blob] = 1.0
			blob_data[blob + 3] = ground.x
			blob_data[blob + 5] = 1.0
			blob_data[blob + 7] = ground.y
			blob_data[blob + 10] = 1.0
			blob_data[blob + 11] = ground.z
			blobs += 1
		_batches[look].buffer = data
		_batches[look].visible_instance_count = drawn
	_blobs.buffer = blob_data
	_blobs.visible_instance_count = blobs
