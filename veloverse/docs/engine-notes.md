# Veloverse: engine notes

How things work and the tips that save time. `CLAUDE.md` has the goal and the rules, and wins
where the two differ. The old plans are in `docs/archive/`, for history only.

## The look and `oc.py look`

- `look` sets parameters without a rebuild, then takes stills. Always pass `--shots`. Its flags:
  - `--tint LAYER=r,g,b` and `--grey LAYER=0..1`: ground colours.
  - `--grass DENSITY CULL` and `--fog DENSITY [FALLOFF]`.
  - `--post KEY=F`: the grade.
  - `--ground-param`, `--blade-param` (`heath:NAME=` for MI_Heath), `--plant-param` and
    `--prop-param SLUG:NAME=`: material parameters, `NAME=F` or `NAME=R,G,B`.
- Bake the values that win into `oc_climb.py` (`LOOK_TINTS`, `LOOK_GREYS`, `PLANT_LOOK`,
  `BLADE_LOOK`, `PROP_LOOK`) or `biomes.PRESETS["world"]`. Every look re-applies the baked
  values first, so a trial value lasts one look.
- **Post keys:**
  - The grade: ev, exposure, contrast, saturation, vignette, sky, sun, temp, sky_warm.
  - Fog and sky: fog_start, fog_inscatter, volumetric, aerial, clouds, cloud_bottom,
    cloud_wind.
  - Light: shafts, contact, bloom, flare, sun_pitch, sun_yaw.
  - Post keys persist in the level, so put a test value back explicitly.
- **The daylight baseline:** `ev=-2.6 exposure=0.5 sky=1.5 temp=6500 sky_warm=0 sun_pitch=-50
  sun_yaw=43.7 contrast=1.1 saturation=1.05 vignette=0.3 fog_start=150 volumetric=0
  contact=0.05 clouds=-0.2 cloud_wind=0`, with fog `0.004 0.02`.
- **Keep exposure locked** (`ev`). Auto exposure undoes every albedo change.
- **In full sun, bright albedo bleaches to white.** Keep blade tips and ground tints dark.
  Sand and snow may need their own exposure.
- **False colour finds a layer.** `--tint Grass=1,0,0 --tint Sand=0,0,1` on one shot shows
  which paint layer a surface is. A bad colour is often the wrong layer, not the wrong tint.

## Stills

- Stills wait at least 240 frames, then until the grass instance count holds steady (the log
  says `OC settled <shot>`). Missing grass is more often timing than density.
- The first view of a session renders brighter and hazier. For an A/B, use the same shot list
  in the same order.
- `cloud_wind=0` freezes the clouds, so the sky matches between shots.
- The editor reads `OC_STILLS` when it starts: export it before `restart`, or the stills land
  in `out/climb_stills` and the next look overwrites them.
- Review from a contact sheet, then full-resolution crops of what changed. Keep the sheet
  scripts in `out/qa`.

## Grass and plants

- **Paint layers carry grass.** Each layer's `GRASS` rows grow where its weight is.
  - A layer with a "patches" mix (`MIXES`, whose fifth item is the patch noise's low and high)
    can also have a `"<layer> floor"` table, which grows on the litter between the patches.
  - Grass and flowers go on the patches; ferns, shrubs, logs and stones on the litter.
- **Weights are per landscape vertex (7 m).** Painting can't make a sharp edge, so the road
  edge comes from the road fold (`road_dist.png` and `_road_fold` in `oc_climb.py`):
  - "Road clear" is the road's half width plus a reach per kind: blades 0.35 m, plants 0.6 m,
    props 1.5 m.
  - `out/qa/road_dist_check.py` decodes the map across the road at each view.
  - M_Blade's "Road debug", "Road debug sample" and "Road debug uv" (0..1) colour the blades
    by the decoded distance, the raw texel and the UV. Raise `--post ev` about 4 so the colours
    don't clip in the sun.
- **Blades** are `make_grass.py` meshes, imported by `rebuild-ground --rebuild-grass`.
  - M_Blade colours them from their UVs alone. u is random per blade (1.0 for a seed head),
    and v runs from 0 at the root to 1 at the tip.
  - Its parameters: root and tip colour, drift tint, dry share, and wind.
  - Stills can't show motion, so check wind with `--blade-param Wind=40`.
- **Scans** (M_MS_Plant):
  - "Up normal" bends the normal up, so the carpet lights as one surface.
  - "Root shade" and "Root height" darken toward the root.
  - "Flat" and "Flat colour" pull a scan toward the ground's green.
  - Never push "Tint" hard: it goes neon.
  - Keep scans near their own size: scale 2 or more reads as sparse reeds.
- **Check the inputs.** An opacity map that peaks under the clip value makes a plant
  invisible. Never use an opacity scan as a ground surface: check `opacity` in
  `fab_prep/index.json`.
- **Density** multiplies per mesh variant and again by `look --grass`. Instances grow as
  density times cull squared. The laptop editor dies of a GPUVM fault at high counts: that's a
  limit on building here, not on the world.
- **Far canopy:** "Grass canopy start/span/share" on MI_ClimbGround tints the Grass layer's
  ground toward the blades' colour past the cull.

## Importing Fab assets

New downloads land in `/workspace/ue/fab/<biome>/<slug>/` (the asset agent's `MANIFEST.tsv`).
Megascans with FBX meshes go in three steps, chained in one background run:

1. `uv run -q --no-project --with pillow python -I scripts/prep_megascans.py`, offline and
   with no editor needed. It skips assets already in `/workspace/ue/fab_prep` and adds the rest
   to its `index.json`. Assets with no FBX (a "UE only" download) are left out.
2. `scripts/oc.py stop`, then `OC_STAGE=import`, `assign` and `warm` in turn, each
   `bash scripts/ue_run.sh megascans.py LOG` with `OC_ONLY` set to the new `biome/slug` pairs.
   It's a second Unreal, so the oc server's editor must be stopped first. `ue_run.sh` exits 0
   even when the script fails, so grep each log for `Traceback` and `MEGASCANS failed` (an
   asset that wouldn't import; the rest go on) before going on. In a queue step write
   `if grep -q ...; then exit 1; fi`: `set -e` ignores a failing `! grep`.
   The assign stage keeps the existing M_MS_Plant and M_MS_Prop, since `rebuild-ground` adds
   the road fold and moss cap to them. Until 12 Nov it rebuilt them plain, and plants grew on
   the road until the next `rebuild-ground`.
3. Add rows to `GRASS` (`oc_climb.py`) or a preset (`biomes.py`), then
   `rebuild-ground --rebuild-grass` and a `look`.

## Material graph gotchas

- **HLSL writes constants with 8 decimals** (`%0.8f`). A tiny constant like 1/(700·4096) became
  3.5e-7, 0.35% out. Divide by the big number instead of multiplying by its reciprocal.
- **"Local" space is the primitive's, not the instance's,** for instanced meshes (landscape grass
  included): TransformPosition Local→World of (0,0,0) is one point per grass component. Use
  "Instance & Particle" space (`TRANSFORMPOSSOURCE_INSTANCE`) for a per-instance origin.

## Builds and the editor

- `rebuild-ground --rebuild-grass` builds the 1M-triangle prop meshes one at a time (all at
  once crashed Vulkan). Give it `--timeout 2400`.
- `oc.py wait JOB` prints a failed job's traceback. The log loses it after a restart.
- Chains: `set -o pipefail`, and `&&` between every step, including any step that reads the
  stills. Piping a step into `| tail` or joining with `;` has let a chain go on after a failure.
- **The landscape:** 4065² at 7 m builds its grid in minutes. 8129² lost the GPU.
- **Riding:** `DefaultEngine.ini` sets `landscape.RenderNanite=0`. With it on, the first game
  frame grew to 57 GB. `play.sh` widens World Partition's loading range (`OC_RANGE`, in cm),
  or the landscape is cut at 256 m.
- **The sea plane** trips a handled "precision loss" ensure. It's harmless; tiling the sea
  would silence it.
- **Trees** are static Nanite bakes (`oc.py static-trees`). Skeletal PVE trees overflow the
  skinning buffer.
- **PCG:** its classes are exposed to Python (`out/qa/probe_pcg.py`). Placement rules live in
  the numpy generators for now, which are fast and can't crash the editor.

## Performance (for later)

When the time comes: measure on a real gaming GPU (a pod or a packaged build), never in the
laptop editor. Levers that keep the look: TSR at a lower screen percentage, haze, the far canopy
tint instead of far blades, Nanite and occlusion culling, HLODs, and terrain that hides cull
edges.
