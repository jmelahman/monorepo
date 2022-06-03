load("@buildifier_prebuilt//:rules.bzl", "buildifier")
load("@com_github_aignas_rules_shellcheck//:def.bzl", "shellcheck_test")

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

shellcheck_test(
    name = "shellcheck",
    data = glob(
        ["**/*.sh"],
        exclude = ["bazel-*"],
    ),
    tags = ["lint"],
)
