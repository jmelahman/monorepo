package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/tools"
)

func newReg(t *testing.T) *tools.Registry {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return tools.New(app.New(store))
}

// call runs a tool and round-trips its result through JSON into out.
func call(t *testing.T, reg *tools.Registry, ctx context.Context, name, in string, out any) {
	t.Helper()
	v, err := reg.Call(ctx, name, json.RawMessage(in))
	if err != nil {
		t.Fatalf("%s(%s): %v", name, in, err)
	}
	if out != nil {
		b, _ := json.Marshal(v)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
	}
}

func TestSchemasAreValidJSON(t *testing.T) {
	reg := newReg(t)
	seenWrite := false
	for _, tool := range reg.List() {
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("%s: %v", tool.Name, err)
		}
		if schema["type"] != "object" {
			t.Errorf("%s: schema type = %v", tool.Name, schema["type"])
		}
		if tool.Description == "" {
			t.Errorf("%s: no description", tool.Name)
		}
		// Read-only tools are listed first.
		if tool.Mutates {
			seenWrite = true
		} else if seenWrite {
			t.Errorf("%s: read-only tool listed after a write tool", tool.Name)
		}
	}
}

func TestCallValidation(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()
	if _, err := reg.Call(ctx, "nope", nil); !errors.Is(err, tools.ErrUnknownTool) {
		t.Errorf("unknown tool: %v", err)
	}
	if _, err := reg.Call(ctx, "create_step", json.RawMessage(`{}`)); !errors.Is(err, db.ErrInvalid) {
		t.Errorf("missing required: %v", err)
	}
	if _, err := reg.Call(ctx, "create_step", json.RawMessage(`{"title":"x","lane":"done"}`)); !errors.Is(err, db.ErrInvalid) {
		t.Errorf("create in done: %v", err)
	}
	if _, err := reg.Call(ctx, "create_step", json.RawMessage(`{"title":"x","surprise":true}`)); err == nil {
		t.Error("unknown field accepted")
	}
	// Empty input is fine for tools without required args.
	if _, err := reg.Call(ctx, "get_today", nil); err != nil {
		t.Errorf("get_today(nil): %v", err)
	}
}

func TestWritesAreRecordedAndUndoable(t *testing.T) {
	reg := newReg(t)
	a := reg.App()
	var checkin db.Checkin
	call(t, reg, context.Background(), "record_checkin", `{"mood":3,"energy":2}`, &checkin)
	ctx := app.WithActor(context.Background(), app.Actor{Source: "curator", CheckinID: &checkin.ID})

	var v db.Value
	call(t, reg, ctx, "create_value", `{"name":"Health"}`, &v)
	var g db.Goal
	call(t, reg, ctx, "create_goal", `{"title":"Sleep better","value_id":1}`, &g)
	var st db.Step
	call(t, reg, ctx, "create_step", `{"title":"Phone out of bedroom","goal_id":1,"lane":"week"}`, &st)
	call(t, reg, ctx, "move_step", `{"id":1,"lane":"today"}`, &st)
	if st.Lane != db.LaneToday {
		t.Fatalf("move: %+v", st)
	}
	call(t, reg, ctx, "let_go_step", `{"id":1}`, &st)
	if st.Lane != db.LaneLetGo {
		t.Fatalf("let go: %+v", st)
	}

	actions, err := a.Store.ListActions(&checkin.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 5 {
		t.Fatalf("got %d actions, want 5", len(actions))
	}
	for _, act := range actions {
		if act.Source != "curator" || act.Summary == "" {
			t.Errorf("action: %+v", act)
		}
	}

	// Undo newest first: let go → move → create.
	for _, act := range actions[:3] {
		if _, err := a.Undo(act.ID); err != nil {
			t.Fatalf("undo %s: %v", act.Tool, err)
		}
	}
	if _, err := a.Store.GetStep(1); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("step should be gone after undoing create: %v", err)
	}
}

func TestUndoRestoresPreviousState(t *testing.T) {
	reg := newReg(t)
	a := reg.App()
	ctx := context.Background()
	call(t, reg, ctx, "create_step", `{"title":"Call mom","lane":"week","energy_cost":2}`, nil)
	call(t, reg, ctx, "update_step", `{"id":1,"title":"Text mom","energy_cost":1}`, nil)
	actions, err := a.Store.ListActions(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Undo(actions[0].ID); err != nil {
		t.Fatal(err)
	}
	st, err := a.Store.GetStep(1)
	if err != nil {
		t.Fatal(err)
	}
	if st.Title != "Call mom" || st.EnergyCost != 2 {
		t.Fatalf("undo update: %+v", st)
	}
}

func TestRecordCheckinUsesActorCheckin(t *testing.T) {
	reg := newReg(t)
	a := reg.App()
	c, err := a.Store.CreateCheckin(db.CheckinPatch{Date: ptr(a.Today())})
	if err != nil {
		t.Fatal(err)
	}
	ctx := app.WithActor(context.Background(), app.Actor{Source: "curator", CheckinID: &c.ID})
	var got db.Checkin
	call(t, reg, ctx, "record_checkin", `{"anxiety":7,"note":"big meeting"}`, &got)
	if got.ID != c.ID || got.Anxiety == nil || *got.Anxiety != 7 {
		t.Fatalf("record_checkin: %+v", got)
	}
}

func TestSaveRetroDraftKeepsUserText(t *testing.T) {
	reg := newReg(t)
	a := reg.App()
	w, err := a.CurrentWeek()
	if err != nil {
		t.Fatal(err)
	}
	mine := "I walked twice"
	if _, err := a.Store.UpsertRetro(w.ID, db.RetroPatch{WentWell: &mine}); err != nil {
		t.Fatal(err)
	}
	var r db.Retro
	call(t, reg, context.Background(), "save_retro_draft", `{"went_well":"AI text","draft":"draft"}`, &r)
	if r.WentWell != mine || r.AIDraft != "draft" {
		t.Fatalf("retro: %+v", r)
	}
}

func TestRememberForget(t *testing.T) {
	reg := newReg(t)
	ctx := context.Background()
	var n db.Note
	call(t, reg, ctx, "remember", `{"text":"Prefers mornings for hard tasks"}`, &n)
	var notes []db.Note
	call(t, reg, ctx, "list_curator_notes", `{}`, &notes)
	if len(notes) != 1 {
		t.Fatalf("notes: %+v", notes)
	}
	call(t, reg, ctx, "forget", `{"id":1}`, nil)
	call(t, reg, ctx, "list_curator_notes", `{}`, &notes)
	if len(notes) != 0 {
		t.Fatalf("after forget: %+v", notes)
	}
}

func TestResultText(t *testing.T) {
	if got := tools.ResultText(map[string]int{"a": 1}); !strings.Contains(got, `"a": 1`) && !strings.Contains(got, `"a":1`) {
		t.Errorf("ResultText = %q", got)
	}
}

func ptr[T any](v T) *T { return &v }
