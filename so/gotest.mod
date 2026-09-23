// Alternate modfile for running the solod test suite under the regular Go
// toolchain against the gocompat implementations:
//
//	go -C so run -modfile=gotest.mod ./test
//
// Both dependencies resolve to local directories, so this needs no network
// and no sum file.
module github.com/jmelahman/check-symlinks/so

go 1.26

require (
	github.com/jmelahman/check-symlinks v0.0.0
	solod.dev v0.3.0
)

replace github.com/jmelahman/check-symlinks => ../

replace solod.dev => ../gocompat
