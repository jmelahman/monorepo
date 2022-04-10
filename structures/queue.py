import typing

ItemType = str


class Queue:
    queue: typing.List

    def __init__(self) -> None:
        self.queue = []

    def enqueue(self, item: ItemType) -> None:
        self.queue.append(item)

    def empty(self) -> bool:
        return bool(not self.queue)

    def dequeue(self) -> typing.Optional[ItemType]:
        if not self.queue:
            return None
        item = self.queue.pop(0)
        assert isinstance(item, str)
        return item
