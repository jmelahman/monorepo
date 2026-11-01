class_name CityStream
extends Node3D
## The street as the rider comes to it: sections built on another thread far enough ahead to
## be lost in the fog, and thrown away once behind. Sections only ever go forwards, one
## building at a time, so builders have their static caches to themselves.

signal arrived(section: Node3D)
signal left(section: Node3D)

## A whole number of [constant CityMesh.CHUNK], so a section is a set of the same meshes.
const SECTION := CityMesh.CHUNK * 2.0
const BEHIND := 96.0
## Where the distance fog closes in.
const AHEAD := 480.0

## Off for stills and movies, which must come out the same however long a build takes.
var threaded := true

var _build: Callable
var _end := 0.0
var _next := 0
var _opening := true
var _loaded: Array[StreetSection] = []
var _building: StreetSection
var _task := -1


## [param build] makes a [StreetSection]'s root node; the street stops at [param end].
func _init(build: Callable, end: float) -> void:
	_build = build
	_end = end
	name = "CityStream"


## Builds everything in view of [param s], to have it before the first frame.
func begin(s: float) -> void:
	_next = floori((s - BEHIND) / SECTION)
	while _wanted(s):
		_arrive(_made(_section()))


func update(s: float) -> void:
	_drop_behind(s)
	if _task >= 0:
		if not WorkerThreadPool.is_task_completed(_task):
			return
		WorkerThreadPool.wait_for_task_completion(_task)
		_task = -1
		_arrive(_building)
		_building = null
	if not _wanted(s):
		return
	if threaded:
		_building = _section()
		_task = WorkerThreadPool.add_task(_made.bind(_building))
	else:
		_arrive(_made(_section()))


func _wanted(s: float) -> bool:
	var from := _next * SECTION
	return from < s + AHEAD and from < _end


func _section() -> StreetSection:
	var from := _next * SECTION
	var section := StreetSection.new(from, from + SECTION, _opening)
	_next += 1
	_opening = false
	return section


func _made(section: StreetSection) -> StreetSection:
	section.root = _build.call(section)
	return section


func _arrive(section: StreetSection) -> void:
	add_child(section.root)
	_loaded.append(section)
	arrived.emit(section.root)


func _drop_behind(s: float) -> void:
	for index: int in range(_loaded.size() - 1, -1, -1):
		var section := _loaded[index]
		if section.until < s - BEHIND:
			left.emit(section.root)
			section.root.queue_free()
			_loaded.remove_at(index)


func _exit_tree() -> void:
	if _task >= 0:
		WorkerThreadPool.wait_for_task_completion(_task)
		_task = -1
