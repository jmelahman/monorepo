from __future__ import annotations

from enum import Enum

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
