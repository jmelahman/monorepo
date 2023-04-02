import os
import unittest


class FormatTest(unittest.TestCase):
    def test_pass(self) -> None:
        for env, value in os.environ.items():
            print(env, value)


if __name__ == "__main__":
    unittest.main()
