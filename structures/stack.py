import typing

ItemType = str


class Stack:
    stack: typing.List[ItemType]

    def __init__(self) -> None:
        self.stack = []

    def enqueue(self, item: ItemType) -> None:
        self.stack.append(item)

    def empty(self) -> bool:
        return bool(not self.stack)

    def dequeue(self) -> typing.Optional[ItemType]:
        if not self.stack:
            return None
        item = self.stack.pop()
        return item
