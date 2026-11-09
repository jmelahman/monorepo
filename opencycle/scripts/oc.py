#!/usr/bin/env python3
"""The one entry point to the climb's Unreal pipeline. Synchronous: it returns only when the work
is done, failed, crashed, or a hard timeout passed, and says which, in the last line.

    python3 scripts/oc.py status
    python3 scripts/oc.py look --tint Dirt=0.58,0.6,0.63 [--grey Dirt=0.8] [--shots road_50]
    python3 scripts/oc.py stills [--shots road_08,hairpin_above]
    python3 scripts/oc.py rebuild-ground [--no-stills] [--rebuild-grass]
    python3 scripts/oc.py wait JOB | log [N] | restart | stop

It makes sure one supervised warm editor is up (oc_supervisor.sh runs oc_server.py in it; a second
Unreal is never started), writes a job file, and polls for the result. No log watcher to forget.
Exit status: 0 done, 1 failed (traceback printed), 2 crashed (log tail printed), 3 timed out (the
job is NOT cancelled: `oc.py wait JOB` resumes), 4 no usable editor.

--timeout is the limit in seconds for waiting on the job (default 360), --boot-timeout for the
editor to start (default 300). rebuild-ground is two jobs, each with its own limit: the rebuild,
on an editor restarted onto the empty map, then stills (the first opening of the level after a
rebuild compiles the landscape's shaders, which can take minutes).
"""

import argparse
import fcntl
import json
import os
import re
import subprocess
import sys
import time

DIR = os.environ.get("OC_DIR") or "/workspace/ue/out/oc"
JOBS = f"{DIR}/jobs"
STATE = f"{DIR}/state.json"
LOG = f"{DIR}/editor.log"
SUPERVISOR = "/workspace/ue/scripts/oc_supervisor.sh"
OK, FAILED, CRASHED, TIMEOUT, NO_EDITOR = 0, 1, 2, 3, 4
# What in a log tail says why an editor died. (r.GPUCrashDebugging is a config line, not a crash.)
CRASH_WORDS = re.compile(
    r"Fatal error|DEVICE_LOST|SIGSEGV|signal 11|Assertion failed|Ensure condition|out of memory"
    r"|Killed|VK_ERROR|GPU crash|double free|Unhandled Exception",
    re.IGNORECASE,
)


def say(message):
    print(message, flush=True)


# --- pure helpers (tested in test_oc.py) -----------------------------------------------------


def parse_tint(text):
    """'Dirt=0.58,0.6,0.63' -> ('Dirt', [0.58, 0.6, 0.63])."""
    name, sep, rgb = text.rpartition("=")
    try:
        values = [float(v) for v in rgb.split(",")]
    except ValueError:
        values = []
    if not sep or not name.strip() or len(values) != 3:
        raise ValueError(f"tint {text!r}: expected NAME=R,G,B, e.g. Dirt=0.58,0.6,0.63")
    return name.strip(), values


def parse_grey(text):
    """'Dirt=0.8' -> ('Dirt', 0.8): how far that surface's colour goes to grey, 0 to 1."""
    name, sep, amount = text.rpartition("=")
    try:
        value = float(amount)
    except ValueError:
        value = -1.0
    if not sep or not name.strip() or not 0.0 <= value <= 1.0:
        raise ValueError(f"grey {text!r}: expected NAME=F with F from 0 to 1, e.g. Dirt=0.8")
    return name.strip(), value


def clean_log(raw):
    """Log text from bytes: NUL bytes removed (the editor log holds them), undecodable bytes
    replaced, and the r.GPUCrashDebugging config lines (a false 'crash' match) dropped."""
    text = raw.replace(b"\0", b"").decode("utf-8", errors="replace")
    return [line for line in text.splitlines() if "r.GPUCrash" not in line]


def crash_lines(lines):
    return [line for line in lines if CRASH_WORDS.search(line)]


def job_status(names, jid):
    """queued, running, done, failed, crashed or None, from the file names in the jobs folder."""
    for status, suffix in (
        ("done", "done"),
        ("failed", "failed"),
        ("crashed", "crashed"),
        ("running", "running"),
        ("queued", "json"),
    ):
        if f"{jid}.{suffix}" in names:
            return status
    return None


# --- the editor ------------------------------------------------------------------------------


def tail(path, count=60):
    try:
        with open(path, "rb") as f:
            f.seek(0, os.SEEK_END)
            f.seek(max(0, f.tell() - 400_000))
            return clean_log(f.read())[-count:]
    except OSError:
        return []


def editor_pids():
    out = subprocess.run(["pgrep", "-x", "UnrealEditor"], capture_output=True, text=True)
    cmd = subprocess.run(["pgrep", "-x", "UnrealEditor-Cmd"], capture_output=True, text=True)
    return [int(p) for p in (out.stdout + cmd.stdout).split()]


def read_state():
    try:
        with open(STATE) as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


def our_server(pids):
    """The heartbeat of our server if it belongs to a live editor, else None."""
    state = read_state()
    return state if state and state.get("pid") in pids else None


def supervisor_running():
    os.makedirs(DIR, exist_ok=True)
    with open(f"{DIR}/supervisor.lock", "a") as f:
        try:
            fcntl.flock(f, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            return True
        return False


def start_supervisor():
    os.makedirs(JOBS, exist_ok=True)
    os.chmod(DIR, 0o777)
    os.chmod(JOBS, 0o777)
    with open(f"{DIR}/supervisor.log", "a") as log:
        subprocess.Popen(
            ["bash", SUPERVISOR],
            stdout=log,
            stderr=log,
            stdin=subprocess.DEVNULL,
            start_new_session=True,
            env={**os.environ, "OC_DIR": DIR},
        )


def mark_crashed():
    """A .running job whose editor is gone is .crashed, with the log tail (the supervisor does
    this too; whichever is first)."""
    for name in os.listdir(JOBS):
        if name.endswith(".running"):
            base = f"{JOBS}/{name[:-8]}"
            with open(f"{base}.crashed", "w") as f:
                f.write("\n".join(tail(LOG, 80)) + "\n")
            os.chmod(f"{base}.crashed", 0o666)
            os.remove(f"{base}.running")


def ensure_editor(boot_timeout):
    """Returns the server's state once a supervised editor of ours is up, or exits with NO_EDITOR.
    Never starts an Unreal itself, and the supervisor never starts a second."""
    os.makedirs(JOBS, exist_ok=True)
    deadline = time.time() + boot_timeout
    started = 0.0  # when we started the supervisor
    last = 0.0
    while True:
        pids = editor_pids()
        state = our_server(pids)
        if state:
            return state
        up = supervisor_running()
        if pids and not up:
            say(
                f"NO EDITOR: an Unreal (pid {', '.join(map(str, pids))}) is running that this "
                "pipeline does not control (no oc server heartbeat, no supervisor). Not starting "
                "a second one. Quit it (the old server: touch /workspace/ue/out/cmd/quit.py) "
                "and run this again."
            )
            sys.exit(NO_EDITOR)
        if not up:
            if started and time.time() - started > 15:
                say("NO EDITOR: the supervisor stopped (three exits in ten minutes?). Its log:")
                say(supervisor_tail())
                say("Editor log tail:\n" + "\n".join(tail(LOG, 40)))
                sys.exit(NO_EDITOR)
            if not started:
                if not pids:
                    mark_crashed()
                say("starting the supervised editor (a minute or two to load)")
                start_supervisor()
                started = time.time()
        if time.time() > deadline:
            say(f"NO EDITOR: not ready after {boot_timeout:.0f} s. Editor log tail:")
            say("\n".join(tail(LOG, 40)))
            sys.exit(NO_EDITOR)
        if time.time() - last > 20:
            last = time.time()
            say(f"waiting for the editor ({int(boot_timeout - (deadline - time.time()))} s)")
        time.sleep(2)


def supervisor_tail():
    try:
        with open(f"{DIR}/supervisor.log", "rb") as f:
            return "\n".join(clean_log(f.read())[-15:])
    except OSError:
        return "(no supervisor log)"


# --- jobs ------------------------------------------------------------------------------------


def submit(op, args=None):
    jid = f"{int(time.time() * 1000)}_{op.replace('-', '_')}"
    tmp = f"{JOBS}/{jid}.tmp"
    with open(tmp, "w") as f:
        json.dump({"op": op, "args": args or {}}, f)
    os.chmod(tmp, 0o666)
    os.replace(tmp, f"{JOBS}/{jid}.json")
    return jid


def report(jid, status):
    base = f"{JOBS}/{jid}"
    if status == "crashed":
        with open(f"{base}.crashed", errors="replace") as f:
            lines = clean_log(f.read().encode())
        hints = crash_lines(lines)
        say(f"CRASHED: the editor died running {jid}. Likely lines:")
        say("\n".join(hints[-10:]) or "(none matched; see the tail)")
        say("--- log tail ---\n" + "\n".join(lines[-40:]))
        say("The supervisor is restarting the editor on the empty map.")
        return CRASHED
    with open(f"{base}.{status}") as f:
        result = json.load(f)
    if status == "failed":
        say(f"FAILED: {jid}\n{result.get('error', '(no traceback)')}")
        return FAILED
    for note in result.get("notes", []):
        say(note)
    missing = [p for p in result.get("files", []) if not os.path.exists(p)]
    for path in result.get("files", []):
        say(f"still {path}")
    if missing:
        say(f"FAILED: reported but missing: {missing}")
        return FAILED
    say(f"DONE: {jid}")
    return OK


def wait(jid, timeout):
    """Blocks until the job has a result, its editor died, or `timeout` seconds pass."""
    start = time.time()
    beat = start
    gone = 0
    while True:
        names = set(os.listdir(JOBS))
        status = job_status(names, jid)
        if status in ("done", "failed", "crashed"):
            return report(jid, status)
        if status is None:
            say(f"FAILED: no job {jid}")
            return FAILED
        pids = editor_pids()
        gone = 0 if pids else gone + 1
        if gone >= 5 and status == "running":
            time.sleep(3)  # the supervisor marks it; otherwise do it here
            if job_status(set(os.listdir(JOBS)), jid) == "running":
                mark_crashed()
            continue
        if time.time() - start > timeout:
            state = our_server(pids) or {}
            age = time.time() - state.get("time", 0)
            say(
                f"TIMEOUT: {jid} is still {status} after {timeout:.0f} s (editor "
                f"{'up' if pids else 'DOWN'}, heartbeat {age:.0f} s old"
                f"{', frozen?' if pids and age > 120 else ''}). Not cancelled. Resume with "
                f"`oc.py wait {jid}`; `oc.py log` shows the editor; `oc.py restart` abandons it. "
                "A first open of the level after a ground rebuild compiles the landscape's "
                "shaders for minutes."
            )
            return TIMEOUT
        if time.time() - beat > 30:
            beat = time.time()
            notes = [ln for ln in tail(LOG, 200) if "OC " in ln]
            say(
                f"... {jid} {status}, {time.time() - start:.0f} s"
                + (f"; last: {notes[-1].strip()[-120:]}" if notes else "")
            )
        time.sleep(1)


def run(op, args, ns):
    ensure_editor(ns.boot_timeout)
    return wait(submit(op, args), ns.timeout)


def restart_clean(ns):
    """Restart the editor onto the empty startup map and wait until it is back."""
    state = ensure_editor(ns.boot_timeout)
    say("restarting the editor onto the empty map")
    code = wait(submit("restart", {}), 60)
    if code != OK:
        sys.exit(code)
    deadline = time.time() + ns.boot_timeout
    while time.time() < deadline:
        new = our_server(editor_pids())
        if new and new["boot"] != state["boot"]:
            return new
        time.sleep(2)
    say("NO EDITOR: it did not come back after the restart. Editor log tail:")
    say("\n".join(tail(LOG, 40)))
    sys.exit(NO_EDITOR)


# --- commands --------------------------------------------------------------------------------


def cmd_status(ns):
    pids = editor_pids()
    state = our_server(pids)
    say(f"editor pids: {pids or 'none'}; supervisor: {'up' if supervisor_running() else 'down'}")
    if state:
        say(
            f"our server: boot {state['boot']}, map {state['map'] or '(none)'}, "
            f"heartbeat {time.time() - state['time']:.0f} s ago, job {state['job']}"
        )
    elif pids:
        say("the running Unreal is not ours (no oc heartbeat)")
    os.makedirs(JOBS, exist_ok=True)
    names = set(os.listdir(JOBS))
    ids = sorted({n.split(".")[0] for n in names if n.split(".")[-1] != "tmp"})[-5:]
    for jid in ids:
        say(f"job {jid}: {job_status(names, jid)}")
    say("log tail:\n" + "\n".join(tail(LOG, 5)))
    return OK


def parse_post(text):
    """`exposure=-0.5` as ("exposure", -0.5): see oc_climb.POST_KEYS."""
    key, _, value = text.partition("=")
    try:
        return key.strip(), float(value)
    except ValueError:
        raise SystemExit(f"--post {text!r}: use KEY=NUMBER") from None


def cmd_look(ns):
    tints = dict(parse_tint(t) for t in ns.tint or [])
    greys = dict(parse_grey(t) for t in ns.grey or [])
    args = {
        "tint": tints,
        "grey": greys,
        "grass": ns.grass,
        "fog": ns.fog,
        "post": dict(parse_post(t) for t in ns.post or []),
        "stills": not ns.no_stills,
        "shots": shots(ns),
    }
    return run("look", args, ns)


def cmd_stills(ns):
    return run("stills", {"shots": shots(ns)}, ns)


def cmd_rebuild_ground(ns):
    restart_clean(ns)  # no landscape is loaded on a fresh editor: the one safe time to rebuild
    code = wait(submit("rebuild-ground", {"rebuild_grass": ns.rebuild_grass}), ns.timeout)
    if code != OK or ns.no_stills:
        return code
    return wait(submit("stills", {"shots": shots(ns)}), ns.timeout)


def cmd_probe(ns):
    return run("probe", {"path": os.path.abspath(ns.path)}, ns)


def cmd_fps(ns):
    args = {"quality": ns.quality, "frames": ns.frames, "shots": shots(ns)}
    args.update(tree_cull=ns.tree_cull, tree_shadows=ns.tree_shadows, tree_wpo=ns.tree_wpo)
    return run("fps", args, ns)


def cmd_static_trees(ns):
    restart_clean(ns)  # no level loaded: the skeletal trees aren't in use
    return wait(submit("static-trees", {}), ns.timeout)


def cmd_build_level(ns):
    """The whole level from world_gen/climb (run make_climb.py first): each stage on a freshly
    restarted editor, since the ground stage deletes L_Hero and the dress stage loads it."""
    for stage in ("ground", "dress"):
        restart_clean(ns)
        code = wait(submit("build-level", {"stage": stage}), ns.timeout)
        if code != OK:
            return code
    return OK


def cmd_wait(ns):
    return wait(ns.job, ns.timeout)


def cmd_log(ns):
    say("\n".join(tail(LOG, ns.lines)))
    return OK


def cmd_restart(ns):
    restart_clean(ns)
    say("DONE: the editor is back on the empty map")
    return OK


def cmd_stop(ns):
    if not editor_pids() or not our_server(editor_pids()):
        say("no editor of ours is running")
        return OK
    open(f"{DIR}/stop", "w").close()
    os.chmod(f"{DIR}/stop", 0o666)
    code = wait(submit("stop", {}), 60)
    say("DONE: the editor and supervisor are stopping" if code == OK else "stop failed")
    return code


def shots(ns):
    return [s for s in (getattr(ns, "shots", None) or "").split(",") if s] or None


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawTextHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)

    def add(name, fn, **kw):
        s = sub.add_parser(name, help=kw.pop("help", None))
        s.set_defaults(fn=fn)
        s.add_argument("--timeout", type=float, default=360.0)
        s.add_argument("--boot-timeout", type=float, default=300.0)
        return s

    add("status", cmd_status, help="what is running; starts nothing")
    s = add("look", cmd_look, help="set ground tints (parameters only), then stills")
    s.add_argument("--tint", action="append", metavar="NAME=R,G,B")
    s.add_argument("--grey", action="append", metavar="NAME=F", help="0 keeps the hue, 1 is grey")
    s.add_argument(
        "--grass", nargs=2, type=float, metavar=("DENSITY", "CULL"), help="factors on the table"
    )
    s.add_argument(
        "--fog",
        type=float,
        nargs="+",
        metavar="F",
        help="height fog: DENSITY [FALLOFF]; the defaults are 0.02 and 0.2",
    )
    s.add_argument(
        "--post",
        action="append",
        metavar="KEY=F",
        help="light and grade: exposure, contrast, saturation, vignette, fog_start (m),"
        " volumetric (0/1), shafts, sky, sun (see oc_climb.POST_KEYS)",
    )
    s.add_argument("--no-stills", action="store_true")
    s.add_argument("--shots")
    s = add("stills", cmd_stills, help="stills from the editor viewport")
    s.add_argument("--shots")
    s = add("rebuild-ground", cmd_rebuild_ground, help="restart on the empty map, rebuild, stills")
    s.add_argument("--no-stills", action="store_true")
    s.add_argument("--rebuild-grass", action="store_true")
    s.add_argument("--shots")
    s = add("build-level", cmd_build_level, help="rebuild L_Hero from world_gen/climb")
    s.set_defaults(timeout=1800.0)  # the ground stage imports the landscape: minutes
    add("static-trees", cmd_static_trees, help="static Nanite copies of the PVE trees")
    s = add("probe", cmd_probe, help="run a read-only inspection script from out/qa")
    s.add_argument("path")
    s = add("fps", cmd_fps, help="frame times at each still's view (quality 0 Low .. 3 Epic)")
    s.add_argument("--quality", type=int, default=0)
    s.add_argument("--frames", type=int, default=240)
    s.add_argument("--tree-cull", type=float, help="tree draw distance in m for this run")
    s.add_argument("--tree-shadows", type=int, choices=(0, 1))
    s.add_argument("--tree-wpo", type=int, choices=(0, 1), help="tree wind (world position offset)")
    s.add_argument("--shots")
    s = add("wait", cmd_wait, help="resume waiting for a job after a timeout")
    s.add_argument("job")
    s = add("log", cmd_log, help="the editor log tail")
    s.add_argument("lines", nargs="?", type=int, default=60)
    add("restart", cmd_restart, help="restart the editor onto the empty map")
    add("stop", cmd_stop, help="stop the editor and the supervisor")
    ns = p.parse_args(argv)
    try:
        code = ns.fn(ns)
    except ValueError as e:
        say(f"error: {e}")
        code = FAILED
    sys.exit(code)


if __name__ == "__main__":
    main()
