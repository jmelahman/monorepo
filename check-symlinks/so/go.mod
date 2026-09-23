// Solod build module. The root module replaces solod.dev with gocompat/ so
// that the plain Go toolchain (what pre-commit's golang language runs)
// produces a functional binary; this nested module builds the same cli
// package against the real solod.dev instead:
//
//	so build -o check-symlinks ./so
//	so test ./so
//
// Replace directives only apply in the main module, so pulling the cli
// package in from the root via `replace` leaves the root's gocompat
// replacement inert here.
module github.com/jmelahman/check-symlinks/so

go 1.26

require (
	github.com/jmelahman/check-symlinks v0.0.0
	solod.dev v0.3.0
)

replace github.com/jmelahman/check-symlinks => ../
