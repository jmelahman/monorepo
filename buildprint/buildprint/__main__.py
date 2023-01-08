from buildprint.version import version, version_info


def main() -> int:
    print(version)
    print(version_info)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
