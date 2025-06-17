package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	ollama "github.com/ollama/ollama/api"
)

var EditFileDefintion = ToolDefinition{
	Tool: ollama.Tool{
		Type:     "function",
		Function: EditFileTool,
	},
	Function: EditFile,
}

var EditFileTool = ollama.ToolFunction{
	Name: "edit_file",
	Description: `Make edits to a text file.

Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

If the file specified with path doesn't exist, it will be created.
`,
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
		Required: []string{"path", "old_str", "new_str"},
		Properties: map[string]struct {
			Type        ollama.PropertyType `json:"type"`
			Items       any                 `json:"items,omitempty"`
			Description string              `json:"description"`
			Enum        []any               `json:"enum,omitempty"`
		}{
			"path": {
				Type:        ollama.PropertyType{"String"},
				Description: "The path to the file",
			},
			"old_str": {
				Type:        ollama.PropertyType{"String"},
				Description: "Text to search for - must match exactly and must only have one match exactly",
			},
			"new_str": {
				Type:        ollama.PropertyType{"String"},
				Description: "Text to replace old_str with",
			},
		},
	},
}

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for - must match exactly and must only have one match exactly"`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters")
	}

	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldStr == "" {
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		return "", err
	}

	oldContent := string(content)
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)

	if oldContent == newContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("old_str not found in file")
	}

	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		return "", err
	}

	return "OK", nil
}

func createNewFile(filePath, content string) (string, error) {
	dir := path.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	return fmt.Sprintf("Successfully created file %s", filePath), nil
}
