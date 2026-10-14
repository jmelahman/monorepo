#!/usr/bin/env python3
"""Serve the site locally, rebuilding and reloading the browser on changes.

A Markdown page is re-rendered when it changes, and every page is when the
template or the renderer does. HTML is served with a script appended that
listens on /__reload and refreshes the page, or only swaps the stylesheets
for a CSS change.
"""

from __future__ import annotations

import argparse
import contextlib
from http import HTTPStatus
from http.server import SimpleHTTPRequestHandler
from http.server import ThreadingHTTPServer
import os
from pathlib import Path
import subprocess
import threading
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parent.parent
RELOAD_PATH = "/__reload"
POLL_SECONDS = 0.25
KEEPALIVE_SECONDS = 15
# Kept in step with the pages ./build skips.
NOT_PAGES = {"README.md", "CLAUDE.md"}
REBUILD_ALL = {ROOT / "template.html", ROOT / "scripts" / "render.py"}
CLIENT = b"""
<script>
new EventSource("/__reload").onmessage = (event) => {
  if (event.data !== "css") return location.reload();
  for (const link of document.querySelectorAll('link[rel="stylesheet"]')) {
    const url = new URL(link.href);
    if (url.origin !== location.origin) continue;
    url.searchParams.set("v", Date.now());
    link.href = url;
  }
};
</script>
"""


class Reloads:
    """The latest change, for the open pages waiting to hear about one."""

    def __init__(self) -> None:
        self.changed = threading.Condition()
        self.count = 0
        self.kind = "page"

    def announce(self, kind: str) -> None:
        with self.changed:
            self.count += 1
            self.kind = kind
            self.changed.notify_all()

    def wait(self, seen: int) -> tuple[int, str | None]:
        with self.changed:
            if self.count == seen:
                self.changed.wait(KEEPALIVE_SECONDS)
            return self.count, self.kind if self.count != seen else None


RELOADS = Reloads()


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self) -> None:
        path = urlsplit(self.path).path
        if path == RELOAD_PATH:
            self.stream_reloads()
            return
        file = Path(self.translate_path(self.path))
        if file.is_dir() and path.endswith("/"):
            file = file / "index.html"
        if file.suffix == ".html" and file.is_file():
            self.send_page(file)
            return
        super().do_GET()

    def send_page(self, file: Path) -> None:
        body = file.read_bytes() + CLIENT
        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def stream_reloads(self) -> None:
        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        seen = RELOADS.count
        with contextlib.suppress(BrokenPipeError, ConnectionResetError):
            while True:
                seen, kind = RELOADS.wait(seen)
                # A comment line keeps the connection open and finds closed tabs.
                self.wfile.write(f"data: {kind}\n\n".encode() if kind else b": waiting\n\n")
                self.wfile.flush()

    def end_headers(self) -> None:
        self.send_header("Cache-Control", "no-store")
        super().end_headers()

    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        pass


def sources() -> dict[Path, int]:
    paths = [path for path in ROOT.rglob("*.md") if path.name not in NOT_PAGES]
    paths += (ROOT / "css").glob("*.css")
    paths += REBUILD_ALL
    return {path: path.stat().st_mtime_ns for path in paths if path.is_file()}


def rebuild(changed: set[Path]) -> str | None:
    """Regenerate what the changed sources feed and name the reload to send."""
    names = ", ".join(str(path.relative_to(ROOT)) for path in sorted(changed))
    pages = sorted(path for path in changed if path.suffix == ".md")
    if changed & REBUILD_ALL:
        command = ["./build"]
    elif pages:
        command = ["uv", "run", "scripts/render.py", "--variable=header=true", *map(str, pages)]
    else:
        print(f"changed {names}", flush=True)
        return "css"
    env = {**os.environ, "SKIP_FEED": "1"}
    started = time.monotonic()
    if subprocess.run(command, cwd=ROOT, env=env, check=False).returncode:  # noqa: S603
        print(f"failed to rebuild after {names}", flush=True)
        return None
    print(f"rebuilt after {names} in {time.monotonic() - started:.1f}s", flush=True)
    return "page"


def watch() -> None:
    before = sources()
    while True:
        time.sleep(POLL_SECONDS)
        after = sources()
        changed = {path for path, modified in after.items() if before.get(path) != modified}
        before = after
        if changed and (kind := rebuild(changed)):
            RELOADS.announce(kind)


def main() -> int:
    parser = argparse.ArgumentParser(description="Serve the site and reload it on changes.")
    parser.add_argument("--port", type=int, default=8000)
    args = parser.parse_args()

    threading.Thread(target=watch, daemon=True).start()
    ThreadingHTTPServer.daemon_threads = True
    server = ThreadingHTTPServer(("", args.port), lambda *a: Handler(*a, directory=str(ROOT)))
    print(f"serving http://localhost:{args.port}", flush=True)
    with contextlib.suppress(KeyboardInterrupt):
        server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
