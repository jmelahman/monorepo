import sys
import shutil
import unittest
from unittest import mock

from pybazel.pybazel.models.info import InfoKey
from pybazel.tests.integration.fixtures import api_clients, OUTPUT_BASE


class InfoTest(unittest.TestCase):
    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(OUTPUT_BASE)

    def test_info(self) -> None:
        for api in api_clients:
            api.info()
            for key in InfoKey:
                api.info(key=key)


if __name__ == "__main__":
    unittest.main()
