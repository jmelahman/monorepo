import shutil
import tempfile
import unittest

import pybazel

_OUTPUT_BASE = tempfile.mkdtemp()


class BuildTest(unittest.TestCase):
    def tearDownClass(cls) -> None:
        shutil.rmtree(_OUTPUT_BASE)

    def setUp(self) -> None:
        self.bazel = pybazel.BazelClient(
            bazel_options=["--output_base", _OUTPUT_BASE],
            workspace_dir="/home/jamison/code/monorepo",
        )

    def test_build_go(self) -> None:
        self.bazel.build(
            [
                "//tools/build/tests/examples:greeter",
                "//tools/build/tests/examples:greeter_lib",
                "//tools/build/tests/examples:greeter_test",
            ]
        )


if __name__ == "__main__":
    unittest.main()
