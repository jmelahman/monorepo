# Buildprint

Objectives:

- High-level abstraction on top of builkdite
  - https://buildkite.com/blog/how-bazel-built-its-ci-system-on-top-of-buildkite
- handles sharding natively
  - https://github.com/philwo/bazel-utils/blob/main/sharding/sharding.py for reference
- Suppports a culprit finder (stretch goal)
- Can run locally
  - This might be obsolete if I can figure out how to execute a pipeline with the buildite-agent locally
- Two staged upload:
  - git-based "smoke tests" then bazel-based full suite.
  - ideally this is one step with the git-based operations in the background
    - any way to immediately determine bazel is unrequired is a +
  - This need bazel-diff integration to this end
    - If it does, consider using worktrees to execute bazel-diff in
- has a public schema that can be used for validation
  - proto-backed?