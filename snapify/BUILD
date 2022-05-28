load("@pydeps//:requirements.bzl", "requirement")

py_binary(
    name = "snapify",
    srcs = ["snapify.py"],
    deps = [
        requirement("requests"),
        requirement("urllib3"),
    ],
)

py_library(
    name = "snapify_testdata",
    srcs = glob(["testdata/*.py"]),
)

py_test(
    name = "snapify_test",
    srcs = ["snapify_test.py"],
    deps = [
        ":snapify",
        ":snapify_testdata",
    ],
)
