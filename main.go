// pkglint is a security-focused linter for Arch Linux PKGBUILDs.
//
// It statically analyzes PKGBUILDs and install scriptlets — never sourcing
// them — and reports integrity, hermeticity, and code-execution findings
// with an overall letter grade per package.
//
// The command itself lives in internal/cli; this file is the shim that makes
// the README's install line — `go install github.com/jmelahman/pkglint@latest`
// — resolve to it.
package main

import (
	"os"

	"github.com/jmelahman/pkglint/internal/cli"
)

func main() {
	os.Exit(cli.Main())
}
