package db_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

func TestPushSubscriptions(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	sub := db.PushSubscription{Endpoint: "https://push.example/a", P256dh: "p1", Auth: "a1", Events: []string{"finished"}, UserAgent: "phone"}
	if err := store.UpsertPushSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	// Re-subscribing the same endpoint replaces keys and events in place.
	sub.P256dh, sub.Auth, sub.Events = "p2", "a2", []string{"awaiting_perm", "error"}
	if err := store.UpsertPushSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPushSubscription(ctx, db.PushSubscription{Endpoint: "https://push.example/b", P256dh: "p", Auth: "a"}); err != nil {
		t.Fatal(err)
	}

	subs, err := store.ListPushSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("got %d subscriptions, want 2", len(subs))
	}
	got, err := store.GetPushSubscription(ctx, sub.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if got.P256dh != "p2" || got.Auth != "a2" || !reflect.DeepEqual(got.Events, sub.Events) {
		t.Errorf("after upsert = %+v", got)
	}
	none, err := store.GetPushSubscription(ctx, "https://push.example/b")
	if err != nil {
		t.Fatal(err)
	}
	if len(none.Events) != 0 {
		t.Errorf("subscription with no events = %v, want none", none.Events)
	}

	if err := store.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
	if _, err := store.GetPushSubscription(ctx, sub.Endpoint); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("get after delete: err = %v, want sql.ErrNoRows", err)
	}
}

func TestVAPIDKeys_FirstWriterWins(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	if _, _, err := store.VAPIDKeys(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("empty store: err = %v, want sql.ErrNoRows", err)
	}
	if err := store.SetVAPIDKeysIfUnset(ctx, "priv1", "pub1"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetVAPIDKeysIfUnset(ctx, "priv2", "pub2"); err != nil {
		t.Fatal(err)
	}
	priv, pub, err := store.VAPIDKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if priv != "priv1" || pub != "pub1" {
		t.Errorf("keys = %q, %q; the first pair must stick", priv, pub)
	}
}
