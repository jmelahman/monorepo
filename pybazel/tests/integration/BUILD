load("@rules_python//python:defs.bzl", "py_library", "py_test")

py_library(
    name = "fixtures",
    srcs = ["fixtures.py"],
    tags = [
        "manual",
        "integration",
    ],
    deps = [
        "//pybazel/pybazel/api:client",
    ],
)

py_test(
    name = "info_test",
    srcs = ["info_test.py"],
    deps = [
        ":fixtures",
        "//pybazel/pybazel/models:info",
    ],
)
