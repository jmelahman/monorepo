from __future__ import annotations

import os
from typing import Any
import unittest
from unittest import mock

from snapify.pysnapify import main, constants
from snapify.tests.testdata import names, os_release, user_config


def _base_mock_open(filename: str, release: bytes) -> Any:
    if filename == "/etc/os-release":
        content = release
    elif os.path.basename(filename) == "config":
        content = user_config.ARCH_IGNORE_DOCKER
    elif filename == "/var/cache/snapd/names":
        content = names.NAMES
    else:
        raise FileNotFoundError(filename)
    file_object = mock.mock_open(read_data=content).return_value
    file_object.__iter__.return_value = content.splitlines(True)
    return file_object


def mock_open_arch(filename: str, _: Any = None) -> Any:
    return _base_mock_open(filename, os_release.ARCH)


def mock_open_manjaro(filename: str, _: Any = None) -> Any:
    return _base_mock_open(filename, os_release.MANJARO)


class SnapifyTest(unittest.TestCase):
    def test_supported_distros(self) -> None:
        subtests = [
            (os_release.ARCH, constants.SupportedDistro.ARCH),
            (os_release.MANJARO, constants.SupportedDistro.MANJARO),
        ]
        for release_data, expected_distro in subtests:
            with self.subTest(distro=expected_distro), mock.patch(
                "builtins.open", mock.mock_open(read_data=release_data)
            ):
                distro = main.Snapifier._check_supported_distro()
                self.assertEqual(distro, expected_distro)

    @mock.patch("os.path.exists", return_value=False)
    def test_no_user_config(self, mock_path_exists: mock.MagicMock) -> None:
        main.Snapifier._read_user_config()

    @mock.patch("os.path.exists", return_value=True)
    def test_basic_user_config(self, mock_path_exists: mock.MagicMock) -> None:
        with mock.patch(
            "builtins.open", mock.mock_open(read_data=user_config.ARCH_IGNORE_DOCKER)
        ):
            config = main.Snapifier._read_user_config()
            self.assertEqual(config, {constants.SupportedDistro.ARCH: ["docker"]})

    @mock.patch("builtins.open", new=mock_open_arch)
    @mock.patch("snapify.pysnapify.manager.snap.Snapd.get_installed_packages", return_value=[])
    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    @mock.patch("os.path.exists")
    def test_snapifier_arch(
        self,
        mock_path_exists: mock.MagicMock,
        mock_get_executable: mock.MagicMock,
        mock_installed_snaps: mock.MagicMock,
    ) -> None:
        mock_path_exists.side_effect = [
            True,  # ~/.config/snapify/config
            True,  # /var/cache/snapd/names
        ]
        snapifier = main.Snapifier(noninteractive=False)
        self.assertEqual(snapifier._distro, constants.SupportedDistro.ARCH)
        self.assertEqual(snapifier._config, {constants.SupportedDistro.ARCH: ["docker"]})

    @mock.patch("builtins.open", new=mock_open_manjaro)
    @mock.patch("snapify.pysnapify.manager.snap.Snapd.get_installed_packages", return_value=[])
    @mock.patch("snapify.pysnapify.manager.utils.get_executable")
    @mock.patch("os.path.exists")
    def test_snapifier_manjaro(
        self,
        mock_path_exists: mock.MagicMock,
        mock_get_executable: mock.MagicMock,
        mock_installed_snaps: mock.MagicMock,
    ) -> None:
        mock_path_exists.side_effect = [
            True,  # ~/.config/snapify/config
            True,  # /var/cache/snapd/names
        ]
        snapifier = main.Snapifier(noninteractive=False)
        self.assertEqual(snapifier._distro, constants.SupportedDistro.MANJARO)
        self.assertEqual(snapifier._config, {constants.SupportedDistro.ARCH: ["docker"]})


if __name__ == "__main__":
    unittest.main()
