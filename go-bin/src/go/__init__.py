from __future__ import annotations

import os
import sys


def main() -> None:
    goroot = os.path.abspath(os.path.dirname(__file__))
    go_bin = os.path.join(goroot, "bin", "go")
    os.environ["GOROOT"] = goroot
    if os.name == "posix":
        os.execv(go_bin, [go_bin, *sys.argv[1:]])  # noqa: S606
    else:
        import subprocess  # noqa: PLC0415 - only Windows needs it

        sys.exit(subprocess.call([go_bin, *sys.argv[1:]]))  # noqa: S603
