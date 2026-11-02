class_name CitySegment
extends RefCounted
## One road of the city, between two junctions, and what stands along it. It's the same on every
## course that rides it. The road is laid out with the plan; the rest waits until a rider gets
## here, because the far end of the city may never be needed.


## Everything along the road, decided all at once.
class City:
	extends RefCounted
	var street: Street
	var skyline: Skyline
	var undercity: Undercity


var name := ""
var road: Route
var districts: Districts
## Where other roads leave or join this one.
var forks: Array[Fork] = []

var _dice := 0
var _order: Array[Array] = []
var _roads: RoadMap
var _city: City


func _init(called: String, dice: int, along: Route, order: Array[Array], roads: RoadMap) -> void:
	name = called
	road = along
	districts = Districts.generate(dice, along.length, order)
	_dice = dice
	_order = order
	_roads = roads


## Made on whichever thread first wants it, which is only ever one at a time.
func city(kerb: float) -> City:
	if not _city:
		var made := City.new()
		made.street = Street.generate(_dice, road.length, kerb, _order, forks)
		made.skyline = Skyline.generate(_dice, made.street, road, road.length, _roads)
		made.undercity = Undercity.generate(_dice, made.street, kerb, road.length)
		_city = made
	return _city
