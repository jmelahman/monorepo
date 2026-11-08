"""Prepares the Megascans downloads in /workspace/ue/fab for import: each asset's maps shrunk to
2K and packed, and an index the importer reads.

Run: python3 -I scripts/prep_megascans.py. It needs Pillow, which it looks for in $OC_PYLIB
(default /opt/oc-pylib; pip install --target there). Assets already prepared are skipped;
OC_FORCE=1 redoes them.

Per asset, in /workspace/ue/fab_prep/<biome>/<slug>/:
- D.png: base colour, with the opacity in alpha for plants.
- N.png: the normal map, DirectX style (green flipped if the source is OpenGL style; the style is
  read off the height map rather than assumed).
- M.png: R ambient occlusion, G roughness, B height.
- T.png: the translucency, for plants.
- mesh/: the FBX files, hard-linked, alone, so Unreal's importer finds none of the 8K maps the
  FBX names and imports no textures of its own.
"""

import json
import os
import sys
from concurrent.futures import ProcessPoolExecutor
from pathlib import Path

sys.path.append(os.environ.get("OC_PYLIB", "/opt/oc-pylib"))
from PIL import Image, ImageChops

Image.MAX_IMAGE_PIXELS = None
SRC = Path("/workspace/ue/fab")
OUT = Path("/workspace/ue/fab_prep")
SIZE = 2048
FORCE = os.environ.get("OC_FORCE") == "1"
# Quixel ships one normal style for everything; OC_NORMALS=probe prints each asset's score instead.
OPENGL = os.environ.get("OC_NORMALS", "gl") == "gl"


def find(folder, kind):
    """The largest jpg of one map type, skipping the billboard set."""
    hits = [
        p
        for p in folder.glob("*.jpg")
        if p.stem.endswith(f"_{kind}") and "_Billboard_" not in p.name
    ]
    return max(hits, key=lambda p: p.stat().st_size) if hits else None


def load(path, mode):
    img = Image.open(path)
    img.draft(mode, (SIZE, SIZE))  # jpeg decodes at a fraction of 8K, much faster
    img = img.convert(mode)
    if img.size != (SIZE, SIZE):
        img = img.resize((SIZE, SIZE), Image.Resampling.LANCZOS)
    return img


def opengl_score(normal, height):
    """How much green rises where the height rises down the image, from -1 to 1. Positive means
    OpenGL style (+Y up), negative DirectX; near zero says little."""
    g = normal.getchannel("G").resize((256, 256))
    h = height.resize((256, 256))
    gs, hs = list(g.get_flattened_data()), list(h.get_flattened_data())
    total = norm_g = norm_h = 0.0
    for row in range(1, 255):
        for col in range(256):
            dh = hs[(row + 1) * 256 + col] - hs[(row - 1) * 256 + col]
            dg = gs[row * 256 + col] - 128
            total += dg * dh
            norm_g += dg * dg
            norm_h += dh * dh
    return total / max((norm_g * norm_h) ** 0.5, 1e-9)


def prep(job):
    folder, out = job
    meta_path = next(folder.glob("*.json"))
    meta = json.loads(meta_path.read_text())
    cats = meta.get("categories") or []
    kind = "plant" if "3dplant" in cats else "surface" if "surface" in cats else "prop"
    (out / "mesh").mkdir(parents=True, exist_ok=True)
    for fbx in folder.glob("*.fbx"):
        link = out / "mesh" / fbx.name
        if not link.exists():
            os.link(fbx, link)
    done = out / "asset.json"
    if done.exists() and not FORCE:
        return json.loads(done.read_text())

    colour = load(find(folder, "BaseColor"), "RGB")
    opacity = find(folder, "Opacity")
    if opacity:
        colour.putalpha(load(opacity, "L"))
    colour.save(out / "D.png")

    height_src = find(folder, "Displacement") or find(folder, "Bump")
    height = load(height_src, "L") if height_src else Image.new("L", (SIZE, SIZE), 128)
    normal = load(find(folder, "Normal"), "RGB")
    score = opengl_score(normal, height) if height_src else 0.0
    flipped = OPENGL
    if flipped:
        r, g, b = normal.split()
        normal = Image.merge("RGB", (r, ImageChops.invert(g), b))
    normal.save(out / "N.png")

    ao = find(folder, "AO")
    rough = find(folder, "Roughness")
    gloss = find(folder, "Gloss")
    if rough:
        rough_img = load(rough, "L")
    elif gloss:
        rough_img = ImageChops.invert(load(gloss, "L"))
    else:
        rough_img = Image.new("L", (SIZE, SIZE), 200)
    ao_img = load(ao, "L") if ao else Image.new("L", (SIZE, SIZE), 255)
    Image.merge("RGB", (ao_img, rough_img, height)).save(out / "M.png")

    trans = find(folder, "Translucency")
    if trans:
        load(trans, "RGB").save(out / "T.png")

    tags = meta.get("semanticTags") or {}
    params = {
        p["Name"]: p["Value"] for p in (meta.get("material_params") or {}).get("material", [])
    }
    scan = {m["key"]: m["value"] for m in meta.get("meta") or []}
    meshes = sorted(p.name for p in folder.glob("*.fbx"))
    tris = {
        Path(m["uri"]).name: m["tris"]
        for m in meta.get("models") or []
        if m.get("mimeType") == "application/x-fbx" and "tris" in m
    }
    record = {
        "biome": folder.parent.name,
        "slug": folder.name,
        "id": meta["id"],
        "name": meta.get("name", folder.name),
        "kind": kind,
        "src": str(folder),
        "meshes": [{"file": f, "tris": tris.get(f.split("_Raw_")[-1])} for f in meshes],
        "normal_flipped": flipped,
        "opengl_score": round(score, 3),
        "opacity": bool(opacity),
        "translucency": bool(trans),
        "min_size": tags.get("minSize"),
        "max_size": tags.get("maxSize"),
        "scan_area": scan.get("scanArea"),
        "translucency_tint": params.get("Translucency Tint"),
        "backside_tint": params.get("Base Color Tint Leaf Backside"),
    }
    done.write_text(json.dumps(record, indent=1))
    return record


def main():
    jobs = [
        (folder, OUT / folder.parent.name / folder.name)
        for folder in sorted(SRC.glob("*/*"))
        if folder.is_dir() and any(folder.glob("*.json")) and any(folder.glob("*BaseColor*.jpg"))
    ]
    records = []
    with ProcessPoolExecutor(max_workers=8) as pool:
        for job, result in zip(jobs, pool.map(prep, jobs), strict=True):
            records.append(result)
            print(f"{job[1].relative_to(OUT)}: {result['kind']}, score={result['opengl_score']}")
    (OUT / "index.json").write_text(json.dumps(records, indent=1))
    print(f"{len(records)} assets", file=sys.stderr)


if __name__ == "__main__":
    main()
