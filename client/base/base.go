package base

import (
	"context"
	"encoding/json"

	"github.com/revrost/go-openrouter/jsonschema"
)

type Content struct {
	Text  string
	ID    string
	Name  string
	Type  string
	Input json.RawMessage
}

type Message struct {
	ToolCallID string    `json:"tool_call_id,omitempty"`
	Role       string    `json:"role"`
	Content    []Content `json:"content,omitzero"`
}

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// InputSchema is a standardized JSON schema definition for the tool's input.
	InputSchema jsonschema.Definition `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

type Client interface {
	GetModel() string
	RunInference(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error)
	NewUserMessage(message string) Message
	NewToolMessage(content Content) Message
	ExecuteTool(id, name string, input json.RawMessage, tools []ToolDefinition) Content
}
