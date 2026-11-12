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
- **The coast:** a lime strip along both road edges (the ground paint, not blades); the far fields are flat texture with no
  blades.
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
4. More dramatic terrain at the next reshape: ridges, cliffs, cuttings and gorges that frame
   the road, with big vistas between them.
5. Later: HLODs for the far landscape and trees, L_World's own ground material, per-island
   grass presets, and PCG placement at world scale.
