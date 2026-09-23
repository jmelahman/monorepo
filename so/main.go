// Command check-symlinks reports broken symbolic links.
package main

import (
	"solod.dev/so/os"

	"github.com/jmelahman/check-symlinks/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}
