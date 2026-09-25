package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

// runLeaf executes a parent command with an inherited --server flag and
// returns what resolveURL reports inside the leaf's RunE — the same shape as
// the real CLI subcommands.
func runLeaf(t *testing.T, args ...string) string {
	t.Helper()
	var serverURL string
	var got string
	parent := &cobra.Command{Use: "root"}
	addServerFlag(parent, &serverURL)
	leaf := &cobra.Command{
		Use: "leaf",
		RunE: func(cmd *cobra.Command, _ []string) error {
			got = resolveURL(cmd, serverURL)
			return nil
		},
	}
	parent.AddCommand(leaf)
	parent.SetArgs(append([]string{"leaf"}, args...))
	if err := parent.Execute(); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestResolveURL(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		if got := runLeaf(t); got != "http://localhost:8080" {
			t.Errorf("resolveURL = %q, want default", got)
		}
	})

	t.Run("env wins over default", func(t *testing.T) {
		t.Setenv("APP_URL", "http://env:1234")
		if got := runLeaf(t); got != "http://env:1234" {
			t.Errorf("resolveURL = %q, want env value", got)
		}
	})

	t.Run("explicit flag wins over env", func(t *testing.T) {
		t.Setenv("APP_URL", "http://env:1234")
		if got := runLeaf(t, "--server", "http://flag:5678"); got != "http://flag:5678" {
			t.Errorf("resolveURL = %q, want flag value", got)
		}
	})
}

// The chat event stream must be able to flush through the logging
// middleware's wrapper.
func TestStatusRecorderFlushes(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := http.NewResponseController(&statusRecorder{ResponseWriter: rec}).Flush(); err != nil {
		t.Fatal(err)
	}
	if !rec.Flushed {
		t.Fatal("not flushed")
	}
}
