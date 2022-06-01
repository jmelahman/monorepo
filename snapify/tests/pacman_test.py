import unittest
from unittest import mock

from snapify.pysnapify.manager import pacman


_FAKE_PACKAGES = b"""archlinux-keyring
bash
bazel
libxcrypt
libxcrypt-compat
fake-snapify
pacman
pacman-mirrorlist
python
"""
_PACMAN_BIN = "/usr/bin/pacman"

class PacmanTest(unittest.TestCase):
    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    def setUp(self, mock_get_executable) -> None:
        mock_get_executable.side_effect = [
            _PACMAN_BIN,
            "/usr/bin/sudo",
        ]
        self.pacman = pacman.Pacman(noninteractive = False, ignored_packages = [])

    @mock.patch("subprocess.check_output", return_value=_FAKE_PACKAGES)
    def test_get_installed(self, mock_subprocess) -> None:
        installed_packages = self.pacman.get_installed_packages()
        self.assertIn("fake-snapify", installed_packages)
        mock_subprocess.assert_called_once_with([_PACMAN_BIN, "-Qq"])
        # Verify caching
        self.pacman.get_installed_packages()
        mock_subprocess.assert_called_once()

    @mock.patch("subprocess.check_output", return_value=_FAKE_PACKAGES)
    def test_get_installed(self, mock_subprocess) -> None:
        subtests = [
            ("foo", False),
            ("fake-snapify", True),
        ]
        for package, is_expected in subtests:
            with self.subTest(package=package):
                self.assertEqual(self.pacman.has_installed(package), is_expected)


if __name__ == "__main__":
    unittest.main()
