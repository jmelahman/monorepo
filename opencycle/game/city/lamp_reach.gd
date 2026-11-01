class_name LampReach
extends Node
## Which of the street's lamps are worth paying for, from where the rider is. Close ones throw
## shadows, further ones only light, and past that they're out: the ride passes hundreds, and
## a shadow redrawn for a lamp down the road was most of the frame.

const SHADOWS_WITHIN := 35.0
const LIT_WITHIN := 170.0
## So a lamp on the line between two states does not flicker.
const MARGIN := 3.0

var shadows_within := SHADOWS_WITHIN
var lit_within := LIT_WITHIN
var _lamps: Array[Light3D] = []
var _casts := PackedByteArray()


## Takes on every lamp below [param root] but the sky's, whose reach isn't a matter of distance.
func gather(root: Node) -> void:
	_lamps.clear()
	_casts.clear()
	for node: Node in root.find_children("*", "Light3D", true, false):
		if node is DirectionalLight3D:
			continue
		var lamp := node as Light3D
		_lamps.append(lamp)
		_casts.append(1 if lamp.shadow_enabled else 0)


func update(rider: Vector3) -> void:
	for index: int in _lamps.size():
		var lamp := _lamps[index]
		var away := lamp.global_position.distance_to(rider)
		# Setting these redraws the lamp's shadow, even to what they already were.
		var lit := away < lit_within + (MARGIN if lamp.visible else 0.0)
		if lamp.visible != lit:
			lamp.visible = lit
		if _casts[index] == 1:
			var casts := away < shadows_within + (MARGIN if lamp.shadow_enabled else 0.0)
			if lamp.shadow_enabled != casts:
				lamp.shadow_enabled = casts
