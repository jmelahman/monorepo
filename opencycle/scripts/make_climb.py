"""The climb's hero stretch: a kilometre of bike path on a wooded flank, 2 km of ground at a
metre a sample. The road traverses up the slope at about 6.5%, turns on one hairpin and comes back
above itself, with cut banks above it and fill below. Beech woods give way to spruce as it climbs.

Run with system Python: python3 -I scripts/make_climb.py. It borrows make_island.py's noise,
road grading and carving, and writes the same files island_world.py reads, to
/workspace/ue/world_gen/climb:
- height.r16, <Layer>.r8, info.json, layers.png, preview.png: the ground and its paint;
- road_hero_<k>.gltf: the tarmac; routes.json: the ride, up and back down;
- trees.json: [kind, x, y, z, yaw, scale] in cm, kind "beech" or "spruce";
- rails.json: guard-rail runs on the drop side, each a list of [x, y, z] in cm;
- props.json: [kind, x, y, z, yaw], a bench at the hairpin and woodpiles by the path.

Coordinates are Unreal's: X east, Y south, metres, the origin in the middle.
The road is a bike-only path, 3 m wide. The slope rises to the
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
# The backdrop is alpine, as in the style shots: the woods thin out at the treeline into meadow,
# and snow lies on the peaks. Metres of height, wandering with noise.
# v35: 130 (was 420, far above the path), so the top of the hero stretch rides out of the
# spruce into open heath, as in ref/climb/alpine_meadow_trail.jpg. The full climb will stretch
# the zones back out.
TREELINE = 130.0
SNOWLINE = 600.0
# Peaks where the path's views end, east and west: (x, y, height, radius) in metres.
# Gaussian, at the map's edge: a softer, nearer tail lifted the path 40 m and moved the bench.
PEAKS = [(990.0, 330.0, 480.0, 200.0), (-990.0, 230.0, 520.0, 200.0)]
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
mi.ROAD_HALF = 1.5  # a bike-only path, 3 m of tarmac
mi.SHOULDER = 0.4
mi.BANK_SLOPE = 1.3  # cut and fill banks: short and steep, as a real cut is, not a long soil ramp
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
    gully = (mi.fbm(x * 1.0, y * 0.35, 160, 4, SEED) - 0.5) * 18
    swell = (mi.fbm(x, y, 420, 3, SEED + 7) - 0.5) * 40
    crag = (mi.ridged(x, y, 90, 4, SEED + 13) - 0.35) * 9 * mi.smoothstep(100, 400, up)
    bumps = (mi.fbm(x, y, 14, 3, SEED + 21) - 0.5) * 1.6
    # A rim of ridges round the map's edge, so the horizon is mountains, not the 2 km square's
    # edge and the sea past it. The road stays inside 560 m of the middle, clear of it.
    edge = np.maximum(np.abs(x), np.abs(y)) + (mi.fbm(x, y, 260, 3, SEED + 31) - 0.5) * 160
    rim = 250 * mi.smoothstep(520, 1080, edge)  # gentle: 350 m over 350 m read as a cliff
    peaks = sum(
        top
        * np.exp(-((x - px) ** 2 + (y - py) ** 2) / (r * r))
        * (0.8 + 0.4 * mi.ridged(x, y, 140, 4, SEED + 33))
        for px, py, top, r in PEAKS
    )
    return (base + gully + swell + crag + bumps + rim + peaks).astype(np.float32)


def at(xy):
    """Grid row and column of points in metres."""
    return (
        np.round(xy[:, 1] / SPACING + (RES - 1) / 2).astype(int),
        np.round(xy[:, 0] / SPACING + (RES - 1) / 2).astype(int),
    )


def lay_road(h):
    xy = mi.catmull(PLAN, False)
    z = np.maximum(mi.smooth(mi.sample(h, xy), 20, False), 1.0)
    z = mi.grade_fit(z, {}, False)
    return {"hero": {"xy": xy, "z": z, "closed": False}}


def open_valley_side(h, x, y, road):
    """Where the path crosses a spur, the valley side falls away from its edge instead of rising
    into a second bank, so the path never runs in a trench. Within 12 m on the downhill (+Y)
    side, the ground is at most 0.3 m above the path plus 15 cm a metre, fading out by 25 m.

    The uphill side is capped too: a cut of a metre or two, then a gentle wooded slope that rejoins
    the mountainside by 35 m, as a path benched into a hillside looks. Uncapped, the cuts across
    the steep flank stood 5-8 m high, bare walls no reference photo shows."""
    xy, z = road["xy"], road["z"]
    mark = np.zeros(h.shape, dtype=bool)
    row, col = at(xy)
    mark[row, col] = True
    index = np.full(h.shape, -1, dtype=np.int64)
    index[row, col] = np.arange(len(xy))
    dist, (ri, ci) = mi.ndimage.distance_transform_edt(~mark, return_indices=True)
    dist = dist * SPACING
    nearest = index[ri, ci]
    below = (y - xy[nearest, 1]) > 0
    cap = z[nearest] + 0.3 + 0.15 * np.maximum(dist - mi.ROAD_HALF - mi.SHOULDER, 0)
    weight = mi.smoothstep(25.0, 12.0, dist) * below * (dist > mi.ROAD_HALF + mi.SHOULDER)
    h = h - weight * np.maximum(h - cap, 0)
    edge = np.maximum(dist - mi.ROAD_HALF - mi.SHOULDER, 0)
    bump = (mi.fbm(x, y, 9, 2, SEED + 55) - 0.5) * 1.2
    cap_up = z[nearest] + np.minimum(1.3 * edge, 1.4 + bump + 0.28 * edge)
    weight_up = mi.smoothstep(35.0, 18.0, dist) * ~below * (dist > mi.ROAD_HALF + mi.SHOULDER)
    return (h - weight_up * np.maximum(h - cap_up, 0)).astype(np.float32)


def roughen_banks(h, x, y, road_dist):
    """Carved banks are smooth ramps. Give them the lumps, steps and hollows of a bank that has
    slumped and grown over: half a metre of relief at a few metres, broad slumps of a metre or so
    at ten, and rills washed down the face, kept off the path's edge."""
    gy, gx = np.gradient(h, SPACING)
    steep = mi.smoothstep(0.3, 0.6, np.hypot(gx, gy))
    near = mi.smoothstep(mi.ROAD_HALF + 1.0, mi.ROAD_HALF + 2.5, road_dist)
    near *= mi.smoothstep(25.0, 15.0, road_dist)
    lumps = (mi.fbm(x, y, 3.5, 3, SEED + 54) - 0.5) * 1.1
    lumps += (mi.fbm(x, y, 10, 2, SEED + 56) - 0.5) * 1.8
    # Rills: narrow grooves a few metres apart, drawn out downhill (the flank falls toward +Y).
    lumps -= mi.smoothstep(0.6, 0.85, mi.ridged(x, y * 0.3, 5, 2, SEED + 57)) * 0.5
    return (h + steep * near * lumps).astype(np.float32)


def paint(x, y, h, road_dist):
    gy, gx = np.gradient(h, SPACING)
    slope = np.hypot(gx, gy)
    # Bare rock only on the natural crags: a cut bank by the path is soil and litter with roots,
    # as in the reference photos, not a wall of pale granite.
    rock = mi.smoothstep(0.85, 1.1, slope) * mi.smoothstep(10.0, 25.0, road_dist)
    # Only past 25 m from the path: the short steep cuts by it are soil, and a scanned granite floor
    # on them read as a cobbled wall.
    rock = np.maximum(rock, mi.smoothstep(1.2, 1.5, slope) * mi.smoothstep(15.0, 25.0, road_dist))
    # v29: narrower (was 0.2 to 1.0 m past the tarmac): its bare 0.8 m band read as a dark strip
    # beside the path (QA, v27), and grass can't grow on it (Dirt lies under the road too).
    shoulder = mi.smoothstep(mi.ROAD_HALF + 0.6, mi.ROAD_HALF + 0.15, road_dist)
    # The cut and fill banks by the path are bare dark soil, as in the reference photos, rather
    # than lawn humps; the edge wanders so it doesn't read as a painted band.
    bank = mi.smoothstep(0.4, 0.6, slope + 0.15 * (mi.fbm(x, y, 6, 2, SEED + 50) - 0.5))
    bank *= mi.smoothstep(16.0, 9.0, road_dist + 4 * mi.fbm(x, y, 11, 2, SEED + 51))
    # Real cut banks are only bare in places: about half grows over with the woods' floor (ferns,
    # tufts, roots, stones), and stones and small outcrops break through here and there.
    overgrown = mi.smoothstep(0.42, 0.58, mi.fbm(x, y, 7, 3, SEED + 52))
    overgrown *= mi.smoothstep(mi.ROAD_HALF + 1.5, mi.ROAD_HALF + 3.5, road_dist)
    # Rare, and not right by the path: at 0.76-0.84 with the slumped banks' relief, outcrops took
    # whole cuts and read as pale granite walls (v12, found by false-colouring the layers).
    outcrop = bank * mi.smoothstep(0.86, 0.9, mi.fbm(x, y, 9, 2, SEED + 53)) * (slope > 0.7)
    outcrop *= mi.smoothstep(mi.ROAD_HALF + 3.0, mi.ROAD_HALF + 6.0, road_dist)
    rock = np.maximum(rock, outcrop)
    # The banks' soil is its own layer ("Sand"), so their stones and ferns stay off the path.
    soil = bank * (1 - rock) * (1 - overgrown) * (1 - shoulder)
    # Grass: the valley floor and clearings up the flank, and a broken verge beside the path, as in
    # the spruce photos: 1-2 m wide with a wandering edge, gone for a few metres here and there.
    floor = mi.smoothstep(VALLEY_Y - 140, VALLEY_Y - 40, y)
    clearing = mi.smoothstep(0.66, 0.74, mi.fbm(x, y, 120, 3, SEED + 40))
    reach = mi.ROAD_HALF + 1.6 + 1.2 * (mi.fbm(x, y, 5, 2, SEED + 58) - 0.5)
    verge = mi.smoothstep(mi.ROAD_HALF + 0.6, mi.ROAD_HALF + 1.0, road_dist)
    verge *= mi.smoothstep(reach + 0.8, reach, road_dist)
    verge *= mi.smoothstep(0.38, 0.5, mi.fbm(x, y, 12, 2, SEED + 59))
    # Above the treeline, alpine meadow.
    alpine = mi.smoothstep(
        TREELINE - 30, TREELINE + 15, h + 40 * (mi.fbm(x, y, 90, 3, SEED + 41) - 0.5)
    )
    # Rock slabs lie flat in the heath, tens of metres apart, clear of the path.
    slab = alpine * mi.smoothstep(0.7, 0.76, mi.fbm(x, y, 25, 3, SEED + 44))
    rock = np.maximum(rock, slab * mi.smoothstep(mi.ROAD_HALF + 2.0, mi.ROAD_HALF + 5.0, road_dist))
    soil = soil * (1 - rock)
    # Above the treeline most ground is heath (the "Desert" layer, unused on the climb till
    # now); meadow grass keeps the damper hollows, the valley floor and the path's verge.
    damp = mi.smoothstep(0.52, 0.64, mi.fbm(x, y, 45, 3, SEED + 43))
    heath = alpine * (1 - damp) * (1 - np.maximum(floor, verge))
    grass = np.clip(np.maximum(np.maximum(np.maximum(floor, clearing), verge), alpine), 0, 1)
    # Snow on the heights, thinner on crags, in drifts rather than a level line.
    snow = mi.smoothstep(
        SNOWLINE - 40, SNOWLINE + 40, h + 90 * (mi.fbm(x, y, 70, 3, SEED + 42) - 0.5)
    )
    # Steep faces shed their snow: dark rock ribs between snowfields, not a flat white cone (QA,
    # v17). slope is rise over run, so 0.7 is 35 degrees.
    snow *= 1 - mi.smoothstep(0.7, 1.1, slope)
    rest = (1 - shoulder - soil) * (1 - snow)
    soft = (1 - rock) * rest
    out = {k: np.zeros_like(h) for k in mi.LAYERS}
    out["Grass"] = soft * np.clip(grass - heath, 0, 1)
    out["Desert"] = soft * heath
    out["Forest"] = soft * (1 - grass)
    out["Rock"] = rock * rest
    out["Sand"] = soil * (1 - snow)
    out["Dirt"] = shoulder
    out["Snow"] = snow * (1 - shoulder)
    return out, slope, grass, heath


# The share of trees kept beyond the full-density reach: static Nanite trees cover the whole map.
FAR = float(os.environ.get("OC_FAR_TREES", "0.45"))


def trees(h, road_dist, slope, grass, heath):
    """Stems scattered about 4.5 m apart in the woods, in clumps and gaps (not rows), thinning to
    nothing in clearings and on crags, kept off the verge. Beech below, spruce above, mixed across
    a band with noise."""
    rng = np.random.default_rng(SEED + 100)
    step = 4.5
    reach = 320.0  # metres from the road at full density; beyond it the woods thin to FAR
    c = np.arange(-(RES - 1) / 2 + step / 2, (RES - 1) / 2, step)
    gx, gy = np.meshgrid(c, c)
    # A jitter wider than the cell, so no rows show down a line of sight.
    px = (gx + rng.normal(0, 0.6, gx.shape) * step).ravel()
    py = (gy + rng.normal(0, 0.6, gy.shape) * step).ravel()
    row, col = at(np.stack([px, py], 1))
    ok = (row >= 0) & (row < RES) & (col >= 0) & (col < RES)
    px, py, row, col = px[ok], py[ok], row[ok], col[ok]
    d = road_dist[row, col]
    # A few spruce stand out in the heath, alone or in small groups.
    wood = 1 - grass[row, col] + 0.16 * heath[row, col]
    # Clumps and gaps tens of metres across, and finer ones a few trees wide.
    clump = mi.smoothstep(0.3, 0.62, mi.fbm(px, py, 60, 3, SEED + 70))
    clump *= 0.55 + 0.45 * mi.smoothstep(0.25, 0.55, mi.fbm(px, py, 14, 2, SEED + 71))
    wood = wood * np.clip(0.15 + 1.1 * clump, 0, 1)
    keep = (
        # Off the verge, and thinning in over a few metres rather than standing in a line along
        # it: in the reference photos the woods come right up to the path.
        (d > mi.ROAD_HALF + 1.8)
        & (rng.random(d.shape) < mi.smoothstep(mi.ROAD_HALF + 1.8, mi.ROAD_HALF + 8, d))
        # Spruce and beech grow on the cut banks too, short of a cliff.
        & (slope[row, col] < 1.45)
        & (rng.random(d.shape) < wood * 0.92 * np.where(d < reach, 1.0, FAR))
        # The treeline: stems thin out over 80 m of height, the last of them spruce.
        & (rng.random(d.shape) < mi.smoothstep(TREELINE + 45, TREELINE - 20, h[row, col]))
    )
    px, py, row, col = px[keep], py[keep], row[keep], col[keep]
    # The lowest ground under the trunk, so it stands in the slope rather than on its high side.
    z = np.min(
        [
            h[np.clip(row + i, 0, RES - 1), np.clip(col + j, 0, RES - 1)]
            for i in (-1, 0, 1)
            for j in (-1, 0, 1)
        ],
        axis=0,
    )
    # The change from beech to spruce, in metres of height, wandering with noise.
    line = 95 + 30 * (mi.fbm(px, py, 150, 3, SEED + 60) - 0.5)
    spruce = rng.random(z.shape) < mi.smoothstep(line - 25, line + 25, z)
    # Toward the treeline the spruce are stunted: half size where the woods end.
    stunt = 1 - 0.5 * mi.smoothstep(TREELINE - 30, TREELINE + 20, z)
    out = []
    for xm, ym, zm, s, small in zip(px, py, z, spruce, stunt, strict=True):
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
                round(float(zm) * 100 - 30, 1),
                round(float(rng.uniform(0, 360)), 1),
                round(float(size * small), 3),
            ]
        )
    return out


def props(h, road, road_dist):
    """Things people left: a bench on the hairpin's outside looking down the valley, and stacks
    of cut logs on flat spots beside the path. [kind, x, y, z, yaw] in cm and degrees."""
    xy, z = road["xy"], road["z"]
    d = np.gradient(xy, axis=0)
    d /= np.linalg.norm(d, axis=1, keepdims=True)
    out = []
    apex = int(np.argmax(xy[:, 0]))  # the hairpin turns at the road's east end
    for side in (-1, 1):
        normal = np.array([-d[apex, 1], d[apex, 0]]) * side
        p = xy[apex] + normal * (mi.ROAD_HALF + 2.2)
        ground = mi.sample(h, p[None])[0]
        if ground < z[apex] - 0.3:  # the drop side, behind the fence
            continue
        # On the uphill side, facing across the path and out over the valley (the bench's
        # sitter looks along its +X).
        yaw = np.degrees(np.arctan2(-normal[1], -normal[0]))
        out.append(
            [
                "bench",
                *(p * 100).round(1).tolist(),
                round(float(ground) * 100, 1),
                round(float(yaw), 1),
            ]
        )
        break
    rng = np.random.default_rng(SEED + 200)
    placed = []
    for k in rng.permutation(np.arange(80, len(xy) - 80)):
        if len(placed) == 3:
            break
        if any(abs(k - j) < 200 for j in placed) or abs(k - apex) < 60:
            continue
        for side in (-1, 1):
            normal = np.array([-d[k, 1], d[k, 0]]) * side
            spot = xy[k] + normal * (mi.ROAD_HALF + 3.2)
            ends = [spot + d[k] * 1.8, spot - d[k] * 1.8, spot + normal * 0.8]
            hs = mi.sample(h, np.array(ends))
            if np.ptp(hs) < 0.35 and abs(hs.mean() - z[k]) < 0.6:
                yaw = np.degrees(np.arctan2(d[k, 1], d[k, 0]))
                out.append(
                    [
                        "woodpile",
                        *(spot * 100).round(1).tolist(),
                        round(float(hs.min()) * 100, 1),
                        round(float(yaw), 1),
                    ]
                )
                placed.append(k)
                break
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
    h = open_valley_side(h, x, y, road)
    h = roughen_banks(h, x, y, road_dist)
    layers, slope, grass, heath = paint(x, y, h, road_dist)
    tree_list = trees(h, road_dist, slope, grass, heath)
    rail_runs = rails(h, road)
    prop_list = props(h, road, road_dist)
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
    with open(f"{OUT}/props.json", "w") as f:
        json.dump(prop_list, f)
    length = float(np.linalg.norm(np.diff(xy, axis=0), axis=1).sum())
    grade = np.diff(z) / np.linalg.norm(np.diff(xy, axis=0), axis=1)
    spruce = sum(t[0] == "spruce" for t in tree_list)
    print(
        f"{OUT}: ground {h.min():.0f}..{h.max():.0f} m; road {length:.0f} m from {z[0]:.0f} to"
        f" {z[-1]:.0f} m, mean {(z[-1] - z[0]) / length:.1%}, steepest {grade.max():.1%};"
        f" {len(pieces)} road pieces; trees {len(tree_list)} ({spruce} spruce);"
        f" rails {len(rail_runs)} runs, {sum(map(len, rail_runs))} m;"
        f" props {[p[0] for p in prop_list]}"
    )


if __name__ == "__main__":
    main()
