extends SceneTree
## Bakes Poly Haven's multi-million-triangle trees into the cards the game draws,
## lit softly from above with real shadows, so every needle keeps the depth and
## self-shade that flat cards can't work out for themselves:
##
## [code]<tree>_slices.webp[/code] is thin vertical slices through the trunk at
## evenly spaced angles, for the close trees: a fan of quads that keeps every real
## needle at a dozen triangles a tree. [code]<tree>_impostor.webp[/code] is the whole
## tree from four sides, for the distant ones. [code]<tree>.json[/code] is what the
## game needs to size them.
##
## Needs a real GPU and assets/bake-sources/. Run it through scripts/bake.sh.

const SOURCE := "res://assets/bake-sources/%s/%s_1k.gltf"
const OUT := "res://assets/baked/%s"
const TREES: PackedStringArray = ["fir_tree_01", "pine_tree_01"]
const SLICES := 6
const SLICE_HEIGHT := 2048
const IMPOSTOR_HEIGHT := 1024
## Lossy, because these go in the repo: a tenth of the size of a PNG and looks the same
## once it is on a card.
const WEBP_QUALITY := 0.9
## The fan's outer edge has gaps between slices, so each is thicker than it looks
## like it should be. Too thick and a slice shows branches that aren't in its plane.
const SLAB := 2.2
## Slices and views rotate about the trunk, so a corner of the bounds can swing
## further out than the bounds' own width.
const WIDEN := 1.25
const DISTANCE := 100.0


func _initialize() -> void:
	_bake_all.call_deferred()


func _bake_all() -> void:
	for tree: String in TREES:
		await _bake(tree)
	quit()


func _bake(tree: String) -> void:
	var doc := GLTFDocument.new()
	var state := GLTFState.new()
	var err := doc.append_from_file(SOURCE % [tree, tree], state)
	if err != OK:
		push_error("bake: cannot read %s: %s" % [tree, error_string(err)])
		quit(1)
		return
	var scene: Node3D = doc.generate_scene(state)
	# Each file holds three trees side by side; the first is the fullest.
	for child: Node in scene.get_children():
		if not child.name.contains("_a_"):
			scene.remove_child(child)
			child.free()
	var viewport := SubViewport.new()
	viewport.own_world_3d = true
	viewport.transparent_bg = true
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(viewport)
	viewport.add_child(scene)
	var camera := Camera3D.new()
	camera.projection = Camera3D.PROJECTION_ORTHOGONAL
	viewport.add_child(camera)

	viewport.add_child(_daylight())
	var world := WorldEnvironment.new()
	world.environment = _ambient()
	viewport.add_child(world)

	var bounds := _two_sided(scene, AABB())
	var center := bounds.get_center()
	var height := bounds.size.y
	var width := maxf(bounds.size.x, bounds.size.z) * WIDEN
	camera.size = height
	camera.near = DISTANCE - SLAB * 0.5
	camera.far = DISTANCE + SLAB * 0.5
	var slice_width := int(SLICE_HEIGHT * width / height)
	# A half-turn sees the same slice from behind, so the fan spans only that.
	_save(
		await _sheet(viewport, camera, center, Vector2i(slice_width, SLICE_HEIGHT), SLICES, PI),
		OUT % (tree + "_slices.webp")
	)
	camera.near = 0.1
	camera.far = DISTANCE * 2.0
	var view_width := int(IMPOSTOR_HEIGHT * width / height)
	_save(
		await _sheet(viewport, camera, center, Vector2i(view_width, IMPOSTOR_HEIGHT), 4, TAU),
		OUT % (tree + "_impostor.webp")
	)
	var meta := {"height": height, "width": width, "y_min": bounds.position.y, "slices": SLICES}
	var file := FileAccess.open(OUT % (tree + ".json"), FileAccess.WRITE)
	file.store_string(JSON.stringify(meta, "  ") + "\n")
	file.close()
	print("bake: %s %s" % [tree, meta])
	viewport.queue_free()
	await process_frame


## [param count] renders of the tree at angles evenly spread across [param span],
## side by side in one image.
func _sheet(
	viewport: SubViewport,
	camera: Camera3D,
	center: Vector3,
	cell: Vector2i,
	count: int,
	span: float
) -> Image:
	viewport.size = cell
	var sheet := Image.create_empty(cell.x * count, cell.y, false, Image.FORMAT_RGBA8)
	for i: int in count:
		var angle := i * span / count
		camera.global_position = center + Vector3(sin(angle), 0.0, cos(angle)) * DISTANCE
		camera.look_at(center)
		# One frame to submit, one for the readback to hold it.
		await RenderingServer.frame_post_draw
		await RenderingServer.frame_post_draw
		var shot: Image = viewport.get_texture().get_image()
		shot.convert(Image.FORMAT_RGBA8)
		sheet.blit_rect(shot, Rect2i(Vector2i.ZERO, cell), Vector2i(cell.x * i, 0))
	return sheet


func _save(image: Image, path: String) -> void:
	# Scissored cards sample their mips, where a transparent black edge reads as
	# a dark halo; this bleeds the colour out under the transparent pixels.
	image.fix_alpha_edges()
	var err := image.save_webp(path, true, WEBP_QUALITY)
	if err != OK:
		push_error("bake: cannot write %s: %s" % [path, error_string(err)])
		quit(1)


static func _daylight() -> DirectionalLight3D:
	var light := DirectionalLight3D.new()
	light.rotation_degrees = Vector3(-80.0, 20.0, 0.0)
	light.light_energy = 0.75
	light.shadow_enabled = true
	light.directional_shadow_mode = DirectionalLight3D.SHADOW_ORTHOGONAL
	light.directional_shadow_max_distance = DISTANCE * 2.0
	return light


static func _ambient() -> Environment:
	var environment := Environment.new()
	environment.background_mode = Environment.BG_CLEAR_COLOR
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	environment.ambient_light_color = Color(0.9, 0.95, 1.0)
	environment.ambient_light_energy = 0.55
	return environment


## Makes every material opaque and two-sided, and returns the bounds of [param node].
func _two_sided(node: Node, bounds: AABB) -> AABB:
	var instance := node as MeshInstance3D
	if instance:
		for s: int in instance.mesh.get_surface_count():
			var material := instance.mesh.surface_get_material(s) as BaseMaterial3D
			if material:
				material.transparency = BaseMaterial3D.TRANSPARENCY_DISABLED
				material.cull_mode = BaseMaterial3D.CULL_DISABLED
		var box := instance.global_transform * instance.get_aabb()
		bounds = box if bounds.size == Vector3.ZERO else bounds.merge(box)
	for child: Node in node.get_children():
		bounds = _two_sided(child, bounds)
	return bounds
