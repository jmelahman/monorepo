class_name ForestBiome
extends RefCounted
## A conifer forest: rough hills, a country road, firs and pines, ferns and rocks.


static func build(parent: Node3D, run_seed: int, route: Route, quality: String) -> void:
	var terrain := Terrain.generate(run_seed, route)
	var plants := Planting.generate(run_seed, terrain)
	parent.add_child(TerrainMesh.build(terrain))
	parent.add_child(RoadMesh.build(route))
	parent.add_child(Flora.build(plants, route))
	Look.apply(parent, quality)
