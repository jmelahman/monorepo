package llm_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/jmelahman/local-ai-detection/llm"
)

const model = "pretend GGUF bytes"

// hub serves body for every request and counts them.
func hub(t *testing.T, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/acme/tiny-GGUF/resolve/0123abc/tiny.gguf" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func spec(endpoint string) llm.Spec {
	sum := sha256.Sum256([]byte(model))
	return llm.Spec{
		Endpoint: endpoint,
		Repo:     "acme/tiny-GGUF",
		Revision: "0123abc",
		File:     "tiny.gguf",
		SHA256:   hex.EncodeToString(sum[:]),
		Size:     int64(len(model)),
	}
}

func TestFetch_DownloadsOnceThenUsesTheCache(t *testing.T) {
	t.Parallel()
	srv, requests := hub(t, model)
	cache := t.TempDir()

	for range 2 {
		path, err := llm.Fetch(t.Context(), spec(srv.URL), cache, nil)
		if err != nil {
			t.Fatal(err)
		}
		if b, err := os.ReadFile(path); err != nil || string(b) != model {
			t.Fatalf("ReadFile(%s) = %q, %v; want %q", path, b, err, model)
		}
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("two fetches made %d requests, want 1: the second should hit the cache", n)
	}
}

func TestFetch_DigestMismatchLeavesNothingInTheCache(t *testing.T) {
	t.Parallel()
	srv, _ := hub(t, "tampered GGUF bytes") // the pinned size, other bytes
	cache := t.TempDir()
	s := spec(srv.URL)

	_, err := llm.Fetch(t.Context(), s, cache, nil)
	if !errors.Is(err, llm.ErrDigestMismatch) {
		t.Fatalf("Fetch = %v, want ErrDigestMismatch", err)
	}
	entries, err := os.ReadDir(filepath.Dir(s.Path(cache)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("cache holds %v after a failed fetch, want nothing", entries)
	}
}

func TestFetch_OversizedBodyIsRejected(t *testing.T) {
	t.Parallel()
	srv, _ := hub(t, model+" and then some more")

	_, err := llm.Fetch(t.Context(), spec(srv.URL), t.TempDir(), nil)
	if !errors.Is(err, llm.ErrDigestMismatch) {
		t.Errorf("Fetch = %v, want ErrDigestMismatch", err)
	}
}

func TestFetch_TruncatedCacheEntryIsRefetched(t *testing.T) {
	t.Parallel()
	srv, requests := hub(t, model)
	cache := t.TempDir()
	s := spec(srv.URL)
	if err := os.MkdirAll(filepath.Dir(s.Path(cache)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path(cache), []byte("pretend"), 0o600); err != nil {
		t.Fatal(err)
	}

	path, err := llm.Fetch(t.Context(), s, cache, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != model {
		t.Errorf("cache holds %q, want the refetched %q", b, model)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("made %d requests, want 1", n)
	}
}

func TestFetch_HTTPErrorIsReported(t *testing.T) {
	t.Parallel()
	srv, _ := hub(t, model)
	s := spec(srv.URL)
	s.File = "missing.gguf"

	if _, err := llm.Fetch(t.Context(), s, t.TempDir(), nil); err == nil {
		t.Error("Fetch of a 404 = nil error, want one")
	}
}
