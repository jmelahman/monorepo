package tools

import (
	"encoding/json"
	"os"
	"path/filepath"

	ollama "github.com/ollama/ollama/api"
)

var ListFilesDefinition = ToolDefinition{
	Tool: ollama.Tool{
		Type:     "function",
		Function: ListFilesTool,
	},
	Function: ListFiles,
}

var ListFilesTool = ollama.ToolFunction{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, lists files in the current directory.",
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
		Type: "object",
		Properties: map[string]struct {
			Type        ollama.PropertyType `json:"type"`
			Items       any                 `json:"items,omitempty"`
			Description string              `json:"description"`
			Enum        []any               `json:"enum,omitempty"`
		}{
			"path": {
				Type:        ollama.PropertyType{"String"},
				Description: "Optional relative path to list files from. Defaults to current directory if not provided.",
			},
		},
	},
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
