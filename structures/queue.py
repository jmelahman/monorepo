from __future__ import annotations

ItemType = str


class Queue:
    queue: list[ItemType]

    def __init__(self) -> None:
        self.queue = []

    def enqueue(self, item: ItemType) -> None:
        self.queue.append(item)

    def empty(self) -> bool:
        return bool(not self.queue)

    def dequeue(self) -> ItemType | None:
        if not self.queue:
            return None
        return self.queue.pop(0)
