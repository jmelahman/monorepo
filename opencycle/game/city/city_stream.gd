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
var _course: Course
var _sections_of_course: Array[StreetSection] = []
var _next := 0
var _opening := true
var _loaded: Array[StreetSection] = []
var _building: StreetSection
var _task := -1


## [param build] makes a [StreetSection]'s root node, all the way round [param course].
func _init(build: Callable, course: Course) -> void:
	_build = build
	_course = course
	name = "CityStream"
	for leg: Course.Leg in course.legs:
		var from := 0.0
		while from < leg.segment.road.length:
			var to := minf(from + SECTION, leg.segment.road.length)
			var section := StreetSection.new(from, to)
			section.segment = leg.segment
			section.offset = leg.offset
			_sections_of_course.append(section)
			from = to


## Builds everything in view of [param s], to have it before the first frame.
func begin(s: float) -> void:
	_next = -_sections_of_course.size()
	while _reached(_next) + _course_from(_next) + SECTION <= s - BEHIND:
		_next += 1
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
	return _reached(_next) + _course_from(_next) < s + AHEAD


## How far the laps before section [param index] come to. Past the finish is the next lap's start.
func _reached(index: int) -> float:
	return floorf(float(index) / _sections_of_course.size()) * _course.length


## How far round its own lap section [param index] starts.
func _course_from(index: int) -> float:
	var plan := _sections_of_course[posmod(index, _sections_of_course.size())]
	return plan.offset + plan.from


func _section() -> StreetSection:
	var plan := _sections_of_course[posmod(_next, _sections_of_course.size())]
	# Whatever runs into a road from before its start has only its first section to go in.
	var section := StreetSection.new(plan.from, plan.to, _opening or plan.from == 0.0)
	section.segment = plan.segment
	section.offset = plan.offset + _reached(_next)
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
		if section.offset + section.until < s - BEHIND:
			left.emit(section.root)
			section.root.queue_free()
			_loaded.remove_at(index)


func _exit_tree() -> void:
	if _task >= 0:
		WorkerThreadPool.wait_for_task_completion(_task)
		_task = -1
