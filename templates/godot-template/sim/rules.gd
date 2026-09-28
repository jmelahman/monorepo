class_name Rules
extends RefCounted
## The game's rules as one pure function: [method reduce] takes a state and an
## action and returns the next state and what happened. It touches no node, no
## clock, no file and no global RNG, and [code]tests/sim/test_purity.gd[/code]
## enforces that rather than trusting discipline, because everything built on
## it depends on it: replays, saves, balance simulations that run a thousand
## games in a second, and tests that need no scene.
##
## Actions and events are plain dictionaries with a [code]"type"[/code], so an
## action log is a JSON array and a replay is a loop. The sim authors no prose:
## a refusal is a code, and the presentation layer owns the sentence (and its
## translation).

const TARGET := 50
## Rolling this ends the turn and loses the pot.
const BUST := 1


class Step:
	extends RefCounted
	var state: RunState
	var events: Array[Dictionary]

	func _init(next: RunState, happened: Array[Dictionary]) -> void:
		state = next
		events = happened


static func reduce(state: RunState, action: Dictionary) -> Step:
	if state.won:
		return _refuse(state, &"game_over")
	match action.get("type", &""):
		&"roll":
			return _roll(state)
		&"bank":
			return _bank(state)
		var other:
			return _refuse(state, &"unknown_action", {"action": str(other)})


## Replays [param actions] from a fresh run of [param run_seed]; the last state.
static func replay(run_seed: int, actions: Array[Dictionary]) -> RunState:
	var state := RunState.start(run_seed)
	for action: Dictionary in actions:
		state = reduce(state, action).state
	return state


static func _roll(state: RunState) -> Step:
	var next := state.duplicate_state()
	var value := Rng.range_at(state.run_seed, [state.turn, state.rolls], 1, 6)
	var events: Array[Dictionary] = [{"type": &"rolled", "value": value}]
	next.rolls += 1
	if value == BUST:
		events.append({"type": &"busted", "lost": next.pot})
		_end_turn(next)
	else:
		next.pot += value
	return Step.new(next, events)


static func _bank(state: RunState) -> Step:
	if state.pot == 0:
		return _refuse(state, &"empty_pot")
	var next := state.duplicate_state()
	var events: Array[Dictionary] = [{"type": &"banked", "amount": next.pot}]
	next.score += next.pot
	_end_turn(next)
	if next.score >= TARGET:
		next.won = true
		events.append({"type": &"won", "turns": next.turn})
	return Step.new(next, events)


static func _end_turn(state: RunState) -> void:
	state.pot = 0
	state.rolls = 0
	state.turn += 1


## The state is returned unchanged; the refusal is an event like any other, so
## the presentation layer can say why without the sim writing the sentence.
static func _refuse(state: RunState, code: StringName, operands: Dictionary = {}) -> Step:
	var event: Dictionary = {"type": &"refused", "code": code}
	event.merge(operands)
	return Step.new(state, [event])
