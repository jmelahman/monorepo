// Root module, used by `go build` / `go install ./...` (which is what
// pre-commit's golang language runs): solod.dev is replaced with gocompat/,
// a small Go-stdlib-backed implementation of the solod APIs this tool uses,
// because the real solod.dev packages are transpiler stubs that only become
// functional when translated to C. The solod toolchain builds against the
// real solod.dev through the nested module in so/ (see so/go.mod).
//
// Note: `go install github.com/jmelahman/undot@version` is refused by
// Go because of the replace directive — that's intentional; install from a
// release binary or a local checkout instead.
module github.com/jmelahman/undot

go 1.22

require solod.dev v0.3.0

replace solod.dev => ./gocompat
