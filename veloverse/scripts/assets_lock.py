"""The asset lock: every input the world is built from that git doesn't hold, pinned by hash.

    uv run -q --no-project python -I scripts/assets_lock.py write [ROOT]   # hash into the lock
    uv run -q --no-project python -I scripts/assets_lock.py check [ROOT]   # verify against it

(Python 3.11+, for tomllib.)

ROOT is the working project (default /workspace/ue). Two kinds of asset are locked:

- fab: Megascans from fab.com, one folder per listing in fab/<biome>/<slug>. Fab's Standard
  licence doesn't let them be republished, so each is fetched by hand from its listing (free, with
  an Epic account: the FBX "Raw" zip, unzipped into that folder). `check` names the listings
  missing or changed.
- polyhaven: CC0 models and textures that scripts/fetch_assets.py downloads.
- generated: textures our own make_* scripts write; `url` names the script. Locked so `check`
  notices when a generator's output drifts.
- engine: textures exported from the pinned engine's sample content; `url` names the asset.
- local: anything in textures/ not yet traced. Each should become generated or be archived
  where anyone can fetch it; the lock has none today.

Each asset is pinned by a tree digest: the SHA-256 of its files' relative paths and SHA-256s,
sorted, so any added, removed or changed file shows. Hashing runs in parallel; the first `write`
over ~50 GB takes a few minutes.
"""

import csv
import hashlib
import os
import sys
import tomllib
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

LOCK = Path(__file__).resolve().parent.parent / "assets.lock.toml"
SKIP = {".DS_Store", "Thumbs.db"}


def file_hash(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(1 << 22):
            h.update(chunk)
    return h.hexdigest()


def files_under(folder):
    out = []
    for dirpath, dirnames, filenames in os.walk(folder):
        dirnames.sort()
        for name in sorted(filenames):
            if name not in SKIP:
                out.append(Path(dirpath, name))
    return out


def tree(folder, pool):
    """(digest, file count, bytes) of everything under `folder`."""
    paths = files_under(folder)
    hashes = pool.map(file_hash, paths)
    lines = sorted(
        f"{p.relative_to(folder).as_posix()}\0{h}\n" for p, h in zip(paths, hashes, strict=True)
    )
    digest = hashlib.sha256("".join(lines).encode()).hexdigest()
    return digest, len(paths), sum(p.stat().st_size for p in paths)


def fab_assets(root):
    """The Fab manifest's rows that have a folder: (biome, slug, name, url)."""
    with open(root / "fab" / "MANIFEST.tsv", newline="") as f:
        rows = list(csv.DictReader(f, delimiter="\t"))
    out = []
    for r in rows:
        folder = root / "fab" / r["biome"] / r["folder"]
        if r["fab url"].startswith("https://") and folder.is_dir():
            out.append((r["biome"], r["folder"], r["asset name"], r["fab url"]))
    return out


def polyhaven_assets(root):
    """Poly Haven downloads: models fetch_assets.py wrote, and the textures' maps (*_<N>k.jpg)."""
    out = []
    models = root / "assets_ph" / "models"
    if models.is_dir():
        out += [p for p in sorted(models.iterdir()) if p.is_dir()]
    return out + sorted((root / "textures").glob("*_[0-9]k.jpg"))


# Where each non-download texture comes from: (glob, source, url).
ORIGINS = [
    ("bamboo_leaves_*", "generated", "scripts/make_bamboo.py"),
    ("sugi_leaves_*", "generated", "scripts/make_sugi.py"),
    ("momiji_*", "generated", "scripts/make_village.py"),
    ("rain_streaks.png", "generated", "scripts/make_village.py"),
    ("sea_waves_nor_dx.png", "generated", "scripts/make_wave.py"),
    ("urls.txt", "polyhaven", "https://polyhaven.com"),
    (
        "T_Conifer_Foliage_01_*",
        "engine",
        "/ProceduralVegetationEditor/SampleAssets/StarterContent/ConiferTree_01/Textures",
    ),
    (
        "T_LeafTree_01_Foliage_*",
        "engine",
        "/ProceduralVegetationEditor/SampleAssets/StarterContent/DeciduousTree_01/Textures",
    ),
]


def other_assets(root):
    """The rest of textures/, each as (path, source, url) from ORIGINS, or local if untraced."""
    out = []
    for p in sorted((root / "textures").iterdir()):
        if p.match("*_[0-9]k.jpg"):
            continue
        hit = next(((s, u) for g, s, u in ORIGINS if p.match(g)), ("local", ""))
        out.append((p, *hit))
    return out


def q(s):
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'


def write(root):
    entries = []
    with ThreadPoolExecutor(os.cpu_count() or 4) as pool:
        for biome, slug, name, url in fab_assets(root):
            digest, n, size = tree(root / "fab" / biome / slug, pool)
            entries.append(("fab", f"fab/{biome}/{slug}", digest, n, size, name, url))
        for p in polyhaven_assets(root):
            rel = p.relative_to(root).as_posix()
            if p.is_dir():
                digest, n, size = tree(p, pool)
            else:
                digest, n, size = file_hash(p), 1, p.stat().st_size
            entries.append(("polyhaven", rel, digest, n, size, p.stem, "https://polyhaven.com"))
        for p, source, url in other_assets(root):
            rel = p.relative_to(root).as_posix()
            entries.append((source, rel, file_hash(p), 1, p.stat().st_size, p.stem, url))
    total = sum(e[4] for e in entries)
    lines = [
        "# Generated by scripts/assets_lock.py write; verify with `check`. Do not edit by hand.",
        f"# {len(entries)} assets, {total / 1e9:.1f} GB.",
        "",
    ]
    for source, path, digest, n, size, name, url in entries:
        lines += [
            "[[asset]]",
            f"source = {q(source)}",
            f"path = {q(path)}",
            f"name = {q(name)}",
            f"url = {q(url)}",
            f"files = {n}",
            f"bytes = {size}",
            f"sha256 = {q(digest)}",
            "",
        ]
    LOCK.write_text("\n".join(lines))
    print(f"{LOCK}: {len(entries)} assets, {total / 1e9:.1f} GB")


def check(root):
    assets = tomllib.loads(LOCK.read_text())["asset"]
    bad = []
    with ThreadPoolExecutor(os.cpu_count() or 4) as pool:
        for a in assets:
            p = root / a["path"]
            if not p.exists():
                bad.append(("missing", a))
                continue
            digest = tree(p, pool)[0] if p.is_dir() else file_hash(p)
            if digest != a["sha256"]:
                bad.append(("changed", a))
    for why, a in bad:
        print(f"{why}\t{a['path']}\t{a['name']}\t{a['url']}")
    print(f"{len(assets) - len(bad)}/{len(assets)} assets match the lock")
    return 1 if bad else 0


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in ("write", "check"):
        sys.exit(__doc__)
    root = Path(sys.argv[2] if len(sys.argv) > 2 else "/workspace/ue")
    if sys.argv[1] == "write":
        write(root)
        return 0
    return check(root)


if __name__ == "__main__":
    sys.exit(main())
