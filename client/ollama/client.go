package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jmelahman/agent/client/base"
	"github.com/jmelahman/agent/utils"
	ollamaapi "github.com/ollama/ollama/api"
	log "github.com/sirupsen/logrus"
)

// Default Ollama host and model
const (
	defaultOllamaHost  = "http://localhost:11434"
	defaultOllamaModel = "llama3:latest" // A model known to support tools; adjust if necessary
)

type Client struct {
	ollamaClient *ollama.Client
	Model        string
}

// NewClient creates and configures a new Ollama client.
// It uses OLLAMA_HOST and OLLAMA_MODEL environment variables if set,
// otherwise defaults to localhost and a predefined model.
func NewClient() *Client {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = defaultOllamaHost
	}

	oclient, err := ollama.New(ollama.WithHost(host))
	utils.Must(fmt.Sprintf("create Ollama client with host %s", host), err)

	// Check if the Ollama server is responding
	// Note: Version() is not a method on ollama.Client.
	// A simple request like List or Heartbeat can be used.
	// Let's use List as a simple connectivity check.
	_, err = oclient.List(context.Background())
	if err != nil {
		log.Warnf("Ollama server at %s may not be responding or an error occurred: %v. Ensure Ollama is running and OLLAMA_HOST is set correctly.", host, err)
		// Depending on desired behavior, could exit or allow to proceed and fail later.
		// utils.Must will panic, so if we want to proceed, we should just log.
		// For now, let's be strict as per utils.Must usage elsewhere.
		utils.Must(fmt.Sprintf("connect to Ollama server at %s", host), err)
	}
	log.Infof("Successfully connected to Ollama at %s", host)

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = defaultOllamaModel
	}

	// Optional: Check if the model exists locally.
	_, err = oclient.Show(context.Background(), &ollamaapi.ShowRequest{Name: model})
	if err != nil {
		log.Warnf("Model '%s' not found locally or Ollama error: %v. Ensure the model is pulled using 'ollama pull %s'.", model, err, model)
		// Proceeding, assuming the model might be valid but not yet pulled, or an alias.
	} else {
		log.Infof("Using Ollama model: %s", model)
	}

	return &Client{
		ollamaClient: oclient,
		Model:        model,
	}
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
	apiMessages := make([]ollamaapi.Message, len(messages))
	for i, m := range messages {
		apiMsg := ollamaapi.Message{Role: m.Role}
		var toolCalls []ollamaapi.ToolCall
		var textParts []string

		for _, contentItem := range m.Content {
			// Tool calls in history are expected to be on "assistant" messages.
			if contentItem.Type == "tool_use" && m.Role == "assistant" {
				toolCalls = append(toolCalls, ollamaapi.ToolCall{
					Function: ollamaapi.ToolCallFunction{
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

	apiTools := []ollamaapi.Tool{}
	if len(tools) > 0 {
		for _, tool := range tools {
			apiTools = append(apiTools, ollamaapi.Tool{
				Type: "function",
				Function: ollamaapi.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			})
		}
	}

	req := ollamaapi.ChatRequest{
		Model:    c.Model,
		Messages: apiMessages,
		Format:   "json", // Request JSON mode for better tool argument generation.
		// Stream: false by default.
	}
	if len(apiTools) > 0 {
		req.Tools = apiTools
		log.Debugf("Requesting Ollama with %d tools.", len(apiTools))
	}

	var finalResponse ollamaapi.ChatResponse
	err := c.ollamaClient.Chat(ctx, &req, func(r ollamaapi.ChatResponse) error {
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
			responseContentItems = append(responseContentItems, base.Content{
				ID:    "", // Ollama does not provide an ID for tool calls.
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments, // json.RawMessage
				Type:  "tool_use",
			})
			log.Debugf("Tool Call to Execute: Name: %s, Args: %s", tc.Function.Name, string(tc.Function.Arguments))
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
