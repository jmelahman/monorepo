"""Downloads the coast ride's CC0 models (glTF) and textures from Poly Haven, and writes
assets_ph/README.md with each one's source and licence. Run with system Python; it skips
files it already has, and fetches several assets at once (OC_FETCH_THREADS, default 8).

    python3 -I scripts/fetch_assets.py [ASSET ...]
"""

import json
import os
import sys
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor

ROOT = "/workspace/ue"
MODELS = os.path.join(ROOT, "assets_ph", "models")
TEXTURES = os.path.join(ROOT, "textures")

# id -> resolution. Trees and big rocks fill the frame; small props and ground cover don't.
MODEL_IDS = {
    # jungle and verge
    "island_tree_01": "2k",
    "island_tree_02": "2k",
    "island_tree_03": "2k",
    "tree_small_02": "2k",
    "jacaranda_tree": "2k",
    "fern_02": "1k",
    "pachira_aquatica_01": "1k",
    "calathea_orbifolia_01": "1k",
    "anthurium_botany_01": "1k",
    "shrub_01": "1k",
    "shrub_02": "1k",
    "shrub_03": "1k",
    "shrub_04": "1k",
    "grass_medium_01": "1k",
    "grass_medium_02": "1k",
    "grass_bermuda_01": "1k",
    "moss_01": "1k",
    "root_cluster_01": "1k",
    "dead_tree_trunk": "1k",
    "boulder_01": "2k",
    "rock_moss_set_01": "1k",
    "nettle_plant": "1k",
    "weed_plant_02": "1k",
    "celandine_01": "1k",
    "dandelion_01": "1k",
    # coast and mountain
    "coastal_cliff_01": "2k",
    "coastal_cliff_02": "2k",
    "coastal_cliff_04": "2k",
    "coast_rocks_01": "2k",
    "coast_rocks_02": "2k",
    "coast_rocks_05": "2k",
    "coast_land_rocks_02": "2k",
    "mountainside": "2k",
    "rock_face_01": "2k",
    "rock_face_02": "2k",
    "sand_rocks_small_01": "1k",
    # town
    "modular_electricity_poles": "1k",
    "concrete_road_barrier": "1k",
    "plastic_crate_01": "1k",
    "planter_pot_clay": "1k",
    "potted_plant_01": "1k",
    "potted_plant_02": "1k",
    "wooden_bucket_01": "1k",
    "Barrel_02": "1k",
    "painted_wooden_bench": "1k",
    "trashbag": "1k",
    "wooden_crate_02": "1k",
}

TEXTURE_IDS = [
    "japanese_stone_wall",
    "cedar_planks",
    "hinoki_planks",
    "grey_roof_tiles",
    "grey_roof_tiles_02",
    "concrete_block_wall",
    "weathered_plank_siding",
    "white_plaster_rough_01",
    "worn_mossy_plasterwall",
    "rusty_metal_sheet",
    "corrugated_iron",
    "painted_metal_shutter",
    "concrete_pavement",
    "brushed_concrete",
    "brown_mud",
    "brown_mud_leaves_01",
    "coast_sand_01",
    "damp_beach_sand",
    "rock_face",
    "cliff_side",
    "mossy_rock",
    "sparse_grass",
    "grass_path_2",
    "forest_ground_05",
    "bamboo_wall",
    "wood_shutter",
    "dark_wooden_planks",
    "rocky_terrain_02",
]
TEXTURE_KINDS = ("diff", "nor_dx", "rough", "arm")
THREADS = int(os.environ.get("OC_FETCH_THREADS", "8"))


HEADERS = {"User-Agent": "veloverse-asset-fetch/1.0"}  # Poly Haven refuses urllib's default agent


def opened(url, timeout, tries=4):
    """urlopen with retries: a flaky network drops some of many parallel requests."""
    for attempt in range(tries):
        try:
            return urllib.request.urlopen(
                urllib.request.Request(url, headers=HEADERS), timeout=timeout
            )
        except OSError:  # URLError, timeouts and resets; HTTP errors other than 5xx don't recover
            if attempt == tries - 1:
                raise
            time.sleep(2**attempt)


def get_json(url):
    with opened(url, 60) as response:
        return json.load(response)


def download(url, path):
    if os.path.exists(path) and os.path.getsize(path) > 0:
        return False
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with opened(url, 600) as response, open(path + ".part", "wb") as out:
        while chunk := response.read(1 << 20):
            out.write(chunk)
    os.replace(path + ".part", path)
    return True


def fetch_model(asset, resolution):
    listing = get_json(f"https://api.polyhaven.com/files/{asset}")
    files = listing["gltf"]
    resolution = resolution if resolution in files else sorted(files)[0]
    entry = files[resolution]["gltf"]
    folder = os.path.join(MODELS, asset)
    got = download(entry["url"], os.path.join(folder, os.path.basename(entry["url"])))
    for name, include in entry.get("include", {}).items():
        got |= download(include["url"], os.path.join(folder, name))
    # glTF can't say "cut out with this separate alpha map", so its leaves arrive opaque; fetch the
    # alpha maps alongside for the builder's masked foliage material.
    for key, sizes in listing.items():
        if "alpha" not in key.lower() or not isinstance(sizes, dict):
            continue
        size = resolution if resolution in sizes else sorted(sizes)[0]
        for fmt in ("png", "jpg"):
            if fmt in sizes[size]:
                url = sizes[size][fmt]["url"]
                got |= download(url, os.path.join(folder, "textures", os.path.basename(url)))
                break
    return resolution, got


def fetch_texture(asset):
    files = get_json(f"https://api.polyhaven.com/files/{asset}")
    kinds = []
    for kind, key in (
        ("diff", "Diffuse"),
        ("nor_dx", "nor_dx"),
        ("rough", "Rough"),
        ("arm", "arm"),
    ):
        if key not in files or "2k" not in files[key]:
            continue
        entry = files[key]["2k"].get("jpg")
        if entry:
            download(entry["url"], os.path.join(TEXTURES, os.path.basename(entry["url"])))
            kinds.append(kind)
    return kinds


def model_row(asset, resolution):
    try:
        used, got = fetch_model(asset, resolution)
    except Exception as error:  # keep going; the builder skips what's missing
        print(f"model {asset} FAILED {error}", flush=True)
        return None
    print(f"model {asset} {used} {'downloaded' if got else 'cached'}", flush=True)
    return f"| {asset} | model (glTF) | {used} | https://polyhaven.com/a/{asset} |"


def texture_row(asset):
    try:
        kinds = fetch_texture(asset)
    except Exception as error:
        print(f"texture {asset} FAILED {error}", flush=True)
        return None
    print(f"texture {asset} {kinds}", flush=True)
    return f"| {asset} | texture ({', '.join(kinds)}) | 2k | https://polyhaven.com/a/{asset} |"


def main():
    only = set(sys.argv[1:])
    lines = [
        "# Poly Haven assets",
        "",
        "Downloaded by `scripts/fetch_assets.py` from https://polyhaven.com. Every asset is CC0 "
        "(public domain, https://polyhaven.com/license): "
        "no attribution needed, credited here anyway.",
        "",
        "| asset | kind | resolution | page |",
        "|---|---|---|---|",
    ]
    with ThreadPoolExecutor(THREADS) as pool:
        jobs = [pool.submit(model_row, a, r) for a, r in MODEL_IDS.items() if not only or a in only]
        jobs += [pool.submit(texture_row, a) for a in TEXTURE_IDS if not only or a in only]
        rows = [j.result() for j in jobs]  # in the lists' order, whatever order they finish in
    lines += [r for r in rows if r]
    if not only:
        with open(os.path.join(ROOT, "assets_ph", "README.md"), "w") as f:
            f.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    main()
