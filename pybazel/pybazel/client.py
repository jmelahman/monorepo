from __future__ import annotations

import os
import subprocess
from typing import Any

from .api.client import APIClient
from .utils import logging

log = logging.getLogger(__name__)


class BazelClient:
    def __init__(self, *args: Any, **kwargs: Any) -> None:
        self.api = APIClient(*args, **kwargs)

    def info(self, *args: Any, **kwargs: Any) -> None:
        """
        An object for invoking the info command. See also,
        https://docs.bazel.build/versions/main/command-line-reference.html#info-options
        """
        print(self.api.info(*args, **kwargs))
