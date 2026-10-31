extends SceneTree
## Bakes shop stock into one sheet: rows of real scanned goods, as a shelf would hold them,
## each cell a strip 1.2 m wide and 0.4 m high with a clear background. A shelf then costs
## two triangles, and from the road it's as good as the real objects. Which cell belongs to
## which trade is in [Shopfronts].
##
## Needs a real GPU. Run it through scripts/bake.sh.

const OUT := "res://assets/baked/goods.webp"
const MODEL := "res://assets/polyhaven/%s/%s_1k.gltf"
const SHEET := 2048
const COLUMNS := 2
const ROWS := 6
const WIDE := 1.2
const HIGH := 0.4
const WEBP_QUALITY := 0.9
const TILT := 7.0
## Each cell's goods, as [model, how tall its tallest part stands in metres]. Models keep
## their parts' proportions, so a tin stays smaller than the milk beside it.
const CELLS: Array[Array] = [
	[["russian_food_cans_01", 0.2], ["bleach_bottle", 0.24], ["all_purpose_cleaner", 0.25]],
	[["russian_food_cans_01", 0.16], ["multi_cleaner_bottle", 0.27], ["spray_paint_bottles", 0.2]],
	[["all_purpose_cleaner", 0.2], ["russian_food_cans_01", 0.24], ["bleach_bottle", 0.2]],
	[["long_life_food", 0.32], ["russian_food_cans_01", 0.22], ["bleach_bottle", 0.24]],
	[["russian_food_cans_01", 0.24], ["food_lime_01", 0.08], ["food_pomegranate_01", 0.11]],
	[["long_life_food", 0.28], ["multi_cleaner_bottle", 0.26], ["russian_food_cans_01", 0.2]],
	[["television_02", 0.3], ["cardboard_box_01", 0.2]],
	[["Television_01", 0.28], ["cardboard_box_01", 0.14], ["television_02", 0.2]],
	[["cardboard_box_01", 0.3], ["spray_paint_bottles", 0.2], ["television_02", 0.22]],
	[["brass_pot_01", 0.2], ["russian_food_cans_01", 0.2], ["long_life_food", 0.3]],
	[["brass_pot_01", 0.16], ["spray_paint_bottles", 0.15], ["russian_food_cans_01", 0.18]],
	[["brass_pot_01", 0.15], ["brass_pot_01", 0.09], ["food_pomegranate_01", 0.07]],
]

var _parts: Dictionary[String, Array] = {}


func _initialize() -> void:
	_bake.call_deferred()


func _bake() -> void:
	var sheet := Image.create_empty(SHEET, SHEET, false, Image.FORMAT_RGBA8)
	var cell := Vector2i(Vector2(SHEET / float(COLUMNS), SHEET / float(ROWS)))
	var viewport := SubViewport.new()
	viewport.own_world_3d = true
	viewport.transparent_bg = true
	viewport.size = cell
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(viewport)
	_stage(viewport)
	for index: int in CELLS.size():
		var shelf := _fill_shelf(CELLS[index], index)
		viewport.add_child(shelf)
		for _i: int in 3:
			await RenderingServer.frame_post_draw
		var shot: Image = viewport.get_texture().get_image()
		shot.convert(Image.FORMAT_RGBA8)
		var at := Vector2i(index % COLUMNS * cell.x, floori(float(index) / COLUMNS) * cell.y)
		sheet.blit_rect(shot, Rect2i(Vector2i.ZERO, cell), at)
		viewport.remove_child(shelf)
		shelf.free()
	sheet.generate_mipmaps()
	var err := sheet.save_webp(OUT, false, WEBP_QUALITY)
	if err != OK:
		push_error("bake: cannot write %s: %s" % [OUT, error_string(err)])
	print("bake: goods, %d cells" % CELLS.size())
	quit()


static func _stage(viewport: SubViewport) -> void:
	var camera := Camera3D.new()
	camera.projection = Camera3D.PROJECTION_ORTHOGONAL
	camera.size = HIGH
	camera.rotation_degrees = Vector3(-TILT, 0.0, 0.0)
	camera.position = Vector3(0.0, HIGH * 0.5 + 2.0 * sin(deg_to_rad(TILT)), 2.0)
	camera.near = 0.1
	camera.far = 5.0
	viewport.add_child(camera)
	var key := DirectionalLight3D.new()
	key.rotation_degrees = Vector3(-55.0, -15.0, 0.0)
	key.light_energy = 1.3
	viewport.add_child(key)
	var environment := Environment.new()
	# Metal has to have something to reflect, or brass comes out black. The bake's clear
	# background keeps this colour out of the picture itself.
	environment.background_mode = Environment.BG_COLOR
	environment.background_color = Color(0.8, 0.72, 0.6)
	environment.reflected_light_source = Environment.REFLECTION_SOURCE_BG
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	environment.ambient_light_color = Color.WHITE
	environment.ambient_light_energy = 0.65
	var world := WorldEnvironment.new()
	world.environment = environment
	viewport.add_child(world)


## Two rows, the back one peering over the front, in whatever fits along the shelf.
func _fill_shelf(goods: Array, index: int) -> Node3D:
	var shelf := Node3D.new()
	var dice := Rng.derive(1234, [index])
	var count := 0
	for row: int in 2:
		var x := -WIDE * 0.55
		var depth := -0.24 * row
		var lift := 0.04 * row
		while x < WIDE * 0.55:
			var entry: Array = goods[int(Rng.unit(dice, count * 3) * goods.size())]
			var model: String = entry[0]
			var tall: float = entry[1]
			var parts := _parts_of(model)
			var part: Mesh = parts[int(Rng.unit(dice, count * 3 + 1) * parts.size())]
			var bounds := part.get_aabb()
			var scale := tall * lerpf(0.9, 1.08, Rng.unit(dice, count * 3 + 2)) / _height_of(parts)
			var item := MeshInstance3D.new()
			item.mesh = part
			var turn := (Rng.unit(dice, count * 3 + 7) - 0.5) * 0.7
			var basis := Basis(Vector3.UP, turn).scaled(Vector3.ONE * scale)
			var width := bounds.size.x * scale
			item.transform = Transform3D(
				basis,
				(
					Vector3(x + width * 0.5, lift - bounds.position.y * scale, depth)
					- basis * (bounds.get_center() * Vector3(1.0, 0.0, 1.0))
				)
			)
			shelf.add_child(item)
			# Stack anything low, the way a shop uses the height it has.
			var rise := bounds.size.y * scale
			var layer := 1
			while rise * (layer + 1) < HIGH * 0.8:
				var above := item.duplicate() as MeshInstance3D
				above.position.y += rise * layer
				shelf.add_child(above)
				layer += 1
			x += width + 0.008
			count += 1
	return shelf


func _parts_of(model: String) -> Array:
	if model not in _parts:
		var scene: PackedScene = load(MODEL % [model, model])
		var node := scene.instantiate()
		var found: Array = []
		for child: Node in node.find_children("*", "MeshInstance3D", true, false):
			# A sticker is a skin for its bottle, and no use standing on a shelf by itself.
			if not child.name.ends_with("_sticker"):
				found.append((child as MeshInstance3D).mesh)
		node.free()
		_parts[model] = found
	return _parts[model]


static func _height_of(parts: Array) -> float:
	var tallest := 0.0
	for part: Mesh in parts:
		tallest = maxf(tallest, part.get_aabb().size.y)
	return tallest
