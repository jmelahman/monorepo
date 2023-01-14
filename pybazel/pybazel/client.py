from __future__ import annotations

import os
import subprocess
from typing import Any

from pybazel.api.client import APIClient
from pybazel.utils import logging

log = logging.getLogger(__name__)


class BazelClient:
    def __init__(self, *args: Any, **kwargs: Any) -> None:
        self.api = APIClient(*args, **kwargs)
        self.info = self.api.info
        self.query = self.api.query
