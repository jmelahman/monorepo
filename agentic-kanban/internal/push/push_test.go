package push_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/push"
	"github.com/jmelahman/kanban/internal/push/pushtest"
)

func newStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPublicKeyIsStable(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	first, err := push.New(store).PublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// A second service over the same store stands in for a server restart:
	// a new key would orphan every existing subscription.
	second, err := push.New(store).PublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Errorf("public key changed: %q then %q", first, second)
	}
}

func TestSend(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	fake := pushtest.NewService(t)
	for _, sub := range []db.PushSubscription{
		fake.Subscription(t, "/wants-finished", push.EventFinished, push.EventError),
		fake.Subscription(t, "/wants-perm", push.EventAwaitingPerm),
		fake.Subscription(t, "/gone/expired", push.EventFinished),
	} {
		if err := store.UpsertPushSubscription(ctx, sub); err != nil {
			t.Fatal(err)
		}
	}

	sent, err := fake.Sender(store).Send(ctx, push.Payload{Event: push.EventFinished, Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Errorf("sent = %d, want 1", sent)
	}
	hits := fake.Hits()
	sort.Strings(hits)
	if want := []string{"/gone/expired", "/wants-finished"}; !reflect.DeepEqual(hits, want) {
		t.Errorf("push service saw %v, want %v (only devices opted into the event)", hits, want)
	}
	if _, err := store.GetPushSubscription(ctx, fake.URL+"/gone/expired"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("410 subscription still stored: err = %v", err)
	}
	if _, err := store.GetPushSubscription(ctx, fake.URL+"/wants-finished"); err != nil {
		t.Errorf("accepted subscription was dropped: %v", err)
	}
}

func TestSendTo(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	fake := pushtest.NewService(t)
	// No opted-in events: a test notification must still reach the device.
	sub := fake.Subscription(t, "/device")
	if err := store.UpsertPushSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	other := fake.Subscription(t, "/other", push.Events...)
	if err := store.UpsertPushSubscription(ctx, other); err != nil {
		t.Fatal(err)
	}
	svc := fake.Sender(store)
	if err := svc.SendTo(ctx, sub.Endpoint, push.Payload{Event: "test"}); err != nil {
		t.Fatal(err)
	}
	if hits := fake.Hits(); !reflect.DeepEqual(hits, []string{"/device"}) {
		t.Errorf("push service saw %v, want only /device", hits)
	}
	if err := svc.SendTo(ctx, fake.URL+"/unknown", push.Payload{Event: "test"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unknown endpoint: err = %v, want sql.ErrNoRows", err)
	}
}

// Endpoints are supplied by API callers, so the default client must refuse
// to be pointed at the server's own network.
func TestDefaultClientRefusesNonPublicEndpoints(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	fake := pushtest.NewService(t)
	if err := store.UpsertPushSubscription(ctx, fake.Subscription(t, "/internal", push.EventFinished)); err != nil {
		t.Fatal(err)
	}
	sent, err := push.New(store).Send(ctx, push.Payload{Event: push.EventFinished, Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 || len(fake.Hits()) != 0 {
		t.Fatalf("sent = %d, hits = %v; a loopback endpoint must not be contacted", sent, fake.Hits())
	}
}

// One stalled push service must not hold up the other devices.
func TestSendDoesNotWaitOnASlowEndpoint(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	fake := pushtest.NewService(t)
	for _, path := range []string{"/slow/a", "/slow/b", "/fast"} {
		if err := store.UpsertPushSubscription(ctx, fake.Subscription(t, path, push.EventFinished)); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan int, 1)
	go func() {
		sent, _ := fake.Sender(store).Send(ctx, push.Payload{Event: push.EventFinished, Title: "t"})
		done <- sent
	}()
	select {
	case got := <-fake.Delivered:
		if got != "/fast" {
			t.Fatalf("first delivery was %s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the fast device was not reached while others were stalled")
	}
	fake.Release()
	if sent := <-done; sent != 3 {
		t.Errorf("sent = %d, want 3", sent)
	}
}

// A ticket title can be pasted paragraphs; Web Push carries about 4 KB.
func TestNotifySessionTruncatesLongTitles(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	fake := pushtest.NewService(t)
	if err := store.UpsertPushSubscription(ctx, fake.Subscription(t, "/device", push.EventFinished)); err != nil {
		t.Fatal(err)
	}
	board := &db.Board{Name: strings.Repeat("b", 2000), Slug: "long"}
	if err := store.CreateBoard(ctx, board); err != nil {
		t.Fatal(err)
	}
	cols, err := store.ListColumns(ctx, board.ID)
	if err != nil || len(cols) == 0 {
		t.Fatalf("columns: %v", err)
	}
	ticket := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: strings.Repeat("é", 6000), Slug: "long"}
	if err := store.CreateTicket(ctx, ticket); err != nil {
		t.Fatal(err)
	}
	sess := &db.Session{TicketID: ticket.ID, WorktreePath: t.TempDir(), Status: db.SessionStatusIdle}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	fake.Sender(store).NotifySession(sess.ID, push.EventFinished)
	select {
	case <-fake.Delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("a session with a very long ticket title was never notified")
	}
}
