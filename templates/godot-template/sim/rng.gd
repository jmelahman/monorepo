class_name Rng
extends RefCounted
## Stateless, seeded randomness for the sim: every draw is a pure function of
## the run's seed and the coordinates of the draw ([code]derive(run_seed, turn,
## roll)[/code]), so replaying the same actions from the same seed replays the
## same dice, and adding a draw somewhere never shifts every draw after it.
##
## Not [RandomNumberGenerator]: its state would have to live in the run state
## and advance in lockstep with every caller, and not [method @GlobalScope.hash],
## which Godot does not promise to keep stable between versions. This is
## murmur3's finaliser over 32-bit words, done in 16-bit halves because
## GDScript ints are signed 64-bit and a full 32x32 product can overflow them.

const MASK := 0xFFFFFFFF


## A well-mixed 32-bit value for [param run_seed] at [param coords].
static func derive(run_seed: int, coords: Array[int]) -> int:
	var h := run_seed & MASK
	for c: int in coords:
		h = _mix(h ^ _mix(c & MASK))
	return h


## A uniform integer in [code][lo, hi][/code] (inclusive) at [param coords]. The
## modulo bias is below one in 2^27 for ranges this small, which no player will
## ever meet.
static func range_at(run_seed: int, coords: Array[int], lo: int, hi: int) -> int:
	return lo + derive(run_seed, coords) % (hi - lo + 1)


static func _mix(x: int) -> int:
	x ^= x >> 16
	x = _mul32(x, 0x85EBCA6B)
	x ^= x >> 13
	x = _mul32(x, 0xC2B2AE35)
	x ^= x >> 16
	return x


static func _mul32(a: int, b: int) -> int:
	var lo := a * (b & 0xFFFF)
	var hi := (a * (b >> 16)) & 0xFFFF
	return (lo + (hi << 16)) & MASK
