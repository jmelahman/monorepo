from __future__ import annotations

import os
import sys


def main() -> None:
    goroot = os.path.abspath(os.path.dirname(__file__))
    go_bin = os.path.join(goroot, "bin", "go")
    os.environ["GOROOT"] = goroot
    os.execv(go_bin, [go_bin, *sys.argv[1:]])  # noqa: S606
