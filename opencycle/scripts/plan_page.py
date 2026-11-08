"""Writes out/long_plan.html: the long ride from above, by zone, with its height and weather.

OC_PLAN=long OC_KM=10 python3 -I scripts/plan_page.py
"""

import html
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import coast_layout as L

COLOURS = {
    "town": "#d9a066",
    "paddies": "#9bc53d",
    "jungle": "#2e8b57",
    "village": "#c0392b",
    "coast": "#3d9be9",
}
PATH = L.CoastPath()
km = PATH.length / 100000
W = 1400
xs, ys = [x / 100 for x in PATH.xs], [y / 100 for y in PATH.ys]
pad = 900 if PATH.closed else 0
x0, x1 = min(xs) - 200 - pad, max(xs) + 200 + pad
y0, y1 = min(ys) - 700 - (pad if PATH.closed else 0), max(ys) + 900
scale = W / (x1 - x0)
H = int((y1 - y0) * scale)


def px(x, y):
    return (x - x0) * scale, (y - y0) * scale


land, sea = "#5d7a4a", "#1f5f8b"
parts = [
    f'<svg viewBox="0 0 {W} {H}" width="100%" '
    f'style="background:{sea if PATH.closed else land};border-radius:6px">'
]
# The sea: sample the coastline every 25 m along the path.
shore = []
for i in range(0, PATH.count, 25):
    s = i * PATH.STEP
    x, y, _ = PATH.point(s, PATH.coast_gap(s))
    shore.append(px(x / 100, y / 100))
poly = " ".join(f"{a:.1f},{b:.1f}" for a, b in shore)
if PATH.closed:  # an island: the coastline runs right round, outside the loop
    parts.append(f'<polygon points="{poly}" fill="{land}"/>')
else:
    parts.append(f'<polygon points="0,{H} {poly} {W},{H}" fill="{sea}" opacity="0.9"/>')
for mx, my, h, r in L.MOUNTAINS:
    cx, cy = px(mx / 100, my / 100)
    parts.append(
        f'<circle cx="{cx:.0f}" cy="{cy:.0f}" r="{r / 100 * scale:.0f}" '
        'fill="#4a5a3a" opacity="0.6"/>'
        f'<text x="{cx:.0f}" y="{cy:.0f}" fill="#eee" '
        f'font-size="12" text-anchor="middle">▲ {h / 100:.0f} m</text>'
    )
step = 10
for i in range(0, PATH.count - (0 if PATH.closed else step), step):
    s = i * PATH.STEP
    j = (i + step) % PATH.count
    a, b = px(xs[i], ys[i]), px(xs[j], ys[j])
    parts.append(
        f'<line x1="{a[0]:.1f}" y1="{a[1]:.1f}" x2="{b[0]:.1f}" y2="{b[1]:.1f}" '
        f'stroke="{COLOURS[PATH.zone(s)]}" stroke-width="5" stroke-linecap="round"/>'
    )
for k in range(0, int(km) + 1):
    i = min(PATH.count - 1, int(k * 100000 / PATH.STEP))
    a = px(xs[i], ys[i])
    parts.append(
        f'<circle cx="{a[0]:.1f}" cy="{a[1]:.1f}" r="4" fill="#fff"/>'
        f'<text x="{a[0]:.1f}" y="{a[1] - 9:.1f}" '
        f'fill="#fff" font-size="12" text-anchor="middle">{k} km</text>'
    )
parts.append("</svg>")

# Height and weather strips, metres along the ride.
PH = 160
length_m = PATH.length / 100
pts = " ".join(
    f"{m / length_m * W:.1f},{PH - 10 - PATH.height(m * 100) / 100 * 2.6:.1f}"
    for m in range(0, int(length_m), 20)
)
strip = [
    f'<svg viewBox="0 0 {W} {PH + 70}" width="100%" style="background:#1b1b1b;border-radius:6px">'
]
for name, start in L.ZONES:
    x = start / length_m * W
    strip.append(
        f'<line x1="{x:.0f}" y1="0" x2="{x:.0f}" y2="{PH}" '
        f'stroke="{COLOURS[name]}" stroke-width="2"/>'
        f'<text x="{x + 3:.0f}" y="14" fill="{COLOURS[name]}" font-size="12">{name}</text>'
    )
strip.append(f'<polyline points="{pts}" fill="none" stroke="#eee" stroke-width="2"/>')
for m in range(0, int(length_m), 25):
    w = L.weather(m)
    x = m / length_m * W
    sun = min(1.0, w["lux"] / 90000) * min(1.0, w["elevation"] / 30)
    colour = f"rgb({int(60 + 190 * sun)},{int(70 + 160 * sun)},{int(110 + 60 * sun)})"
    if w["rain"] > 0.5:
        colour = "#3b4a63"
    elif w["fog"] > 0.03:
        colour = "#8a96a3"
    strip.append(
        f'<rect x="{x:.1f}" y="{PH + 8}" width="{25 / length_m * W + 0.6:.1f}" '
        f'height="22" fill="{colour}"/>'
    )
for k in range(0, int(km) + 1):
    x = k * 1000 / length_m * W
    strip.append(f'<text x="{x:.0f}" y="{PH + 50}" fill="#aaa" font-size="12">{k} km</text>')
strip.append("</svg>")

rows = "".join(
    f"<tr><td style='color:{COLOURS[n]}'>{html.escape(n)}</td><td>{a / 100000:.2f}</td>"
    f"<td>{(b - a) / 100:.0f} m</td></tr>"
    for name in COLOURS
    for n, (a, b) in ((name, sp) for sp in PATH.spans(name))
)
page = f"""<!doctype html><meta charset=utf-8><title>Long ride plan</title>
<style>body{{background:#111;color:#ddd;font:14px sans-serif;margin:16px;max-width:1440px}}td{{padding:2px 12px}}a{{color:#8cf}}</style>
<h1>The long ride, {km:.1f} km{" loop" if PATH.closed else ""} <small style=color:#888>· seeded plan, the first 2.1 km as before · <a href=coast.html>stills</a> · <a href=clips.html>clips</a></small></h1>
<p>From above, north up: the route coloured by zone, {"a lap of an island ridden anticlockwise, the sea always on the right and the peaks inside" if PATH.closed else "the sea to the south, peaks behind"}. Below it: height along the ride
(×2.6 vertical) and the sky (blue-grey is rain, grey is mist, then the sun lowering toward evening).</p>
{"".join(parts)}<h2>Height and weather</h2>{"".join(strip)}
<h2>Stretches</h2><table><tr><th>zone</th><th>starts (km)</th><th>length</th></tr>{rows}</table>
"""  # noqa: E501, RUF001
out = os.path.join("/workspace/ue/out", os.environ.get("OC_PAGE") or "long_plan.html")
with open(out, "w") as f:
    f.write(page)
print(out)
