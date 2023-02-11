from __future__ import annotations

import unittest
from unittest import mock

from pybazel.pybazel.models.info import InfoKey
from pybazel.tests.unit.fixtures import API_CLIENTS


class InfoTest(unittest.TestCase):
    @mock.patch("subprocess.check_output", return_value=b"fake: info")
    def test_info(self, mock_run: mock.MagicMock) -> None:
        for api in API_CLIENTS:
            api.info()
            api.info(configuration_options=["--bar"])
            for key in InfoKey:
                api.info(key=key)
                api.info(key=key, configuration_options=["--bar"])


if __name__ == "__main__":
    unittest.main()
