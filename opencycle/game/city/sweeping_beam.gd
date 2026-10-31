class_name SweepingBeam
extends SpotLight3D
## A searchlight on a roof, swinging on the ride's clock. Only the fog shows it.

var aim_from := Vector3.UP
var across := Vector3.RIGHT
var along := Vector3.FORWARD
var phase := 0.0


func _ready() -> void:
	add_to_group(&"ride_clock")


func set_ride_time(time: float) -> void:
	var t := time * 0.22 + phase * TAU
	var aim := aim_from + across * sin(t) * 0.55 + along * cos(t * 0.73) * 0.4
	look_at_from_position(position, position + aim.normalized(), Vector3.RIGHT)
