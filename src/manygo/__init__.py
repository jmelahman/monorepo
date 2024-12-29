from __future__ import annotations

from enum import Enum

class GOOS(Enum):
    """Enumeration of valid Go operating systems."""
    DARWIN = "darwin"
    LINUX = "linux"
    WINDOWS = "windows"
    FREEBSD = "freebsd"
    OPENBSD = "openbsd"
    NETBSD = "netbsd"
    SOLARIS = "solaris"
    AIX = "aix"
    PLAN9 = "plan9"
    JS = "js"
    ANDROID = "android"
    IOS = "ios"

class GOARCH(Enum):
    """Enumeration of valid Go architectures."""
    AMD64 = "amd64"
    ARM64 = "arm64"
    ARM = "arm"
    X86 = "386"
    MIPS = "mips"
    MIPS64 = "mips64"
    PPC64 = "ppc64"
    S390X = "s390x"
    RISCV64 = "riscv64"
    WASM = "wasm"
