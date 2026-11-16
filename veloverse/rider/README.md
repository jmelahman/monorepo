# Player rider: skinned cyclist on the hoods

`build_rider.py` builds the rider, the gear, three looping pedal actions (one per effort) and a
`Cockpit` action for the first-person camera, headless. It works in
Blender from the MPFB 2 base human. The tunable values are constants at the top of the script.
The bike is loaded only as reference geometry: `../bike/bike.blend` if it exists. Otherwise
`../bike/build_bike.py` builds it into `out/bike_ref/`. The bike is never exported or modified.

```sh
blender -b -P build_rider.py                        # full build + renders (~9.5 min, 24-core CPU)
blender -b -P build_rider.py -- --no-render         # mesh, FBX, textures and stats only
blender -b -P build_rider.py -- --quick             # 1K textures, low-sample half-size renders
blender -b -P build_rider.py -- --out /some/dir     # write outputs elsewhere (default: ./out)
blender -b -P build_rider.py -- --bike-blend PATH   # use another bike .blend as the reference
```

`--stage probe` and `--stage fit` stop early and save `probe.blend` or `fit.blend`, for
debugging. The script exits non-zero on any error.

Outputs go to `out/`. They are regenerated and not committed:

- `rider.fbx`: the armature, the one mesh and the `Pedal_Easy`, `Pedal`, `Pedal_Hard` and
  `Cockpit` actions. No bike.
- `rider.blend`: the rider plus the bike for reference. The IK constraints are already removed.
- `T_*.png`: textures, 2048² (1024² with `--quick`), including the tint masks (see Materials).
- `render_*.png`: renders with the bike: 1600 px wide, or 1280×720 for the game's views.
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
- **Build targets:** a lean road rider who sits inside the bike's width:
  - narrower shoulders, torso and hips, a smaller seat and less V-taper, with the lats
    flattened so the upper back doesn't bulge from behind;
  - a shorter neck;
  - flatter deltoids, so the shoulders run into the arms instead of standing off them like
    pads, with defined biceps and triceps;
  - a rider's legs: more thigh and calf muscle and girth, with little fat on them;
  - a toned stomach.
- **The kit is cloth, not skin** (it's painted on the body, so the body is shaped for it):
  - `fill_cleft` fills the cleft of the seat out to the cheeks either side, so the shorts
    span it;
  - `drape_kit` smooths the jersey and shorts in the riding pose, mostly outward, so they
    bridge the spine's furrow, the shoulder blades and the armpits. The legs are left alone.
    The kit is painted from where the skin was before the drape, so its hems stay straight.
- **Height:** scaled uniformly to exactly 1.780 m standing.
- **Fit:** the rider's proportions change to fit the bike. The saddle and bars never move.
  - Leg length uses MPFB's `upperlegs-height` and `lowerlegs-height` targets at +0.165, solved
    in the default pose. That puts the knee at the bottom of the stroke at 144.2° (target 145°).
  - The hip joint starts the fit at x −0.24 (sat back on the saddle, which opens the elbows a
    little) and ends at z 1.076. The thigh is 0.454 m and the calf 0.470 m.
- **Helpers:** MPFB's helper geometry is removed. Only the eyeballs and eyelashes are kept, as
  cheap meshes.
- **Forearms:** remodelled for the cockpit view (`shape_arms`), in the hoods grip with the palm
  in and the thumb up. Each forearm gets one level of subdivision, and each section is remapped
  to a flattened oval, wider across than deep. It tapers from 9.2 × 6.5 cm just below the elbow
  to 5.4 × 4.0 cm at the wrist. The brachioradialis bulges along the thumb side, which is the
  top edge from the cockpit, and the flexors fill the palm side. The forearm is straight from
  elbow to wrist: the skin passes from the lower arm to the hand within 12 mm each side of the
  wrist joint, so the bend sits at the wrist. The knuckles' ridge is raised 1.5 mm on the back
  of each hand. Each hand, fingers included, is about 3.4k triangles.
- **Hair and brows:** the hair is short and painted on the scalp in the body texture, under the
  helmet. In the riding crouch the helmet's rim covers the hairline from behind. The brows are thin strips. There's no particle hair.

**Pose on the hoods**, the default `Pedal` effort, measured at frame 0. It's a compact aero
position, built to read well from the game's chase camera straight behind:

| Measure | Value |
|---|---|
| Back, hip joint to the base of the neck | 25.0° from horizontal |
| Torso pitch | 69.7° |
| Shoulders | dropped 8°, drawn back 5° |
| Elbows | 48° bent, with the forearms running to the outside of the hoods |
| Sit bones | on the saddle top, sunk 3 mm into the pad |
| Palms | on top of the hood bodies, 0.542 m forward of the BB |
| Fingers | solved against the hoods (below) |
| Head | lifted to look up the road to 10 m ahead, tipped 10° down from that line of sight; the neck sits 15° past halfway between the back and the head |

The back is rounded: the pelvis takes 62% of the torso's pitch and the spine curls the rest,
mostly low down, so the lower back doesn't hollow and the seat doesn't stick out. From
straight behind, the arms hang close to the torso from the shoulders to the outside of the
hoods: the elbow pole sits back and down in the plane of the arm, so the elbows neither bow
in under the chest nor flare out.

The hands are fixed on the hoods, so a lower back can only come from bending the elbows. The
fit sets the back angle. The shoulders move between 5° drawn back and 30° protracted to keep
the elbows 24° bent, and the arms take up the rest. The fit runs again after the grip lifts
the palms (below). `Pedal_Easy` holds that bend with the shoulders 9° forward. In `Pedal`
and `Pedal_Hard` the shoulders are drawn back to their limit and the elbows bend further, to
48° and 77°.

**The grip** is fitted to the bike's hoods in two steps, once the proportions are settled.
The hands are fixed in the world, so one solve holds for every frame, and Unreal only plays it.

1. **Place the palm.** The hand is set in a handshake grip: rolled 88° outward, not yawed and
   pitched 2° down. The palm point is on the hood's centreline, 0.542 m forward of the BB
   (on the hood body, behind the horn). It rises until the palm, the knuckles and the fingers'
   first segments rest 1.5 mm off the hood: 19.7 mm above the hood top on this bike. The arms
   follow by IK. A search over yaw, pitch and roll found that only a near-handshake roll lets
   every finger touch the hood with its tip below its knuckle; at 45–60° the ring and little
   fingers hang in the air.
2. **Close the fingers and thumbs.** Each joint, knuckle first, starts at a relaxed curl. If
   it's in the hood there, it opens to where it's clearest. Then it closes until its skin
   would come within 1.5 mm of the hood, lever or bar. The skin is checked by skinning the
   hand in numpy against the cockpit mesh.

The result, checked from above, behind, outside, inside and in front: the palm sits on top of
the hood body, and the index and middle fingers wrap the hood's body. The ring finger rests on
the hood's lower edge and the little finger curls round the bar below it, and the thumb wraps
under the hood's inside. Every fingertip rests within 1.5–3.6 mm of the hood or bar tape. `Cockpit` has its own grip, in the drops (below).

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

## Animation: three efforts and the cockpit

There are three pedal actions, one per effort, and `Cockpit`. All four share the crank
timing, so the efforts blend frame for frame:

| Action | Back | Elbows | Pelvis roll | Head aims | Use for |
|---|---|---|---|---|---|
| `Pedal_Easy` | 35° | 25° | ±0.9° | 12 m ahead | recovery, low power |
| `Pedal` (default) | 25° | 48° | ±1.5° | 10 m ahead | steady riding |
| `Pedal_Hard` | 15° | 77° | ±2.7° | 8 m ahead | efforts, sprints, steep climbs |
| `Cockpit` | (8°) | 64° | ±1.5° | hidden | the first-person camera only |

Each effort is a full fit: the same hands on the hoods (`Cockpit`: in the drops), the same
cleats on the pedals, and the
back, shoulders and elbows solved again. `rider.blend` keeps `Pedal` on the armature, and the
others as fake-user actions. The FBX exports all four as takes named `Armature|<action>`.

**`Cockpit`** is for the first-person view, and only the arms are meant to be seen:

- **Camera:** at (−0.27, 0, 1.12), looking forward, pitched 8° down (the game's `CockpitTilt`), 61° horizontal FOV at
  16:9. The bars sit in the bottom third.
- **Hands:** in the hooks of the drops (`place_drops`), not on the hoods. From this camera
  the hoods grip showed only the thumb's edge of a hand over a small hood; a fist round the
  bar reads at a glance. The palm rests against the outside of the hook, thumb up. The
  middle and ring fingers wrap the front of the bar and the index finger lies up the bar
  toward the lever. The hook curves away from the little finger, so it curls like the ring
  finger rather than closing on air. The palm sits 10 mm behind the tube's middle
  (`DROP_BACK`), which leaves room for the thumb: its first joint swings 20° and it closes
  round the inside of the bar, 2 mm clear of the tape. Further forward the thumb's base sat
  in the tape and the thumb lay through the bar in this view. The palm points are
  (0.551, ±0.228, 0.794).
- **Arms:** the shoulders are drawn down and back (the pelvis dropped to z 0.76 behind the
  saddle, the torso pitched 88°, the shoulders square and the elbows out), so the forearms
  reach in from the bottom corners of the frame to the drops.
- **Checked every 4th frame:** the camera sees no vertex of the torso, the upper arms or the
  head, the arm and shoulder skin stays 0.46 m from the camera, and the forearms enter at
  the bottom edge, 74% of the way out to the sides.
- **The rest is folded away and must be hidden:** the head is pulled down to the top tube
  (the helmet sits 20 mm into the frame), the pelvis is behind and below the saddle, and the
  knees bend 34–84°. Hide the head and legs in the game (`render_game_cockpit.png` shrinks
  the `head` bone to a point for the same reason). Don't blend `Cockpit` with the efforts.

**Driving it from power in Unreal:** import the takes as three animation sequences and put
them in a 1D blend space on W/kg, for example Easy at 1.5, Pedal at 3.0 and Hard at 5.0.
Scale the play rate by cadence / 60, since one loop is one revolution at 60 rpm. Keep every
sample's crank phase in sync (a sync group, or one shared time), as the blend assumes it.
Blending halfway between two efforts moves the hands and feet off the bike by at most 3.8 mm
(Easy to Pedal) and 5.6 mm (Pedal to Hard). That's too small to see; a two-bone IK on the
hands in the Anim Blueprint would remove it if it ever shows.

Every action:

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
- **Knees:** from about 70–72° at the top to 141–147° at the bottom of the stroke, the same on both
  legs, by effort. They never lock.
- **Upper body:**
  - The hands are fixed in the world.
  - The pelvis rolls (±0.9° to ±2.7°, by effort) and drops with each downstroke.
  - The upper torso counter-rolls only 0.25°, just enough to hold the shoulders still.
  - The head stays aimed at the road, 8–12 m ahead by effort.
- **Loop seam:** every bone's step from frame 59 back to frame 0 is checked against the largest
  step between consecutive frames. In every action it is 18.6 mm and 2.3° at most, against
  19.1–20.0 mm and 2.6° (28.4 mm and 3.6° in `Cockpit`), so the loop has no pop.

## Materials

There are 7 slots on the one mesh. The textures are 2048², with the normal maps in OpenGL
convention (green up). The roughness is a constant per material, except on the body, which has
`T_Body_Roughness` (skin about 0.70, kit 0.85, as before). Everything is kept matte and
simple, with flat colours and faint surface detail, so the rider sits in the world rather than
looking like a studio scan.

| Slot | Textures | Roughness | Other | Used on |
|---|---|---|---|---|
| `M_Skin` | `T_Body_BaseColor`, `T_Body_Normal`, `T_Body_Roughness` | 0.70 (texture) | subsurface 0.05 | skin, the short hair on the scalp |
| `M_Kit` | `T_Body_BaseColor`, `T_Body_Normal`, `T_Body_Roughness` (same as skin) | 0.85 (texture) | | jersey, bib shorts, socks (painted on the body) |
| `M_Helmet` | `T_Helmet_BaseColor`, `T_Helmet_Normal` | 0.55 | | helmet shell |
| `M_Lens` | none: base (0.010, 0.011, 0.014) | 0.15 | metallic 0.3 | glasses lens, eyeballs |
| `M_Shoe` | `T_Shoe_BaseColor`, `T_Shoe_Normal` | 0.60 | | shoe uppers, soles, cleats |
| `M_Strap` | none: base (0.012, 0.012, 0.013) | 0.80 | | helmet straps and buckle, the helmet's inner liner, glasses frame |
| `M_Hair` | none: base (0.030, 0.019, 0.012) | 0.80 | | brows, eyelashes |

The kit is a texture on the body, not cloth. Colours are given in linear RGB:

- **Jersey:** navy (0.010, 0.024, 0.075), with a crew neck that leaves the throat bare and
  short sleeves that end 4 cm above the elbow with a hem: a fold in the normal map, slightly
  darker.
- **Accents:** orange (0.80, 0.22, 0.05) on the sleeve cuffs and the side panels, each a
  straight round-ended stripe 56 mm wide from the hem to the armpit.
- **Bib shorts:** black, ending just above the knee (90% of the way down the thigh).
- **Socks:** white, mid-calf.
- **No marks:** there are no brands, logos or text anywhere.
- **Normal map:** a faint knit on the kit and barely any pore detail on the skin. On the arms
  and hands it adds faint flexor tendons on the inside of the wrist, two veins on the forearm
  (the cephalic along the thumb side and a branch across the back), extensor tendons fanning
  over the back of the hand, and bumps and creases over the knuckles. The skin is slightly
  glossier over those (the roughness drops by up to 0.1).
- **Tan line:** faint (`TAN_PALE` 0.15; at 0.80 it read as a bandage under the sleeve and as
  pale patches at the cockpit frame's edge). The skin under the sleeve is slightly paler,
  and the pale skin runs 1 cm into the cockpit frame along the forearm. `stats.json` gives the cut as
  `forearm_m.tan_line_t`, from 0 at the elbow to 1 at the wrist.

The gear:

- **Helmet:** a smooth aero road shell in plain navy that wraps the skull: the rim sits above
  the brow, level over the ears and well down the back of the head, with a short tail and
  slimmer sides. It is smooth shaded. The vents are shallow dips (6 mm) in the shell, painted
  black and normal-mapped; the paint is wider than the dip, so the slot's edge stays clean.
  Black rim and straps.
- **Shoes:** white uppers with perforations, a black sole, heel cup and strap, and a cleat.

### Tint masks: skin tone, kit colours and swaps

The base colour textures bake one look. To change colours in Unreal without new textures, the
build also writes masks (linear, not sRGB), one region per channel:

| Texture | R | G | B |
|---|---|---|---|
| `T_Body_Mask` | jersey | accent (side panels, cuffs) | shorts |
| `T_Body_Mask2` | socks | hair | skin |
| `T_Helmet_Mask` | shell | rim | vents |
| `T_Shoe_Mask` | upper | trim (sole, heel cup, strap) | cleat |

- **Channels sum to 1** on the body, and edges are antialiased, so a region's colour is a
  plain weighted sum: `colour = Σ channel × its colour parameter`.
- **`T_Body_Shade`** is greyscale: the baked colour's brightness over its region's flat
  colour, with 0.5 meaning no change. It carries the skin's mottling, the darker hems and the
  tan line. Multiply the tinted colour by `T_Body_Shade / 0.5`.
- **Defaults:** `stats.json` lists each channel's linear colour under `masks`. With those as
  the parameters, the tinted material closely matches `T_*_BaseColor`.
- **In Unreal:** import the masks with sRGB off (compression "Masks"). Make one material per
  slot with a vector parameter per channel, and set them on a dynamic material instance:
  skin tone and hair on `M_Skin`, the kit colours on `M_Kit`. Both slots use the same body
  textures, so both can share one parent material.
- **Other uniforms:** a design that keeps these regions is only a new set of colours. A new
  cut (longer sleeves, a different panel) is a change to `BodyMap.labels` in
  `build_rider.py` and a rebuild; the UVs and skeleton don't change, so a new set of body
  textures drops onto the same mesh.
- **Accessories:** the helmet, glasses, straps and shoes are separate material slots joined
  into the one mesh. To swap them in game they would export as their own skinned meshes on
  the same skeleton; that split isn't built yet.

## Stats (full build)## Stats (full build)

| | |
|---|---|
| Triangles | **65,212**: Skin 19,896, Kit 5,992, Helmet 13,272, Strap 15,472, Shoe 8,376, Lens 1,804, Hair 400 |
| Bones | 53 |
| Most influences per vertex | 4 |
| Largest cleat-to-axle distance | 0.1 mm |
| Knee angle, min / max | 70.8° / 144.2° in `Pedal`; 71.8° / 147.4° in `Pedal_Easy`; 69.8° / 141.3° in `Pedal_Hard` |
| Largest hand-to-grip distance | 0.0 mm (to the palm point: the hoods, or the drops in `Cockpit`), every action |
| Interpenetration deeper than 5 mm | kit into the frame (the inner thighs on the top tube), 6.9–8.2 mm in the efforts; `Cockpit` also has its folded head, helmet and legs in the frame and fork, up to 20.3 mm (hidden in game) |
| Hand and foot drift at a halfway blend | 3.8 mm Easy/Pedal, 5.6 mm Pedal/Hard |
| Forearm, width × depth (`forearm_m`) | 9.2 × 6.5 cm below the elbow (t 0.12), 8.1 × 5.5 cm mid (t 0.5), 5.4 × 4.0 cm at the wrist (t 0.95), both sides |

**Frame 0, per action.** Unreal can check itself against these; `stats.json` has them under
`frame0`, and the game cameras under `cameras`. The palms and hood contacts are the same in
every effort: palm L (0.542, 0.199, 0.923), R (0.542, −0.199, 0.923); hood contact (the top of
the hood under the palm) L (0.542, 0.199, 0.892), R (0.542, −0.199, 0.892). `Cockpit`'s palms
are in the drops, at (0.551, ±0.228, 0.794).

| Action | Back | Eye point | `head` bone |
|---|---|---|---|
| `Pedal_Easy` | 34.9° | (0.382, −0.003, 1.427) | (0.291, −0.003, 1.411) |
| `Pedal` | 25.0° | (0.427, −0.004, 1.325) | (0.336, −0.004, 1.311) |
| `Pedal_Hard` | 15.1° | (0.456, −0.008, 1.216) | (0.364, −0.008, 1.203) |
| `Cockpit` | 8.4° | (0.221, 0.004, 0.663) | (0.179, 0.004, 0.745) |

The penetration check confirms each candidate vertex with ray-parity counts. The nearest face's
normal alone misread a fingertip in the hollow between the hood and the lever as 27.6 mm inside.

The game's assumed eye point is (0.18, ·, 1.50). On this bike, at a 25° back, the real one is
0.25 m further forward and 0.17 m lower. The game's camera should take the eye point from
`stats.json` (or from the `head` bone) rather than the constant.

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

- `render_game_chase.png`: the game's chase camera, 5.1 m behind (0.18, 0, 1.62) along a view
  pitched 2° down, 61° HFOV, 16:9.
- `render_game_cockpit.png`: `Cockpit` from the game's cockpit camera, with the head hidden.
- `render_hands_top.png`: both hands on the hoods from above and behind.
- `render_cockpit_closeup.png`: `Cockpit` from the cockpit camera at 30° HFOV, centred on the
  right hand in the drops.
- `render_chase.png`: about 3 m behind and 1.4 m up.
- `render_side_drive.png` and `render_side_non_drive.png`.
- `render_front.png`.
- `render_efforts.png`: the drive side at frame 0 in each effort, side by side.
- `render_contact.png`: the drive side at frames 0, 8, 15, 23, 30, 38, 45 and 53, each labelled
  with its crank angle.
