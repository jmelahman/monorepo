## Compiles every script we own, with the autoloads in scope, and fails on any
## error or warning. Godot's own --check-only cannot do this: it runs without
## autoloads, so every script naming Settings or Saves fails there, and it has
## no way to make a warning fatal. project.godot keeps the unsafe_* family at
## warn so the editor stays cheap to prototype in; this is where they stop.
##
## Usage (scripts/lint.sh runs it):
##   godot --headless --path . -d -s res://tools/check.gd
## -d matters: GDScript reports warnings only with a debugger attached.
extends SceneTree

## Everything but the third-party addons, which setup.sh fetches. The agent
## bridge lives under addons/ but is ours, which is why project.godot's
## directory_rules turn warnings back on for it: setting that from here does
## nothing, since the rules are read once at startup.
const ROOTS: PackedStringArray = [
	"res://addons/agent_bridge",
	"res://core",
	"res://game",
	"res://sim",
	"res://tests",
	"res://tools"
]


class _Collector:
	extends Logger
	var problems: PackedStringArray = []

	func _log_error(
		_function: String,
		file: String,
		line: int,
		code: String,
		rationale: String,
		_editor_notify: bool,
		error_type: int,
		_script_backtraces: Array[ScriptBacktrace]
	) -> void:
		var kind := "warning" if error_type == ERROR_TYPE_WARNING else "error"
		problems.append("%s:%d: %s: %s" % [file, line, kind, rationale if rationale else code])


func _initialize() -> void:
	var collector := _Collector.new()
	OS.add_logger(collector)
	var own: Script = get_script()
	var count := 0
	for root_dir: String in ROOTS:
		for path: String in _scripts(root_dir):
			# Reloading the running script out from under itself crashes the VM,
			# and this file was compiled before the logger existed, so its own
			# warnings print but cannot fail the run. Keep it small.
			if path == own.resource_path:
				continue
			count += 1
			if ResourceLoader.load(path, "", ResourceLoader.CACHE_MODE_IGNORE) == null:
				collector.problems.append("%s: does not compile" % path)
	OS.remove_logger(collector)
	for problem: String in collector.problems:
		printerr(problem)
	print("check: %d scripts, %d problem(s)" % [count, collector.problems.size()])
	quit(1 if collector.problems else 0)


static func _scripts(dir_path: String) -> PackedStringArray:
	var out: PackedStringArray = []
	for file: String in DirAccess.get_files_at(dir_path):
		if file.get_extension() == "gd":
			out.append(dir_path.path_join(file))
	for sub: String in DirAccess.get_directories_at(dir_path):
		out.append_array(_scripts(dir_path.path_join(sub)))
	return out
