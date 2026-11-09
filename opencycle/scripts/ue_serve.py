"""Keeps one editor open and runs the Python files dropped into a folder, so each change is a
command to a warm editor instead of a fresh start (which costs minutes of loading and building).

Start it once, with a light startup map (the project's default, the coast town, builds for
minutes on every start):
    OC_EDITOR=1 OC_START_MAP=/Engine/Maps/Templates/OpenWorld \\
        bash scripts/ue_run.sh ue_serve.py serve.log

Then write a command, e.g. out/cmd/0001_stills.py. The server runs it in the editor and renames it
to .done (or .failed, with the traceback in .failed.txt). A command may define steps(), a
generator: each yield gives the editor frames (10, or the number yielded) before the next part, as
climb_world.py's build does. A command named quit.py stops the server and the editor.

OC_CMD (default /workspace/ue/out/cmd) is the folder.
"""

import os
import time
import traceback

import unreal

CMD = os.environ.get("OC_CMD") or "/workspace/ue/out/cmd"
GAP = 10
state = {"steps": None, "wait": 0, "busy": False, "name": None, "last": 0.0}


def log(message):
    unreal.log(f"SERVE {message}")


def done(name, ok, detail=""):
    path = f"{CMD}/{name}"
    os.replace(path, f"{path[:-3]}.{'done' if ok else 'failed'}")
    if detail:
        with open(f"{path[:-3]}.failed.txt", "w") as f:
            f.write(detail)
    log(f"{'done' if ok else 'FAILED'} {name}")
    state["steps"], state["name"] = None, None


def start(name):
    log(f"run {name}")
    if name == "quit.py":
        os.remove(f"{CMD}/{name}")
        unreal.unregister_slate_post_tick_callback(handle)
        unreal.SystemLibrary.quit_editor()
        return
    scope = {"__file__": f"{CMD}/{name}", "__name__": "__oc_command__"}
    with open(f"{CMD}/{name}") as f:
        exec(compile(f.read(), f"{CMD}/{name}", "exec"), scope)
    if "steps" in scope:
        state["steps"], state["name"] = scope["steps"](), name
    else:
        done(name, True)


def tick(_delta):
    # A command can pump the editor's frames itself, which calls this again inside it: skip.
    if state["busy"]:
        return
    if state["wait"] > 0:
        state["wait"] -= 1
        return
    state["busy"] = True
    try:
        if state["steps"] is not None:
            try:
                frames = next(state["steps"])
                state["wait"] = GAP if frames is None else frames
            except StopIteration:
                done(state["name"], True)
        elif time.time() - state["last"] > 1.0:
            state["last"] = time.time()
            waiting = sorted(n for n in os.listdir(CMD) if n.endswith(".py"))
            if waiting:
                name = waiting[0]
                try:
                    start(name)
                except Exception:
                    done(name, False, traceback.format_exc())
    except Exception:
        name = state["name"]
        if name:
            done(name, False, traceback.format_exc())
        else:
            unreal.log_error(f"SERVE {traceback.format_exc()}")
    finally:
        state["busy"] = False


os.makedirs(CMD, exist_ok=True)
# -ExecutePythonScript closes the editor on the first tick after the script returns unless told not
# to.
unreal.EditorPythonScriptingLibrary.set_keep_python_script_alive(True)
handle = unreal.register_slate_post_tick_callback(tick)
log(f"ready, watching {CMD}")
