class_name StreetSection
extends RefCounted
## One stretch of street being built. The sim lays out the whole route; every builder walks
## all of it and makes only what's anchored here, so nothing is made twice and index-based
## dice come out the same however the street is cut up.

var from := 0.0
var to := 0.0
## Whatever started before this section and carries on into it belongs to the first section
## built, because the one it started in never will be.
var first := false
## How far along the rider can be and still see something made here.
var until := 0.0
var root: Node3D
## Which road of the city this is on, and how far round the course that road starts.
var segment: CitySegment
var offset := 0.0
## Where a road this course doesn't ride stops being built, or below 0 if it doesn't.
var ends_at := -1.0


func _init(start: float, end: float, opening := false) -> void:
	from = start
	to = end
	until = end
	first = opening


func holds(s: float) -> bool:
	return s >= from and s < to


## Whether a long piece running [param start] to [param end] is made here. The section keeps
## it until the rider is past its far end.
func takes(start: float, end: float) -> bool:
	if holds(start) or (first and start < from and end > from):
		until = maxf(until, end)
		return true
	return false
