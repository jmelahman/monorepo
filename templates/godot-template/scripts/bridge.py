#!/usr/bin/env python3
"""Client for addons/agent_bridge: sends one command to a running game.

Start the game with scripts/play.sh first. Standard library only, so it runs
anywhere Python does with nothing to install.

    scripts/bridge.py ping
    scripts/bridge.py tree [path] [depth]
    scripts/bridge.py get /root/Main/Hud/Score text
    scripts/bridge.py set /root/Main/Hud/Score modulate 'var:Color(1, 0, 0)'
    scripts/bridge.py call /root/Main dispatch '{"type": "roll"}'
    scripts/bridge.py eval 'scene.state.score'
    scripts/bridge.py action roll
    scripts/bridge.py key Space
    scripts/bridge.py click 960 540
    scripts/bridge.py wait 30
    scripts/bridge.py screenshot shot.png
    scripts/bridge.py quit
    scripts/bridge.py raw '{"cmd": "get", "args": {...}}'

Positional arguments that parse as JSON are sent as JSON, so 3 is a number
and '{"type": "roll"}' is an object; anything else is a string. The result
prints as JSON; a refused command prints its error and exits 1.
"""

from __future__ import annotations

import json
import os
import socket
import sys
from typing import Any

PORT = int(os.environ.get("AGENT_BRIDGE_PORT", "9877"))
TIMEOUT = float(os.environ.get("AGENT_BRIDGE_TIMEOUT", "30"))

# Each command's positional arguments, in order.
COMMANDS: dict[str, list[str]] = {
    "ping": [],
    "tree": ["path", "depth"],
    "get": ["path", "prop"],
    "set": ["path", "prop", "value"],
    "call": ["path", "method", "*args"],
    "eval": ["expr"],
    "action": ["name", "frames"],
    "key": ["key", "frames"],
    "click": ["x", "y", "button"],
    "wait": ["frames"],
    "screenshot": ["path"],
    "quit": ["code"],
}


def parse(value: str) -> Any:
    try:
        return json.loads(value)
    except json.JSONDecodeError:
        return value


def build(cmd: str, values: list[str]) -> dict[str, Any]:
    if cmd == "raw":
        return json.loads(values[0])
    if cmd not in COMMANDS:
        sys.exit(f"unknown command {cmd}; one of {', '.join([*COMMANDS, 'raw'])}")
    args: dict[str, Any] = {}
    names = COMMANDS[cmd]
    for i, name in enumerate(names):
        if name.startswith("*"):
            args[name[1:]] = [parse(v) for v in values[i:]]
            break
        if i < len(values):
            # A path the game writes to is resolved here, where the caller is.
            if cmd == "screenshot":
                args[name] = os.path.abspath(values[i])
            elif name in ("path", "expr", "method", "name", "key", "prop"):
                args[name] = values[i]
            else:
                args[name] = parse(values[i])
    return {"cmd": cmd, "args": args}


def send(request: dict[str, Any]) -> dict[str, Any]:
    request.setdefault("id", 1)
    try:
        with socket.create_connection(("127.0.0.1", PORT), timeout=TIMEOUT) as sock:
            sock.sendall((json.dumps(request) + "\n").encode())
            data = b""
            while not data.endswith(b"\n"):
                chunk = sock.recv(65536)
                if not chunk:
                    break
                data += chunk
    except ConnectionRefusedError:
        sys.exit(f"no bridge on 127.0.0.1:{PORT}; start the game with scripts/play.sh")
    if not data and request["cmd"] == "quit":
        return {"ok": True, "result": None}
    return json.loads(data)


def main() -> None:
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print(__doc__)
        return
    reply = send(build(sys.argv[1], sys.argv[2:]))
    if not reply.get("ok"):
        print(reply.get("error"), file=sys.stderr)
        sys.exit(1)
    print(json.dumps(reply.get("result"), indent=2))


if __name__ == "__main__":
    main()
