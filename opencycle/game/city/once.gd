class_name Once
extends RefCounted
## Materials and kits every section of street shares, made the first time one is asked for.
## Sharing them keeps sections from loading models and piling up materials as the ride goes on.

static var _made: Dictionary[String, Variant] = {}


static func made(key: String, make: Callable) -> Variant:
	if key not in _made:
		_made[key] = make.call()
	return _made[key]
