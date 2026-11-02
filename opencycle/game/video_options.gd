class_name VideoOptions
extends RefCounted
## What the ride spends the GPU on, kept between runs. Each option is an index into its choices, so
## the panel that edits them doesn't need to know what any of them mean. A still, a clip or a bench
## starts from [method new], which is High, and never reads what a player saved, so it always
## comes out the same.

const PATH := "user://video.cfg"
const SECTION := "video"
const CUSTOM := "Custom"
## The only one that holds 30 fps on most of a ride on the Radeon 890M this is built on.
const DEFAULT_PRESET := "Low"
## Every option, its choices, and a line about what it does.
const OPTIONS: Dictionary[String, Dictionary] = {
	"render_scale":
	{
		"label": "Render scale",
		"choices": ["50%", "60%", "67%", "75%", "85%", "100%"],
		"about": "Draws fewer pixels and scales them up. Usually the biggest saving.",
	},
	"antialiasing":
	{
		"label": "Anti-aliasing",
		"choices": ["Off", "FXAA", "TAA"],
		"about": "TAA is smoothest in motion and costs the most. FXAA is cheap but softer.",
	},
	"shadows":
	{
		"label": "Shadows",
		"choices": ["Off", "Low", "Medium", "High"],
		"about": "How far off lamps throw shadows, and how sharp those are. A big saving.",
	},
	"textures":
	{
		"label": "Texture detail",
		"choices": ["Low", "Medium", "High"],
		"about": "Samples blurrier texture levels with less filtering. A small saving.",
	},
	"fog":
	{
		"label": "Volumetric fog",
		"choices": ["Off", "Low", "High"],
		"about": "The lit haze that gives the street its depth. A small saving.",
	},
	"ambient_occlusion":
	{
		"label": "Ambient occlusion",
		"choices": ["Off", "Low", "High"],
		"about": "The soft dark in corners and under ledges. A moderate saving.",
	},
	"glow":
	{
		"label": "Glow",
		"choices": ["Off", "On"],
		"about": "Halos around signs and lamps. Almost free.",
	},
	"reflections":
	{
		"label": "Reflection probes",
		"choices": ["Off", "On"],
		"about": "What glossy surfaces reflect. Off makes glass and metal duller.",
	},
	"rain":
	{
		"label": "Rain",
		"choices": ["Off", "On"],
		"about": "Rain on the camera where the street is open to the sky.",
	},
	"fullscreen":
	{
		"label": "Fullscreen",
		"choices": ["Off", "On"],
		"about": "Fills the screen, without changing how many pixels are drawn.",
	},
	"vsync":
	{
		"label": "V-sync",
		"choices": ["Off", "On"],
		"about": "Stops tearing, for a little delay.",
	},
	"frame_limit":
	{
		"label": "Frame limit",
		"choices": ["30", "60", "None"],
		"about": "An even 30 looks better than a ride that swings between 30 and 60.",
	},
	"show_fps":
	{
		"label": "Show frame rate",
		"choices": ["Off", "On"],
		"about": "Frame rate and GPU time in the corner, to tune these by.",
	},
}
## Choices for what a preset sets. The rest are the player's whatever the preset.
const PRESETS: Dictionary[String, Dictionary] = {
	"Low":
	{
		"render_scale": 2,
		"antialiasing": 1,
		"shadows": 0,
		"textures": 0,
		"fog": 1,
		"ambient_occlusion": 0,
		"glow": 1,
		"reflections": 1,
		"rain": 1,
	},
	"Medium":
	{
		"render_scale": 3,
		"antialiasing": 1,
		"shadows": 1,
		"textures": 1,
		"fog": 1,
		"ambient_occlusion": 1,
		"glow": 1,
		"reflections": 1,
		"rain": 1,
	},
	"High":
	{
		"render_scale": 5,
		"antialiasing": 2,
		"shadows": 3,
		"textures": 2,
		"fog": 2,
		"ambient_occlusion": 2,
		"glow": 1,
		"reflections": 1,
		"rain": 1,
	},
}
const OTHERS: Dictionary[String, int] = {
	"fullscreen": 0,
	"vsync": 1,
	"frame_limit": 0,
	"show_fps": 0,
}
const SCALES: Array[float] = [0.5, 0.6, 0.67, 0.75, 0.85, 1.0]
const FRAME_LIMITS: Array[int] = [30, 60, 0]
## Metres off that lamps throw shadows. Below nothing, so none ever do.
const LAMP_SHADOWS: Array[float] = [-10.0, 15.0, 25.0, LampReach.SHADOWS_WITHIN]
const SKY_ATLAS: Array[int] = [2048, 2048, 4096, 8192]
const LAMP_ATLAS: Array[int] = [2048, 2048, 4096, 4096]
const SKY_REACH: Array[float] = [100.0, 100.0, 150.0, 200.0]
const SOFTNESS: Array[RenderingServer.ShadowQuality] = [
	RenderingServer.SHADOW_QUALITY_HARD,
	RenderingServer.SHADOW_QUALITY_HARD,
	RenderingServer.SHADOW_QUALITY_SOFT_VERY_LOW,
	RenderingServer.SHADOW_QUALITY_SOFT_LOW,
]
const MIP_BIAS: Array[float] = [1.0, 0.5, 0.0]
const ANISOTROPY: Array[Viewport.AnisotropicFiltering] = [
	Viewport.ANISOTROPY_DISABLED,
	Viewport.ANISOTROPY_2X,
	Viewport.ANISOTROPY_4X,
]
const FOG_GRID: Array[Vector2i] = [Vector2i(64, 32), Vector2i(64, 32), Vector2i(128, 64)]
const OCCLUSION: Array[RenderingServer.EnvironmentSSAOQuality] = [
	RenderingServer.ENV_SSAO_QUALITY_LOW,
	RenderingServer.ENV_SSAO_QUALITY_LOW,
	RenderingServer.ENV_SSAO_QUALITY_MEDIUM,
]

## A render scale that isn't one of the choices, from [code]--scale[/code], or 0.
var exact_scale := 0.0

var _values: Dictionary[String, int] = {}


func _init() -> void:
	_values.assign(OTHERS)
	use_preset("High")


## What the player chose last, or [constant DEFAULT_PRESET] if they haven't.
static func saved(path := PATH) -> VideoOptions:
	var options := VideoOptions.new()
	options.use_preset(DEFAULT_PRESET)
	var file := ConfigFile.new()
	if file.load(path) != OK:
		return options
	for key: String in OPTIONS:
		var stored: Variant = file.get_value(SECTION, key, options.value(key))
		if stored is int:
			var choice: int = stored
			options.set_value(key, choice)
	return options


func save(path := PATH) -> void:
	var file := ConfigFile.new()
	for key: String in _values:
		file.set_value(SECTION, key, _values[key])
	var err := file.save(path)
	if err != OK:
		push_warning("video: cannot save %s: %s" % [path, error_string(err)])


func value(key: String) -> int:
	return _values[key]


func set_value(key: String, choice: int) -> void:
	var choices: Array = OPTIONS[key]["choices"]
	_values[key] = clampi(choice, 0, choices.size() - 1)


func is_on(key: String) -> bool:
	return value(key) > 0


func use_preset(chosen: String) -> void:
	var settings: Dictionary = PRESETS[chosen]
	for key: String in settings:
		var choice: int = settings[key]
		set_value(key, choice)


## Which preset this is, or [constant CUSTOM] if it's been changed from all of them.
func preset() -> String:
	for candidate: String in PRESETS:
		var settings: Dictionary = PRESETS[candidate]
		var matches := true
		for key: String in settings:
			matches = matches and settings[key] == value(key)
		if matches:
			return candidate
	return CUSTOM


## What [code]--off[/code] names: costly features to leave out, to see what they cost.
func switch_off(names: PackedStringArray) -> void:
	var features: Dictionary[String, String] = {
		"shadows": "shadows",
		"aa": "antialiasing",
		"fog": "fog",
		"ssao": "ambient_occlusion",
		"glow": "glow",
		"probes": "reflections",
		"rain": "rain",
	}
	for feature: String in names:
		if feature in features:
			set_value(features[feature], 0)
		else:
			push_warning("video: nothing called %s to switch off" % feature)


func render_scale() -> float:
	return exact_scale if exact_scale > 0.0 else SCALES[value("render_scale")]


func lamp_shadows_within() -> float:
	return LAMP_SHADOWS[value("shadows")]


## Everything the renderer holds globally or per viewport.
func apply_to_viewport(viewport: Viewport) -> void:
	var scale := render_scale()
	viewport.scaling_3d_mode = (
		Viewport.SCALING_3D_MODE_FSR if scale < 1.0 else Viewport.SCALING_3D_MODE_BILINEAR
	)
	viewport.scaling_3d_scale = scale
	var aa := value("antialiasing")
	viewport.use_taa = aa == 2
	viewport.screen_space_aa = (
		Viewport.SCREEN_SPACE_AA_FXAA if aa == 1 else Viewport.SCREEN_SPACE_AA_DISABLED
	)
	var textures := value("textures")
	viewport.texture_mipmap_bias = MIP_BIAS[textures]
	viewport.anisotropic_filtering_level = ANISOTROPY[textures]
	var shadows := value("shadows")
	RenderingServer.directional_shadow_atlas_set_size(SKY_ATLAS[shadows], true)
	viewport.positional_shadow_atlas_size = LAMP_ATLAS[shadows]
	RenderingServer.directional_soft_shadow_filter_set_quality(SOFTNESS[shadows])
	RenderingServer.positional_soft_shadow_filter_set_quality(SOFTNESS[shadows])
	var fog: Vector2i = FOG_GRID[value("fog")]
	RenderingServer.environment_set_volumetric_fog_volume_size(fog.x, fog.y)
	var occlusion := value("ambient_occlusion")
	RenderingServer.environment_set_ssao_quality(OCCLUSION[occlusion], true, 0.5, 2, 50.0, 300.0)


## The window, which only matters to someone riding: nothing else should open fullscreen.
func apply_to_window() -> void:
	DisplayServer.window_set_mode(
		(
			DisplayServer.WINDOW_MODE_FULLSCREEN
			if is_on("fullscreen")
			else DisplayServer.WINDOW_MODE_WINDOWED
		)
	)
	DisplayServer.window_set_vsync_mode(
		DisplayServer.VSYNC_ENABLED if is_on("vsync") else DisplayServer.VSYNC_DISABLED
	)
	Engine.max_fps = FRAME_LIMITS[value("frame_limit")]


## The environment and the sky's light, below [param root]. Safe to run again, either way round.
func apply_to_world(root: Node) -> void:
	for node: Node in root.find_children("*", "WorldEnvironment", true, false):
		var environment: Environment = (node as WorldEnvironment).environment
		environment.volumetric_fog_enabled = is_on("fog")
		environment.ssao_enabled = is_on("ambient_occlusion")
		environment.glow_enabled = is_on("glow")
	var shadows := value("shadows")
	for node: Node in root.find_children("*", "DirectionalLight3D", true, false):
		var light := node as DirectionalLight3D
		if not light.has_meta(&"casts"):
			light.set_meta(&"casts", light.shadow_enabled)
		if light.get_meta(&"casts"):
			# Without its shadow the sky would shine straight through the deck.
			light.visible = shadows > 0
			light.directional_shadow_max_distance = SKY_REACH[shadows]
			light.directional_shadow_mode = (
				DirectionalLight3D.SHADOW_PARALLEL_2_SPLITS
				if shadows == 1
				else DirectionalLight3D.SHADOW_PARALLEL_4_SPLITS
			)
	apply_to_section(root)


## What each section of the street brings with it.
func apply_to_section(root: Node) -> void:
	for node: Node in root.find_children("*", "ReflectionProbe", true, false):
		(node as ReflectionProbe).visible = is_on("reflections")
