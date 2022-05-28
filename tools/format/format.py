#!/usr/bin/env python3.10
import os
import subprocess
import sys


def main() -> int:
    return subprocess.run(
        ["python", "-m", "black", ".", *sys.argv],
        cwd=os.getenv("BUILD_WORKSPACE_DIRECTORY", "."),
    ).returncode


if __name__ == "__main__":
    sys.exit(main())
