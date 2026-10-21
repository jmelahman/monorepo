package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchStaysOffThisComputer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("# Notes\n\nhello"))
	}))
	defer srv.Close()

	_, err := (&Client{}).Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "won't fetch") {
		t.Fatalf("fetching %s: err = %v, want a refusal", srv.URL, err)
	}

	src, err := (&Client{HTTP: srv.Client()}).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if src.Title != "Notes" {
		t.Errorf("title = %q, want Notes", src.Title)
	}
}
