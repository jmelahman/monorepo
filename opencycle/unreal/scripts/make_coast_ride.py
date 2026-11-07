"""Builds the coast ride's sequence and render config: /Game/Coast/SEQ_Coast and MRQ_Coast.

Two modes, by OC_MODE:
    ride    a cine camera riding coast_layout's path from OC_FROM metres for OC_SECONDS at
            OC_SPEED m/s, keeping left as Japanese cyclists do, with a pedalling sway
    stills  one held shot per entry of OC_STILLS, "METRES[:YAW[:PITCH[:LATERAL[:EYE]]]]" joined by
            ',', each held for HOLD frames so Lumen and TAA settle; keep the last frame of each

Other settings: OC_OUT (frames folder), OC_RES (WIDTHxHEIGHT, default 1920x1080), OC_EYE (cm,
default 150), OC_SWAY (default 1.6), OC_FOCAL (mm, default 16), OC_CVARS ("name=value;..." for the
render).
"""

import importlib.util
import math
import os

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/make_coast_ride.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


layout = sibling("coast_layout")
PATH = layout.CoastPath()
MODE = os.environ.get("OC_MODE") or "ride"
FROM = float(os.environ.get("OC_FROM") or "0") * 100.0
SECONDS = float(os.environ.get("OC_SECONDS") or "10")
SPEED = float(os.environ.get("OC_SPEED") or "7") * 100.0
EYE = float(os.environ.get("OC_EYE") or "150")
SWAY = float(os.environ.get("OC_SWAY") or "1.6")
FOCAL = float(os.environ.get("OC_FOCAL") or "16")
OUT = os.environ.get("OC_OUT") or "/workspace/ue/out/coast"
WIDTH, HEIGHT = (int(v) for v in (os.environ.get("OC_RES") or "1920x1080").split("x"))
FPS = 24
HOLD = int(os.environ.get("OC_HOLD") or "40")
FOLDER = "/Game/Coast"
KEEP_LEFT = -45.0


def log(message):
    unreal.log(f"RIDE {message}")


def fresh(name, cls, factory):
    path = f"{FOLDER}/{name}"
    if unreal.EditorAssetLibrary.does_asset_exist(path):
        unreal.EditorAssetLibrary.delete_asset(path)
    return unreal.AssetToolsHelpers.get_asset_tools().create_asset(name, FOLDER, cls, factory)


def pose(s, lateral=KEEP_LEFT, eye=EYE):
    """Camera (x, y, z, yaw, pitch) at s: yaw looks a few metres ahead, pitch follows the grade."""
    x, y, z = PATH.point(s, lateral, eye)
    ahead = PATH.at(PATH.wrap(s + 600))
    yaw = math.degrees(math.atan2(ahead[1] - y, ahead[0] - x))
    grade = math.degrees(math.atan2(PATH.height(PATH.wrap(s + 800)) - PATH.height(s), 800))
    return x, y, z, yaw, grade * 0.6 - 3.0


def sway(t):
    crank = 2 * math.pi * 1.42 * t
    return (
        SWAY * (0.9 * math.sin(2 * crank) + 0.4 * math.sin(0.37 * crank + 1.0)),
        SWAY * (0.7 * math.sin(crank) + 0.25 * math.sin(0.23 * crank + 2.0)),
        SWAY * (0.35 * math.sin(0.5 * crank + 0.5) + 0.2 * math.sin(0.17 * crank)),
    )


unreal.EditorLoadingAndSavingUtils.load_map(f"{FOLDER}/L_Coast")
actors = unreal.get_editor_subsystem(unreal.EditorActorSubsystem)
camera = actors.spawn_actor_from_class(unreal.CineCameraActor, unreal.Vector(0, 0, 0))
lens = camera.get_cine_camera_component()
lens.filmback.sensor_width = 23.76
lens.filmback.sensor_height = 13.365
lens.current_focal_length = FOCAL
lens.focus_settings.focus_method = unreal.CameraFocusMethod.DISABLE
lens.current_aperture = 5.6
look = lens.post_process_settings
look.override_auto_exposure_method = True
look.auto_exposure_method = unreal.AutoExposureMethod.AEM_HISTOGRAM
look.override_auto_exposure_apply_physical_camera_exposure = True
look.auto_exposure_apply_physical_camera_exposure = False
look.override_bloom_intensity = True
look.bloom_intensity = 0.25
if MODE == "stills":
    # A still holds for HOLD frames, about a second: at the default speed (about a stop a second)
    # the
    # exposure never settles between a sunny coast and deep jungle. Stills adapt at once instead.
    look.override_auto_exposure_speed_up = True
    look.auto_exposure_speed_up = 20.0
    look.override_auto_exposure_speed_down = True
    look.auto_exposure_speed_down = 20.0
look.override_motion_blur_amount = True
look.motion_blur_amount = 0.0
lens.post_process_settings = look

keys = []  # (frame, x, y, z, roll, pitch, yaw)
if MODE == "stills":
    shots = [
        entry.split(":")
        for entry in (
            os.environ.get("OC_STILLS") or "50,300,550,830,930,1010,1090,1300,1700,2050"
        ).split(",")
    ]
    for index, shot in enumerate(shots):
        metres = float(shot[0])
        d_yaw = float(shot[1]) if len(shot) > 1 and shot[1] else 0.0
        d_pitch = float(shot[2]) if len(shot) > 2 and shot[2] else 0.0
        lateral = float(shot[3]) if len(shot) > 3 and shot[3] else KEEP_LEFT
        eye = float(shot[4]) if len(shot) > 4 and shot[4] else EYE
        x, y, z, yaw, pitch = pose(metres * 100.0, lateral, eye)
        for frame in (index * HOLD, index * HOLD + HOLD - 1):
            keys.append((frame, x, y, z, 0.0, pitch + d_pitch, yaw + d_yaw))
    frames = len(shots) * HOLD
else:
    frames = round(SECONDS * FPS)
    for frame in range(frames + 1):
        s = PATH.wrap(FROM + SPEED * frame / FPS)
        x, y, z, yaw, pitch = pose(s)
        lift, tilt, turn = sway(frame / FPS)
        keys.append((frame, x, y, z + lift, tilt, pitch, yaw + turn))

# Yaw must not jump by 360 between keys.
unwrapped, last = [], None
for k in keys:
    yaw = k[6]
    if last is not None:
        while yaw - last > 180:
            yaw -= 360
        while yaw - last < -180:
            yaw += 360
    last = yaw
    unwrapped.append((*k[:6], yaw))

sequence = fresh("SEQ_Coast", unreal.LevelSequence, unreal.LevelSequenceFactoryNew())
sequence.set_display_rate(unreal.FrameRate(FPS, 1))
sequence.set_playback_start(0)
sequence.set_playback_end(frames)
binding = sequence.add_spawnable_from_instance(camera)
actors.destroy_actor(camera)
motion = binding.add_track(unreal.MovieScene3DTransformTrack).add_section()
motion.set_range(0, frames)
channels = motion.get_all_channels()
interp = (
    unreal.MovieSceneKeyInterpolation.CONSTANT
    if MODE == "stills"
    else unreal.MovieSceneKeyInterpolation.LINEAR
)
for frame, *values in unwrapped:
    for channel, value in zip(channels[:6], values, strict=False):
        channel.add_key(
            unreal.FrameNumber(frame), value, 0.0, unreal.MovieSceneTimeUnit.DISPLAY_RATE, interp
        )
# The weather: the sun, sky light and fog follow coast_layout.weather by where the camera is.
weather_keys = [
    (frame, values[0], values[1], layout.weather(PATH.locate(values[0], values[1])[0] / 100.0))
    for frame, *values in unwrapped
    if MODE == "stills" or frame % 12 == 0 or frame == frames
]
level_actors = {a.get_actor_label(): a for a in actors.get_all_level_actors()}


def key_float(binding, component, prop, values):
    track = binding.add_track(unreal.MovieSceneFloatTrack)
    track.set_property_name_and_path(prop, prop)
    section = track.add_section()
    section.set_range(0, frames)
    channel = section.get_all_channels()[0]
    for frame, value in values:
        channel.add_key(
            unreal.FrameNumber(frame), value, 0.0, unreal.MovieSceneTimeUnit.DISPLAY_RATE, interp
        )


def bind_component(actor, cls):
    component = actor.get_component_by_class(cls)
    try:
        return sequence.add_possessable(component)
    except Exception as exc:
        log(f"weather: cannot bind {cls}: {exc}")
        return None


azimuth = float((os.environ.get("OC_SUN") or "34,-62").split(",")[1])
if os.environ.get("OC_DRY") != "1":
    sun = level_actors.get("Sun")
    if sun:
        sun_binding = sequence.add_possessable(sun)
        turn = sun_binding.add_track(unreal.MovieScene3DTransformTrack).add_section()
        turn.set_range(0, frames)
        ch = turn.get_all_channels()
        place = sun.get_actor_location()
        for frame, _, _, w in weather_keys:
            for channel, value in zip(
                ch[:9],
                (place.x, place.y, place.z, 0.0, -w["elevation"], azimuth, 1.0, 1.0, 1.0),
                strict=False,
            ):
                channel.add_key(
                    unreal.FrameNumber(frame),
                    value,
                    0.0,
                    unreal.MovieSceneTimeUnit.DISPLAY_RATE,
                    interp,
                )
        b = bind_component(sun, unreal.DirectionalLightComponent)
        if b:
            key_float(b, None, "Intensity", [(f, w["lux"]) for f, _, _, w in weather_keys])
    sky_light = level_actors.get("SkyLight")
    if sky_light:
        b = bind_component(sky_light, unreal.SkyLightComponent)
        if b:
            key_float(b, None, "Intensity", [(f, w["sky"]) for f, _, _, w in weather_keys])
    fog = level_actors.get("Fog")
    if fog:
        b = bind_component(fog, unreal.ExponentialHeightFogComponent)
        if b:
            key_float(b, None, "FogDensity", [(f, w["fog"]) for f, _, _, w in weather_keys])
    # Exposure stays within a few stops of what the light calls for (EV100 of the sun and sky on a
    # mid-grey surface): it may darken for a bright sea, open a little in shade, but no further.
    post = level_actors.get("Post")
    if post:
        b = sequence.add_possessable(post)

        def ev(w):
            return math.log2((w["lux"] + 8000.0 * w["sky"]) / 2.5)

        for prop, offset in (
            ("AutoExposureMinBrightness", -6.5),
            ("AutoExposureMaxBrightness", 0.5),
        ):
            track = b.add_track(unreal.MovieSceneFloatTrack)
            track.set_property_name_and_path(prop, f"Settings.{prop}")
            section = track.add_section()
            section.set_range(0, frames)
            channel = section.get_all_channels()[0]
            for f, _, _, w in weather_keys:
                channel.add_key(
                    unreal.FrameNumber(f),
                    ev(w) + offset,
                    0.0,
                    unreal.MovieSceneTimeUnit.DISPLAY_RATE,
                    interp,
                )
        log(
            f"exposure: EV {min(ev(w) for *_, w in weather_keys):.1f}.."
            f"{max(ev(w) for *_, w in weather_keys):.1f}"
        )
    log(
        f"weather: {len(weather_keys)} keys, "
        f"rain {max(w['rain'] for *_, w in weather_keys):.2f} at most"
    )
    # Rain: shells round the camera, carried along with it; the material shows streaks only where
    # it rains.
    rain_mesh = (
        unreal.load_asset("/Game/Coast/Gen/Kit/V_rain/V_rain")
        if unreal.EditorAssetLibrary.does_asset_exist("/Game/Coast/Gen/Kit/V_rain/V_rain")
        else None
    )
    if rain_mesh is None:
        for path in unreal.EditorAssetLibrary.list_assets(
            "/Game/Coast/Gen/Kit/V_rain", recursive=True
        ):
            asset = unreal.load_asset(path)
            if isinstance(asset, unreal.StaticMesh):
                rain_mesh = asset
    if rain_mesh and any(w["rain"] > 0 for *_, w in weather_keys):
        shell = actors.spawn_actor_from_class(unreal.StaticMeshActor, unreal.Vector(0, 0, 0))
        shell.static_mesh_component.set_static_mesh(rain_mesh)
        shell.static_mesh_component.set_editor_property("cast_shadow", False)
        shell.static_mesh_component.set_editor_property(
            "mobility", unreal.ComponentMobility.MOVABLE
        )
        rain_binding = sequence.add_spawnable_from_instance(shell)
        actors.destroy_actor(shell)
        carry = rain_binding.add_track(unreal.MovieScene3DTransformTrack).add_section()
        carry.set_range(0, frames)
        ch = carry.get_all_channels()
        for frame, x, y, z, *_ in unwrapped:
            for channel, value in zip(
                ch[:9], (x, y, z, 0.0, 0.0, 0.0, 1.0, 1.0, 1.0), strict=False
            ):
                channel.add_key(
                    unreal.FrameNumber(frame),
                    value,
                    0.0,
                    unreal.MovieSceneTimeUnit.DISPLAY_RATE,
                    interp,
                )
        log("rain shells bound to the camera")

cut = sequence.add_track(unreal.MovieSceneCameraCutTrack).add_section()
cut.set_range(0, frames)
cut.set_camera_binding_id(sequence.get_portable_binding_id(sequence, binding))

config = fresh(
    "MRQ_Coast", unreal.MoviePipelinePrimaryConfig, unreal.MoviePipelinePrimaryConfigFactory()
)
output = config.find_or_add_setting_by_class(unreal.MoviePipelineOutputSetting)
output.output_directory = unreal.DirectoryPath(OUT)
output.output_resolution = unreal.IntPoint(WIDTH, HEIGHT)
output.file_name_format = "frame.{frame_number}"
output.override_existing_output = True
config.find_or_add_setting_by_class(unreal.MoviePipelineDeferredPassBase)
config.find_or_add_setting_by_class(unreal.MoviePipelineImageSequenceOutput_PNG)
warm = config.find_or_add_setting_by_class(unreal.MoviePipelineAntiAliasingSetting)
warm.engine_warm_up_count = 120
warm.render_warm_up_count = 64
warm.use_camera_cut_for_warm_up = False
# OC_SAMPLES renders each frame that many times and averages them, jittered: clean foliage edges and
# smooth clouds, where one TAA frame posterises them. Stills default to 8; a ride to 1 (plain TAA).
samples = int(os.environ.get("OC_SAMPLES") or ("8" if MODE == "stills" else "1"))
if samples > 1:
    warm.override_anti_aliasing = True
    warm.anti_aliasing_method = unreal.AntiAliasingMethod.AAM_NONE
    warm.spatial_sample_count = samples
    log(f"samples: {samples} per frame")
config.find_or_add_setting_by_class(unreal.MoviePipelineGameOverrideSetting)
# OC_CVARS: "name=value;name=value" console variables set for the render.
# The half-resolution, temporally reprojected cloud target posterises clouds into flat bands in MRQ.
cvars = [
    entry.split("=", 1)
    for entry in (os.environ.get("OC_CVARS") or "r.VolumetricRenderTarget=0").split(";")
    if "=" in entry
]
if cvars:
    console = config.find_or_add_setting_by_class(unreal.MoviePipelineConsoleVariableSetting)
    for name, value in cvars:
        console.add_or_update_console_variable(name.strip(), float(value))
    log(f"cvars: {cvars}")
for asset in (sequence, config):
    unreal.EditorAssetLibrary.save_loaded_asset(asset, False)
log(f"saved {MODE}: {frames} frames -> {OUT}")
