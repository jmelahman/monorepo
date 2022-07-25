import os
import tempfile
from unittest import mock

from pybazel.pybazel.api.client import APIClient

OUTPUT_BASE = tempfile.mkdtemp()
_DEFAULT_OPTIONS = ["--output_base", OUTPUT_BASE]

api_fixtures = [
    (_DEFAULT_OPTIONS + [], "bazel"),
]

api_clients = [
    APIClient(
        bazel_options=bazel_options,
        workspace=os.path.join(os.path.expanduser("~"), "code", "monorepo")
    )
    for bazel_options, workspace in api_fixtures
]
