// A stand-in for solod.dev, backed by the Go standard library.
//
// The real solod.dev packages are transpiler stubs: their bodies are C
// declarations that only work once `so` has translated the program, so a
// plain `go build` of this repository would fail to link against them. The
// root module replaces solod.dev with this module so the Go toolchain can
// build check-symlinks unaided — that is what pre-commit's `golang` language
// does for the hook in .pre-commit-hooks.yaml.
//
// Only the API surface check-symlinks uses is implemented, and only as far
// as check-symlinks uses it. Solod's allocator- and buffer-taking signatures
// are kept so the same source compiles both ways; under Go the garbage
// collector owns the memory, so those arguments are accepted and ignored.
module solod.dev

go 1.22
