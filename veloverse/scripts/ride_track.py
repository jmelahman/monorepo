"""Writes /Game/Coast/Ride/DA_RideTrack, the playable ride's track, from coast_layout's path.

The rider (VeloverseUE's ARidePawn) reads the centreline, its sample spacing, whether it loops,
the weather along it and the rain shells from this asset, so the game rides the same road, light
and rain the stills do. Run after make_coast.py, whenever the path or the weather changes.
"""

import importlib.util
import os

import unreal


def sibling(name):
    here = os.path.dirname(
        os.path.abspath(globals().get("__file__") or "/workspace/ue/scripts/ride_track.py")
    )
    spec = importlib.util.spec_from_file_location(name, os.path.join(here, f"{name}.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


layout = sibling("coast_layout")
FOLDER = "/Game/Coast/Ride"
NAME = "DA_RideTrack"
WEATHER_STEP = 10.0  # metres between weather rows; the rider eases between them
SUN_AZIMUTH = float((os.environ.get("OC_SUN") or "34,-62").split(",")[1])


def log(message):
    unreal.log(f"TRACK {message}")


track_class = unreal.load_class(None, "/Script/VeloverseUE.RideTrack")
if track_class is None:
    raise RuntimeError("no RideTrack class: build the VeloverseUE module first")

path = layout.CoastPath()
asset_path = f"{FOLDER}/{NAME}"
if unreal.EditorAssetLibrary.does_asset_exist(asset_path):
    track = unreal.load_asset(asset_path)
else:
    factory = unreal.DataAssetFactory()
    factory.set_editor_property("data_asset_class", track_class)
    track = unreal.AssetToolsHelpers.get_asset_tools().create_asset(
        NAME, FOLDER, track_class, factory
    )

centre = [unreal.Vector(x, y, z) for x, y, z in zip(path.xs, path.ys, path.zs, strict=True)]
track.set_editor_property("centre", centre)
track.set_editor_property("step", path.STEP)
track.set_editor_property("closed", path.closed)
track.set_editor_property("keep_left", 45.0)
track.set_editor_property("sun_azimuth", SUN_AZIMUTH)

metres = path.length / 100.0
rows = []
m = 0.0
while m <= metres + WEATHER_STEP:
    w = layout.weather(min(m, metres))
    row = unreal.RideWeather()
    row.set_editor_property("metres", m)
    for key in ("lux", "elevation", "sky", "fog", "rain"):
        row.set_editor_property(key, w[key])
    row.set_editor_property("wet", layout.wetness(min(m, metres)))
    rows.append(row)
    m += WEATHER_STEP
track.set_editor_property("weather", rows)

# The stretches, for the HUD: consecutive zones of one name are one stretch.
stretches = []
for name, start in layout.ZONES:
    if start < metres and not (
        stretches and stretches[-1].get_editor_property("name") == name.title()
    ):
        row = unreal.RideStretch()
        row.set_editor_property("name", name.title())
        row.set_editor_property("metres", float(start))
        stretches.append(row)
track.set_editor_property("stretches", stretches)

rain = None
for asset in unreal.EditorAssetLibrary.list_assets("/Game/Coast/Gen/Kit/V_rain", recursive=True):
    loaded = unreal.load_asset(asset)
    if isinstance(loaded, unreal.StaticMesh):
        rain = loaded
track.set_editor_property("rain_shell", rain)
mpc = "/Game/Coast/Materials/MPC_Ride"
if unreal.EditorAssetLibrary.does_asset_exist(mpc):
    track.set_editor_property("collection", unreal.load_asset(mpc))

unreal.EditorAssetLibrary.save_loaded_asset(track, only_if_is_dirty=False)
log(
    f"{len(centre)} samples, {metres / 1000:.2f} km, closed={path.closed}, "
    f"{len(rows)} weather rows, {len(stretches)} stretches, rain shell {'yes' if rain else 'none'}"
)
