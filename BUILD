load("@buildifier_prebuilt//:rules.bzl", "buildifier")

buildifier(
    name = "buildifier",
    exclude_patterns = [
        "./.git/*",
    ],
)

buildifier(
    name = "buildifier.check",
    # TODO
    # lint_mode = "warn",
    diff_command = "diff -q",
    exclude_patterns = [
        "./.git/*",
    ],
    mode = "diff",
)
