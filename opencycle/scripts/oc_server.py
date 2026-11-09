"""The warm editor's side of oc.py: runs inside Unreal (started by oc_supervisor.sh), takes job
files from OC_DIR/jobs and runs them, one at a time, a few frames per step.

A job is JOBS/<id>.json, {"op": "look", "args": {...}}. The server renames it <id>.running, runs
oc_climb.op_<op>(**args) and writes <id>.done or <id>.failed (JSON: ok, error, files, notes). If
the editor dies mid-job, the supervisor (or oc.py) renames the .running file <id>.crashed.

An op must be a generator: each `yield n` gives the editor n frames (default 10) before the next
part, and its `return` value (a dict) is the job's result. That makes "split long work across
frames" the only way to write one. oc_climb.py is read afresh for every job, so a code change
needs no editor restart.

Also here: restart and stop (the editor quits after answering, and the supervisor starts it again
on the empty startup map), and state.json, the heartbeat oc.py reads.
"""

import contextlib
import importlib.util
import inspect
import json
import os
import time
import traceback
import uuid

import unreal

DIR = os.environ.get("OC_DIR") or "/workspace/ue/out/oc"
JOBS = f"{DIR}/jobs"
SCRIPTS = "/workspace/ue/scripts"
GAP = 10  # editor frames between steps
LOW_MEMORY_GB = 6.0  # the machine's RAM killer acts near 3 GB free
BOOT = uuid.uuid4().hex[:8]
S = {"gen": None, "wait": 0, "busy": False, "job": None, "op": None, "quit_in": None}
T = {"poll": 0.0, "beat": 0.0, "warned": 0.0}


def log(message):
    unreal.log(f"OC {message}")


def write(path, data):
    tmp = f"{path}.tmp"
    with open(tmp, "w") as f:
        json.dump(data, f)
    os.chmod(tmp, 0o666)
    os.replace(tmp, path)


def heartbeat(force=False):
    if not force and time.time() - T["beat"] < 2.0:
        return
    T["beat"] = time.time()
    try:
        world = unreal.UnrealEditorSubsystem().get_editor_world()
        level = world.get_path_name() if world else ""
    except Exception:
        level = "?"
    write(
        f"{DIR}/state.json",
        {"boot": BOOT, "pid": os.getpid(), "time": time.time(), "job": S["job"], "map": level},
    )


def memory_gb():
    with open("/proc/meminfo") as f:
        for line in f:
            if line.startswith("MemAvailable:"):
                return int(line.split()[1]) / 1e6
    return 99.0


def finish(ok, **result):
    jid = S["job"]
    result.update(ok=ok, id=jid, op=S["op"])
    write(f"{JOBS}/{jid}.{'done' if ok else 'failed'}", result)
    with contextlib.suppress(FileNotFoundError):
        os.remove(f"{JOBS}/{jid}.running")
    log(f"{'done' if ok else 'FAILED'} {jid}")
    S.update(gen=None, job=None, op=None)


def load_ops():
    spec = importlib.util.spec_from_file_location("oc_climb", f"{SCRIPTS}/oc_climb.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def start(jid):
    running = f"{JOBS}/{jid}.running"
    os.replace(f"{JOBS}/{jid}.json", running)
    S["job"] = jid
    log(f"run {jid}")
    with open(running) as f:
        job = json.load(f)
    S["op"] = op = job["op"]
    if op in ("restart", "stop"):
        finish(True, notes=[f"{op}: the editor quits now"])
        S["quit_in"] = 30
        return
    fn = getattr(load_ops(), "op_" + op.replace("-", "_"), None)
    if fn is None:
        raise RuntimeError(f"no op {op!r}")
    gen = fn(**job.get("args", {}))
    if not inspect.isgenerator(gen):
        raise RuntimeError(f"op {op!r} is not a generator; ops must yield between steps")
    S["gen"] = gen


def tick(_delta):
    # A step can pump the editor's frames itself (material builds), which calls this again inside
    # it: skip those.
    if S["busy"]:
        return
    S["busy"] = True
    try:
        heartbeat()
        if S["quit_in"] is not None:
            S["quit_in"] -= 1
            if S["quit_in"] <= 0:
                write(f"{DIR}/quitting", {})  # tells the supervisor this exit is on purpose
                S["quit_in"] = 10**9
                unreal.SystemLibrary.quit_editor()
        elif S["wait"] > 0:
            S["wait"] -= 1
        elif S["gen"] is not None:
            try:
                frames = next(S["gen"])
                S["wait"] = GAP if frames is None else int(frames)
            except StopIteration as stop:
                finish(True, **(stop.value or {}))
        elif time.time() - T["poll"] > 1.0:
            T["poll"] = time.time()
            waiting = sorted(n for n in os.listdir(JOBS) if n.endswith(".json"))
            if waiting and memory_gb() < LOW_MEMORY_GB:
                if time.time() - T["warned"] > 60:
                    T["warned"] = time.time()
                    log(f"waiting: {memory_gb():.1f} GB free, under {LOW_MEMORY_GB}")
            elif waiting:
                start(waiting[0][:-5])
    except Exception:
        if S["job"]:
            finish(False, error=traceback.format_exc())
        else:
            unreal.log_error(f"OC {traceback.format_exc()}")
    finally:
        S["busy"] = False


os.makedirs(JOBS, exist_ok=True)
# -ExecutePythonScript closes the editor on the first tick after the script returns unless told not
# to.
unreal.EditorPythonScripting.set_keep_python_script_alive(True)
unreal.register_slate_post_tick_callback(tick)
heartbeat(force=True)
log(f"ready, boot {BOOT}, watching {JOBS}")
