# Veloverse: the world plan

The plan of record for the hub world, `/Game/World/L_World`. Keep this current: what exists,
what's wrong and what's next. Put the history in the storybook, not here.

## The world

Harbour Isle sits in the middle: a coast road, the harbour town and a quay. Four islands hang
off it, each on two bridges:

| Island | Biome |
| --- | --- |
| Greenwood | broadleaf forest |
| Vael | mountain: spruce, rock, snow |
| The Moor | heath and open grass |
| Red Sands | desert and canyon rock |

`ROUTES` in `make_chain.py` lists the rides. Any set of islands makes a loop with no road ridden
twice. `climb_world.py` writes the first route as `DA_RideTrack` and the rest as
`DA_Route_<key>`. CLAUDE.md, "Building the world", has the pipeline.

## Open problems (from the latest stills)

- **The Moor:** the path reads too bright.
- **Greenwood:** no grass or plants under the trees, though the Forest layer is painted there.
- **The coast:** the far fields are flat texture with no blades. (The lime strip along the road
  edges was the road rim's ground-RVT blend; off from v43.)
- **The quay:** the sand reads as cracked mud.
- **Vael:** the slopes wash out in the sun.

## Next

1. Fix the open problems above.
2. A sparse clump layer from the grass cull out to 150–200 m, so the carpet doesn't stop at
   the cull.
3. Bring in the Fab assets as they arrive, in this order:
   - meadow ground cover: grasses, white, yellow and purple wildflowers, ferns and low shrubs;
   - alpine cliffs, scree and snow-capped rock;
   - clutter: logs, stumps, stones and moss;
   - ground textures: coastal grass and sand, and fine sand or shingle for the quay.
4. The reshape, from the AAA gap review (`/workspace/ue/out/qa/aaa_gap_review.md`):
   - in v44: droplet erosion on the outer islands, a 1150 m Vael (tree line 820 m, snow 900 m),
     strata benches on steep rock, Fab cliff faces on steep ground near the road with scree below,
     no pylons, and the rider and stills in the right lane;
   - written for v45: island roads bent round the land (`contour()`), sea cliffs on about half
     the hub's shore with the coast road 45-70 m from the edge (`SEA_CLIFF`, `hug()`), and sea
     stacks off the cliffs (`sea_stacks()`);
   - still to come: a gorge on Red Sands, real trees in clustered size classes with an
     understory, a Heath layer for the Moor, walls, fences, hamlets and a quay, and water with a
     shoreline.
   The overcast lighting from v43 stays.
5. Later: HLODs for the far landscape and trees, L_World's own ground material, per-island
   grass presets, and PCG placement at world scale.
