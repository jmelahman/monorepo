package main

import (
	"fmt"
	"os"

	"github.com/jmelahman/kanban/cmd/server"
	"github.com/jmelahman/kanban/internal/sshsig"
)

func main() {
	// Kanban is its own gpg.ssh.program when [git] sign_commits is on, and git
	// execs that program directly (no shell) with ssh-keygen's arguments, so
	// there is no room for a subcommand name: a leading -Y is the signal.
	// Cobra would reject it as an unknown flag anyway.
	if len(os.Args) > 1 && os.Args[1] == "-Y" {
		os.Exit(sshsig.Main(os.Args[1:]))
	}

	if err := server.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
