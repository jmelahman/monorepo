class Label:
    def __init__(self, name: str) -> None:
        self._name = name

    @property
    def name(self) -> str:
        return self._name

    def __repr__(self) -> str:
        return f"Label({self.name})"

    def __str__(self) -> str:
        return self.name
