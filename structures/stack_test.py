import unittest

from structures import stack


class StackTest(unittest.TestCase):
    def test_instantiation(self) -> None:
        stack.Stack()

    def test_empty(self) -> None:
        q: stack.Stack = stack.Stack()
        self.assertTrue(q.empty())

    def test_enqueue(self) -> None:
        item = "foo"
        q: stack.Stack = stack.Stack()
        q.enqueue(item)
        self.assertFalse(q.empty())

    def test_dequeue(self) -> None:
        item = "foo"
        q: stack.Stack = stack.Stack()
        q.enqueue(item)
        current = q.dequeue()
        self.assertEqual(current, item)
        self.assertTrue(q.empty())

    def test_dequeue_empty(self) -> None:
        q: stack.Stack = stack.Stack()
        q.dequeue()
        self.assertTrue(q.empty())

    def test_lifo(self) -> None:
        item = "foo"
        item_two = "bar"
        q: stack.Stack = stack.Stack()
        q.enqueue(item)
        q.enqueue(item_two)
        first_out = q.dequeue()
        self.assertEqual(item_two, first_out)


if __name__ == "__main__":
    unittest.main()
