class_name CourseMap
extends Control
## A course as a line, and where the rider is on it. The city is very long and thin, so this is a
## schematic: it stretches across the road to fill its box, as far as [constant MOST_STRETCH].

const MOST_STRETCH := 6.0
const STEPS := 360
const OTHER_STEPS := 60
const RIDER := 7.0
const LINE := 3.0
const OTHER := Color(0.94, 0.92, 0.87, 0.2)

## Metres round the course the rider is, or below nothing to show no rider.
var rider := -1.0:
	set(value):
		rider = value
		queue_redraw()

var _course: Course
var _whole_city := false
var _line := PackedVector2Array()
var _others: Array[PackedVector2Array] = []
var _bounds := Rect2()
var _scale := Vector2.ONE
var _offset := Vector2.ZERO


## Fits [param course] in the box. With [param whole_city] the roads it doesn't take show too.
func show_course(course: Course, whole_city := false) -> void:
	_course = course
	_whole_city = whole_city
	_fit()


func _notification(what: int) -> void:
	if what == NOTIFICATION_RESIZED and _course:
		_fit()


func _fit() -> void:
	var points: Array[PackedVector2Array] = [_sampled(_course, _course.length, STEPS)]
	var ridden: Array[Route] = []
	for leg: Course.Leg in _course.legs:
		ridden.append(leg.segment.road)
	var others: Array[PackedVector2Array] = []
	if _whole_city:
		for road_name: String in _course.city:
			var road := _course.city[road_name].road
			if road not in ridden:
				others.append(_sampled(road, road.length, OTHER_STEPS))
	_bounds = Rect2(points[0][0], Vector2.ZERO)
	for line: PackedVector2Array in points + others:
		for point: Vector2 in line:
			_bounds = _bounds.expand(point)
	var room := size - Vector2.ONE * RIDER * 4.0
	var wide := room.x / maxf(_bounds.size.x, 1.0)
	var high := room.y / maxf(_bounds.size.y, 1.0)
	_scale = Vector2(wide, minf(high, wide * MOST_STRETCH))
	if high < wide:
		_scale = Vector2(minf(wide, high * MOST_STRETCH), high)
	_offset = size * 0.5 - _bounds.get_center() * _scale
	_line = _placed(points[0])
	_others.clear()
	for line: PackedVector2Array in others:
		_others.append(_placed(line))
	queue_redraw()


static func _sampled(road: Route, long: float, steps: int) -> PackedVector2Array:
	var line := PackedVector2Array()
	for step: int in steps + 1:
		var at := road.at(long * step / steps)
		line.append(Vector2(at.x, at.z))
	return line


func _placed(line: PackedVector2Array) -> PackedVector2Array:
	var placed := PackedVector2Array()
	for point: Vector2 in line:
		placed.append(point * _scale + _offset)
	return placed


func _draw() -> void:
	if _line.is_empty():
		return
	for line: PackedVector2Array in _others:
		draw_polyline(line, OTHER, LINE * 0.7, true)
	draw_polyline(_line, UiStyle.INK if _whole_city else Color(UiStyle.INK, 0.7), LINE, true)
	# The start line, across the road.
	draw_circle(_line[0], LINE * 1.5, UiStyle.COLD)
	if rider >= 0.0 and _course:
		var at := _course.at(rider)
		var place := Vector2(at.x, at.z) * _scale + _offset
		draw_circle(place, RIDER + 3.0, Color(0.0, 0.0, 0.0, 0.6))
		draw_circle(place, RIDER, UiStyle.ACCENT)
