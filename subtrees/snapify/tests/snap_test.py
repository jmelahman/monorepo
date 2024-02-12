from __future__ import annotations

import unittest
from unittest import mock

from snapify.pysnapify.manager import snap
from snapify.tests.testdata import names

_SNAP_LIST = b"""Name                               Version                     Rev    Tracking          Publisher     Notes
bare                               1.0                         5      latest/stable     canonical    base
core                               16-2.55.5                   13250  latest/stable     canonical    core
core18                             20220428                    2409   latest/stable     canonical    base
core20                             20220512                    1494   latest/stable     canonical    base
"""
_PACMAN_BIN = "/usr/bin/fake-pacman"
_SUDO_BIN = "/usr/bin/fake-sudo"


class SnapTest(unittest.TestCase):
    @mock.patch("subprocess.check_output", return_value=_SNAP_LIST)
    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    @mock.patch(
        "snapify.pysnapify.manager.snap.Snapd._names_exists",
        return_value=False,
    )
    def setUp(
        self,
        _mock_names_exists: mock.MagicMock,
        mock_get_executable: mock.MagicMock,
        _mock_subprocess: mock.MagicMock,
    ) -> None:
        self._default_get_executable = [
            _PACMAN_BIN,
            _SUDO_BIN,
        ]
        mock_get_executable.side_effect = self._default_get_executable
        self.snap = snap.Snapd(noninteractive=False, ignored_packages=[])

    def test_get_installed(self) -> None:
        expected_packages = ["bare", "core", "core18", "core20"]
        installed_packages = self.snap.get_installed_packages()
        self.assertEqual(expected_packages, installed_packages)

    @mock.patch("snapify.pysnapify.manager.snap.Snapd._names_exists", return_value=True)
    def test_get_available(self, _mock_names_exists: mock.MagicMock) -> None:
        with mock.patch("builtins.open", mock.mock_open(read_data=names.NAMES)):
            self.snap.get_available_packages()
            # self.assertIn("fake-snapify", available_packages)


if __name__ == "__main__":
    unittest.main()
