package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/jmelahman/chui/cmd"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "chui",
		Short: "An AI chat UI application",
		Long:  "chui is a lightweight yet pleasant, terminal-based AI chat client",
	}

	chatCmd := cmd.NewChatCommand()
	chatCmd.Use = "chat"
	rootCmd.AddCommand(chatCmd)
	rootCmd.RunE = chatCmd.RunE

	serveCmd := cmd.NewServeCommand()
	rootCmd.AddCommand(serveCmd)

	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}
