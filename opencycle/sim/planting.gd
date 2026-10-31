class_name Planting
extends RefCounted
## What grows where. The rules, not the art: each kind of thing is scattered over
## a jittered grid, and each cell decides from the seed and the ground whether it
## grows anything, so a change to one kind never reshuffles another.
##
## The biome is a conifer forest. Firs take the shaded, damper ground and pines
## the sunnier slopes; both thin out on steep ground and leave clearings. Young
## trees crowd the forest's edge, where the light reaches them. Ferns and rocks
## gather under the trees and grass takes the open verges.

enum Kind { FIR, PINE, FERN, GRASS, ROCK, BOULDER, STUMP }

## The compass direction a slope faces that gets the most sun, in plan.
const SUNWARD := Vector2(0.5, 0.866)
const TREE_CELL := 5.0
const TREE_NEAR_REACH := 180.0
## Further out only the canopy shows, so fewer, larger trees make the same forest.
const FAR_TREE_CELL := 10.0
const FAR_TREE_SIZE := 1.35
const TREE_REACH := 620.0
const FERN_CELL := 1.1
const FERN_REACH := 34.0
const GRASS_CELL := 0.55
const GRASS_REACH := 11.0
const ROCK_CELL := 7.0
const ROCK_REACH := 70.0
const FOREST_SCALE := 1.0 / 90.0
const EDGE_SCALE := 1.0 / 45.0
const CLUMP_SCALE := 1.0 / 14.0


## A list of transforms for each [enum Kind].
static func generate(run_seed: int, terrain: Terrain) -> Dictionary[Kind, Array]:
	var forest := ValueNoise.new(Rng.derive(run_seed, [10]))
	var clumps := ValueNoise.new(Rng.derive(run_seed, [11]))
	var plants: Dictionary[Kind, Array] = {}
	for kind: int in Kind.values():
		plants[kind] = [] as Array[Transform3D]
	_scatter(
		run_seed,
		terrain,
		20,
		TREE_CELL,
		0.0,
		TREE_NEAR_REACH,
		_grow_tree.bind(terrain, forest, plants, 1.0)
	)
	_scatter(
		run_seed,
		terrain,
		24,
		FAR_TREE_CELL,
		TREE_NEAR_REACH,
		TREE_REACH,
		_grow_tree.bind(terrain, forest, plants, FAR_TREE_SIZE)
	)
	_scatter(
		run_seed,
		terrain,
		21,
		FERN_CELL,
		0.0,
		FERN_REACH,
		_grow_fern.bind(terrain, forest, clumps, plants)
	)
	_scatter(
		run_seed,
		terrain,
		22,
		GRASS_CELL,
		0.0,
		GRASS_REACH,
		_grow_grass.bind(terrain, clumps, plants)
	)
	_scatter(run_seed, terrain, 23, ROCK_CELL, 0.0, ROCK_REACH, _grow_rock.bind(terrain, plants))
	return plants


static func _grow_tree(
	x: float,
	z: float,
	offset: float,
	cell: int,
	terrain: Terrain,
	forest: ValueNoise,
	plants: Dictionary[Kind, Array],
	scale: float,
) -> void:
	var cover := forest.fractal(x * FOREST_SCALE, z * FOREST_SCALE, 2)
	# The forest's edge wanders towards the road and away from it.
	var edge := forest.value(x * EDGE_SCALE + 40.0, z * EDGE_SCALE)
	var margin := 7.0 + (edge + 1.0) * 9.0
	var clear := absf(offset) - Route.HALF_WIDTH - margin
	if clear < 0.0 or Rng.unit(cell, 0) > 0.62 + cover * 1.4:
		return
	var ground := terrain.normal(x, z, 3.0)
	if ground.y < 0.78:
		return
	var sunny := Vector2(ground.x, ground.z).dot(SUNWARD) * 3.0
	var kind := Kind.PINE if sunny + cover * 0.8 + Rng.unit(cell, 1) > 0.9 else Kind.FIR
	# Mostly grown trees, some young ones; at the edge, mostly young ones.
	var youth := pow(Rng.unit(cell, 2), 1.6 if clear > 12.0 else 3.5)
	var size := lerpf(0.22, 1.3, 1.0 - youth) * scale
	plants[kind].append(_placed(cell, terrain, x, z, size))


static func _grow_fern(
	x: float,
	z: float,
	offset: float,
	cell: int,
	terrain: Terrain,
	forest: ValueNoise,
	clumps: ValueNoise,
	plants: Dictionary[Kind, Array],
) -> void:
	if absf(offset) < Route.HALF_WIDTH + 2.5:
		return
	var clump := clumps.value(x * CLUMP_SCALE, z * CLUMP_SCALE)
	var shade := forest.value(x * FOREST_SCALE, z * FOREST_SCALE)
	if Rng.unit(cell, 0) > 0.3 + clump * 0.75 + shade * 0.3:
		return
	plants[Kind.FERN].append(_placed(cell, terrain, x, z, lerpf(0.9, 1.9, Rng.unit(cell, 2))))


static func _grow_grass(
	x: float,
	z: float,
	offset: float,
	cell: int,
	terrain: Terrain,
	clumps: ValueNoise,
	plants: Dictionary[Kind, Array],
) -> void:
	if absf(offset) < Route.HALF_WIDTH + 1.0:
		return
	var clump := clumps.value(x * CLUMP_SCALE * 2.0, z * CLUMP_SCALE * 2.0)
	if Rng.unit(cell, 0) > 0.6 + clump * 0.5:
		return
	plants[Kind.GRASS].append(_placed(cell, terrain, x, z, lerpf(1.0, 2.0, Rng.unit(cell, 2))))


static func _grow_rock(
	x: float,
	z: float,
	offset: float,
	cell: int,
	terrain: Terrain,
	plants: Dictionary[Kind, Array],
) -> void:
	if absf(offset) < Route.HALF_WIDTH + 2.0:
		return
	var roll := Rng.unit(cell, 0)
	if roll > 0.3:
		return
	var kind := Kind.ROCK
	var size := lerpf(0.4, 1.5, Rng.unit(cell, 2))
	if roll < 0.02:
		kind = Kind.BOULDER
		size *= 0.6
	elif roll < 0.05:
		kind = Kind.STUMP
		size = 1.0
	plants[kind].append(_placed(cell, terrain, x, z, size))


## Calls [param grow] for one jittered point in every cell of a grid between
## [param inner] and [param outer] metres from the road, with its plan position,
## its signed distance from the road and a value to draw its randomness from.
static func _scatter(
	run_seed: int,
	terrain: Terrain,
	layer: int,
	size: float,
	inner: float,
	outer: float,
	grow: Callable,
) -> void:
	var rect := terrain.route.extent().grow(outer).intersection(terrain.bounds)
	var base := Rng.derive(run_seed, [layer])
	for row: int in int(rect.size.y / size):
		var row_base := Rng.derive(base, [row])
		for column: int in int(rect.size.x / size):
			var cell := Rng.derive(row_base, [column])
			var x := rect.position.x + (column + Rng.unit(cell, 3)) * size
			var z := rect.position.y + (row + Rng.unit(cell, 4)) * size
			var offset := terrain.route.nearest(x, z).y
			var distance := absf(offset)
			if distance >= inner and distance < outer:
				grow.call(x, z, offset, cell)


static func _placed(cell: int, terrain: Terrain, x: float, z: float, size: float) -> Transform3D:
	var basis := Basis(Vector3.UP, Rng.unit(cell, 5) * TAU).scaled(Vector3.ONE * size)
	return Transform3D(basis, Vector3(x, terrain.height(x, z), z))
