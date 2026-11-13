"""How long the queue's builds take, step by step, from their logs.

    python scripts/job_times.py [count]

Each oc.py step logs an id of its start time in ms ("1794609455057_look"); a job's end is its log's
last write. Prints the last `count` jobs (default 6), the running one included.
"""

import os
import re
import sys
import time

QUEUE = "/workspace/ue/out/oc/queue"
STEP = re.compile(r"\b(\d{13})_([a-z_-]+)")


def steps(path):
    seen = {}
    with open(path, errors="replace") as f:
        for stamp, name in STEP.findall(f.read()):
            seen.setdefault(stamp, name)
    return sorted((int(s) / 1000, n) for s, n in seen.items() if n != "restart")


def main():
    count = int(sys.argv[1]) if len(sys.argv) > 1 else 6
    logs = [os.path.join(QUEUE, "done", n) for n in os.listdir(f"{QUEUE}/done")]
    logs += [os.path.join(QUEUE, n) for n in os.listdir(QUEUE)]
    logs = sorted((p for p in logs if p.endswith(".log")), key=os.path.getmtime)[-count:]
    for path in logs:
        found = steps(path)
        if not found:
            continue
        running = os.path.dirname(path) == QUEUE
        end = time.time() if running else os.path.getmtime(path)
        start = found[0][0]
        name = os.path.basename(path)[:-4]
        state = "running" if running else "done"
        print(
            f"{name:28} {time.strftime('%m-%d %H:%M', time.localtime(start))}  "
            f"{(end - start) / 60:5.1f} min  {state}"
        )
        for (t, step), (t_next, _) in zip(found, [*found[1:], (end, "")], strict=True):
            print(f"    {step:16} {(t_next - t) / 60:5.1f} min")


if __name__ == "__main__":
    main()
