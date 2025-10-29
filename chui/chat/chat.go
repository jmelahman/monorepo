package chat

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Role string

type Message struct {
	// content string
	// role    Role
}

type Conversation struct {
	id       string
	summary  string
	messages []Message
}

func (c Conversation) Title() string       { return c.summary }
func (c Conversation) Description() string { return "" }
func (c Conversation) FilterValue() string { return c.summary }

type model struct {
	conversations list.Model
	chatLog       []string
	input         textinput.Model
	vpMain        viewport.Model
	vpSidebar     viewport.Model
	width         int
	height        int
}

func InitialModel() model {
	items := []list.Item{
		Conversation{id: "1", summary: "Chat with Alice", messages: []Message{}},
		Conversation{id: "2", summary: "Work discussion", messages: []Message{}},
		Conversation{id: "3", summary: "Intermediate cycling training", messages: []Message{}},
		Conversation{id: "4", summary: "How to make a great TUI in Golang", messages: []Message{}},
	}

	l := list.New(items, list.NewDefaultDelegate(), 20, 10)
	l.Title = "Conversations"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	return model{
		conversations: l,
		chatLog:       []string{"Welcome to Ollama Chat!"},
		input:         ti,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) resize(width, height int) {
	m.width = width
	m.height = height

	// Sidebar viewport
	m.vpSidebar = viewport.Model{
		Width:  20,
		Height: height,
	}
	m.vpSidebar.SetContent(m.conversations.View())

	// Main viewport (content + input)
	mainContent := lipgloss.JoinVertical(lipgloss.Top,
		"",
		m.input.View(),
	)
	m.vpMain = viewport.Model{
		Width:  width - 22, // minus sidebar + gap
		Height: height,
	}
	m.vpMain.SetContent(mainContent)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m2 := &m
		m2.resize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if strings.TrimSpace(m.input.Value()) != "" {
				m.chatLog = append(m.chatLog, "You: "+m.input.Value())
				m.chatLog = append(m.chatLog, "Bot: Echo: "+m.input.Value()) // placeholder for assistant
				m.input.SetValue("")
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.conversations, cmd = m.conversations.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	sidebarStyle := lipgloss.NewStyle().Height(m.height - 2).Width(20).Border(lipgloss.RoundedBorder())
	mainStyle := lipgloss.NewStyle().Height(m.height).PaddingLeft(2)

	mainContent := lipgloss.JoinVertical(lipgloss.Top,
		"Hello cruel world",
		m.input.View(),
	)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Render(m.conversations.View()),
		mainStyle.Render(mainContent),
	)
}
