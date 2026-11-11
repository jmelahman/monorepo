# OpenCycle: the climb plan

This is the plan of record, written 2026-11-09 after the user's verdict on the island ("4 FPS, no
trees, every biome looks awful"). Read it at the start of every session. Update the "Where things
stand" section before you end one.

## Goal

**One realistic mountain climb that looks good and runs well.** Only after that come other places.

The climb starts in beech woods. It rises through spruce and larch to the treeline, then crosses
dwarf pine and juniper, and finishes in snow at a viewpoint. It is about 8 km at 5–8%.

What the island taught us: we built breadth first (seven routes, six biomes, a town) and none of it
reached a good look or a playable frame rate. From now on we finish one stretch before starting the
next.

## Target hardware (the user's decision)

| Tier | Machine | Target |
| --- | --- | --- |
| Floor | This laptop: Radeon 890M iGPU | **30 fps at Low** (`scalability 0`), 1080p output, upscaled from a lower internal resolution if needed |
| Full | A discrete GPU, RTX 4070 class and up | Epic: Lumen, Nanite, dense foliage, the "immersive" look |

Every milestone is measured on both tiers. A milestone isn't done until the laptop holds 30 fps at
Low on it. The discrete tier is measured on a rented Runpod GPU. Quote the price first, and ask the
user before any spend over $10.

## Quality bar

- **Reference photos.** Collect 10–15 real road-climb photos for each zone before building it (beech
  woods, spruce forest, treeline, snow). Save them in `/workspace/ue/ref/climb/` with their source
  URLs, and use only photos whose licence allows it.
- **QA.** A Sonnet subagent scores each still from 1 to 10 against the matching reference photos, and
  lists the five worst defects. Tell it that sky banding seen through Read is a downscale artifact.
- **Bar to pass a stretch.** Epic scores at least 7 with no "High" defect. Low scores at least 5,
  meaning it is clearly the same place, only simpler.
- **Show the user** before moving on whenever the look changes a lot. Put stills in the storybook
  (https://claude.ai/artifact/UuLeEWcskoabDCuZceuLqb).

## Measurements so far (2026-11-09)

All of these come from the same offscreen ride test (`scripts/ride_test.sh`) on the 890M.

| Level | Settings | fps | Notes |
| --- | --- | --- | --- |
| Scanned island (`/Game/Island`) | Epic, 720p | 3–4 | Megascans grass and props; no trees |
| Japanese town (`/Game/Coast`, 7 Nov) | Epic, 720p | 2.5–5.5 | Free Poly Haven assets |

Neither reaches 30 fps yet.

**A stylized look is out (the user's decision, 9 Nov).** The low-poly valley in `/Game/Stylized`
was only a probe. At Low on this laptop it rode at 70–95 fps (GPU 8–11 ms, `out/stylized_low.log`),
so the engine and game code leave plenty of room. The cost is in what we draw: grass cards,
scanned meshes, Lumen and VSM. Don't build on that level or its look.

## Assets we have

- **Standing trees, today.** Unreal's Procedural Vegetation plugin ships `PVE_Conifer_01` and
  `PVE_Deciduous_Tree_01` (in `/Game/Spike`, used by the 6 Nov forest spike, `out/local_v11`). They
  use Nanite foliage, so they need `r.Nanite.AllowAssemblies=1` and `r.Nanite.Foliage=1`. Never
  rebuild their meshes (that breaks them into cubes or paper cards). They aren't proven on Low yet.
- **Megascans, already downloaded** (`/workspace/ue/fab/MANIFEST.tsv`): grasses, rocks, cliffs, snow,
  ground surfaces, trunks, logs, stumps, dwarf birch. No canopy trees.
- **Fab trees, on request.** `/workspace/ue/fab/CLIMB_BRIEF.md` is the brief for a separate browser
  session that fetches spruce, larch, beech, pine, alder, birch and treeline shrubs. Prefer ones with
  LODs and billboards. They land in `fab/climb/` with a `SUMMARY.md`.

## Milestones

1. **Hero stretch, 1 km, Epic.** The beech-to-spruce transition at 6–7%:
   - a real road (asphalt, edge lines, gravel shoulder, guard rail on the drop side),
   - a cut slope above and a drop below,
   - trees from PVE now, swapped for Fab trees when they arrive,
   - grass and rocks only where the camera sees them.

   Build it as its own level (`/Game/Climb/L_Hero`). Iterate on stills until it passes the bar.
2. **Hero stretch on Low at 30 fps.** Use tree LODs and impostors, short cull distances for grass,
   no Lumen on Low (sky light plus a baked or distance-field AO fallback), virtual shadow maps off,
   and TSR from a lower internal resolution. Profile with `stat unit` / `stat gpu` / the CSV
   profiler. Add a Low-specific foliage density cvar if needed.
3. **The whole climb.** Repeat the hero recipe for the beech woods, the treeline and the snow summit,
   one zone at a time, each with its own references and QA. Then the full 8 km route, with its ride
   track, weather and start-screen entry.
4. **Packaged build.** A shipping build with cooked textures (the "Preparing Textures" stall goes
   away). Measure both tiers.
5. **Only then:** other places, reusing the recipe (coast road, valley farmland).

## Blockers and risks

- **Nanite foliage on the iGPU.** We don't know its cost at Low. Measure it in milestone 2 before
  relying on it; Fab trees with classic LODs are the fallback.
- **Memory.** An external killer acts at about 3 GB free. Run only one Unreal at a time. The 1 m
  island level didn't fit.
- **The editor sometimes hangs after logging its exit.** `ue_run.sh` and `ride_test.sh` handle it;
  never kill an Unreal yourself.

## Where things stand

- 2026-11-10, night (user asleep, "continue as much as possible"):
  - v15: the bark fix worked (red-brown trunks); QA 7, 6, 7, 6, 5, 6, 5.
  - False colour found the cold blue-grey floor was the Forest layer's neutral litter under the
    blue sky, and the pale banks and the far hillside's leopard spots were "Grass mix"
    (lawn_grass), which had no tint at all. Both now tinted (LOOK_TINTS).
  - New `look --post temp=K` (white balance; higher is warmer) and `sky_warm=0..1` (sky light
    colour). Positional `unreal.Color` is BGRA: use keywords. 6700 and 0.35 are baked into the
    look args.
  - v16: warm floor, clover and dandelions on Forest (still about 31 per 100 m^2), tundra
    boulders, subtler road repair patches. Boulders came out near white: `PROP_LOOK` now tints
    props' MIs on each ground build.
  - v17: an alpine backdrop in make_climb.py (TREELINE 420 m, SNOWLINE 600 m, two Gaussian PEAKS
    at x = ±990 m) and triplanar ground on slopes. The first try's wide peak tails lifted the
    path by 100 m and dropped its props; the chain now greps the generator's line for 'bench'.
    QA 7, 7, 8, 6, 6, 6, 6: the peak ends the views but reads as a flat pale cone, and the banks
    still look smeared (soil_mud is a smooth mud scan; the heightfield is 1 m).
  - The boulder's tint was set (probe) and its albedo is dark (D mean 0.34 sRGB, roughness 0.9),
    yet it reads pale grey in sun: it needs moss, not only tint.
  - v18 (running, `--rebuild-grass`): M_MS_Prop rebuilt with a moss cap (`_prop_master`, "Moss
    amount" set by PROP_LOOK); Forest grass doubled near the path at a 20-30 m cull (fewer
    instances overall than v17); yellower moss and lawn tints.
  - v18 looked almost unchanged; one boulder grew a (too lime) moss cap. Probes found why the
    floor never became a carpet:
    - GRASS densities are per mesh variant. Ferns (1.5-2 m, 6-9 variants) covered about twice
      the floor that all the grass (20-60 cm clumps) did. Forest draws about 250 per 100 m^2,
      not 31.
    - The grass scans are near black as scanned (grass_clumps 0.14 sRGB), and the moss scan is
      dull olive, so dark tufts sat on a paler grey-teal felt: the style shots' inverse.
  - v19 (running): grass drawn 1.4-2.4x with a per-plant tint (PLANT_SLUG_LOOK), fewer ferns,
    uncut_grass patches on the forest floor, stony_soil banks, darker moss, peaks shedding snow
    on steep faces. The chain also probes the sky into out/qa/sky_probe.txt.
  - v19 done: QA 7, 7, 7, 6, 6, 6, 6.5. The floor is a green carpet at last. Storybook v19
    (v18 folded in: a look test overwrote its stills). Defects: banks still streaked, the new
    grass patches lime with hard edges, pointed grey rocks (the nordic scans, untinted) at the
    bench and 95%, a smooth pink peak, flat dark spruce.
  - New `look --post clouds=F cloud_density=F sky_lum=F` (MI_ClimbClouds on the
    VolumetricCloud). clouds=0.6 with sky_lum=0.7 turned the sky overcast white and washed the
    scene out: not used.
  - `look` now applies PROP_LOOK, so a prop's tint needs only a restart and a look.
  - The cloud material's default coverage is -0.2 (density 0.008), so clouds=0.6 was a full
    overcast. Pass `--post clouds=-0.2` to restore the default.
  - **v20: the smeared banks' cause.** A second false colour showed the streaks were "Sand mix",
    and the Forest base and "Dirt mix" used the same scan: alpine_needles, an opacity scan whose
    D is 8% needles and 92% dilated padding. All three are forest_floor now, and the streaks are
    gone. The banks then read smooth and mauve, so the soil and steep shade are warmer, and the
    banks get more roots, stones, bigger tufts and clover (v20 chain).
  - Still open: a pale grey low-detail boulder by the bench and at 95% ignores every prop's
    tint (tundra, mossy_rocks, the nordic rocks). It appeared with v16's tundra boulders; maybe
    an HLOD proxy. New `look --prop-param SLUG:NAME=...` for such tests.
  - v20 done: QA 7, 7, 7.5, 6, 6, 7, 6.5; storybook v20. Defects: banks maroon and smooth, the
    upper forest a stop dark, sparse low cover, yellow tufts, uniform grey tarmac.
  - fps at Low (v20, all trees, 300 frames): road_08 22.1, road_50 20.5, road_72 20.8, bench
    23.3 (p95 50-59 ms). Up from 16-19 on 9 Nov despite more grass.
  - **The mystery boulder solved (v21).** `SM_nordic_forest_ground_patch_root`, `SM_fallen_log`
    and `SM_mossy_wood_stump` had an empty material slot (out/qa/probe_nomat.py), so they drew in
    the engine's default grey and ignored every MI tint. `_grass_type` now fills empty slots with
    the scan's own MI on each ground build. v20's denser bank roots had spread them everywhere.
  - v21 look tests: exposure 0.8 barely lifted the spruce; `--post sky=1.8` (skylight) did, and
    is now in the look args. The banks' maroon came from STEEP (0.8, 0.6, 0.38) on top of the
    red soil tint: STEEP is (0.7, 0.6, 0.48), Sand (0.3, 0.22, 0.12). Grass tufts less yellow
    (grass_clumps tint 2.4, 4.0, 1.2). Bank grass 28/18, fewer dry tussocks, wider tarmac
    variation. `look --prop-param` now reaches plants too.
  - v21 done: QA 7.5, 7.5, 8, 6.5, 6.5, 5.5, 7; storybook v21. hairpin_above regressed (haze
    brighter with sky 1.8). The banks stayed bare: false colour showed the bank is Sand (0.94
    weight 2-4 m from the centre line, Forest above), GT_Sand holds its 37 varieties, and 3x
    density added nothing on the bank while it filled the flat floor. Suspect: align_to_surface
    tilted tufts sideways out of the 50 degree cuts. v22 grows plants upright (props still align).
  - v22: upright plants helped a little (more standing tufts at 72% and 95%), not a carpet.
    Landscape grass is placed per metre of map, not of slope: a 3 m cut at 50 degrees gets the
    grass of about 2 m of flat ground. The root patches, now with their material, read as flat
    pink slabs on the banks. Fog 0.003 vs 0.004 made no visible difference to the hairpin haze.
    One road_30 still came out bare once (grass still streaming in); a retake was fine.
  - v23: bank grass 60/40 at 1.4-2.4x, root patches 1.5 (was 4), warmer Sand and Dirt tints,
    warmer tarmac (ROAD_WARM 1.15, 1.0, 0.72). QA unchanged (7.5, 7.5, 8, 6.5, 6.5, 5.5, 7).
  - **The bare banks were a timing bug.** A fixed 480-frame wait shot views before their grass
    was built. Stills now `_settle`: at least 240 frames, then until the count of
    GrassInstancedStaticMeshComponent instances on the landscape proxies holds for 4 checks
    (views needed 360-780 frames). Not found via the HISM class. Looks also re-apply the baked
    tints and plant/prop params first, so false colour no longer leaks into later looks.
  - New `oc.py fps --try LABEL=CVAR V;CVAR V` measures each try alone at every view.
  - v24: "Forest mix" (the mint floor patches) tint (0.2, 0.3, 0.06), grey 0; Sand (0.3, 0.24,
    0.1). QA 7.5, 7.5, 8, 6.5, 6.5, 5.5, 7: banks clearly better (brown, grassed), floor no
    longer mint. Storybook v24 (img18). Top open defects: hairpin_above smeared and hazy; flat
    grey tarmac (no wear, edges, litter); grass reads as tufts not a carpet, no flowers; dark,
    flat spruce interior; banks smooth with few boulders or roots.
  - Perf ablation (`fps --quality 0 --try`, v24, all trees): base 21.5, 20.0, 20.7, 23.3
    (road_08, road_50, road_72, bench). Grass off +1-2.3; Lumen GI off +0.5-1.6; reflections off
    +0.6-1.9; screen percentage 67 only +0.3-2.2, so the frame is not pixel-bound. VSM off
    (shadow maps) halves it (10.9-14.2): keep VSM. With trees culled it was ~29, so the cost is
    the trees' draw/CPU side, not shading. Next: measure in `-game` (no editor overhead), then
    tree instance count, HLOD/imposters for far trees, and Nanite on the static trees.
  - v25 batch (running): road centre worn darker and smoother, less pink (ROAD_WARM 1.05, 1,
    0.76), darker edge soil, more litter mid-road; a kikuyu carpet on Forest (16, 15-20 m cull),
    more dandelions, twice the boulders, more mossy rocks on banks. New look keys for the
    hairpin haze: `aerial`, `cloud_bottom`, `fog_inscatter`.
  - v25 first stills: road less pink, but the worn centre at roughness 0.55 drew a wet,
    sky-bright stripe (now 0.82, a "worn rough" param). The kikuyu carpet barely shows.
  - Hairpin haze, still open. Each tested on hairpin_above and changing nothing visible: fog
    0.0008-0.004 and falloff 0.02-0.08; aerial perspective 0.3 (the property is spelled
    `aerial_pespective_view_distance_scale` in the engine); directional fog inscatter 0; cloud
    layer bottom 8 km; bloom 0; the 64 World Partition HLOD actors hidden. It needs the sun
    (sun 0 is black under the locked EV). Suspects left: back-lit foliage translucency and the
    pale Rock layer on the cliff toward the sun; next, a camera turned away from the sun.
  - v25 (worn rough 0.82): QA 7.5, 7.5, 8, 7, 7, 5.5, 7 (+1 in all); boulders helped the spruce.
    The kikuyu read as a pale grey-green wash. Storybook v25 (img19).
  - The haze is partly the low sun: it sits 16 degrees up and about 52 degrees right of the
    hairpin camera. `--post sun_pitch=-35` thinned the haze and showed the snowy peak, but lit
    the spruce flat and bright (moody look lost). Kept -16.3 (yaw 43.7); a user decision.
    New keys: `sun_pitch`, `sun_yaw`, `bloom`, `flare`. Post keys persist in the level, so put
    a changed one back explicitly (sun_pitch=-16.3 sun_yaw=43.7, bloom=0.675, aerial=1).
  - v26: kikuyu tinted like wild_grass, 28 at 1.4-2.2x on Forest, 24 on Sand banks. QA 7.5,
    7.5, 8, 7, 7, 5.5, 7.5: a green carpet at last, but lime and self-lit under the spruce, with
    bare soil between clumps and one grass height. Storybook v26 (img20). The grass rebuild held
    (no GPUVM fault) at these densities.
  - The tint look shot road_72 bare: `_settle` saw road_50's grass count (1199 vs 1227) hold
    steady before road_72's grass began. Steady now counts only once the count has moved.
  - v27: grass tints (1.5, 2.8, 1.1) / (1.0, 1.8, 0.75), TranslucencyTint halved, wider sizes,
    twice the bank clover. QA 7.5, 7.5, 7.5, 7, 7, 5.5, 7.5: deeper green, less glow, not
    olive. Open: bare soil between bank tufts; the hazy view from above. Storybook v27 (img21).
  - v28: darker Sand/Dirt tints so the gaps read as shadow (tried by look), back-light lower
    again, and a Dirt GRASS row on the bare verge. The Dirt row grew through the whole tarmac:
    the landscape under the road mesh is painted Dirt too. Never give Dirt grass. Reverted.
  - v29: the shoulder narrowed in make_climb (0.15-0.6 m past the tarmac, was 0.2-1.0), so
    the bank and floor grass reach the path. QA 7.5, 7.5, 7, 7, 7, 5.5, 7.5 (it wobbles about
    0.5 on unchanged frames; trust the stills). The band is gone. Storybook v29 (img22).
  - **v30, the vegetation reset** (the user: "disappointed with the grass and ferns", worse than
    Skyrim's). v12-v29 tuned tints and densities; the faults were structural. See "Vegetation:
    the approach" in CLAUDE.md for the rules. What changed:
    - `GRASS["Forest"]` grows only on the Forest layer's grass patches and `GRASS["Forest
      floor"]` (ferns, shrubs, logs, stones) on the litter between (`Ground.patch`; the MIXES
      row's fifth item is the patch noise's low and high).
    - M_MS_Plant: world-space normal bent up ("Up normal" 0.85), root darkening ("Root shade"
      0.5 over "Root height" 14 cm), and "Flat" 0.7 on the three grasses toward "Flat colour"
      (0.1, 0.2, 0.03) with "Flat gain" 12. The first render at gain 4 was grey and dull.
    - "Forest mix" tint (0.2, 0.34, 0.05); Forest grass densities x1.67; scales cut to 0.9-2.0.
    - Result, from full-size crops against v29 (no QA run): the neon, the sparkle and the
      patchy yellow-green are gone, the colour is even, and yellow dandelions show. Open:
      1. Cover is thinner than v29 up close (road_30's banks, the bench): soil shows between
         blades, and the blades are paler than the ground under them. Darken "Flat colour"
         toward the ground or lift the "Forest mix" tint (look), then add a short
         close-culled row.
      2. Ferns show their fronds but still read near black.
      3. The litter patches read grey-brown mud at the bench; their share may be too high.
      4. Sand (banks) and Grass (meadow) still use one even table each.
    - Storybook v30 (img23).
  - Perf in `-game`: PythonScriptPlugin is "UncookedOnly", so it should load in `UnrealEditor
    -game` (uncooked). A game-mode fps script could find the PlayerController with
    `unreal.ObjectIterator`, BugItGo to each view, and time a slate post-tick callback. Not built.
- 2026-11-10, midday: **v14, exposure locked at EV -4.5.** The scene sits near EV -5 (1.5 to 2.5
  rendered black). With darker soil (LOOK_TINTS), green grass and moss, a dark fence and road, QA
  scored 7, 6, 6, 5, 4, 6, 4 (v13: 6, 5, 6, 6, 6, 6, 5): better in the beech, a stop too dark in
  the spruce. Storybook v14.
  - Bugs found: `_tree_look` set the bark MIs on a struct copy, so the trunks stayed grey (now
    `sm.set_material`). The road's leaf blobs read as pale ovals on the dark tarmac (now darker
    and fewer).
  - v15 chain running: those two fixes plus `--post exposure=0.5`.
  - QA's other defects: no grass carpet or flowers; a blue-grey cast at the bench; lime verge
    patches; a leopard-spot blotch on the far hillside; a blurry, stretched cut bank; milky fog.
  - Next batch (`--rebuild-grass`): bigger, denser grass clumps with flowers within the budget,
    greener ground, big boulders; then sky and the snowy peak.
- 2026-11-10, late morning: **a new look reference and the exposure finding.**
  - The user added two modded Skyrim screenshots as the target look, "vibrant yet realistic".
    They are copied to `ref/climb/style_*.jpg` and listed in SOURCES.tsv. Against them our stills
    are washed out: pale beige floors, pale grey trunks, no carpet of grass, flat light.
  - Auto exposure undid albedo changes: soil tinted to half its albedo looked almost unchanged.
    The sun is 6 lux and auto exposure ranged from -10 to 20 EV. New `look --post ev=N` locks it
    (-4.5 is right).
  - Next: lock exposure darker with more contrast and saturation; a green ground under denser,
    larger grass clumps with flowers; a green, mossy forest floor in place of beige litter;
    red-brown upper trunks on the pines; scattered big boulders; and a moodier sky.
  - Queued in code (not built): road darken 0.3 and warmer; the edge soil narrowed and browned
    (the road flecks); timber without pale lichen or silver speckle, specular 0.3.

- 2026-11-10, morning (batch v13): one rebuild with grass for QA's v12 list.
  - First try: ferns raised to 9/8/6 per 100 m^2 (about 40 plants on Forest in all). The first look
    after a restart hit the radv GPUVM fault (DEVICE_LOST). The user also found too much fern and
    too little grass, and the spruce photos agree: grassy verges, dry tussocks, few ferns.
  - Rebalanced (building): Forest ferns 3/2/1.5, no leather_fern, grass_clumps 8, wild_grass 6,
    dry_grass 4 (about 30 in all, as v12); banks' ferns 2/1 with grass 12/8 and dry grass 4. A broken
    grass verge in `make_climb.paint`: 1-2 m wide, starting 0.6 m past the tarmac.
  - The peak (user question): above the treeline the climb is alpine meadow, then dwarf pine, then
    rock, scree and snow patches at the top, with only tufts and cushion plants there.
  - Kept from the first try:
  - Ferns matte: M_MS_Plant gets "Specular" (0.15, was a fixed 0.4) and "Rough floor" (0.6), both
    in `PLANT_LOOK`; leather_fern, the glossiest, cut to 1 per 100 m^2.
  - Soil: `LOOK_TINTS` Sand (0.42, 0.34, 0.28) and Dirt (0.4, 0.32, 0.26) on the soil_mud bases.
  - Cover: Forest ferns 9/8/6, bearberry 6, dry grass 3; grass scale 0.6–1.4 (was 0.85–1.3).
  - Road: darken 0.4, the scan's colour clamped at 0.3 (the speckle), roughness at least 0.7,
    specular 0.3.

- 2026-11-10, early (storybook v12): **colour fixes; QA level.**
  - Litter tinted `LITTER` (0.38, 0.3, 0.23) on the Forest base and the Sand/Dirt mixes; plants
    `PLANT_LOOK` (Tint 0.6, 0.62, 0.42, TranslucencyTint 0.2, 0.24, 0.12, Grey 0.12).
  - Sonnet QA: road_08 5, road_30 6, road_50 6, road_72 5, road_95 6, hairpin_above 5, bench 5
    (v11: 6, 6, 6, 5, 5, 5, 6). The fixes worked, but QA now ranks other defects first.
  - Open, in order:
    1. Blue-white, metallic fern planes in every ground still: the plant specular (0.4 in
       M_MS_Plant) against the sky, or one variety. Try a "Specular" param near 0.1 by look.
    2. Road too pale: lilac-grey with a bright, regular speckle. Darker (about 0.15 albedo), warmer,
       with low-frequency patches and leaf fall; the exposure may also be lifting it.
    3. Soil still uniform and blurry up close: detail normal, triple the tiling.
    4. Ferns in rows on bare soil: denser cover, scale 0.6–1.4, sticks and leaves.
    5. hairpin_above: a smeared pastel hillside and milky fog.
    6. The fence reads speckled and plasticky.

- 2026-11-09, late night (batch v12): **colour and surface, in one rebuild.**
  - `_plant_master` rebuilds M_MS_Plant in place in every `rebuild-ground`: "Grey" 0.25, a
    per-instance brightness "Variation" 0.5, and one plant in five ("Dry share") turned to a
    "Dry tint" of yellow-brown. Rerunning megascans.py's assign stage puts back the plain master.
  - Ground: roughness at least 0.75 (`ROUGH_FLOOR`) and specular 0.3, against the glitter; steep
    Forest/Sand/Dirt shaded toward (0.7, 0.6, 0.5) (`STEEP`); the Forest mix is now
    `mossy_forest_floor` in 10 m patches (grey 0.3).
  - Forest cover adds alpine_bearberry 3.0, dry_grass 1.5 and dead_shrubs 0.3.
  - Road: darken 0.5 with a warm (1.08, 1.0, 0.86); the soil edge frays 20–60 cm in.
  - Fog 0.004.
  - Result: the stills barely moved. A `look --plant-param` pass found the ferns' glow was the
    scans' translucency tint (now 0.2, 0.24, 0.12 in `PLANT_LOOK`). A false-colour pass found
    the pale banks were Rock: the outcrop noise on lumpy banks took whole cuts. Outcrops are now
    rare (fbm 0.86–0.9, slope > 0.7) and 3–6 m off the path, and Rock is darker.

- 2026-11-09, night (storybook v11): **ground cover renders.** The batch below built cleanly,
  then a look pass set plants to 0.62,0.62,0.5 and grass to ×1.2.
  - Sonnet QA: road_08 6, road_30 6, road_50 6, road_72 5, road_95 5, hairpin_above 5, bench 6
    (up from 5, 5, 5, 4, 5, 6, 5).
  - Open, in order (the next batch):
    1. Ferns too saturated and uniform: darker olive, per-instance colour variation, some dead
       or brown fronds, varied scale.
    2. Bank soil pale beige with a glittery specular: roughness about 0.9, browner, a finer
       detail normal, stones and roots. The visible banks are the Forest layer.
    3. Tarmac blue-grey with hard clean edges: warmer and darker, worn gravelly edges, litter.
    4. Cover between ferns too sparse: moss, grass and litter under them.
    5. hairpin_above: the far slope is smeared and the haze milky. Cover the far slope, thin
       the fog.
    6. `fern`'s UVs only partly match its atlas; trunks are clean grey cylinders.

- 2026-11-09, night (batch): **one rebuild for ground, ferns and banks.**
  - Diagnosed first. A 1080p still with a 3 GB streaming pool made the trunks crisp but not the
    ground, so the blur comes from the material. The textures are 2K in the World group.
  - The green/brown blotches came from the olive `forest_floor` mixed into the bank soil and the
    path edge in 3 m patches. Both now mix brown `alpine_needles`, and the Forest mix grey is 0.55.
  - Steep ground takes its normal from the side projection, as its colour already did. It used
    to take half a flattened, stretched top normal.
  - Fern slivers came from the cut-out in D's alpha averaging away in the mips, under the 0.33
    clip. `_keep_alpha_coverage` turns on scale-mips-for-alpha-coverage for every grass scan.
  - **The big find:** no grass or fern had ever rendered except `fern`. The source opacity JPEGs
    keep the mask in one colour channel, so the greyscale conversion in prep peaked at 76 of 255,
    under the 0.33 clip. Prep now takes the strongest channel and stretches it to full range.
    `oc.py reimport-plants` swaps in the fixed D textures. `fern`'s own UVs match its atlas only
    in part (19% opaque coverage), so it may still look thin.
  - The tints barely changed the banks: the cuts are about 1 m wide, so the "Sand" layer covers
    0.09% of the map, and the visible banks are the Forest layer.
  - Ferns are about 1.6× denser (Forest 6/6/4/2, Sand 8/6). The banks have slumps and rills
    (`roughen_banks`).

- 2026-11-09, late (storybook v10): **lower cuts, cleaner path.**
  - `open_valley_side` now caps the uphill side too: a cut of 1–2 m (BANK_SLOPE 1.3), then a
    gentle 0.28 slope that rejoins the hillside by 35 m. Uncapped, cuts across the steep flank
    stood 5–8 m high.
  - The bank soil is its own landscape layer, which reuses the unused "Sand" slot
    (`oc_climb.SURFACES/MIXES/GRASS["Sand"]`). Its roots, stones and ferns stay off the tarmac,
    while the path edge stays "Dirt" with no grass. There is no granite within 25 m of the path.
  - Trees grow on slopes up to 1.45 and come in from 3.3 m off the path.
  - The road's dark ellipses were the cracked-tarmac mask (8 m, hard). That mask is now 2.5 m,
    soft and at most 55%. Repairs are 1.6 m world-grid blocks.
  - One `build-level` dress stage hit a GPU reset (radv GPUVM fault, DEVICE_LOST) right after the
    new bank grass was added. A retry passed. If it recurs, lower the `GRASS["Sand"]` densities.
  - Sonnet QA: road_08 5, road_30 5, road_50 5, road_72 4, road_95 5, hairpin_above 6, bench 5.
  - Open, in order:
    1. Ground texture smeared and low-detail on slopes: add a fine detail albedo/normal, and
       make the colour blotches smaller.
    2. Ferns and tufts render as tiny dark specks: check their scale, lighting and density.
    3. Banks are still smooth: add 0.5–3 m relief and gullies.
    4. Trunks are clean grey cylinders.
    5. Then the full climb.

- 2026-11-09, night (storybook v9): **wear and life.**
  - A weathered fence: leaning round posts, missing and broken rails, moss and damp.
  - A worn path: cracked and patched tarmac, with soil and leaves at the edges.
  - A bench at the hairpin and log stacks by the path. A needle-litter forest floor and soil cut
    banks.
  - Trees fade in from the path. Trunks stand at the lowest ground under them and posts go
    deeper. The sun has contact shadows (`look --post contact=0.05`).
  - `make_climb.open_valley_side` stops the path running in a trench across spurs.
  - Open:
    - Bare, smooth cut banks: they need grass, roots, ferns, stones and finer shape.
    - The hairpin view from above is hazy.
    - QA re-score not yet done.
    - Then the full climb.

- 2026-11-09, small hours: **47k static Nanite trees with leaves across the whole map**
  (`make_climb.py` FAR 0.45 beyond the road band, `climb_world.TREE_CULL` 2 km). `oc.py
  static-trees` bakes `SM_spruce` / `SM_beech` from the PVE assemblies; each twig and leaf part is
  simplified to 2500 tris so the Nanite build fits in RAM. New: `oc.py fps` and `oc.py probe`.
  Perf at Low is about 17 fps with all trees (29 with none); shadows off, a 300 m cull or no wind
  each gain little. The user said **visuals first, perf later**. Storybook version 8.
  Sonnet QA scores: road_08 5, road_30 6, road_50 6, road_72 4, road_95 5, hairpin_above 7.
  Its top defects: pale plaster-like rock cuts, neon/yellow verge smear, empty forest floor,
  tree rows, flat washed-out light, a too-clean road edge.

- 2026-11-09, late night: the road is now a **3 m bike-only path** (no markings) with a timber
  fence. `oc.py build-level` rebuilds L_Hero in 5 minutes with no crash. `look` gained `--grass
  DENSITY CULL` (2.0 1.5 now) and `--fog DENSITY FALLOFF` (0.008 0.02 now; the template's falloff
  of 0.2 hid all fog above 100 m). A rim of ridges round the map hides its edge and the sea.
  Storybook version 7 has the stills. Open defects: smeared grass banks, pale tarmac and fence,
  the near rim reads as a bare cliff, trees only within 320 m of the path (static trees next).

- 2026-11-09, night: **the loop is `scripts/oc.py`** (run from /workspace/ue). It's one command
  that blocks until the job is done, failed or crashed, and says which on its last line.
  - Commands: `oc.py look --tint Dirt=r,g,b --grey Rock=0.75 [--shots road_72]`, then `stills`,
    `rebuild-ground`, `status`, `wait JOB`.
  - A look change with one still takes 16 s.
  - It starts its own supervised editor (`oc_supervisor.sh` runs `oc_server.py`).
  - `oc_climb.py` holds the climb's material and ops, with no island_world monkeypatching.
  - The `ue_serve` and `out/cmd` entry below is superseded.
  - Look set so far: the Dirt shoulders are tinted 0.62,0.63,0.65 with grey 0.85, Rock has
    grey 0.75, and Rock mix has grey 0.6.

- 2026-11-09, evening: the crash rules are enforced in code, not kept in memory.
  - Start the server with `nohup bash scripts/ue_serve.sh >out/serve_supervisor.log 2>&1 &`
    from /workspace/ue. When the editor dies, the supervisor marks the running command
    `.crashed` (log tail in `.crashed.txt`) and restarts on the empty map. It gives up after
    three exits in ten minutes. Each run's log is kept in `out/serve_logs/`.
  - The landscape uses `MI_ClimbGround`. Recolour it with `climb_world.tune({"Dirt": (r, g,
    b)})`: parameters only, no compile, safe with the level open.
  - `ground_material()` raises if a landscape is loaded. To rebuild, send `quit.py` (which also
    stops the supervisor), start `ue_serve.sh` again, then queue the rebuild.
  - The server holds commands while under 6 GB of memory is free.

- 2026-11-09, later: L_Hero is built (landscape, road, rails, 3,600 PVE trees). The loop is one
  warm editor (`scripts/ue_serve.py`) that runs command files dropped into `out/cmd`: a look
  change plus six stills takes about 2.5 minutes. Stills now come from the editor viewport, so
  grass and tree LODs load. The rock is now granite and the shoulders gravel.
  - **Crash rule:** never rebuild the ground material while the landscape is loaded. The editor
    recompiles every landscape piece in one frame, and the Radeon driver resets the GPU
    (DEVICE_LOST, twice). Build the material first, then open the level (`out/cmd/0005_look.py`).
  - Never reload L_Hero while it's open (segfault). `NO_SKELETAL` / `OC_NO_SKELETAL` leaves
    the skeletal trees unloaded.
  - The skeletal trees overflow the GPU skinning buffer (`r.Skinning.Buffers.*`). The PVE
    plugin can export static meshes (`EPVExportMeshType::StaticMesh`), but only from its
    editor UI, not from Python.

- 2026-11-09: Plan written. The user ruled out a stylized look; the stylized valley was only a
  frame-rate probe (see Measurements). Fab trees are
  being fetched by a separate session from `fab/CLIMB_BRIEF.md`, with results in
  `fab/climb/SUMMARY.md`. Next: milestone 1 with the PVE trees.
- 2026-11-09, later: milestone 1 started.
  - `make_climb.py` generates the hero stretch into `world_gen/climb`. It is 2 km of ground at 1 m
    and 1.2 km of road at 6% average with one hairpin, with about 20k beech and spruce stems and
    guard-rail runs.
  - `climb_world.py` builds `/Game/Climb/L_Hero` from that output. It isn't built yet.
  - Nine reference photos are in `/workspace/ue/ref/climb/`, with sources and licences in
    `SOURCES.tsv`.
  - The first Fab trees have landed in `fab/climb/`. They are single-LOD FBX files, with no spruce
    or beech yet.
  - Fixed an editor crash when a level has no ride track (it broke the `L_Forest` spike).
- **Only one Unreal at a time, across all sessions.** Two sessions rode at once on 9 Nov, and both
  wrote `/workspace/ue/ride_test.log`. Check `pgrep -a UnrealEditor` before starting one.
- 2026-11-10, evening (the latest entry):
  - **v31:** three times the scan grass, drawn taller and further (grass rows only: `look
    --grass` multiplies props too). A full carpet with no soil showing, but still short cards.
  - **The user's new direction (10 Nov):** two more references (the Folkvangr tundra grass and
    `ref/climb/alpine_meadow_trail.jpg`), high-end visuals first, and "think big ... everything
    is still on the table". Agreed plan: daylight, grass made of real blades, then placement by
    ecology with PCG.
  - **v32: daylight and blade grass.** Sun pitch -50 with ev=-2.6, sky=1.5, temp=6500,
    sky_warm=0. `make_grass.py` writes nine blade clumps; the ground build imports them as
    Nanite meshes with M_Blade (`BLADE_LOOK`, `look --blade-param`). The first chain shot the
    old grass: the glTF import puts meshes in `<slug>/<file>/StaticMeshes`, the grass types
    found none, rebuild-ground failed and `| tail -1` hid it (chains now use `set -o
    pipefail`). The first real blades bleached white in full sun and their seed heads glowed;
    darker tips, less translucency and roughness 0.85 fixed both. glTF v survived the import
    unflipped (roots dark, tips light). Storybook v32 (img24).
  - **v33:** blades half as wide and nearly twice as many per clump (1,000 to 1,300 tris), more
    brightness spread and dry tips, and nearly twice the blades on the cut banks ("Sand"). The
    grass reads as grass up close and the banks are covered. A restart after the mesh rebuild
    took over 60 s and timed out the chain; the stills step then ran on v32's files, so gate
    any post-processing with `&&`, never `;`. Storybook v33 (img25).
  - **v34:** re-matched to daylight and added wind. BLADE_LOOK drifts toward a duller, cooler
    green with more dry tips; PLANT_LOOK's Tint and TranslucencyTint are up so ferns aren't
    black; ROAD_DARKEN 0.19. M_Blade has a world position offset ("Wind" in cm at the tip,
    "Wind speed"); `look --blade-param Wind=40` shows the bend in a still. The bench's road
    is still pale: false colour showed it is the road mesh, not a landscape layer, seen
    against the sun, so it is sheen (try a lower specular and a rougher floor in
    `road_material`). Storybook v34 (img26).
  - Next: see "What to do next" in CLAUDE.md.
