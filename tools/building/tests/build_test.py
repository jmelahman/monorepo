from __future__ import annotations

import shutil
import tempfile
import unittest

import pybazel

_OUTPUT_BASE = tempfile.mkdtemp()


class BuildTest(unittest.TestCase):
    @classmethod
    def tearDownClass(cls) -> None:
        shutil.rmtree(_OUTPUT_BASE)

    def setUp(self) -> None:
        self.bazel = pybazel.BazelClient(
            workspace="/home/jamison/code/monorepo",
            output_base=_OUTPUT_BASE,
        )

    def test_build_go(self) -> None:
        self.bazel.build(
            [
                "//tools/build/tests/examples:greeter",
                "//tools/build/tests/examples:greeter_lib",
                "//tools/build/tests/examples:greeter_test",
            ],
        )


if __name__ == "__main__":
    unittest.main()
