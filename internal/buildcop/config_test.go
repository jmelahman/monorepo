package buildcop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeProjectToml drops a .kanban.toml in a fresh tempdir and returns the dir
// path, suitable for passing to ResolveConfig.
func writeProjectToml(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if contents != "" {
		if err := os.WriteFile(filepath.Join(dir, ".kanban.toml"), []byte(contents), 0o644); err != nil {
			t.Fatalf("write .kanban.toml: %v", err)
		}
	}
	// Point kanbantoml's user-config lookup at an empty XDG dir so user
	// settings don't bleed in from the host running the tests.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return dir
}

func TestResolveConfigIntervalDefault(t *testing.T) {
	repo := writeProjectToml(t, `[buildcop]
enabled = true
`)
	got := ResolveConfig(repo)
	if got.Interval != DefaultInterval {
		t.Errorf("Interval = %s; want default %s", got.Interval, DefaultInterval)
	}
}

func TestResolveConfigIntervalCustom(t *testing.T) {
	repo := writeProjectToml(t, `[buildcop]
enabled = true
interval = "10m"
`)
	got := ResolveConfig(repo)
	if got.Interval != 10*time.Minute {
		t.Errorf("Interval = %s; want 10m", got.Interval)
	}
}

func TestResolveConfigIntervalInvalidFallsBackToDefault(t *testing.T) {
	repo := writeProjectToml(t, `[buildcop]
enabled = true
interval = "not-a-duration"
`)
	got := ResolveConfig(repo)
	if got.Interval != DefaultInterval {
		t.Errorf("Interval = %s; want default %s after invalid parse", got.Interval, DefaultInterval)
	}
}

func TestResolveConfigIntervalSubSecondRejected(t *testing.T) {
	repo := writeProjectToml(t, `[buildcop]
enabled = true
interval = "5ns"
`)
	got := ResolveConfig(repo)
	if got.Interval != DefaultInterval {
		t.Errorf("Interval = %s; want default %s when below 1s minimum", got.Interval, DefaultInterval)
	}
}
