package ollama

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jmelahman/agent/client/base"
	"github.com/jmelahman/agent/utils"
	"github.com/ollama/ollama/api"
)

type Client struct {
	Client *api.Client
	Model  string
}

func NewClient() Client {
	client, err := api.ClientFromEnvironment()
	utils.Must("instantiate ollama client", err)
	return Client{
		Client: client,
		Model:  "deepseek-r1:8b",
	}
}

func (c Client) GetModel() string {
	return c.Model
}

func (c Client) NewUserMessage(message string) base.Message {
	return base.Message{
		Role: "user",
		Content: []base.Content{
			{Text: message},
		},
	}
}

func (c Client) NewToolMessage(content base.Content) base.Message {
	return base.Message{
		ToolCallID: content.ID,
		Role:       "tool",
		Content: []base.Content{
			content,
		},
	}
}

func (c Client) ExecuteTool(id, name string, input json.RawMessage, tools []base.ToolDefinition) base.Content {
	for _, tool := range tools {
		if tool.Name == name {
			response, err := tool.Function(input)
			if err != nil {
				return base.Content{
					Text:  err.Error(),
					ID:    id,
					Name:  name,
					Input: input,
					Type:  "tool_error",
				}
			}
			return base.Content{
				Text:  response,
				ID:    id,
				Name:  name,
				Input: input,
				Type:  "tool_result",
			}
		}
	}
	return base.Content{
		Text:  "tool not found",
		ID:    id,
		Name:  name,
		Input: input,
		Type:  "tool_error",
	}
}

func (c Client) RunInference(ctx context.Context, messages []base.Message, tools []base.ToolDefinition) (base.Message, error) {
	// We'll use the official client to make the API call

	// Build the chat messages array
	params := []models.ChatMessage{}
	for _, m := range messages {
		msg := models.ChatMessage{
			Role: m.Role,
		}
		if len(m.Content) > 0 {
			// We have an Ollama-specific format for content
			contents := []models.Content{}
			for _, ct := range m.Content {
				// Convert to the official models.Content
				if ct.Type == "text" || ct.Type == "" {
					contents = append(contents, models.Content{
						Content:     ct.Text,
						ContentType: "text",
					})
				} else {
					// For other content types, we might need to support them differently
					// But the official client doesn't handle them in the same way
					contents = append(contents, models.Content{
						Content:     ct.Text,
						ContentType: ct.Type,
					})
				}
			}
			msg.Content = contents
		}
		params = append(params, msg)
	}

	// If we are using tools, set them
	var toolDefs []models.ToolDefinition
	if len(tools) > 0 {
		toolDefs = make([]models.ToolDefinition, len(tools))
		for i, t := range tools {
			toolDefs[i] = models.ToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: *t.InputSchema,
			}
		}
	}

	// Make the API call
	resp, err := c.cfg.Chat(
		ctx,
		models.ChatRequest{
			Messages: params,
			Tools:    toolDefs,
		},
	)

	if err != nil {
		return base.Message{}, err
	}

	// Extract the response content
	respMessage := resp.Message
	respContent := respMessage.Content

	// Convert each content part to base.Content
	var contents []base.Content
	for _, c := range respContent {
		contents = append(contents, base.Content{
			Text: c.Content,
			Type: c.ContentType,
		})
	}

	return base.Message{
		Role:    respMessage.Role,
		Content: contents,
	}, nil
}
