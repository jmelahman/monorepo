load("@pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_test")

py_test(
    name = "logging_test",
    srcs = ["logging_test.py"],
    deps = [
        "//buildprint:logging",
        requirement("freezegun"),
    ],
)

py_test(
    name = "git_test",
    srcs = ["git_test.py"],
    deps = [
        "//buildprint/client:git",
    ],
)
