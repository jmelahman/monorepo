"""Summarises bench.sh's CSVs: frame, game, render and GPU times after a warm-up, as mean and
percentiles. Usage: python3 bench_summary.py out/bench/*.csv [--skip=300]"""

import csv
import sys

skip = 300
paths = []
for arg in sys.argv[1:]:
    if arg.startswith("--skip="):
        skip = int(arg.split("=", 1)[1])
    else:
        paths.append(arg)


def pct(values, p):
    values = sorted(values)
    return values[min(len(values) - 1, int(p / 100 * len(values)))]


for path in paths:
    with open(path, newline="") as f:
        rows = [r for r in csv.reader(f)]
    header = rows[0]
    data = [r for r in rows[1:] if len(r) == len(header) and r[0][:1].isdigit()][skip:]
    print(f"{path}: {len(data)} frames after skipping {skip}")
    for column in ("FrameTime", "GameThreadTime", "RenderThreadTime", "GPUTime", "RHIThreadTime"):
        if column not in header:
            continue
        i = header.index(column)
        values = [float(r[i]) for r in data if r[i]]
        if not values:
            continue
        mean = sum(values) / len(values)
        line = (
            f"  {column:17s} mean {mean:6.2f} ms  p50 {pct(values, 50):6.2f}  "
            f"p95 {pct(values, 95):6.2f}  p99 {pct(values, 99):6.2f}"
        )
        if column == "FrameTime":
            line += f"   -> {1000 / mean:5.1f} fps mean, {1000 / pct(values, 95):5.1f} fps at p95"
        print(line)
