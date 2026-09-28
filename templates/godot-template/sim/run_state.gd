class_name RunState
extends RefCounted
## Everything a run is, and nothing it is not: no nodes, no resources, no
## references into the scene tree. It converts to and from plain JSON-safe
## dictionaries, which is what the save file stores and what the round-trip
## test holds it to, so a field that cannot survive [method to_dict] cannot
## quietly enter it.
##
## The sample game is push-your-luck dice: roll to grow the pot, bank to keep
## it, and a 1 loses the pot. Reach [constant Rules.TARGET] to win.

var run_seed: int
## Turns taken; also the first coordinate of every die, see [Rng].
var turn: int = 0
## Rolls this turn; the second coordinate.
var rolls: int = 0
var pot: int = 0
var score: int = 0
var won: bool = false


static func start(from_seed: int) -> RunState:
	var s := RunState.new()
	s.run_seed = from_seed
	return s


func duplicate_state() -> RunState:
	return RunState.from_dict(to_dict())


func to_dict() -> Dictionary[String, Variant]:
	return {
		"seed": run_seed,
		"turn": turn,
		"rolls": rolls,
		"pot": pot,
		"score": score,
		"won": won,
	}


## JSON has one number type, so ints come back as floats and are narrowed here.
## A missing field takes its default: that is what lets a save gain an optional
## field without a version bump (see [code]core/saves.gd[/code]).
static func from_dict(d: Dictionary) -> RunState:
	var s := RunState.new()
	s.run_seed = _int(d.get("seed", 0))
	s.turn = _int(d.get("turn", 0))
	s.rolls = _int(d.get("rolls", 0))
	s.pot = _int(d.get("pot", 0))
	s.score = _int(d.get("score", 0))
	s.won = _int(d.get("won", false)) != 0
	return s


## JSON's float, or an int or a bool, as an int. type_convert rather than int()
## because the value is a Variant, and int(Variant) is the unsafe call the
## project's warnings exist to catch.
static func _int(value: Variant) -> int:
	return type_convert(value, TYPE_INT)
