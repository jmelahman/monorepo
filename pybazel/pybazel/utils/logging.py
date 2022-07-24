import logging
import os
from typing import Any

import colorama

logging.basicConfig(
    format=f"{colorama.Fore.GREEN}%(levelname)s:{colorama.Style.RESET_ALL} %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)

def getLogger(*args: Any, **kwargs: Any) -> logging.Logger:
    return logging.getLogger(*args, **kwargs)
