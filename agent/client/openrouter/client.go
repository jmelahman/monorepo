package openrouter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmelahman/agent/client/base"
	"github.com/jmelahman/agent/client/common" // Added import for common package
	openrouter "github.com/revrost/go-openrouter"
	log "github.com/sirupsen/logrus"
)

func NewClient(key string) Client {
	client := openrouter.NewClient(
		key,
		openrouter.WithXTitle("Agent"),
		openrouter.WithHTTPReferer("https://jamison.lahman.dev"),
	)
	return Client{
		Client: client,
		Model:  "deepseek/deepseek-chat-v3-0324:free", // Supports tools
		//Model: "mistralai/devstral-small:free", // Supports tools
		//Model: "deepseek/deepseek-r1-0528-qwen3-8b:free",
		//Model: "deepseek/deepseek-r1-0528:free",
	}
}

type Client struct {
	Client *openrouter.Client
	Model  string
}

func (c Client) GetModel() string {
	return c.Model
}

func (c Client) NewUserMessage(message string) base.Message {
	return base.Message{
		Role: openrouter.ChatMessageRoleUser,
		Content: []base.Content{
			{Text: message},
		},
	}
}

func (c Client) NewToolResult(id, output string, failed bool) base.Content {
	return base.Content{
		ID:   id,
		Text: output,
	}
}

func (c Client) NewToolMessage(content base.Content) base.Message {
	return base.Message{
		ToolCallID: content.ID,
		Role:       openrouter.ChatMessageRoleTool,
		Content: []base.Content{
			{Text: content.Text},
		},
	}
}

func (c Client) RunInference(
	ctx context.Context,
	messages []base.Message,
	tools []base.ToolDefinition,
) (base.Message, error) {
	input := make([]openrouter.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		content, err := c.convertContent(m.Content[0])
		log.Error("convert message content", err) // This log seems to always show nil error, consider removing or changing level if not an actual error.
		input[i] = openrouter.ChatCompletionMessage{
			Role:    m.Role,
			Content: content,
		}
	}

	openrouterTools := []openrouter.Tool{}
	for _, tool := range tools {
		orTool, err := common.AdaptBaseToolToOpenRouterTool(tool)
		if err != nil {
			log.Errorf("Failed to adapt tool '%s' for OpenRouter: %v. Skipping tool.", tool.Name, err)
			continue
		}
		openrouterTools = append(openrouterTools, orTool)
	}

	resp, err := c.Client.CreateChatCompletion(
		ctx,
		openrouter.ChatCompletionRequest{
			Model:    c.Model,
			Messages: input,
			Tools:    openrouterTools,
		},
	)
	if err != nil {
		return base.Message{}, err
	}

	msg := resp.Choices[0].Message
	var content []base.Content
	if msg.Content.Text != "" {
		content = append(content, base.Content{Text: msg.Content.Text, Type: "text"})
	}
	for _, toolCall := range msg.ToolCalls {
		var args json.RawMessage
		for _, tool := range tools {
			if tool.Name == toolCall.Function.Name {
				if err := json.Unmarshal([]byte(msg.ToolCalls[0].Function.Arguments), &args); err != nil {
					return base.Message{}, fmt.Errorf("Error unmarshalling arguments: %v\n", err)
				}
				break
			}
		}
		content = append(content, base.Content{
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Text:  msg.Content.Text,
			Input: args,
			Type:  "tool_use",
		})
	}
	return base.Message{Role: msg.Role, Content: content}, nil
}

func (c Client) ExecuteTool(
	id string,
	name string,
	input json.RawMessage,
	tools []base.ToolDefinition,
) base.Content {
	var toolDef base.ToolDefinition
	var found bool
	for _, tool := range tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return c.NewToolResult(id, "tool not found", true)
	}

	fmt.Printf("\u001b[93mtool\u001b[0m: %s(%s)\n", name, input)
	response, err := toolDef.Function(input)
	if err != nil {
		return c.NewToolResult(id, err.Error(), true)
	}
	return c.NewToolResult(id, response, false)
}

func (c Client) convertContent(a any) (openrouter.Content, error) {
	switch v := a.(type) {
	case string:
		return openrouter.Content{Text: v}, nil
	case openrouter.Content:
		return v, nil
	case base.Content:
		return openrouter.Content{
			Text: v.Text,
		}, nil
	default:
		return openrouter.Content{}, fmt.Errorf("unsupported content type: %T", v)
	}
}
