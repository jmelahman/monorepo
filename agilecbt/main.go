package main

import (
	"fmt"
	"os"

	"github.com/jmelahman/agilecbt/cmd/server"
)

func main() {
	if err := server.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
