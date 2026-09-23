package github

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jmelahman/kanban/internal/db"
)

type capturedEvent struct {
	BoardID int64
	Type    string
	Data    any
}

type captureBus struct {
	events []capturedEvent
}

func (c *captureBus) Publish(boardID int64, typ string, data any) {
	c.events = append(c.events, capturedEvent{boardID, typ, data})
}

// TestApplyTransitionPublishesPRFields exercises the poller's applyTransition
// path on first PR observation: PR fields should be persisted to the DB AND
// the published session_updated event should include pr_number / pr_url /
// pr_state / pr_title when JSON-marshalled (the SSE wire format).
func TestApplyTransitionPublishesPRFields(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	board := db.Board{Name: "test", Slug: "test", RepoPath: "/tmp/fake-repo", BaseBranch: "main"}
	if err := store.CreateBoard(ctx, &board); err != nil {
		t.Fatal(err)
	}
	cols, err := store.ListColumns(ctx, board.ID)
	if err != nil {
		t.Fatal(err)
	}
	colByName := make(map[string]*db.Column, len(cols))
	for i := range cols {
		colByName[cols[i].Name] = &cols[i]
	}
	t1 := db.Ticket{BoardID: board.ID, ColumnID: colByName["In Progress"].ID, Title: "t", Slug: "t", Position: 0}
	if err := store.CreateTicket(ctx, &t1); err != nil {
		t.Fatal(err)
	}
	sess := db.Session{TicketID: t1.ID, BranchName: "kanban/test/t", Status: db.SessionStatusIdle}
	if err := store.UpsertSession(ctx, &sess); err != nil {
		t.Fatal(err)
	}

	bus := &captureBus{}
	poller := NewPoller(store, bus, nil, 30*time.Second)

	pr := ghPR{
		Number:  42,
		HTMLURL: "https://github.com/x/y/pull/42",
		State:   "open",
		Title:   "Fix things",
	}
	pr.Head.Ref = sess.BranchName

	if err := poller.applyTransition(ctx, &board, &sess, sess.PRState, "open", pr, defaultConfig(), colByName); err != nil {
		t.Fatalf("applyTransition: %v", err)
	}

	// DB should now have PR fields persisted.
	got, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PRNumber == nil || *got.PRNumber != 42 {
		t.Errorf("db PRNumber = %v, want 42", got.PRNumber)
	}
	if got.PRURL != pr.HTMLURL {
		t.Errorf("db PRURL = %q, want %q", got.PRURL, pr.HTMLURL)
	}
	if got.PRState != "open" {
		t.Errorf("db PRState = %q, want open", got.PRState)
	}

	// Bus should have a session_updated event whose serialized payload
	// contains pr_number / pr_url (this is what the SSE handler ships).
	var pub *capturedEvent
	for i := range bus.events {
		if bus.events[i].Type == "session_updated" {
			pub = &bus.events[i]
			break
		}
	}
	if pub == nil {
		t.Fatalf("no session_updated event published; got events: %+v", bus.events)
	}
	payload, err := json.Marshal(pub.Data)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["pr_number"] == nil {
		t.Errorf("session_updated JSON missing pr_number; payload=%s", payload)
	}
	if decoded["pr_url"] == nil {
		t.Errorf("session_updated JSON missing pr_url; payload=%s", payload)
	}
	if decoded["pr_state"] == nil {
		t.Errorf("session_updated JSON missing pr_state; payload=%s", payload)
	}
}
