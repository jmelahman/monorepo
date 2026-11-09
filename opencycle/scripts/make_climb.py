"""The climb's hero stretch: a kilometre of mountain road on a wooded flank, 2 km of ground at a
metre a sample. The road traverses up the slope at about 6.5%, turns on one hairpin and comes back
above itself, with cut banks above it and fill below. Beech woods give way to spruce as it climbs.

Run with system Python: python3 -I scripts/make_climb.py. It borrows make_island.py's noise,
road grading and carving, and writes the same files island_world.py reads, to
/workspace/ue/world_gen/climb:
- height.r16, <Layer>.r8, info.json, layers.png, preview.png: the ground and its paint;
- road_hero_<k>.gltf: the tarmac; routes.json: the ride, up and back down;
- trees.json: [kind, x, y, z, yaw, scale] in cm, kind "beech" or "spruce";
- rails.json: guard-rail runs on the drop side, each a list of [x, y, z] in cm.

Coordinates are Unreal's: X east, Y south, metres, the origin in the middle. The slope rises to the
north (-Y); the valley floor is to the south.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_island as mi
import road_mesh

np, Image = mi.np, mi.Image

RES = 2033  # 16 landscape components of 127 quads, plus one
SPACING = 1.0
OUT = "/workspace/ue/world_gen/climb"
SEED = 31
VALLEY_Y = 640.0  # metres south of the origin: the valley floor's line
FLANK = 0.34  # the mountainside's rise per metre north of it
# The road's plan in km: a traverse east, a hairpin turning north, and back west above it.
PLAN = [
    (-0.540, 0.452),
    (-0.460, 0.440),
    (-0.330, 0.418),
    (-0.200, 0.392),
    (-0.080, 0.372),
    (0.010, 0.352),
    (0.046, 0.341),
    (0.062, 0.329),
    (0.062, 0.314),
    (0.048, 0.303),
    (0.022, 0.300),
    (-0.060, 0.292),
    (-0.180, 0.268),
    (-0.300, 0.250),
    (-0.420, 0.226),
    (-0.520, 0.210),
]
# Make make_island's helpers work on this grid and this road.
mi.RES, mi.SPACING = RES, SPACING
mi.ROAD_HALF = 3.0
mi.SHOULDER = 0.8
mi.BANK_SLOPE = 0.95  # cut and fill banks, steeper than the island's: this is a mountain road
mi.CORRIDOR = 45.0
mi.MAX_GRADE = 0.085


def terrain(x, y):
    """The flank: a valley floor in the south rising north at FLANK, with spurs and gullies down
    it, a far side of the valley, and metre-scale bumps."""
    up = VALLEY_Y - y
    # Soft knee at the valley floor; the far side rises again south of it.
    base = FLANK * (np.sqrt(up * up + 60.0**2) + up) / 2
    base += 0.28 * np.clip(-up - 120, 0, None)
    # Spurs and gullies run down the slope: stretched along Y, varying across X.
    gully = (mi.fbm(x * 1.0, y * 0.35, 160, 4, SEED) - 0.5) * 34
    swell = (mi.fbm(x, y, 420, 3, SEED + 7) - 0.5) * 40
    crag = (mi.ridged(x, y, 90, 4, SEED + 13) - 0.35) * 9 * mi.smoothstep(100, 400, up)
    bumps = (mi.fbm(x, y, 14, 3, SEED + 21) - 0.5) * 1.6
    return (base + gully + swell + crag + bumps).astype(np.float32)


def at(xy):
    """Grid row and column of points in metres."""
    return (
        np.round(xy[:, 1] / SPACING + (RES - 1) / 2).astype(int),
        np.round(xy[:, 0] / SPACING + (RES - 1) / 2).astype(int),
    )


def lay_road(h):
    xy = mi.catmull(PLAN, False)
    z = np.maximum(mi.smooth(mi.sample(h, xy), 40, False), 1.0)
    z = mi.grade_fit(z, {}, False)
    return {"hero": {"xy": xy, "z": z, "closed": False}}


def paint(x, y, h, road_dist):
    gy, gx = np.gradient(h, SPACING)
    slope = np.hypot(gx, gy)
    rock = mi.smoothstep(0.78, 1.05, slope)
    shoulder = mi.smoothstep(mi.ROAD_HALF + 2.2, mi.ROAD_HALF + 0.6, road_dist)
    # Grass: the valley floor, a verge a few metres wide, and clearings up the flank.
    floor = mi.smoothstep(VALLEY_Y - 140, VALLEY_Y - 40, y)
    verge = mi.smoothstep(mi.ROAD_HALF + 9, mi.ROAD_HALF + 4, road_dist + 4 * mi.fbm(x, y, 9, 2))
    clearing = mi.smoothstep(0.66, 0.74, mi.fbm(x, y, 120, 3, SEED + 40))
    grass = np.clip(np.maximum.reduce([floor, verge, clearing]), 0, 1)
    soft = (1 - rock) * (1 - shoulder)
    out = {k: np.zeros_like(h) for k in mi.LAYERS}
    out["Grass"] = soft * grass
    out["Forest"] = soft * (1 - grass)
    out["Rock"] = rock * (1 - shoulder)
    out["Dirt"] = shoulder
    return out, slope, grass


def trees(h, road_dist, slope, grass):
    """Stems on a jittered grid about 5.5 m apart in the woods, thinning to nothing in clearings
    and on crags, kept off the verge. Beech below, spruce above, mixed across a band with noise."""
    rng = np.random.default_rng(SEED + 100)
    step = 5.5
    reach = 320.0  # metres from the road
    c = np.arange(-(RES - 1) / 2 + step / 2, (RES - 1) / 2, step)
    gx, gy = np.meshgrid(c, c)
    px = (gx + rng.uniform(-0.45, 0.45, gx.shape) * step).ravel()
    py = (gy + rng.uniform(-0.45, 0.45, gy.shape) * step).ravel()
    row, col = at(np.stack([px, py], 1))
    ok = (row >= 0) & (row < RES) & (col >= 0) & (col < RES)
    px, py, row, col = px[ok], py[ok], row[ok], col[ok]
    d = road_dist[row, col]
    wood = 1 - grass[row, col]
    keep = (
        (d > mi.ROAD_HALF + 5.5 + rng.uniform(0, 4, d.shape))
        & (d < reach)
        & (slope[row, col] < 1.0)
        & (rng.random(d.shape) < wood * 0.92)
    )
    px, py, row, col = px[keep], py[keep], row[keep], col[keep]
    z = h[row, col]
    # The change from beech to spruce, in metres of height, wandering with noise.
    line = 95 + 30 * (mi.fbm(px, py, 150, 3, SEED + 60) - 0.5)
    spruce = rng.random(z.shape) < mi.smoothstep(line - 25, line + 25, z)
    out = []
    for xm, ym, zm, s in zip(px, py, z, spruce, strict=True):
        roll = rng.random()
        size = (
            rng.uniform(1.25, 1.5)
            if roll < 0.08
            else rng.uniform(0.55, 0.8)
            if roll < 0.35
            else rng.uniform(0.85, 1.2)
        )
        out.append(
            [
                "spruce" if s else "beech",
                round(float(xm) * 100, 1),
                round(float(ym) * 100, 1),
                round(float(zm) * 100 - 15, 1),
                round(float(rng.uniform(0, 360)), 1),
                round(float(size), 3),
            ]
        )
    return out


def rails(h, road):
    """Guard rail on the outside of the road wherever the ground falls away more than 1.5 m within
    6 m of its edge: the valley side of each traverse and round the hairpin."""
    xy, z = road["xy"], road["z"]
    d = np.gradient(xy, axis=0)
    d /= np.linalg.norm(d, axis=1, keepdims=True)
    runs, run = [], []
    for k in range(len(xy)):
        best = None
        for side in (-1, 1):
            normal = np.array([-d[k, 1], d[k, 0]]) * side
            p = xy[k] + normal * (mi.ROAD_HALF + 0.6)
            probe = xy[k] + normal * (mi.ROAD_HALF + 6)
            drop = z[k] - mi.sample(h, probe[None])[0]
            if drop > 1.5 and (best is None or drop > best[0]):
                best = (drop, p)
        if best is not None:
            p = best[1]
            run.append(
                [
                    round(float(p[0]) * 100, 1),
                    round(float(p[1]) * 100, 1),
                    round(float(z[k]) * 100, 1),
                ]
            )
        elif run:
            if len(run) > 12:
                runs.append(run)
            run = []
    if len(run) > 12:
        runs.append(run)
    return runs


def preview(h, layers, road, tree_list):
    gy, gx = np.gradient(h, SPACING)
    n = np.stack([-gx, -gy, np.ones_like(h)], -1)
    n /= np.linalg.norm(n, axis=-1, keepdims=True)
    light = np.array([-0.5, -0.6, 0.62])
    shade = np.clip(n @ (light / np.linalg.norm(light)), 0.2, 1)
    names = list(layers)
    colours = np.array([mi.LAYERS[k] for k in names], dtype=np.float32)
    top = np.argmax(np.stack([layers[k] for k in names]), 0)
    img = (colours[top] * shade[..., None]).astype(np.uint8)
    for kind, x, y, *_ in tree_list:
        r, c = int(y / 100 + (RES - 1) / 2), int(x / 100 + (RES - 1) / 2)
        img[r, c] = (20, 60, 25) if kind == "spruce" else (90, 120, 40)
    row, col = at(road["xy"])
    img[row, col] = (40, 40, 40)
    Image.fromarray(img).save(f"{OUT}/preview.png")


def main():
    os.makedirs(OUT, exist_ok=True)
    c = (np.arange(RES, dtype=np.float32) - (RES - 1) / 2) * SPACING
    x, y = np.meshgrid(c, c)
    h = terrain(x, y)
    roads = lay_road(h)
    road = roads["hero"]
    h, road_dist = mi.carve(h, roads)
    layers, slope, grass = paint(x, y, h, road_dist)
    tree_list = trees(h, road_dist, slope, grass)
    rail_runs = rails(h, road)
    mi.OUT = OUT
    mi.write_maps(h, np.ones_like(h, dtype=bool), layers)
    preview(h, layers, road, tree_list)
    xy, z = road["xy"], road["z"]
    centre = [(a * 100, b * 100, c * 100 + 2) for (a, b), c in zip(xy, z, strict=True)]
    pieces = road_mesh.road_pieces(centre, mi.ROAD_HALF * 100, False)
    for k, piece in enumerate(pieces):
        piece.write(f"{OUT}/road_hero_{k}.gltf")
    line = np.column_stack([xy, z])
    ride = np.concatenate([line, line[::-1][1:]])  # up and back down
    with open(f"{OUT}/routes.json", "w") as f:
        json.dump({"island": ride.round(2).tolist()}, f)
    with open(f"{OUT}/trees.json", "w") as f:
        json.dump(tree_list, f)
    with open(f"{OUT}/rails.json", "w") as f:
        json.dump(rail_runs, f)
    length = float(np.linalg.norm(np.diff(xy, axis=0), axis=1).sum())
    grade = np.diff(z) / np.linalg.norm(np.diff(xy, axis=0), axis=1)
    spruce = sum(t[0] == "spruce" for t in tree_list)
    print(
        f"{OUT}: ground {h.min():.0f}..{h.max():.0f} m; road {length:.0f} m from {z[0]:.0f} to"
        f" {z[-1]:.0f} m, mean {(z[-1] - z[0]) / length:.1%}, steepest {grade.max():.1%};"
        f" {len(pieces)} road pieces; trees {len(tree_list)} ({spruce} spruce);"
        f" rails {len(rail_runs)} runs, {sum(map(len, rail_runs))} m"
    )


if __name__ == "__main__":
    main()
