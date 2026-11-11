"""Lists the assets under some content roots with their class, one line each prefixed ASSET.

OC_ROOTS  comma-separated content paths (default the vegetation, biome and Megascans plugins)
"""

import os

import unreal

ROOTS = (
    os.environ.get("OC_ROOTS")
    or "/ProceduralVegetationEditor,/PCGBiomeSample,/PCGBiomeCore,/MegascansPlugin"
).split(",")
registry = unreal.AssetRegistryHelpers.get_asset_registry()
registry.search_all_assets(True)
for root in ROOTS:
    assets = registry.get_assets_by_path(root, recursive=True)
    unreal.log(f"ASSET {root}: {len(assets)}")
    for data in sorted(assets, key=lambda d: str(d.package_name)):
        unreal.log(f"ASSET {data.asset_class_path.asset_name} {data.package_name}")
