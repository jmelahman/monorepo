package server

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveConfigScope(t *testing.T) {
	cases := []struct {
		local, global, write bool
		want                 string
		wantErr              bool
	}{
		{false, false, false, "effective", false},
		{true, false, false, "local", false},
		{false, true, false, "global", false},
		{true, true, false, "", true},  // mutually exclusive
		{false, false, true, "", true}, // writes need an explicit scope
		{false, true, true, "global", false},
	}
	for _, c := range cases {
		got, err := resolveConfigScope(c.local, c.global, c.write)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveConfigScope(%v,%v,%v): want error", c.local, c.global, c.write)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("resolveConfigScope(%v,%v,%v) = %q, %v; want %q", c.local, c.global, c.write, got, err, c.want)
		}
	}
}

func TestRunConfig(t *testing.T) {
	t.Setenv("KANBAN_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	srv, _, board := newKanbanCLITestServer(t)
	ctx := t.Context()
	var out bytes.Buffer

	if err := runConfigSet(ctx, srv.URL, &out, "sync.allow_rebase", "true", "global", ""); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := runConfigGet(ctx, srv.URL, &out, "sync.allow_rebase", "effective", "", false); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "true" {
		t.Errorf("get = %q, want true", out.String())
	}

	out.Reset()
	if err := runConfigList(ctx, srv.URL, &out, "effective", "", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "KEY") || !strings.Contains(out.String(), "sync.allow_rebase") {
		t.Errorf("list output: %q", out.String())
	}

	// Local scope writes to the seeded board's project file.
	out.Reset()
	if err := runConfigSet(ctx, srv.URL, &out, "branches.prefix", "feat", "local", board.Slug); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := runConfigUnset(ctx, srv.URL, &out, "sync.allow_rebase", "global", ""); err != nil {
		t.Fatal(err)
	}

	// A non-boolean value for a bool key fails client-side during coercion.
	if err := runConfigSet(ctx, srv.URL, io.Discard, "sync.allow_rebase", "notabool", "global", ""); err == nil {
		t.Error("expected coercion error")
	}
	// Unknown keys are rejected before any request.
	if err := runConfigUnset(ctx, srv.URL, io.Discard, "bogus.key", "global", ""); err == nil {
		t.Error("expected unknown-key error")
	}
}

func TestRunConfigGetJSON(t *testing.T) {
	t.Setenv("KANBAN_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	srv, _, _ := newKanbanCLITestServer(t)
	ctx := t.Context()
	var out bytes.Buffer

	if err := runConfigSet(ctx, srv.URL, &out, "github.draft_column", "Draft", "global", ""); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runConfigGet(ctx, srv.URL, &out, "github.draft_column", "global", "", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"value":"Draft"`) {
		t.Errorf("json get output: %q", out.String())
	}
}
