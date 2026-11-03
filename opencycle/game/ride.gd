class_name Ride
extends Node3D
## Builds the world from a seed and flies the camera along a replay. Someone riding gets a loading
## screen, a HUD, pausing and the video options they saved. Stills, clips and benches get none of
## that, at the High preset. Arguments go after [code]--[/code]:
##
## [code]--biome=neon|forest[/code], [code]--seed=N[/code], [code]--course=inner|medium|full[/code],
## [code]--replay=res://replays/x.json[/code] and [code]--quality=ship|max[/code] choose
## what to show. On the city, [code]--video=Low|Medium|High[/code] starts from a preset,
## [code]--scale=0.5[/code] overrides its render scale and
## [code]--off=shadows,aa,fog,ssao,glow,probes,rain[/code] leaves those features out.
## [code]--reach=SHADOWS,LIT[/code] says how far off the street's lamps throw shadows and light.
## [code]--still=out.png[/code] with [code]--at=SECONDS[/code] renders one frame and quits,
## and [code]--look=YAW,PITCH,SIDEWAYS[/code] turns its camera and moves it off the racing line.
## [code]--bench=out.json[/code] rides the whole replay as fast as the GPU allows and
## reports its frame times, and [code]--rate=4[/code] rides it faster for a quick
## read. With Godot's [code]--write-movie[/code] and
## [code]--fixed-fps[/code], the ride advances one fixed step per frame however long a
## frame takes, and quits at the end so the movie is finalised. [code]--pass=depth|seg[/code]
## draws it as a control video for a model to restyle, in step with the colour render.

const WARMUP_FRAMES := 60
const BENCH_STRETCH := 100
const TARGET_FPS := 30
const MENU := "res://game/ui/menu.tscn"
## Frames the street draws behind the loading screen, to compile its shaders and fill its probes.
const SETTLE_FRAMES := 20
const RAIN := 0.65
const BIOME_REPLAYS: Dictionary[String, String] = {
	"forest": "res://replays/benchmark.json",
	"neon": "res://replays/neon.json",
}

var at_seconds := 0.0
var route: Route
var replay: Replay

var _time := 0.0
var _rate := 1.0
var _still_path := ""
var _bench_path := ""
var _gpu_times := PackedFloat32Array()
var _frame_times := PackedFloat32Array()
var _startup_ms := 0
var _warmup := WARMUP_FRAMES
var _primitives := 0
var _draw_calls := 0
var _uses_ride_time := false
var _gpu_by_stretch: Dictionary[int, PackedFloat32Array] = {}
var _primitive_counts := PackedInt32Array()
var _rain: MeshInstance3D
var _lamp_reach: LampReach
var _course: Course
var _stream: CityStream
var _video: VideoOptions
var _playing := false
var _hud: Hud
var _pause: PauseMenu

@onready var _camera: Camera3D = %Camera


func _ready() -> void:
	set_process(false)
	var started := Time.get_ticks_msec()
	var args := arguments()
	var seed_text: String = args.get("seed", "1")
	var run_seed := seed_text.to_int()
	var quality: String = args.get("quality", "ship")
	_still_path = args.get("still", "")
	_bench_path = args.get("bench", "")
	_playing = _still_path.is_empty() and _bench_path.is_empty() and not _is_movie()
	var at_text: String = args.get("at", "0")
	at_seconds = at_text.to_float()
	var rate_text: String = args.get("rate", "1")
	_rate = rate_text.to_float()
	var biome: String = args.get("biome", "neon")
	var which: String = args.get("course", Session.course)
	_video = _video_options(args)
	var loading: LoadingScreen
	if _playing:
		_video.apply_to_window()
		loading = LoadingScreen.new(Menu.title_of(which))
		add_child(loading)
		# Long enough for it to reach the screen before anything holds up the main thread.
		await get_tree().process_frame
		await get_tree().process_frame
	else:
		# An even 30 beats a ride that swings between 30 and 60. A bench needs to see the limit.
		Engine.max_fps = TARGET_FPS if _bench_path.is_empty() else 0
	var replay_path: String = args.get("replay", BIOME_REPLAYS.get(biome, ""))
	var parsed: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(replay_path))
	match biome:
		"neon":
			_course = CityPlan.new(run_seed).course(which)
			route = _course
			replay = Replay.from_dict(parsed, _course.length)
			_stream = NeonBiome.build(self, _course, quality)
			_rain = Weather.rain()
			_camera.add_child(_rain)
			_uses_ride_time = true
			_lamp_reach = LampReach.new()
			add_child(_lamp_reach)
			_camera.fov = NeonBiome.FOV
			_apply_video()
		_:
			route = Route.generate(run_seed)
			replay = Replay.from_dict(parsed)
			ForestBiome.build(self, run_seed, route, quality)
			if args.has("scale"):
				get_viewport().scaling_3d_scale = _video.render_scale()
	if args.has("reach"):
		var reach_text: String = args["reach"]
		var reach := reach_text.split_floats(",")
		_lamp_reach.shadows_within = reach[0]
		_lamp_reach.lit_within = reach[1]
	if args.has("pass"):
		add_child(ControlPass.new(args["pass"], _camera))
	_time = at_seconds
	_camera.transform = replay.camera_at(route, _time)
	if _stream:
		_stream.threaded = _playing
		_stream.arrived.connect(_on_section_arrived)
		_stream.left.connect(_lamp_reach.remove)
		var from := replay.ridden(at_seconds)
		if _playing:
			await _load_street(from, loading)
		else:
			_stream.begin(from)
	_startup_ms = Time.get_ticks_msec() - started
	print("ride: built %s in %d ms" % [biome, _startup_ms])
	_send_time()
	if _still_path and args.has("look"):
		_look(args["look"])
	if _bench_path:
		DisplayServer.window_set_vsync_mode(DisplayServer.VSYNC_DISABLED)
		RenderingServer.viewport_set_measure_render_time(get_viewport().get_viewport_rid(), true)
	if _still_path:
		_take_still.call_deferred()
	if _playing:
		_start_riding(loading)
	set_process(true)


## What to render with: what the player saved, or for anything else a fixed starting point.
func _video_options(args: Dictionary[String, String]) -> VideoOptions:
	var options := VideoOptions.saved() if _playing else VideoOptions.new()
	if args.has("video"):
		options.use_preset(args["video"])
	if args.has("off"):
		var off_text: String = args["off"]
		options.switch_off(off_text.split(","))
	if args.has("scale"):
		var scale_text: String = args["scale"]
		options.exact_scale = scale_text.to_float()
	return options


## Puts the options into effect, on the street as it stands and on what arrives after this.
func _apply_video() -> void:
	_video.apply_to_viewport(get_viewport())
	_video.apply_to_world(self)
	_lamp_reach.shadows_within = _video.lamp_shadows_within()
	_rain.visible = _video.is_on("rain")
	if _playing:
		_video.apply_to_window()
	if _hud:
		_hud.show_frame_rate(get_viewport().get_viewport_rid(), _video.is_on("show_fps"))


## Builds what's in view a section at a time, with the screen still moving.
func _load_street(from: float, loading: LoadingScreen) -> void:
	_stream.start_at(from)
	while _stream.wanted(from):
		_stream.update(from)
		loading.show_progress(_stream.progress(from))
		await get_tree().process_frame
	_lamp_reach.update(_camera.global_position)
	for _i: int in SETTLE_FRAMES:
		await get_tree().process_frame


func _start_riding(loading: LoadingScreen) -> void:
	if loading:
		loading.finish()
	_hud = Hud.new(replay, route, _course)
	add_child(_hud)
	_hud.show_frame_rate(get_viewport().get_viewport_rid(), _video.is_on("show_fps"))
	_pause = PauseMenu.new(_video)
	_pause.restarted.connect(_restart)
	_pause.ended.connect(_end_ride)
	if _lamp_reach:
		_pause.video_changed.connect(_apply_video)
	add_child(_pause)
	Input.mouse_mode = Input.MOUSE_MODE_HIDDEN


func _unhandled_input(event: InputEvent) -> void:
	if not _pause or get_tree().paused:
		return
	if event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		_pause.open()
	elif event is InputEventKey:
		var key := event as InputEventKey
		if key.pressed and not key.echo and key.physical_keycode == KEY_H:
			_hud.visible = not _hud.visible


func _restart() -> void:
	get_tree().paused = false
	get_tree().reload_current_scene()


func _end_ride() -> void:
	get_tree().paused = false
	get_tree().change_scene_to_file(MENU)


func _on_section_arrived(section: Node3D) -> void:
	_video.apply_to_section(section)
	_lamp_reach.add(section)


func _process(delta: float) -> void:
	if _still_path:
		return
	if _bench_path and _warmup > 0:
		# The first frames compile shaders and stream textures in; a ride that
		# has settled is what's being measured, and from the top of the replay.
		_warmup -= 1
		return
	_time += delta * _rate
	_camera.transform = replay.camera_at(route, _time)
	_send_time()
	if _hud:
		_hud.show_at(_time, delta)
	if _bench_path:
		_frame_times.append(delta)
		var gpu := RenderingServer.viewport_get_measured_render_time_gpu(
			get_viewport().get_viewport_rid()
		)
		_gpu_times.append(gpu)
		@warning_ignore("integer_division")
		var stretch := int(replay.distance_at(_time)) / BENCH_STRETCH * BENCH_STRETCH
		if stretch not in _gpu_by_stretch:
			_gpu_by_stretch[stretch] = PackedFloat32Array()
		_gpu_by_stretch[stretch].append(gpu)
		var drawn := RenderingServer.get_rendering_info(
			RenderingServer.RENDERING_INFO_TOTAL_PRIMITIVES_IN_FRAME
		)
		_primitive_counts.append(drawn)
		_primitives = maxi(_primitives, drawn)
		_draw_calls = maxi(
			_draw_calls,
			RenderingServer.get_rendering_info(
				RenderingServer.RENDERING_INFO_TOTAL_DRAW_CALLS_IN_FRAME
			)
		)
	if _time >= replay.duration() and (_bench_path or _is_movie()):
		if _bench_path:
			_write_report()
		get_tree().quit()


## Anything animated on the street runs off the ride's clock, so a replay looks the same.
func _send_time() -> void:
	if _uses_ride_time:
		_stream.update(replay.ridden(_time))
		_lamp_reach.update(_camera.global_position)
		RenderingServer.global_shader_parameter_set("ride_time", _time)
		get_tree().call_group(&"ride_clock", &"set_ride_time", _time)
		var open := _course.open_to_the_sky(replay.ridden(_time))
		((_rain.mesh as QuadMesh).material as ShaderMaterial).set_shader_parameter(
			"amount", open * RAIN
		)


## Metrics that say whether the ride was smooth, not just fast on average.
static func summarise(frame_times: PackedFloat32Array) -> Dictionary:
	var sorted := frame_times.duplicate()
	sorted.sort()
	var total := 0.0
	for t: float in sorted:
		total += t
	var count := sorted.size()
	var slowest_percent := 0.0
	@warning_ignore("integer_division")
	var tail := maxi(count / 100, 1)
	for i: int in tail:
		slowest_percent += sorted[count - 1 - i]
	return {
		"frames": count,
		"average_fps": count / total,
		"one_percent_low_fps": tail / slowest_percent,
		"worst_frame_ms": sorted[count - 1] * 1000.0,
	}


func _write_report() -> void:
	var report := summarise(_frame_times)
	# A window on a workspace nobody is looking at gets throttled, which caps the frame rate
	# above whatever the GPU could do. This is what the GPU itself spent.
	_gpu_times.sort()
	report["gpu_ms_median"] = _gpu_times[int(_gpu_times.size() * 0.5)]
	report["gpu_ms_99th"] = _gpu_times[int(_gpu_times.size() * 0.99)]
	var by_stretch := {}
	for stretch: int in _gpu_by_stretch:
		var times: PackedFloat32Array = _gpu_by_stretch[stretch]
		times.sort()
		by_stretch[str(stretch)] = times[int(times.size() * 0.5)]
	report["gpu_ms_median_by_stretch"] = by_stretch
	_primitive_counts.sort()
	report["median_primitives"] = _primitive_counts[int(_primitive_counts.size() * 0.5)]
	report["startup_ms"] = _startup_ms
	report["peak_primitives"] = _primitives
	report["peak_draw_calls"] = _draw_calls
	report["render_size"] = str(get_viewport().get_visible_rect().size)
	report["gpu"] = RenderingServer.get_video_adapter_name()
	var video_bytes := RenderingServer.get_rendering_info(
		RenderingServer.RENDERING_INFO_VIDEO_MEM_USED
	)
	report["peak_video_memory_mb"] = video_bytes / 1048576.0
	var file := FileAccess.open(_bench_path, FileAccess.WRITE)
	file.store_string(JSON.stringify(report, "  ") + "\n")
	file.close()
	print("ride: %s" % report)


## Points a still at something the ride only passes: degrees left and up, metres to the right.
func _look(spec: String) -> void:
	var parts := spec.split_floats(",")
	_camera.translate(Vector3(parts[2], 0.0, 0.0))
	_camera.rotate_y(deg_to_rad(parts[0]))
	_camera.rotate_object_local(Vector3.RIGHT, deg_to_rad(parts[1]))


func _take_still() -> void:
	# Long enough for the shaders, and for TAA's history to settle.
	for _i: int in 90:
		await RenderingServer.frame_post_draw
	var err := get_viewport().get_texture().get_image().save_png(_still_path)
	if err != OK:
		push_error("ride: cannot write %s: %s" % [_still_path, error_string(err)])
	get_tree().quit()


static func _is_movie() -> bool:
	return OS.has_feature("movie")


static func arguments() -> Dictionary[String, String]:
	var args: Dictionary[String, String] = {}
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--") and "=" in arg:
			args[arg.get_slice("=", 0).trim_prefix("--")] = arg.get_slice("=", 1)
	return args
