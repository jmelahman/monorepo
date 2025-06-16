package tools

import (
	"encoding/json"
	ollama "github.com/ollama/ollama/api"
	"os"
)

var ReadFileDefintion = ToolDefinition{
	Tool: ollama.Tool{
		Type:     "function",
		Function: ReadFileTool,
	},
	Function: ReadFile,
}

var ReadFileTool = ollama.ToolFunction{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this when you want to see what's inside a file. Do not use this with directory names.",
	Parameters: struct {
		Type       string   `json:"type"`
		Defs       any      `json:"$defs,omitempty"`
		Items      any      `json:"items,omitempty"`
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type        ollama.PropertyType `json:"type"`
			Items       any                 `json:"items,omitempty"`
			Description string              `json:"description"`
			Enum        []any               `json:"enum,omitempty"`
		} `json:"properties"`
	}{
		Type:     "object",
		Required: []string{"path"},
		Properties: map[string]struct {
			Type        ollama.PropertyType `json:"type"`
			Items       any                 `json:"items,omitempty"`
			Description string              `json:"description"`
			Enum        []any               `json:"enum,omitempty"`
		}{
			"path": {
				Type:        ollama.PropertyType{"String"},
				Description: "The relative path to the file to read in the working directory",
			},
		},
	},
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}

	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
