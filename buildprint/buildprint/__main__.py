#!/usr/bin/env python3
import argparse

from buildprint._version import __version__, __version_info__


def get_parsed_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "-v",
        "--version",
        action="store_true",
        default=False,
        help="print version info",
    )
    return parser.parse_args()


def main() -> int:
    args = get_parsed_args()
    if args.version:
        print(__version__)
    else:
        print(__version_info__)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
