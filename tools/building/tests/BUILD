load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_test")

py_test(
    name = "build_test",
    srcs = ["build_test.py"],
    data = [
        "//tools/building/tests/examples:greeter",
        "//tools/building/tests/examples:greeter_lib",
        "//tools/building/tests/examples:greeter_test",
    ],
    tags = ["manual"],
    deps = [
        requirement("pybazel"),
    ],
)
