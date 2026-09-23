package harness

import (
	"fmt"

	"github.com/jmelahman/kanban/internal/kanbantoml"
)

// ReadUserHarness returns the harness ID set in the user config, if any.
func ReadUserHarness() (string, bool) {
	id, ok := readHarnessAt(userOnly())
	return id, ok
}

// Resolve picks the effective harness for a request, honoring user config,
// then the per-repo project config, then the default. Unknown IDs fall
// through to the next layer so a stale entry doesn't strand the user.
func Resolve(repoPath string) Harness {
	merged := kanbantoml.Load(repoPath)
	if merged.Harness != nil && merged.Harness.ID != nil {
		if id := *merged.Harness.ID; IsKnown(id) {
			return Get(id)
		}
	}
	return Default()
}

// ForSession picks the harness for a session: its own stored choice when
// that is a known ID, else the user/project default for repoPath.
func ForSession(sessionHarness, repoPath string) Harness {
	if IsKnown(sessionHarness) {
		return Get(sessionHarness)
	}
	return Resolve(repoPath)
}

// WriteUserHarness sets (or clears, when id == "") the user-config harness
// key, preserving any other top-level keys already in the file.
func WriteUserHarness(id string) error {
	if id != "" && !IsKnown(id) {
		return fmt.Errorf("unknown harness %q", id)
	}
	return kanbantoml.WriteUserHarness(id)
}

// userOnly returns a File loaded from only the user config (no project
// layer), used by ReadUserHarness which exposes user-set state.
func userOnly() kanbantoml.File {
	// Load with empty repoPath skips the project file.
	return kanbantoml.Load("")
}

func readHarnessAt(f kanbantoml.File) (string, bool) {
	if f.Harness == nil || f.Harness.ID == nil || *f.Harness.ID == "" {
		return "", false
	}
	return *f.Harness.ID, true
}
