load("@rules_cc//cc:defs.bzl", "cc_test")

cc_test(
    name = "fibonacci_test",
    srcs = ["fibonacci_test.cc"],
    tags = ["manual"],
    deps = [
        "//physics/lib:fibonacci",
        "@com_google_googletest//:gtest_main",
    ],
)
