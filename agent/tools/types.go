package tools

import (
	"encoding/json"
	ollama "github.com/ollama/ollama/api"
)

type ToolDefinition struct {
	Tool     ollama.Tool
	Function func(input json.RawMessage) (string, error)
}
