#!/usr/bin/env python3.10
import os
import subprocess
import sys


def main() -> None:
    subprocess.check_call(
        ["python", "-m", "black", ".", *sys.argv],
        cwd=os.getenv("BUILD_WORKSPACE_DIRECTORY", "."),
    )


if __name__ == "__main__":
    main()
