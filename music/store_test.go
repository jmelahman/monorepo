package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
buy) shift; echo "{\"artist\":\"A\",\"album\":\"B\",\"title\":\"$*\",\"file\":\"/m/A/B/C.mp3\"}" ;;
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
