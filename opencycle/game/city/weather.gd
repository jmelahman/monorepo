class_name Weather
extends RefCounted
## What's in the air: mist along the street, steam from the covers in the road, and rain.

const MIST: Shader = preload("res://game/mist.gdshader")
const STEAM: Shader = preload("res://game/steam.gdshader")
const RAIN: Shader = preload("res://game/rain.gdshader")
const MIST_STRETCH := 40.0
const MIST_HEIGHT := 7.0
const STEAM_HEIGHT := 9.0
## One cover in this many steams, which is enough to keep turning up on a ride.
const STEAM_EVERY := 2


static func build(route: Route, street: Street, length: float, section: StreetSection) -> Node3D:
	var root := Node3D.new()
	root.name = "Weather"
	var mist: ShaderMaterial = Once.made("mist", _mist)
	var width := (NeonBiome.STREET_HALF + Street.SIDEWALK + 1.0) * 2.0
	var s := -MIST_STRETCH
	while s < length + 150.0:
		if not section.holds(s):
			s += MIST_STRETCH
			continue
		var volume := FogVolume.new()
		volume.shape = RenderingServer.FOG_VOLUME_SHAPE_BOX
		volume.size = Vector3(width, MIST_HEIGHT, MIST_STRETCH + 2.0)
		volume.material = mist
		volume.basis = Basis(route.right(s), Vector3.UP, route.tangent(s))
		volume.position = route.at(s) + Vector3.UP * MIST_HEIGHT * 0.5
		root.add_child(volume)
		s += MIST_STRETCH
	for index: int in street.covers.size():
		var cover := street.covers[index]
		if not section.holds(cover.x):
			continue
		if index % STEAM_EVERY == 0 and cover.x > -20.0 and cover.x < length + 120.0:
			root.add_child(steam(route, cover, float(index) * 0.37))
	return root


static func _mist() -> ShaderMaterial:
	var mist := ShaderMaterial.new()
	mist.shader = MIST
	return mist


static func steam(route: Route, cover: Vector2, phase: float) -> FogVolume:
	var material := ShaderMaterial.new()
	material.shader = STEAM
	material.set_shader_parameter("phase", phase)
	var volume := FogVolume.new()
	volume.shape = RenderingServer.FOG_VOLUME_SHAPE_BOX
	volume.size = Vector3(5.0, STEAM_HEIGHT, 5.0)
	volume.material = material
	volume.position = (
		route.at(cover.x) + route.right(cover.x) * cover.y + Vector3.UP * STEAM_HEIGHT * 0.5
	)
	return volume


## A quad its shader pins to the whole screen. It rides along with the camera, because one
## left at the start of the route gets culled however large its margin.
static func rain() -> MeshInstance3D:
	var quad := QuadMesh.new()
	quad.size = Vector2(2.0, 2.0)
	var material := ShaderMaterial.new()
	material.shader = RAIN
	quad.material = material
	var drops := MeshInstance3D.new()
	drops.mesh = quad
	drops.extra_cull_margin = 16384.0
	drops.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	return drops
