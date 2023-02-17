load("@rules_python//python:defs.bzl", "py_library", "py_test")

py_library(
    name = "snapify_testdata",
    srcs = glob(["testdata/*.py"]),
)

py_test(
    name = "snapifier_test",
    srcs = ["snapifier_test.py"],
    deps = [
        ":snapify_testdata",
        "//pysnapify:constants",
        "//pysnapify:snapifier",
    ],
)

py_test(
    name = "pacman_test",
    srcs = ["pacman_test.py"],
    deps = [
        "//pysnapify/manager:pacman",
    ],
)

py_test(
    name = "snap_test",
    srcs = ["snap_test.py"],
    deps = [
        ":snapify_testdata",
        "//pysnapify/manager:snap",
    ],
)
