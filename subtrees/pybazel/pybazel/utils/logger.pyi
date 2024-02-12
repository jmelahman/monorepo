import logging
from typing import Any

class ColorFormatter(logging.Formatter):
    def __init__(self, show_timestamps: bool, *args: Any, **kwargs: Any) -> None: ...
    @property
    def show_timestamps(self) -> bool: ...
    def format(self, record: logging.LogRecord) -> str: ...

def getLogger(*args: Any, **kwargs: Any) -> logging.Logger: ...  # noqa: N802
