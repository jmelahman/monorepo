import unittest
from unittest import mock

import snapify
from testdata import os_release

class SnapifyTest(unittest.TestCase):
    def test_snapify_arch(self):
        with mock.patch("builtins.open", mock.mock_open(read_data=os_release.ARCH)):
            snapify.main()


if __name__ == "__main__":
    unittest.main()
