load("@buildifier_prebuilt//:rules.bzl", "buildifier")

buildifier(
    name = "buildifier",
    exclude_patterns = [
        "./.git/*",
    ],
)

buildifier(
    name = "buildifier.check",
    diff_command = "diff -q",
    exclude_patterns = [
        "./.git/*",
    ],
    lint_mode = "warn",
    lint_warnings = ["all"],
    mode = "diff",
)
