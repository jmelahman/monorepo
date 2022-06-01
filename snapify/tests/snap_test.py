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


class SnapTest(unittest.TestCase):
    @mock.patch("subprocess.check_output", return_value=_SNAP_LIST)
    @mock.patch(
        "snapify.pysnapify.manager.snap.Snapd._names_exists", return_value=False
    )
    def setUp(
        self, mock_names_exists: mock.MagicMock, mock_subprocess: mock.MagicMock
    ) -> None:
        self.snap = snap.Snapd(noninteractive=False, ignored_packages=[])

    def test_get_installed(self) -> None:
        expected_packages = ["bare", "core", "core18", "core20"]
        installed_packages = self.snap.get_installed_packages()
        self.assertEqual(expected_packages, installed_packages)

    @mock.patch("snapify.pysnapify.manager.snap.Snapd._names_exists", return_value=True)
    def test_get_available(self, mock_names_exists: mock.MagicMock) -> None:
        available_packages = self.snap.get_available_packages()
        with mock.patch("builtins.open", mock.mock_open(read_data=names.NAMES)):
            pass
            # self.assertIn("fake-snapify", available_packages)


if __name__ == "__main__":
    unittest.main()
