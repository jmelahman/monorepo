extends Node
## Player settings: one [ConfigFile] at [code]user://settings.cfg[/code], applied
## on load and on every change. Settings are not saves: they outlive runs, are
## read before anything else, and a corrupt one costs a preference rather than
## progress, so they skip the versioning in [code]saves.gd[/code] and fall
## back to defaults key by key.
##
## Add a setting by adding it to [constant DEFAULTS] and, if it does something
## on its own, a case in [method _apply]. A view that shows it listens to
## [signal changed].

signal changed(key: String, value: Variant)

const PATH := "user://settings.cfg"
## Bus volumes are linear 0..1, which is what a slider means to a player; the
## decibels are Godot's business.
const DEFAULTS: Dictionary[String, Variant] = {
	"audio/master": 1.0,
	"audio/music": 0.8,
	"audio/sfx": 0.8,
	"display/fullscreen": false,
	"display/vsync": true,
	# Empty means the OS locale.
	"game/locale": "",
}

## Which bus in default_bus_layout.tres each volume drives.
const BUSES: Dictionary[String, String] = {
	"audio/master": "Master",
	"audio/music": "Music",
	"audio/sfx": "SFX",
}

var _config := ConfigFile.new()


func _ready() -> void:
	# A missing file is a first launch and a corrupt one is treated as one:
	# every key falls back to its default below.
	_config.load(PATH)
	for key: String in DEFAULTS:
		_apply(key, get_value(key))


func get_value(key: String) -> Variant:
	assert(key in DEFAULTS, "unknown setting %s" % key)
	var default: Variant = DEFAULTS[key]
	var value: Variant = _config.get_value(_section(key), _name(key), default)
	# A hand-edited file can hold anything; the wrong type is the default.
	return value if typeof(value) == typeof(default) else default


## Typed reads, for callers that hand the value straight to a typed API.
## get_value has already made the type match DEFAULTS, so these cannot fail.
func get_float(key: String) -> float:
	return get_value(key)


func get_bool(key: String) -> bool:
	return get_value(key)


func get_string(key: String) -> String:
	return get_value(key)


func set_value(key: String, value: Variant) -> void:
	assert(key in DEFAULTS, "unknown setting %s" % key)
	if get_value(key) == value:
		return
	_config.set_value(_section(key), _name(key), value)
	var err := _config.save(PATH)
	if err != OK:
		push_warning("settings: could not save %s (%s)" % [PATH, error_string(err)])
	_apply(key, value)
	changed.emit(key, value)


func _apply(key: String, value: Variant) -> void:
	match key:
		"audio/master", "audio/music", "audio/sfx":
			var linear: float = value
			_set_bus_volume(BUSES[key], linear)
		"display/fullscreen":
			# The window is not ours to resize in a test run or under --headless.
			if _owns_window():
				DisplayServer.window_set_mode(
					(
						DisplayServer.WINDOW_MODE_FULLSCREEN
						if value
						else DisplayServer.WINDOW_MODE_WINDOWED
					)
				)
		"display/vsync":
			if _owns_window():
				DisplayServer.window_set_vsync_mode(
					DisplayServer.VSYNC_ENABLED if value else DisplayServer.VSYNC_DISABLED
				)
		"game/locale":
			var locale: String = value
			TranslationServer.set_locale(locale if locale else OS.get_locale())


func _set_bus_volume(bus_name: String, linear: float) -> void:
	var bus := AudioServer.get_bus_index(bus_name)
	if bus == -1:
		push_warning("settings: no audio bus %s in default_bus_layout.tres" % bus_name)
		return
	AudioServer.set_bus_volume_db(bus, linear_to_db(clampf(linear, 0.0, 1.0)))
	AudioServer.set_bus_mute(bus, linear <= 0.0)


static func _owns_window() -> bool:
	return DisplayServer.get_name() != "headless" and not OS.has_environment("GODOT_TEST")


static func _section(key: String) -> String:
	return key.get_slice("/", 0)


static func _name(key: String) -> String:
	return key.get_slice("/", 1)
