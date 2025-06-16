package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/jmelahman/agent/client/base"
	"github.com/jmelahman/agent/client/ollama"
	"github.com/jmelahman/agent/client/openrouter"
	"github.com/jmelahman/agent/tools"
	log "github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	log.SetLevel(log.DebugLevel)

	_ = openrouter.NewClient(os.Getenv("OPENROUTER_API_KEY"))
	// client := openrouter.NewClient(os.Getenv("OPENROUTER_API_KEY"))
	client := ollama.NewClient()

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	tools := []base.ToolDefinition{
		tools.ReadFileDefinition,
	}
	agent := NewAgent(client, getUserMessage, tools)
	err := agent.Run(context.Background())
	must("run agent", err)
}

func NewAgent(
	client base.Client,
	getUserMessage func() (string, bool),
	tools []base.ToolDefinition,
) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

type Agent struct {
	client         base.Client
	getUserMessage func() (string, bool)
	tools          []base.ToolDefinition
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []base.Message{}

	fmt.Printf("Chat with an Agent (%s)\nModel: %s\n", version, a.client.GetModel())

	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok {
				break
			}

			userMessage := a.client.NewUserMessage(userInput)
			conversation = append(conversation, userMessage)
		}

		log.Debug("Running inference...")
		message, err := a.client.RunInference(ctx, conversation, a.tools)
		must("run inference", err)
		conversation = append(conversation, message)

		log.Debug("Parsing messages...")
		toolResults := []base.Content{}
		for _, content := range message.Content {
			switch content.Type {
			case "text":
				fmt.Printf("\u001b[92mAgent\u001b[0m: %s\n", content.Text)
			case "tool_use":
				result := a.client.ExecuteTool(
					content.ID,
					content.Name,
					content.Input,
					a.tools,
				)
				toolResults = append(toolResults, result)
				conversation = append(conversation, a.client.NewToolMessage(result))
			}
		}
		if len(toolResults) == 0 {
			readUserInput = true
			continue
		}
		readUserInput = false
	}

	return nil
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
