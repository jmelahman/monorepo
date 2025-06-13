package common

import (
	"encoding/json"

	"github.com/jmelahman/agent/client/base"
	ollama "github.com/ollama/ollama/api"
	openrouter "github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
	log "github.com/sirupsen/logrus"
)

func convertStringSliceToInterface(strs []string) []interface{} {
	interfaces := make([]interface{}, len(strs))
	for i, s := range strs {
		interfaces[i] = s
	}
	return interfaces
}

// ToolFunctionProperty defines the expected structure for Ollama's tool function properties
type ToolFunctionProperty struct {
	Type        string                 `json:"type"`
	Items       map[string]interface{} `json:"items,omitempty"`
	Description string                 `json:"description,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
}

// AdaptBaseToolToOllamaTool converts a base.ToolDefinition to an ollama.Tool.
// It translates the jsonschema.Definition from base.ToolDefinition.InputSchema
// into the structure expected by ollama.ToolFunction.Parameters.
func AdaptBaseToolToOllamaTool(td base.ToolDefinition) (ollama.Tool, error) {
	// Define the anonymous struct type that ollama.ToolFunction.Parameters expects.
	// This structure is based on the ollama API's internal representation.
	type OllamaToolParameters struct {
		Type       string                          `json:"type"`
		Defs       any                             `json:"$defs,omitempty"` // Not currently mapped from jsonschema.Definition
		Items      any                             `json:"items,omitempty"`  // For schema of type array
		Required   []string                        `json:"required,omitempty"`
		Properties map[string]ToolFunctionProperty `json:"properties,omitempty"`
	}

	paramsForOllama := OllamaToolParameters{}

	// Use the InputSchema from base.ToolDefinition
	schemaDef := td.InputSchema

	// If schemaDef is the zero value, treat as a tool with no parameters.
	// Ollama expects type "object" and an empty properties map for this.
	if schemaDef.Type == "" && len(schemaDef.Properties) == 0 && schemaDef.Items == nil && len(schemaDef.Required) == 0 {
		paramsForOllama.Type = "object"
		paramsForOllama.Properties = make(map[string]ToolFunctionProperty)
	} else {
		paramsForOllama.Type = string(schemaDef.Type)
		paramsForOllama.Required = schemaDef.Required

		if schemaDef.Type == jsonschema.Array && schemaDef.Items != nil {
			// Convert jsonschema.Definition.Items (*jsonschema.Definition)
			// to the 'any' type expected by OllamaToolParameters.Items.
			// This simplified conversion creates a map like {"type": "string"}.
			// For complex item types, this part might need expansion.
			itemDef := schemaDef.Items
			paramsForOllama.Items = map[string]interface{}{
				"type": string(itemDef.Type),
			}
			// If itemDef.Properties, itemDef.Enum etc. need to be mapped, add logic here.
		}

		if len(schemaDef.Properties) > 0 {
			paramsForOllama.Properties = make(map[string]ToolFunctionProperty)
			for name, propDef := range schemaDef.Properties {
				// Convert jsonschema.DataType to string for Ollama
				propType := string(propDef.Type)
				ollamaProp := ToolFunctionProperty{
					Type:        propType,
					Description: propDef.Description,
					Enum:        convertStringSliceToInterface(propDef.Enum),
				}

				// Handle 'items' for array properties within Properties
				if propDef.Type == jsonschema.Array && propDef.Items != nil {
					itemSchema := propDef.Items
					// Simplified item schema representation for property items.
					ollamaProp.Items = map[string]interface{}{
						"type": string(itemSchema.Type),
					}
					// If itemSchema.Properties, etc. for nested items need mapping, add here.
				}
				paramsForOllama.Properties[name] = ollamaProp
			}
		} else if paramsForOllama.Type == "object" {
			// Ensure Properties is an empty map if type is object and no properties are defined.
			paramsForOllama.Properties = make(map[string]ToolFunctionProperty)
		}
	}

	// Marshal and unmarshal paramsForOllama to ensure it's a valid JSON structure
	paramsJSON, err := json.Marshal(paramsForOllama)
	if err != nil {
		return ollama.Tool{}, err
	}

	var paramsInterface any
	if err := json.Unmarshal(paramsJSON, &paramsInterface); err != nil {
		return ollama.Tool{}, err
	}

	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
			Name:        td.Name,
			Description: td.Description,
			Parameters:  paramsInterface,
		},
	}, nil
}

// AdaptBaseToolToOpenRouterTool converts a base.ToolDefinition to an openrouter.Tool.
// openrouter.FunctionDefinition.Parameters expects an 'any' type that should be
// a JSON schema object. jsonschema.Definition fits this requirement.
func AdaptBaseToolToOpenRouterTool(td base.ToolDefinition) (openrouter.Tool, error) {
	// The InputSchema from base.ToolDefinition is already a jsonschema.Definition,
	// which is compatible with what OpenRouter expects for parameters.
	// If td.InputSchema is a zero value (e.g. for no parameters),
	// it should represent an empty object schema: { "type": "object", "properties": {} }
	// The jsonschema.Definition zero value might not directly translate to this,
	// so ensure tools define a minimal schema if they have no params.
	// For now, we pass it directly.

	// If InputSchema is effectively empty (zero struct), provide a default OpenRouter schema for no params.
	var paramsSchema any = td.InputSchema
	if td.InputSchema.Type == "" && len(td.InputSchema.Properties) == 0 && td.InputSchema.Items == nil && len(td.InputSchema.Enum) == 0 {
		log.Debugf("Tool '%s' has an empty InputSchema, providing default empty object schema for OpenRouter.", td.Name)
		paramsSchema = jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: make(map[string]jsonschema.Definition),
		}
	}

	f := openrouter.FunctionDefinition{
		Name:        td.Name,
		Description: td.Description,
		Parameters:  paramsSchema, // jsonschema.Definition is assigned to any
	}
	return openrouter.Tool{
		Type:     openrouter.ToolTypeFunction,
		Function: &f,
	}, nil
}
