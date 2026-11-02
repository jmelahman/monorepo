class_name Session
extends RefCounted
## What the menu chose, for the ride it starts, and for next time.

const PATH := "user://session.cfg"

static var course := "inner"


static func recall(path := PATH) -> void:
	var file := ConfigFile.new()
	if file.load(path) != OK:
		return
	var stored: Variant = file.get_value("ride", "course", course)
	if stored is String and stored in CityPlan.COURSES:
		course = stored


static func remember(path := PATH) -> void:
	var file := ConfigFile.new()
	file.set_value("ride", "course", course)
	var err := file.save(path)
	if err != OK:
		push_warning("session: cannot save %s: %s" % [path, error_string(err)])
