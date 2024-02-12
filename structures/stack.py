from __future__ import annotations

ItemType = str


class Stack:
    stack: list[ItemType]

    def __init__(self) -> None:
        self.stack = []

    def enqueue(self, item: ItemType) -> None:
        self.stack.append(item)

    def empty(self) -> bool:
        return bool(not self.stack)

    def dequeue(self) -> ItemType | None:
        if not self.stack:
            return None
        return self.stack.pop()
