// Root module, used by `go build` and `go install ./...` — which is what
// pre-commit's `golang` language runs for the hook in .pre-commit-hooks.yaml.
// solod.dev is replaced with gocompat/, a small Go-stdlib-backed
// implementation of the solod APIs this tool uses, because the real
// solod.dev packages are transpiler stubs that only become functional once
// translated to C. The solod toolchain builds the same cli package against
// the real solod.dev through the nested module in so/ (see so/go.mod).
//
// Note: `go install github.com/jmelahman/check-symlinks@version` is refused
// by Go because of the replace directive — that is intentional. Install a
// release binary, the PyPI package, or run `go install ./...` from a
// checkout.
module github.com/jmelahman/check-symlinks

go 1.22

require solod.dev v0.3.0

replace solod.dev => ./gocompat
