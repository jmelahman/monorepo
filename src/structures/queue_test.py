import unittest

from src.structures import queue


class QueueTest(unittest.TestCase):
    def test_instantiation(self) -> None:
        queue.Queue()

    def test_empty(self) -> None:
        q: queue.Queue = queue.Queue()
        self.assertTrue(q.empty())

    def test_enqueue(self) -> None:
        item = "foo"
        q: queue.Queue = queue.Queue()
        q.enqueue(item)
        self.assertFalse(q.empty())

    def test_dequeue(self) -> None:
        item = "foo"
        q: queue.Queue = queue.Queue()
        q.enqueue(item)
        current = q.dequeue()
        self.assertEqual(current, item)
        self.assertTrue(q.empty())

    def test_dequeue_empty(self) -> None:
        q: queue.Queue = queue.Queue()
        current = q.dequeue()
        self.assertTrue(q.empty())

    def test_fifo(self) -> None:
        item = "foo"
        item_two = "bar"
        q: queue.Queue = queue.Queue()
        q.enqueue(item)
        q.enqueue(item_two)
        first_out = q.dequeue()
        self.assertEqual(item, first_out)


if __name__ == "__main__":
    unittest.main()
