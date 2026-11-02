class_name Course
extends Route
## A loop through the city: its roads one after another, as one road the rider can ride. Distance
## wraps at the finish, because that's also the start.


## One road of the course, and how far round the course it starts.
class Leg:
	extends RefCounted
	var segment: CitySegment
	var offset := 0.0


var legs: Array[Leg] = []


func _init(segments: Array[CitySegment]) -> void:
	var total := 0.0
	for segment: CitySegment in segments:
		var leg := Leg.new()
		leg.segment = segment
		leg.offset = total
		legs.append(leg)
		total += segment.road.length
	length = total


func at(s: float) -> Vector3:
	var wrapped := fposmod(s, length)
	var leg := leg_at(wrapped)
	return leg.segment.road.at(wrapped - leg.offset)


func tangent(s: float) -> Vector3:
	var wrapped := fposmod(s, length)
	var leg := leg_at(wrapped)
	return leg.segment.road.tangent(wrapped - leg.offset)


## The leg holding [param s], which has to be between 0 and the course's length.
func leg_at(s: float) -> Leg:
	for index: int in range(legs.size() - 1, -1, -1):
		if s >= legs[index].offset:
			return legs[index]
	return legs[0]


## How much rain gets down to the road, from 0 under a roof to 1.
func open_to_the_sky(s: float) -> float:
	var wrapped := fposmod(s, length)
	var leg := leg_at(wrapped)
	return leg.segment.districts.open_to_the_sky(wrapped - leg.offset)


func kind_at(s: float) -> Districts.Kind:
	var wrapped := fposmod(s, length)
	var leg := leg_at(wrapped)
	return leg.segment.districts.kind_at(wrapped - leg.offset)
