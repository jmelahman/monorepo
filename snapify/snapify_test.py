import os
import unittest
from unittest import mock

import snapify
from testdata import names, os_release, user_config


def _base_mock_open(filename: str, release: bytes) -> any:
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


def mock_open_arch(filename: str, _: any = None) -> any:
    return _base_mock_open(filename, os_release.ARCH)


def mock_open_manjaro(filename: str, _: any = None) -> any:
    return _base_mock_open(filename, os_release.MANJARO)


class SnapifyTest(unittest.TestCase):
    def test_supported_distros(self):
        subtests = [
            (os_release.ARCH, snapify.SupportedDistro.ARCH),
            (os_release.MANJARO, snapify.SupportedDistro.MANJARO),
        ]
        for release_data, expected_distro in subtests:
            with self.subTest(distro=expected_distro), mock.patch(
                "builtins.open", mock.mock_open(read_data=release_data)
            ):
                distro = snapify.Snapifier._check_supported_distro()
                self.assertEqual(distro, expected_distro)

    @mock.patch("os.path.exists", return_value=False)
    def test_no_user_config(self, mock_path_exists):
        snapify.Snapifier._read_user_config()

    @mock.patch("os.path.exists", return_value=True)
    def test_basic_user_config(self, mock_path_exists):
        with mock.patch(
            "builtins.open", mock.mock_open(read_data=user_config.ARCH_IGNORE_DOCKER)
        ):
            config = snapify.Snapifier._read_user_config()
            self.assertEqual(config, {snapify.SupportedDistro.ARCH: ["docker"]})

    @mock.patch("builtins.open", new=mock_open_arch)
    @mock.patch("snapify.Snapd.get_installed_packages", return_value=[])
    @mock.patch("snapify._get_executable")
    @mock.patch("os.path.exists")
    def test_snapifier(
        self, mock_path_exists, mock_get_executable, mock_installed_snaps
    ):
        mock_path_exists.side_effect = [
            True,  # ~/.config/snapify/config
            True,  # /var/cache/snapd/names
        ]
        mock_get_executable, side_effect = ["snap", "sudo"]
        snapifier = snapify.Snapifier(noninteractive=False)
        self.assertEqual(snapifier._distro, snapify.SupportedDistro.ARCH)
        self.assertEqual(snapifier._config, {snapify.SupportedDistro.ARCH: ["docker"]})

    @mock.patch("builtins.open", new=mock_open_manjaro)
    @mock.patch("snapify.Snapd.get_installed_packages", return_value=[])
    @mock.patch("snapify._get_executable")
    @mock.patch("os.path.exists")
    def test_snapifier(
        self, mock_path_exists, mock_get_executable, mock_installed_snaps
    ):
        mock_path_exists.side_effect = [
            True,  # ~/.config/snapify/config
            True,  # /var/cache/snapd/names
        ]
        mock_get_executable, side_effect = ["snap", "sudo"]
        snapifier = snapify.Snapifier(noninteractive=False)
        self.assertEqual(snapifier._distro, snapify.SupportedDistro.MANJARO)
        self.assertEqual(snapifier._config, {snapify.SupportedDistro.ARCH: ["docker"]})


if __name__ == "__main__":
    unittest.main()
