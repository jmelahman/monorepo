"""Writes the coast ride's ground as glTF tiles into assets_gen/terrain, plus the sea.

Near tiles (200 m, 1.25 m spacing) cover everything within NEAR_REACH of the path; far tiles
(2 km, 20 m spacing) carry the land out to the mountains, sunk SINK cm wherever a near tile lies
over them so the two never fight. Vertex colours carry the material masks the terrain material
blends by:

    R  rock: steep ground
    G  sand: the shore, around sea level
    B  mud: the flooded paddy fields
    A  forest floor (litter) rather than grass

Run with system Python: python3 -I scripts/make_terrain.py [--only=near|far|sea]
"""

import math
import multiprocessing
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import coast_layout
import gltf_writer

OUT = os.path.join(os.environ.get("OC_GEN") or "/workspace/ue/assets_gen", "terrain")
NEAR_TILE = 20000.0
NEAR_STEP = 125.0
NEAR_REACH = 26000.0
FAR_TILE = 200000.0
FAR_STEP = 2000.0
SINK = 300.0

PATH = coast_layout.CoastPath()
xs, ys = PATH.xs, PATH.ys
# Far tiles reach 4 km behind the start and past the end, 6 km inland and 4 km out to sea: the
# short ride's box was exactly that, (-4, -6) to (6, 4) km.
FAR_BOX = (
    round((min(xs) - 400000.0) / 100000) * 100000.0,
    round((min(ys) - 600000.0) / 100000) * 100000.0,
    max(xs) + 390000.0,
    max(ys) + 340000.0,
)  # x0, y0, x1, y1
BOX = (min(xs) - NEAR_REACH, min(ys) - NEAR_REACH, max(xs) + NEAR_REACH, max(ys) + NEAR_REACH)


def near_origin():
    return (math.floor(BOX[0] / NEAR_TILE) * NEAR_TILE, math.floor(BOX[1] / NEAR_TILE) * NEAR_TILE)


def near_tiles():
    """Tile corners whose square comes within NEAR_REACH of the path."""
    ox, oy = near_origin()
    tiles = []
    nx = math.ceil((BOX[2] - ox) / NEAR_TILE)
    ny = math.ceil((BOX[3] - oy) / NEAR_TILE)
    for i in range(nx):
        for j in range(ny):
            x0, y0 = ox + i * NEAR_TILE, oy + j * NEAR_TILE
            cx, cy = x0 + NEAR_TILE / 2, y0 + NEAR_TILE / 2
            _, lateral = PATH.locate(cx, cy)
            if abs(lateral) - NEAR_TILE * 0.71 < NEAR_REACH:
                tiles.append((x0, y0))
    return tiles


NEAR = None


def covered(x, y):
    """Whether (x, y) lies under a near tile."""
    global NEAR
    if NEAR is None:
        NEAR = set(near_tiles())
    ox, oy = near_origin()
    key = (
        ox + math.floor((x - ox) / NEAR_TILE) * NEAR_TILE,
        oy + math.floor((y - oy) / NEAR_TILE) * NEAR_TILE,
    )
    return key in NEAR


def masks(x, y, h, slope, where):
    s, lateral = where
    weights = PATH.zones(s)
    noise = coast_layout.fbm(x / 1500, y / 1500, 3, seed=21)
    rock = coast_layout.smoothstep(0.55, 1.0, slope + 0.25 * noise)
    # Sand only on the beach itself: the verge a few metres above the tide is grass.
    shore = 1 - coast_layout.smoothstep(
        coast_layout.SEA_LEVEL + 60, coast_layout.SEA_LEVEL + 180 + 80 * noise, h
    )
    # And only on the sea side (lateral > 0): the cut above the coast road is as low, but earth.
    sand = (
        shore
        * (1 - coast_layout.smoothstep(0.5, 0.9, slope))
        * coast_layout.smoothstep(-300, 100, lateral)
    )
    width = PATH.width(s) / 2 + coast_layout.SHELF
    across = abs(lateral) - width
    mud = (
        weights.get("paddies", 0)
        * coast_layout.smoothstep(120, 220, across)
        * (1 - coast_layout.smoothstep(9000, 14000, abs(lateral)))
    )
    # The coast's sea side (lateral > 0, where the rail runs) is an open verge of rough grass; only
    # its
    # wooded landward slope has a litter floor.
    wild = weights.get("jungle", 0) + weights.get("coast", 0) * (
        1 - coast_layout.smoothstep(-400, 0, lateral)
    )
    # Forest floor: the jungle and coast, and the wooded hills behind the town, but not the town's
    # own verges and gardens, which are grass.
    hills = coast_layout.smoothstep(6000, 16000, abs(lateral)) * coast_layout.smoothstep(
        100, 1500, h
    )
    forest = max(wild * coast_layout.smoothstep(150, 500, across + 200 * noise), hills)
    # The village: mossy forest floor on the lots and slopes, the stream bed's mud and stones.
    village = weights.get("village", 0)
    if village:
        lo, hi = coast_layout.STREAM
        bed = coast_layout.smoothstep(lo - 200, lo, lateral) * (
            1 - coast_layout.smoothstep(hi, hi + 250, lateral)
        )
        mud = max(mud, village * bed)
        forest = max(forest, village * coast_layout.smoothstep(0, 60, across))
    # A worn strip along the path's edge, where wheels and feet have scuffed the grass to earth.
    worn = (1 - coast_layout.smoothstep(0, 40 + 50 * noise, across)) * (0.4 + 0.3 * noise)
    forest = max(forest, worn * (1 - village))
    forest *= 1 - mud
    return (rock, sand, mud, min(1.0, forest))


def build_tile(args):
    kind, x0, y0, size, step = args
    n = round(size / step)
    heights = {}
    places = {}
    for i in range(-1, n + 2):
        for j in range(-1, n + 2):
            x, y = x0 + i * step, y0 + j * step
            where = PATH.locate(x, y)
            h = PATH.ground(x, y)
            if kind == "far" and covered(x, y):
                h -= SINK
            heights[i, j] = h
            places[i, j] = where
    mesh = gltf_writer.Mesh()
    part = mesh.part("Terrain")
    ids = {}
    for i in range(n + 1):
        for j in range(n + 1):
            x, y = x0 + i * step, y0 + j * step
            dzdx = (heights[i + 1, j] - heights[i - 1, j]) / (2 * step)
            dzdy = (heights[i, j + 1] - heights[i, j - 1]) / (2 * step)
            normal = (-dzdx, -dzdy, 1.0)
            slope = math.sqrt(dzdx * dzdx + dzdy * dzdy)
            color = masks(x, y, heights[i, j], slope, places[i, j])
            ids[i, j] = part.vertex((x, y, heights[i, j]), normal, (x / 100.0, y / 100.0), color)
    for i in range(n):
        for j in range(n):
            # The writer wants each face's corners in the order whose cross product, taken in
            # Unreal's numbers, points out of the face (as Mesh.box does): +X then +Y for a floor.
            a, b, c, d = ids[i, j], ids[i + 1, j], ids[i + 1, j + 1], ids[i, j + 1]
            part.quad(a, b, c, d)
    name = f"{kind}_{int(x0 / 100)}_{int(y0 / 100)}".replace("-", "m")
    mesh.write(os.path.join(OUT, f"T_{name}.gltf"))
    return name


def build_sea():
    mesh = gltf_writer.Mesh()
    part = mesh.part("Sea")
    # 30 km square for the short ride, as it always was; a long ride stretches it east to fit.
    size, n = 3000000.0, 60
    reach = max(size, max(xs) + 1600000.0)
    across = round(n * reach / size)
    left, top, deep, down = -size / 2 + 100000, -size / 2 + 1000000, size, n
    if PATH.closed:  # an island: sea on every side, 12 km past the loop
        left, top = min(xs) - 1200000.0, min(ys) - 1200000.0
        reach, deep = max(xs) + 1200000.0 - left, max(ys) + 1200000.0 - top
        across, down = round(n * reach / size), round(n * deep / size)
    ids = {}
    for i in range(across + 1):
        for j in range(down + 1):
            x, y = left + i * reach / across, top + j * deep / down
            ids[i, j] = part.vertex((x, y, coast_layout.SEA_LEVEL), (0, 0, 1), (x / 100, y / 100))
    for i in range(across):
        for j in range(down):
            part.quad(ids[i, j], ids[i + 1, j], ids[i + 1, j + 1], ids[i, j + 1])
    mesh.write(os.path.join(OUT, "T_sea.gltf"))


def main():
    only = next((a.split("=", 1)[1] for a in sys.argv[1:] if a.startswith("--only=")), None)
    os.makedirs(OUT, exist_ok=True)
    jobs = []
    if only in (None, "near"):
        jobs += [("near", x0, y0, NEAR_TILE, NEAR_STEP) for x0, y0 in near_tiles()]
    if only in (None, "far"):
        x = FAR_BOX[0]
        while x < FAR_BOX[2]:
            y = FAR_BOX[1]
            while y < FAR_BOX[3]:
                jobs.append(("far", x, y, FAR_TILE, FAR_STEP))
                y += FAR_TILE
            x += FAR_TILE
    if only in (None, "sea"):
        build_sea()
    print(f"{len(jobs)} tiles", flush=True)
    with multiprocessing.Pool(max(1, (os.cpu_count() or 4) - 2)) as pool:
        for name in pool.imap_unordered(build_tile, jobs):
            print(name, flush=True)


if __name__ == "__main__":
    main()
