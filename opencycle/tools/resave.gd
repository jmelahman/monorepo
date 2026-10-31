## Loads and re-saves every .tscn and .tres we own, so a hand-written or
## agent-written file ends up byte for byte what the editor would write:
## defaults dropped, properties in canonical order, uids present. Without it,
## the first person to open a scene in the editor commits a diff they did not
## make.
##
## A headless save strips uids (only the editor assigns them), so this puts
## them back: the file keeps its own uid, or gets the cached or a fresh one,
## and each ext_resource takes its uid from the cache that --import builds.
## scripts/resave.sh runs --import first for that reason.
##
## Usage:
##   godot --headless --path . -s res://tools/resave.gd            # rewrite in place
##   godot --headless --path . -s res://tools/resave.gd -- --check # fail if any would change
extends SceneTree

const ROOTS: PackedStringArray = [
	"res://core", "res://game", "res://sim", "res://tests", "res://tools"
]
const EXTENSIONS: PackedStringArray = ["tscn", "tres"]


func _initialize() -> void:
	var check := "--check" in OS.get_cmdline_user_args()
	# The project root itself, not recursively: default_bus_layout.tres and
	# the like live there, and addons/ does not belong to us.
	var files := _files("res://", false)
	for root_dir: String in ROOTS:
		files.append_array(_files(root_dir, true))
	var changed: PackedStringArray = []
	for path: String in files:
		var before := FileAccess.get_file_as_string(path)
		var after := _resave(path, before)
		if after.is_empty():
			quit(2)
			return
		if after != before:
			changed.append(path)
			if not check:
				_write(path, after)
	if check and changed:
		printerr("resave: %d file(s) differ from what the editor would write;" % changed.size())
		printerr("run scripts/resave.sh and review the diff:\n  " + "\n  ".join(changed))
		quit(1)
		return
	var verb := "would change" if check else "rewritten"
	print("resave: %d files, %d %s" % [files.size(), changed.size(), verb])
	quit(0)


## The canonical text of [param path], or "" if it cannot be loaded or saved.
## Saved over the real path so ext_resource paths stay relative to it, then
## put back; the caller decides whether to keep the result.
func _resave(path: String, before: String) -> String:
	var res := ResourceLoader.load(path, "", ResourceLoader.CACHE_MODE_IGNORE)
	if res == null:
		printerr("resave: cannot load %s" % path)
		return ""
	var err := ResourceSaver.save(res, path)
	if err != OK:
		printerr("resave: cannot save %s (%s)" % [path, error_string(err)])
		return ""
	var after := FileAccess.get_file_as_string(path)
	_write(path, before)
	return _with_uids(path, before, after)


func _with_uids(path: String, before: String, after: String) -> String:
	var lines := after.split("\n")
	if not lines[0].contains(" uid="):
		var uid := _attr(before.get_slice("\n", 0), "uid")
		if uid.is_empty():
			var id := ResourceLoader.get_resource_uid(path)
			if id == ResourceUID.INVALID_ID:
				id = ResourceUID.create_id()
				print("resave: new uid for %s" % path)
			uid = ResourceUID.id_to_text(id)
		lines[0] = lines[0].trim_suffix("]") + ' uid="%s"]' % uid
	for i: int in lines.size():
		var line := lines[i]
		if not line.begins_with("[ext_resource ") or line.contains(" uid="):
			continue
		var id := ResourceLoader.get_resource_uid(_attr(line, "path"))
		if id == ResourceUID.INVALID_ID:
			continue
		# After type=, which is where the editor puts it.
		var type := 'type="%s"' % _attr(line, "type")
		lines[i] = line.replace(type, type + ' uid="%s"' % ResourceUID.id_to_text(id))
	return "\n".join(lines)


static func _attr(line: String, name: String) -> String:
	var key := ' %s="' % name
	var start := line.find(key)
	if start == -1:
		return ""
	start += key.length()
	return line.substr(start, line.find('"', start) - start)


static func _write(path: String, text: String) -> void:
	var file := FileAccess.open(path, FileAccess.WRITE)
	file.store_string(text)
	file.close()


static func _files(dir_path: String, recursive: bool) -> PackedStringArray:
	var out: PackedStringArray = []
	for file: String in DirAccess.get_files_at(dir_path):
		if file.get_extension() in EXTENSIONS:
			out.append(dir_path.path_join(file))
	if recursive:
		for sub: String in DirAccess.get_directories_at(dir_path):
			out.append_array(_files(dir_path.path_join(sub), true))
	return out
