from unittest import mock

from pybazel.pybazel.api.client import APIClient

api_fixtures = [
    ([], None),
    ([], ""),
    ([], "bazel"),
    ([], "/bin/bazel"),
    (["--foo"], "/bin/bazel"),
]

with mock.patch.object(APIClient, 'info', return_value="/home/user/foo_workspace"):
    api_clients = [
        APIClient(bazel_options=bazel_options, workspace=workspace)
        for bazel_options, workspace in api_fixtures
    ]
