extends Node
## Versioned JSON saves in [code]user://saves/<slot>.json[/code]. The file is an
## envelope, [code]{"version": N, "data": {...}}[/code]; [code]data[/code] is
## whatever the caller hands over, which for a run is [method RunState.to_dict].
##
## Two rules keep old saves loading. Adding an optional field needs nothing:
## [method RunState.from_dict] defaults what is missing. Changing what an
## existing field means bumps [constant VERSION] and adds a step to
## [method _migrate] that rewrites the old shape into the new one, so a save
## from any earlier version walks forward one step at a time. A save from a
## newer build than this one is refused rather than guessed at.
##
## Writes are atomic: the new file is written beside the old one and renamed
## over it, with the previous save kept as [code].bak[/code], because a crash or
## a power cut halfway through a write should cost the last move, not the run.

const VERSION := 1
const DIR := "user://saves"


func write(slot: String, data: Dictionary) -> Error:
	var err := DirAccess.make_dir_recursive_absolute(DIR)
	if err != OK:
		return err
	var path := _path(slot)
	var tmp := path + ".tmp"
	var file := FileAccess.open(tmp, FileAccess.WRITE)
	if file == null:
		return FileAccess.get_open_error()
	file.store_string(JSON.stringify({"version": VERSION, "data": data}, "\t"))
	file.close()
	if FileAccess.file_exists(path):
		# Rename rather than copy: on every desktop filesystem it cannot leave
		# a half-written file behind. Windows will not rename over an existing
		# file, hence moving the old one aside first.
		DirAccess.remove_absolute(path + ".bak")
		err = DirAccess.rename_absolute(path, path + ".bak")
		if err != OK:
			return err
	return DirAccess.rename_absolute(tmp, path)


## The migrated [code]data[/code] of [param slot], or an empty dictionary when
## there is no usable save. A corrupt file falls back to the previous one.
func read(slot: String) -> Dictionary:
	var path := _path(slot)
	for candidate: String in [path, path + ".bak"]:
		if not FileAccess.file_exists(candidate):
			continue
		var data := _read_file(candidate)
		if not data.is_empty():
			return data
		push_warning("saves: %s is unreadable; trying the backup" % candidate)
	return {}


func has(slot: String) -> bool:
	return FileAccess.file_exists(_path(slot)) or FileAccess.file_exists(_path(slot) + ".bak")


func erase(slot: String) -> void:
	for suffix: String in ["", ".bak", ".tmp"]:
		DirAccess.remove_absolute(_path(slot) + suffix)


func _read_file(path: String) -> Dictionary:
	# An instance rather than JSON.parse_string, which logs an engine error for
	# the corrupt file this is prepared to meet.
	var json := JSON.new()
	if json.parse(FileAccess.get_file_as_string(path)) != OK or not json.data is Dictionary:
		return {}
	var envelope: Dictionary = json.data
	var version: int = type_convert(envelope.get("version", 0), TYPE_INT)
	if version < 1 or not envelope.get("data") is Dictionary:
		return {}
	var data: Dictionary = envelope["data"]
	if version > VERSION:
		push_warning("saves: %s is from a newer build (v%d > v%d)" % [path, version, VERSION])
		return {}
	return _migrate(data, version)


## Rewrites [param data] from [param from] up to [constant VERSION], one version
## at a time. A new step looks like:
## [codeblock]
## if from < 2:
##     data["best"] = data.get("high_score", 0)  # v2 renamed high_score
##     data.erase("high_score")
## [/codeblock]
static func _migrate(data: Dictionary, from: int) -> Dictionary:
	assert(from <= VERSION)
	return data


static func _path(slot: String) -> String:
	assert(slot.is_valid_filename(), "save slot must be a plain file name")
	return DIR.path_join(slot + ".json")
