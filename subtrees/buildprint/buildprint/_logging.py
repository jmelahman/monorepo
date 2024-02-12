from __future__ import annotations

import enum
import logging
import os
import time
from typing import Any

from colorama import Fore
from colorama import Style


class LogLevel(enum.Enum):
    DEBUG = "DEBUG"
    INFO = "INFO"
    WARNING = "WARNING"
    ERROR = "ERROR"
    CRITICAL = "CRITICAL"


class ColorFormatter(logging.Formatter):
    def __init__(self, show_timestamps: bool, *args: Any, **kwargs: Any) -> None:
        self._show_timestamps = show_timestamps
        super().__init__(*args, **kwargs)

    @property
    def show_timestamps(self) -> bool:
        return self._show_timestamps

    def format(self, record: logging.LogRecord) -> str:
        if record.levelno == logging.DEBUG:
            color = Fore.YELLOW
        elif record.levelno == logging.INFO:
            color = Fore.CYAN
        elif record.levelno == logging.WARNING:
            color = Fore.MAGENTA
        elif record.levelno in (logging.ERROR, logging.CRITICAL):
            color = Fore.RED
        else:
            color = Fore.RESET

        record.msg = f"{color}{record.levelname}:{Style.RESET_ALL} {record.msg}"
        if self.show_timestamps:
            record.msg = f"[{time.strftime('%H:%M:%S', time.gmtime())}] {record.msg}"
        return super().format(record)


def getLogger(  # noqa: N802
    name: str = "root",
    loglevel: LogLevel = LogLevel.INFO,
    *args: Any,
    **kwargs: Any,
) -> logging.Logger:
    show_timestamps = bool(kwargs.pop("timestamps", False))
    logger = logging.getLogger(name, *args, **kwargs)
    logger.setLevel(os.environ.get("LOGLEVEL", loglevel.value))

    console_handler = logging.StreamHandler()
    console_handler.setFormatter(ColorFormatter(show_timestamps))
    logger.addHandler(console_handler)

    return logger
