package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/push"
	"github.com/jmelahman/kanban/internal/push/pushtest"
)

// statusOf drains the response and returns its status code.
func statusOf(t *testing.T, resp *http.Response) int {
	t.Helper()
	readBody(t, resp)
	return resp.StatusCode
}

func mustList(t *testing.T, e *testEnv) []db.PushSubscription {
	t.Helper()
	subs, err := e.store.ListPushSubscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return subs
}

func TestPushSubscriptionEndpoints(t *testing.T) {
	e := newEnv(t)

	key := decodeJSON[map[string]string](t, e.get("/api/push/vapid-key"))["public_key"]
	if key == "" {
		t.Fatal("no VAPID public key")
	}
	if again := decodeJSON[map[string]string](t, e.get("/api/push/vapid-key"))["public_key"]; again != key {
		t.Errorf("VAPID key changed between calls: %q then %q", key, again)
	}

	keys := pushtest.NewService(t).Subscription(t, "/keys")
	body := func(endpoint string, events ...string) map[string]any {
		return map[string]any{
			"endpoint": endpoint,
			"keys":     map[string]string{"p256dh": keys.P256dh, "auth": keys.Auth},
			"events":   events,
		}
	}
	badKeys := body("https://push.example/x", push.EventFinished)
	badKeys["keys"] = map[string]string{"p256dh": "p", "auth": "a"}
	for name, bad := range map[string]map[string]any{
		"http endpoint": body("http://push.example/x", push.EventFinished),
		"unknown event": body("https://push.example/x", "task_exited"),
		"missing keys":  {"endpoint": "https://push.example/x"},
		"bogus keys":    badKeys,
		"huge endpoint": body("https://push.example/"+strings.Repeat("x", 4096), push.EventFinished),
	} {
		if got := statusOf(t, e.put("/api/push/subscription", bad)); got != 400 {
			t.Errorf("%s: status = %d, want 400", name, got)
		}
	}

	const endpoint = "https://push.example/device"
	if got := statusOf(t, e.put("/api/push/subscription", body(endpoint, push.EventFinished))); got != 200 {
		t.Fatalf("subscribe: status = %d", got)
	}
	if got := statusOf(t, e.put("/api/push/subscription", body(endpoint, push.EventAwaitingPerm, push.EventError))); got != 200 {
		t.Fatalf("update events: status = %d", got)
	}
	sub, err := e.store.GetPushSubscription(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(sub.Events) != fmt.Sprint([]string{push.EventAwaitingPerm, push.EventError}) {
		t.Errorf("stored events = %v", sub.Events)
	}

	// The device count is capped, but a subscribed device can still update.
	for i := 0; len(mustList(t, e)) < 50; i++ {
		if got := statusOf(t, e.put("/api/push/subscription", body(fmt.Sprintf("https://push.example/fill/%d", i)))); got != 200 {
			t.Fatalf("fill %d: status = %d", i, got)
		}
	}
	if got := statusOf(t, e.put("/api/push/subscription", body("https://push.example/one-too-many"))); got != 409 {
		t.Errorf("subscription past the limit: status = %d, want 409", got)
	}
	if got := statusOf(t, e.put("/api/push/subscription", body(endpoint, push.EventAwaitingPerm, push.EventError))); got != 200 {
		t.Errorf("update at the limit: status = %d, want 200", got)
	}

	if got := statusOf(t, e.send("DELETE", "/api/push/subscription", map[string]string{"endpoint": endpoint})); got != 204 {
		t.Errorf("unsubscribe: status = %d, want 204", got)
	}
	if got := statusOf(t, e.post("/api/push/test", map[string]string{"endpoint": endpoint})); got != 404 {
		t.Errorf("test push to an unsubscribed device: status = %d, want 404", got)
	}
}

func TestPushTest_ReachesOnlyTheCaller(t *testing.T) {
	e := newEnv(t)
	fake := pushtest.NewService(t)
	ctx := context.Background()
	mine := fake.Subscription(t, "/mine")
	for _, sub := range []db.PushSubscription{mine, fake.Subscription(t, "/theirs", push.Events...)} {
		if err := e.store.UpsertPushSubscription(ctx, sub); err != nil {
			t.Fatal(err)
		}
	}
	if got := statusOf(t, e.post("/api/push/test", map[string]string{"endpoint": mine.Endpoint})); got != 204 {
		t.Fatalf("status = %d, want 204", got)
	}
	if hits := fake.Hits(); len(hits) != 1 || hits[0] != "/mine" {
		t.Errorf("push service saw %v, want only /mine", hits)
	}
}

// TestSessionStatusPush: a notification goes out when the agent leaves
// "working" for a state that needs the user, and at no other time.
func TestSessionStatusPush(t *testing.T) {
	e := newEnv(t)
	fake := pushtest.NewService(t)
	ctx := context.Background()
	for _, ev := range push.Events {
		if err := e.store.UpsertPushSubscription(ctx, fake.Subscription(t, "/"+ev, ev)); err != nil {
			t.Fatal(err)
		}
	}
	sess := e.seedSession(e.seedTicket(e.seedBoard("Push"), "Fix login"))
	path := fmt.Sprintf("/api/sessions/%d/status", sess.ID)
	report := func(status string) {
		t.Helper()
		if got := statusOf(t, e.patch(path, map[string]string{"status": status})); got != 204 {
			t.Fatalf("report %s: status = %d", status, got)
		}
	}
	expect := func(want string) {
		t.Helper()
		select {
		case got := <-fake.Delivered:
			if got != want {
				t.Fatalf("delivered to %s, want %s", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no delivery to %s", want)
		}
	}

	report(db.SessionStatusWorking)
	report(db.SessionStatusWorking)
	report(db.SessionStatusAwaitingPerm)
	expect("/" + push.EventAwaitingPerm)
	report(db.SessionStatusAwaitingPerm) // repeat: no second alert
	report(db.SessionStatusWorking)      // PostToolUse after the user approved
	report(db.SessionStatusAwaitingPerm) // a second prompt in the same turn
	expect("/" + push.EventAwaitingPerm)
	// No hook reports working between approving a prompt and the turn
	// ending in worktrees without PostToolUse; the finish must still notify.
	report(db.SessionStatusIdle)
	expect("/" + push.EventFinished)
	report(db.SessionStatusIdle)         // repeat
	report(db.SessionStatusAwaitingPerm) // Claude's idle reminder, not a prompt
	report(db.SessionStatusWorking)
	report(db.SessionStatusIdle)
	expect("/" + push.EventFinished)

	// Deliveries are asynchronous, so give a wrong one time to show up.
	select {
	case got := <-fake.Delivered:
		t.Fatalf("unexpected delivery to %s", got)
	case <-time.After(300 * time.Millisecond):
	}
	if hits := fake.Hits(); len(hits) != 4 {
		t.Errorf("push service saw %v, want exactly four deliveries", hits)
	}
}

// A subscription the push service has dropped answers 410, not a bare 502:
// the app renews the subscription on that status.
func TestPushTest_GoneSubscription(t *testing.T) {
	e := newEnv(t)
	fake := pushtest.NewService(t)
	gone := fake.Subscription(t, "/gone/mine")
	if err := e.store.UpsertPushSubscription(context.Background(), gone); err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"endpoint": gone.Endpoint}
	if got := statusOf(t, e.post("/api/push/test", body)); got != 410 {
		t.Errorf("first test: status = %d, want 410", got)
	}
	if got := statusOf(t, e.post("/api/push/test", body)); got != 404 {
		t.Errorf("after the prune: status = %d, want 404", got)
	}
}
