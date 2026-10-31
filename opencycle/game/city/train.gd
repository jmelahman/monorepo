class_name Train
extends Node3D
## A train crossing the street on the ride's clock: gone most of the time, then a long
## bright line through the rail deck.

const HALF_RUN := 170.0
const SPEED := 34.0
const WAIT := 7.0

var centre := Vector3.ZERO
var across := Vector3.RIGHT
var phase := 0.0


func _ready() -> void:
	add_to_group(&"ride_clock")


func set_ride_time(time: float) -> void:
	var lap := (HALF_RUN * 2.0) / SPEED + WAIT
	var into := fposmod(time + phase * lap, lap)
	position = centre + across * (into * SPEED - HALF_RUN)
