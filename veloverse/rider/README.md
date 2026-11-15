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
- `T_*.png`: textures, 2048² (1024² with `--quick`).
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
- **Build targets:** a slim, narrow climber's silhouette that sits inside the bike's width:
  - narrower shoulders, torso and hips, a smaller seat and less V-taper;
  - slimmer upper arms, and thin legs: less thigh and calf girth and little fat on them;
  - a toned stomach.
- **Height:** scaled uniformly to exactly 1.780 m standing.
- **Fit:** the rider's proportions change to fit the bike. The saddle and bars never move.
  - Leg length uses MPFB's `upperlegs-height` and `lowerlegs-height` targets at +0.227, solved
    in the default pose. That puts the knee at the bottom of the stroke at 145.1° (target 145°).
  - The hip joint starts the fit at x −0.24 (sat back on the saddle, which opens the elbows a
    little) and ends at z 1.066. The thigh is 0.451 m and the calf 0.464 m.
- **Helpers:** MPFB's helper geometry is removed. Only the eyeballs and eyelashes are kept, as
  cheap meshes.
- **Hair and brows:** the hair is short and painted on the scalp in the body texture, under the
  helmet. The brows are thin strips. There's no particle hair.

**Pose on the hoods**, the default `Pedal` effort, measured at frame 0. It's a compact aero
position, built to read well from the game's chase camera straight behind:

| Measure | Value |
|---|---|
| Back, hip joint to the base of the neck | 32.0° from horizontal |
| Torso pitch | 59.1° |
| Shoulders | rounded forward and down: protracted 16°, dropped 8° |
| Elbows | 52° short of straight, tucked in under the chest (y ±0.13 at the elbow) |
| Sit bones | on the saddle top, sunk 3 mm into the pad |
| Palms | on top of the hood bodies, 0.553 m forward of the BB |
| Fingers | solved against the hoods (below) |
| Head | low, the neck extended 30° so the eyes look up the road, 10 m ahead |

From straight behind, the arms barely show outside the torso: the elbow pole sits back, down
and in, under the chest. The hoods are about as wide as the torso, so a few centimetres of
upper arm still show either side, as on a real rider.

The hands are fixed on the hoods, so a lower back can only come from bending the elbows. The
fit sets the back angle. The shoulders protract between 16° and 30° to keep the elbows at
least 17° bent, and the arms take up the rest. At 32° and 28° the shoulders are fully drawn
in, so the elbows bend 52° and 65°: more than "slightly bent", but the only way to keep both
those backs and the hands where they are.

**The grip** is fitted to the bike's hoods in two steps, once the proportions are settled.
The hands are fixed in the world, so one solve holds for every frame, and Unreal only plays it.

1. **Place the palm.** The hand is set in a handshake grip: rolled 75° outward, yawed 10° and
   pitched 5° down. The palm point is on the hood's centreline, 0.553 m forward of the BB
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
the hood body, the fingers curl round its outside and down over the lever blade, and the
thumb wraps the inside. No finger points up. Every finger rests 1.5 mm off the hood except the
right middle finger, which stops 8.7 mm short (the drive-side hood isn't a mirror of the
other).

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
| `Pedal_Easy` | 40° | 19° | ±0.9° | 12 m ahead | recovery, low power |
| `Pedal` (default) | 32° | 52° | ±1.5° | 10 m ahead | steady riding |
| `Pedal_Hard` | 28° | 65° | ±2.7° | 8 m ahead | efforts, sprints, steep climbs |
| `Cockpit` | (12°) | 27° | ±1.5° | hidden | the first-person camera only |

Each effort is a full fit: the same hands on the hoods, the same cleats on the pedals, and the
back, shoulders and elbows solved again. `rider.blend` keeps `Pedal` on the armature, and the
others as fake-user actions. The FBX exports all four as takes named `Armature|<action>`.

**`Cockpit`** is for the first-person view, and only the arms are meant to be seen:

- **Camera:** at (−0.27, 0, 1.12), looking forward, pitched 5° down, 61° horizontal FOV at
  16:9. The bars sit in the bottom third.
- **Hands:** exactly as in `Pedal`: the same world transforms and finger curls.
- **Arms:** the shoulders are drawn down and back (the pelvis dropped to z 0.78 behind the
  saddle, the torso pitched 80°, the elbows pointing out), so the forearms reach in from the
  bottom corners of the frame to the hoods.
- **Checked every 4th frame:** the camera sees no vertex of the torso, the upper arms or the
  head, the arm and shoulder skin stays 0.41 m from the camera, and the forearms enter at
  the bottom edge, 76% of the way out to the sides.
- **The rest is folded away and must be hidden:** the head is pulled down to the top tube
  (the helmet sits 17 mm into the frame), the pelvis is behind and below the saddle, and the
  knees bend 43–96°. Hide the head and legs in the game (`render_game_cockpit.png` shrinks
  the `head` bone to a point for the same reason). Don't blend `Cockpit` with the efforts.

**Driving it from power in Unreal:** import the takes as three animation sequences and put
them in a 1D blend space on W/kg, for example Easy at 1.5, Pedal at 3.0 and Hard at 5.0.
Scale the play rate by cadence / 60, since one loop is one revolution at 60 rpm. Keep every
sample's crank phase in sync (a sync group, or one shared time), as the blend assumes it.
Blending halfway between two efforts moves the hands and feet off the bike by at most 8.2 mm
(Easy to Pedal) and 1.1 mm (Pedal to Hard). That's too small to see; a two-bone IK on the
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
- **Knees:** from about 70° at the top to 144–145° at the bottom of the stroke, the same on both
  legs, in every effort. They never lock.
- **Upper body:**
  - The hands are fixed in the world.
  - The pelvis rolls (±0.9° to ±2.7°, by effort) and drops with each downstroke.
  - The upper torso counter-rolls only 0.25°, just enough to hold the shoulders still.
  - The head stays aimed at the road, 8–12 m ahead by effort.
- **Loop seam:** every bone's step from frame 59 back to frame 0 is checked against the largest
  step between consecutive frames. In every action it is 18.3 mm and 2.3° at most, against
  19.5 mm and 2.6° (23.6 mm and 3.0° in `Cockpit`), so the loop has no pop.

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
| Knee angle, min / max | 70.4° / 144.6° in `Pedal`; 70.1° / 144.0° in `Pedal_Easy` |
| Largest hand-to-hood distance | 0.0 mm (to the palm point), every action |
| Interpenetration deeper than 5 mm | kit into the frame (the inner thighs on the top tube), 7.0–7.8 mm in the efforts; `Cockpit` also has its folded head, helmet and legs in the frame, up to 17.5 mm (hidden in game) |
| Hand and foot drift at a halfway blend | 8.2 mm Easy/Pedal, 1.1 mm Pedal/Hard |

**Frame 0, per action.** Unreal can check itself against these; `stats.json` has them under
`frame0`, and the game cameras under `cameras`. The palms and hood contacts are the same in
every action: palm L (0.553, 0.198, 0.923), R (0.553, −0.198, 0.923); hood contact (the top of
the hood under the palm) L (0.553, 0.198, 0.894), R (0.553, −0.198, 0.894).

| Action | Back | Eye point | `head` bone |
|---|---|---|---|
| `Pedal_Easy` | 40.0° | (0.366, −0.002, 1.467) | (0.281, −0.002, 1.436) |
| `Pedal` | 32.0° | (0.412, −0.004, 1.396) | (0.326, −0.004, 1.367) |
| `Pedal_Hard` | 28.0° | (0.432, −0.007, 1.352) | (0.345, −0.007, 1.326) |
| `Cockpit` | 12.3° | (0.181, 0.004, 0.714) | (0.141, 0.004, 0.795) |

The penetration check confirms each candidate vertex with ray-parity counts. The nearest face's
normal alone misread a fingertip in the hollow between the hood and the lever as 27.6 mm inside.

The game's assumed eye point is (0.18, ·, 1.50). On this bike, at a 32° back, the real one is
0.23 m further forward and 0.10 m lower. The game's camera should take the eye point from
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
- `render_chase.png`: about 3 m behind and 1.4 m up.
- `render_side_drive.png` and `render_side_non_drive.png`.
- `render_front.png`.
- `render_efforts.png`: the drive side at frame 0 in each effort, side by side.
- `render_contact.png`: the drive side at frames 0, 8, 15, 23, 30, 38, 45 and 53, each labelled
  with its crank angle.
