package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeStore writes a store provider that fails its first run and buys on
// the second, counting its runs in a file beside it.
func fakeStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	script := `#!/bin/sh
count="` + dir + `/runs"
echo x >> "$count"
if [ "$(wc -l < "$count")" -lt 2 ]; then
  echo "the store is down" >&2
  exit 1
fi
echo '{"artist":"A","album":"B","title":"C","file":"/m/A/B/C.mp3"}'
`
	if err := os.WriteFile(store, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return store
}

func waitFor(t *testing.T, q *buyQueue, status string) buy {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if l := q.list(); len(l) > 0 && l[0].Status == status {
			return l[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no buy reached %s: %+v", status, q.list())
	return buy{}
}

func TestBuyQueueRetries(t *testing.T) {
	saved := buyBackoff
	buyBackoff = []time.Duration{10 * time.Millisecond}
	t.Cleanup(func() { buyBackoff = saved })

	state := t.TempDir()
	q, err := newBuyQueue(fakeStore(t), t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	first, err := q.add("A", "C")
	if err != nil {
		t.Fatal(err)
	}
	// The same track again is the same buy.
	if again, _ := q.add("a", "c"); again.ID != first.ID {
		t.Fatalf("a second buy for the same track: %s, %s", first.ID, again.ID)
	}

	ctx := t.Context()
	go q.run(ctx)
	b := waitFor(t, q, buyDone)
	if b.Attempts != 2 || b.Result == nil || b.Result.Album != "B" {
		t.Fatalf("done after the retry with the result: %+v", b)
	}

	// The queue outlives the server.
	again, err := newBuyQueue("unused", "", state)
	if err != nil {
		t.Fatal(err)
	}
	if l := again.list(); len(l) != 1 || l[0].Status != buyDone {
		t.Fatalf("reloaded: %+v", l)
	}
}

func TestBuyQueueFailsAfterRetries(t *testing.T) {
	saved := buyBackoff
	buyBackoff = nil
	t.Cleanup(func() { buyBackoff = saved })

	q, err := newBuyQueue(fakeStore(t), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.add("A", "C"); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	go q.run(ctx)
	if b := waitFor(t, q, buyFailed); b.Error != "the store is down" {
		t.Fatalf("the provider's own words: %q", b.Error)
	}
}

func TestBuyQueueResumesInterrupted(t *testing.T) {
	state := t.TempDir()
	q, err := newBuyQueue("unused", "", state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.add("A", "C"); err != nil {
		t.Fatal(err)
	}
	if b, _ := q.next(); b == nil {
		t.Fatal("nothing due")
	}
	// The server stops here, mid-buy.
	again, err := newBuyQueue("unused", "", state)
	if err != nil {
		t.Fatal(err)
	}
	if l := again.list(); l[0].Status != buyQueued {
		t.Fatalf("an interrupted buy runs again: %+v", l[0])
	}
}

func TestBuyGoesThroughTheProxy(t *testing.T) {
	store := filepath.Join(t.TempDir(), "store")
	script := `#!/bin/sh
echo "{\"artist\":\"A\",\"album\":\"B\",\"title\":\"$HTTPS_PROXY\",\"file\":\"/m/A/B/C.mp3\"}"
`
	if err := os.WriteFile(store, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	q, err := newBuyQueue(store, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	q.proxy = "http://gluetun:8888"
	res, err := q.attempt(t.Context(), "A", "C")
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != q.proxy {
		t.Fatalf("the provider saw HTTPS_PROXY=%q", res.Title)
	}
}
