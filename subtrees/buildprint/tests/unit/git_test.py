from __future__ import annotations

import pathlib
import unittest
from unittest import mock

from buildprint.client import _git


class GitTest(unittest.TestCase):
    def test_get_changed_files(self) -> None:
        git_client = _git.GitClient(
            "HEAD~1", pathlib.Path("/home/jamison/code/monorepo")
        )
        changed_files = git_client.diff_base()
        print(changed_files)


if __name__ == "__main__":
    unittest.main()
