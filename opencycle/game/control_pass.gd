class_name ControlPass
extends Node
## Draws the ride as a control video for a model that makes a render look filmed, in step
## with the colour render so the two line up frame for frame. [code]depth[/code] is inverse
## distance, white close by and black at the sky. [code]seg[/code] gives everything drawn
## with one shader one flat colour, so road, wall, sign and leaf each read as a kind of thing.
## Whatever is only seen through (rain, glows, mist) is left out of it.

const DEPTH := preload("res://game/control_depth.gdshader")
const SEG := preload("res://game/control_seg.gdshader")

var kind: String
var _camera: Camera3D
var _flat: Dictionary[Material, Material] = {}
var _untextured := StandardMaterial3D.new()
var _meshes: Dictionary[Mesh, Mesh] = {}


func _init(pass_kind: String, camera: Camera3D) -> void:
	kind = pass_kind
	_camera = camera


func _ready() -> void:
	var world := get_parent()
	for node: Node in world.find_children("*", "WorldEnvironment", true, false):
		(node as WorldEnvironment).environment = _plain()
		(node as WorldEnvironment).camera_attributes = null
	_camera.attributes = null
	if kind == "depth":
		_camera.add_child(_depth_quad())
	for node: Node in world.find_children("*", "", true, false):
		_take(node)
	get_tree().node_added.connect(_take)


## Nothing between the surfaces and the file: no tone, glow, fog or sky.
static func _plain() -> Environment:
	var environment := Environment.new()
	environment.background_mode = Environment.BG_COLOR
	environment.background_color = Color.BLACK
	environment.tonemap_mode = Environment.TONE_MAPPER_LINEAR
	return environment


func _depth_quad() -> MeshInstance3D:
	var material := ShaderMaterial.new()
	material.shader = DEPTH
	material.render_priority = Material.RENDER_PRIORITY_MAX
	var quad := QuadMesh.new()
	quad.size = Vector2(2.0, 2.0)
	quad.material = material
	var screen := MeshInstance3D.new()
	screen.mesh = quad
	screen.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	screen.extra_cull_margin = 16384.0
	return screen


func _take(node: Node) -> void:
	if kind != "seg":
		return
	if node is FogVolume or node is Decal or node is GPUParticles3D or node is CPUParticles3D:
		(node as Node3D).visible = false
	elif node is GeometryInstance3D:
		var instance := node as GeometryInstance3D
		instance.material_overlay = null
		if instance.material_override:
			instance.material_override = _replace(instance.material_override)
		elif instance is MeshInstance3D and (instance as MeshInstance3D).mesh:
			var single := instance as MeshInstance3D
			for i: int in single.mesh.get_surface_count():
				single.set_surface_override_material(i, _replace(single.get_active_material(i)))
		elif instance is MultiMeshInstance3D and (instance as MultiMeshInstance3D).multimesh:
			var many := (instance as MultiMeshInstance3D).multimesh
			if many.mesh:
				many.mesh = _remade(many.mesh)


## A shared mesh drawn as many, with its surfaces replaced once for every instance of it.
func _remade(mesh: Mesh) -> Mesh:
	if mesh not in _meshes:
		var copy: Mesh = mesh.duplicate()
		if copy is PrimitiveMesh:
			(copy as PrimitiveMesh).material = _replace((copy as PrimitiveMesh).material)
		elif copy is ArrayMesh:
			for i: int in copy.get_surface_count():
				copy.surface_set_material(i, _replace(copy.surface_get_material(i)))
		_meshes[mesh] = copy
		_meshes[copy] = copy
	return _meshes[mesh]


func _replace(material: Material) -> Material:
	if material == null:
		material = _untextured
	if material in _flat:
		return _flat[material]
	_flat[material] = _nothing() if _see_through(material) else _flat_for(material)
	return _flat[material]


func _nothing() -> Material:
	var material := StandardMaterial3D.new()
	material.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	material.albedo_color = Color(0.0, 0.0, 0.0, 0.0)
	return material


static func _see_through(material: Material) -> bool:
	if material is BaseMaterial3D:
		return (material as BaseMaterial3D).transparency == BaseMaterial3D.TRANSPARENCY_ALPHA
	if material is ShaderMaterial and (material as ShaderMaterial).shader:
		var code := (material as ShaderMaterial).shader.code
		return "blend_" in code or ("ALPHA" in code and "ALPHA_SCISSOR" not in code)
	return false


func _flat_for(material: Material) -> Material:
	var flat := ShaderMaterial.new()
	flat.shader = SEG
	flat.set_shader_parameter("colour", _colour(_kind_of(material)))
	var cutout := _cutout(material)
	if cutout:
		flat.set_shader_parameter("cutout", cutout)
		flat.set_shader_parameter("cut", true)
	return flat


## What a surface is, as near as its material says: its shader, or failing that its texture.
static func _kind_of(material: Material) -> String:
	if material is ShaderMaterial and (material as ShaderMaterial).shader:
		return (material as ShaderMaterial).shader.resource_path
	if material is BaseMaterial3D:
		var standard := material as BaseMaterial3D
		if standard.albedo_texture:
			return standard.albedo_texture.resource_path
		return str(standard.albedo_color)
	return material.get_class()


## Well apart in hue for kinds that hash apart, and never black, which is the sky.
static func _colour(of: String) -> Color:
	var hash_value := of.hash()
	var hue := float(hash_value & 0xffff) / 65535.0
	var value := 0.55 + 0.45 * float((hash_value >> 16) & 0xff) / 255.0
	return Color.from_hsv(hue, 0.85, value)


static func _cutout(material: Material) -> Texture2D:
	if material is BaseMaterial3D:
		var standard := material as BaseMaterial3D
		if standard.transparency != BaseMaterial3D.TRANSPARENCY_DISABLED:
			return standard.albedo_texture
		return null
	if not material is ShaderMaterial or not (material as ShaderMaterial).shader:
		return null
	var shader_material := material as ShaderMaterial
	var code := shader_material.shader.code
	if "ALPHA_SCISSOR" not in code and "discard" not in code:
		return null
	for uniform: Dictionary in shader_material.shader.get_shader_uniform_list():
		if uniform["type"] == TYPE_OBJECT and uniform["hint_string"] == "Texture2D":
			var uniform_name: StringName = uniform["name"]
			var texture: Variant = shader_material.get_shader_parameter(uniform_name)
			if texture is Texture2D:
				return texture
	return null
