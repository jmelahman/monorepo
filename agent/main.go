package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jmelahman/agent/tools"
	ollama "github.com/ollama/ollama/api"
	log "github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	log.SetLevel(log.DebugLevel)

	client, err := ollama.ClientFromEnvironment()
	must("initialize ollama client", err)

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	tools := []tools.ToolDefinition{
		tools.EditFileDefintion,
		tools.ReadFileDefintion,
		tools.ListFilesDefinition,
	}
	agent := NewAgent(client, getUserMessage, tools)
	must("run agent", agent.Run(context.TODO()))
}

func NewAgent(client *ollama.Client, getUserMessage func() (string, bool), tools []tools.ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		model:          "mistral",
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

type Agent struct {
	client         *ollama.Client
	model          string
	getUserMessage func() (string, bool)
	tools          []tools.ToolDefinition
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []ollama.Message{}

	fmt.Printf("Chat with an Agent (%s)\nModel: %s\n", version, a.model)

	resp, err := a.client.List(ctx)
	must("list models", err)
	found := false
	for _, m := range resp.Models {
		if m.Model == a.model || m.Model == a.model+":latest" {
			found = true
			break
		}
	}
	if !found {
		req := &ollama.PullRequest{Model: a.model}
		progressFunc := func(resp ollama.ProgressResponse) error {
			fmt.Printf("Progress: status=%v, total=%v, completed=%v\r", resp.Status, resp.Total, resp.Completed)
			return nil
		}
		must("pull model", a.client.Pull(ctx, req, progressFunc))
		// Clear progress.
		fmt.Printf("                                                                               \r")
	}

	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok {
				break
			}

			userMessage := ollama.Message{Role: "user", Content: userInput}
			conversation = append(conversation, userMessage)
		}
		readUserInput = true

		log.Debug("Running inference...")
		message, err := a.runInference(ctx, conversation)
		must("run inference", err)

		if message.Content != "" {
			fmt.Printf("\u001b[92mAgent\u001b[0m: %s\n", message.Content)
			conversation = append(conversation, *message)
		}

		log.Debug("Parsing messages...")
		for _, tc := range message.ToolCalls {
			result := a.executeTool(tc.Function.Name, json.RawMessage(tc.Function.Arguments.String()))
			conversation = append(conversation, result)
			readUserInput = false
		}
	}

	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []ollama.Message) (*ollama.Message, error) {
	var tools []ollama.Tool
	for _, t := range a.tools {
		tools = append(tools, t.Tool)
	}
	stream := false
	req := ollama.ChatRequest{
		Model:    a.model,
		Messages: conversation,
		Stream:   &stream,
		Tools:    tools,
	}
	var finalResponse ollama.ChatResponse
	err := a.client.Chat(ctx, &req, func(r ollama.ChatResponse) error {
		finalResponse = r
		return nil
	})

	return &finalResponse.Message, err
}

func (a *Agent) executeTool(name string, input json.RawMessage) ollama.Message {
	var toolDef tools.ToolDefinition
	var found bool
	for _, tool := range a.tools {
		if tool.Tool.Function.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return newToolResult("tool not found")
	}

	fmt.Printf("\u001b[93mtool\u001b[0m: %s(%s)\n", name, input)
	response, err := toolDef.Function(input)
	if err != nil {
		return newToolResult(err.Error())
	}
	return newToolResult(response)
}

func newToolResult(text string) ollama.Message {
	return ollama.Message{Content: text, Role: "tool"}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
