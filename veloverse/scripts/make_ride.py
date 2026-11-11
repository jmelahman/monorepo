"""Builds a ride for Movie Render Queue: a level sequence with a cine camera riding at cycling
speed,
and a render config that writes PNGs. Run it as a commandlet, then render with

    UnrealEditor PROJECT MAP -game -LevelSequence=/Game/Spike/SEQ_Ride
    -MoviePipelineConfig=/Game/Spike/MRQ_Ride ...

Settings come from the environment:
    OC_LEVEL    map whose PlayerStart sets the start and heading (default the PCG biome sample)
    OC_SECONDS  length of the ride (default 3)
    OC_SPEED    metres per second (default 8)
    OC_EYE      camera height above the ground in cm (default 160)
    OC_OUT      folder the frames go to (default /workspace/ue/out/ride)
    OC_RES      WIDTHxHEIGHT (default 1920x1080)
    OC_AT       X,Y,Z camera start in cm, instead of the PlayerStart and eye height
    OC_YAW      heading in degrees, instead of the PlayerStart's
    OC_CURVE    bend amplitude in cm of the shared road's centreline (default 1200, as
                make_forest.py)
    OC_WAVELENGTH   the bend's wavelength in cm (default 45000)
    OC_PITCH    camera pitch in degrees (default -2)
    OC_EV       fixed exposure as EV100, through ISO 100 and f/5.6 (default automatic, like a GoPro)

OC_CURVE and OC_WAVELENGTH bend only the default ride, whose position and yaw follow the
same centreline forest_layout lays the road and shoulders out on. An explicit OC_AT or
OC_YAW keeps the camera on the old straight line along its heading, so those overrides do
not follow the bend.
"""

import importlib.util
import math
import os

import unreal


def sibling(name):
    """Imports one of our own helper modules by file path.

    A plain import would need the scripts folder on sys.path, where its inspect.py
    shadows the stdlib inspect module that Unreal's own tooling may still want untouched.
    """
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/make_ride.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


forest_layout = sibling("forest_layout")

LEVEL = os.environ.get("OC_LEVEL") or "/PCGBiomeSample/Maps/BiomeSampleLevel"
SECONDS = float(os.environ.get("OC_SECONDS") or "3")
SPEED = float(os.environ.get("OC_SPEED") or "8") * 100.0
EYE = float(os.environ.get("OC_EYE") or "160")
OUT = os.environ.get("OC_OUT") or "/workspace/ue/out/ride"
WIDTH, HEIGHT = (int(v) for v in (os.environ.get("OC_RES") or "1920x1080").split("x"))
PITCH = float(os.environ.get("OC_PITCH") or "-2")
EV = os.environ.get("OC_EV")
CURVE = float(os.environ.get("OC_CURVE") or "1200")  # same default as make_forest.py
WAVELENGTH = float(os.environ.get("OC_WAVELENGTH") or "45000")
FPS = 24
FOLDER = "/Game/Spike"
KEY_STEP = 500.0  # cm of road between two transform keys; ~1 cm off the curve at most


def log(message):
    unreal.log(f"RIDE {message}")


def fresh(name, cls, factory):
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)
    return unreal.AssetToolsHelpers.get_asset_tools().create_asset(name, FOLDER, cls, factory)


unreal.EditorLoadingAndSavingUtils.load_map(LEVEL)
actors = unreal.get_editor_subsystem(unreal.EditorActorSubsystem)
start = next((a for a in actors.get_all_level_actors() if isinstance(a, unreal.PlayerStart)), None)
if start:
    origin, extent = start.get_actor_bounds(False)
    ground = origin.z - extent.z
    yaw = start.get_actor_rotation().yaw
else:  # our own levels: the road starts at the origin and runs along +X
    origin, ground, yaw = unreal.Vector(0, 0, 0), 0.0, 0.0
eye = ground + EYE
if os.environ.get("OC_AT"):
    x, y, eye = (float(v) for v in os.environ["OC_AT"].split(","))
    origin = unreal.Vector(x, y, eye)
if os.environ.get("OC_YAW"):
    yaw = float(os.environ["OC_YAW"])
heading = unreal.Vector(math.cos(math.radians(yaw)), math.sin(math.radians(yaw)), 0.0)
log(f"start {origin.x:.0f},{origin.y:.0f} ground {ground:.0f} yaw {yaw:.1f}")

# The ride rides the same centreline the forest road and its shoulders are laid out on.
# Only the default camera bends: an explicit OC_AT or OC_YAW keeps the old straight ride.
road = forest_layout.RoadLayout(amplitude=CURVE, wavelength=WAVELENGTH)
overridden = bool(os.environ.get("OC_AT") or os.environ.get("OC_YAW"))
log(
    f"curve {CURVE:.0f} cm amplitude at {WAVELENGTH:.0f} cm wavelength"
    + ("" if overridden else ", followed by the default camera")
    + (", ignored for the OC_AT/OC_YAW override" if overridden else "")
)

# The camera, set up in the level and then copied into the sequence as a spawnable.
camera = actors.spawn_actor_from_class(
    unreal.CineCameraActor, unreal.Vector(origin.x, origin.y, eye)
)
lens = camera.get_cine_camera_component()
lens.filmback.sensor_width = 23.76
lens.filmback.sensor_height = 13.365
lens.current_focal_length = 14.0  # about 80 degrees across, a handlebar camera
lens.focus_settings.focus_method = unreal.CameraFocusMethod.DISABLE
# A real camera's exposure: automatic by default, or fixed through ISO 100 and f/5.6 at OC_EV.
lens.current_aperture = 5.6
look = lens.post_process_settings
look.override_auto_exposure_method = True
look.override_auto_exposure_apply_physical_camera_exposure = True
if EV:
    look.auto_exposure_method = unreal.AutoExposureMethod.AEM_MANUAL
    look.auto_exposure_apply_physical_camera_exposure = True
    look.override_camera_iso = True
    look.camera_iso = 100.0
    look.override_camera_shutter_speed = True
    look.camera_shutter_speed = 2.0 ** float(EV) / 5.6**2
else:
    look.auto_exposure_method = unreal.AutoExposureMethod.AEM_HISTOGRAM
    look.auto_exposure_apply_physical_camera_exposure = False
    look.override_auto_exposure_min_brightness = True
    look.auto_exposure_min_brightness = -10.0
    look.override_auto_exposure_max_brightness = True
    look.auto_exposure_max_brightness = 20.0
look.override_motion_blur_amount = True
look.motion_blur_amount = 0.0
lens.post_process_settings = look

frames = round(SECONDS * FPS)
sequence = fresh("SEQ_Ride", unreal.LevelSequence, unreal.LevelSequenceFactoryNew())
sequence.set_display_rate(unreal.FrameRate(FPS, 1))
sequence.set_playback_start(0)
sequence.set_playback_end(frames)

binding = sequence.add_spawnable_from_instance(camera)
actors.destroy_actor(camera)

motion = binding.add_track(unreal.MovieScene3DTransformTrack).add_section()
motion.set_range(0, frames)
channels = motion.get_all_channels()
# The start and end frames still fix the ride's length; extra keys in between only keep the
# camera on the bend (a 500 cm chord sits within about 1 cm of the default curve).
if SPEED > 0:
    step = max(1, round(KEY_STEP * FPS / SPEED))
    key_frames = sorted(set(range(0, frames + 1, step)) | {0, frames})
else:
    key_frames = (0, frames)
# A bike, not a dolly: a small bob and roll at the pedalling rhythm and a slow steering wobble,
# keyed every frame. OC_SWAY scales it (0 for the old rail).
SWAY = float(os.environ.get("OC_SWAY") or "1")
if SWAY > 0 and SPEED > 0:
    key_frames = range(0, frames + 1)


def sway(t):
    """Height (cm), roll and yaw (degrees) offsets at t seconds; cadence about 85 rpm."""
    crank = 2 * math.pi * 1.42 * t
    return (
        SWAY * (0.9 * math.sin(2 * crank) + 0.4 * math.sin(0.37 * crank + 1.0)),
        SWAY * (0.7 * math.sin(crank) + 0.25 * math.sin(0.23 * crank + 2.0)),
        SWAY * (0.35 * math.sin(0.5 * crank + 0.5) + 0.2 * math.sin(0.17 * crank)),
    )


for frame in key_frames:
    along = SPEED * frame / FPS
    if overridden:  # a hand-placed camera keeps the pre-curve straight ride exactly
        where = (origin.x + heading.x * along, origin.y + heading.y * along, eye)
        pitch, roll, yaw_key = PITCH, 0.0, yaw
    else:
        where, (pitch, yaw_key, roll) = road.world_pose(
            along, height_cm=eye, pitch_deg=PITCH, origin=(origin.x, origin.y), base_yaw=yaw
        )
    if SWAY > 0 and SPEED > 0:
        lift, tilt, turn = sway(frame / FPS)
        where, roll, yaw_key = (where[0], where[1], where[2] + lift), roll + tilt, yaw_key + turn
    for channel, value in zip(channels[:6], (*where, roll, pitch, yaw_key), strict=False):
        channel.add_key(
            unreal.FrameNumber(frame),
            value,
            0.0,
            unreal.MovieSceneTimeUnit.DISPLAY_RATE,
            unreal.MovieSceneKeyInterpolation.LINEAR,
        )

cut = sequence.add_track(unreal.MovieSceneCameraCutTrack).add_section()
cut.set_range(0, frames)
cut.set_camera_binding_id(sequence.get_portable_binding_id(sequence, binding))

config = fresh(
    "MRQ_Ride", unreal.MoviePipelinePrimaryConfig, unreal.MoviePipelinePrimaryConfigFactory()
)
output = config.find_or_add_setting_by_class(unreal.MoviePipelineOutputSetting)
output.output_directory = unreal.DirectoryPath(OUT)
output.output_resolution = unreal.IntPoint(WIDTH, HEIGHT)
output.file_name_format = "frame.{frame_number}"
output.override_existing_output = True
config.find_or_add_setting_by_class(unreal.MoviePipelineDeferredPassBase)
config.find_or_add_setting_by_class(unreal.MoviePipelineImageSequenceOutput_PNG)
# Lumen and the PCG scatter both need frames to settle before the first one is kept.
warm = config.find_or_add_setting_by_class(unreal.MoviePipelineAntiAliasingSetting)
warm.engine_warm_up_count = 120
warm.render_warm_up_count = 64
warm.use_camera_cut_for_warm_up = False
config.find_or_add_setting_by_class(unreal.MoviePipelineGameOverrideSetting)

for asset in (sequence, config):
    unreal.EditorAssetLibrary.save_loaded_asset(asset, False)
log(f"saved {sequence.get_path_name()} ({frames} frames) and {config.get_path_name()} -> {OUT}")
