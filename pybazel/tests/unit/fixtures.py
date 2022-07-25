from unittest import mock

from pybazel.pybazel.api.client import APIClient

_API_FIXTURES = [
    ([], None),
    ([], ""),
    ([], "bazel"),
    ([], "/bin/bazel"),
    (["--foo"], "/bin/bazel"),
]

with mock.patch.object(APIClient, 'info', return_value="/home/user/foo_workspace"):
    API_CLIENTS = []
    for bazel_options, workspace in _API_FIXTURES:
        assert isinstance(bazel_options, list)
        API_CLIENTS.append(
            APIClient(bazel_options=bazel_options, workspace=workspace)
        )
