load("@rules_python//python:defs.bzl", "py_library", "py_test")

py_library(
    name = "snapify_testdata",
    srcs = glob(["testdata/*.py"]),
)

py_test(
    name = "snapify_test",
    srcs = ["snapify_test.py"],
    deps = [
        ":snapify_testdata",
        "//snapify/pysnapify:main",
        "//snapify/pysnapify:constants",
    ],
)
