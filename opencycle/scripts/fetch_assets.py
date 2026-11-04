#!/usr/bin/env python3
"""Fetches the CC0 art OpenCycle renders with, from Poly Haven.

Every file is pinned by md5 in assets.lock.json, the md5s Poly Haven publishes,
and a present file with the right sum is not fetched again. To add an asset, add
it to a list below and run with --relock.

Poly Haven's trees are millions of triangles, so nothing renders them directly:
tools/bake_foliage.gd bakes them into the cards committed under assets/baked/.
They land in assets/bake-sources/, which Godot ignores, and only a bake needs them.

    scripts/fetch_assets.py                download what the game needs
    scripts/fetch_assets.py --bake-sources also download what a bake needs (1.4 GB)
    scripts/fetch_assets.py --relock       rebuild the lock from Poly Haven first
"""

import argparse
import hashlib
import json
import sys
import urllib.request
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parent.parent
LOCK = ROOT / "assets.lock.json"
DEST = ROOT / "assets" / "polyhaven"
BAKE_DEST = ROOT / "assets" / "bake-sources"
API = "https://api.polyhaven.com/files/"
# Poly Haven's API refuses the default urllib agent.
AGENT = "opencycle-asset-fetch"

BAKE_MODELS = ("fir_tree_01", "pine_tree_01")
MODELS = (
    "fern_02",
    "grass_medium_01",
    "rock_moss_set_01",
    "boulder_01",
    "tree_stump_01",
    "rock_moss_set_02",
    "grass_medium_02",
    "moss_01",
    "pine_sapling_small",
    "dry_branches_medium_01",
    "exterior_aircon_unit",
    "cardboard_box_01",
    "trashbag",
    "metal_trash_can",
    "water_manhole_cover",
    "ladder_sectioned_01",
    "utility_box_01",
    "plastic_crate_02",
    "plastic_monobloc_chair_01",
    "propane_tank",
    "modular_urban_apartments_facade",
    "modular_fire_escape",
    "steel_frame_shelves_01",
    "steel_frame_shelves_02",
    "wooden_display_shelves_01",
    "worn_metal_rack",
    "painted_wooden_shelves",
    "long_life_food",
    "russian_food_cans_01",
    "all_purpose_cleaner",
    "bleach_bottle",
    "multi_cleaner_bottle",
    "spray_paint_bottles",
    "medical_box",
    "food_lime_01",
    "food_pomegranate_01",
    "television_02",
    "Television_01",
    "CashRegister_01",
    "brass_pot_01",
    "chinese_stool",
    "small_wooden_table_01",
    "hanging_industrial_lamp",
)
TEXTURES = (
    "asphalt_02",
    "forrest_ground_03",
    "gravel_road",
    "rectangular_facade_tiles",
    "square_tiled_wall",
    "worn_plaster_wall",
    "concrete_wall_004",
    "wood_textured_concrete",
    "ribbed_concrete_wall",
    "concrete_wall_007",
    "dirty_concrete",
    "concrete_slab_wall",
    "rusty_metal_02",
    "rusty_corrugated_iron",
    "rusty_painted_metal",
    "rusted_shutter",
    "rusty_metal_shutter",
    "painted_metal_shutter",
    "worn_corrugated_iron",
    "square_concrete_pavers",
    "patterned_paving",
    "road_damaged",
)
SKIES = ("kloofendal_48d_partly_cloudy_puresky",)
TEXTURE_MAPS = ("Diffuse", "nor_gl", "Rough", "AO")


def get(url: str) -> bytes:
    request = urllib.request.Request(url, headers={"User-Agent": AGENT})
    with urllib.request.urlopen(request, timeout=120) as response:
        return response.read()


def relock() -> None:
    files: dict[str, dict[str, str]] = {}

    def add(path: str, entry: dict[str, Any]) -> None:
        files[path] = {"url": entry["url"], "md5": entry["md5"]}

    for name in MODELS + BAKE_MODELS:
        info = json.loads(get(API + name))
        gltf = info["gltf"]["1k"]["gltf"]
        add(f"{name}/{name}_1k.gltf", gltf)
        for path, entry in gltf["include"].items():
            add(f"{name}/{path}", entry)
        # A glTF's diffuse is a JPEG, so a leaf's cut-out comes only in its own map, which
        # game/plant.gdshader reads.
        for kind in info:
            if kind.lower().endswith("alpha"):
                add(f"{name}/textures/{name}_{kind.lower()}_1k.jpg", info[kind]["1k"]["jpg"])
    for name in TEXTURES:
        info = json.loads(get(API + name))
        for kind in TEXTURE_MAPS:
            if kind in info:
                add(f"{name}/{name}_{kind.lower()}_2k.jpg", info[kind]["2k"]["jpg"])
    for name in SKIES:
        add(f"{name}/{name}_2k.hdr", json.loads(get(API + name))["hdri"]["2k"]["hdr"])
    LOCK.write_text(json.dumps(files, indent=2, sort_keys=True) + "\n")


def fetch(bake_sources: bool) -> None:
    files: dict[str, dict[str, str]] = json.loads(LOCK.read_text())
    fetched = 0
    for path, entry in files.items():
        is_bake_source = path.split("/")[0] in BAKE_MODELS
        if is_bake_source and not bake_sources:
            continue
        out = (BAKE_DEST if is_bake_source else DEST) / path
        if out.exists() and hashlib.md5(out.read_bytes()).hexdigest() == entry["md5"]:
            continue
        data = get(entry["url"])
        got = hashlib.md5(data).hexdigest()
        if got != entry["md5"]:
            sys.exit(f"md5 mismatch for {entry['url']}: want {entry['md5']}, got {got}")
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_bytes(data)
        fetched += 1
    if bake_sources:
        (BAKE_DEST / ".gdignore").touch()
    print(f"assets: {fetched} fetched")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bake-sources", action="store_true")
    parser.add_argument("--relock", action="store_true")
    args = parser.parse_args()
    if args.relock:
        relock()
    fetch(args.bake_sources)


if __name__ == "__main__":
    main()
