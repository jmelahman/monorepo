// Package mcp exposes the tool registry over the Model Context Protocol
// (streamable HTTP at /mcp), so Claude Code, Claude Desktop, and the
// claude-code curator backend can all drive the app.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/prompt"
	"github.com/jmelahman/agilecbt/internal/tools"
)

// Handler serves MCP. It is stateless: each request gets a fresh server, so
// the ?checkin=ID query parameter (set by the curator's claude-code backend)
// can attribute tool calls to that check-in's chat.
func Handler(reg *tools.Registry, version string) http.Handler {
	return sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		actor := app.Actor{Source: "mcp"}
		if s := r.URL.Query().Get("checkin"); s != "" {
			if id, err := strconv.ParseInt(s, 10, 64); err == nil {
				actor = app.Actor{Source: "curator", CheckinID: &id}
			}
		}
		return NewServer(reg, actor, version)
	}, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

// NewServer builds an MCP server whose tool calls act as actor.
func NewServer(reg *tools.Registry, actor app.Actor, version string) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "agilecbt", Title: "AgileCBT", Version: version}, &sdk.ServerOptions{
		Instructions: "AgileCBT is the user's personal planning + CBT app. Use the daily_checkin prompt to run a check-in. Changes you make are logged and undoable by the user. Never delete; use let_go_step.",
	})
	notDestructive := false
	for _, t := range reg.List() {
		s.AddTool(&sdk.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			Annotations: &sdk.ToolAnnotations{ReadOnlyHint: !t.Mutates, DestructiveHint: &notDestructive},
		}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			out, err := reg.Call(app.WithActor(ctx, actor), t.Name, req.Params.Arguments)
			if err != nil {
				return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: tools.ErrorText(err)}}, IsError: true}, nil
			}
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: tools.ResultText(out)}}}, nil
		})
	}
	s.AddPrompt(&sdk.Prompt{
		Name:        "daily_checkin",
		Title:       "Daily check-in",
		Description: "Run a CBT-informed daily standup with the AgileCBT curator.",
		Arguments:   []*sdk.PromptArgument{{Name: "kind", Description: "morning or evening (default: by time of day)"}},
	}, func(ctx context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
		a := reg.App()
		kind := req.Params.Arguments["kind"]
		if kind == "" {
			kind = app.DefaultKind(a.Now())
		}
		text := fmt.Sprintf("%s\n\n---\n\nPlease run my %s check-in now. Start by calling get_today and list_curator_notes, then greet me and ask how I'm arriving.",
			prompt.Curator(a.Setting(app.SettingCrisisResources)), kind)
		return &sdk.GetPromptResult{
			Description: "AgileCBT " + kind + " check-in",
			Messages:    []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: text}}},
		}, nil
	})
	return s
}
