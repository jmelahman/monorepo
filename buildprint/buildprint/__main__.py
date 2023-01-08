from buildprint.version import __version__, __version_info__


def main() -> int:
    print(__version__)
    print(__version_info__)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
