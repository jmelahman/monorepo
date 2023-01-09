from __future__ import annotations

import enum


class InfoKey(enum.Enum):
    """
    Optional arguements for the info command. See also,
    https://docs.bazel.build/versions/main/user-manual.html#info

    pybazel.help.info_keys()
    """

    ### Configuration-independent data
    # https://docs.bazel.build/versions/main/user-manual.html#configuration-independent-data
    release = "release"
    workspace = "workspace"
    install_base = "install_base"
    output_base = "output_base"
    execution_root = "execution_root"
    output_path = "output_path"
    server_pid = "server_pid"
    server_log = "server_log"
    command_log = "command_log"
    used_heap_size = "used-heap-size"
    committed_head_size = "committed-heap-size"
    max_heap_size = "max-heap-size"
    gc_time = "gc-time"
    gc_count = "gc-count"
    package_path = "package_path"

    ### Configuration-specific data
    # https://docs.bazel.build/versions/main/user-manual.html#configuration-specific-data
    bazel_bin = "bazel-bin"
    bazel_testlogs = "bazel-testlogs"
    bazel_genfiles = "bazel-genfiles"
