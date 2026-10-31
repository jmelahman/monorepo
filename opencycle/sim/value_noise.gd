class_name ValueNoise
extends RefCounted
## Seeded value noise, built on [Rng] so a world's shape is stable between Godot
## versions, which [FastNoiseLite] does not promise. Lattice points are cached:
## a world samples the same few thousand of them millions of times.

var _seed: int
var _lattice: Dictionary[int, float] = {}


func _init(run_seed: int) -> void:
	_seed = run_seed


## Smooth noise in [code][-1, 1][/code], with features about one unit across.
func value(x: float, z: float) -> float:
	var ix := floori(x)
	var iz := floori(z)
	var fx := x - ix
	var fz := z - iz
	var sx := fx * fx * (3.0 - 2.0 * fx)
	var sz := fz * fz * (3.0 - 2.0 * fz)
	var near := lerpf(_at(ix, iz), _at(ix + 1, iz), sx)
	var far := lerpf(_at(ix, iz + 1), _at(ix + 1, iz + 1), sx)
	return lerpf(near, far, sz)


## [param octaves] layers of [method value], each twice as fine and half as tall,
## normalised back to about [code][-1, 1][/code].
func fractal(x: float, z: float, octaves: int) -> float:
	var total := 0.0
	var amplitude := 1.0
	var weight := 0.0
	for octave: int in octaves:
		# The offset keeps the octaves' lattices from lining up at the origin.
		total += amplitude * value(x + octave * 17.3, z - octave * 9.1)
		weight += amplitude
		amplitude *= 0.5
		x *= 2.0
		z *= 2.0
	return total / weight


func _at(ix: int, iz: int) -> float:
	var key := (ix << 24) ^ (iz & 0xFFFFFF)
	if key in _lattice:
		return _lattice[key]
	var v := float(Rng.derive(_seed, [ix, iz])) / float(Rng.MASK) * 2.0 - 1.0
	_lattice[key] = v
	return v
