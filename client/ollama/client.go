package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jmelahman/agent/client/base"
	ollama "github.com/ollama/ollama/api"
	log "github.com/sirupsen/logrus"
)

const (
	defaultOllamaModel = "qwen3:0.6b"
)

type Client struct {
	Client *ollama.Client
	Model  string
}

// NewClient creates and configures a new Ollama client.
// It uses OLLAMA_HOST and OLLAMA_MODEL environment variables if set,
// otherwise defaults to localhost and a predefined model.
func NewClient() (*Client, error) {
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = defaultOllamaModel
	}

	client, err := ollama.ClientFromEnvironment()
	if err != nil {
		return &Client{}, err
	}

	return &Client{
		Client: client,
		Model:  model,
	}, nil
}

func (c *Client) GetModel() string {
	return c.Model
}

func (c *Client) NewUserMessage(message string) base.Message {
	return base.Message{
		Role: "user",
		Content: []base.Content{
			{Text: message, Type: "text"},
		},
	}
}

// newToolResult is an internal helper to structure tool execution results.
// Ollama tool calls do not have an 'ID', so the id field in base.Content will be empty.
func (c *Client) newToolResult(id string, output string, failed bool) base.Content {
	return base.Content{
		ID:   id, // Will be empty as Ollama doesn't provide tool_call_id.
		Text: output,
		Type: "tool_result", // Internal type, not for LLM.
		// Name is not relevant for a tool result content.
	}
}

func (c *Client) NewToolMessage(content base.Content) base.Message {
	// content.ID is the ID of the tool call that generated this result.
	// For Ollama, this ID will be empty.
	// A tool result message for Ollama is: { "role": "tool", "content": "<result>" }
	return base.Message{
		ToolCallID: content.ID, // Will be empty for Ollama.
		Role:       "tool",
		Content: []base.Content{
			{Text: content.Text, Type: "text"}, // The result text.
		},
	}
}

func (c *Client) RunInference(
	ctx context.Context,
	messages []base.Message,
	tools []base.ToolDefinition,
) (base.Message, error) {
	apiMessages := make([]ollama.Message, len(messages))
	for i, m := range messages {
		apiMsg := ollama.Message{Role: m.Role}
		var toolCalls []ollama.ToolCall
		var textParts []string

		for _, contentItem := range m.Content {
			// Tool calls in history are expected to be on "assistant" messages.
			if contentItem.Type == "tool_use" && m.Role == "assistant" {
				toolCalls = append(toolCalls, ollama.ToolCall{
					Function: ollama.ToolCallFunction{
						Name:      contentItem.Name,
						Arguments: contentItem.Input, // json.RawMessage
					},
				})
			} else if contentItem.Text != "" {
				// This handles user text, tool result text (role: "tool"), and assistant text.
				textParts = append(textParts, contentItem.Text)
			}
		}

		if len(textParts) > 0 {
			apiMsg.Content = strings.Join(textParts, "\n")
		}
		if len(toolCalls) > 0 {
			apiMsg.ToolCalls = toolCalls
		}

		// If an assistant message has only tool_calls, Content should be empty.
		// If textParts is empty, apiMsg.Content remains its zero value (empty string).

		apiMessages[i] = apiMsg
		log.Debugf("Ollama Input Message History [%d]: Role: %s, Content: '%s', ToolCalls: %d", i, apiMsg.Role, apiMsg.Content, len(apiMsg.ToolCalls))
	}

	apiTools := []ollama.Tool{}
	if len(tools) > 0 {
		for _, tool := range tools {
			apiTools = append(apiTools, ollama.Tool{
				Type: "function",
				Function: ollama.ToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			})
		}
	}

	stream := false
	req := ollama.ChatRequest{
		Model:    c.Model,
		Messages: apiMessages,
		Stream:   &stream,
		// Format:   "json", // Request JSON mode for better tool argument generation.
	}
	if len(apiTools) > 0 {
		req.Tools = apiTools
		log.Debugf("Requesting Ollama with %d tools.", len(apiTools))
	}

	var finalResponse ollama.ChatResponse
	err := c.Client.Chat(ctx, &req, func(r ollama.ChatResponse) error {
		// For non-streaming, this callback is invoked once with the complete response.
		finalResponse = r
		return nil
	})

	if err != nil {
		return base.Message{}, fmt.Errorf("ollama chat API error: %w", err)
	}

	// Process the captured response (finalResponse.Message)
	assistantMessage := base.Message{Role: finalResponse.Message.Role}
	var responseContentItems []base.Content

	if finalResponse.Message.Content != "" {
		responseContentItems = append(responseContentItems, base.Content{
			Text: finalResponse.Message.Content,
			Type: "text",
		})
		log.Debugf("Ollama Response (Text): %s", finalResponse.Message.Content)
	}

	if len(finalResponse.Message.ToolCalls) > 0 {
		log.Debugf("Ollama Response: %d tool_call(s) received.", len(finalResponse.Message.ToolCalls))
		for _, tc := range finalResponse.Message.ToolCalls {
			// Ollama's ToolCall does not have an ID.
			// tc.Function.Arguments is map[string]interface{}, needs to be marshalled to json.RawMessage
			argumentsJSON, err := json.Marshal(tc.Function.Arguments)
			if err != nil {
				log.Errorf("Failed to marshal tool call arguments for %s: %v", tc.Function.Name, err)
				return base.Message{}, fmt.Errorf("failed to marshal tool call arguments for tool %s: %w", tc.Function.Name, err)
			}

			responseContentItems = append(responseContentItems, base.Content{
				ID:    "", // Ollama does not provide an ID for tool calls.
				Name:  tc.Function.Name,
				Input: argumentsJSON,
				Type:  "tool_use",
			})
		}
	}
	assistantMessage.Content = responseContentItems

	if len(assistantMessage.Content) == 0 {
		log.Warn("Ollama response message has no text content or tool calls. This might indicate an issue or an empty response from the model.")
		// Return an empty assistant message to avoid breaking the chat loop.
		assistantMessage.Content = []base.Content{{Text: "", Type: "text"}}
	}

	return assistantMessage, nil
}

func (c *Client) ExecuteTool(
	id string, // This ID will be empty as it originates from the model's tool_call, which Ollama doesn't ID.
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
		errMsg := fmt.Sprintf("tool not found: %s", name)
		log.Errorf(errMsg)
		return c.newToolResult(id, errMsg, true) // id will be empty
	}

	fmt.Printf("\u001b[93mtool\u001b[0m: %s(%s)\n", name, string(input))
	response, err := toolDef.Function(input)
	if err != nil {
		errMsg := fmt.Sprintf("error executing tool %s: %v", name, err)
		log.Errorf(errMsg)
		return c.newToolResult(id, errMsg, true) // id will be empty
	}
	log.Debugf("Tool %s executed successfully, result: %s", name, response)
	return c.newToolResult(id, response, false) // id will be empty
}
