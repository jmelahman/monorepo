# Ideas

Things worth doing that nobody has started. Delete an entry when it lands.

## Go build-info artifact rule (PB8xx)

Every binary the Go linker produces carries a build-info section: module
path, dependency list, and the build settings that shaped it (`-trimpath`,
`-buildmode`, `CGO_ENABLED`, `-ldflags`, `vcs.revision`). The standard
library's `debug/buildinfo` reads it straight from the file — the same kind
of read the PB8xx rules already do with `debug/elf`, nothing executed.

A package-scope rule over `pkglint build`'s artifact:

- For every ELF executable, try to read Go build info; non-Go binaries have
  none and are skipped.
- Report a binary whose settings lack `-trimpath=true` (PB915's artifact-side
  twin) and one that lacks `-buildmode=pie` (PB914's).

Why it is stronger than the PKGBUILD-level checks: PB914/PB915 only see
`go build` lines they can parse. A build that runs through `make` (anubis)
is invisible to them, and a GOFLAGS that reaches the build through an
indirection pkglint cannot follow draws a false finding. The build-info rule
reads the outcome and does not care how the flag got there.

Once it exists, PB915's fixer can move from FixUnsafe to FixSafe: the edit
is then verified by the artifact rather than trusted. PB914 stays unsafe —
some programs really are incompatible with PIE (consul says so in a comment)
and only running the result shows it, which is outside pkglint.

Later, the same read can back a `-X main.version` vs pkgver comparison and a
`vcs.revision` vs `#commit=` comparison.

Fixture: a tiny Go binary built with and without the flags, checked in under
the package test data. Roughly a hundred lines with tests.
