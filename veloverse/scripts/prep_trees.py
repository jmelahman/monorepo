"""Prepares the Fab tree packs in /workspace/ue/fab/climb for import_trees.py: each species' mesh
and its bark and leaf textures in /workspace/ue/fab_prep/trees/<kind>/, with an index.json.

    uv run -q --no-project --with pillow python -I scripts/prep_trees.py

Per species: bark_D.png and leaf_D.png (colour; the leaf's alpha is its cut-out, packed from the
pack's separate opacity map where it has one), bark_N.png and leaf_N.png (DirectX normals, the
green flipped where the pack's are OpenGL style), all at most 2048 px. The FBX is copied alone, so
Interchange doesn't import a pack's loose textures beside it.

The packs, their licences and why these five are in /workspace/ue/fab/climb/SUMMARY.md.
"""

import json
import os
import shutil

from PIL import Image, ImageOps

Image.MAX_IMAGE_PIXELS = None
SRC = "/workspace/ue/fab/climb"
OUT = "/workspace/ue/fab_prep/trees"
SIZE = 2048
PINE = "conifer_standin_01/source/PineTree/Textures"

# kind: the pack's FBX; each material slot's part ("bark" or "leaf"); each part's colour, normal
# (None for flat), whether that normal is OpenGL style, and the leaf's cut-out (a file and its
# channel, or "A" for the colour's own alpha); `keep` names the meshes to place (the rest, the
# packs' own lower LODs, are deleted after import: Nanite makes its own).
SPECIES = {
    "beech": {
        "fbx": "european_beech_01/FBX/Beech Tree.fbx",
        "slots": {"_Beech_Leaves_": "leaf", "_Beech_Trunk_": "bark"},
        "bark": (
            "european_beech_01/Textures/_Beech_2K/_Beech_Trunk_/_Beech_Trunk_Base_color.png",
            "european_beech_01/Textures/_Beech_2K/_Beech_Trunk_/_Beech_Trunk_Normal.png",
            False,
        ),
        "leaf": (
            "european_beech_01/Textures/_Beech_2K/_Beech_Leaves_/_Beech_Leaves_Base_color.png",
            "european_beech_01/Textures/_Beech_2K/_Beech_Leaves_/_Beech_Leaves_Normal.png",
            False,
            ("european_beech_01/Textures/_Beech_2K/_Beech_Leaves_/_Beech_Leaves_Opacity.png", "L"),
        ),
    },
    "birch": {
        "fbx": "birch_pack_lods/source/Birch_trees_pack.fbx",
        "slots": {"Birch_Atlas": "leaf", "Birch_bark": "bark"},
        "bark": (
            "birch_pack_lods/textures/Birtch_main_Color.png",
            "birch_pack_lods/textures/Birtch_main_Normal.png",
            True,
        ),
        "leaf": (
            "birch_pack_lods/textures/Atlas_Color.png",
            "birch_pack_lods/textures/Atlas_Normal.png",
            True,
            "A",
        ),
        "drop": "LOD",
    },
    "pine": {
        "fbx": "conifer_standin_01/source/PineTree/Source/Pine Tree G.fbx",
        "slots": {"Pine_Texture_2_Mat": "leaf", "pine_bark_diff_4k_Mat": "bark"},
        "bark": (
            "conifer_standin_01/source/PineTree/Textures/Bark Base Colour.jpg",
            "conifer_standin_01/source/PineTree/Textures/Bark Normal Map (GL).jpg",
            True,
        ),
        "leaf": (
            f"{PINE}/Leaf Base Colour (set Roughness at 0.75).png",
            None,
            False,
            ("conifer_standin_01/source/PineTree/Textures/Opacity Pine 2.png", "R"),
        ),
    },
    "windpine": {
        "fbx": "windswept_pine_01/source/GrafTree2HIGH.fbx",
        "slots": {"Needles": "leaf", "GrafTree2TrunkHIGH": "bark", "GrafTree2BranchesHIGH": "twig"},
        "bark": (
            "windswept_pine_01/source/Retopo_graftree2.001_Bake1_PBR StoA_Diffuse.png",
            None,
            False,
        ),
        "twig": ("windswept_pine_01/source/graf2branches1k_upscayl_4x_ultrasharp.jpg", None, False),
        "leaf": ("windswept_pine_01/textures/Needles.png", None, False, "A"),
    },
    "snag": {
        "fbx": "dead_broadleaf_snag_01/mid/Dead_Tree_qlEtl_Mid.fbx",
        "slots": {"MatID_1": "bark"},
        "bark": (
            "dead_broadleaf_snag_01/mid/Dead_Tree_qlEtl_Mid_2K_BaseColor.jpg",
            "dead_broadleaf_snag_01/mid/Dead_Tree_qlEtl_Mid_2K_Normal.jpg",
            True,
        ),
    },
}


def fit(im):
    if max(im.size) > SIZE:
        im = im.resize((SIZE, SIZE * im.size[1] // im.size[0]), Image.LANCZOS)
    return im


def normal(path, opengl, out):
    im = fit(Image.open(f"{SRC}/{path}").convert("RGB"))
    if opengl:
        r, g, b = im.split()
        im = Image.merge("RGB", (r, ImageOps.invert(g), b))
    im.save(out)


def colour(path, cut, out):
    im = Image.open(f"{SRC}/{path}")
    if cut is None:
        fit(im.convert("RGB")).save(out)
        return
    if cut == "A":
        im = fit(im.convert("RGBA"))
    else:
        file, channel = cut
        mask = Image.open(f"{SRC}/{file}")
        mask = mask.convert("L") if channel == "L" else mask.convert("RGB").getchannel(channel)
        im = fit(im.convert("RGB"))
        im.putalpha(mask.resize(im.size, Image.LANCZOS))
    im.save(out)


def main():
    index = []
    for kind, spec in SPECIES.items():
        out = f"{OUT}/{kind}"
        os.makedirs(out, exist_ok=True)
        shutil.copyfile(f"{SRC}/{spec['fbx']}", f"{out}/{kind}.fbx")
        parts = sorted(set(spec["slots"].values()))
        for part in parts:
            col, nor, opengl, *cut = spec[part]
            colour(col, cut[0] if cut else None, f"{out}/{part}_D.png")
            if nor:
                normal(nor, opengl, f"{out}/{part}_N.png")
        index.append(
            {
                "kind": kind,
                "fbx": f"{out}/{kind}.fbx",
                "slots": spec["slots"],
                "parts": parts,
                "normals": [p for p in parts if spec[p][1]],
                "drop": spec.get("drop"),
            }
        )
        print(f"{kind}: {', '.join(parts)}", flush=True)
    with open(f"{OUT}/index.json", "w") as f:
        json.dump(index, f, indent=1)


if __name__ == "__main__":
    main()
