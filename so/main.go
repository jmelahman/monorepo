// Command undot keeps a repository's root free of tool configuration
// clutter by storing config files under .config/ and symlinking them back
// into the root.
package main

import (
	"solod.dev/so/os"

	"github.com/jmelahman/undot/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}
