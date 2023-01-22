import logging
import os
from typing import Any

from colorama import Fore
from colorama import Style

logging.basicConfig(
    format=f"{Fore.GREEN}%(levelname)s:{Style.RESET_ALL} %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)


def getLogger(*args: Any, **kwargs: Any) -> logging.Logger:
    return logging.getLogger(*args, **kwargs)
