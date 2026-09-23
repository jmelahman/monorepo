// Package version reports the version of the running binary.
package version

import "runtime/debug"

// version is stamped at build time by GoReleaser:
//
//	-ldflags "-X github.com/jmelahman/golang-template/internal/version.version=v1.2.3"
var version string

// Version returns the stamped version, falling back to the module version
// recorded by `go install` and finally to "dev" for `go run` builds.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
