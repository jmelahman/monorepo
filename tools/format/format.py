#!/usr/bin/env python3.10
from __future__ import annotations

import os
import subprocess
import sys


def main() -> int:
    return subprocess.run(
        [sys.executable, "-m", "black", ".", *sys.argv],
        cwd=os.getenv("BUILD_WORKSPACE_DIRECTORY", "."), check=False,
    ).returncode


if __name__ == "__main__":
    sys.exit(main())
