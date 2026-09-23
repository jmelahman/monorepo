package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

type cfgEntry struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Source   string `json:"source"`
	Writable bool   `json:"writable"`
}

type cfgView struct {
	Scope   string     `json:"scope"`
	Board   string     `json:"board"`
	Entries []cfgEntry `json:"entries"`
}

func (v cfgView) find(key string) (cfgEntry, bool) {
	for _, e := range v.Entries {
		if e.Key == key {
			return e, true
		}
	}
	return cfgEntry{}, false
}

// redirectUserConfig points $KANBAN_CONFIG at a throwaway file so global-scope
// writes never touch the developer's real ~/.config/kanban/config.toml.
func redirectUserConfig(t *testing.T) {
	t.Helper()
	t.Setenv("KANBAN_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
}

func TestConfig_GlobalSetGetUnset(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global",
		"set":   map[string]any{"github.draft_column": "Draft"},
	}), 200)

	view := decodeJSON[cfgView](t, e.get("/api/config?scope=global"))
	if entry, ok := view.find("github.draft_column"); !ok || entry.Value != "Draft" || entry.Source != "global" {
		t.Fatalf("global view missing key: %+v", view)
	}

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global",
		"unset": []string{"github.draft_column"},
	}), 200)

	view = decodeJSON[cfgView](t, e.get("/api/config?scope=global"))
	if _, ok := view.find("github.draft_column"); ok {
		t.Fatalf("key still present after unset: %+v", view)
	}
}

func TestConfig_EffectiveIncludesRuntime(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global",
		"set":   map[string]any{"sync.allow_rebase": true},
	}), 200)

	view := decodeJSON[cfgView](t, e.get("/api/config"))
	if view.Scope != "effective" {
		t.Fatalf("default scope = %q", view.Scope)
	}
	if entry, ok := view.find("sync.allow_rebase"); !ok || entry.Value != true || entry.Source != "global" {
		t.Fatalf("effective view: %+v", entry)
	}
	if entry, ok := view.find("data_dir"); !ok || entry.Writable {
		t.Fatalf("runtime data_dir entry: %+v ok=%v", entry, ok)
	}
	if entry, ok := view.find("port_range_start"); !ok || entry.Writable {
		t.Fatalf("runtime port_range_start entry: %+v ok=%v", entry, ok)
	}
}

func TestConfig_LocalScope(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)
	b := e.seedBoard("Local Board")

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "local",
		"board": b.Slug,
		"set":   map[string]any{"branches.prefix": "feat"},
	}), 200)

	data, err := os.ReadFile(filepath.Join(e.repoPath, ".kanban.toml"))
	if err != nil {
		t.Fatalf("project .kanban.toml not written: %v", err)
	}
	if !strings.Contains(string(data), "feat") {
		t.Fatalf("prefix not in file: %s", data)
	}

	view := decodeJSON[cfgView](t, e.get("/api/config?scope=local&board="+b.Slug))
	if entry, ok := view.find("branches.prefix"); !ok || entry.Value != "feat" || entry.Source != "local" {
		t.Fatalf("local view: %+v", view)
	}
}

func TestConfig_Collections(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global",
		"set": map[string]any{
			"devcontainer.run_args":         []string{"--a", "--b"},
			"devcontainer.container_env.FOO": "bar",
			"task":                          []map[string]any{{"label": "web", "container_port": 3000}},
		},
	}), 200)

	view := decodeJSON[cfgView](t, e.get("/api/config?scope=global"))

	if entry, ok := view.find("devcontainer.run_args"); !ok {
		t.Fatalf("run_args missing: %+v", view)
	} else if arr, _ := entry.Value.([]any); len(arr) != 2 || arr[0] != "--a" {
		t.Fatalf("run_args value: %#v", entry.Value)
	}
	if entry, ok := view.find("devcontainer.container_env"); !ok {
		t.Fatalf("container_env missing: %+v", view)
	} else if m, _ := entry.Value.(map[string]any); m["FOO"] != "bar" {
		t.Fatalf("container_env value: %#v", entry.Value)
	}
	if entry, ok := view.find("task"); !ok {
		t.Fatalf("task missing: %+v", view)
	} else if arr, _ := entry.Value.([]any); len(arr) != 1 {
		t.Fatalf("task value: %#v", entry.Value)
	}
}

func TestConfig_Errors(t *testing.T) {
	redirectUserConfig(t)
	e := newEnv(t)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global", "set": map[string]any{"bogus.key": 1},
	}), 400)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global", "set": map[string]any{"sync.allow_rebase": "nope"},
	}), 400)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "global", "set": map[string]any{"harness.id": "definitely-not-a-harness"},
	}), 400)

	// JSON null in set is rejected (callers must use unset).
	assertStatus(t, e.patch("/api/config",
		`{"scope":"global","set":{"sync.allow_rebase":null}}`), 400)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "local", "set": map[string]any{"branches.prefix": "x"},
	}), 400)

	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "weird", "set": map[string]any{"sync.allow_rebase": true},
	}), 400)

	// A mount-path-only board has no repo to host a local .kanban.toml.
	mb := &db.Board{Name: "Mount Only", Slug: "mount-only", MountPath: "/x", BaseBranch: "main"}
	if err := e.store.CreateBoard(context.Background(), mb); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, e.patch("/api/config", map[string]any{
		"scope": "local", "board": "mount-only", "set": map[string]any{"branches.prefix": "x"},
	}), 422)
}
