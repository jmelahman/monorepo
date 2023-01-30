load("@rules_python//python:defs.bzl", "py_test")
load("@pip_deps//:requirements.bzl", "requirement")

py_test(
    name = "logging_test",
    srcs = ["logging_test.py"],
    deps = [
        "//buildprint:logging",
        requirement("freezegun"),
    ],
)
