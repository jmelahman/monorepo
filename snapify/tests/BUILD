py_library(
    name = "snapify_testdata",
    srcs = glob(["testdata/*.py"]),
)

py_test(
    name = "snapify_test",
    srcs = ["snapify_test.py"],
    deps = [
        "//snapify",
        ":snapify_testdata",
    ],
)
