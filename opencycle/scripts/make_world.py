"""The world plan: the island's layout at seven times the size, about 57 km across, so each
biome holds a one to two hour ride and the ring road is a five to six hour lap.

This is a plan, not a landscape: a coarse height map (28 m a sample), the regions, the roads
every 10 m with graded heights, and the rides with their length, climb and time. Unreal never
loads it whole. A stretch of it is built as a 2 km window through the climb pipeline
(`make_climb.py`, `oc.py build-level`) with that region's biome preset; `windows` in world.json
names one such stretch per biome.

Run: uv run -q --no-project --with numpy --with scipy --with pillow python -I
scripts/make_world.py. Writes to /workspace/ue/world_gen/world:
- preview.png (and preview_small.jpg): the map, shaded, coloured by region, with the roads and
  a 5 km grid;
- world.json: the grid, the regions, each ride's length, climb, time and biome shares, and a
  hero window per biome;
- roads.json: each road's centreline every 10 m as [x, y, z] in metres;
- height.npy: the heights in metres, float32, RES x RES, rows along Y.

Coordinates are Unreal's: X east, Y south, metres, the origin in the middle of the map. The
layout (regions, road control points, rides) is `make_island.py`'s, in its kilometres times K.
"""

import heapq
import json
import math
import os
import sys

import numpy as np
from PIL import Image
from scipy import ndimage

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import make_island as mi

K = 7.0  # world metres per island metre
RES = 2049
EXTENT = 8128.0 * K
SPACING = EXTENT / (RES - 1)
STEP = 10.0  # metres between road points
OUT = "/workspace/ue/world_gen/world"
SEED = 7
# Regions added to the island's: centre (island km), reach (island km).
REGIONS = {**mi.REGIONS, "heath": ((-2.0, -1.9), 0.9)}
COLOURS = {**mi.COLOURS, "heath": (150, 122, 132)}
# Metres of mid-scale relief (2 km hills) each region adds to the island's broad shape.
RELIEF = {"mountain": 160, "desert": 50, "forest": 90, "meadow": 35, "town": 15, "heath": 70}
# The road wanders either side of its drawn line: (wavelength m, amplitude m).
WANDER = ((1700, 70), (2900, 110), (4600, 150))
CLIMB_CELL = 80.0
CLIMB_SMOOTH = 600.0
SNOW_LINE = 900.0
# Riding speed in km/h by grade, for the ride times: 30 on the flat, 11 up 7%, 50 down 5%.
FLAT_SPEED, UP_SLOW, DOWN_FAST = 30.0, 270.0, 400.0


def grid():
    c = (np.arange(RES, dtype=np.float32) - (RES - 1) / 2) * SPACING
    return np.meshgrid(c, c)


def sample(a, xy):
    col = xy[:, 0] / SPACING + (RES - 1) / 2
    row = xy[:, 1] / SPACING + (RES - 1) / 2
    return ndimage.map_coordinates(a, [row, col], order=1, mode="nearest")


def terrain(x, y):
    xi, yi = x / K, y / K  # the island's own metres
    land = mi.land_mask(xi, yi)
    inland = ndimage.distance_transform_edt(land) * SPACING
    offshore = ndimage.distance_transform_edt(~land) * SPACING
    mi.REGIONS = REGIONS
    weights = mi.region_weights(xi, yi)
    heights = mi.region_heights(xi, yi)
    heights["heath"] = 300 + 140 * mi.fbm(xi, yi, 700, 4, SEED + 71)
    hills = mi.fbm(x, y, 2200, 5, SEED + 81) - 0.5
    h = sum(weights[k] * (heights[k] + 2 * RELIEF[k] * hills) for k in weights)
    cliff = mi.smoothstep(-1800, -2600, xi) * mi.smoothstep(2400, 1200, yi)
    cliff = cliff + mi.smoothstep(-2200, -3000, yi)
    cliff = np.clip(cliff + 0.5 * (mi.fbm(xi, yi, 900, 3, SEED + 61) - 0.5), 0, 1)
    fall = 1500 + (120 - 1500) * cliff
    beach = 2.5 + 1.5 * mi.fbm(x, y, 400, 2, SEED + 62)
    h = beach + (np.maximum(h, 4) - beach) * mi.smoothstep(0, 1, inland / fall)
    sea = -1.0 - 40 * mi.smoothstep(0, 3000, offshore)
    return np.where(land, h, sea).astype(np.float32), land, inland, weights, cliff


def wander(xy, closed, inland):
    """The line moved sideways by a few slow waves, less near the shore and not at all at an
    open road's ends, then stepped evenly again."""
    s = np.arange(len(xy)) * STEP
    t = np.gradient(xy, axis=0)
    t /= np.linalg.norm(t, axis=1, keepdims=True) + 1e-9
    rng = np.random.default_rng(SEED + len(xy))
    off = sum(a * np.sin(2 * math.pi * s / w + rng.uniform(0, 6.28)) for w, a in WANDER)
    off = off * mi.smoothstep(400, 1500, sample(inland, xy))
    if closed:
        off = off * mi.smoothstep(0, 1500, np.minimum(s, s[-1] - s))  # the seam
    else:
        off = off * mi.smoothstep(0, 1500, np.minimum(s, s[-1] - s))
    moved = xy + np.stack([-t[:, 1], t[:, 0]], axis=1) * off[:, None]
    seg = np.linalg.norm(np.diff(moved, axis=0), axis=1)
    at = np.concatenate([[0], np.cumsum(seg)])
    even = np.arange(0, at[-1], STEP)
    return np.stack([np.interp(even, at, moved[:, i]) for i in (0, 1)], axis=1)


def climb(h, start, roads):
    """Control points (world km) from `start` to the summit: `make_island.climb` at world size.
    A least-cost path over CLIMB_CELL cells where a step steeper than the climb's grade costs
    many times its length, so the road winds and switches back."""
    grade = mi.CLIMB_GRADE
    cx = np.arange((start[0] - 1.6 * K) * 1000, (start[0] + 1.6 * K) * 1000, CLIMB_CELL)
    cy = np.arange((start[1] - 1.7 * K) * 1000, (start[1] + 0.15 * K) * 1000, CLIMB_CELL)
    gx, gy = np.meshgrid(cx, cy)
    soft = ndimage.gaussian_filter(h, CLIMB_SMOOTH / SPACING)
    g = sample(soft, np.stack([gx.ravel(), gy.ravel()], axis=1)).reshape(gx.shape)
    side = np.hypot(*np.gradient(g, CLIMB_CELL))
    rows, cols = g.shape
    near = np.zeros(g.shape, dtype=bool)
    for road in roads.values():
        col = np.round((road["xy"][:, 0] - cx[0]) / CLIMB_CELL).astype(int)
        row = np.round((road["xy"][:, 1] - cy[0]) / CLIMB_CELL).astype(int)
        ok = (col >= 0) & (col < cols) & (row >= 0) & (row < rows)
        near[row[ok], col[ok]] = True
    near = ndimage.distance_transform_edt(~near) * CLIMB_CELL < 600
    near &= np.hypot(gx - start[0] * 1000, gy - start[1] * 1000) > 900
    goal = np.unravel_index(np.argmax(g), g.shape)
    begin = (
        round((start[1] * 1000 - cy[0]) / CLIMB_CELL),
        round((start[0] * 1000 - cx[0]) / CLIMB_CELL),
    )
    steps = [
        (dr, dc)
        for dr in range(-2, 3)
        for dc in range(-2, 3)
        if (dr, dc) != (0, 0) and math.gcd(abs(dr), abs(dc)) == 1
    ]
    angle = [math.atan2(dr, dc) for dr, dc in steps]
    turns = [[abs((b - a + math.pi) % (2 * math.pi) - math.pi) for b in angle] for a in angle]
    best = np.full((*g.shape, len(steps)), np.inf)
    came, todo = {}, []
    for d in range(len(steps)):
        best[(*begin, d)] = 0.0
        todo.append((0.0, (*begin, d)))
    end = None
    while todo:
        cost, state = heapq.heappop(todo)
        r, c, d = state
        if (r, c) == goal:
            end = state
            break
        if cost > best[state]:
            continue
        for e, (dr, dc) in enumerate(steps):
            turn = turns[d][e]
            nr, nc = r + dr, c + dc
            if turn > math.pi / 2 + 1e-6 or not (0 <= nr < rows and 0 <= nc < cols):
                continue
            if g[nr, nc] < 15:
                continue
            length = CLIMB_CELL * math.hypot(dr, dc)
            rise = (g[nr, nc] - g[r, c]) / length
            steep = max(0.0, abs(rise) - grade) / grade
            down = max(0.0, -rise) / grade
            across = max(0.0, side[nr, nc] - 0.45)
            crowd = 20.0 if near[nr, nc] else 0.0
            step = length * (1 + 30 * steep + 80 * steep * steep + 1.5 * down + 4 * across + crowd)
            step += 60 * (turn / (math.pi / 2)) ** 2
            nxt = (nr, nc, e)
            if cost + step < best[nxt]:
                best[nxt] = cost + step
                came[nxt] = state
                heapq.heappush(todo, (cost + step, nxt))
    path = [end]
    while path[-1][:2] != begin:
        path.append(came[path[-1]])
    path = [p[:2] for p in reversed(path)]
    top = np.array(goal) * CLIMB_CELL
    short = [np.hypot(*(np.array(p) * CLIMB_CELL - top)) < 700 for p in path]
    path = path[: short.index(True) + 1]
    pts = np.array([(cx[c], cy[r]) for r, c in path]) / 1000
    pts[0] = start
    return [tuple(p) for p in mi.simplify(pts, 30 / 1000)]


def lay_roads(h, inland):
    """Each road every STEP metres with a graded height; later roads meet earlier ones."""
    roads = {}
    plans = {name: [(a * K, b * K) for a, b in pts] for name, pts in mi.ROADS.items()}
    plans.update({name: None for name in mi.CLIMBS})
    for name, points in plans.items():
        closed = name in mi.CLOSED
        if points is None:
            a, b = mi.CLIMBS[name]
            points = climb(h, (a * K, b * K), roads)
        ends = {}
        for end in (0, -1) if not closed else ():
            for other in roads.values():
                d = np.linalg.norm(other["xy"] - np.array(points[end]) * 1000, axis=1)
                k = int(np.argmin(d))
                if d[k] < 300 * K:
                    points[end] = tuple(other["xy"][k] / 1000)
                    ends[end] = other["z"][k]
                    break
        xy = mi.catmull(points, closed, step=STEP)
        if name not in mi.CLIMBS:
            xy = wander(xy, closed, inland)
        z = np.maximum(mi.smooth(sample(h, xy), 30, closed), 3.0)
        fixed = {(len(z) - 1 if end else 0): v / STEP for end, v in ends.items()}
        z = mi.grade_fit(z / STEP, fixed, closed) * STEP
        roads[name] = {"xy": xy, "z": z, "closed": closed}
    return roads


def hours(line):
    d = np.linalg.norm(np.diff(line[:, :2], axis=0), axis=1)
    grade = np.diff(line[:, 2]) / np.maximum(d, 1e-6)
    speed = np.where(grade > 0, FLAT_SPEED - UP_SLOW * grade, FLAT_SPEED - DOWN_FAST * grade)
    return float((d / 1000 / np.clip(speed, 9, 55)).sum())


def shares(line, weights, inland):
    """The share of a ride in each biome: the strongest region, or coast within 1.2 km of sea."""
    names = list(weights)
    w = np.stack([sample(weights[n], line[:, :2]) for n in names])
    best = np.array(names)[np.argmax(w, axis=0)]
    best = np.where(sample(inland, line[:, :2]) < 1200, "coast", best)
    return {n: round(float((best == n).mean()), 2) for n in [*names, "coast"] if (best == n).any()}


def windows(rides, weights, inland):
    """One 2 km stretch per biome to build first: where a ride is deepest in that biome."""
    out = {}
    for biome in [*weights, "coast"]:
        top = (-1.0, None)
        for name, line in rides.items():
            if biome == "coast":
                score = 1 - sample(inland, line[:, :2]) / 1200
            else:
                score = sample(weights[biome], line[:, :2])
            k = int(np.argmax(score))
            if score[k] > top[0]:
                top = (float(score[k]), (name, k, line[k]))
        name, k, p = top[1]
        out[biome] = {
            "ride": name,
            "at_km": round(k * STEP / 1000, 1),
            "centre": [round(float(v)) for v in p],
        }
    return out


def preview(h, land, weights, roads, cliff):
    gy, gx = np.gradient(h, SPACING)
    light = np.array([-0.6, -0.6, 0.55])
    light /= np.linalg.norm(light)
    n = np.stack([-gx * 4, -gy * 4, np.ones_like(h)])  # relief exaggerated to read at 28 m
    shade = np.clip((n * light[:, None, None]).sum(0) / np.linalg.norm(n, axis=0), 0, 1)
    rgb = np.zeros((*h.shape, 3), dtype=np.float32)
    for name, w in weights.items():
        rgb += w[..., None] * np.array(COLOURS[name], dtype=np.float32)
    snow = mi.smoothstep(SNOW_LINE - 100, SNOW_LINE + 50, h)[..., None]
    rgb = rgb * (1 - snow) + 245 * snow
    beach = (land & (h < 5.5) & (cliff < 0.5))[..., None]
    rgb = np.where(beach, np.array(COLOURS["beach"], dtype=np.float32), rgb)
    rgb *= (0.35 + 0.8 * shade)[..., None]
    sea = (
        np.array(COLOURS["sea"], dtype=np.float32)
        * (1.2 - 0.6 * mi.smoothstep(0, -30, h))[..., None]
    )
    rgb = np.where(land[..., None], rgb, sea)
    on = np.zeros(h.shape, dtype=bool)
    for road in roads.values():
        col = np.round(road["xy"][:, 0] / SPACING + (RES - 1) / 2).astype(int)
        row = np.round(road["xy"][:, 1] / SPACING + (RES - 1) / 2).astype(int)
        on[np.clip(row, 0, RES - 1), np.clip(col, 0, RES - 1)] = True
    rgb = np.where(ndimage.binary_dilation(on)[..., None], 250.0, rgb)
    c = (np.arange(RES) - (RES - 1) / 2) * SPACING
    lines = np.abs(c - np.round(c / 5000) * 5000) < SPACING / 2
    rgb[lines, :, :] = rgb[lines, :, :] * 0.6 + 40
    rgb[:, lines, :] = rgb[:, lines, :] * 0.6 + 40
    image = Image.fromarray(np.clip(rgb, 0, 255).astype(np.uint8))
    image.save(f"{OUT}/preview.png")
    image.resize((1024, 1024), Image.LANCZOS).save(f"{OUT}/preview_small.jpg", quality=88)


def main():
    os.makedirs(OUT, exist_ok=True)
    x, y = grid()
    h, land, inland, weights, cliff = terrain(x, y)
    roads = lay_roads(h, inland)
    mi.ROUTES = {
        name: [
            (r, (a[0] * K, a[1] * K), b if b == "end" else (b[0] * K, b[1] * K)) for r, a, b in legs
        ]
        for name, legs in mi.ROUTES.items()
    }
    rides = mi.stitch(roads)
    preview(h, land, weights, roads, cliff)
    np.save(f"{OUT}/height.npy", h)
    info = {
        "extent_m": EXTENT,
        "res": RES,
        "spacing_m": SPACING,
        "scale": K,
        "regions": {
            n: {"centre_km": [c[0] * K, c[1] * K], "reach_km": r * K}
            for n, (c, r) in REGIONS.items()
        },
        "rides": {},
        "windows": windows(rides, weights, inland),
    }
    for name, line in rides.items():
        d = np.linalg.norm(np.diff(line[:, :2], axis=0), axis=1)
        info["rides"][name] = {
            "km": round(float(d.sum()) / 1000, 1),
            "climb_m": round(float(np.clip(np.diff(line[:, 2]), 0, None).sum())),
            "top_m": round(float(line[:, 2].max())),
            "steepest": round(float(np.abs(np.diff(line[:, 2]) / np.maximum(d, 1e-6)).max()), 3),
            "hours": round(hours(line), 2),
            "biomes": shares(line, weights, inland),
        }
        print(name, info["rides"][name])
    with open(f"{OUT}/world.json", "w") as f:
        json.dump(info, f, indent=1)
    with open(f"{OUT}/roads.json", "w") as f:
        json.dump(
            {n: np.column_stack([r["xy"], r["z"]]).round(1).tolist() for n, r in roads.items()}, f
        )
    print(
        f"{OUT}: {EXTENT / 1000:.0f} km at {SPACING:.0f} m, heights {h.min():.0f}..{h.max():.0f} m"
    )
    print("windows", info["windows"])


if __name__ == "__main__":
    main()
