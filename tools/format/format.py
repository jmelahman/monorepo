#!/usr/bin/env python3.10
import os
import subprocess


def main() -> None:
    subprocess.check_call(
        ["python", "-m", "black", "."],
        cwd=os.environ["BUILD_WORKSPACE_DIRECTORY"],
    )


if __name__ == "__main__":
    main()
