package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"chui/cmd"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "chui",
		Short: "A chat UI application",
		Long:  "chui is a terminal-based chat application with UI capabilities",
	}

	// Add the chat command as the default command
	chatCmd := cmd.NewChatCommand()
	chatCmd.Use = "chat"
	rootCmd.AddCommand(chatCmd)
	rootCmd.RunE = chatCmd.RunE

	// Add the serve command
	serveCmd := cmd.NewServeCommand()
	rootCmd.AddCommand(serveCmd)

	// Add documentation command
	docCmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doc.GenMarkdownTree(rootCmd, "./docs/")
		},
	}
	rootCmd.AddCommand(docCmd)

	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}
