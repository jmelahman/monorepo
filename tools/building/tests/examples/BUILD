load("@rules_python//python:defs.bzl", "py_binary", "py_library", "py_test")

package(default_visibility = ["//tools/building/tests:__subpackages__"])

py_library(
    name = "greeter_lib",
    srcs = ["greeter.py"],
)

py_binary(
    name = "greeter",
    srcs = ["greeter.py"],
    main = "greeter.py",
    deps = [
        ":greeter_lib",
    ],
)

py_test(
    name = "greeter_test",
    srcs = ["greeter_test.py"],
    deps = [
        ":greeter_lib",
    ],
)
