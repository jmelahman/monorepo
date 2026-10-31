extends SceneTree
## Bakes Rocketbox people into sheets of a walk cycle seen from every side, so a crowd is
## one quad each. Colour and normals both, so shop and sign light still falls on them.
## Columns are the walk's frames then two of standing about; rows are the views.
## The sources are fetched by scripts/fetch_people.py. Needs a real GPU.

const SOURCE := "res://assets/bake-sources/rocketbox/"
const OUT := "res://assets/baked/"
const CELL := Vector2i(200, 360)
const WALK_FRAMES := 8
const STAND_FRAMES := 2
const VIEWS := 8
## Metres of person the cell's height shows, from the ground up.
const HEIGHT := 1.9
const WEBP_QUALITY := 0.88
## What the avatars face in their own space, as a turn about up from +Z.
const FORWARD_YAW := 0.0
const NORMAL_SHADER := """
shader_type spatial;
render_mode unshaded;
uniform sampler2D mask : filter_linear_mipmap;
void fragment() {
	// The viewport will encode this as sRGB, and the sheet has to hold it as plain numbers.
	ALBEDO = pow(normalize(NORMAL) * 0.5 + 0.5, vec3(2.2));
	ALPHA = texture(mask, UV).a;
	ALPHA_SCISSOR_THRESHOLD = 0.5;
}
"""
## Each avatar and the clips that suit it, in order: a walk, then standing.
const PEOPLE: Array[Array] = [
	["Female_Adult_01", "f_walk_neutral_01", "f_idle_neutral_01"],
	["Female_Adult_02", "f_walk_stroll_01", "f_idle_neutral_01"],
	["Female_Adult_05", "f_walk_neutral_02", "f_idle_neutral_01"],
	["Female_Adult_08", "f_walk_neutral_01", "f_idle_neutral_01"],
	["Female_Adult_11", "f_walk_stroll_01", "f_idle_neutral_01"],
	["Female_Adult_14", "f_walk_neutral_02", "f_idle_neutral_01"],
	["Female_Party_01", "f_walk_neutral_01", "f_idle_neutral_01"],
	["Male_Adult_01", "m_walk_neutral_01", "m_cell_phone_textmessage"],
	["Male_Adult_03", "m_walk_stroll_01", "m_idle_neutral_01"],
	["Male_Adult_05", "m_walk_neutral_02", "m_cell_phone_textmessage"],
	["Male_Adult_08", "m_walk_neutral_01", "m_idle_neutral_01"],
	["Male_Adult_11", "m_walk_stroll_01", "m_cell_phone_textmessage"],
	["Male_Adult_14", "m_walk_neutral_02", "m_idle_neutral_01"],
	["Male_Adult_17", "m_walk_neutral_01", "m_idle_neutral_01"],
]

var _strides: Array[float] = []


func _initialize() -> void:
	_bake.call_deferred()


func _bake() -> void:
	DisplayServer.window_set_vsync_mode(DisplayServer.VSYNC_DISABLED)
	var viewport := SubViewport.new()
	viewport.size = CELL
	viewport.own_world_3d = true
	viewport.transparent_bg = true
	viewport.msaa_3d = Viewport.MSAA_4X
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(viewport)
	var camera := Camera3D.new()
	camera.projection = Camera3D.PROJECTION_ORTHOGONAL
	camera.size = HEIGHT
	camera.position = Vector3(0.0, HEIGHT * 0.5, 5.0)
	viewport.add_child(camera)
	var names: Array[String] = []
	for person: Array in PEOPLE:
		var avatar: String = person[0]
		var walk: String = person[1]
		var stand: String = person[2]
		await _bake_person(viewport, avatar, walk, stand)
		names.append(avatar)
		print("bake: %s" % avatar)
	var meta := {
		"cell": [CELL.x, CELL.y],
		"walk_frames": WALK_FRAMES,
		"stand_frames": STAND_FRAMES,
		"views": VIEWS,
		"height": HEIGHT,
		"people": names,
		"strides": _strides,
	}
	var file := FileAccess.open(OUT + "people.json", FileAccess.WRITE)
	file.store_string(JSON.stringify(meta, " ") + "\n")
	file.close()
	quit()


func _bake_person(viewport: SubViewport, avatar: String, walk: String, stand: String) -> void:
	var body := _load_scene("%s%s/Export/%s.fbx" % [SOURCE, avatar, avatar])
	var pivot := Node3D.new()
	viewport.add_child(pivot)
	pivot.add_child(body)
	var player: AnimationPlayer = body.find_children("*", "AnimationPlayer", true, false)[0]
	var library := AnimationLibrary.new()
	var walking := _clip(walk)
	library.add_animation("walk", walking)
	library.add_animation("stand", _clip(stand))
	player.add_animation_library("clips", library)
	var meshes := body.find_children("*", "MeshInstance3D", true, false)
	var mesh_node: MeshInstance3D = meshes[0]
	var colours := _colour_materials(mesh_node, avatar)
	var normals := _normal_materials(colours)
	var size := Vector2i(CELL.x * (WALK_FRAMES + STAND_FRAMES), CELL.y * VIEWS)
	var sheet := Image.create_empty(size.x, size.y, false, Image.FORMAT_RGBA8)
	var normal_sheet := Image.create_empty(size.x, size.y, false, Image.FORMAT_RGBA8)
	var pass_targets: Array[Image] = [sheet, normal_sheet]
	var pass_materials: Array[Array] = [colours, normals]
	for which: int in 2:
		var materials: Array = pass_materials[which]
		for surface: int in materials.size():
			var fitted: Material = materials[surface]
			mesh_node.set_surface_override_material(surface, fitted)
		for view: int in VIEWS:
			pivot.rotation.y = FORWARD_YAW - TAU * view / VIEWS
			for column: int in WALK_FRAMES + STAND_FRAMES:
				_pose(player, column)
				await RenderingServer.frame_post_draw
				await RenderingServer.frame_post_draw
				var shot: Image = viewport.get_texture().get_image()
				shot.convert(Image.FORMAT_RGBA8)
				var at := Vector2i(column * CELL.x, view * CELL.y)
				pass_targets[which].blit_rect(shot, Rect2i(Vector2i.ZERO, CELL), at)
	pivot.free()
	_strides.append(walking.get_meta("stride"))
	_save(sheet, "person_%s.webp" % avatar)
	normal_sheet.resize(size.x >> 1, size.y >> 1, Image.INTERPOLATE_BILINEAR)
	# Lossless, since lossy halves the sharpness of colour, which is all a normal map is.
	_save(normal_sheet, "person_%s_n.webp" % avatar, false)


func _pose(player: AnimationPlayer, column: int) -> void:
	if column < WALK_FRAMES:
		player.play("clips/walk")
		var walk_length := player.get_animation("clips/walk").length
		player.seek(walk_length * column / WALK_FRAMES, true)
	else:
		player.play("clips/stand")
		var stand_length := player.get_animation("clips/stand").length
		var into := float(column - WALK_FRAMES + 1) / (STAND_FRAMES + 1)
		player.seek(stand_length * into, true)
	player.pause()


func _colour_materials(mesh_node: MeshInstance3D, avatar: String) -> Array[Material]:
	var fitted: Array[Material] = []
	for surface: int in mesh_node.mesh.get_surface_count():
		var named := mesh_node.mesh.surface_get_material(surface).resource_name
		var path := "%s%s/Textures/%s_color.tga" % [SOURCE, avatar, named]
		var image := Image.load_from_file(ProjectSettings.globalize_path(path))
		image.generate_mipmaps()
		var material := StandardMaterial3D.new()
		material.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		material.albedo_texture = ImageTexture.create_from_image(image)
		material.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA_SCISSOR
		material.alpha_scissor_threshold = 0.5
		fitted.append(material)
	return fitted


func _normal_materials(colours: Array[Material]) -> Array[Material]:
	var shader := Shader.new()
	shader.code = NORMAL_SHADER
	var fitted: Array[Material] = []
	for colour: Material in colours:
		var material := ShaderMaterial.new()
		material.shader = shader
		material.set_shader_parameter("mask", (colour as StandardMaterial3D).albedo_texture)
		fitted.append(material)
	return fitted


## A clip with only its bones' turns, since its helpers don't exist on an avatar and its
## bone lengths aren't the avatar's. The walk goes on the spot: how far it covered is kept
## as its stride, so a walker's feet can be made to match the ground.
func _clip(clip: String) -> Animation:
	var scene := _load_scene("%sclips/%s.fbx" % [SOURCE, clip])
	var player: AnimationPlayer = scene.find_children("*", "AnimationPlayer", true, false)[0]
	var animation: Animation = player.get_animation(player.get_animation_list()[0]).duplicate()
	for track: int in range(animation.get_track_count() - 1, -1, -1):
		var path := String(animation.track_get_path(track))
		var is_root := path == "Skeleton3D:Bip01"
		var is_turn := animation.track_get_type(track) == Animation.TYPE_ROTATION_3D
		if not path.begins_with("Skeleton3D:") or not (is_turn or is_root):
			animation.remove_track(track)
		elif is_root and not is_turn:
			_walk_on_the_spot(animation, track)
	scene.free()
	return animation


## Takes the straight line from first to last key out of the root, leaving the sway.
func _walk_on_the_spot(animation: Animation, track: int) -> void:
	var keys := animation.track_get_key_count(track)
	var first: Vector3 = animation.track_get_key_value(track, 0)
	var last: Vector3 = animation.track_get_key_value(track, keys - 1)
	var covered := Vector3(last.x - first.x, 0.0, last.z - first.z)
	animation.set_meta("stride", covered.length())
	for key: int in keys:
		var at: Vector3 = animation.track_get_key_value(track, key)
		var along := animation.track_get_key_time(track, key) / animation.length
		animation.track_set_key_value(track, key, at - covered * along)


func _load_scene(path: String) -> Node:
	var document := FBXDocument.new()
	var state := FBXState.new()
	# The files name textures on their author's disk; the right ones are loaded above.
	state.handle_binary_image = GLTFState.HANDLE_BINARY_DISCARD_TEXTURES
	var err := document.append_from_file(ProjectSettings.globalize_path(path), state)
	if err != OK:
		push_error("bake: cannot read %s: %s" % [path, error_string(err)])
		quit(1)
	return document.generate_scene(state)


func _save(sheet: Image, file: String, lossy := true) -> void:
	sheet.fix_alpha_edges()
	var err := sheet.save_webp(OUT + file, lossy, WEBP_QUALITY)
	if err != OK:
		push_error("bake: cannot write %s: %s" % [file, error_string(err)])
		quit(1)
