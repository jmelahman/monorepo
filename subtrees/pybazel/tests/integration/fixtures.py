import os
import tempfile

from pybazel.pybazel.client import BazelClient

OUTPUT_BASE = tempfile.mkdtemp()
_DEFAULT_OPTIONS = ["--output_base", OUTPUT_BASE]

_API_FIXTURES = [
    (_DEFAULT_OPTIONS + [], "bazel"),
]

API_CLIENTS = [
    BazelClient(
        bazel_options=bazel_options,
        workspace=os.path.join(os.path.expanduser("~"), "code", "monorepo"),
    )
    for bazel_options, workspace in _API_FIXTURES
]
