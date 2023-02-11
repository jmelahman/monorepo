from unittest import mock

from pybazel.pybazel.client import BazelClient

_API_FIXTURES = [
    ([], None),
    ([], ""),
    ([], "bazel"),
    ([], "/bin/bazel"),
    (["--foo"], "/bin/bazel"),
]

with mock.patch.object(BazelClient, "info", return_value="/home/user/foo_workspace"):
    API_CLIENTS = []
    for bazel_options, workspace in _API_FIXTURES:
        assert isinstance(bazel_options, list)
        API_CLIENTS.append(
            BazelClient(bazel_options=bazel_options, workspace=workspace)
        )
