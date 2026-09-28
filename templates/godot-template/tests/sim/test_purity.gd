extends GdUnitTestSuite
## sim/ is a pure function of its inputs, and this enforces it rather than
## trusting review to: no nodes or scene tree, no engine singletons that reach
## the clock, the filesystem, the OS or the global RNG, and no loading of
## anything outside sim/. A sim that stays pure can be replayed, simulated in
## bulk for balance and tested without a scene; one exception quietly ends all
## three.

const SIM := "res://sim"
## Word-bounded, so `rolls` does not trip on `OS` and `turn` on `Time`.
const FORBIDDEN: PackedStringArray = [
	"Node",
	"Node2D",
	"Node3D",
	"Control",
	"SceneTree",
	"get_tree",
	"get_node",
	"Engine",
	"OS",
	"Time",
	"FileAccess",
	"DirAccess",
	"Input",
	"ResourceLoader",
	"randi",
	"randf",
	"randi_range",
	"randf_range",
	"randomize",
	"RandomNumberGenerator",
	"Settings",
	"Saves",
	"Platform",
	"Dev",
]


func test_sim_touches_nothing_outside_itself() -> void:
	var pattern := RegEx.create_from_string("\\b(%s)\\b" % "|".join(FORBIDDEN))
	var loads := RegEx.create_from_string('(pre)?load\\("(?<path>[^"]+)"\\)')
	var comment := RegEx.create_from_string("#.*$")
	var files := _scripts(SIM)
	assert_array(files).is_not_empty()
	for path: String in files:
		var lines := FileAccess.get_file_as_string(path).split("\n")
		for n: int in lines.size():
			var code := comment.sub(lines[n], "")
			var hit := pattern.search(code)
			if hit:
				fail("%s:%d uses %s: %s" % [path, n + 1, hit.get_string(1), lines[n].strip_edges()])
			for m: RegExMatch in loads.search_all(code):
				if not m.get_string("path").begins_with(SIM + "/"):
					fail("%s:%d loads %s from outside sim/" % [path, n + 1, m.get_string("path")])


func _scripts(dir_path: String) -> PackedStringArray:
	var out: PackedStringArray = []
	for file: String in DirAccess.get_files_at(dir_path):
		if file.get_extension() == "gd":
			out.append(dir_path.path_join(file))
	for sub: String in DirAccess.get_directories_at(dir_path):
		out.append_array(_scripts(dir_path.path_join(sub)))
	return out
