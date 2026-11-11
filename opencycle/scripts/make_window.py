"""Cut a 2 km window out of the world plan: the prototype join between the plan
(`make_world.py`) and the stretch pipeline (`make_climb.py`).

Given a biome named in world.json's `windows`, this cuts a RES x RES window at 1 m a sample
(2032 m square) centred on that biome's point:

- heights: the plan's 28 m heights upsampled bicubically, plus detail noise at the scales
  `make_climb.terrain` uses (gullies about 160 m, a swell of 420 m, crags of 90 m on steep
  or mountain ground, metre-scale bumps). The noise is seeded in world metres, so two windows
  that overlap agree.
- the road: the plan road under the window's centre (from roads.json, every 10 m), the run
  of it inside the window, as a centripetal Catmull-Rom every metre. Its heights are the new
  ground along it, smoothed and regraded with `make_island.grade_fit` (under 8%), with both
  ends pinned to the plan road's own height there, so the stretch meets the rest of the road.

Run: uv run -q --no-project --with numpy --with scipy --with pillow python -I
scripts/make_window.py <biome> (one of world.json's windows: mountain, desert, forest, meadow,
town, heath, coast). Run make_world.py first. Writes to /workspace/ue/world_gen/window/<biome>:
- preview.png: shaded relief with the road drawn (red) and a 250 m grid;
- window.json: the biome, the ride and plan road it came from, the centre (world m), the road
  polyline every metre as [x, y, z] in window metres (origin at the centre, X east, Y south,
  z in world metres above sea level), the ground's min and max height, the road's length,
  climb and grades;
- height.npy: the window's heights, float32, RES x RES, rows along Y.

It does not write to world_gen/climb or touch Unreal. What remains to plug it into
make_climb.py:
1. In make_climb: when a window is given (say OC_WINDOW=<biome>), load height.npy instead of
   calling `terrain()`, drop the rim of ridges (the real neighbours are the horizon now; a
   distant-terrain mesh or a lower-resolution ring of the plan would replace it), and take
   PLAN / `lay_road` from window.json's road instead of the hand-drawn hairpin.
2. Heights: make_climb's world is relative (the valley floor near 0). Either subtract the
   window's min height, recording the offset in info.json, or keep sea level at z 0 so a
   coast window's sea plane (`climb_world.sea`, `sea_cm`) sits at the real shore.
3. Road carving: run `open_valley_side` and `roughen_banks` on the window as on the flank;
   the plan road's cut and fill (world.json `roads`) say where to expect deep cuts.
4. Junctions: a window that holds a junction needs both roads; this takes only the one under
   the centre.
5. Paint, trees and props then follow the biome preset (OC_BIOME) as now; treeline and
   snowline are absolute heights in SHAPES, so check them against the window's real heights.
"""

import json
import os
import sys

import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_island as mi

WORLD = "/workspace/ue/world_gen/world"
OUT = "/workspace/ue/world_gen/window"
RES = 2033  # make_climb's grid: 16 landscape components of 127 quads, plus one
SPACING = 1.0
HALF = (RES - 1) / 2 * SPACING
SEED = 31
EDGE = 20.0  # m: the road is cut this far inside the window's edge


def plan_heights(world, h, x, y):
    """The plan's heights at world points x, y (m), bicubic."""
    spacing, res = world["spacing_m"], world["res"]
    col = x / spacing + (res - 1) / 2
    row = y / spacing + (res - 1) / 2
    return ndimage.map_coordinates(h, [row, col], order=3, mode="nearest").astype(np.float32)


def detail(x, y, base):
    """make_climb's detail noise at world points x, y (m), scaled to the ground: crags only
    where it is steep or high, nothing below the sea."""
    gy, gx = np.gradient(base, SPACING)
    steep = mi.smoothstep(0.15, 0.5, np.hypot(gx, gy)) + mi.smoothstep(850, 1050, base)
    steep = np.clip(steep, 0, 1)
    gully = (mi.fbm(x * 1.0, y * 0.35, 160, 4, SEED) - 0.5) * 12
    swell = (mi.fbm(x, y, 420, 3, SEED + 7) - 0.5) * 16
    crag = (mi.ridged(x, y, 90, 4, SEED + 13) - 0.35) * 7 * steep
    bumps = (mi.fbm(x, y, 14, 3, SEED + 21) - 0.5) * 1.6
    land = mi.smoothstep(0.0, 4.0, base)
    return (gully + swell + crag + bumps) * land


def road_in_window(roads, centre):
    """The plan road nearest the centre and the run of it inside the window round its nearest
    point: (road name, points [x, y, z] in world m)."""
    best = None
    for name, pts in roads.items():
        pts = np.asarray(pts)
        d = np.linalg.norm(pts[:, :2] - centre, axis=1)
        k = int(np.argmin(d))
        if best is None or d[k] < best[0]:
            best = (d[k], name, pts, k)
    _, name, pts, k = best
    inside = np.all(np.abs(pts[:, :2] - centre) < HALF - EDGE, axis=1)
    a = b = k
    while a > 0 and inside[a - 1]:
        a -= 1
    while b < len(pts) - 1 and inside[b + 1]:
        b += 1
    return name, pts[a : b + 1]


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__.split("\n\n")[3])
    biome = sys.argv[1]
    with open(f"{WORLD}/world.json") as f:
        world = json.load(f)
    if biome not in world["windows"]:
        sys.exit(f"no window {biome!r}; one of {', '.join(world['windows'])}")
    with open(f"{WORLD}/roads.json") as f:
        roads = json.load(f)
    plan = np.load(f"{WORLD}/height.npy")
    win = world["windows"][biome]
    centre = np.array(win["centre"][:2], dtype=np.float64)

    # The ground: the plan upsampled, plus detail. World coordinates for the noise.
    c = (np.arange(RES, dtype=np.float64) - (RES - 1) / 2) * SPACING
    lx, ly = np.meshgrid(c, c)
    wx, wy = lx + centre[0], ly + centre[1]
    base = plan_heights(world, plan, wx, wy)
    h = (base + detail(wx, wy, base)).astype(np.float32)
    del plan, wx, wy

    # The road: the plan's run inside the window, every metre, regraded on the new ground.
    road_name, run = road_in_window(roads, centre)
    if len(run) < 3:
        sys.exit(f"the plan road {road_name} has no run inside the window")
    local = (run[:, :2] - centre) / 1000  # catmull takes km
    xy = mi.catmull([tuple(p) for p in local], False, step=1.0)
    if np.linalg.norm(xy[-1] - local[-1] * 1000) < 0.5:
        xy = xy[:-1]  # no sliver of a last metre: its grade would be noise
    xy = np.vstack([xy, local[-1] * 1000])
    col = xy[:, 0] / SPACING + (RES - 1) / 2
    row = xy[:, 1] / SPACING + (RES - 1) / 2
    ground = ndimage.map_coordinates(h, [row, col], order=1, mode="nearest")
    z = np.maximum(mi.smooth(ground, 20, False), 1.0)
    z = mi.grade_fit(z, {0: float(run[0, 2]), len(z) - 1: float(run[-1, 2])}, False)
    seg = np.linalg.norm(np.diff(xy, axis=0), axis=1)
    grade = np.diff(z) / np.maximum(seg, 1e-6)

    out = f"{OUT}/{biome}"
    os.makedirs(out, exist_ok=True)
    np.save(f"{out}/height.npy", h)
    info = {
        "biome": biome,
        "ride": win["ride"],
        "plan_road": road_name,
        "centre_world_m": [round(float(v)) for v in centre],
        "res": RES,
        "spacing_m": SPACING,
        "height_min_m": round(float(h.min()), 1),
        "height_max_m": round(float(h.max()), 1),
        "road_km": round(float(seg.sum()) / 1000, 3),
        "road_climb_m": round(float(np.clip(np.diff(z), 0, None).sum()), 1),
        "road_steepest": round(float(np.abs(grade).max()), 3),
        "road_mean_grade": round(float(np.abs(grade).mean()), 3),
        "road_cut_fill_max_m": round(float(np.abs(z - ground).max()), 1),
        "road": np.column_stack([xy, z]).round(2).tolist(),
    }
    with open(f"{out}/window.json", "w") as f:
        json.dump(info, f)
    preview(h, xy, f"{out}/preview.png")
    print(
        f"{biome}: road {road_name} ({win['ride']} ride), {info['road_km']} km in the window,"
        f" climbs {info['road_climb_m']} m, steepest {info['road_steepest']},"
        f" cut/fill up to {info['road_cut_fill_max_m']} m;"
        f" ground {info['height_min_m']}..{info['height_max_m']} m -> {out}"
    )


def preview(h, xy, path):
    gy, gx = np.gradient(h, SPACING)
    light = np.array([-0.6, -0.6, 0.55])
    light /= np.linalg.norm(light)
    n = np.stack([-gx, -gy, np.ones_like(h)])
    shade = np.clip((n * light[:, None, None]).sum(0) / np.linalg.norm(n, axis=0), 0, 1)
    tint = np.where(h[..., None] < 0.5, [60, 90, 130], [150, 150, 140]).astype(np.float32)
    rgb = tint * (0.3 + 0.9 * shade)[..., None]
    lines = (np.arange(RES) % 250) == 0
    rgb[lines] = rgb[lines] * 0.7 + 30
    rgb[:, lines] = rgb[:, lines] * 0.7 + 30
    image = Image.fromarray(np.clip(rgb, 0, 255).astype(np.uint8))
    draw = ImageDraw.Draw(image)
    mid = (RES - 1) / 2
    pts = [(float(p[0] / SPACING + mid), float(p[1] / SPACING + mid)) for p in xy]
    draw.line(pts, fill=(220, 30, 30), width=5)
    image.resize((1024, 1024), Image.LANCZOS).save(path)


if __name__ == "__main__":
    main()
