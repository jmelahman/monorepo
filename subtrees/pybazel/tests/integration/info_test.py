from __future__ import annotations

import shutil
import unittest

from pybazel.pybazel.models.info import InfoKey
from pybazel.tests.integration.fixtures import API_CLIENTS
from pybazel.tests.integration.fixtures import OUTPUT_BASE


class InfoTest(unittest.TestCase):
    @classmethod
    def tearDownClass(cls: type[InfoTest]) -> None:
        shutil.rmtree(OUTPUT_BASE, ignore_errors=True)

    def test_info(self) -> None:
        for api in API_CLIENTS:
            api.info()
            for key in InfoKey:
                api.info(key=key)


if __name__ == "__main__":
    unittest.main()
