load("@buildifier_prebuilt//:rules.bzl", "buildifier")
load("@com_github_aignas_rules_shellcheck//:def.bzl", "shellcheck_test")

py_binary(
  name = "program",
  srcs = ["program.py"],
  deps = [
      "@pip_deps//requests:pkg",
  ],
)

filegroup(
  name = "program_zip",
  srcs = [":program"],
  output_group = "python_zip_file",
)

genrule(
  name = "program_zip_py_executable",
  srcs = [":program_zip"],
  outs = ["program_zip_py_executable.par"],
  cmd = "echo '#!/usr/bin/env python3' | cat - $< >$@",
  executable = True,
)

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
        exclude = ["bazel-*/**/*"],
    ),
)
