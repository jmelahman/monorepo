from __future__ import annotations

# TODO: Convert to enum once mypyc supports them.
# See, https://github.com/mypyc/mypyc/issues/896.
# import enum

_KNOWN_KEYS = frozenset(
    [
        "release",
        "workspace",
        "install_base",
        "output_base",
        "execution_root",
        "output_path",
        "server_pid",
        "server_log",
        "command_log",
        "used-heap-size",
        "committed-heap-size",
        "max-heap-size",
        "gc-time",
        "gc-count",
        "package_path",
        "bazel-bin",
        "bazel-testlogs",
        "bazel-genfiles",
    ]
)


class InfoKey:
    def __init__(self, value: str) -> None:
        if value not in _KNOWN_KEYS:
            raise ValueError(f"Unknown info key: {value}")
        self._value = value

    @property
    def value(self) -> str:
        return self._value


# class InfoKey(enum.Enum):
#     """
#     Optional arguements for the info command. See also,
#     https://docs.bazel.build/versions/main/user-manual.html#info

#     pybazel.help.info_keys()
#     """

#     ### Configuration-independent data
#     # https://docs.bazel.build/versions/main/user-manual.html#configuration-independent-data
#     release = "release"
#     workspace = "workspace"
#     install_base = "install_base"
#     output_base = "output_base"
#     execution_root = "execution_root"
#     output_path = "output_path"
#     server_pid = "server_pid"
#     server_log = "server_log"
#     command_log = "command_log"
#     used_heap_size = "used-heap-size"
#     committed_head_size = "committed-heap-size"
#     max_heap_size = "max-heap-size"
#     gc_time = "gc-time"
#     gc_count = "gc-count"
#     package_path = "package_path"

#     ### Configuration-specific data
#     # https://docs.bazel.build/versions/main/user-manual.html#configuration-specific-data
#     bazel_bin = "bazel-bin"
#     bazel_testlogs = "bazel-testlogs"
#     bazel_genfiles = "bazel-genfiles"
