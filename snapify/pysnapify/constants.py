import enum


class SnapifyConfigError(Exception):
    __module__ = "builtins"


class SupportedDistro(enum.Enum):
    ARCH = "arch"
    MANJARO = "manjaro"

