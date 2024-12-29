from __future__ import annotations

from enum import Enum

# https://go.dev/doc/install/source#environment
# See also, `$ go tools dist list`
class GOOS(Enum):
    """Enumeration of valid Go operating systems."""
    AIX = "aix"
    ANDROID = "android"
    DARWIN = "darwin"
    FREEBSD = "freebsd"
    IOS = "ios"
    JS = "js"
    LINUX = "linux"
    NETBSD = "netbsd"
    OPENBSD = "openbsd"
    PLAN9 = "plan9"
    SOLARIS = "solaris"
    WINDOWS = "windows"

class GOARCH(Enum):
    """Enumeration of valid Go architectures."""
    AMD64 = "amd64"
    ARM = "arm"
    ARM64 = "arm64"
    MIPS = "mips"
    MIPS64 = "mips64"
    PPC64 = "ppc64"
    RISCV64 = "riscv64"
    S390X = "s390x"
    WASM = "wasm"
    X86 = "386"

DistList = frozenset([
    "aix/ppc64",
    "android/386",
    "android/amd64",
    "android/arm",
    "android/arm64",
    "darwin/amd64",
    "darwin/arm64",
    "dragonfly/amd64",
    "freebsd/386",
    "freebsd/amd64",
    "freebsd/arm",
    "freebsd/arm64",
    "freebsd/riscv64",
    "illumos/amd64",
    "ios/amd64",
    "ios/arm64",
    "js/wasm",
    "linux/386",
    "linux/amd64",
    "linux/arm",
    "linux/arm64",
    "linux/loong64",
    "linux/mips",
    "linux/mips64",
    "linux/mips64le",
    "linux/mipsle",
    "linux/ppc64",
    "linux/ppc64le",
    "linux/riscv64",
    "linux/s390x",
    "netbsd/386",
    "netbsd/amd64",
    "netbsd/arm",
    "netbsd/arm64",
    "openbsd/386",
    "openbsd/amd64",
    "openbsd/arm",
    "openbsd/arm64",
    "openbsd/ppc64",
    "openbsd/riscv64",
    "plan9/386",
    "plan9/amd64",
    "plan9/arm",
    "solaris/amd64",
    "wasip1/wasm",
    "windows/386",
    "windows/amd64",
    "windows/arm",
    "windows/arm64",
])
