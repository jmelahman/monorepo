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
_NEVER_AVAILABLE = "never-available"
_PACMAN_BIN = "/usr/bin/fake-pacman"
_SUDO_BIN = "/usr/bin/fake-sudo"


class PacmanTest(unittest.TestCase):
    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    def setUp(self, mock_get_executable: mock.MagicMock) -> None:
        self._default_get_executable = [
            _PACMAN_BIN,
            _SUDO_BIN,
        ]
        mock_get_executable.side_effect = self._default_get_executable
        self.pacman = pacman.Pacman(noninteractive=False, ignored_packages=[])

    @mock.patch("subprocess.check_output", return_value=_FAKE_PACKAGES)
    def test_get_installed(self, mock_subprocess: mock.MagicMock) -> None:
        installed_packages = self.pacman.get_installed_packages()
        self.assertIn("fake-snapify", installed_packages)
        mock_subprocess.assert_called_once_with([_PACMAN_BIN, "-Qq"])
        # Verify caching
        self.pacman.get_installed_packages()
        mock_subprocess.assert_called_once()

    @mock.patch("subprocess.run")
    def test_filter_removeable(self, mock_subprocess: mock.MagicMock) -> None:
        # TODO: Make assertions.
        self.pacman.filter_removeable(["foo", "bar"])

    @mock.patch("subprocess.check_output", return_value=_FAKE_PACKAGES)
    def test_has_installed(self, mock_subprocess: mock.MagicMock) -> None:
        subtests = [
            ("foo", False),
            ("fake-snapify", True),
        ]
        for package, is_expected in subtests:
            with self.subTest(package=package):
                self.assertEqual(self.pacman.has_installed(package), is_expected)

    @mock.patch("snapify.pysnapify.manager.pacman.Pacman._run", return_value=0)
    def test_has_available(self, mock_subprocess: mock.MagicMock) -> None:
        package_available = self.pacman.has_available("fake-snapify")
        self.assertTrue(package_available)
        mock_subprocess.assert_called_once_with([_PACMAN_BIN, "-Qs", "^fake-snapify$"])

    @mock.patch("subprocess.run")
    def test_not_has_available(self, mock_subprocess: mock.MagicMock) -> None:
        package_available = self.pacman.has_available("foobar")
        self.assertFalse(package_available)
        mock_subprocess.assert_called_once_with([_PACMAN_BIN, "-Qs", "^foobar$"])

    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    def test_never_has_available(self, mock_get_executable: mock.MagicMock) -> None:
        mock_get_executable.side_effect = self._default_get_executable
        self.pacman = pacman.Pacman(
            noninteractive=False, ignored_packages=[_NEVER_AVAILABLE]
        )
        package_available = self.pacman.has_available(_NEVER_AVAILABLE)
        self.assertFalse(package_available)

    @mock.patch("subprocess.check_call")
    def test_install(self, mock_subprocess: mock.MagicMock) -> None:
        self.pacman.install(["fake-snapify"])
        mock_subprocess.assert_called_once_with(
            [_SUDO_BIN, _PACMAN_BIN, "-S", "fake-snapify"]
        )

    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    @mock.patch("subprocess.check_call")
    def test_install_noninteractive(
        self, mock_subprocess: mock.MagicMock, mock_get_executable: mock.MagicMock
    ) -> None:
        mock_get_executable.side_effect = self._default_get_executable
        self.pacman = pacman.Pacman(noninteractive=True, ignored_packages=[])
        self.pacman.install(["fake-snapify"])
        mock_subprocess.assert_called_once_with(
            [_SUDO_BIN, _PACMAN_BIN, "-S", "--noconfirm", "fake-snapify"]
        )

    @mock.patch("subprocess.check_call")
    def test_remove(self, mock_subprocess: mock.MagicMock) -> None:
        self.pacman.remove(["fake-snapify"])
        mock_subprocess.assert_called_once_with(
            [_SUDO_BIN, _PACMAN_BIN, "-Rs", "fake-snapify"]
        )

    @mock.patch("subprocess.check_call")
    def test_purge(self, mock_subprocess: mock.MagicMock) -> None:
        self.pacman.remove(["fake-snapify"], purge=True)
        mock_subprocess.assert_called_once_with(
            [_SUDO_BIN, _PACMAN_BIN, "-Rsn", "fake-snapify"]
        )

    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    @mock.patch("subprocess.check_call")
    def test_remove_noninteractive(
        self, mock_subprocess: mock.MagicMock, mock_get_executable: mock.MagicMock
    ) -> None:
        mock_get_executable.side_effect = self._default_get_executable
        self.pacman = pacman.Pacman(noninteractive=True, ignored_packages=[])
        self.pacman.remove(["fake-snapify"])
        mock_subprocess.assert_called_once_with(
            [_SUDO_BIN, _PACMAN_BIN, "-Rs", "--noconfirm", "fake-snapify"]
        )

    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    @mock.patch("subprocess.check_call")
    def test_purge_noninteractive(
        self, mock_subprocess: mock.MagicMock, mock_get_executable: mock.MagicMock
    ) -> None:
        mock_get_executable.side_effect = self._default_get_executable
        self.pacman = pacman.Pacman(noninteractive=True, ignored_packages=[])
        self.pacman.remove(["fake-snapify"], purge=True)
        mock_subprocess.assert_called_once_with(
            [_SUDO_BIN, _PACMAN_BIN, "-Rsn", "--noconfirm", "fake-snapify"]
        )


if __name__ == "__main__":
    unittest.main()
