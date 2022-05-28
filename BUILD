load("@buildifier_prebuilt//:rules.bzl", "buildifier")

buildifier(
    name = "buildifier",
    exclude_patterns = [
        "./.git/*",
    ],
)

buildifier(
    name = "buildifier.check",
    exclude_patterns = [
        "./.git/*",
    ],
    # TODO
    # lint_mode = "warn",
    mode = "diff",
)
