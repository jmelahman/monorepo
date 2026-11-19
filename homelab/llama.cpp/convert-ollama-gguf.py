#!/usr/bin/env python3
"""Repackage an Ollama model blob as a GGUF that llama.cpp can load.

Ollama stores each model as a plain GGUF in its blob store, but some blobs
carry extra data that llama.cpp rejects:

  * A vision (or audio) tower fused into the same file (`v.*`, `mm.*`, `a.*`
    tensors). llama.cpp expects those in a separate mmproj file, so it aborts
    with "wrong number of tensors".
  * Metadata arrays sized differently from what llama.cpp expects, such as a
    3-entry `rope.dimension_sections` where 4 are required.

This rewrites the header (optionally dropping tensors and patching metadata)
and repacks the kept tensor data verbatim, so quantized weights are untouched
but the file shrinks when tensors are dropped.

Usage:
    ./convert-ollama-gguf.py --strip-vision \\
        --append-zero qwen35moe.rope.dimension_sections \\
        --out models/model.gguf BLOB
"""

from __future__ import annotations

import argparse
import os
import re
import struct
import sys
from typing import Any, IO

# GGUF metadata value type -> struct format
SCALAR = {
    0: "B",  # uint8
    1: "b",  # int8
    2: "H",  # uint16
    3: "h",  # int16
    4: "I",  # uint32
    5: "i",  # int32
    6: "f",  # float32
    7: "?",  # bool
    10: "Q",  # uint64
    11: "q",  # int64
    12: "d",  # float64
}
STRING = 8
ARRAY = 9


def read_exact(f: IO[bytes], n: int) -> bytes:
    data = f.read(n)
    if len(data) != n:
        raise ValueError("unexpected end of file")
    return data


def read_string(f: IO[bytes]) -> str:
    (n,) = struct.unpack("<Q", read_exact(f, 8))
    return read_exact(f, n).decode("utf-8")


def read_value(f: IO[bytes], vtype: int) -> Any:
    if vtype == STRING:
        return read_string(f)
    if vtype == ARRAY:
        (etype,) = struct.unpack("<I", read_exact(f, 4))
        (n,) = struct.unpack("<Q", read_exact(f, 8))
        return etype, [read_value(f, etype) for _ in range(n)]
    fmt = "<" + SCALAR[vtype]
    return struct.unpack(fmt, read_exact(f, struct.calcsize(fmt)))[0]


def write_string(f: IO[bytes], s: str) -> None:
    data = s.encode("utf-8")
    f.write(struct.pack("<Q", len(data)))
    f.write(data)


def write_value(f: IO[bytes], vtype: int, value: Any) -> None:
    if vtype == STRING:
        write_string(f, value)
    elif vtype == ARRAY:
        etype, items = value
        f.write(struct.pack("<IQ", etype, len(items)))
        for item in items:
            write_value(f, etype, item)
    else:
        f.write(struct.pack("<" + SCALAR[vtype], value))


def main() -> None:  # noqa: C901, PLR0912, PLR0915
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("source", help="Ollama blob or GGUF to read")
    ap.add_argument("--out", required=True, help="GGUF to write")
    ap.add_argument(
        "--strip-vision",
        action="store_true",
        help="drop fused vision/audio tensors (v.*, mm.*, a.*)",
    )
    ap.add_argument(
        "--append-zero",
        action="append",
        default=[],
        metavar="KEY",
        help="append 0 to an integer-array metadata value, e.g. "
        "qwen35moe.rope.dimension_sections (repeatable)",
    )
    ap.add_argument(
        "--drop-prefix",
        action="append",
        default=[],
        metavar="PREFIX",
        help="drop tensors whose name starts with PREFIX (repeatable), e.g. mtp.",
    )
    ap.add_argument(
        "--rename",
        action="append",
        default=[],
        metavar="REGEX=REPL",
        help="rename tensors whose name matches REGEX (repeatable), e.g. "
        r"'(blk\.\d+\.ssm_dt)$=\1.bias'",
    )
    ap.add_argument(
        "--fill-zero",
        action="append",
        default=[],
        metavar="KEY=VALUE",
        help="replace zeros in an integer-array metadata value (repeatable), "
        "e.g. qwen35moe.attention.head_count_kv=2",
    )
    args = ap.parse_args()

    with open(args.source, "rb") as src:
        magic = read_exact(src, 4)
        if magic != b"GGUF":
            sys.exit(f"{args.source}: not a GGUF file (magic {magic!r})")
        (version,) = struct.unpack("<I", read_exact(src, 4))
        (n_tensors,) = struct.unpack("<Q", read_exact(src, 8))
        (n_kv,) = struct.unpack("<Q", read_exact(src, 8))

        kv = []
        for _ in range(n_kv):
            key = read_string(src)
            (vtype,) = struct.unpack("<I", read_exact(src, 4))
            kv.append((key, vtype, read_value(src, vtype)))

        tensors = []
        for _ in range(n_tensors):
            name = read_string(src)
            (n_dims,) = struct.unpack("<I", read_exact(src, 4))
            dims = struct.unpack("<" + "Q" * n_dims, read_exact(src, 8 * n_dims))
            (ttype,) = struct.unpack("<I", read_exact(src, 4))
            (offset,) = struct.unpack("<Q", read_exact(src, 8))
            tensors.append((name, dims, ttype, offset))

        values = {key: value for key, vtype, value in kv}
        alignment = values.get("general.alignment", 32)
        data_start = (src.tell() + alignment - 1) // alignment * alignment
        src.seek(0, os.SEEK_END)
        file_size = src.tell()

        # Tensor data is packed back-to-back in offset order, so each tensor's
        # size is the gap to the next one (and the rest of the data section for
        # the last one).
        data_size = file_size - data_start
        sized = []
        for i, (name, dims, ttype, offset) in enumerate(tensors):
            end = tensors[i + 1][3] if i + 1 < len(tensors) else data_size
            sized.append((name, dims, ttype, offset, end - offset))

        if args.strip_vision:
            sized = [t for t in sized if not t[0].startswith(("v.", "mm.", "a."))]

        for prefix in args.drop_prefix:
            sized = [t for t in sized if not t[0].startswith(prefix)]

        for spec in args.rename:
            pattern, sep, repl = spec.partition("=")
            if not sep:
                sys.exit(f"--rename: expected REGEX=REPL, got {spec!r}")
            sized = [
                (re.sub(pattern, repl, name), dims, ttype, offset, size)
                for name, dims, ttype, offset, size in sized
            ]

        # Repack the kept tensors contiguously.
        tensors = []
        data_offset = 0
        for name, dims, ttype, offset, size in sized:
            tensors.append((name, dims, ttype, data_offset, offset, size))
            data_offset += size

        for key in args.append_zero:
            for i, (k, vtype, value) in enumerate(kv):
                if k != key:
                    continue
                etype, items = value
                if vtype != ARRAY or etype not in (4, 5, 10, 11):
                    sys.exit(f"{key}: expected an integer array")
                kv[i] = (k, vtype, (etype, [*items, 0]))

        for spec in args.fill_zero:
            key, sep, raw = spec.partition("=")
            if not sep:
                sys.exit(f"--fill-zero: expected KEY=VALUE, got {spec!r}")
            fill = int(raw)
            found = False
            for i, (k, vtype, value) in enumerate(kv):
                if k != key:
                    continue
                etype, items = value
                if vtype != ARRAY or etype not in (4, 5, 10, 11):
                    sys.exit(f"{key}: expected an integer array")
                kv[i] = (k, vtype, (etype, [fill if v == 0 else v for v in items]))
                found = True
            if not found:
                sys.exit(f"{key}: not found")

        tmp = args.out + ".tmp"
        with open(tmp, "wb") as out:
            out.write(b"GGUF")
            out.write(struct.pack("<I", version))
            out.write(struct.pack("<Q", len(tensors)))
            out.write(struct.pack("<Q", len(kv)))
            for key, vtype, value in kv:
                write_string(out, key)
                out.write(struct.pack("<I", vtype))
                write_value(out, vtype, value)
            for name, dims, ttype, offset, _, _ in tensors:
                write_string(out, name)
                out.write(struct.pack("<I", len(dims)))
                out.write(struct.pack("<" + "Q" * len(dims), *dims))
                out.write(struct.pack("<I", ttype))
                out.write(struct.pack("<Q", offset))
            pad = (-out.tell()) % alignment
            out.write(b"\0" * pad)

            for _, _, _, _, offset, size in tensors:
                src.seek(data_start + offset)
                remaining = size
                while remaining:
                    chunk = src.read(min(remaining, 64 << 20))
                    if not chunk:
                        raise ValueError("unexpected end of file")
                    out.write(chunk)
                    remaining -= len(chunk)

        os.replace(tmp, args.out)
        out_size = os.path.getsize(args.out)
        print(
            f"{args.out}: {len(tensors)} tensors, {len(kv)} metadata keys ({out_size / 1e9:.1f} GB)"
        )


if __name__ == "__main__":
    main()
