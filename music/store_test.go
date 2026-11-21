package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A fake provider: it answers a search and an album's page, and "buys" by
// echoing the flags it was given, so nothing is ever bought.
func fakeCatalogue(t *testing.T) (*buyQueue, *http.ServeMux) {
	t.Helper()
	store := filepath.Join(t.TempDir(), "store")
	script := `#!/bin/sh
case "$1" in
search) [ "$2" = "--" ] || exit 2; echo "[{\"artist\":\"A\",\"album\":\"$3\",\"url\":\"/download/A/B\"}]" ;;
album) [ "$2" = "--" ] || exit 2; echo "{\"artist\":\"$3\",\"name\":\"$4\",\"tracks\":[]}" ;;
buy) [ -n "$FAKE_SLOW" ] && [ "$2" != "--dry-run" ] && sleep 30; shift; echo "{\"artist\":\"A\",\"album\":\"B\",\"title\":\"$*\",\"file\":\"/m/A/B/C.mp3\"}" ;;
*) exit 2 ;;
esac
`
	if err := os.WriteFile(store, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	q, err := newBuyQueue(store, "/m", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	routeStore(mux, q)
	return q, mux
}

func TestStoreLooks(t *testing.T) {
	_, mux := fakeCatalogue(t)
	for path, want := range map[string]string{
		"/api/store/search?q=-B":             `"album":"-B"`,
		"/api/store/album?artist=A&album=-B": `"name":"-B"`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}

// Picked tracks are queued one buy each, bought from the album's page by
// their number there.
func TestBuyAlbum(t *testing.T) {
	q, mux := fakeCatalogue(t)
	for body, code := range map[string]int{
		`{"artist":"A","album":"B","page":"javascript:x","tracks":[{"number":1,"title":"C"}]}`:  http.StatusBadRequest,
		`{"artist":"A","album":"B","page":"/download/A/B","tracks":[]}`:                         http.StatusBadRequest,
		`{"artist":"A","album":"B","page":"/download/A/B","tracks":[{"number":0,"title":"C"}]}`: http.StatusBadRequest,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/buy/album", strings.NewReader(body)))
		if w.Code != code {
			t.Errorf("%s: %d, want %d", body, w.Code, code)
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/buy/album", strings.NewReader(
		`{"artist":"A","album":"B","page":"/download/A/B","tracks":[{"number":1,"title":"C"},{"number":2,"title":"D"}]}`)))
	var bs []buy
	if err := json.NewDecoder(w.Body).Decode(&bs); w.Code != http.StatusAccepted || err != nil || len(bs) != 2 {
		t.Fatalf("%d %v %+v", w.Code, err, bs)
	}
	b, _ := q.next()
	if b == nil || b.Title != "C" || b.Link != "/download/A/B" {
		t.Fatalf("first due: %+v", b)
	}
	res, err := q.attempt(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--artist=A --title=C --dir=/m --link=/download/A/B --number=1"; res.Title != want {
		t.Errorf("the provider was given %q, want %q", res.Title, want)
	}
}

// A preview asks the provider for a dry run; a buy picked from one names
// the store's track exactly.
func TestBuyPreviewed(t *testing.T) {
	// The queue's own run of the buy hangs until the test ends, so it
	// never lands.
	t.Setenv("FAKE_SLOW", "1")
	fake, _ := fakeCatalogue(t)
	mux := http.NewServeMux()
	// The queue saves itself as the buy starts, racing the test's end:
	// it is stopped, and given a moment, before its state is removed.
	// Cleanups run last first.
	state := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	})
	routeBuys(ctx, mux, fake.store, "", "/m", state)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/buy/preview?artist=A&title=-C", nil))
	if want := "--dry-run --artist=A --title=-C --dir=/m"; !strings.Contains(w.Body.String(), want) {
		t.Errorf("preview: %d %s, want the provider given %q", w.Code, w.Body, want)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/buy", strings.NewReader(`{"artist":"A","title":"C","page":"/download/A/B"}`)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("a page without a number: %d", w.Code)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/buy", strings.NewReader(`{"artist":"A","title":"C","album":"B","page":"/download/A/B","number":4}`)))
	var b buy
	if err := json.NewDecoder(w.Body).Decode(&b); w.Code != http.StatusAccepted || err != nil || b.Page != "/download/A/B" || b.Number != 4 || b.Album != "B" {
		t.Errorf("a previewed buy: %d %v %+v", w.Code, err, b)
	}
}
