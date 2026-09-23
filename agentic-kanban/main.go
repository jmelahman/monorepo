package main

import (
	"fmt"
	"os"

	"github.com/jmelahman/kanban/cmd/server"
)

func main() {
	if err := server.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
