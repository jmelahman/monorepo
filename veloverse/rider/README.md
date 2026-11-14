# Player rider: skinned cyclist on the hoods

`build_rider.py` builds the rider, the gear and one looping pedal action headless. It works in
Blender from the MPFB 2 base human. The tunable values are constants at the top of the script.
The bike is loaded only as reference geometry: `../bike/bike.blend` if it exists. Otherwise
`../bike/build_bike.py` builds it into `out/bike_ref/`. The bike is never exported or modified.

```sh
blender -b -P build_rider.py                        # full build + renders (~5.5 min, 24-core CPU)
blender -b -P build_rider.py -- --no-render         # mesh, FBX, textures and stats only
blender -b -P build_rider.py -- --quick             # 1K textures, low-sample half-size renders
blender -b -P build_rider.py -- --out /some/dir     # write outputs elsewhere (default: ./out)
blender -b -P build_rider.py -- --bike-blend PATH   # use another bike .blend as the reference
```

`--stage probe` and `--stage fit` stop early and save `probe.blend` or `fit.blend`, for
debugging. The script exits non-zero on any error.

Outputs go to `out/`. They are regenerated and not committed:

- `rider.fbx`: the armature, the one mesh and the `Pedal` action. No bike.
- `rider.blend`: the rider plus the bike for reference. The IK constraints are already removed.
- `T_*.png`: textures, 2048² (1024² with `--quick`).
- `render_*.png`: renders at 1600 px wide, with the bike.
- `stats.json`: the checks listed below.

## Prerequisite: MPFB 2

The script needs MPFB 2.0.18, installed as a Blender extension in the `user_default`
repository. Install it from Blender's *Get Extensions* (search "MPFB"), or from the file:

```sh
blender --command extension install-file -r user_default mpfb-2.0.18.zip
```

The file is `mpfb-2.0.18.zip`, with sha256
`c075b3e9b94b8e5556a5f03f4653602d02ea306de08126e8f55ed6dee28a2dad`. The script enables the
extension itself, and nothing else needs downloading.

## Frame and units

The frame is the same as the bike's:

- **Axes:** metres, +X forward, +Z up. The drive side is −Y, and the rider's left is +Y.
- **Origin:** the armature object and the `Root` bone sit at (0, 0, 0), on the ground directly
  under the bottom bracket. Place the rider with the same transform as the bike's `Bike` empty.
- **No root motion:** `Root` is not animated.
- **FBX settings:** the same as the bike's, giving UnitScaleFactor 100 (1 BU is 100 UE cm).
  - Z up and forward Y, with tangent space and face smoothing.
  - Deform bones only, no leaf bones, and the armature node type is Null.
  - It contains the armature and the mesh only.
- **Importing in UE:** turn **Convert Scene** on and use uniform scale 1.
  - Blender +X stays UE +X (forward), and the drive side lands on UE +Y.
- **Transforms:** all are applied, with scale 1 and no negative scales.

The pedal geometry follows the bike:

- The BB centre is at z = 0.269 and the cranks are 172.5 mm.
- The pedal axles sit at y = ±0.125: that's the pedal body centre of `PedalL` and `PedalR`.

## Body

The body is MPFB 2's base human, built with these settings:

- **Macros:** male, 25 years, muscle 0.55, weight 0.12, "ideal" proportions.
- **Build targets:** a slim, narrow climber's silhouette that sits inside the bike's width:
  - narrower shoulders, torso and hips, a smaller seat and less V-taper;
  - slimmer upper arms, and thin legs: less thigh and calf girth and little fat on them;
  - a toned stomach.
- **Height:** scaled uniformly to exactly 1.780 m standing.
- **Fit:** the rider's proportions change to fit the bike. The saddle and bars never move.
  - Leg length uses MPFB's `upperlegs-height` and `lowerlegs-height` targets at +0.152. That puts
    the knee at the bottom of the stroke at 144.6° (target 145°).
  - The hip joint ends up at x −0.215, z 1.064. The thigh is 0.446 m and the calf 0.461 m.
- **Helpers:** MPFB's helper geometry is removed. Only the eyeballs and eyelashes are kept, as
  cheap meshes.
- **Hair and brows:** the hair is short and painted on the scalp in the body texture, under the
  helmet. The brows are thin strips. There's no particle hair.

**Pose on the hoods**, measured at frame 0:

| Measure | Value |
|---|---|
| Back, hip joint to the base of the neck | 42.0° from horizontal |
| Torso pitch | 48.7° |
| Shoulder protraction | 16.6° |
| Elbows | 17° short of straight |
| Sit bones | on the saddle top, sunk 3 mm into the pad |
| Palms | on the hood bodies, 0.513 m forward of the BB, the horns in front of the knuckles |
| Fingers | solved against the hoods (below) |
| Head | its forward axis meets the road 10 m ahead |

**The grip** is fitted to the bike's hoods in two steps, once the proportions are settled.
The hands are fixed in the world, so one solve holds for every frame, and Unreal only plays it.

1. **Place the palm.** The palm point starts on the hood's centreline, 0.538 m forward of the BB
   and 9 mm over its top. It then moves, within 25 mm back, 10 mm forward, 20 mm out and 10 mm
   up, to the smallest move that rests the palm, the knuckles and the fingers' first segments
   1.5 mm off the hood. The horn then sits in front of the knuckles instead of through the
   index finger. The arms follow by IK. On this bike it moves 25 mm back and 10 mm up.
2. **Close the fingers and thumbs.** Each joint, knuckle first, starts at a relaxed curl. If
   it's in the hood there, it opens by up to 15° to where it's clearest. Then it closes until
   its skin would come within 1.5 mm of the hood, lever or bar. Fingers in open air stop at a
   loose curl, not a fist. The skin is checked by skinning the hand in numpy against the
   cockpit mesh.

The result: the index finger lies along the horn, the middle finger drapes over the lever
blade, the ring and little fingers curl under the hood, and the thumb wraps its inside.

## Skeleton

MPFB's `game_engine` rig has **53 bones, all deforming**. The bone axes are MPFB's.

```
Root
└─ pelvis
   ├─ spine_01 ─ spine_02 ─ spine_03
   │                         ├─ neck_01 ─ head
   │                         ├─ clavicle_l ─ upperarm_l ─ lowerarm_l ─ hand_l
   │                         │     └─ {thumb,index,middle,ring,pinky}_{01,02,03}_l
   │                         └─ clavicle_r ─ upperarm_r ─ lowerarm_r ─ hand_r
   │                               └─ {thumb,index,middle,ring,pinky}_{01,02,03}_r
   ├─ thigh_l ─ calf_l ─ foot_l ─ ball_l
   └─ thigh_r ─ calf_r ─ foot_r ─ ball_r
```

In full:

- `Root`, `pelvis`, `spine_01`, `spine_02`, `spine_03`, `neck_01`, `head`
- per side (`_l`, `_r`):
  - `clavicle`, `upperarm`, `lowerarm`, `hand`;
  - `thumb_01`–`03`, `index_01`–`03`, `middle_01`–`03`, `ring_01`–`03`, `pinky_01`–`03`;
  - `thigh`, `calf`, `foot`, `ball`.

Skinning:

- There are at most **4 influences per vertex**, renormalised.
- The helmet, straps, glasses and brows are rigid to `head`.
- The shoes take their weights from the foot underneath them.
- The cleats are rigid to `foot_*`.

## Animation: `Pedal`

There is one action, `Pedal`, the FBX take of the same name:

- **Frames:** 0–59 at 30 fps, one crank revolution, so 60 rpm at 30 fps. Frame 60 would equal
  frame 0, so it isn't included; loop 0–59. The crank angle is linear in time: 6° per frame.
- **Frame 0:** the drive-side (right) crank points forward (+X), and the crank turns so that a
  pedal at the top moves forward. To match the bike, rotate `Cranks` by −6° × frame about Y.
- **Keys:** location and quaternion on every deform bone, on every frame, linear. It was solved
  with Blender IK, baked, and then the constraints were removed.
  - Leg chains: thigh → calf → foot, with the cleat target on the pedal axle.
  - Arm chains: upperarm → lowerarm → hand, with the palm target on the hood.
- **The cleat point** is the cleat's centre projected onto the pedal axle: 12 mm under the cleat
  face (the pedal's stack) and 10 mm behind the ball of the foot. It stays on the axle within
  0.1 mm on every frame, with the pedals level.
- **Ankling:** pitch = 5° − 2° cos φ − 10° sin φ + 1° cos 2φ, with φ the crank angle from
  forward. The heel drops about 10° on the downstroke and the toe dips through the bottom.
- **Knees:** from 70.0° at the top to 144.6° at the bottom of the stroke, the same on both legs.
  They never lock.
- **Upper body:**
  - The hands are fixed in the world.
  - The pelvis rolls ±1.5° and drops 3 mm with each downstroke.
  - The upper torso counter-rolls only 0.25°, just enough to hold the shoulders still. The
    upper arms then swing at most 2.7° in a stroke, and the forearms 2°.
  - The head stays level on the road, 10 m ahead.
- **Loop seam:** every bone's step from frame 59 back to frame 0 is checked against the largest
  step between consecutive frames. It's 18.1 mm and 2.3°, against 19.4 mm and 2.6°, so the
  loop has no pop.

## Materials

There are 7 slots on the one mesh. The textures are 2048², with the normal maps in OpenGL
convention (green up). The roughness is a constant per material. Everything is kept matte and
simple, with flat colours and faint surface detail, so the rider sits in the world rather than
looking like a studio scan.

| Slot | Textures | Roughness | Other | Used on |
|---|---|---|---|---|
| `M_Skin` | `T_Body_BaseColor`, `T_Body_Normal` | 0.70 | subsurface 0.05 | skin, the short hair on the scalp |
| `M_Kit` | `T_Body_BaseColor`, `T_Body_Normal` (same as skin) | 0.85 | | jersey, bib shorts, socks (painted on the body) |
| `M_Helmet` | `T_Helmet_BaseColor`, `T_Helmet_Normal` | 0.55 | | helmet shell |
| `M_Lens` | none: base (0.010, 0.011, 0.014) | 0.15 | metallic 0.3 | glasses lens, eyeballs |
| `M_Shoe` | `T_Shoe_BaseColor`, `T_Shoe_Normal` | 0.60 | | shoe uppers, soles, cleats |
| `M_Strap` | none: base (0.012, 0.012, 0.013) | 0.80 | | helmet straps and buckle, the helmet's inner liner, glasses frame |
| `M_Hair` | none: base (0.030, 0.019, 0.012) | 0.80 | | brows, eyelashes |

The kit is a texture on the body, not cloth. Colours are given in linear RGB:

- **Jersey:** navy (0.010, 0.024, 0.075), with short sleeves.
- **Accents:** orange (0.80, 0.22, 0.05) on the side panels and the sleeve cuffs.
- **Bib shorts:** black, ending just above the knee.
- **Socks:** short and white.
- **No marks:** there are no brands, logos or text anywhere.
- **Normal map:** a faint knit on the kit and barely any pore detail on the skin.

The gear:

- **Helmet:** a vented road shell in navy, with an orange stripe each side and an orange tail
  band. Black rim and straps.
- **Shoes:** white uppers with perforations, a black sole, heel cup and strap, and a cleat.

## Stats (full build)

| | |
|---|---|
| Triangles | **49,632**: Skin 17,040, Kit 5,668, Helmet 6,664, Strap 9,680, Shoe 8,376, Lens 1,804, Hair 400 |
| Bones | 53 |
| Most influences per vertex | 4 |
| Largest cleat-to-axle distance | 0.1 mm |
| Knee angle, min / max | 70.1° / 144.6°, both legs |
| Largest hand-to-hood distance | 0.0 mm (to the palm point) |
| Interpenetration deeper than 5 mm | index knuckles on the brake levers, 9.6 mm (39 vertices); shorts into the saddle pad, 7.5 mm (16 vertices) |
| Eye point, frame 0 | (0.359, −0.004, 1.530) |
| Game's assumed eye point | (0.18, ·, 1.50) |

The eye point is 0.18 m further forward and 0.03 m higher than the game assumes. A 178 cm
rider on the hoods of this bike, with the back at 42°, puts the eyes over the front of the top
tube, not over the BB. The game's camera should take the eye point from `stats.json` (or from
the `head` bone) rather than the constant.

## Assets and licences

| Asset | Source | Licence |
|---|---|---|
| Base human mesh, macro and proportion targets, `game_engine` rig and weights, eyeballs, eyelashes | MPFB 2.0.18 (MakeHuman), installed as above | data **CC0 1.0** |
| MPFB add-on code (used to build only, not shipped) | same package | GPL-3.0-or-later |
| Helmet, straps, glasses, brows, shoes, cleats | built in `build_rider.py` | this repository |
| All textures | generated in `build_rider.py` (numpy) | this repository |
| Bike (reference only, not exported) | `../bike/build_bike.py` | this repository |

No paid assets are used, and nothing comes from Fab, Sketchfab or MetaHuman. There's no cloth or
physics simulation.

## Renders

- `render_chase.png`: about 3 m behind and 1.4 m up.
- `render_side_drive.png` and `render_side_non_drive.png`.
- `render_front.png`.
- `render_contact.png`: the drive side at frames 0, 8, 15, 23, 30, 38, 45 and 53, each labelled
  with its crank angle.
