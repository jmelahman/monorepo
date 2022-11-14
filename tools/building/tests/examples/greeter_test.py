import unittest

from tools.building.tests.examples import greeter


class TestHello(unittest.TestCase):
    def test_say_hello(self) -> None:
        greeter.say_hello()


if __name__ == "__main__":
    unittest.main()
