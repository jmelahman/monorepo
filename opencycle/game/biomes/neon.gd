class_name NeonBiome
extends RefCounted
## The bottom of a future city: a wet road under the concrete deck the rest of the city
## stands on, mostly shuttered, lit by what falls through a slot overhead and the few signs
## still on. The lighting is art-directed. docs/realism-standard.md is what it's held to.

const LENGTH := 1000.0
const STREET_HALF := Route.HALF_WIDTH + 1.0
## Wide enough to look up: the tall city overhead is half of what's worth seeing.
const FOV := 68.0
## Off while the environment is being worked on, to keep the street itself in view.
const CROWD := false


static func build(parent: Node3D, run_seed: int, route: Route, quality: String) -> void:
	RenderingServer.global_shader_parameter_add(
		"ride_time", RenderingServer.GLOBAL_VAR_TYPE_FLOAT, 0.0
	)
	var street := Street.generate(run_seed, LENGTH, STREET_HALF)
	var skyline := Skyline.generate(run_seed, street, route, LENGTH)
	var best := quality == "max"
	parent.add_child(StreetGround.build(route, street, LENGTH))
	parent.add_child(Facades.build(route, street))
	var undercity := Undercity.generate(run_seed, street, STREET_HALF, LENGTH)
	parent.add_child(Landmarks.build(route, skyline))
	parent.add_child(Overhead.build(route, undercity, LENGTH))
	parent.add_child(Quarters.build(route, street))
	parent.add_child(Shopfronts.build(route, street, LENGTH))
	parent.add_child(StreetWall.build(route, street))
	parent.add_child(StreetDressing.build(route, street))
	parent.add_child(Signage.build(route, street, LENGTH))
	parent.add_child(Furniture.build(route, street, LENGTH))
	if CROWD:
		parent.add_child(Pedestrians.build(run_seed, route, street, LENGTH))
	parent.add_child(Weather.build(route, street, LENGTH))
	_environment(parent, best, route)


static func _environment(parent: Node3D, best: bool, route: Route) -> void:
	var sky_material := ProceduralSkyMaterial.new()
	# Nothing but dark past the ends of the deck.
	sky_material.sky_top_color = Color(0.004, 0.006, 0.01)
	sky_material.sky_horizon_color = Color(0.02, 0.03, 0.045)
	sky_material.ground_horizon_color = Color(0.01, 0.015, 0.02)
	sky_material.ground_bottom_color = Color(0.0, 0.0, 0.0)
	sky_material.sun_angle_max = 0.0
	var sky := Sky.new()
	sky.sky_material = sky_material
	var environment := Environment.new()
	environment.background_mode = Environment.BG_SKY
	environment.sky = sky
	# Shadows stay blue, so every warm light in the street has something to stand against.
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	environment.ambient_light_color = Color(0.25, 0.4, 0.7)
	environment.ambient_light_energy = 0.4
	environment.reflected_light_source = Environment.REFLECTION_SOURCE_SKY
	environment.tonemap_mode = Environment.TONE_MAPPER_ACES
	environment.tonemap_white = 7.0
	environment.adjustment_enabled = true
	environment.adjustment_contrast = 1.15
	environment.adjustment_saturation = 0.85
	# Wide, soft halos: the air is thick, and a sign lights what's around it.
	environment.glow_enabled = true
	environment.glow_intensity = 0.4
	environment.glow_bloom = 0.0
	environment.glow_hdr_threshold = 1.3
	environment.glow_blend_mode = Environment.GLOW_BLEND_MODE_SCREEN
	for level: int in 7:
		environment.set_glow_level(level, 1.0 if level >= 2 and level <= 5 else 0.0)
	environment.ssao_enabled = true
	environment.ssao_radius = 1.8
	environment.ssao_intensity = 3.5
	environment.ssao_light_affect = 0.35
	environment.fog_enabled = true
	environment.fog_mode = Environment.FOG_MODE_DEPTH
	environment.fog_light_color = Color(0.025, 0.04, 0.055)
	environment.fog_depth_begin = 60.0
	environment.fog_depth_end = 480.0
	environment.fog_depth_curve = 1.4
	# Every light in the street shows up in this, which is what makes the street feel deep.
	RenderingServer.environment_set_volumetric_fog_volume_size(192, 128)
	environment.volumetric_fog_enabled = true
	environment.volumetric_fog_density = 0.013
	environment.volumetric_fog_albedo = Color(0.85, 0.85, 0.95)
	environment.volumetric_fog_anisotropy = 0.35
	environment.volumetric_fog_length = 140.0
	environment.volumetric_fog_detail_spread = 2.0
	environment.volumetric_fog_ambient_inject = 0.1
	environment.volumetric_fog_sky_affect = 0.15
	var world := WorldEnvironment.new()
	world.environment = environment
	parent.add_child(world)
	_probes(parent, route)
	# No moon up there, but the cold light on the upper floors sets off the street's heat.
	var cold := DirectionalLight3D.new()
	cold.rotation_degrees = Vector3(-62.0, 35.0, 0.0)
	cold.light_color = Color(0.5, 0.68, 0.88)
	cold.light_energy = 0.45
	cold.light_specular = 0.15
	cold.light_volumetric_fog_energy = 0.0
	# Only gets in through the slots in the deck, and rakes down the walls from there.
	cold.shadow_enabled = true
	cold.directional_shadow_max_distance = 200.0
	cold.directional_shadow_mode = DirectionalLight3D.SHADOW_PARALLEL_4_SPLITS
	parent.add_child(cold)
	# What comes back up off the wet street: no shadows, but it models every ledge and soffit.
	for yaw: float in [-150.0, 30.0]:
		var bounce := DirectionalLight3D.new()
		bounce.rotation_degrees = Vector3(24.0, yaw, 0.0)
		bounce.light_color = Color(0.5, 0.56, 0.68)
		bounce.light_energy = 0.4
		bounce.light_specular = 0.0
		bounce.light_volumetric_fog_energy = 0.0
		parent.add_child(bounce)
	var viewport := parent.get_viewport()
	viewport.use_taa = true
	RenderingServer.directional_shadow_atlas_set_size(8192, true)
	viewport.positional_shadow_atlas_size = 8192
	if best:
		viewport.msaa_3d = Viewport.MSAA_4X


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
