from __future__ import annotations

import enum

NONINTERACTIVE_DEFAULT = False


class SupportedDistro(enum.Enum):
    ARCH = "arch"
    MANJARO = "manjaro"
