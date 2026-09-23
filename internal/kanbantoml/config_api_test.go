package kanbantoml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestEditFileRoundTripPreservesCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := EditFile(path, map[string]any{
		"task":                  []TaskEntry{{Label: "web", ContainerPort: 3000}},
		"devcontainer.run_args": []string{"--cap-add=NET_ADMIN"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	// A second, unrelated edit must leave the collections intact even though
	// go-toml re-reads them as untyped maps/slices.
	if err := EditFile(path, map[string]any{"harness.id": "claude"}, nil); err != nil {
		t.Fatal(err)
	}

	f := readFileAt(path)
	if f.Harness == nil || f.Harness.ID == nil || *f.Harness.ID != "claude" {
		t.Fatalf("harness.id not set: %+v", f.Harness)
	}
	if len(f.Tasks) != 1 || f.Tasks[0].Label != "web" || f.Tasks[0].ContainerPort != 3000 {
		t.Fatalf("task block lost: %+v", f.Tasks)
	}
	if f.Devcontainer == nil || len(f.Devcontainer.RunArgs) != 1 || f.Devcontainer.RunArgs[0] != "--cap-add=NET_ADMIN" {
		t.Fatalf("run_args lost: %+v", f.Devcontainer)
	}
}

func TestEditFileUnsetPrunesEmptySection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := EditFile(path, map[string]any{
		"sync.allow_rebase":   true,
		"github.draft_column": "Draft",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := EditFile(path, nil, []string{"sync.allow_rebase"}); err != nil {
		t.Fatal(err)
	}
	f := readFileAt(path)
	if f.Sync != nil {
		t.Errorf("empty [sync] section not pruned: %+v", f.Sync)
	}
	if f.GitHub == nil || f.GitHub.DraftColumn == nil || *f.GitHub.DraftColumn != "Draft" {
		t.Fatalf("unrelated key lost: %+v", f.GitHub)
	}
	// Removing the last key should delete the file entirely.
	if err := EditFile(path, nil, []string{"github.draft_column"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("empty file should be removed, stat err = %v", err)
	}
}

func TestEditFileConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	keys := []string{
		"github.draft_column", "github.review_column", "github.done_column",
		"github.closed_column", "branches.prefix", "plans.dir",
		"errors.board_name", "harness.id",
	}
	var wg sync.WaitGroup
	for _, k := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			if err := EditFile(path, map[string]any{k: "v-" + k}, nil); err != nil {
				t.Errorf("EditFile %s: %v", k, err)
			}
		}(k)
	}
	wg.Wait()

	f := readFileAt(path)
	for _, k := range keys {
		if _, ok := GetValue(f, k); !ok {
			t.Errorf("key %q lost after concurrent writes; file = %+v", k, f)
		}
	}
}

func TestEditFileEmptyPath(t *testing.T) {
	if err := EditFile("", map[string]any{"harness.id": "claude"}, nil); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestNormalizeValue(t *testing.T) {
	t.Run("bool", func(t *testing.T) {
		v, err := NormalizeValue("sync.allow_rebase", json.RawMessage("true"))
		if err != nil || v != true {
			t.Fatalf("got %v, %v", v, err)
		}
	})
	t.Run("type mismatch", func(t *testing.T) {
		if _, err := NormalizeValue("sync.allow_rebase", json.RawMessage(`"nope"`)); err == nil {
			t.Fatal("expected type error")
		}
	})
	t.Run("null rejected", func(t *testing.T) {
		if _, err := NormalizeValue("sync.allow_rebase", json.RawMessage("null")); err == nil {
			t.Fatal("expected null error")
		}
	})
	t.Run("unknown key", func(t *testing.T) {
		if _, err := NormalizeValue("bogus.key", json.RawMessage("1")); err == nil {
			t.Fatal("expected unknown-key error")
		}
	})
	t.Run("merge default strategy", func(t *testing.T) {
		v, err := NormalizeValue("merge.default_strategy", json.RawMessage(`"squash"`))
		if err != nil || v != "squash" {
			t.Fatalf("got %v, %v", v, err)
		}
	})
	t.Run("merge default strategy rejects unknown", func(t *testing.T) {
		// "merge" is a sync strategy — the merge spelling is "merge-commit",
		// and a typo here would only surface at merge time.
		if _, err := NormalizeValue("merge.default_strategy", json.RawMessage(`"merge"`)); err == nil {
			t.Fatal("expected strategy validation error")
		}
	})
	t.Run("interval invalid", func(t *testing.T) {
		if _, err := NormalizeValue("buildcop.interval", json.RawMessage(`"3x"`)); err == nil {
			t.Fatal("expected duration parse error")
		}
	})
	t.Run("interval too short", func(t *testing.T) {
		if _, err := NormalizeValue("buildcop.interval", json.RawMessage(`"500ms"`)); err == nil {
			t.Fatal("expected min-interval error")
		}
	})
	t.Run("task missing label", func(t *testing.T) {
		if _, err := NormalizeValue("task", json.RawMessage(`[{"container_port":3000}]`)); err == nil {
			t.Fatal("expected missing-label error")
		}
	})
	t.Run("task ok", func(t *testing.T) {
		v, err := NormalizeValue("task", json.RawMessage(`[{"label":"web","container_port":3000}]`))
		ts, _ := v.([]TaskEntry)
		if err != nil || len(ts) != 1 || ts[0].Label != "web" || ts[0].ContainerPort != 3000 {
			t.Fatalf("got %#v, %v", v, err)
		}
	})
	t.Run("buildcop board missing repo", func(t *testing.T) {
		if _, err := NormalizeValue("buildcop.boards", json.RawMessage(`[{"branch":"main"}]`)); err == nil {
			t.Fatal("expected missing-repo_path error")
		}
	})
	t.Run("map entry", func(t *testing.T) {
		v, err := NormalizeValue("devcontainer.container_env.FOO", json.RawMessage(`"bar"`))
		if err != nil || v != "bar" {
			t.Fatalf("got %v, %v", v, err)
		}
	})
	t.Run("map entry non-string", func(t *testing.T) {
		if _, err := NormalizeValue("devcontainer.container_env.FOO", json.RawMessage("123")); err == nil {
			t.Fatal("expected string error for map entry")
		}
	})
}

func TestCoerceCLIValue(t *testing.T) {
	if v, err := CoerceCLIValue("sync.allow_rebase", "true"); err != nil || v != true {
		t.Errorf("bool: %v, %v", v, err)
	}
	if _, err := CoerceCLIValue("sync.allow_rebase", "nope"); err == nil {
		t.Error("expected bool parse error")
	}
	if v, err := CoerceCLIValue("github.draft_column", "Draft"); err != nil || v != "Draft" {
		t.Errorf("string: %v, %v", v, err)
	}
	if v, err := CoerceCLIValue("devcontainer.run_args", `["--a","--b"]`); err != nil {
		t.Errorf("array: %v, %v", v, err)
	}
	if _, err := CoerceCLIValue("devcontainer.run_args", "not json"); err == nil {
		t.Error("expected JSON parse error for array")
	}
	if v, err := CoerceCLIValue("devcontainer.container_env.FOO", "bar"); err != nil || v != "bar" {
		t.Errorf("map entry: %v, %v", v, err)
	}
	if _, err := CoerceCLIValue("bogus.key", "x"); err == nil {
		t.Error("expected unknown-key error")
	}
}

func TestLookup(t *testing.T) {
	if _, sub, ok := Lookup("sync.allow_rebase"); !ok || sub != "" {
		t.Errorf("scalar: sub=%q ok=%v", sub, ok)
	}
	if spec, sub, ok := Lookup("devcontainer.container_env.FOO"); !ok || sub != "FOO" || spec.Key != "devcontainer.container_env" {
		t.Errorf("map entry: spec=%q sub=%q ok=%v", spec.Key, sub, ok)
	}
	if _, _, ok := Lookup("nope.nope"); ok {
		t.Error("unknown key resolved")
	}
}

func TestGetValue(t *testing.T) {
	rebase := true
	prefix := "feat"
	devToolbar := true
	f := File{
		Sync:       &SyncSection{AllowRebase: &rebase},
		Branches:   &BranchesSection{Prefix: &prefix},
		DevToolbar: &DevToolbarSection{Enabled: &devToolbar},
		Tasks:      []TaskEntry{{Label: "web", ContainerPort: 3000}},
	}
	if v, ok := GetValue(f, "sync.allow_rebase"); !ok || v != true {
		t.Errorf("scalar: %v, %v", v, ok)
	}
	if v, ok := GetValue(f, "dev_toolbar.enabled"); !ok || v != true {
		t.Errorf("dev_toolbar.enabled: %v, %v", v, ok)
	}
	if v, ok := GetValue(f, "branches.prefix"); !ok || v != "feat" {
		t.Errorf("string: %v, %v", v, ok)
	}
	if v, ok := GetValue(f, "task"); !ok {
		t.Errorf("task: %v, %v", v, ok)
	}
	if _, ok := GetValue(f, "github.draft_column"); ok {
		t.Error("unset key reported as set")
	}
}
