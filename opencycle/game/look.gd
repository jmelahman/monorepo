class_name Look
extends RefCounted
## The mood of a biome: sky, sun, fog and how much the renderer spends on them.
##
## [code]ship[/code] is what has to hold frame rate on an integrated GPU.
## [code]max[/code] is for slow captures, to show what the world can look like.

const SKY := (
	"res://assets/polyhaven/kloofendal_48d_partly_cloudy_puresky/%s_2k.hdr"
	% ["kloofendal_48d_partly_cloudy_puresky"]
)


static func apply(parent: Node3D, quality: String) -> void:
	var best := quality == "max"
	parent.add_child(_sun(best))
	var world := WorldEnvironment.new()
	world.environment = _environment(best)
	parent.add_child(world)
	var viewport := parent.get_viewport()
	if best:
		viewport.scaling_3d_mode = Viewport.SCALING_3D_MODE_BILINEAR
		viewport.scaling_3d_scale = 1.0
		viewport.use_taa = true
		viewport.msaa_3d = Viewport.MSAA_4X
	else:
		# Not FSR2: it posterises this sky into flat bands of colour. Going below 0.67
		# gains little here: the 890M averaged 53 fps at 0.67 and 57 at 0.5.
		viewport.scaling_3d_mode = Viewport.SCALING_3D_MODE_FSR
		viewport.scaling_3d_scale = 0.67
		viewport.use_taa = true
	RenderingServer.directional_shadow_atlas_set_size(8192 if best else 2048, true)


static func _sun(best: bool) -> DirectionalLight3D:
	var sun := DirectionalLight3D.new()
	sun.rotation_degrees = Vector3(-42.0, -35.0, 0.0)
	sun.light_color = Color(1.0, 0.95, 0.86)
	sun.light_energy = 1.6
	sun.shadow_enabled = true
	sun.directional_shadow_max_distance = 260.0 if best else 90.0
	sun.directional_shadow_mode = (
		DirectionalLight3D.SHADOW_PARALLEL_4_SPLITS
		if best
		else DirectionalLight3D.SHADOW_PARALLEL_2_SPLITS
	)
	return sun


static func _environment(best: bool) -> Environment:
	var panorama := PanoramaSkyMaterial.new()
	panorama.panorama = load(SKY)
	var sky := Sky.new()
	sky.sky_material = panorama
	var environment := Environment.new()
	environment.background_mode = Environment.BG_SKY
	environment.sky = sky
	environment.ambient_light_source = Environment.AMBIENT_SOURCE_SKY
	environment.ambient_light_energy = 0.7
	environment.reflected_light_source = Environment.REFLECTION_SOURCE_SKY
	environment.tonemap_mode = Environment.TONE_MAPPER_AGX
	environment.fog_enabled = true
	environment.fog_mode = Environment.FOG_MODE_DEPTH
	environment.fog_light_color = Color(0.68, 0.76, 0.85)
	environment.fog_depth_begin = 90.0
	environment.fog_depth_end = 1900.0
	environment.fog_depth_curve = 1.4
	environment.fog_aerial_perspective = 0.6
	environment.ssao_enabled = best
	environment.ssil_enabled = best
	environment.sdfgi_enabled = best
	environment.volumetric_fog_enabled = best
	environment.volumetric_fog_density = 0.0012
	return environment
