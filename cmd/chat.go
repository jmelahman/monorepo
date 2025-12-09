package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmelahman/chui/chat"
	"github.com/spf13/cobra"
)

func NewChatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start a chat session",
		Long:  "Start an interactive chat session in the terminal",
		RunE:  runChat,
	}
	return cmd
}

func runChat(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(chat.InitialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
