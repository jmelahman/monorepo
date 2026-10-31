extends Node3D
## Builds the world from a seed and flies the camera along a replay. Arguments go
## after [code]--[/code]:
##
## [code]--seed=N[/code], [code]--replay=res://replays/x.json[/code],
## [code]--quality=ship|max[/code] choose what to show. [code]--still=out.png[/code]
## with [code]--at=SECONDS[/code] renders one frame and quits.
## [code]--bench=out.json[/code] rides the whole replay as fast as the GPU allows and
## reports its frame times. With Godot's [code]--write-movie[/code] and
## [code]--fixed-fps[/code], the ride advances one fixed step per frame however long a
## frame takes, and quits at the end so the movie is finalised.

const WARMUP_FRAMES := 60

var at_seconds := 0.0
var route: Route
var replay: Replay

var _time := 0.0
var _still_path := ""
var _bench_path := ""
var _frame_times := PackedFloat32Array()
var _startup_ms := 0
var _warmup := WARMUP_FRAMES
var _primitives := 0
var _draw_calls := 0

@onready var _camera: Camera3D = %Camera


func _ready() -> void:
	var started := Time.get_ticks_msec()
	var args := _arguments()
	var run_seed := int(args.get("seed", "1"))
	var quality: String = args.get("quality", "ship")
	_still_path = args.get("still", "")
	_bench_path = args.get("bench", "")
	at_seconds = float(args.get("at", "0"))
	var replay_path: String = args.get("replay", "res://replays/benchmark.json")
	var parsed: Variant = JSON.parse_string(FileAccess.get_file_as_string(replay_path))
	replay = Replay.from_dict(parsed)
	route = Route.generate(run_seed)
	var terrain := Terrain.generate(run_seed, route)
	var plants := Planting.generate(run_seed, terrain)
	add_child(TerrainMesh.build(terrain))
	add_child(RoadMesh.build(route))
	add_child(Flora.build(plants))
	Look.apply(self, quality)
	_startup_ms = Time.get_ticks_msec() - started
	print("ride: built in %d ms, %s" % [_startup_ms, _counts(plants)])
	_time = at_seconds
	_camera.transform = replay.camera_at(route, _time)
	if _bench_path:
		Engine.max_fps = 0
		DisplayServer.window_set_vsync_mode(DisplayServer.VSYNC_DISABLED)
	if _still_path:
		_take_still.call_deferred()


func _process(delta: float) -> void:
	if _still_path:
		return
	if _bench_path and _warmup > 0:
		# The first frames compile shaders and stream textures in; a ride that
		# has settled is what's being measured, and from the top of the replay.
		_warmup -= 1
		return
	_time += delta
	_camera.transform = replay.camera_at(route, _time)
	if _bench_path:
		_frame_times.append(delta)
		_primitives = maxi(
			_primitives,
			RenderingServer.get_rendering_info(
				RenderingServer.RENDERING_INFO_TOTAL_PRIMITIVES_IN_FRAME
			)
		)
		_draw_calls = maxi(
			_draw_calls,
			RenderingServer.get_rendering_info(
				RenderingServer.RENDERING_INFO_TOTAL_DRAW_CALLS_IN_FRAME
			)
		)
	if _time >= replay.duration() and (_bench_path or OS.has_feature("movie") or _is_movie()):
		if _bench_path:
			_write_report()
		get_tree().quit()


## Metrics that say whether the ride was smooth, not just fast on average.
static func summarise(frame_times: PackedFloat32Array) -> Dictionary:
	var sorted := frame_times.duplicate()
	sorted.sort()
	var total := 0.0
	for t: float in sorted:
		total += t
	var count := sorted.size()
	var slowest_percent := 0.0
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
	report["startup_ms"] = _startup_ms
	report["peak_primitives"] = _primitives
	report["peak_draw_calls"] = _draw_calls
	report["render_size"] = str(get_viewport().get_visible_rect().size)
	report["gpu"] = RenderingServer.get_video_adapter_name()
	report["peak_video_memory_mb"] = (
		RenderingServer.get_rendering_info(RenderingServer.RENDERING_INFO_VIDEO_MEM_USED) / 1048576
	)
	var file := FileAccess.open(_bench_path, FileAccess.WRITE)
	file.store_string(JSON.stringify(report, "  ") + "\n")
	file.close()
	print("ride: %s" % report)


func _take_still() -> void:
	# Long enough for the shaders, and for TAA's history to settle.
	for _i: int in 90:
		await RenderingServer.frame_post_draw
	var err := get_viewport().get_texture().get_image().save_png(_still_path)
	if err != OK:
		push_error("ride: cannot write %s: %s" % [_still_path, error_string(err)])
	get_tree().quit()


static func _is_movie() -> bool:
	return "--write-movie" in OS.get_cmdline_args()


static func _counts(plants: Dictionary[Planting.Kind, Array]) -> Dictionary:
	var counts := {}
	for kind: Planting.Kind in plants:
		counts[Planting.Kind.keys()[kind]] = plants[kind].size()
	return counts


static func _arguments() -> Dictionary[String, String]:
	var args: Dictionary[String, String] = {}
	for arg: String in OS.get_cmdline_user_args():
		if arg.begins_with("--") and "=" in arg:
			args[arg.get_slice("=", 0).trim_prefix("--")] = arg.get_slice("=", 1)
	return args
