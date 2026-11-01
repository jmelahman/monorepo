"""Write a Kev checkpoint's head.pt as head.safetensors, which kev-go reads.

head.pt is a pickle, so kev-go never opens it. This loads it with weights_only=True,
which refuses anything but tensors and plain data.

    python -I convert_head.py <checkpoint-dir>
"""

from __future__ import annotations

import json
from pathlib import Path
import sys

from safetensors.torch import save_file  # ty: ignore[unresolved-import]
import torch  # ty: ignore[unresolved-import]

FIELDS = (
    "base",
    "base_revision",
    "lora",
    "head_dim",
    "option_isolation",
    "special_embeddings",
    "weights_dtype",
    "weights",
    "temperature",
)


def main(path: Path) -> None:
    head = torch.load(path / "head.pt", weights_only=True, map_location="cpu")
    meta = {k: json.dumps(head[k]) for k in FIELDS if k in head}
    save_file(
        {k: v.float().contiguous() for k, v in head["head"].items()},
        path / "head.safetensors",
        metadata=meta,
    )
    print({k: head.get(k) for k in FIELDS})


if __name__ == "__main__":
    main(Path(sys.argv[1]))
