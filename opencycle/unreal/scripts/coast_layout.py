"""The coast ride's geography: one narrow bike path through a small town, across rice paddies,
up through jungle and along a mountainside above the sea.

Pure Python, no Unreal, so the geometry can be tested and the mesh builders can share it.
Units are Unreal's: centimetres, +X east, +Y south (right of a rider heading +X), +Z up.
The sea lies to the south (+Y) and the mountains to the north (-Y).

    path = CoastPath()
    x, y, z, yaw = path.at(s)           # s is centimetres along the path from its start
    s, lateral = path.locate(x, y)      # lateral > 0 is right of the path, towards the sea
    h = path.ground(x, y)               # terrain height there
"""

import itertools
import math
import os
import random

M = 100.0  # centimetres per metre

# Plan of the path, in metres. Heading east, curving right (south) to the coast.
CONTROL = [
    (-60, 0),
    (0, 0),
    (110, 4),
    (220, -6),
    (320, 6),  # town
    (430, 14),
    (540, 36),
    (640, 70),  # paddies
    (730, 120),
    (810, 190),
    (880, 265),
    (960, 330),
    (1050, 380),  # jungle, climbing
    (1150, 405),
    (1260, 410),
    (1370, 440),
    (1480, 485),
    (1590, 505),
    (1700, 500),  # coast
    (1810, 520),
    (1920, 555),
    (2030, 570),
    (2120, 575),
]

# Height of the path above the town, by distance along it (metres).
PROFILE = [
    (0, 0),
    (380, 0),
    (660, 0.4),
    (760, 3),
    (900, 12),
    (1050, 24),
    (1180, 33),
    (1330, 38),
    (1480, 34),
    (1630, 40),
    (1800, 36),
    (1950, 31),
    (2300, 28),
]

# Where each part of the ride starts (metres along the path); each blends into the next over BLEND.
ZONES = [("town", 0), ("paddies", 390), ("jungle", 680), ("village", 900), ("coast", 1130)]
BLEND = 40.0

SEA_LEVEL = -8 * M
SEA_FLOOR = -26 * M
PATH_WIDTH = {"town": 300.0, "paddies": 220.0, "jungle": 250.0, "village": 420.0, "coast": 270.0}
SHELF = 120.0  # cm of level ground either side of the path before the terrain takes over

# How far (cm) the ground takes to meet the land around it, beyond the shelf, on each side.
# Narrow in the paddies (a dike) and on the coast (a road cut into the slope).
REACH = {
    "town": (3000.0, 3000.0),
    "paddies": (150.0, 150.0),
    "jungle": (1400.0, 1400.0),
    "village": (60.0, 1.0),
    "coast": (900.0, None),
}

# The village is a gorge: lots for the inns on the left, a stream in a channel on the right.
VILLAGE_LOTS = 1700.0  # cm of level ground left of the path's centre for the buildings
BANK = 235.0  # lateral cm where the bank starts down to the stream, just past the kerb
STREAM = (550.0, 1150.0)  # lateral cm of the stream bed's flat bottom
STREAM_DEPTH = 380.0  # cm below the path

# The weather along the ride, by metres: sun into cloud, rain over the village, clearing to a low
# golden sun on the coast. (metres, sun lux, sun elevation, fog density, sky light, rain 0..1)
WEATHER = [
    (0, 90000, 34, 0.006, 1.0, 0.0),
    (600, 90000, 33, 0.006, 1.0, 0.0),
    (820, 25000, 31, 0.025, 0.8, 0.0),
    (885, 4000, 30, 0.08, 0.55, 1.0),
    (1110, 4000, 27, 0.08, 0.55, 1.0),
    (1200, 50000, 21, 0.015, 0.9, 0.0),
    (1450, 75000, 15, 0.008, 1.0, 0.0),
    (2150, 70000, 8, 0.007, 1.0, 0.0),
]
WEATHER_KEYS = ("lux", "elevation", "fog", "sky", "rain")
DRY_AFTER = 120.0  # metres after the rain stops that the road stays wet

# Peaks behind the coast and the town: (x, y, height, radius) in cm.
MOUNTAINS = [
    (150000, -170000, 70000, 110000),
    (60000, -150000, 38000, 80000),
    (230000, -110000, 42000, 70000),
    (-40000, -120000, 26000, 70000),
]

# OC_PLAN=long carries the ride on past the coast for about OC_KM kilometres, in stretches that
# repeat the zones in a seeded order, and closes it into a loop round an island: the rider turns
# left all the way, so the mountains stand inside and the sea stays on the right. It comes back
# into the start town from the west (see extend_plan at the end). The first 2 km are the same.
PLAN = os.environ.get("OC_PLAN", "short")
LONG_KM = float(os.environ.get("OC_KM", "10"))
CLOSED = PLAN == "long"


def smoothstep(a, b, x):
    if a == b:
        return 0.0 if x < a else 1.0
    t = min(1.0, max(0.0, (x - a) / (b - a)))
    return t * t * (3 - 2 * t)


def lerp(a, b, t):
    return a + (b - a) * t


def _hash(ix, iy, seed):
    n = (ix * 374761393 + iy * 668265263 + seed * 2147483647) & 0xFFFFFFFF
    n = ((n ^ (n >> 13)) * 1274126177) & 0xFFFFFFFF
    return ((n ^ (n >> 16)) & 0xFFFF) / 65535.0


def value_noise(x, y, seed=0):
    """Smooth value noise in [0, 1] with unit cells."""
    ix, iy = math.floor(x), math.floor(y)
    fx, fy = x - ix, y - iy
    fx, fy = fx * fx * (3 - 2 * fx), fy * fy * (3 - 2 * fy)
    a = _hash(ix, iy, seed)
    b = _hash(ix + 1, iy, seed)
    c = _hash(ix, iy + 1, seed)
    d = _hash(ix + 1, iy + 1, seed)
    return lerp(lerp(a, b, fx), lerp(c, d, fx), fy)


def fbm(x, y, octaves=4, seed=0):
    """Fractal noise in roughly [-1, 1]."""
    total, amplitude, norm = 0.0, 1.0, 0.0
    for octave in range(octaves):
        total += amplitude * (value_noise(x, y, seed + octave * 31) * 2 - 1)
        norm += amplitude
        x, y, amplitude = x * 2.03 + 17.1, y * 2.03 - 9.7, amplitude * 0.5
    return total / norm


def _catmull_rom(points, per_metre=1.0):
    """Centripetal Catmull-Rom through the control points, sampled about every 1/per_metre m."""
    out = []
    for i in range(1, len(points) - 2):
        p0, p1, p2, p3 = points[i - 1 : i + 3]

        def knot(t, a, b):
            return t + max(1e-6, math.dist(a, b)) ** 0.5

        t0 = 0.0
        t1 = knot(t0, p0, p1)
        t2 = knot(t1, p1, p2)
        t3 = knot(t2, p2, p3)
        steps = max(2, int(math.dist(p1, p2) * per_metre))
        for k in range(steps):
            t = t1 + (t2 - t1) * k / steps

            def mix(a, b, ta, tb, t=t):
                wa, wb = (tb - t) / (tb - ta), (t - ta) / (tb - ta)
                return (a[0] * wa + b[0] * wb, a[1] * wa + b[1] * wb)

            a1, a2, a3 = mix(p0, p1, t0, t1), mix(p1, p2, t1, t2), mix(p2, p3, t2, t3)
            b1, b2 = mix(a1, a2, t0, t2), mix(a2, a3, t1, t3)
            out.append(mix(b1, b2, t1, t2))
    out.append(points[-2])
    return out


def _catmull_rom_loop(points, per_metre=1.0):
    """The same, closed: the last point runs back into the first. No repeated end sample."""
    wrapped = [points[-1], *list(points), points[0], points[1]]
    return _catmull_rom(wrapped, per_metre)[:-1]


class CoastPath:
    """The path's centreline, its height, its zones, and the ground around it."""

    STEP = 100.0  # cm between centreline samples
    CELL = 2000.0  # cm per spatial-hash cell for locate()

    def __init__(self):
        self.closed = CLOSED
        if self.closed:
            # Starts at the town's (0, 0); CONTROL's first point closes the loop behind it.
            plan = _catmull_rom_loop(CONTROL[1:] + CONTROL[:1], per_metre=1.0)
            plan.append(plan[0])
        else:
            plan = _catmull_rom(CONTROL, per_metre=1.0)
        # Resample to even arc length so s maps straight to a sample index.
        lengths = [0.0]
        for a, b in itertools.pairwise(plan):
            lengths.append(lengths[-1] + math.dist(a, b) * M)
        self.length = lengths[-1]
        self.xs, self.ys = [], []
        j = 0
        if self.closed:
            # A whole number of even steps round the loop, the last running back into the first.
            count = max(3, round(self.length / self.STEP))
            self.STEP = self.length / count
        else:
            count = int(self.length // self.STEP) + 1
        for i in range(count):
            s = i * self.STEP
            while j < len(lengths) - 2 and lengths[j + 1] < s:
                j += 1
            span = lengths[j + 1] - lengths[j] or 1.0
            t = (s - lengths[j]) / span
            self.xs.append(lerp(plan[j][0], plan[j + 1][0], t) * M)
            self.ys.append(lerp(plan[j][1], plan[j + 1][1], t) * M)
        self.count = len(self.xs)
        self.zs = [self.height(i * self.STEP) for i in range(self.count)]
        self.grid = {}
        for i in range(self.count):
            self.grid.setdefault(self._cell(self.xs[i], self.ys[i]), []).append(i)
        self._coast_spans = self.spans("coast")

    # --- along the path -----------------------------------------------------------------

    def _cell(self, x, y):
        return (math.floor(x / self.CELL), math.floor(y / self.CELL))

    def wrap(self, s):
        """s brought onto the path: round the loop, or held to the ends of an open path."""
        if self.closed:
            return s % self.length
        return min(max(s, 0.0), self.length)

    def _index(self, i):
        return i % self.count if self.closed else min(max(i, 0), self.count - 1)

    def height(self, s):
        """Path height at s, smoothly through PROFILE."""
        metres = (s % self.length if self.closed else s) / M
        for (s0, z0), (s1, z1) in itertools.pairwise(PROFILE):
            if metres <= s1:
                t = 0.0 if s1 == s0 else (metres - s0) / (s1 - s0)
                t = t * t * (3 - 2 * t)
                return lerp(z0, z1, max(0.0, t)) * M
        return PROFILE[-1][1] * M

    def at(self, s):
        """(x, y, z, yaw degrees) on the centreline at s."""
        if self.closed:
            s %= self.length
            f = s / self.STEP
            i = min(int(f), self.count - 1)
        else:
            s = min(max(s, 0.0), (self.count - 1) * self.STEP)
            f = s / self.STEP
            i = min(int(f), self.count - 2)
        t = f - i
        n = self._index(i + 1)
        x = lerp(self.xs[i], self.xs[n], t)
        y = lerp(self.ys[i], self.ys[n], t)
        a, b = self._index(i - 2), self._index(i + 3)
        yaw = math.degrees(math.atan2(self.ys[b] - self.ys[a], self.xs[b] - self.xs[a]))
        return x, y, self.height(s), yaw

    def point(self, s, lateral, dz=0.0):
        """World (x, y, z) `lateral` cm right of the centreline at s, at path height + dz."""
        x, y, z, yaw = self.at(s)
        r = math.radians(yaw)
        return x - math.sin(r) * lateral, y + math.cos(r) * lateral, z + dz

    def zone_start(self, name):
        """Where the first stretch of `name` starts, in cm."""
        return self.spans(name)[0][0]

    def spans(self, name):
        """[(start, end)] in cm of every stretch of `name`, in order."""
        out = []
        for k, (zone, start) in enumerate(ZONES):
            if zone == name:
                end = ZONES[k + 1][1] * M if k + 1 < len(ZONES) else self.length
                out.append((start * M, min(end, self.length)))
        return [(a, b) for a, b in out if a < self.length]

    PLOT = 2600.0  # a paddy plot's side, cm

    def paddy_level(self, s):
        """The water level of the paddy plot alongside s: each plot is level, a step below the dike
        at its middle, so where the path climbs the fields climb in terraces. None outside
        paddies."""
        if self.closed:
            s %= self.length
        for start, end in self.spans("paddies"):
            if start <= s < end:
                k = math.floor((s - start - 400) / self.PLOT)
                return self.height(start + 400 + (k + 0.5) * self.PLOT) - 70
        return None

    def zones(self, s):
        """{zone: weight} at s, weights summing to 1, blending over BLEND metres."""
        weights = {}
        metres = (s % self.length if self.closed else s) / M
        for k, (name, start) in enumerate(ZONES):
            end = ZONES[k + 1][1] if k + 1 < len(ZONES) else 1e9
            w = smoothstep(start - BLEND / 2, start + BLEND / 2, metres) if k else 1.0
            w *= 1.0 - smoothstep(end - BLEND / 2, end + BLEND / 2, metres)
            if w > 0:
                weights[name] = weights.get(name, 0.0) + w
        total = sum(weights.values()) or 1.0
        return {k: v / total for k, v in weights.items()}

    def zone(self, s):
        """The zone with the most weight at s."""
        weights = self.zones(s)
        return max(weights, key=weights.get)

    def width(self, s):
        return sum(PATH_WIDTH[k] * w for k, w in self.zones(s).items())

    # --- around the path ----------------------------------------------------------------

    def locate(self, x, y):
        """(s, lateral) of the nearest centreline point; lateral > 0 is right (south)."""
        cx, cy = self._cell(x, y)
        best, best_d = None, float("inf")
        for radius in range(0, 40):
            if radius > 6 and best is None:
                # Far from the path: a coarse scan of every 10th sample beats searching ever-wider
                # rings of empty cells, then the fine samples around the winner.
                coarse = min(
                    range(0, self.count, 10),
                    key=lambda j: (self.xs[j] - x) ** 2 + (self.ys[j] - y) ** 2,
                )
                best = min(
                    range(max(0, coarse - 10), min(self.count, coarse + 11)),
                    key=lambda j: (self.xs[j] - x) ** 2 + (self.ys[j] - y) ** 2,
                )
                break
            for gx in range(cx - radius, cx + radius + 1):
                for gy in range(cy - radius, cy + radius + 1):
                    if max(abs(gx - cx), abs(gy - cy)) != radius:
                        continue
                    for i in self.grid.get((gx, gy), ()):
                        d = (self.xs[i] - x) ** 2 + (self.ys[i] - y) ** 2
                        if d < best_d:
                            best, best_d = i, d
            if best is not None and math.sqrt(best_d) < radius * self.CELL:
                break
        if best is None and self.closed:  # cannot happen past the coarse scan; keep it defined
            best = 0
        if best is None:  # far from everything: project onto the nearest end
            best = (
                0
                if math.dist((x, y), (self.xs[0], self.ys[0]))
                < math.dist((x, y), (self.xs[-1], self.ys[-1]))
                else self.count - 1
            )
        # Refine between the neighbouring samples.
        i = best
        a, b = self._index(i - 1), self._index(i + 1)
        dx, dy = self.xs[b] - self.xs[a], self.ys[b] - self.ys[a]
        seg = dx * dx + dy * dy or 1.0
        t = ((x - self.xs[a]) * dx + (y - self.ys[a]) * dy) / seg
        t = min(1.0, max(0.0, t))
        span = (b - a) % self.count if self.closed else b - a
        s = (a + t * span) * self.STEP
        if self.closed:
            s %= self.length
        px, py = self.xs[a] + dx * t, self.ys[a] + dy * t
        length = math.sqrt(seg)
        lateral = ((x - px) * -dy + (y - py) * dx) / length
        # Past either end the path carries straight on, so lateral stays a distance across it.
        return s, lateral

    def coast_gap(self, s):
        """How far (cm) right of the path the coastline runs at s. Along the coast it wanders
        between about 25 and 70 m; before the coast it swings 600 m out to sea, so the town and
        paddies have no shore."""
        gap = 4600 + 2200 * fbm(s / 30000, 0.5, 3, seed=5)
        coastal = 0.0
        for a, b in self._coast_spans:
            after = 1.0 if b >= self.length else 1 - smoothstep(b, b + 45000, s)
            coastal = max(coastal, smoothstep(a - 45000, a, s) * after)
        return gap + (1 - coastal) * 60000

    def coast_offset(self, x, y, where=None):
        """Signed distance (cm) inland from the coastline: > 0 on land, < 0 at sea."""
        s, lateral = where or self.locate(x, y)
        return self.coast_gap(s) - lateral

    def land(self, x, y, where=None):
        """The landscape's height before the path is cut into it. `where` is locate(x, y), if
        known."""
        s, lateral = where or self.locate(x, y)
        weights = self.zones(s)
        path_z = self.height(s)
        north = max(0.0, -lateral)
        south = max(0.0, lateral)
        rough = fbm(x / 9000, y / 9000, 4, seed=1)
        # Behind the town and paddies: gentle hills rising to the north, a plain to the south.
        valley = (
            0.0
            + 3000 * smoothstep(9000, 40000, north) * (0.6 + 0.4 * rough)
            + 300 * rough * smoothstep(2000, 8000, north + south)
        )
        # The jungle runs up a valley: hills both sides.
        jungle = (
            path_z
            + 5000 * smoothstep(1500, 15000, north)
            + 2500 * smoothstep(2000, 12000, south)
            + 900 * rough
        )
        # The coast: mountain slopes above the path, falling to the sea below it.
        coast = path_z + 12000 * smoothstep(500, 30000, north) ** 0.8 + 1200 * rough
        coast_south = path_z - 300 * smoothstep(0, 4000, south)
        # The village gorge: level lots then a steep wooded slope on the left; on the right the bank
        # drops to the stream and the far side climbs.
        lo, hi = STREAM
        village_north = path_z + smoothstep(VILLAGE_LOTS, VILLAGE_LOTS + 4500, north) ** 1.3 * (
            5500 + 1200 * rough
        )
        bank = min(1.0, max(0.0, (south - BANK) / (lo - BANK)))
        village_south = (
            path_z
            - STREAM_DEPTH * bank * (1 - smoothstep(hi, hi + 350, south))
            + 250 * smoothstep(hi, hi + 350, south)
            + smoothstep(hi + 350, 7000, south) * (4500 + 1000 * rough)
        )
        village = village_north if lateral <= 0 else village_south
        coast = coast if lateral <= 0 else coast_south
        h = (
            valley * (weights.get("town", 0) + weights.get("paddies", 0))
            + jungle * weights.get("jungle", 0)
            + coast * weights.get("coast", 0)
        )
        h += village * weights.get("village", 0)
        # Beyond the valley sides, ridged hills that keep climbing, so no slope ends in a flat top.
        # The sea: land drops to the sea floor across the coastline.
        inland = self.coast_offset(x, y, (s, lateral))
        cliff = 1600 + 900 * fbm(x / 4000, y / 4000, 3, seed=9)
        # They rise only well back from the shore, or the shore cuts them off in a sheer wall.
        h += self._far_hills(x, y, abs(lateral)) * smoothstep(cliff, cliff + 25000, inland)
        # Mountains: a volcano-like peak behind the coast and a long ridge behind the town.
        h += self._mountains(x, y)
        shore = smoothstep(-2500, cliff, inland)
        return lerp(SEA_FLOOR, h, shore)

    def _far_hills(self, x, y, away):
        ridge = 1 - abs(fbm(x / 28000, y / 28000, 4, seed=21))
        swell = 0.5 + 0.5 * fbm(x / 70000, y / 70000, 2, seed=23)
        return smoothstep(12000, 70000, away) * (4000 + 14000 * swell) * ridge**1.5

    def _mountains(self, x, y):
        total = 0.0
        for px, py, height, radius in MOUNTAINS:
            d = math.dist((x, y), (px, py)) / radius
            if d < 1.6:
                ridge = 1 + 0.25 * fbm(x / 20000, y / 20000, 5, seed=3)
                total = max(total, height * ridge * max(0.0, 1 - d) ** 1.6)
        return total

    def ground(self, x, y):
        """Terrain height: the land, levelled into a shelf for the path and blended back out."""
        s, lateral = self.locate(x, y)
        weights = self.zones(s)
        path_z = self.height(s) - 4.0  # the path ribbon lies 4 cm above
        # The village's bank drops from the kerb: no shelf there.
        half = self.width(s) / 2 + SHELF * (1 - 0.8 * weights.get("village", 0))
        across = abs(lateral) - half
        land = self.land(x, y, (s, lateral))
        if across <= 0:
            base = path_z
        else:
            reach = 0.0
            for name, w in weights.items():
                left, right = REACH[name]
                if lateral > 0 and right is None:  # coast: down to the sea, whatever the distance
                    right = max(400.0, self.coast_offset(x, y, (s, lateral)) + across)
                reach += w * (right if lateral > 0 else left)
            t = smoothstep(0, reach, across)
            base = lerp(path_z, land, t)
        # Paddies: level fields sunk below the dike either side of the path.
        paddy = weights.get("paddies", 0)
        if paddy and across > 0:
            level = self.paddy_level(s)
            field = self.height(s) - 70 if level is None else level
            sink = smoothstep(0, 150, across) * (1 - smoothstep(9000, 16000, abs(lateral)))
            base = lerp(base, lerp(base, field, sink), paddy)
        return base

    def slope(self, x, y, step=150.0):
        """Steepness of the ground at (x, y), as rise over run."""
        dx = (self.ground(x + step, y) - self.ground(x - step, y)) / (2 * step)
        dy = (self.ground(x, y + step) - self.ground(x, y - step)) / (2 * step)
        return math.sqrt(dx * dx + dy * dy)


def weather(metres):
    """{lux, elevation, fog, sky, rain} at `metres` along the ride, eased between WEATHER's rows."""
    rows = WEATHER
    if metres <= rows[0][0]:
        row = rows[0][1:]
    elif metres >= rows[-1][0]:
        row = rows[-1][1:]
    else:
        for a, b in itertools.pairwise(rows):
            if metres <= b[0]:
                t = smoothstep(a[0], b[0], metres)
                # Light fades in ratios, so ease lux and fog in log space.
                row = []
                for k, (va, vb) in enumerate(zip(a[1:], b[1:], strict=False)):
                    if k in (0, 2):
                        row.append(math.exp(lerp(math.log(va), math.log(vb), t)))
                    else:
                        row.append(lerp(va, vb, t))
                break
    return dict(zip(WEATHER_KEYS, row, strict=False))


def rain_span():
    """(first, last) metres of full rain, and where it starts and stops fading."""
    wet = [r[0] for r in WEATHER if r[5] >= 1.0]
    before = max(r[0] for r in WEATHER if r[0] < wet[0])
    after = min(r[0] for r in WEATHER if r[0] > wet[-1])
    return before, wet[0], wet[-1], after


# --- the long ride ---------------------------------------------------------------------------

# The order the zones come round in after the first coast, and each stretch's length in metres.
CYCLE = [
    ("jungle", 600, 900),
    ("town", 380, 560),
    ("paddies", 450, 750),
    ("jungle", 550, 850),
    ("village", 280, 380),
    ("coast", 1100, 1700),
]
# How far the heading may swing per 110 m step (degrees), and the height each zone settles at (m).
TURN = {"town": 7, "paddies": 4, "jungle": 18, "village": 5, "coast": 12}
LEVEL = {
    "town": (2, 6),
    "paddies": (0.5, 2),
    "jungle": (12, 22),
    "village": (20, 28),
    "coast": (28, 46),
}
STEP_M = 110.0


def _plan_length(points):
    plan = _catmull_rom(points)
    return sum(math.dist(a, b) for a, b in itertools.pairwise(plan))


def _base_sun(metres, total):
    """The long ride's sun after the rain: high through the afternoon, low and golden at the end."""
    rows = [
        (1200, 50000, 30),
        (1450, 80000, 32),
        (total * 0.5, 85000, 30),
        (total * 0.8, 75000, 18),
        (total, 60000, 7),
    ]
    if metres <= rows[0][0]:
        return rows[0][1:]
    for a, b in itertools.pairwise(rows):
        if metres <= b[0]:
            t = smoothstep(a[0], b[0], metres)
            return (math.exp(lerp(math.log(a[1]), math.log(b[1]), t)), lerp(a[2], b[2], t))
    return rows[-1][1:]


def _turn(a):
    """An angle in degrees brought into (-180, 180]."""
    return (a + 180.0) % 360.0 - 180.0


def _stretches(done, total, rng):
    """[(zone, start, end)] metres: the first coast runs on, the cycle follows, and a town closes
    the loop into the town it began in."""
    stretches = [("coast", ZONES[-1][1], done + rng.uniform(700, 1000))]
    home = rng.uniform(450, 600)
    k = 0
    while stretches[-1][2] < total - home:
        name, lo, hi = CYCLE[k % len(CYCLE)]
        begin = stretches[-1][2]
        stretches.append((name, begin, min(total - home, begin + rng.uniform(lo, hi))))
        k += 1
    if stretches[-1][2] - stretches[-1][1] < 200 and len(stretches) > 2:  # no stub before home
        stretches.pop()
        stretches[-1] = (stretches[-1][0], stretches[-1][1], total - home)
    if stretches[-1][0] == "town":
        stretches[-1] = ("town", stretches[-1][1], total)
    else:
        stretches.append(("town", total - home, total))
    return stretches


def extend_plan(km=LONG_KM, seed=2026):
    """Rewrites CONTROL, PROFILE, ZONES, WEATHER and MOUNTAINS in place for a loop of about `km`."""
    rng = random.Random(seed)
    target = km * 1000.0
    # Keep the hand-laid start up to its last real control point; drop its end phantom. Its first
    # point, 60 m behind the start, becomes the last point of the loop.
    del CONTROL[-1]
    done = _plan_length([*CONTROL, CONTROL[-1]])
    x, y = CONTROL[-1]
    heading = math.degrees(math.atan2(y - CONTROL[-2][1], x - CONTROL[-2][0]))
    # The island: an ellipse twice as wide as deep, its south shore along the hand-laid ride and
    # sized so its rim is a little shorter than the ride, which wanders. Heading east, the rider
    # turns left (north, -Y) round it: east, north, west, south and back east into the town.
    a = target * 0.93 / 4.97
    b = 0.55 * a
    cx, cy = -900.0 + a, 380.0 - b
    first = math.degrees(math.atan2((y - cy) / b, (x - cx) / a))
    marks = []
    angle = first - 25
    while angle > -200:
        marks.append(
            (cx + a * math.cos(math.radians(angle)), cy + b * math.sin(math.radians(angle)))
        )
        angle -= 18
    # Down the west side towards the town; a straight lead-in finishes it, level and heading east.
    marks += [(-820.0, -260.0)]
    lead_in = [(-430.0, -8.0), (-320.0, -2.0), (-200.0, 0.0)]
    # A first guess at the stretches so the wander suits each zone; refitted to the real length.
    guess = _stretches(done, target, random.Random(seed + 1))
    walked = done
    while True:
        zone = next((n for n, s0, s1 in guess if s0 <= walked < s1), "town")
        while marks and math.dist((x, y), marks[0]) < 250:
            marks.pop(0)
        goal = marks[0] if marks else lead_in[0]
        if not marks and math.dist((x, y), lead_in[0]) < 1.9 * STEP_M:
            break
        bearing = math.degrees(math.atan2(goal[1] - y, goal[0] - x))
        steer = max(-14.0, min(14.0, 0.35 * _turn(bearing - heading)))
        wander = rng.uniform(-1, 1) * TURN[zone] * (0.7 if len(marks) > 1 else 0.0)
        heading = _turn(heading + steer + wander)
        x += STEP_M * math.cos(math.radians(heading))
        y += STEP_M * math.sin(math.radians(heading))
        CONTROL.append((round(x, 1), round(y, 1)))
        walked += STEP_M
    CONTROL.extend(lead_in)
    # The first control point stays first: CoastPath starts from CONTROL[1], so it closes the loop.
    loop = _catmull_rom_loop(CONTROL[1:] + CONTROL[:1])
    total = sum(math.dist(p, q) for p, q in zip(loop, loop[1:] + loop[:1], strict=False))
    stretches = _stretches(done, total, random.Random(seed + 1))
    # Zones: the hand-laid ones stand; the generated stretches follow.
    ZONES[:] = [z for z in ZONES] + [(n, s0) for n, s0, s1 in stretches[1:]]
    # Height: each stretch eases to its zone's level across its start, at no more than about 5%,
    # and the last town comes down to the start's level before the loop closes.
    del PROFILE[next(i for i, (s, _) in enumerate(PROFILE) if s > done - 150) :]
    z = PROFILE[-1][1]
    for n, s0, s1 in stretches[1:]:
        last = s1 >= total
        level = 0.0 if last else rng.uniform(*LEVEL[n])
        ramp = max(150.0, abs(level - z) / 0.05)
        if not last:
            ramp = min(ramp, (s1 - s0) * 1.2)
        begin = max(PROFILE[-1][0] + 40, s0 - ramp / 2)
        if last:
            begin = max(PROFILE[-1][0] + 40, min(begin, total - 80 - ramp))
        PROFILE.append((begin, z))
        PROFILE.append((begin + ramp, level))
        s = begin + ramp
        # Long stretches roll a little rather than run dead level.
        while not last and s1 - s > 450:
            s += rng.uniform(300, 420)
            if s < s1 - 150:
                PROFILE.append((s, level + rng.uniform(-1, 1) * (5 if n == "coast" else 2)))
        z = PROFILE[-1][1]
    PROFILE.append((total + 400, 0.0))
    # Weather: the hand-laid sun and rain to the end of the shower, then the base sun, and a mist
    # that settles in each later gorge.
    rows = [r for r in WEATHER if r[0] <= 1110]
    marks = [1200, 1450] + [total * f for f in (0.5, 0.8, 1.0)]
    for n, s0, s1 in stretches[1:]:
        if n == "village":
            marks += [s0 - 180, s0 + 40, s1 - 40, s1 + 180]
    for m in sorted(set(round(v) for v in marks if 1110 < v <= total)):
        lux, elevation = _base_sun(m, total)
        fog, sky = 0.007, 1.0
        for n, s0, s1 in stretches[1:]:
            if n == "village":
                mist = smoothstep(s0 - 180, s0 + 40, m) * (1 - smoothstep(s1 - 40, s1 + 180, m))
                fog, sky, lux = (
                    lerp(fog, 0.05, mist),
                    lerp(sky, 0.65, mist),
                    lerp(lux, lux * 0.3, mist),
                )
        rows.append((m, lux, elevation, fog, sky, 0.0))
    WEATHER[:] = rows
    # The mountains stand inside the loop, clear of the path: one big peak where the island is
    # widest and smaller ones round it.
    MOUNTAINS[:] = _island_peaks(rng)
    return stretches


def _island_peaks(rng):
    """(x, y, height, radius) cm for peaks inside the loop, each ending well short of the path."""
    plan = _catmull_rom_loop(CONTROL[1:] + CONTROL[:1])
    xs, ys = [p[0] for p in plan], [p[1] for p in plan]

    def inside(px, py):
        hit = False
        for (ax, ay), (bx, by) in zip(plan, plan[1:] + plan[:1], strict=False):
            if (ay > py) != (by > py) and px < ax + (py - ay) * (bx - ax) / (by - ay):
                hit = not hit
        return hit

    def clear(px, py):
        return min(math.dist((px, py), p) for p in plan[::4])

    candidates = []
    for gx in range(int(min(xs)), int(max(xs)), 60):
        for gy in range(int(min(ys)), int(max(ys)), 60):
            if inside(gx, gy):
                candidates.append((clear(gx, gy), gx, gy))
    peaks = []
    for _ in range(5):
        best = None
        for room, gx, gy in candidates:
            # Room from the path, and from the peaks already placed.
            for px, py, _, r in peaks:
                room = min(room, math.dist((gx, gy), (px / M, py / M)) - 0.5 * r / M)
            if best is None or room > best[0]:
                best = (room, gx, gy)
        room, gx, gy = best
        if room < 350:
            break
        radius = min(1300.0, room - 160.0)
        height = radius * (0.62 if not peaks else rng.uniform(0.35, 0.5))
        peaks.append((gx * M, gy * M, height * M, radius * M))
    return peaks


STRETCHES = extend_plan() if PLAN == "long" else None
