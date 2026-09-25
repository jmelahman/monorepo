package mcp_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/mcp"
	"github.com/jmelahman/agilecbt/internal/tools"
)

func TestMCPRoundTrip(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := app.New(store)
	reg := tools.New(a)
	srv := httptest.NewServer(mcp.Handler(reg, "test"))
	defer srv.Close()

	today := a.Today()
	c, err := store.CreateCheckin(db.CheckinPatch{Date: &today})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: fmt.Sprintf("%s?checkin=%d", srv.URL, c.ID)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	list, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != len(reg.List()) {
		t.Fatalf("got %d tools, want %d", len(list.Tools), len(reg.List()))
	}

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: "create_step", Arguments: map[string]any{"title": "Water the plants", "lane": "today"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content[0])
	}
	actions, err := store.ListActions(&c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Source != "curator" {
		t.Fatalf("?checkin should attribute the action to the curator chat: %+v", actions)
	}

	// Validation errors come back as tool errors the model can read.
	res, err = sess.CallTool(ctx, &sdk.CallToolParams{Name: "create_step", Arguments: map[string]any{"title": "x", "energy_cost": 7}})
	if err == nil && !res.IsError {
		t.Fatal("expected a tool error for energy_cost 7")
	}

	p, err := sess.GetPrompt(ctx, &sdk.GetPromptParams{Name: "daily_checkin", Arguments: map[string]string{"kind": "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	text := p.Messages[0].Content.(*sdk.TextContent).Text
	if !strings.Contains(text, "evening check-in") || !strings.Contains(text, "988") {
		t.Fatalf("prompt: %.200s", text)
	}
}
