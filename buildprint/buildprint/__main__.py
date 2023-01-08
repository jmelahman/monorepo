import time

from buildprint.version import version, version_info


def fib(n: int) -> int:
    if n <= 1:
        return n
    else:
        return fib(n - 2) + fib(n - 1)


t0 = time.time()
fib(32)
print(time.time() - t0)


def main() -> int:
    print(version)
    print(version_info)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
