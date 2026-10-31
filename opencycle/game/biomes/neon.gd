class_name NeonBiome
extends RefCounted
## A cyberpunk street at night, built on what Mong Kok actually looks like: bright at
## street level, damp rather than soaked, with shops full of goods and signs hanging
## out over the road. docs/realism-standard.md is what it's held to.

const LENGTH := 200.0
const STREET_HALF := Route.HALF_WIDTH + 1.0


static func build(parent: Node3D, run_seed: int, route: Route, quality: String) -> void:
	RenderingServer.global_shader_parameter_add(
		"ride_time", RenderingServer.GLOBAL_VAR_TYPE_FLOAT, 0.0
	)
	var street := Street.generate(run_seed, LENGTH, STREET_HALF)
	var best := quality == "max"
	parent.add_child(StreetGround.build(route, street, LENGTH))
	parent.add_child(Facades.build(route, street))
	parent.add_child(Shopfronts.build(route, street, LENGTH))
	parent.add_child(Signage.build(route, street))
	parent.add_child(Furniture.build(route, street, LENGTH))
	_environment(parent, best, route)


static func _environment(parent: Node3D, best: bool, route: Route) -> void:
	var sky_material := ProceduralSkyMaterial.new()
	# City sky: never black, lit from below by everything under it.
	sky_material.sky_top_color = Color(0.012, 0.014, 0.03)
	sky_material.sky_horizon_color = Color(0.09, 0.06, 0.09)
	sky_material.ground_horizon_color = Color(0.05, 0.04, 0.05)
	sky_material.ground_bottom_color = Color(0.0, 0.0, 0.0)
	sky_material.sun_angle_max = 0.0
	var sky := Sky.new()
	sky.sky_material = sky_material
	var environment := Environment.new()
	environment.background_mode = Environment.BG_SKY
	environment.sky = sky
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	environment.ambient_light_color = Color(0.5, 0.48, 0.55)
	environment.ambient_light_energy = 0.28
	environment.reflected_light_source = Environment.REFLECTION_SOURCE_SKY
	environment.tonemap_mode = Environment.TONE_MAPPER_FILMIC
	environment.tonemap_white = 6.0
	environment.glow_enabled = true
	environment.glow_intensity = 0.35
	environment.glow_bloom = 0.02
	environment.glow_hdr_threshold = 1.1
	environment.ssr_enabled = true
	environment.ssr_max_steps = 96 if best else 32
	environment.ssr_depth_tolerance = 0.5
	environment.ssao_enabled = true
	environment.ssao_radius = 0.8
	environment.ssao_intensity = 2.0
	environment.fog_enabled = true
	environment.fog_mode = Environment.FOG_MODE_DEPTH
	environment.fog_light_color = Color(0.1, 0.08, 0.11)
	environment.fog_depth_begin = 40.0
	environment.fog_depth_end = 420.0
	environment.fog_depth_curve = 1.6
	environment.volumetric_fog_enabled = best
	environment.volumetric_fog_density = 0.008
	var world := WorldEnvironment.new()
	world.environment = environment
	parent.add_child(world)
	_probes(parent, route)
	var viewport := parent.get_viewport()
	viewport.use_taa = true
	if best:
		viewport.msaa_3d = Viewport.MSAA_4X
	else:
		viewport.scaling_3d_mode = Viewport.SCALING_3D_MODE_FSR
		viewport.scaling_3d_scale = 0.67


## Captures of the signs, so anything glossy shows what is around it. Rendered once at
## the start: nothing they see moves enough to matter.
static func _probes(parent: Node3D, route: Route) -> void:
	var s := 0.0
	while s < LENGTH + 40.0:
		var probe := ReflectionProbe.new()
		probe.update_mode = ReflectionProbe.UPDATE_ONCE
		probe.box_projection = true
		probe.size = Vector3(32.0, 40.0, 40.0)
		probe.max_distance = 60.0
		probe.ambient_mode = ReflectionProbe.AMBIENT_ENVIRONMENT
		probe.position = route.at(s) + Vector3.UP * 3.0
		parent.add_child(probe)
		s += 36.0
