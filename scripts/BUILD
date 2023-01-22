load("@bazel_bats//:rules.bzl", "bats_test")

sh_binary(
    name = "pre_push",
    srcs = ["pre-push.bash"],
)

bats_test(
    name = "pre_push_test",
    srcs = ["pre-push_test.bats"],
    deps = [
        ":pre_push",
        "@bats_shellmock//:shellmock",
    ],
)
