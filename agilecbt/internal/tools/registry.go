// Package tools is the AI tool layer shared by the MCP endpoint and the
// in-app curator. Every tool takes JSON input described by a JSON Schema, so
// it plugs into any LLM provider unchanged. Mutating tools log an ai_actions
// row with before/after snapshots so the user can undo them.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
)

// Tool is one callable capability.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	// Mutates is true for tools that write; read-only tools are safe to call
	// freely.
	Mutates bool
	Run     func(ctx context.Context, a *app.App, in json.RawMessage) (any, error)

	required []string
}

// Registry holds the tool set bound to an App.
type Registry struct {
	app    *app.App
	tools  []Tool
	byName map[string]Tool
}

// New returns the registry with every tool registered.
func New(a *app.App) *Registry {
	r := &Registry{app: a, byName: map[string]Tool{}}
	for _, t := range append(readTools(), writeTools()...) {
		if _, dup := r.byName[t.Name]; dup {
			panic("duplicate tool " + t.Name)
		}
		r.tools = append(r.tools, t)
		r.byName[t.Name] = t
	}
	sort.SliceStable(r.tools, func(i, j int) bool { return !r.tools[i].Mutates && r.tools[j].Mutates })
	return r
}

// App returns the app the tools operate on.
func (r *Registry) App() *app.App { return r.app }

// List returns all tools, read-only ones first.
func (r *Registry) List() []Tool { return r.tools }

// ErrUnknownTool is returned by Call for an unregistered name.
var ErrUnknownTool = errors.New("unknown tool")

// Call runs a tool by name. Errors are meant to be shown to the model so it
// can correct itself; they never contain internal details beyond validation
// messages.
func (r *Registry) Call(ctx context.Context, name string, in json.RawMessage) (any, error) {
	t, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownTool, name)
	}
	if len(in) == 0 || string(in) == "null" {
		in = json.RawMessage("{}")
	}
	var args map[string]any
	if err := json.Unmarshal(in, &args); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON object: %v", err)
	}
	for _, req := range t.required {
		if v, ok := args[req]; !ok || v == nil {
			return nil, fmt.Errorf("%w: missing required argument %q", db.ErrInvalid, req)
		}
	}
	return t.Run(ctx, r.app, in)
}

// ResultText renders a tool result as the text handed back to a model.
func ResultText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// ErrorText renders a tool error for a model: validation problems verbatim,
// anything else generically.
func ErrorText(err error) string {
	switch {
	case errors.Is(err, db.ErrNotFound):
		return "error: not found"
	default:
		return "error: " + err.Error()
	}
}

// define builds a Tool whose input decodes into In.
func define[In any](name, desc string, mutates bool, props []prop, run func(context.Context, *app.App, In) (any, error)) Tool {
	schema, required := buildSchema(props)
	return Tool{
		Name:        name,
		Description: desc,
		InputSchema: schema,
		Mutates:     mutates,
		required:    required,
		Run: func(ctx context.Context, a *app.App, raw json.RawMessage) (any, error) {
			var in In
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&in); err != nil {
				return nil, fmt.Errorf("%w: arguments: %v", db.ErrInvalid, err)
			}
			return run(ctx, a, in)
		},
	}
}

// prop is one property of a tool's input object schema.
type prop struct {
	name     string
	schema   map[string]any
	required bool
}

func (p prop) req() prop { p.required = true; return p }

func str(name, desc string) prop {
	return prop{name: name, schema: map[string]any{"type": "string", "description": desc}}
}

func enum(name, desc string, values ...string) prop {
	return prop{name: name, schema: map[string]any{"type": "string", "description": desc, "enum": values}}
}

func integer(name, desc string, bounds ...int) prop {
	s := map[string]any{"type": "integer", "description": desc}
	if len(bounds) == 2 {
		s["minimum"], s["maximum"] = bounds[0], bounds[1]
	}
	return prop{name: name, schema: s}
}

func array(name, desc string, items map[string]any) prop {
	return prop{name: name, schema: map[string]any{"type": "array", "description": desc, "items": items}}
}

func buildSchema(props []prop) (json.RawMessage, []string) {
	properties := map[string]any{}
	required := []string{}
	for _, p := range props {
		properties[p.name] = p.schema
		if p.required {
			required = append(required, p.name)
		}
	}
	s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return b, required
}
